package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 008 adds tables only: existing data from 001-007 is untouched.
func TestMigrate008_AddsTablesWithoutTouchingExistingData(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql", "006_state_unique.sql", "007_action_log.sql")
	_, err = db.ExecContext(ctx, `INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES ('A', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM characters"))
	assert.Zero(t, scalar(t, db, "SELECT COUNT(*) FROM weapons"))
	assert.Zero(t, scalar(t, db, "SELECT COUNT(*) FROM items"))
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM schema_migrations WHERE name = '008_weapons_items.sql'"))
	// Counters cannot go negative even outside the service.
	_, err = db.ExecContext(ctx, `INSERT INTO items (id, position, name, kind, count, max_count) VALUES ('x', 0, 'X', 'ammo', -1, 1)`)
	require.Error(t, err)
}
