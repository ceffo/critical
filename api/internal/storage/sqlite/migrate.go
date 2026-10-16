package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// ErrUnknownMigrations is returned by Migrate when the database records a
// migration this binary does not embed: the schema is newer than the code.
var ErrUnknownMigrations = errors.New("database records migrations this binary does not embed")

// Migrate runs every embedded migration in lexical order and records each one
// in schema_migrations, so a recorded migration is never run again.
//
// Before applying anything it refuses a database that records a migration the
// binary does not embed (ADR 0024 step 6.2): an older binary does not undo a
// newer schema, so it must not run on one. Nothing is applied in that case.
func (d *DB) Migrate(ctx context.Context) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	// Track applied migrations in a small bookkeeping table so 004's ALTER
	// statements (which are not idempotent) can be skipped on subsequent boots.
	if _, err := d.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT (datetime('now')))`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	if err := d.refuseUnknownMigrations(ctx, entries); err != nil {
		return err
	}

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		var applied int
		if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE name = ?", name).Scan(&applied); err != nil {
			return fmt.Errorf("check %s: %w", name, err)
		}
		if applied > 0 {
			continue
		}
		content, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if err := d.applyMigration(ctx, name, string(content)); err != nil {
			return err
		}
	}
	return nil
}

// refuseUnknownMigrations fails with ErrUnknownMigrations, naming every
// recorded migration missing from embedded, in lexical order (ADR 0024 step 6.2).
func (d *DB) refuseUnknownMigrations(ctx context.Context, embedded []fs.DirEntry) error {
	known := make(map[string]bool, len(embedded))
	for _, e := range embedded {
		known[e.Name()] = true
	}
	rows, err := d.QueryContext(ctx, "SELECT name FROM schema_migrations ORDER BY name")
	if err != nil {
		return fmt.Errorf("list applied migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var unknown []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("list applied migrations: %w", err)
		}
		if !known[name] {
			unknown = append(unknown, name)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("list applied migrations: %w", err)
	}
	if len(unknown) > 0 {
		return fmt.Errorf("%w: %s; run a build that embeds them, or restore the database from the "+
			"snapshot taken before they were applied (docs/adr/0024-deployment.md steps 6.1 and 9.3)",
			ErrUnknownMigrations, strings.Join(unknown, ", "))
	}
	return nil
}

// applyMigration runs one migration file and records it as applied in a single
// transaction (SQLite DDL is transactional), so a failure or a crash leaves the
// schema and data exactly as they were and the migration is retried next boot.
func (d *DB) applyMigration(ctx context.Context, name, content string) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin %s: %w", name, err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := execStatementsOn(ctx, tx, name, content); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (name) VALUES (?)", name); err != nil {
		return fmt.Errorf("record %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", name, err)
	}
	return nil
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// execStatements runs sql statement by statement outside a transaction.
func (d *DB) execStatements(ctx context.Context, migration, sql string) error {
	return execStatementsOn(ctx, d.DB, migration, sql)
}

// execStatements splits sql on semicolons (outside comments and string
// literals) and executes each statement individually. Statements that fail with "duplicate column name" are logged
// and skipped — this makes ALTER TABLE ADD COLUMN idempotent when a migration
// was partially applied by an older runner that lacked tracking.
func execStatementsOn(ctx context.Context, d execer, migration, sql string) error {
	for _, stmt := range splitStatements(sql) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := d.ExecContext(ctx, stmt); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				slog.Default().Warn("migration: column already exists, skipping", "migration", migration, "stmt", stmt[:min(len(stmt), 60)])
				continue
			}
			return fmt.Errorf("exec %s: %w", migration, err)
		}
	}
	return nil
}

// splitStatements splits sql on ';', ignoring any ';' inside a "--" line
// comment, a "/* */" block comment or a '...' string literal (” escapes a
// quote). Comments stay in the returned statements; blank ones are dropped.
func splitStatements(sql string) []string {
	var out []string
	start := 0
	flush := func(end int) {
		if stmt := strings.TrimSpace(sql[start:end]); stmt != "" {
			out = append(out, stmt)
		}
	}
	for i := 0; i < len(sql); i++ {
		switch {
		case sql[i] == '\'':
			// A doubled quote closes and reopens the literal, so plain
			// toggling handles the '' escape.
			for i++; i < len(sql) && sql[i] != '\''; i++ {
			}
		case strings.HasPrefix(sql[i:], "--"):
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
		case strings.HasPrefix(sql[i:], "/*"):
			if end := strings.Index(sql[i+2:], "*/"); end >= 0 {
				i += 2 + end + 1
			} else {
				i = len(sql)
			}
		case sql[i] == ';':
			flush(i)
			start = i + 1
		}
	}
	flush(len(sql))
	return out
}
