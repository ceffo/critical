package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 011 turns the crit multiplier into a decimal: rows saved under 010 keep their
// ranges and values, decimals are accepted, and below 1 is still refused.
func TestMigrate011_KeepsIntegerRowsAndAcceptsDecimals(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql", "010_crit_table.sql")
	_, err = db.ExecContext(ctx, `INSERT INTO crit_table_rows (roll_from, roll_to, multiplier) VALUES (1, 50, 2), (51, 90, 3), (91, 100, 5)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO crit_table_rows (roll_from, roll_to, multiplier) VALUES (101, 100, 1.5)`)
	require.Error(t, err) // 010 still enforces its ranges

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM schema_migrations WHERE name = '011_crit_decimal.sql'"))
	assert.Equal(t, 3, scalar(t, db, "SELECT COUNT(*) FROM crit_table_rows"))
	for from, want := range map[int]float64{1: 2, 51: 3, 91: 5} {
		var to int
		var mult float64
		require.NoError(t, db.QueryRow("SELECT roll_to, multiplier FROM crit_table_rows WHERE roll_from = ?", from).Scan(&to, &mult))
		assert.InDelta(t, want, mult, 0, "row from %d", from)
		assert.Positive(t, to)
	}
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM pragma_table_info('crit_table_rows') WHERE name = 'multiplier' AND type = 'REAL'"))

	_, err = db.ExecContext(ctx, `DELETE FROM crit_table_rows`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO crit_table_rows (roll_from, roll_to, multiplier) VALUES (1, 100, 1.5)`)
	require.NoError(t, err)
	for _, bad := range []string{"(1, 100, 0.99)", "(1, 100, 0)", "(0, 10, 2)", "(1, 101, 2)", "(20, 10, 2)"} {
		_, err = db.ExecContext(ctx, "INSERT INTO crit_table_rows (roll_from, roll_to, multiplier) VALUES "+bad)
		require.Error(t, err, bad)
	}
}
