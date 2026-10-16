package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ADR 0024 step 6.2: a database that records a migration the binary does not
// embed (a rollback to an older build across a migration, or a dev database
// migrated on a newer branch) is refused, naming every unknown migration and
// the snapshot-restore step, and nothing is applied to it.
func TestMigrate_Step6_2_RefusesMigrationsTheBinaryDoesNotEmbed(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// An older schema (001 only) on which a newer build recorded two
	// migrations this binary has never heard of.
	_, err = db.ExecContext(ctx, `CREATE TABLE schema_migrations (name TEXT PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT (datetime('now')))`)
	require.NoError(t, err)
	content, err := migrationFiles.ReadFile("migrations/001_init.sql")
	require.NoError(t, err)
	require.NoError(t, db.execStatements(ctx, "001_init.sql", string(content)))
	_, err = db.ExecContext(ctx, `INSERT INTO schema_migrations (name) VALUES ('001_init.sql'), ('999_newer_b.sql'), ('998_newer_a.sql')`)
	require.NoError(t, err)

	err = db.Migrate(ctx)
	require.ErrorIs(t, err, ErrUnknownMigrations)
	assert.ErrorContains(t, err, "998_newer_a.sql, 999_newer_b.sql")
	assert.ErrorContains(t, err, "docs/adr/0024-deployment.md steps 6.1 and 9.3")
	assert.NotContains(t, err.Error(), "001_init.sql", "an embedded migration is not unknown")

	// Refused before applying anything: 002 onwards did not run.
	assert.Equal(t, 3, count(t, db, `SELECT COUNT(*) FROM schema_migrations`))
	assert.Equal(t, 0, count(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE name = 'action_log'`))
}

// ADR 0024 step 6.2, the other side: a fresh database and one that records
// exactly the embedded migrations (every real database today) still start.
func TestMigrate_Step6_2_AcceptsADatabaseItsBinaryMigrated(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.Migrate(ctx), "fresh database")
	require.NoError(t, db.Migrate(ctx), "fully migrated database")

	all, err := migrationFiles.ReadDir("migrations")
	require.NoError(t, err)
	var sqlFiles int
	for _, e := range all {
		if strings.HasSuffix(e.Name(), ".sql") {
			sqlFiles++
		}
	}
	assert.Equal(t, sqlFiles, count(t, db, `SELECT COUNT(*) FROM schema_migrations`))
}
