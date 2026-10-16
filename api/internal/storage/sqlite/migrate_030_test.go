package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// frStatColumns are the six French stat columns 029 added and 030 drops.
var frStatColumns = []string{
	"casting_time_fr", "range_fr", "duration_fr", "area_of_effect_fr", "components_fr", "saving_throw_fr",
}

// spellColumns lists the spells table's columns.
func spellColumns(t *testing.T, db *DB) map[string]bool {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info('spells')`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		out[n] = true
	}
	require.NoError(t, rows.Err())
	return out
}

// 030 (ADR 0021 step 8): on a database with 029 applied and French stat
// texts stored, the six *_fr columns are dropped and every other column of
// every row is kept (the English texts, the typed stats, the French name and
// description, the key); the recorded artifact is forgotten, so the next
// import re-types the stats.
func TestMigrate030_DropsFrenchStatColumnsKeepingTheRows(t *testing.T) {
	ctx := context.Background()
	db := migratedTo027(t)
	for _, name := range []string{"028_buff_spell_id.sql", "029_spell_stats_fr.sql"} {
		content, err := migrationFiles.ReadFile("migrations/" + name)
		require.NoError(t, err)
		require.NoError(t, db.applyMigration(ctx, name, string(content)))
	}
	_, err := db.ExecContext(ctx, `INSERT INTO spells (id, key, name, name_fr, class, level, casting_time, range,
		duration, area_of_effect, components, saving_throw, description, description_fr, source, stats,
		casting_time_fr, range_fr, duration_fr, area_of_effect_fr, components_fr, saving_throw_fr)
		VALUES (7, 'magic_missile', 'Magic Missile', 'Projectile magique', 'wizard', 1, '1', '60 yds. + 10 yds./level',
		'Instantaneous', '1-5 targets', 'V, S', 'None', 'Text', 'Texte', 'PHB', '{"range":{}}',
		'1', '60 mètres', 'Instantanée', '1 à 5 cibles', 'V, S', 'Aucun')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO catalogue_meta (id, format, version) VALUES (1, 2, '2026-10-15.4')`)
	require.NoError(t, err)

	content, err := migrationFiles.ReadFile("migrations/030_drop_spell_stats_fr.sql")
	require.NoError(t, err)
	require.NoError(t, db.applyMigration(ctx, "030_drop_spell_stats_fr.sql", string(content)))

	cols := spellColumns(t, db)
	for _, c := range frStatColumns {
		assert.False(t, cols[c], "column %s is dropped", c)
	}
	var key, name, nameFR, rng, descFR, stats string
	require.NoError(t, db.QueryRow(`SELECT key, name, name_fr, range, description_fr, stats FROM spells WHERE id = 7`).
		Scan(&key, &name, &nameFR, &rng, &descFR, &stats))
	assert.Equal(t, []string{"magic_missile", "Magic Missile", "Projectile magique", "60 yds. + 10 yds./level", "Texte", `{"range":{}}`},
		[]string{key, name, nameFR, rng, descFR, stats})
	var recorded int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM catalogue_meta`).Scan(&recorded))
	assert.Zero(t, recorded, "the recorded artifact is forgotten: the next startup imports it again and re-types the stats")
}

// A fresh database ends with no French stat column either.
func TestMigrate030_FreshDatabaseHasNoFrenchStatColumns(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background()))
	cols := spellColumns(t, db)
	for _, c := range frStatColumns {
		assert.False(t, cols[c], "column %s", c)
	}
	assert.True(t, cols["range"])
	assert.True(t, cols["stats"])
}
