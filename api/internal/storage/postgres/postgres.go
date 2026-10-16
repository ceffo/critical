// Package postgres is the PostgreSQL storage (ADR 0025): the connection pool, the write
// transactions with their writer lock, the schema migrations the server applies at boot, the
// seeds and the spell catalogue import. The adapters run their sqlc-generated queries on the
// connection Routed picks: the transaction the context carries, else the pool.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/lib/pq" // PostgreSQL driver

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
// string or URL) and checks the connection. It runs no migration and writes nothing: those are
// the startup steps the composition root drives.
func Open(dsn string) (*DB, error) {
	pool, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := pool.Ping(); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	return newDB(pool), nil
}

func newDB(pool *sql.DB) *DB {
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
