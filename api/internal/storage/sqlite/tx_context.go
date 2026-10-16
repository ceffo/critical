package sqlite

import (
	"context"
	"database/sql"
)

type txKey struct{}

// DBTX is what sqlc-generated query code runs on. Both the pool (*DB, through
// its *sql.DB) and a transaction (*sql.Tx) satisfy it.
type DBTX interface {
	Querier
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// InTx runs fn inside one transaction carried by the context it passes to fn.
// DALs built on Conn or Routed pick that transaction up, so writes from several
// adapters (for example a state write and its action-log entry) commit or roll
// back together. A nested InTx joins the outer transaction instead of opening a
// new one. No slow work inside fn: every transaction holds the write lock from
// its BEGIN. *DB satisfies domain.TxRunner through this method.
func (d *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFrom(ctx); ok {
		return fn(ctx)
	}
	return d.WrapTx(ctx, func(q Querier) error {
		tx, _ := q.(*sql.Tx)
		return fn(withTx(ctx, tx))
	})
}

func withTx(ctx context.Context, tx DBTX) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func txFrom(ctx context.Context) (DBTX, bool) {
	tx, ok := ctx.Value(txKey{}).(DBTX)
	return tx, ok && tx != nil
}

// Conn returns the transaction carried by ctx (see InTx), or fallback when ctx
// carries none.
func Conn(ctx context.Context, fallback Querier) Querier {
	if tx, ok := txFrom(ctx); ok {
		return tx
	}
	return fallback
}

// Routed returns a DBTX that runs each statement on the transaction carried by
// the statement's context (see InTx), or on pool when there is none. The
// composition root builds the sqlc-generated query code of an adapter over it,
// so an adapter's queries join the caller's transaction as Conn does
// (ADR 0025 step 5.2).
func Routed(pool DBTX) DBTX { return routed{pool: pool} }

type routed struct{ pool DBTX }

func (r routed) pick(ctx context.Context) DBTX {
	if tx, ok := txFrom(ctx); ok {
		return tx
	}
	return r.pool
}

func (r routed) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.pick(ctx).ExecContext(ctx, query, args...)
}

func (r routed) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return r.pick(ctx).PrepareContext(ctx, query)
}

func (r routed) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.pick(ctx).QueryContext(ctx, query, args...)
}

func (r routed) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return r.pick(ctx).QueryRowContext(ctx, query, args...)
}
