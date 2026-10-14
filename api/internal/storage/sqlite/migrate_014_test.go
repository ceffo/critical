package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 014 adds action_corrections.note (critical-qki.38). A value amend stored
// before it (hp_delta + summary on an hp_change) and a void keep every column,
// with a NULL note; the effective delta and the HP total read as before
// (actionlogdal's TestLegacyValueAmend_ReadsAfterMigration014 reads them
// through the DAL).
func TestMigrate014_KeepsLegacyValueAmends(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql",
		"010_crit_table.sql", "011_crit_decimal.sql", "012_correction_payload.sql", "013_saved_buffs.sql")
	_, err = db.ExecContext(ctx, `INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (1, 'T', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_log (id, character_id, round, kind, summary, hp_delta, payload) VALUES
		(1, 1, 0, 'hp_change', 'orc', -8, '{}'),
		(2, 1, 0, 'hp_change', 'potion', 3, '{}'),
		(3, 1, 0, 'hp_change', 'trap', -6, '{}')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_corrections (id, action_id, kind, hp_delta, summary) VALUES
		(1, 1, 'amend', -12, 'orc axe'),
		(2, 3, 'void', NULL, NULL)`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM schema_migrations WHERE name = '014_correction_note.sql'"))
	var delta int
	var summary string
	var note sql.NullString
	require.NoError(t, db.QueryRow(`SELECT hp_delta, summary, note FROM action_corrections WHERE id = 1`).Scan(&delta, &summary, &note))
	assert.Equal(t, -12, delta)
	assert.Equal(t, "orc axe", summary)
	assert.False(t, note.Valid, "legacy rows have no note")
	assert.Equal(t, 2, scalar(t, db, "SELECT COUNT(*) FROM action_corrections"))
}
