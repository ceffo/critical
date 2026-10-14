package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 013 adds the saved_buffs table, empty. Active buffs written before the note
// existed keep their JSON untouched (a missing note reads as empty in buffdal).
func TestMigrate013_AddsEmptySavedBuffsAndKeepsActiveBuffs(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql",
		"010_crit_table.sql", "011_crit_decimal.sql")
	_, err = db.ExecContext(ctx, `INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (1, 'T', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)
	old := `[{"id":"a","name":"Haste","charges":0,"duration":3,"effects":{}}]`
	_, err = db.ExecContext(ctx, `INSERT INTO active_buffs (character_id, buffs) VALUES (1, ?)`, old)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM schema_migrations WHERE name = '013_saved_buffs.sql'"))
	assert.Equal(t, 0, scalar(t, db, "SELECT COUNT(*) FROM saved_buffs"))
	var got string
	require.NoError(t, db.QueryRow("SELECT buffs FROM active_buffs WHERE character_id = 1").Scan(&got))
	assert.JSONEq(t, old, got)
	_, err = db.ExecContext(ctx, `INSERT INTO saved_buffs (character_id, name) VALUES (1, 'Form')`)
	require.NoError(t, err)
}
