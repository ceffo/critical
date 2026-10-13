package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate runs every embedded migration in lexical order. It is idempotent only
// to the extent the SQL is — see CREATE TABLE IF NOT EXISTS in the migration
// files. ALTER TABLE statements (004) will fail on second run; that is the same
// behavior as the previous implementation.
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

// execStatements splits sql on semicolons and executes each statement
// individually. Statements that fail with "duplicate column name" are logged
// and skipped — this makes ALTER TABLE ADD COLUMN idempotent when a migration
// was partially applied by an older runner that lacked tracking.
func execStatementsOn(ctx context.Context, d execer, migration, sql string) error {
	for _, stmt := range strings.Split(sql, ";") {
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
