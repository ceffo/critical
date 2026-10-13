package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 010 adds the crit table only: an existing DB keeps its data, the table starts
// empty (the app reads that as the placeholder default) and bad rows are
// refused even outside the service.
func TestMigrate010_AddsCritTableWithoutTouchingExistingData(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql")
	_, err = db.ExecContext(ctx, `INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES ('A', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM characters"))
	assert.Zero(t, scalar(t, db, "SELECT COUNT(*) FROM crit_table_rows"))
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM schema_migrations WHERE name = '010_crit_table.sql'"))
	for _, bad := range []string{
		"(0, 10, 2)",  // from below 1
		"(1, 101, 2)", // to above 100
		"(20, 10, 2)", // backwards
		"(1, 100, 0)", // multiplier below 1
	} {
		_, err = db.ExecContext(ctx, "INSERT INTO crit_table_rows (roll_from, roll_to, multiplier) VALUES "+bad)
		require.Error(t, err, bad)
	}
}
