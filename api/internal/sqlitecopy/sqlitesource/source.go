// Package sqlitesource is the copy tool's SQLite side (ADR 0025 step 9): a working copy of the
// app's SQLite file, which the tool brings to the last SQLite schema and reads. It is the only
// package of the api module that links a SQLite driver; only the tool's binary imports it.
package sqlitesource

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/ncruces/go-sqlite3/driver" // SQLite driver, registered as "sqlite3"
	_ "github.com/ncruces/go-sqlite3/embed"  // the SQLite library the driver runs

	"github.com/ceffo/critical/api/internal/sqlitecopy"
)

// migrationFiles are the 33 migrations of the SQLite engine, as the server applied them until it
// moved to PostgreSQL: they bring a file of any earlier release to the last SQLite schema, the
// one the PostgreSQL baseline squashes. They are applied to the working copy only, never changed.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrations returns the embedded SQLite migrations.
func Migrations() ([]sqlitecopy.Migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read the SQLite migrations: %w", err)
	}
	out := make([]sqlitecopy.Migration, 0, len(entries))
	for _, e := range entries {
		body, err := migrationFiles.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		out = append(out, sqlitecopy.Migration{Name: e.Name(), Body: string(body)})
	}
	return out, nil
}

// Source is a working copy of a SQLite file, in a private temporary directory.
type Source struct {
	db  *sql.DB
	dir string
}

var _ sqlitecopy.Source = (*Source)(nil)

// Open copies the SQLite file at path into a new private temporary directory and opens the copy.
// The original is opened read-only (mode=ro) and copied with VACUUM INTO, which reads the
// committed rows of its write-ahead log too: neither the file nor its WAL is ever written, and a
// server running on it meanwhile keeps running. Close deletes the copy.
func Open(ctx context.Context, path string) (_ *Source, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", path, err)
	}
	if st, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("the SQLite file: %w", err)
	} else if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("the SQLite file %s is not a regular file", abs)
	}
	dir, err := os.MkdirTemp("", "critical-sqlite-copy-")
	if err != nil {
		return nil, fmt.Errorf("create the working directory: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()
	work := filepath.Join(dir, "work.db")
	if err := snapshot(ctx, abs, work); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", "file:"+work)
	if err != nil {
		return nil, fmt.Errorf("open the working copy: %w", err)
	}
	// One connection: the working copy is private and the copy reads it sequentially.
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open the working copy: %w", err)
	}
	return &Source{db: db, dir: dir}, nil
}

// snapshot writes a consistent copy of the database at src into the new file dst.
func snapshot(ctx context.Context, src, dst string) error {
	ro, err := sql.Open("sqlite3", "file:"+src+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open %s read-only: %w", src, err)
	}
	defer func() { _ = ro.Close() }()
	if _, err := ro.ExecContext(ctx, "VACUUM INTO ?", dst); err != nil {
		return fmt.Errorf("copy %s: %w", src, err)
	}
	return nil
}

// Close closes the working copy and deletes it.
func (s *Source) Close() error {
	return errors.Join(s.db.Close(), os.RemoveAll(s.dir))
}

func (s *Source) hasTable(ctx context.Context, name string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		"SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n)
	return n > 0, err
}

// Applied lists the migrations the file records, none when it has no schema_migrations.
func (s *Source) Applied(ctx context.Context) ([]string, error) {
	ok, err := s.hasTable(ctx, "schema_migrations")
	if err != nil || !ok {
		return nil, err
	}
	return s.strings(ctx, "SELECT name FROM schema_migrations ORDER BY name")
}

// Apply runs a migration and records it in one transaction (SQLite DDL is transactional), as the
// server's SQLite runner did; the bookkeeping table is the one that runner created.
func (s *Source) Apply(ctx context.Context, m sqlitecopy.Migration) (err error) {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT (datetime('now')))`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	// Without arguments the driver runs every statement of the file.
	if _, err := tx.ExecContext(ctx, m.Body); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (name) VALUES (?)", m.Name); err != nil {
		return err
	}
	return tx.Commit()
}

// Tables lists the app tables: not SQLite's own (sqlite_sequence), not schema_migrations.
func (s *Source) Tables(ctx context.Context) ([]string, error) {
	return s.strings(ctx, `SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite\_%' ESCAPE '\' AND name <> 'schema_migrations'
		ORDER BY name`)
}

// Columns lists the columns of a table.
func (s *Source) Columns(ctx context.Context, table string) ([]string, error) {
	return s.strings(ctx, "SELECT name FROM pragma_table_info(?) ORDER BY cid", table)
}

// Count is the number of rows of a table.
func (s *Source) Count(ctx context.Context, table string) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM "+quote(table)).Scan(&n)
	return n, err
}

// Sequence is the table's entry in sqlite_sequence (AUTOINCREMENT tables), 0 when it has none.
func (s *Source) Sequence(ctx context.Context, table string) (int64, error) {
	ok, err := s.hasTable(ctx, "sqlite_sequence")
	if err != nil || !ok {
		return 0, err
	}
	var seq sql.NullInt64
	err = s.db.QueryRowContext(ctx, "SELECT max(seq) FROM sqlite_sequence WHERE name = ?", table).Scan(&seq)
	return seq.Int64, err
}

// Rows calls emit with every row of a table in primary-key order (the same order on every run).
// Text comes back as a string, never as bytes: the target has no binary column.
func (s *Source) Rows(ctx context.Context, table string, columns []string, emit func(row []any) error) (err error) {
	key, err := s.strings(ctx, "SELECT name FROM pragma_table_info(?) WHERE pk > 0 ORDER BY pk", table)
	if err != nil {
		return err
	}
	if len(key) == 0 {
		key = []string{"rowid"}
	}
	quoted := make([]string, len(columns))
	for i, c := range columns {
		quoted[i] = quote(c)
	}
	for i, k := range key {
		key[i] = quote(k)
	}
	// Quoted identifiers from the two catalogs, no value.
	query := "SELECT " + strings.Join(quoted, ", ") + " FROM " + quote(table) + " ORDER BY " + strings.Join(key, ", ") //nolint:gosec // identifiers only
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		row := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		for i, v := range row {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			}
		}
		if err := emit(row); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Source) strings(ctx context.Context, query string, args ...any) (out []string, err error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// quote quotes an SQLite identifier.
func quote(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
