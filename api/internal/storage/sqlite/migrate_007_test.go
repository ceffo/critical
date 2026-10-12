package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// applyUpTo applies the named migrations only, as an older deployment would
// have, and records them in schema_migrations.
func applyUpTo(t *testing.T, db *DB, names ...string) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (name TEXT PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT (datetime('now')))`)
	require.NoError(t, err)
	for _, name := range names {
		content, err := migrationFiles.ReadFile("migrations/" + name)
		require.NoError(t, err)
		require.NoError(t, db.execStatements(ctx, name, string(content)))
		_, err = db.ExecContext(ctx, "INSERT INTO schema_migrations (name) VALUES (?)", name)
		require.NoError(t, err)
	}
}

func TestMigrate007_CarriesHpEventsIntoActionLog(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql", "006_state_unique.sql")

	_, err = db.ExecContext(ctx, `INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES ('A', 10, 10, 10, 10, 10, 10, 30, 19), ('B', 10, 10, 10, 10, 10, 10, 20, 19)`)
	require.NoError(t, err)
	// Damage, heal and an old-style "undo:" negation row, across two characters.
	_, err = db.ExecContext(ctx, `INSERT INTO hp_events (character_id, delta, source, created_at) VALUES
		(1, -8, 'orc', '2026-10-01 20:00:00'),
		(1,  3, 'potion', '2026-10-01 20:05:00'),
		(2, -4, 'trap', '2026-10-01 20:06:00'),
		(1, -5, 'goblin', '2026-10-01 20:07:00'),
		(1,  5, 'undo:goblin', '2026-10-01 20:08:00')`)
	require.NoError(t, err)
	sumBefore1 := count(t, db, `SELECT SUM(delta) FROM hp_events WHERE character_id = 1`)
	sumBefore2 := count(t, db, `SELECT SUM(delta) FROM hp_events WHERE character_id = 2`)

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, 5, count(t, db, `SELECT COUNT(*) FROM action_log`))
	assert.Equal(t, 5, count(t, db, `SELECT COUNT(*) FROM action_log WHERE kind = 'hp_change' AND round = 0 AND payload = '{}'`))
	assert.Equal(t, sumBefore1, count(t, db, `SELECT SUM(hp_delta) FROM action_log WHERE character_id = 1`))
	assert.Equal(t, sumBefore2, count(t, db, `SELECT SUM(hp_delta) FROM action_log WHERE character_id = 2`))
	assert.Equal(t, 0, count(t, db, `SELECT COUNT(*) FROM action_corrections`))

	// Order, source and timestamps are preserved.
	rows, err := db.QueryContext(ctx, `SELECT summary, hp_delta, created_at FROM action_log WHERE character_id = 1 ORDER BY id`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	type row struct {
		summary string
		delta   int
		at      string
	}
	var got []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.summary, &r.delta, &r.at))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 4)
	assert.Equal(t, row{"orc", -8, "2026-10-01T20:00:00Z"}, got[0])
	assert.Equal(t, row{"undo:goblin", 5, "2026-10-01T20:08:00Z"}, got[3])

	// Re-executing 007 does not duplicate the carried-over rows.
	content, err := migrationFiles.ReadFile("migrations/007_action_log.sql")
	require.NoError(t, err)
	require.NoError(t, db.execStatements(ctx, "007_action_log.sql", string(content)))
	assert.Equal(t, 5, count(t, db, `SELECT COUNT(*) FROM action_log`))

	// At most one void per entry.
	_, err = db.ExecContext(ctx, `INSERT INTO action_corrections (action_id, kind) VALUES (1, 'void')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_corrections (action_id, kind) VALUES (1, 'void')`)
	require.Error(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_corrections (action_id, kind) VALUES (1, 'delete')`)
	require.Error(t, err)
}

func TestMigrate007_EmptyLedger(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background()))
	assert.Equal(t, 0, count(t, db, `SELECT COUNT(*) FROM action_log`))
	assert.Equal(t, 1, count(t, db, `SELECT COUNT(*) FROM schema_migrations WHERE name = '007_action_log.sql'`))
}
