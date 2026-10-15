package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 017 adds the attack_forms table, empty: an existing DB keeps its data and a
// character without a row reads the default form.
func TestMigrate017_AddsEmptyAttackForms(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql",
		"010_crit_table.sql", "011_crit_decimal.sql", "012_correction_payload.sql", "013_saved_buffs.sql",
		"014_correction_note.sql", "015_item_behaviour.sql", "016_initiative.sql")
	_, err = db.ExecContext(ctx, `INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (1, 'T', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM schema_migrations WHERE name = '017_attack_form.sql'"))
	assert.Equal(t, 0, scalar(t, db, "SELECT COUNT(*) FROM attack_forms"))
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM characters"))
	_, err = db.ExecContext(ctx, `INSERT INTO attack_forms (character_id) VALUES (1)`)
	require.NoError(t, err)
	assert.Equal(t, 5, scalar(t, db, "SELECT target_ac FROM attack_forms WHERE character_id = 1"))
	_, err = db.ExecContext(ctx, `UPDATE attack_forms SET target_size = 'huge'`)
	require.Error(t, err, "target size is sm or l")
}
