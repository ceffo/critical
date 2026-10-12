package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func count(t *testing.T, db *DB, q string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(), q, args...).Scan(&n))
	return n
}

func TestMigrate006_DedupesStateTables(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Apply 001..005 only, as an older deployment would have.
	_, err = db.ExecContext(ctx, `CREATE TABLE schema_migrations (name TEXT PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT (datetime('now')))`)
	require.NoError(t, err)
	for _, name := range []string{"001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql"} {
		content, err := migrationFiles.ReadFile("migrations/" + name)
		require.NoError(t, err)
		require.NoError(t, db.execStatements(ctx, name, string(content)))
		_, err = db.ExecContext(ctx, "INSERT INTO schema_migrations (name) VALUES (?)", name)
		require.NoError(t, err)
	}

	_, err = db.ExecContext(ctx, `INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES ('A', 10, 10, 10, 10, 10, 10, 30, 19), ('B', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)
	for _, s := range []string{
		`INSERT INTO active_buffs (character_id, buffs) VALUES (1, '["old"]'), (1, '["new"]'), (2, '["b"]')`,
		`INSERT INTO spell_slots (character_id, slots) VALUES (1, '{"old":1}'), (1, '{"new":1}')`,
		`INSERT INTO round_state (character_id, round_number) VALUES (1, 3), (1, 7), (2, 1)`,
	} {
		_, err = db.ExecContext(ctx, s)
		require.NoError(t, err)
	}

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 2, count(t, db, `SELECT COUNT(*) FROM active_buffs`))
	assert.Equal(t, 1, count(t, db, `SELECT COUNT(*) FROM spell_slots`))
	assert.Equal(t, 2, count(t, db, `SELECT COUNT(*) FROM round_state`))
	var buffs, slots string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT buffs FROM active_buffs WHERE character_id = 1`).Scan(&buffs))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT slots FROM spell_slots WHERE character_id = 1`).Scan(&slots))
	assert.Equal(t, `["new"]`, buffs)
	assert.Equal(t, `{"new":1}`, slots)
	assert.Equal(t, 7, count(t, db, `SELECT round_number FROM round_state WHERE character_id = 1`))

	// Upserts keyed on character_id now work.
	_, err = db.ExecContext(ctx, `INSERT INTO round_state (character_id, round_number) VALUES (1, 8) ON CONFLICT(character_id) DO UPDATE SET round_number = excluded.round_number`)
	require.NoError(t, err)
	assert.Equal(t, 8, count(t, db, `SELECT round_number FROM round_state WHERE character_id = 1`))

	// Running again is a no-op, and re-executing 006 directly is safe.
	require.NoError(t, db.Migrate(ctx))
	content, err := migrationFiles.ReadFile("migrations/006_state_unique.sql")
	require.NoError(t, err)
	require.NoError(t, db.execStatements(ctx, "006_state_unique.sql", string(content)))
	assert.Equal(t, 8, count(t, db, `SELECT round_number FROM round_state WHERE character_id = 1`))
	assert.Equal(t, 6, count(t, db, `SELECT COUNT(*) FROM schema_migrations`))
}
