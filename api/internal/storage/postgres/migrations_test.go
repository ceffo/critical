package postgres

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
}

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

	err := runMigrations(context.Background(), f, migrations("001_baseline.sql", "002_next.sql"))

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

	require.NoError(t, runMigrations(context.Background(), f, embedded), "fresh database")
	require.Len(t, f.recorded, len(embedded))
	f.steps = nil
	require.NoError(t, runMigrations(context.Background(), f, embedded), "fully migrated database")
	assert.Equal(t, []string{"lock", "ensure table", "applied"}, f.steps, "nothing applied again")
}

// ADR 0025 steps 3 and 3.1: the run takes the writer lock before anything else, then applies the
// pending migrations in lexical order, whatever order they are embedded in, skipping the
// recorded ones.
func TestMigrate_Step3_1_LocksThenAppliesThePendingInOrder(t *testing.T) {
	f := &fakeMigrations{recorded: []string{"001_baseline.sql"}}

	require.NoError(t, runMigrations(context.Background(), f,
		migrations("003_c.sql", "001_baseline.sql", "010_d.sql", "002_b.sql")))

	assert.Equal(t, []string{"lock", "ensure table", "applied", "apply 002_b.sql", "apply 003_c.sql", "apply 010_d.sql"}, f.steps)
}

// A migration that fails stops the run with its name: the run's one transaction rolls back
// whole (migrationRun), and the next boot retries it.
func TestMigrate_AFailingMigrationStopsTheRun(t *testing.T) {
	f := &fakeMigrations{applyErr: errors.New("syntax error")}

	err := runMigrations(context.Background(), f, migrations("001_baseline.sql", "002_b.sql"))

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
