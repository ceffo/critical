package sqlite

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 024 adds dice_favourites.position. Favourites saved before keep their
// creation order: each character's rows are numbered 1..n by id, even when
// earlier ones were deleted and the ids have gaps.
func TestMigrate024_BackfillsPositionInCreationOrder(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql",
		"010_crit_table.sql", "011_crit_decimal.sql", "012_correction_payload.sql", "013_saved_buffs.sql",
		"014_correction_note.sql", "015_item_behaviour.sql", "016_initiative.sql", "017_attack_form.sql",
		"018_xp_ledger.sql", "019_attack_form_unknown_ac.sql", "020_skills.sql", "021_character_details.sql",
		"022_dice_favourites.sql", "023_correction_step.sql")
	_, err = db.ExecContext(ctx, `INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES
		(1, 'A', 10, 10, 10, 10, 10, 10, 30, 19), (2, 'B', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO dice_favourites (id, character_id, formula) VALUES
		(2, 1, 'd4'), (3, 2, 'd6'), (5, 1, 'd8'), (9, 1, 'd10')`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))

	pos := func(id int) int {
		return scalar(t, db, "SELECT position FROM dice_favourites WHERE id = "+strconv.Itoa(id))
	}
	assert.Equal(t, 1, pos(2))
	assert.Equal(t, 2, pos(5))
	assert.Equal(t, 3, pos(9))
	assert.Equal(t, 1, pos(3), "another character's numbering starts at 1")
}
