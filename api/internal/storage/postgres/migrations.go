package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// migrationFiles are the schema migrations, applied in lexical order. Schema only: no migration
// holds an INSERT, UPDATE or DELETE (ADR 0025 step 2.1). 001_baseline.sql is the schema the
// SQLite migrations built.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// ErrUnknownMigrations is returned by Migrate when the database records a migration this binary
// does not embed: the schema is newer than the code.
var ErrUnknownMigrations = errors.New("database records migrations this binary does not embed")

// migration is one embedded migration file.
type migration struct{ name, body string }

// migrationTx is what a migration run does inside its one transaction (ADR 0025 step 3).
type migrationTx interface {
	// Lock takes the writer lock of every write transaction (ADR 0025 step 4).
	Lock(ctx context.Context) error
	// EnsureTable creates schema_migrations on a database that has none.
	EnsureTable(ctx context.Context) error
	// Applied lists the recorded migrations.
	Applied(ctx context.Context) ([]string, error)
	// Apply runs a migration's statements and records it as applied.
	Apply(ctx context.Context, m migration) error
	// Store runs the storage's sqlc queries on the run's transaction (the data steps).
	Store() storeq.Querier
}

// dataStep is data a migration needs before its schema change, written in Go on the run's
// transaction: migration files hold no data (ADR 0025 step 2.1).
type dataStep func(ctx context.Context, q storeq.Querier) error

// ownerKeysMigration makes character_id required on weapons, items, proficiencies and bard
// skills. Before it the run gives every row without an owner to the seed character (ownRows):
// rows stored before migration 002, or written by a release without it during a rolling deploy,
// including on a database that gets 002 and this one in the same run.
const ownerKeysMigration = "004_per_character_keys.sql"

// Migrate applies every embedded migration the database does not record yet, in lexical order,
// and records each one in schema_migrations (ADR 0025 step 3). seedsDir is the seeds' directory:
// its character.json names the seed character, the owner of rows without one before
// ownerKeysMigration.
func (d *DB) Migrate(ctx context.Context, seedsDir string) error {
	embedded, err := embeddedMigrations()
	if err != nil {
		return err
	}
	return d.migrations(ctx, func(tx migrationTx) error {
		return runMigrations(ctx, tx, embedded, migrationDataSteps(seedsDir))
	})
}

// migrationDataSteps are the data steps run just before the named migrations.
func migrationDataSteps(seedsDir string) map[string]dataStep {
	characterPath := seedfile.Path(seedsDir, seedfile.CharacterFile)
	return map[string]dataStep{
		ownerKeysMigration: func(ctx context.Context, q storeq.Querier) error { return ownRows(ctx, q, characterPath) },
	}
}

func embeddedMigrations() ([]migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	out := make([]migration, 0, len(entries))
	for _, e := range entries {
		body, err := migrationFiles.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		out = append(out, migration{name: e.Name(), body: string(body)})
	}
	return out, nil
}

// runMigrations is one migration run, in one transaction holding the writer lock from its first
// statement (ADR 0025 step 3.1): of two machines starting together, the second waits, then finds
// every migration recorded and applies none. A failure rolls the whole run back.
//
// Before applying anything it refuses a database that records a migration the binary does not
// embed (ADR 0024 step 6.2): an older binary does not undo a newer schema, so it must not run on
// one. A pending migration with a data step in steps gets it run just before it, in the same
// transaction.
func runMigrations(ctx context.Context, tx migrationTx, embedded []migration, steps map[string]dataStep) error {
	if err := tx.Lock(ctx); err != nil {
		return fmt.Errorf("take the writer lock: %w", err)
	}
	if err := tx.EnsureTable(ctx); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	applied, err := tx.Applied(ctx)
	if err != nil {
		return fmt.Errorf("list applied migrations: %w", err)
	}
	known := make(map[string]bool, len(embedded))
	for _, m := range embedded {
		known[m.name] = true
	}
	done := make(map[string]bool, len(applied))
	var unknown []string
	for _, name := range applied {
		done[name] = true
		if !known[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("%w: %s; run a build that embeds them, or restore the database from the "+
			"backup taken before they were applied (docs/adr/0024-deployment.md steps 6.1 and 9.3)",
			ErrUnknownMigrations, strings.Join(unknown, ", "))
	}
	pending := make([]migration, 0, len(embedded))
	for _, m := range embedded {
		if !done[m.name] {
			pending = append(pending, m)
		}
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].name < pending[j].name })
	for _, m := range pending {
		if step, ok := steps[m.name]; ok {
			if err := step(ctx, tx.Store()); err != nil {
				return fmt.Errorf("before %s: %w", m.name, err)
			}
		}
		if err := tx.Apply(ctx, m); err != nil {
			return fmt.Errorf("apply %s: %w", m.name, err)
		}
	}
	return nil
}

// migrationRun opens the run's transaction and commits it when fn succeeds.
func (d *DB) migrationRun(ctx context.Context, fn func(migrationTx) error) (err error) {
	tx, err := d.begin(ctx, writeTx)
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()
	return fn(pgMigrationTx{tx: tx, lock: d.lock})
}

// pgMigrationTx runs a migration run's steps on its PostgreSQL transaction. The bookkeeping
// table is not part of the migrations (the guard reads it before any is applied), so its
// statements are here rather than in the sqlc queries.
type pgMigrationTx struct {
	tx   DBTX
	lock func(ctx context.Context, tx DBTX) error
}

func (m pgMigrationTx) Lock(ctx context.Context) error { return m.lock(ctx, m.tx) }

func (m pgMigrationTx) Store() storeq.Querier { return storeq.New(m.tx) }

func (m pgMigrationTx) EnsureTable(ctx context.Context) error {
	_, err := m.tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name       TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`)
	return err
}

func (m pgMigrationTx) Applied(ctx context.Context) (names []string, err error) {
	rows, err := m.tx.QueryContext(ctx, `SELECT name FROM schema_migrations ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// Apply runs the whole file as one simple-protocol statement list (no arguments), so a file holds
// any number of statements; DDL is transactional in PostgreSQL.
func (m pgMigrationTx) Apply(ctx context.Context, mig migration) error {
	if _, err := m.tx.ExecContext(ctx, mig.body); err != nil {
		return err
	}
	_, err := m.tx.ExecContext(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, mig.name)
	return err
}
