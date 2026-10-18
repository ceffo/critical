// Package postgres is the PostgreSQL storage (ADR 0025): the connection pool, the write
// transactions with their writer lock, the schema migrations the server applies at boot, the
// seeds and the spell catalogue import. The adapters run their sqlc-generated queries on the
// connection Routed picks: the transaction the context carries, else the pool.
package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
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
// included, waits up to connectWait for a database that is still starting; ending ctx (the server's
// SIGINT/SIGTERM) stops the first one's wait at once. It runs no migration and writes nothing: those
// are the startup steps the composition root drives.
func Open(ctx context.Context, dsn string) (*DB, error) {
	connector, err := pq.NewConnector(dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	pool, err := openPool(ctx, waitingConnector{Connector: connector, wait: connectWait, retryEvery: connectRetryEvery})
	if err != nil {
		return nil, err
	}
	return newDB(pool), nil
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

// connectWait is how long a new connection waits for the database to accept it, retrying every
// connectRetryEvery. The fly cluster stops when idle (ADR 0024 step 2) and the first connection to
// its Flycast address starts it: until PostgreSQL is up, connections are closed (EOF), refused or
// answered "the database system is starting up" (what is retried: retryable). A server booting meanwhile waits instead of exiting (its
// HTTP listener, and so its health check, comes up once the database answers; fly keeps waiting for
// the check during a deploy), and the first request after the cluster's idle stop waits a few
// seconds instead of failing.
const (
	connectWait       = 60 * time.Second
	connectRetryEvery = time.Second
)

// waitingConnector is a driver.Connector whose Connect waits for a starting database
// (waitForDatabase), for at most wait and never beyond the caller's context.
type waitingConnector struct {
	driver.Connector
	wait, retryEvery time.Duration
}

func (c waitingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, c.wait)
	defer cancel()
	var conn driver.Conn
	err := waitForDatabase(ctx, func(ctx context.Context) error {
		var err error
		conn, err = c.Connector.Connect(ctx)
		return err
	}, c.retryEvery)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// waitForDatabase calls connect until it succeeds, retrying every retryEvery while the error is
// one a stopped or starting database gives (retryable). Any other error is returned at once, since
// waiting cannot fix it. When ctx ends first, it returns the last error.
func waitForDatabase(ctx context.Context, connect func(context.Context) error, retryEvery time.Duration) error {
	for attempt := 1; ; attempt++ {
		err := connect(ctx)
		if err == nil {
			return nil
		}
		if !retryable(err) {
			return err
		}
		if ctx.Err() != nil {
			return fmt.Errorf("gave up after %d attempts: %w", attempt, err)
		}
		slog.Warn("postgres: database not accepting connections yet, retrying", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("gave up after %d attempts: %w", attempt, err)
		case <-time.After(retryEvery):
		}
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
