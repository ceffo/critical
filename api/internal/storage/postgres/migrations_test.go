package postgres

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// dataStatement matches a data statement's keyword; lineComment is a "--" comment to the end of
// its line (the migrations use no block comment and no string holding "--").
var (
	dataStatement = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE)\b`)
	lineComment   = regexp.MustCompile(`--[^\n]*`)
)

// ADR 0025 step 2.1: no migration holds data, only schema.
func TestMigrations_Step2_1_SchemaOnly(t *testing.T) {
	entries, err := migrationFiles.ReadDir("migrations")
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	for _, e := range entries {
		body, err := migrationFiles.ReadFile("migrations/" + e.Name())
		require.NoError(t, err)
		code := lineComment.ReplaceAllString(string(body), "")
		assert.Empty(t, dataStatement.FindAllString(code, -1), "%s holds a data statement", e.Name())
		assert.True(t, strings.HasSuffix(e.Name(), ".sql"), "%s is not a .sql migration", e.Name())
	}
}

// The scan above finds a data statement outside a comment, and only there.
func TestMigrations_DataStatementScanFindsOne(t *testing.T) {
	code := lineComment.ReplaceAllString("-- no INSERT here\nCREATE TABLE t (id BIGINT);\ninsert into t values (1);", "")
	assert.Equal(t, []string{"insert"}, dataStatement.FindAllString(code, -1))
}

// fakeMigrations is a migration run's transaction: it records every step and holds the recorded
// migrations.
type fakeMigrations struct {
	recorded []string
	steps    []string
	applyErr error
	// store is what the data steps run on; onApply sees each migration as it is applied.
	store   storeq.Querier
	onApply func(migration)
}

func (f *fakeMigrations) Store() storeq.Querier { return f.store }

func (f *fakeMigrations) Lock(context.Context) error {
	f.steps = append(f.steps, "lock")
	return nil
}

func (f *fakeMigrations) EnsureTable(context.Context) error {
	f.steps = append(f.steps, "ensure table")
	return nil
}

func (f *fakeMigrations) Applied(context.Context) ([]string, error) {
	f.steps = append(f.steps, "applied")
	return append([]string(nil), f.recorded...), nil
}

func (f *fakeMigrations) Apply(_ context.Context, m migration) error {
	f.steps = append(f.steps, "apply "+m.name)
	if f.onApply != nil {
		f.onApply(m)
	}
	if f.applyErr != nil {
		return f.applyErr
	}
	f.recorded = append(f.recorded, m.name)
	return nil
}

func migrations(names ...string) []migration {
	out := make([]migration, len(names))
	for i, n := range names {
		out[i] = migration{name: n, body: "-- " + n}
	}
	return out
}

// ADR 0024 step 6.2: a database that records a migration the binary does not embed (a rollback
// to an older build across a migration, or a dev database migrated on a newer branch) is
// refused, naming every unknown migration and the restore step, and nothing is applied to it.
func TestMigrate_Step6_2_RefusesMigrationsTheBinaryDoesNotEmbed(t *testing.T) {
	f := &fakeMigrations{recorded: []string{"001_baseline.sql", "999_newer_b.sql", "998_newer_a.sql"}}

	err := runMigrations(context.Background(), f, migrations("001_baseline.sql", "002_next.sql"), nil)

	require.ErrorIs(t, err, ErrUnknownMigrations)
	assert.ErrorContains(t, err, "998_newer_a.sql, 999_newer_b.sql")
	assert.ErrorContains(t, err, "docs/adr/0024-deployment.md steps 6.1 and 9.3")
	assert.NotContains(t, err.Error(), "001_baseline.sql", "an embedded migration is not unknown")
	assert.Equal(t, []string{"lock", "ensure table", "applied"}, f.steps, "refused before applying anything")
}

// ADR 0024 step 6.2, the other side: a fresh database gets every embedded migration, and one
// that records exactly the embedded migrations starts and is given nothing.
func TestMigrate_Step6_2_AcceptsADatabaseItsBinaryMigrated(t *testing.T) {
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	require.NotEmpty(t, embedded)
	f := &fakeMigrations{}

	require.NoError(t, runMigrations(context.Background(), f, embedded, nil), "fresh database")
	require.Len(t, f.recorded, len(embedded))
	f.steps = nil
	require.NoError(t, runMigrations(context.Background(), f, embedded, nil), "fully migrated database")
	assert.Equal(t, []string{"lock", "ensure table", "applied"}, f.steps, "nothing applied again")
}

// ADR 0025 steps 3 and 3.1: the run takes the writer lock before anything else, then applies the
// pending migrations in lexical order, whatever order they are embedded in, skipping the
// recorded ones.
func TestMigrate_Step3_1_LocksThenAppliesThePendingInOrder(t *testing.T) {
	f := &fakeMigrations{recorded: []string{"001_baseline.sql"}}

	require.NoError(t, runMigrations(context.Background(), f,
		migrations("003_c.sql", "001_baseline.sql", "010_d.sql", "002_b.sql"), nil))

	assert.Equal(t, []string{"lock", "ensure table", "applied", "apply 002_b.sql", "apply 003_c.sql", "apply 010_d.sql"}, f.steps)
}

// A migration that fails stops the run with its name: the run's one transaction rolls back
// whole (migrationRun), and the next boot retries it.
func TestMigrate_AFailingMigrationStopsTheRun(t *testing.T) {
	f := &fakeMigrations{applyErr: errors.New("syntax error")}

	err := runMigrations(context.Background(), f, migrations("001_baseline.sql", "002_b.sql"), nil)

	require.ErrorContains(t, err, "apply 001_baseline.sql: syntax error")
	assert.Equal(t, []string{"lock", "ensure table", "applied", "apply 001_baseline.sql"}, f.steps)
}

// migrationRun commits the run's transaction when it succeeds and rolls it back when it fails,
// and its lock is the writer lock on that transaction.
func TestMigrationRun_OneTransactionCommittedOrRolledBack(t *testing.T) {
	var ran []string
	d := recordingDB(&ran, nil)
	require.NoError(t, d.migrationRun(context.Background(), func(m migrationTx) error { return m.Lock(context.Background()) }))
	assert.Equal(t, []string{"begin", "lock", "commit"}, ran)

	ran = nil
	err := d.migrationRun(context.Background(), func(m migrationTx) error {
		if err := m.Lock(context.Background()); err != nil {
			return err
		}
		return errors.New("boom")
	})
	require.ErrorContains(t, err, "boom")
	assert.Equal(t, []string{"begin", "lock", "rollback"}, ran)
}

// A data step runs just before its migration, in the run, and only when that migration is
// pending; a failing step stops the run before its migration, which the run's rollback undoes
// with the rest.
func TestMigrate_ADataStepRunsJustBeforeItsPendingMigration(t *testing.T) {
	steps := func(f *fakeMigrations, err error) map[string]dataStep {
		return map[string]dataStep{
			"001_baseline.sql": func(context.Context, storeq.Querier) error {
				f.steps = append(f.steps, "step 001")
				return nil
			},
			"003_c.sql": func(_ context.Context, q storeq.Querier) error {
				assert.Same(t, f.store, q, "the step runs on the run's transaction")
				f.steps = append(f.steps, "step 003")
				return err
			},
		}
	}
	f := &fakeMigrations{recorded: []string{"001_baseline.sql"}, store: newFakeStore()}
	require.NoError(t, runMigrations(context.Background(), f, migrations("001_baseline.sql", "002_b.sql", "003_c.sql"), steps(f, nil)))
	assert.Equal(t, []string{"lock", "ensure table", "applied", "apply 002_b.sql", "step 003", "apply 003_c.sql"}, f.steps)

	f = &fakeMigrations{recorded: []string{"001_baseline.sql"}, store: newFakeStore()}
	err := runMigrations(context.Background(), f, migrations("001_baseline.sql", "002_b.sql", "003_c.sql"), steps(f, errors.New("boom")))
	require.ErrorContains(t, err, "before 003_c.sql: boom")
	assert.Equal(t, []string{"lock", "ensure table", "applied", "apply 002_b.sql", "step 003"}, f.steps)
}

// Every data step names an embedded migration: a renamed file would silently skip its step.
func TestMigrationDataSteps_NameEmbeddedMigrations(t *testing.T) {
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	names := map[string]bool{}
	for _, m := range embedded {
		names[m.name] = true
	}
	steps := migrationDataSteps(repoSeeds())
	require.Contains(t, steps, ownerKeysMigration)
	for name := range steps {
		assert.True(t, names[name], "data step for %s, which is not embedded", name)
	}
}

// The owner becomes required (ownerKeysMigration) with every row owned first, on a database
// that gets migration 002 and the owner keys in one run (production upgraded from a release
// before owners) and on one rows a release without owners wrote to during the deploy: the seed
// character owns them all before the schema change, which would refuse a row without one.
func TestMigrate_RowsWithoutOwnerAreOwnedBeforeTheOwnerKeys(t *testing.T) {
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	store := newFakeStore()
	require.NoError(t, seedOn(store, repoSeeds()))
	id := store.t.Characters[0]
	store.disown()
	f := &fakeMigrations{recorded: []string{"001_baseline.sql"}, store: store}
	var ownedAtApply map[string]int64
	f.onApply = func(m migration) {
		if m.name == ownerKeysMigration {
			ownedAtApply = store.owners()
		}
	}

	require.NoError(t, store.InTx(context.Background(), func(ctx context.Context) error {
		return runMigrations(ctx, f, embedded, migrationDataSteps(repoSeeds()))
	}))
	assert.Equal(t, store.everyRowOwnedBy(id), ownedAtApply, "every row owned when the owner becomes required")
}
