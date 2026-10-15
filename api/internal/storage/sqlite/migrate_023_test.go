package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 023 adds action_corrections.step_id. Corrections written before (a
// payload amend on a save, a note, a void) keep every column with a NULL
// step_id, so they stay in effect until their entry is popped
// (actionlogdal's TestCorrectionStep_PoppedStepPutsItsAmendOutOfEffect reads
// them through the DAL).
func TestMigrate023_KeepsCorrectionsWithoutStep(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql",
		"010_crit_table.sql", "011_crit_decimal.sql", "012_correction_payload.sql", "013_saved_buffs.sql",
		"014_correction_note.sql", "015_item_behaviour.sql", "016_initiative.sql", "017_attack_form.sql",
		"018_xp_ledger.sql", "019_attack_form_unknown_ac.sql", "020_skills.sql", "021_character_details.sql",
		"022_dice_favourites.sql")
	_, err = db.ExecContext(ctx, `INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (1, 'T', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_log (id, character_id, round, kind, summary, hp_delta, payload) VALUES
		(1, 1, 0, 'save', '', 0, '{"passed":false}'),
		(2, 1, 0, 'hp_change', 'orc', -4, '{}')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_corrections (id, action_id, kind, payload, note) VALUES
		(1, 1, 'amend', '{"passed":true}', NULL),
		(2, 1, 'amend', NULL, 'lucky'),
		(3, 2, 'void', NULL, NULL)`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM schema_migrations WHERE name = '023_correction_step.sql'"))
	assert.Equal(t, 3, scalar(t, db, "SELECT COUNT(*) FROM action_corrections WHERE step_id IS NULL"))
	var payload string
	require.NoError(t, db.QueryRow(`SELECT payload FROM action_corrections WHERE id = 1`).Scan(&payload))
	assert.JSONEq(t, `{"passed":true}`, payload)
}
