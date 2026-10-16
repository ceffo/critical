package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
)

// migratedTo030 is a database with every migration before 031 applied.
func migratedTo030(t *testing.T) *DB {
	t.Helper()
	db := migratedTo027(t)
	for _, name := range []string{"028_buff_spell_id.sql", "029_spell_stats_fr.sql", "030_drop_spell_stats_fr.sql"} {
		content, err := migrationFiles.ReadFile("migrations/" + name)
		require.NoError(t, err)
		require.NoError(t, db.applyMigration(context.Background(), name, string(content)))
	}
	return db
}

// columnTypes lists a table's columns with their declared types.
func columnTypes(t *testing.T, db *DB, table string) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT name, type FROM pragma_table_info(?)`, table)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var n, typ string
		require.NoError(t, rows.Scan(&n, &typ))
		out[n] = typ
	}
	require.NoError(t, rows.Err())
	return out
}

// 031 (ADR 0023) on a database holding 030's schema and data: every text
// moves to localized_texts, one row per language present (the French name
// and description included, none for an absent or empty one), the ten text
// columns hold the keys, name_fr and description_fr are gone, and every row
// keeps its id, key, class, level, source and typed stats; game state is not
// touched and the recorded artifact is forgotten (the next startup imports
// it again).
func TestMigrate031_MovesEveryTextToLocalizedTextsKeepingEveryKey(t *testing.T) {
	ctx := context.Background()
	db := migratedTo030(t)
	for _, q := range []string{
		`INSERT INTO spells (id, key, name, name_fr, class, level, school, sphere, casting_time, range, duration,
			area_of_effect, components, saving_throw, description, description_fr, source, stats) VALUES
			(7, 'magic_missile', 'Magic Missile', 'Projectile magique', 'Wizard', 1, 'Evocation', NULL, '1',
				'60 yds. + 10 yds./level', 'Instantaneous', '1-5 targets', 'V, S', 'None', 'Text', 'Texte', 'PHB', '{"range":{}}'),
			(8, 'detect_magic_priest', 'Detect Magic', NULL, 'Priest', 1, 'Divination', 'Divination', '1 rd.',
				'0', '1 turn', '10 ft. x 80 ft.', 'V, S, M', 'None', 'Priest text', '', 'PHB', NULL),
			(12, NULL, 'Detect Magic', NULL, 'Priest', 1, '', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'Old duplicate', NULL, NULL, NULL)`,
		`INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (1, 'T', 10, 10, 10, 10, 10, 10, 30, 19)`,
		`INSERT INTO spell_slots (character_id, slots) VALUES (1, '[{"level":1,"prepared_spell_id":"magic_missile","prepared_spell_name":"Magic Missile","used":false}]')`,
		`INSERT INTO active_buffs (character_id, buffs) VALUES (1, '[{"id":"a","name":"Detect Magic","spell_id":"detect_magic_priest"}]')`,
		`INSERT INTO catalogue_meta (id, format, version) VALUES (1, 1, '2026-10-15.4')`,
		// The id sequence ran past the highest row (a spell was deleted).
		`UPDATE sqlite_sequence SET seq = 20 WHERE name = 'spells'`,
	} {
		_, err := db.ExecContext(ctx, q)
		require.NoError(t, err)
	}

	require.NoError(t, db.Migrate(ctx))

	cols := columnTypes(t, db, "spells")
	assert.NotContains(t, cols, "name_fr")
	assert.NotContains(t, cols, "description_fr")
	for _, c := range textColumns {
		assert.Equal(t, "INTEGER", cols[c], "column %s holds a text key", c)
	}

	rows := catalogueRows(t, db)
	require.Len(t, rows, 3, "every row kept")
	byID := map[int64]catalogueRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	mm := byID[7]
	assert.Equal(t, "magic_missile", mm.Key.String)
	assert.Equal(t, []any{"Wizard", 1, "PHB", `{"range":{}}`}, []any{mm.Class, mm.Level, mm.Source.String, mm.Stats})
	assert.Equal(t, map[string]localized.Text{
		"name":           {"en": "Magic Missile", "fr": "Projectile magique"},
		"description":    {"en": "Text", "fr": "Texte"},
		"school":         {"en": "Evocation"},
		"casting_time":   {"en": "1"},
		"range":          {"en": "60 yds. + 10 yds./level"},
		"duration":       {"en": "Instantaneous"},
		"area_of_effect": {"en": "1-5 targets"},
		"components":     {"en": "V, S"},
		"saving_throw":   {"en": "None"},
	}, mm.Texts, "no sphere: no key")

	dm := byID[8]
	assert.Equal(t, "detect_magic_priest", dm.Key.String)
	assert.Equal(t, localized.Text{"en": "Detect Magic"}, dm.Texts["name"], "no French name: no fr row")
	assert.Equal(t, localized.Text{"en": "Priest text"}, dm.Texts["description"], "an empty French text: no fr row")
	assert.Equal(t, localized.Text{"en": "Divination"}, dm.Texts["sphere"])

	dup := byID[12]
	assert.False(t, dup.Key.Valid, "the unkeyed duplicate stays unkeyed")
	assert.Equal(t, map[string]localized.Text{
		"name": {"en": "Detect Magic"}, "description": {"en": "Old duplicate"},
	}, dup.Texts, "an empty school and NULL texts get no key")
	assertTextIntegrity(t, db)

	assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM spell_slots WHERE slots LIKE '%"prepared_spell_id":"magic_missile"%'`),
		"game state untouched")
	assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM active_buffs WHERE buffs LIKE '%"spell_id":"detect_magic_priest"%'`))
	assert.Zero(t, scalar(t, db, `SELECT COUNT(*) FROM catalogue_meta`), "the next startup imports the artifact again")
	assert.Equal(t, 1, scalar(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_spells_key'`))

	_, err := db.ExecContext(ctx, `INSERT INTO spells (key, class, level) VALUES ('new', 'Wizard', 1)`)
	require.NoError(t, err)
	assert.Equal(t, 21, scalar(t, db, `SELECT id FROM spells WHERE key = 'new'`), "the id sequence is kept")
	_, err = db.ExecContext(ctx, `INSERT INTO spells (key, class, level) VALUES ('magic_missile', 'Wizard', 1)`)
	require.Error(t, err, "keys stay unique")
}

// The import that follows 031 on such a database finds every row by key,
// keeps its id and text keys, and leaves the stored texts as the artifact
// has them.
func TestMigrate031_ThenImportKeepsRowsAndTextKeys(t *testing.T) {
	ctx := context.Background()
	db := migratedTo030(t)
	cat := fixtureCatalogue(t)
	mm := cat.ByID()["magic_missile"]
	_, err := db.ExecContext(ctx, `INSERT INTO spells (id, key, name, name_fr, class, level, description, source)
		VALUES (3, 'magic_missile', ?, 'Ancien nom', ?, ?, 'Old text', 'PHB')`, mm.Name.Default(), mm.Class, mm.Level)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))
	before := byKey(catalogueRows(t, db))["magic_missile"]

	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	after := byKey(catalogueRows(t, db))["magic_missile"]
	assert.Equal(t, int64(3), after.ID)
	assert.Equal(t, before.TextKeys["name"], after.TextKeys["name"], "the name keeps its key")
	assert.Equal(t, before.TextKeys["description"], after.TextKeys["description"])
	assert.Equal(t, mm.Name, after.Texts["name"], "the artifact's names replace the stored ones")
	assert.Equal(t, mm.Description, after.Texts["description"])
	assertTextIntegrity(t, db)
}

// A fresh database ends with the localized_texts table and no per-language
// column.
func TestMigrate031_FreshDatabaseHasLocalizedTexts(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background()))
	assert.Equal(t, map[string]string{"text_id": "INTEGER", "lang": "TEXT", "body": "TEXT"},
		columnTypes(t, db, "localized_texts"))
	cols := columnTypes(t, db, "spells")
	assert.NotContains(t, cols, "name_fr")
	assert.NotContains(t, cols, "description_fr")
	assert.Equal(t, "INTEGER", cols["name"])
}
