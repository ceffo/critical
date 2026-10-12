package sqlite

import (
	"context"
	"database/sql"
)

type txKey struct{}

// InTx runs fn inside one transaction carried by the context it passes to fn.
// DALs built on Conn pick that transaction up, so writes from several adapters
// (for example a state write and its action-log entry) commit or roll back
// together. A nested InTx joins the outer transaction instead of opening a new
// one. *DB satisfies domain.TxRunner through this method.
func (d *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}
	return d.WrapTx(ctx, func(q Querier) error {
		tx, _ := q.(*sql.Tx)
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// Conn returns the transaction carried by ctx (see InTx), or fallback when ctx
// carries none.
func Conn(ctx context.Context, fallback Querier) Querier {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok && tx != nil {
		return tx
	}
	return fallback
}
