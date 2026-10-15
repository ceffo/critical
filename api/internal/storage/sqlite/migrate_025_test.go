package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 025 adds action_log.transaction_id (ADR 0018 step 1): each existing entry
// is its own transaction, voided or not, whatever its kind, so each pops
// alone. Every other column and the corrections are untouched.
func TestMigrate025_EachExistingEntryIsItsOwnTransaction(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql",
		"010_crit_table.sql", "011_crit_decimal.sql", "012_correction_payload.sql", "013_saved_buffs.sql",
		"014_correction_note.sql", "015_item_behaviour.sql", "016_initiative.sql", "017_attack_form.sql",
		"018_xp_ledger.sql", "019_attack_form_unknown_ac.sql", "020_skills.sql", "021_character_details.sql",
		"022_dice_favourites.sql", "023_correction_step.sql", "024_dice_favourites_position.sql")
	_, err = db.ExecContext(ctx, `INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (1, 'T', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)
	// A Next Round written before transactions: the advance and its
	// initiative are two entries; then damage, voided.
	_, err = db.ExecContext(ctx, `INSERT INTO action_log (id, character_id, round, kind, summary, hp_delta, payload) VALUES
		(3, 1, 1, 'round_advance', '', 0, '{"fromRound":1,"toRound":2}'),
		(4, 1, 2, 'initiative', '', 0, '{"round":2,"d10":7}'),
		(9, 1, 2, 'hp_change', 'orc', -4, '{}')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_snapshots (action_id, state_key, before_state, after_state) VALUES (3, 'round', '1', '2')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_corrections (id, action_id, kind) VALUES (1, 9, 'void')`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM schema_migrations WHERE name = '025_action_transaction.sql'"))
	assert.Equal(t, 3, scalar(t, db, "SELECT COUNT(*) FROM action_log WHERE transaction_id = id"), "each entry is its own transaction")
	assert.Equal(t, 0, scalar(t, db, "SELECT COUNT(*) FROM action_log WHERE transaction_id IS NULL OR transaction_id <> id"))
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM action_snapshots WHERE action_id = 3"))
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM action_corrections WHERE action_id = 9 AND kind = 'void'"))
	var kind, payload string
	require.NoError(t, db.QueryRow(`SELECT kind, payload FROM action_log WHERE id = 4`).Scan(&kind, &payload))
	assert.Equal(t, "initiative", kind)
	assert.JSONEq(t, `{"round":2,"d10":7}`, payload)
}
