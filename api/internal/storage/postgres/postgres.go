// Package postgres is the PostgreSQL storage (ADR 0025): the connection pool, the write
// transactions with their writer lock, the schema migrations the server applies at boot, the
// seeds and the spell catalogue import. The adapters run their sqlc-generated queries on the
// connection Routed picks: the transaction the context carries, else the pool.
package postgres

import (
	"cmp"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/lib/pq" // PostgreSQL driver

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// DBTX is what sqlc-generated query code runs on. Both the pool and a transaction satisfy it.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Tx is an open transaction: *sql.Tx.
type Tx interface {
	DBTX
	Commit() error
	Rollback() error
}

// DB is the storage handle the composition root holds.
type DB struct {
	pool *sql.DB
	// begin opens a transaction, lock takes the writer lock on it (ADR 0025 step 4).
	begin func(ctx context.Context, opts *sql.TxOptions) (Tx, error)
	lock  func(ctx context.Context, tx DBTX) error
	// q runs the storage's own statements (seeds, catalogue import) on the transaction the
	// context carries, else on the pool.
	q storeq.Querier
	// migrations opens the one transaction a migration run happens in (ADR 0025 step 3.1).
	migrations func(ctx context.Context, fn func(migrationTx) error) error
}

// Open connects to the PostgreSQL database at dsn (CRITICAL_DSN, a PostgreSQL connection
// string or URL) and checks the connection. Every new connection of the pool, this first one
// included, goes through the gate: it waits up to connectWait for a database that is still
// starting, never beyond the caller's context (the server's SIGINT/SIGTERM at boot, a request's
// deadline afterwards). It runs no migration and writes nothing: those are the startup steps the
// composition root drives.
func Open(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := connectConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	connector, err := pq.NewConnectorConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	pool, err := openPool(ctx, newGate(connector, connectWait, connectRetryEvery, attemptTimeout))
	if err != nil {
		return nil, err
	}
	return newDB(pool), nil
}

// connectConfig is the driver configuration of dsn. Without a connect timeout lib/pq bounds only
// the TCP dial by the context: a database that accepts the connection but never answers the
// handshake (a node starved of memory) would hold the attempt forever. So one attempt gets
// attemptTimeout, unless the DSN sets its own connect_timeout.
func connectConfig(dsn string) (pq.Config, error) {
	cfg, err := pq.NewConfig(dsn)
	if err != nil {
		return pq.Config{}, err
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = attemptTimeout
	}
	return cfg, nil
}

// openPool opens the pool over connector and checks its first connection under ctx.
func openPool(ctx context.Context, connector driver.Connector) (*sql.DB, error) {
	pool := sql.OpenDB(connector)
	if err := pool.PingContext(ctx); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	return pool, nil
}

// connectWait is how long a new connection waits for the database to accept it; the gate tries
// again every connectRetryEvery. The fly cluster stops when idle (ADR 0024 step 2) and the first
// connection to its Flycast address starts it: until PostgreSQL is up, connections are closed
// (EOF), refused or answered "the database system is starting up" (what is retried: retryable). A
// server booting meanwhile waits instead of exiting (its HTTP listener, and so its health check,
// comes up once the database answers; fly keeps waiting for the check during a deploy). A request
// waits at most until its own deadline (middleware.Deadline), far shorter.
//
// attemptTimeout bounds one connection attempt, the handshake included (lib/pq's
// connect_timeout, unless the DSN sets one).
const (
	connectWait       = 60 * time.Second
	connectRetryEvery = time.Second
	attemptTimeout    = 5 * time.Second
)

// ErrUnavailable is what a new connection fails with when the database did not answer within the
// caller's wait: it is stopped, starting, or too starved to accept connections.
var ErrUnavailable = errors.New("database unavailable")

// gate is the pool's driver.Connector. While the database answers, each new connection is one
// attempt of the inner connector. Once an attempt fails as a stopped or starting database fails
// (retryable), the gate is down: one probe alone retries every retryEvery until the database
// answers, and every connection asked for meanwhile waits for that, for at most wait and never
// beyond its caller's context, without dialling. A burst of requests during an outage then puts
// one connection attempt per second on the database instead of one per request, and no request
// waits longer than its own deadline.
type gate struct {
	inner                     driver.Connector
	wait, retryEvery, attempt time.Duration

	mu sync.Mutex
	// down is open while the database is down and closed when it answers again; nil while up.
	down chan struct{}
	// lastErr is the last failed attempt's error while down.
	lastErr error
	// closed ends the probe when the pool closes.
	closed    chan struct{}
	closeOnce sync.Once
}

var _ driver.Connector = (*gate)(nil)

func newGate(inner driver.Connector, wait, retryEvery, attempt time.Duration) *gate {
	return &gate{inner: inner, wait: wait, retryEvery: retryEvery, attempt: attempt, closed: make(chan struct{})}
}

// Driver is the inner connector's driver.
func (g *gate) Driver() driver.Driver { return g.inner.Driver() }

// Close stops the probe; sql.DB.Close calls it.
func (g *gate) Close() error {
	g.closeOnce.Do(func() { close(g.closed) })
	return nil
}

// Connect returns a new connection: at once while the database answers, after waiting for the
// probe while it does not. It fails with ErrUnavailable when its wait or its caller's context ends
// first, and with the attempt's own error when waiting cannot help (a wrong password, a DSN that
// cannot work: retryable).
func (g *gate) Connect(ctx context.Context) (driver.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, g.wait)
	defer cancel()
	for {
		if down, last := g.state(); down != nil {
			select {
			case <-down:
			case <-ctx.Done():
				return nil, fmt.Errorf("%w: %w", ErrUnavailable, cmp.Or(last, ctx.Err()))
			}
		}
		conn, err := g.try(ctx)
		switch {
		case err == nil:
			return conn, nil
		case ctx.Err() != nil:
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		case !retryable(err):
			return nil, err
		}
		g.markDown(err)
	}
}

// try is one attempt of the inner connector, given up when ctx ends even if the driver does not
// watch it; a connection that arrives after that is closed.
func (g *gate) try(ctx context.Context) (driver.Conn, error) {
	type result struct {
		conn driver.Conn
		err  error
	}
	done := make(chan result, 1)
	go func() {
		conn, err := g.inner.Connect(ctx)
		done <- result{conn, err}
	}()
	// A failed attempt's connection is never used, not even closed: lib/pq returns a nil *conn
	// with its error, a non-nil driver.Conn whose Close panics.
	select {
	case r := <-done:
		if r.err != nil {
			return nil, r.err
		}
		return r.conn, nil
	case <-ctx.Done():
		go func() {
			if r := <-done; r.err == nil {
				_ = r.conn.Close()
			}
		}()
		return nil, ctx.Err()
	}
}

func (g *gate) state() (chan struct{}, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.down, g.lastErr
}

// markDown records a failed attempt and, when the gate was up, puts it down and starts the probe.
func (g *gate) markDown(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastErr = err
	if g.down != nil {
		return
	}
	g.down = make(chan struct{})
	slog.Warn("postgres: database not accepting connections, waiting for it", "error", err)
	go g.probe()
}

// markUp puts the gate up and releases every connection waiting.
func (g *gate) markUp() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.down != nil {
		close(g.down)
		g.down, g.lastErr = nil, nil
	}
}

// probe retries every retryEvery, each attempt bounded by g.attempt, until the database answers or
// fails with an error waiting cannot fix (the waiting connections then try themselves and get it),
// or the pool closes. Its connection is closed at once: it only proves the database answers.
func (g *gate) probe() {
	for n := 1; ; n++ {
		select {
		case <-g.closed:
			return
		case <-time.After(g.retryEvery):
		}
		ctx, cancel := context.WithTimeout(context.Background(), g.attempt)
		conn, err := g.try(ctx)
		cancel()
		switch {
		case err == nil:
			_ = conn.Close()
			slog.Info("postgres: database answers again", "attempts", n)
			g.markUp()
			return
		case !retryable(err) && !errors.Is(err, context.DeadlineExceeded):
			g.markUp()
			return
		}
		slog.Warn("postgres: database not accepting connections yet, retrying", "attempt", n, "error", err)
		g.mu.Lock()
		g.lastErr = err
		g.mu.Unlock()
	}
}

// retryable tells whether a failed connection attempt may succeed later, against a cluster that is
// stopped or starting. Exactly these are retried:
//   - SQLSTATE 57P03 cannot_connect_now ("the database system is starting up");
//   - io.EOF and io.ErrUnexpectedEOF: the connection closed during the handshake, which is what the
//     Flycast proxy of a stopped cluster answers while it starts it;
//   - a network error (net.Error: refused, reset, unreachable, a dial or read timeout), except a DNS
//     lookup that found no such host.
//
// Everything else fails at once: any other PostgreSQL error (a wrong password 28P01, a missing
// database 3D000), a DNS name that does not exist (a typo in the DSN), pq.ErrSSLNotSupported
// (sslmode=require against a server without TLS), a TLS or certificate error, an authentication
// method lib/pq does not support, and any other driver or configuration error.
func retryable(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == cannotConnectNow
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return !dnsErr.IsNotFound
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// cannotConnectNow is the SQLSTATE of "the database system is starting up".
const cannotConnectNow = "57P03"

// maxOpenConns caps each server's pool. Every writer waiting on the writer lock holds a connection,
// so without a cap a burst of writes would open one connection each. The fly cluster is shared with
// another app (ADR 0024 step 2) and is a 256 MB node: every PostgreSQL backend costs it several MB,
// and with ten per server (two production machines and one staging) the node ran out of memory and
// requests stalled for seconds. One writer runs at a time and reads are short, so four per server
// (at most 12 for this app) serve a player's page load, whose parallel requests wait in the pool
// instead of failing.
const maxOpenConns = 4

// maxIdleConns is how many unused connections stay open: one, enough for the next request without
// a handshake. database/sql's default of two would keep a spare backend alive for nothing.
const maxIdleConns = 1

// maxIdleTime closes a pooled connection left unused that long: a burst of requests opens a few
// backends and they go away soon after, instead of weighing on the shared node. The fly cluster
// also stops after an hour with no open connection (ADR 0024 step 2): idle connections held
// forever would keep it, and the other app's database, running for good. A connection is never
// reused across a stop of the cluster for lack of traffic, so the first request after one opens a
// new connection instead of failing on a dead one.
const maxIdleTime = 30 * time.Second

// poolSettings is the part of *sql.DB newDB configures.
type poolSettings interface {
	SetMaxOpenConns(n int)
	SetMaxIdleConns(n int)
	SetConnMaxIdleTime(d time.Duration)
}

func configurePool(p poolSettings) {
	p.SetMaxOpenConns(maxOpenConns)
	p.SetMaxIdleConns(maxIdleConns)
	p.SetConnMaxIdleTime(maxIdleTime)
}

func newDB(pool *sql.DB) *DB {
	configurePool(pool)
	d := &DB{
		pool: pool,
		begin: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return pool.BeginTx(ctx, opts)
		},
		lock: func(ctx context.Context, tx DBTX) error { return storeq.New(tx).LockWriters(ctx) },
		q:    storeq.New(Routed(pool)),
	}
	d.migrations = d.migrationRun
	return d
}

// writeTx are the options of every write transaction: READ COMMITTED, stated rather than
// inherited (ADR 0025 step 4.2), so every statement after the lock sees what the writer before
// committed. A stricter level would take its snapshot when the lock statement starts, before the
// lock is granted, and the second writer would not see the first one's commit.
var writeTx = &sql.TxOptions{Isolation: sql.LevelReadCommitted}

// Close closes the pool.
func (d *DB) Close() error { return d.pool.Close() }

// Pool is the connection pool, for Routed.
func (d *DB) Pool() DBTX { return d.pool }

// InTx runs fn inside one write transaction carried by the context it passes to fn. Adapters
// built over Routed pick that transaction up, so writes from several adapters (a state write and
// its action-log entry) commit or roll back together. A nested InTx joins the outer transaction.
// *DB satisfies domain.TxRunner through this method.
//
// The transaction's first statement takes the writer lock (ADR 0025 step 4): a second writer
// waits until the first commits or rolls back, then reads its result, so a stale expected top of
// the action stack is refused as a conflict (409), never a database error. Nothing releases the
// lock but the commit or the rollback. No slow work inside fn: every writer waits for it.
func (d *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := txFrom(ctx); ok {
		return fn(ctx)
	}
	tx, err := d.begin(ctx, writeTx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
				err = fmt.Errorf("%w; rollback: %w", err, rbErr)
			}
			return
		}
		err = tx.Commit()
	}()
	if err := d.lock(ctx, tx); err != nil {
		return fmt.Errorf("take the writer lock: %w", err)
	}
	return fn(WithTx(ctx, tx))
}

type txKey struct{}

// WithTx returns ctx carrying tx as its transaction, as InTx passes it to fn: Conn and Routed
// run on tx under that context.
func WithTx(ctx context.Context, tx DBTX) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// txFrom returns the transaction ctx carries. A nil *sql.Tx stored as a DBTX is a non-nil
// interface, so it is checked for explicitly: it means none.
func txFrom(ctx context.Context) (DBTX, bool) {
	tx, ok := ctx.Value(txKey{}).(DBTX)
	if !ok || tx == nil {
		return nil, false
	}
	if t, isTx := tx.(*sql.Tx); isTx && t == nil {
		return nil, false
	}
	return tx, true
}

// Conn returns the transaction carried by ctx (see InTx), or fallback when ctx carries none.
func Conn(ctx context.Context, fallback DBTX) DBTX {
	if tx, ok := txFrom(ctx); ok {
		return tx
	}
	return fallback
}

// Routed returns a DBTX that runs each statement on the transaction carried by the statement's
// context (see InTx), or on pool when there is none. The composition root builds the
// sqlc-generated query code of every adapter over it, so an adapter's queries join the caller's
// transaction (ADR 0025 step 5.2).
func Routed(pool DBTX) DBTX { return routed{pool: pool} }

type routed struct{ pool DBTX }

func (r routed) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return Conn(ctx, r.pool).ExecContext(ctx, query, args...)
}

func (r routed) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return Conn(ctx, r.pool).PrepareContext(ctx, query)
}

func (r routed) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return Conn(ctx, r.pool).QueryContext(ctx, query, args...)
}

func (r routed) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return Conn(ctx, r.pool).QueryRowContext(ctx, query, args...)
}
