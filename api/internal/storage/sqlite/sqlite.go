// Package sqlite is the SQLite storage primitive: a connection pool, a
// transaction wrapper, and a Querier interface that DALs accept without caring
// whether they hold the pool or a transaction.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
//
// Every pooled connection gets busy_timeout=5000 (so a writer blocked by
// another connection waits instead of failing with SQLITE_BUSY) and, for
// file-backed databases, journal_mode=WAL. Both are applied as DSN _pragma
// parameters, which the driver runs on each new connection. A pragma the
// caller's DSN already sets is left alone.
//
// Transactions begin with BEGIN IMMEDIATE (_txlock=immediate), so every
// transaction takes the write lock up front. A deferred transaction that reads
// and then writes (the action stack's expected-top check, then its pop) cannot
// upgrade in WAL mode once another writer committed after its first read: it
// fails at once with SQLITE_BUSY_SNAPSHOT, which busy_timeout does not retry.
// Taking the lock first makes the second writer wait, then see the first one's
// result and refuse it as a domain conflict.
func Open(dsn string) (*DB, error) {
	db, err := sql.Open("sqlite", withPragmas(dsn))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &DB{DB: db}, nil
}

// withPragmas appends the opener's per-connection pragmas to dsn, which may be
// a bare path or a file: URI with or without query parameters.
func withPragmas(dsn string) string {
	base, query, _ := strings.Cut(dsn, "?")
	lower := strings.ToLower(query)
	inMemory := base == ":memory:" || strings.Contains(base, ":memory:") ||
		strings.Contains(lower, "mode=memory")

	var extra []string
	if !hasPragma(lower, "busy_timeout") {
		extra = append(extra, "_pragma=busy_timeout(5000)")
	}
	if !strings.Contains(lower, "_txlock=") {
		extra = append(extra, "_txlock=immediate")
	}
	if !inMemory && !hasPragma(lower, "journal_mode") {
		extra = append(extra, "_pragma=journal_mode(wal)")
	}
	if len(extra) == 0 {
		return dsn
	}
	if query != "" {
		extra = append([]string{query}, extra...)
	}
	return base + "?" + strings.Join(extra, "&")
}

func hasPragma(lowerQuery, name string) bool {
	for _, kv := range strings.Split(lowerQuery, "&") {
		k, v, _ := strings.Cut(kv, "=")
		if k == "_pragma" && strings.HasPrefix(v, name) {
			return true
		}
	}
	return false
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
