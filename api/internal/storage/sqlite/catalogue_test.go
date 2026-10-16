package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

func fixtureCatalogue(t *testing.T) catalogue.Catalogue {
	t.Helper()
	c, err := catalogueadpt.Load(filepath.Join("..", "..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	return c
}

// withVersion copies c with a new version and the spells edited by edit.
func withVersion(c catalogue.Catalogue, version string, edit func([]catalogue.Spell) []catalogue.Spell) catalogue.Catalogue {
	spells := append([]catalogue.Spell(nil), c.Spells...)
	if edit != nil {
		spells = edit(spells)
	}
	return catalogue.Catalogue{Format: c.Format, Version: version, Spells: spells}
}

func without(id string) func([]catalogue.Spell) []catalogue.Spell {
	return func(in []catalogue.Spell) []catalogue.Spell {
		out := in[:0:0]
		for _, s := range in {
			if s.ID != id {
				out = append(out, s)
			}
		}
		return out
	}
}

// catalogueRow is a spells row with its texts read through their keys.
type catalogueRow struct {
	ID           int64
	Key, Source  sql.NullString
	Class, Stats string
	Level        int
	// Texts are the row's texts by column; a NULL key has no entry.
	Texts map[string]localized.Text
	// TextKeys are the row's text keys by column (NULL: no entry).
	TextKeys map[string]int64
}

// textColumns are the spells columns that key a localized text.
var textColumns = []string{
	"name", "description", "school", "sphere", "casting_time", "range", "duration", "area_of_effect",
	"components", "saving_throw",
}

// storedTexts reads every localized text by key.
func storedTexts(t *testing.T, db *DB) map[int64]localized.Text {
	t.Helper()
	rows, err := db.Query(`SELECT text_id, lang, body FROM localized_texts`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	out := map[int64]localized.Text{}
	for rows.Next() {
		var id int64
		var lang, body string
		require.NoError(t, rows.Scan(&id, &lang, &body))
		if out[id] == nil {
			out[id] = localized.Text{}
		}
		out[id][lang] = body
	}
	require.NoError(t, rows.Err())
	return out
}

func catalogueRows(t *testing.T, db *DB) []catalogueRow {
	t.Helper()
	texts := storedTexts(t, db)
	rows, err := db.Query(`SELECT id, key, class, level, source, COALESCE(stats, ''), ` +
		strings.Join(textColumns, ", ") + ` FROM spells ORDER BY id`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []catalogueRow
	for rows.Next() {
		r := catalogueRow{Texts: map[string]localized.Text{}, TextKeys: map[string]int64{}}
		keys := make([]sql.NullInt64, len(textColumns))
		dest := []any{&r.ID, &r.Key, &r.Class, &r.Level, &r.Source, &r.Stats}
		for i := range keys {
			dest = append(dest, &keys[i])
		}
		require.NoError(t, rows.Scan(dest...))
		for i, c := range textColumns {
			if keys[i].Valid {
				r.Texts[c] = texts[keys[i].Int64]
				r.TextKeys[c] = keys[i].Int64
			}
		}
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// assertTextIntegrity checks ADR 0023's storage invariants: every text key a
// spells column holds has at least one row, no row is unreachable from a
// column, and no body is empty.
func assertTextIntegrity(t *testing.T, db *DB) {
	t.Helper()
	used := map[int64]bool{}
	for _, r := range catalogueRows(t, db) {
		for c, k := range r.TextKeys {
			used[k] = true
			assert.NotEmpty(t, r.Texts[c], "row %d %s: key %d has no row", r.ID, c, k)
		}
	}
	for _, q := range []string{`SELECT name FROM items`, `SELECT name FROM weapons`} {
		for _, k := range nameKeys(t, db, q) {
			used[k] = true
		}
	}
	for id := range storedTexts(t, db) {
		assert.True(t, used[id], "text %d is keyed by no column", id)
	}
	assert.Zero(t, scalar(t, db, `SELECT COUNT(*) FROM localized_texts WHERE body = ''`))
}

func byKey(rows []catalogueRow) map[string]catalogueRow {
	out := map[string]catalogueRow{}
	for _, r := range rows {
		if r.Key.Valid {
			out[r.Key.String] = r
		}
	}
	return out
}

func recordedVersion(t *testing.T, db *DB) string {
	t.Helper()
	var v string
	err := db.QueryRow(`SELECT version FROM catalogue_meta WHERE id = 1`).Scan(&v)
	if err == sql.ErrNoRows {
		return ""
	}
	require.NoError(t, err)
	return v
}

// A fresh database gets every artifact spell under its id, each text as one
// localized_texts row per language present (ADR 0023: no "fr" row for a
// spell without French, no key for a text in no language), the source and
// the typed stats (step 5.6), and the version is recorded (step 5.5).
func TestImportCatalogue_FreshDatabase(t *testing.T) {
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	require.NoError(t, db.ImportCatalogue(context.Background(), cat, nil))

	rows := catalogueRows(t, db)
	keyed := byKey(rows)
	assert.Len(t, rows, len(cat.Spells))
	assert.Len(t, keyed, len(cat.Spells))
	for _, s := range cat.Spells {
		r, ok := keyed[s.ID]
		if !assert.True(t, ok, s.ID) {
			continue
		}
		assert.Equal(t, s.Class, r.Class, s.ID)
		assert.Equal(t, s.Level, r.Level, s.ID)
		assert.Equal(t, s.Source, r.Source.String, s.ID)
		want := map[string]localized.Text{}
		for c, txt := range map[string]localized.Text{
			"name": s.Name, "description": s.Description, "school": s.School, "sphere": s.Sphere,
			"casting_time": s.CastingTime, "range": s.Range, "duration": s.Duration,
			"area_of_effect": s.AreaOfEffect, "components": s.Components, "saving_throw": s.SavingThrow,
		} {
			if len(txt) > 0 {
				want[c] = txt
			}
		}
		assert.Equal(t, want, r.Texts, "%s: every text, in exactly its languages", s.ID)
	}
	mm, armor := keyed["magic_missile"], keyed["armor"]
	assert.Equal(t, "Projectile magique", mm.Texts["name"]["fr"])
	assert.Equal(t, localized.Text{"en": "Armor"}, armor.Texts["name"], "no French name: no fr row")
	assert.NotContains(t, armor.Texts, "sphere", "no sphere: a NULL key")
	assert.Equal(t, cat.Version, recordedVersion(t, db))
	assertTextIntegrity(t, db)
}

func TestImportCatalogue_Step6_TypedStatsParsedAtImport(t *testing.T) {
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	require.NoError(t, db.ImportCatalogue(context.Background(), cat, nil))
	r := byKey(catalogueRows(t, db))["magic_missile"]
	var stats spellstats.Stats
	require.NoError(t, json.Unmarshal([]byte(r.Stats), &stats))
	assert.Equal(t, spellstats.Parse(spellstats.Raw{
		CastingTime: "1", Range: "60 yds. + 10 yds./level", Duration: "Instantaneous",
		AreaOfEffect: "1-5 targets", Components: "V, S", SavingThrow: "None",
	}), stats)
}

// Step 5.6 on a start that imports nothing: the stored typed stats are what
// an older parser made of the stored texts. The start re-types them from
// those texts, writes no other column and keeps the recorded version, so a
// parser change needs no migration and no new artifact version.
func TestImportCatalogue_Step6_AStartReTypesStatsAParserChangeMadeStale(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	fresh := byKey(catalogueRows(t, db))
	_, err := db.Exec(`UPDATE spells SET stats = '{"area":{"raw":"older parser","kind":"special"}}' WHERE key IN ('fireball', 'magic_missile')`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE spells SET stats = NULL WHERE key = 'sleep'`)
	require.NoError(t, err)

	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	after := byKey(catalogueRows(t, db))
	assert.Equal(t, fresh, after, "every stale row re-typed, nothing else written")
	var stats spellstats.Stats
	require.NoError(t, json.Unmarshal([]byte(after["magic_missile"].Stats), &stats))
	assert.Equal(t, "targets", stats.Area.Kind, "parsed again from the stored area text")
	assert.Equal(t, cat.Version, recordedVersion(t, db))
}

// Step 5.3: the recorded version equals the artifact's: nothing is written,
// so a row edited behind the importer's back stays edited.
func TestImportCatalogue_Step3_SameVersionWritesNothing(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	_, err := db.Exec(`UPDATE localized_texts SET body = 'edited'
		WHERE text_id = (SELECT description FROM spells WHERE key = 'fireball') AND lang = 'en'`)
	require.NoError(t, err)
	before := catalogueRows(t, db)

	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	assert.Equal(t, before, catalogueRows(t, db))
	assert.Equal(t, "edited", byKey(catalogueRows(t, db))["fireball"].Texts["description"].Default())
}

// Step 5.4.2: a new version overwrites every catalogue field by key and
// inserts new spells; row ids stay. Equality only: going back to the older
// version imports it again.
func TestImportCatalogue_Step4_NewVersionUpserts(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	first := byKey(catalogueRows(t, db))["fireball"]

	next := withVersion(cat, "1970-01-02", func(in []catalogue.Spell) []catalogue.Spell {
		for i := range in {
			if in[i].ID == "fireball" { // new maps: the slice copy shares the old ones
				in[i].Description = localized.Of("Fixture text, corrected.")
				in[i].Name = localized.Of(in[i].Name.Default())
				in[i].Range = localized.Of("20 yds.")
			}
		}
		return append(in, catalogue.Spell{
			ID: "new_spell", Class: "Priest", Level: 2, School: localized.Of("Alteration"), Sphere: localized.Of("All"),
			CastingTime: localized.Of("1"), Range: localized.Of("0"), Duration: localized.Of("1 rd."),
			AreaOfEffect: localized.Of("The caster"), Components: localized.Of("V"), SavingThrow: localized.Of("None"),
			Name: localized.Text{"en": "New Spell", "fr": "Nouveau sort"}, Description: localized.Of("Fixture text."),
			Source: "TOM",
		})
	})
	require.NoError(t, db.ImportCatalogue(ctx, next, nil))
	rows := byKey(catalogueRows(t, db))
	assert.Len(t, rows, len(cat.Spells)+1)
	fb := rows["fireball"]
	assert.Equal(t, first.ID, fb.ID, "the row keeps its id")
	assert.Equal(t, first.TextKeys, fb.TextKeys, "and its text keys")
	assert.Equal(t, localized.Text{"en": "Fixture text, corrected."}, fb.Texts["description"])
	assert.Equal(t, localized.Text{"en": "Fireball"}, fb.Texts["name"], "a French name the artifact dropped is cleared")
	assert.Zero(t, scalar(t, db, `SELECT COUNT(*) FROM localized_texts
		WHERE text_id = (SELECT name FROM spells WHERE key = 'fireball') AND lang = 'fr'`), "its row is deleted")
	assert.Contains(t, fb.Stats, "20 yds.", "stats parsed again")
	assert.Equal(t, "Nouveau sort", rows["new_spell"].Texts["name"]["fr"])
	assert.Equal(t, localized.Text{"en": "All"}, rows["new_spell"].Texts["sphere"])
	assert.Equal(t, "1970-01-02", recordedVersion(t, db))
	assertTextIntegrity(t, db)

	require.NoError(t, db.ImportCatalogue(ctx, cat, nil), "an older version is an import like any other")
	assert.Equal(t, cat.ByID()["fireball"].Description, byKey(catalogueRows(t, db))["fireball"].Texts["description"])
	assert.NotContains(t, byKey(catalogueRows(t, db)), "new_spell", "unreferenced, so deleted")
	assertTextIntegrity(t, db)
}

// ADR 0023 at re-import: a text the artifact drops whole (a sphere) loses
// its key and every row; one it adds gets a new key; the texts of a deleted
// spell go with it.
func TestImportCatalogue_ReimportDropsAndAddsWholeTexts(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	priest := byKey(catalogueRows(t, db))["detect_magic_priest"]
	require.Contains(t, priest.TextKeys, "sphere")

	next := withVersion(cat, "1970-01-02", func(in []catalogue.Spell) []catalogue.Spell {
		for i := range in {
			switch in[i].ID {
			case "detect_magic_priest":
				in[i].Sphere = nil
			case "armor":
				in[i].Sphere = localized.Of("Protection")
			}
		}
		return without("slow")(in)
	})
	require.NoError(t, db.ImportCatalogue(ctx, next, nil))
	rows := byKey(catalogueRows(t, db))
	assert.NotContains(t, rows["detect_magic_priest"].TextKeys, "sphere")
	assert.Zero(t, scalar(t, db, `SELECT COUNT(*) FROM localized_texts WHERE text_id = `+
		strconv.FormatInt(priest.TextKeys["sphere"], 10)), "the dropped text's rows are deleted")
	assert.Equal(t, localized.Text{"en": "Protection"}, rows["armor"].Texts["sphere"])
	assert.NotContains(t, rows, "slow")
	assertTextIntegrity(t, db)
}

// insertLegacyRow writes a spells row as the retired seed loader did, on a
// database before 031 (texts in the columns): no key, no French texts.
func insertLegacyRow(t *testing.T, db *DB, id int64, s catalogue.Spell, description string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO spells (id, name, class, level, school, sphere, casting_time, range, duration,
		area_of_effect, components, saving_throw, description) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, s.Name.Default(), s.Class, s.Level, s.School.Default(), s.Sphere.Default(), s.CastingTime.Default(),
		s.Range.Default(), s.Duration.Default(), s.AreaOfEffect.Default(), s.Components.Default(),
		s.SavingThrow.Default(), description)
	require.NoError(t, err)
}

// insertOldRow is such a row as migration 031 leaves it: its English texts
// as localized_texts rows under the keys 031 gives them.
func insertOldRow(t *testing.T, db *DB, id int64, s catalogue.Spell, description string) {
	t.Helper()
	texts := []string{
		s.Name.Default(), description, s.School.Default(), s.Sphere.Default(), s.CastingTime.Default(),
		s.Range.Default(), s.Duration.Default(), s.AreaOfEffect.Default(), s.Components.Default(), s.SavingThrow.Default(),
	}
	keys := make([]any, len(texts))
	for i, body := range texts {
		if body == "" {
			continue
		}
		k := (id-1)*10 + int64(i) + 1
		keys[i] = k
		_, err := db.Exec(`INSERT INTO localized_texts (text_id, lang, body) VALUES (?, 'en', ?)`, k, body)
		require.NoError(t, err)
	}
	_, err := db.Exec(`INSERT INTO spells (id, class, level, `+strings.Join(textColumns, ", ")+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, append([]any{id, s.Class, s.Level}, keys...)...)
	require.NoError(t, err)
}

// Step 5.4.1: a database seeded before the catalogue. Its rows are keyed by
// spellsvc.Keys and matched by (name, class, level); only the oldest row of a
// duplicated triple takes the key, the younger one keeps NULL and is left
// alone; no spell is inserted twice and row ids stay.
func TestImportCatalogue_Step4_1_BackfillsKeysOfAnExistingDatabase(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	for i, s := range cat.Spells {
		insertOldRow(t, db, int64(100+i), s, "old scraped text")
	}
	dup := cat.ByID()["fireball"]
	insertOldRow(t, db, 900, dup, "younger duplicate")

	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	rows := catalogueRows(t, db)
	assert.Len(t, rows, len(cat.Spells)+1, "nothing inserted twice")
	keyed := byKey(rows)
	for i, s := range cat.Spells {
		assert.Equal(t, int64(100+i), keyed[s.ID].ID, "%s keeps its row", s.ID)
		assert.Equal(t, s.Description, keyed[s.ID].Texts["description"], "%s overwritten by the artifact", s.ID)
	}
	for _, r := range rows {
		if r.ID == 900 {
			assert.False(t, r.Key.Valid, "the younger duplicate keeps a NULL key")
			assert.Equal(t, localized.Text{"en": "younger duplicate"}, r.Texts["description"], "and is never updated")
		}
	}
}

// Step 5.4.1 verification: rows outside the set the ids were published from
// shift the Keys suffixes. Here a homebrew Wizard "Fortify" takes the slug
// "fortify" and pushes the Priest spell to "fortify_priest", while the
// artifact's "fortify" is the Priest spell. A key whose row is not the
// artifact's spell is refused, listing both triples, and nothing changes.
func TestImportCatalogue_Step4_1_RefusesKeysThatDoNotMatchTheArtifact(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cat := withVersion(fixtureCatalogue(t), "1970-01-03", func(in []catalogue.Spell) []catalogue.Spell {
		return append(in, catalogue.Spell{
			ID: "fortify", Class: "Priest", Level: 3, School: localized.Of("Alteration"), Sphere: localized.Of("War"),
			CastingTime: localized.Of("1"), Range: localized.Of("0"), Duration: localized.Of("1 rd."),
			AreaOfEffect: localized.Of("The caster"), Components: localized.Of("V"), SavingThrow: localized.Of("None"),
			Name: localized.Of("Fortify"), Description: localized.Of("Fixture text."), Source: "TOM",
		})
	})
	priest := cat.ByID()["fortify"]
	insertOldRow(t, db, 1, priest, "old")
	homebrew := priest
	homebrew.Class, homebrew.Level = "Wizard", 1
	insertOldRow(t, db, 2, homebrew, "homebrew")
	before := catalogueRows(t, db)

	err := db.ImportCatalogue(ctx, cat, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fortify: database row 2 is (\"Fortify\", Wizard, 1), the artifact has (\"Fortify\", Priest, 3)")
	assert.Equal(t, before, catalogueRows(t, db), "nothing keyed, nothing written")
	assert.Empty(t, recordedVersion(t, db))
}

// Step 5.4.3: a spell the new artifact drops is deleted when nothing refers
// to it.
func TestImportCatalogue_Step4_3_DeletesAnUnreferencedRemovedSpell(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	require.NoError(t, db.ImportCatalogue(ctx, cat, []catalogue.Ref{{ID: "fireball", Where: "spellbook seed"}}))
	require.NoError(t, db.ImportCatalogue(ctx, withVersion(cat, "1970-01-02", without("slow")),
		[]catalogue.Ref{{ID: "fireball", Where: "spellbook seed"}}))
	keyed := byKey(catalogueRows(t, db))
	assert.NotContains(t, keyed, "slow")
	assert.Len(t, keyed, len(cat.Spells)-1)
}

// plantState writes game state referring to spells in every place step
// 5.4.3 scans.
func plantState(t *testing.T, db *DB) {
	t.Helper()
	for _, q := range []string{
		`INSERT INTO characters (id, name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (1, 'T', 10, 10, 10, 10, 10, 10, 30, 19)`,
		`INSERT INTO spell_slots (character_id, slots) VALUES (1, '[{"level":1,"prepared_spell_id":"sleep","prepared_spell_name":"Sleep","used":false},{"level":1,"prepared_spell_name":"","used":false}]')`,
		`INSERT INTO action_log (id, character_id, round, kind, summary, hp_delta, payload, transaction_id) VALUES
			(1, 1, 0, 'cast', '', 0, '{"action":"cast","spellId":"magic_missile","spell":"Magic Missile","casterLevel":5}', 1),
			(2, 1, 0, 'exposure', '', 0, '{"spellId":"cloudkill","spell":"Cloudkill","casterLevel":9,"round":1,"onSelf":true}', 2),
			(3, 1, 0, 'spell_prep', '', 0, '{"spells":[{"id":"armor","name":"Armor","count":1}]}', 3),
			(4, 1, 0, 'hp_change', 'orc', -3, '{}', 4),
			(5, 1, 0, 'cast', '', 0, '{"action":"cast","spellId":"haste","spell":"Haste","casterLevel":5}', 5),
			(6, 1, 0, 'buff_change', '', 0, '{"action":"dismiss","buff":"Detect Magic","spellId":"detect_magic"}', 6),
			(7, 1, 0, 'round_advance', '', 0, '{"action":"next_round","expired":["Wall of Ice","Gem"],"expiredSpellIds":["wall_of_ice",""]}', 7),
			(8, 1, 0, 'time_passed', '', 0, '{"action":"pass_time","expired":["Chromatic Orb"],"expiredSpellIds":["chromatic_orb"]}', 8),
			(9, 1, 0, 'hp_change', 'x', -1, '{"spellId":"not_a_spell_entry"}', 9)`,
		`INSERT INTO active_buffs (character_id, buffs) VALUES (1, '[{"id":"a","name":"Stoneskin","spell_id":"stoneskin"},{"id":"b","name":"Gem"}]')`,
		`INSERT INTO action_snapshots (action_id, state_key, before_state, after_state) VALUES
			(3, 'spell_slots', '[{"Level":1,"SpellID":"identify","PreparedSpellName":"Identify","Used":false,"Extra":false}]',
				'[{"Level":1,"SpellID":"light","PreparedSpellName":"Light","Used":false,"Extra":false}]'),
			(4, 'buffs', '[{"SpellID":"not_a_slot","spell_id":"improved_strength"}]', '[{"spell_id":"mirror_image"}]'),
			(5, 'round', '[{"spell_id":"not_a_buff_list"}]', '[]')`,
		`INSERT INTO action_corrections (action_id, kind, payload) VALUES (5, 'amend', '{"spellId":"enlarge","casterLevel":5}')`,
	} {
		_, err := db.Exec(q)
		require.NoError(t, err)
	}
}

// Every place game state holds a spell id is scanned; a snapshot of another
// state key and entries of other kinds are not.
func TestStateSpellRefs_ScansEveryPlace(t *testing.T) {
	db := openMigrated(t)
	plantState(t, db)
	refs, err := StateSpellRefs(context.Background(), db)
	require.NoError(t, err)
	got := map[string]string{}
	for _, r := range refs {
		got[r.ID] = r.Where
	}
	assert.Equal(t, map[string]string{
		"sleep":         "spell_slots of character 1",
		"identify":      "action_snapshots 1",
		"light":         "action_snapshots 1",
		"magic_missile": "action_log 1",
		"cloudkill":     "action_log 2",
		"armor":         "action_log 3",
		"haste":         "action_log 5",
		"enlarge":       "action_corrections 1",
		// Buffs from a spell (ADR 0004 step 17).
		"stoneskin":         "active_buffs of character 1",
		"improved_strength": "action_snapshots 2",
		"mirror_image":      "action_snapshots 2",
		"detect_magic":      "action_log 6",
		"wall_of_ice":       "action_log 7",
		"chromatic_orb":     "action_log 8",
	}, got)
}

func TestStateSpellRefs_UnreadableJSONIsAnError(t *testing.T) {
	db := openMigrated(t)
	plantState(t, db)
	_, err := db.Exec(`UPDATE spell_slots SET slots = '{not json' WHERE character_id = 1`)
	require.NoError(t, err)
	_, err = StateSpellRefs(context.Background(), db)
	require.ErrorContains(t, err, "spell_slots of character 1")
}

// Step 5.4.3: an artifact that removes referenced spells is refused, listing
// every such key and where it is used (seeds, slots, snapshots, log entries,
// corrections), and the import rolls back whole.
func TestImportCatalogue_Step4_3_RefusesRemovingReferencedSpells(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	seeds := []catalogue.Ref{{ID: "strength", Where: "spellbook seed"}, {ID: "strength", Where: "spell effects seed"}}
	require.NoError(t, db.ImportCatalogue(ctx, cat, seeds))
	plantState(t, db)
	before := catalogueRows(t, db)

	removed := []string{"strength", "sleep", "identify", "light", "magic_missile", "cloudkill", "armor", "enlarge"}
	next := withVersion(cat, "1970-01-02", func(in []catalogue.Spell) []catalogue.Spell {
		for _, id := range removed {
			in = without(id)(in)
		}
		for i := range in {
			in[i].Description = localized.Of("Fixture text, changed.")
		}
		return in
	})
	err := db.ImportCatalogue(ctx, next, seeds)
	require.Error(t, err)
	for _, want := range []string{
		"strength (spellbook seed, spell effects seed)", "sleep (spell_slots of character 1)",
		"identify (action_snapshots 1)", "light (action_snapshots 1)", "magic_missile (action_log 1)",
		"cloudkill (action_log 2)", "armor (action_log 3)", "enlarge (action_corrections 1)",
	} {
		assert.Contains(t, err.Error(), want)
	}
	assert.Contains(t, err.Error(), "the artifact (version 1970-01-02)")
	assert.Contains(t, err.Error(), "This database was built from catalogue version 1970-01-01", "names the catalogue it expects")
	assert.Contains(t, err.Error(), "`just catalogue`")
	assert.Equal(t, before, catalogueRows(t, db), "rolled back: no spell updated or deleted")
	assert.Equal(t, cat.Version, recordedVersion(t, db), "the old version stays recorded")
}

// Step 5.4.3's refusal on a database whose recorded version migration 030
// forgot: the database was built from a catalogue (its spells are keyed),
// so the refusal says so instead of blaming the seeds that predate the
// catalogue artifact.
func TestImportCatalogue_RefusalWithoutARecordedVersionNamesAnEarlierCatalogue(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	plantState(t, db)
	_, err := db.Exec(`DELETE FROM catalogue_meta`)
	require.NoError(t, err)

	err = db.ImportCatalogue(ctx, withVersion(cat, "1970-01-02", without("magic_missile")), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "magic_missile (action_log 1)")
	assert.Contains(t, err.Error(), "This database was built from an earlier spell catalogue, which holds the spells listed, "+
		"whose version is not recorded")
	assert.NotContains(t, err.Error(), "migration")
	assert.NotContains(t, err.Error(), "predate")
}

// Without a recorded version or a keyed spell, the database holds the spell
// seeds that predate the catalogue artifact.
func TestRecordedCatalogue_NamesWhatTheDatabaseWasBuiltFrom(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	got, err := recordedCatalogue(ctx, db)
	require.NoError(t, err)
	assert.Equal(t, "the spell seeds that predate the catalogue artifact (the real catalogue's spells)", got)

	require.NoError(t, db.ImportCatalogue(ctx, fixtureCatalogue(t), nil))
	got, err = recordedCatalogue(ctx, db)
	require.NoError(t, err)
	assert.Equal(t, "catalogue version 1970-01-01", got)

	_, err = db.Exec(`DELETE FROM catalogue_meta`)
	require.NoError(t, err)
	got, err = recordedCatalogue(ctx, db)
	require.NoError(t, err)
	assert.Contains(t, got, "an earlier spell catalogue")
}

// The upgrade of a database built by the retired seed loader (migrations up
// to 026, spells without keys) holding a spellbook's worth of game state:
// after migration 027 and the import, nothing is lost and every reference
// (seeds and game state) resolves to a served spell.
func TestImportCatalogue_UpgradesADatabaseSeededBeforeTheCatalogue(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	applyUpTo(t, db, "001_init.sql", "002_ledgers.sql", "003_state.sql", "004_saves.sql", "005_spells.sql",
		"006_state_unique.sql", "007_action_log.sql", "008_weapons_items.sql", "009_spell_stats.sql",
		"010_crit_table.sql", "011_crit_decimal.sql", "012_correction_payload.sql", "013_saved_buffs.sql",
		"014_correction_note.sql", "015_item_behaviour.sql", "016_initiative.sql", "017_attack_form.sql",
		"018_xp_ledger.sql", "019_attack_form_unknown_ac.sql", "020_skills.sql", "021_character_details.sql",
		"022_dice_favourites.sql", "023_correction_step.sql", "024_dice_favourites_position.sql",
		"025_action_transaction.sql", "026_redo_correction.sql")
	cat := fixtureCatalogue(t)
	for i, s := range cat.Spells {
		insertLegacyRow(t, db, int64(i+1), s, "old scraped text")
	}
	plantState(t, db)

	require.NoError(t, db.Migrate(ctx))
	require.NoError(t, db.Seed(ctx, repoSeeds()))
	seeds := testdataSeedRefs(t)
	require.NoError(t, db.ImportCatalogue(ctx, cat, seeds))

	rows := catalogueRows(t, db)
	keyed := byKey(rows)
	assert.Len(t, rows, len(cat.Spells), "no spell lost, none duplicated")
	for i, s := range cat.Spells {
		assert.Equal(t, int64(i+1), keyed[s.ID].ID, "%s keeps its row id", s.ID)
	}
	state, err := StateSpellRefs(ctx, db)
	require.NoError(t, err)
	require.NotEmpty(t, state)
	var unresolved []string
	for _, r := range append(seeds, state...) {
		if _, ok := keyed[r.ID]; !ok {
			unresolved = append(unresolved, r.ID+" ("+r.Where+")")
		}
	}
	sort.Strings(unresolved)
	assert.Empty(t, unresolved, "every reference resolves")
	assert.Equal(t, 1, scalar(t, db, "SELECT COUNT(*) FROM spell_slots"), "game state untouched")
	assert.Equal(t, 9, scalar(t, db, "SELECT COUNT(*) FROM action_log"))

	before := catalogueRows(t, db)
	require.NoError(t, db.ImportCatalogue(ctx, cat, seeds))
	assert.Equal(t, before, catalogueRows(t, db), "a restart changes nothing")
}

// testdataSeedRefs are the spell ids the testdata spellbook and spell
// effects refer to (what the composition root passes).
func testdataSeedRefs(t *testing.T) []catalogue.Ref {
	t.Helper()
	var refs []catalogue.Ref
	for _, f := range []string{"spellbook.json", "spell_effects.json"} {
		raw, err := os.ReadFile(filepath.Join(repoSeeds(), f)) //nolint:gosec // fixed test paths
		require.NoError(t, err)
		var entries []struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(raw, &entries))
		for _, e := range entries {
			refs = append(refs, catalogue.Ref{ID: e.ID, Where: f})
		}
	}
	return refs
}
