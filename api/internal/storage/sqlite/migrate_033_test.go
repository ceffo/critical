package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
)

// migratedTo032 is a database with every migration before 033 applied.
func migratedTo032(t *testing.T) *DB {
	t.Helper()
	db := migratedTo031(t)
	content, err := migrationFiles.ReadFile("migrations/032_item_powers.sql")
	require.NoError(t, err)
	require.NoError(t, db.applyMigration(context.Background(), "032_item_powers.sql", string(content)))
	return db
}

// nameKeys are the name keys a query on items or weapons returns.
func nameKeys(t *testing.T, db *DB, q string) []int64 {
	t.Helper()
	rows, err := db.Query(q)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []int64
	for rows.Next() {
		var k int64
		require.NoError(t, rows.Scan(&k))
		out = append(out, k)
	}
	require.NoError(t, rows.Err())
	return out
}

// names are the names of a table's rows by id, read through their keys.
func names(t *testing.T, db *DB, table string) map[string]localized.Text {
	t.Helper()
	texts := storedTexts(t, db)
	rows, err := db.Query(`SELECT id, name FROM ` + table) //nolint:gosec // a fixed table name
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	out := map[string]localized.Text{}
	for rows.Next() {
		var id string
		var key int64
		require.NoError(t, rows.Scan(&id, &key))
		out[id] = texts[key]
	}
	require.NoError(t, rows.Err())
	return out
}

// sheet reads every field of the items and weapons but the name, by id: what
// 033 and the seed reconcile must keep.
func sheet(t *testing.T, db *DB) map[string]string {
	t.Helper()
	rows, err := db.Query(`
		SELECT 'item:' || id, position || '|' || type || '|' || behaviour || '|' || magic_bonus || '|' || ammo_kind || '|' ||
			count || '|' || max_count || '|' || effect_dice || '|' || effect_save || '|' || notes || '|' || powers FROM items
		UNION ALL
		SELECT 'weapon:' || id, position || '|' || damage_sm || '|' || damage_l || '|' || magic_to_hit || '|' ||
			magic_damage || '|' || kind || '|' || two_handed || '|' || ammo_kind FROM weapons`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var id, v string
		require.NoError(t, rows.Scan(&id, &v))
		out[id] = v
	}
	require.NoError(t, rows.Err())
	return out
}

// ADR 0023 step 9, migration 033, on a database holding 032's schema and a
// played inventory: items.name and weapons.name become keys; each name moves
// as the 'en' row of a new key, after the highest key stored (the spells'),
// one key per row; a blank name takes the row's id; every row keeps its id
// and every other field (counts, positions, powers, notes); every key has a
// row and every row is keyed.
func TestMigrate033_Step9_MovesItemAndWeaponNamesKeepingEveryRow(t *testing.T) {
	ctx := context.Background()
	db := migratedTo032(t)
	for _, q := range []string{
		`INSERT INTO spells (id, key, name, class, level) VALUES (3, 'light', 30, 'Wizard', 1)`,
		`INSERT INTO localized_texts (text_id, lang, body) VALUES (30, 'en', 'Light'), (30, 'fr', 'Lumière')`,
		`INSERT INTO items (id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, notes, powers) VALUES
			('arrows_plus1', 0, 'Arrows +1', 'arrow', 'stock', 1, 'arrow', 11, 18, '', '[]'),
			('power_gem_dispel', 5, 'Power Gem', 'gem', 'daily', 0, '', 0, 1, 'mine',
				'[{"id":"thunder_hammer","trigger":{"min_str":18},"effects":{"weapon_buff":5,"multiplier":2,"weapon_kind":"melee"}}]'),
			('blank', 2, '  ', 'gem', 'stock', 0, '', 1, 1, '', '[]')`,
		`INSERT INTO weapons (id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind) VALUES
			('shortbow', 2, 'Short Bow +2', '1d6', '1d6', 2, 2, 'missile', 1, 'arrow'),
			('dagger', 0, 'Dagger +3', '1d4', '1d3', 3, 3, 'melee', 0, '')`,
	} {
		_, err := db.ExecContext(ctx, q)
		require.NoError(t, err)
	}
	before := sheet(t, db)

	require.NoError(t, db.Migrate(ctx))

	assert.Equal(t, "INTEGER", columnTypes(t, db, "items")["name"])
	assert.Equal(t, "INTEGER", columnTypes(t, db, "weapons")["name"])
	assert.Equal(t, before, sheet(t, db), "every row and every other field kept")
	assert.Equal(t, map[string]localized.Text{
		"arrows_plus1":     {"en": "Arrows +1"},
		"power_gem_dispel": {"en": "Power Gem"},
		"blank":            {"en": "blank"},
	}, names(t, db, "items"))
	assert.Equal(t, map[string]localized.Text{
		"shortbow": {"en": "Short Bow +2"},
		"dagger":   {"en": "Dagger +3"},
	}, names(t, db, "weapons"))
	keys := append(nameKeys(t, db, `SELECT name FROM items ORDER BY position, id`),
		nameKeys(t, db, `SELECT name FROM weapons ORDER BY position, id`)...)
	assert.Equal(t, []int64{31, 32, 33, 34, 35}, keys, "after the highest key, items then weapons, in list order")
	assert.Equal(t, localized.Text{"en": "Light", "fr": "Lumière"}, storedTexts(t, db)[30], "spell texts untouched")
	assertTextIntegrity(t, db)
}

// 033 on a fresh database: both tables have the key column, and a seeded
// database has every name keyed (ADR 0023 step 9).
func TestMigrate033_FreshDatabaseKeysEveryName(t *testing.T) {
	db := openMigrated(t)
	require.NoError(t, db.Seed(context.Background(), repoSeeds()))
	assert.Equal(t, "INTEGER", columnTypes(t, db, "items")["name"])
	assertTextIntegrity(t, db)
}

// ADR 0023 step 10: a new database seeded from seeds/ (and the test fixture
// alike) has every item and weapon name in English and French, the Power
// Gem as "Gemme de pouvoir".
func TestSeed_Step10_FreshDatabaseGetsEveryLanguage(t *testing.T) {
	for name, dir := range map[string]string{"campaign seeds": campaignSeeds(), "test fixture": repoSeeds()} {
		t.Run(name, func(t *testing.T) {
			db := openMigrated(t)
			require.NoError(t, db.Seed(context.Background(), dir))
			items, weapons := names(t, db, "items"), names(t, db, "weapons")
			assert.Equal(t, localized.Text{"en": "Power Gem", "fr": "Gemme de pouvoir"}, items["power_gem_dispel"])
			assert.Equal(t, localized.Text{"en": "Long Sword +2", "fr": "Épée longue +2"}, weapons["longsword"])
			for id, n := range items {
				assert.Equal(t, []string{"en", "fr"}, n.Languages(), "item %s", id)
			}
			for id, n := range weapons {
				assert.Equal(t, []string{"en", "fr"}, n.Languages(), "weapon %s", id)
			}
		})
	}
}

// ADR 0023 step 10: on a played database upgraded by 033, the seed gives a
// name it still holds unchanged the languages it lacks (the gem becomes
// "Gemme de pouvoir" in French); a name the player changed is kept, an item
// the player removed is not added back, no count, position or other field
// moves, and a second start writes nothing.
func TestSeed_Step10_ReconcileAddsLanguagesKeepingPlayerNamesAndRows(t *testing.T) {
	ctx := context.Background()
	db := migratedTo032(t)
	for _, q := range []string{
		`INSERT INTO items (id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, notes) VALUES
			('arrows_plus1', 0, 'Arrows +1', 'arrow', 'stock', 1, 'arrow', 3, 18, ''),
			('wand_lightning', 4, 'My wand', 'wand', 'charges', 0, '', 2, 4, ''),
			('power_gem_dispel', 5, 'Power Gem', 'gem', 'daily', 0, '', 0, 1, 'mine'),
			('rope', 6, 'Rope', 'gear', 'stock', 0, '', 1, 1, '')`,
		`INSERT INTO weapons (id, position, name, damage_sm, damage_l, kind) VALUES
			('dagger', 0, 'Dagger +3', '1d4', '1d3', 'melee'),
			('mace', 1, 'Big mace', '1d6', '1d6', 'melee')`,
	} {
		_, err := db.ExecContext(ctx, q)
		require.NoError(t, err)
	}
	require.NoError(t, db.Migrate(ctx))
	// A French name the player typed for the arrows: the seed's English one
	// still matches, the French one does not, so the name is the player's.
	_, err := db.ExecContext(ctx, `INSERT INTO localized_texts (text_id, lang, body)
		SELECT name, 'fr', 'Mes flèches' FROM items WHERE id = 'arrows_plus1'`)
	require.NoError(t, err)
	before := sheet(t, db)

	require.NoError(t, db.Seed(ctx, campaignSeeds()))

	assert.Equal(t, map[string]localized.Text{
		"arrows_plus1":     {"en": "Arrows +1", "fr": "Mes flèches"},
		"wand_lightning":   {"en": "My wand"},
		"power_gem_dispel": {"en": "Power Gem", "fr": "Gemme de pouvoir"},
		"rope":             {"en": "Rope"},
	}, names(t, db, "items"), "removed seed items (fire flasks) stay removed")
	assert.Equal(t, map[string]localized.Text{
		"dagger": {"en": "Dagger +3", "fr": "Dague +3"},
		"mace":   {"en": "Big mace"},
	}, names(t, db, "weapons"))
	assert.Equal(t, before, sheet(t, db), "no count, position or other field moves")
	assertTextIntegrity(t, db)

	texts := storedTexts(t, db)
	require.NoError(t, db.Seed(ctx, campaignSeeds()))
	assert.Equal(t, texts, storedTexts(t, db), "a second start writes nothing")
}

// A seed name must hold a language: an empty map or blank bodies refuse the
// seed and store nothing.
func TestSeed_Step10_RejectsANameWithoutLanguage(t *testing.T) {
	for name, content := range map[string]string{
		"empty map":  `[{"id":"a","name":{},"behaviour":"stock","count":1}]`,
		"blank body": `[{"id":"a","name":{"en":"  "},"behaviour":"stock","count":1}]`,
		"plain text": `[{"id":"a","name":"A","behaviour":"stock","count":1}]`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "items.json"), []byte(content), 0o600))
			weapons, err := os.ReadFile(filepath.Join(repoSeeds(), "weapons.json"))
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "weapons.json"), weapons, 0o600))
			db := openMigrated(t)
			require.Error(t, db.Seed(context.Background(), dir))
			assert.Zero(t, scalar(t, db, "SELECT COUNT(*) FROM items"))
		})
	}
}

// The seed gives each language to a name once: a language added to the seed
// later reaches a filled table on the next start (a name the player changed
// gets it once it reads as the seed's again), but a language the player
// removed from a name (a blank body, step 11) is not put back, even once the
// row was removed and put back under a new key (an undo).
func TestSeed_Step10_ALanguageThePlayerRemovedStaysRemoved(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	others, err := os.ReadDir(repoSeeds())
	require.NoError(t, err)
	for _, f := range others {
		b, err := os.ReadFile(filepath.Join(repoSeeds(), f.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, f.Name()), b, 0o600))
	}
	writeSeeds := func(item, weapon string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "items.json"), []byte(
			`[{"id":"gem","name":`+item+`,"behaviour":"daily","count":1}]`), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "weapons.json"), []byte(
			`[{"id":"axe","name":`+weapon+`,"damage_sm":"1d6","damage_l":"1d4","kind":"melee"}]`), 0o600))
	}
	db := openMigrated(t)
	writeSeeds(`{"en":"Gem"}`, `{"en":"Axe"}`)
	require.NoError(t, db.Seed(ctx, dir))

	setAxe := func(body string) {
		t.Helper()
		_, err := db.ExecContext(ctx, `UPDATE localized_texts SET body = ? WHERE lang = 'en' AND text_id = (SELECT name FROM weapons)`, body)
		require.NoError(t, err)
	}
	// The player renamed the axe: the French seed name is not given...
	setAxe("Big axe")
	writeSeeds(`{"en":"Gem","fr":"Gemme"}`, `{"en":"Axe","fr":"Hache"}`)
	require.NoError(t, db.Seed(ctx, dir))
	require.Equal(t, map[string]localized.Text{"axe": {"en": "Big axe"}}, names(t, db, "weapons"))
	// ...until the name reads as the seed's again (an undo).
	setAxe("Axe")
	require.NoError(t, db.Seed(ctx, dir))
	require.Equal(t, map[string]localized.Text{"gem": {"en": "Gem", "fr": "Gemme"}}, names(t, db, "items"),
		"a language added to the seed later is offered")
	require.Equal(t, map[string]localized.Text{"axe": {"en": "Axe", "fr": "Hache"}}, names(t, db, "weapons"))

	// The player blanks the French names (the DAL deletes the language's row).
	for _, table := range []string{"items", "weapons"} {
		_, err := db.ExecContext(ctx, `DELETE FROM localized_texts WHERE lang = 'fr' AND text_id = (SELECT name FROM `+table+`)`) //nolint:gosec // a fixed table name
		require.NoError(t, err)
	}
	require.NoError(t, db.Seed(ctx, dir))
	assert.Equal(t, map[string]localized.Text{"gem": {"en": "Gem"}}, names(t, db, "items"), "a removed language stays removed")
	assert.Equal(t, map[string]localized.Text{"axe": {"en": "Axe"}}, names(t, db, "weapons"))

	// The gem is removed, then put back under a new key (an undo).
	for _, q := range []string{
		`DELETE FROM localized_texts WHERE text_id = (SELECT name FROM items)`,
		`INSERT INTO localized_texts (text_id, lang, body) SELECT MAX(text_id) + 1, 'en', 'Gem' FROM localized_texts`,
		`UPDATE items SET name = (SELECT MAX(text_id) FROM localized_texts)`,
	} {
		_, err := db.ExecContext(ctx, q)
		require.NoError(t, err)
	}
	texts := storedTexts(t, db)
	require.NoError(t, db.Seed(ctx, dir))
	assert.Equal(t, texts, storedTexts(t, db), "the record follows the id, not the key")
	assertTextIntegrity(t, db)
}
