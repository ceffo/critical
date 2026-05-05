// Package sqlite is the SQLite storage primitive: a connection pool, a
// transaction wrapper, and a Querier interface that DALs accept without caring
// whether they hold the pool or a transaction.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // SQLite driver
)

// ErrNoRows is the storage-level "no rows" sentinel. DALs translate this to
// their own domain not-found error before returning to a service.
var ErrNoRows = sql.ErrNoRows

// Querier is the subset of *sql.DB / *sql.Tx that DALs need. Both the pool and
// a transaction satisfy it, so a DAL written against Querier can run inside or
// outside a transaction without ceremony.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// DB is the storage handle exposed to the composition root. It wraps a
// *sql.DB and adds WrapTx.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite database at dsn and returns a *DB. It does
// NOT run migrations or seed data — those are separate steps the composition
// root drives.
func Open(dsn string) (*DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &DB{DB: db}, nil
}

// WrapTx executes body inside a transaction. body receives a Querier bound to
// the transaction; it should pass that Querier into a fresh DAL constructor and
// call methods on the tx-bound DAL. On error or panic, the transaction is
// rolled back.
func (d *DB) WrapTx(ctx context.Context, body func(Querier) error) (err error) {
	tx, err := d.BeginTx(ctx, nil)
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
	return body(tx)
}
