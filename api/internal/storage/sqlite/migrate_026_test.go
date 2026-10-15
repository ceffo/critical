package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 026 rebuilds action_corrections so a redo correction can be written and an
// entry popped again after a redo can get a second void (ADR 0019 step 1).
// Corrections written before keep their ids and every column, and new rows
// get ids above them.
func TestMigrate026_KeepsCorrectionsAndAllowsRedo(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql",
		"010_crit_table.sql", "011_crit_decimal.sql", "012_correction_payload.sql", "013_saved_buffs.sql",
		"014_correction_note.sql", "015_item_behaviour.sql", "016_initiative.sql", "017_attack_form.sql",
		"018_xp_ledger.sql", "019_attack_form_unknown_ac.sql", "020_skills.sql", "021_character_details.sql",
		"022_dice_favourites.sql", "023_correction_step.sql", "024_dice_favourites_position.sql",
		"025_action_transaction.sql")
	_, err = db.ExecContext(ctx, `INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (1, 'T', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_log (id, character_id, round, kind, summary, hp_delta, payload, transaction_id) VALUES
		(1, 1, 0, 'save', '', 0, '{"passed":false}', 1),
		(2, 1, 0, 'correction', '', 0, '{}', 2),
		(3, 1, 0, 'hp_change', 'orc', -4, '{}', 3)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_corrections (id, action_id, kind, hp_delta, summary, payload, note, step_id, created_at) VALUES
		(4, 1, 'amend', NULL, NULL, '{"passed":true}', NULL, 2, '2026-10-01 10:00:00'),
		(5, 1, 'amend', NULL, NULL, NULL, 'lucky', NULL, '2026-10-01 10:01:00'),
		(7, 3, 'void', NULL, NULL, NULL, NULL, NULL, '2026-10-01 10:02:00')`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM schema_migrations WHERE name = '026_redo_correction.sql'"))
	assert.Equal(t, 3, scalar(t, db, "SELECT COUNT(*) FROM action_corrections"))
	assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM action_corrections
		WHERE id = 4 AND action_id = 1 AND kind = 'amend' AND payload = '{"passed":true}' AND step_id = 2 AND created_at = '2026-10-01 10:00:00'`))
	assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM action_corrections WHERE id = 5 AND note = 'lucky' AND step_id IS NULL`))
	assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM action_corrections WHERE id = 7 AND action_id = 3 AND kind = 'void'`))

	// The entry popped before can be redone, then popped again.
	_, err = db.ExecContext(ctx, `INSERT INTO action_corrections (action_id, kind) VALUES (3, 'redo')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_corrections (action_id, kind) VALUES (3, 'void')`)
	require.NoError(t, err, "a second void after a redo")
	assert.Equal(t, 8, scalar(t, db, `SELECT MIN(id) FROM action_corrections WHERE id > 7`), "new ids continue after the kept ones")
	_, err = db.ExecContext(ctx, `INSERT INTO action_corrections (action_id, kind) VALUES (3, 'erase')`)
	require.Error(t, err, "the kind is still checked")
	assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_action_corrections_action'`))
}
