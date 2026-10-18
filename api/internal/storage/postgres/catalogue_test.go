package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

func fixtureCatalogue(t *testing.T) catalogue.Catalogue {
	t.Helper()
	c, err := catalogueadpt.Load(filepath.Join("..", "..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	return c
}

// importOn runs ImportCatalogue's logic on the fake, which runs the transaction and the queries.
func importOn(f *fakeStore, cat catalogue.Catalogue, seedRefs []catalogue.Ref) error {
	return importCatalogue(context.Background(), f, f, cat, seedRefs)
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
	Level        int64
	// Texts are the row's texts by column; a NULL key has no entry.
	Texts map[string]localized.Text
	// TextKeys are the row's text keys by column (NULL: no entry).
	TextKeys map[string]int64
}

// textColumns are the spells columns that key a localized text, in the order of spellTexts.
var textColumns = []string{
	"name", "description", "school", "sphere", "casting_time", "range", "duration", "area_of_effect",
	"components", "saving_throw",
}

func catalogueRows(f *fakeStore) []catalogueRow {
	var out []catalogueRow
	for _, s := range f.sortedSpells() {
		r := catalogueRow{
			ID: s.ID, Key: s.Key, Source: s.Source, Class: s.Class, Stats: s.Stats.String, Level: s.Level,
			Texts: map[string]localized.Text{}, TextKeys: map[string]int64{},
		}
		for i, c := range textColumns {
			if k := s.Keys[i]; k.Valid {
				r.Texts[c] = localized.Text{}
				for lang, body := range f.t.Texts[k.Int64] {
					r.Texts[c][lang] = body
				}
				r.TextKeys[c] = k.Int64
			}
		}
		out = append(out, r)
	}
	return out
}

// assertTextIntegrity checks ADR 0023's storage invariants: every text key a spells, items or
// weapons column holds has at least one row, no row is unreachable from a column, and no body
// is empty.
func assertTextIntegrity(t *testing.T, f *fakeStore) {
	t.Helper()
	used := map[int64]bool{}
	for _, r := range catalogueRows(f) {
		for c, k := range r.TextKeys {
			used[k] = true
			assert.NotEmpty(t, r.Texts[c], "row %d %s: key %d has no row", r.ID, c, k)
		}
	}
	for _, it := range f.t.Items {
		used[it.Name] = true
		assert.NotEmpty(t, f.t.Texts[it.Name], "item %s: key %d has no row", it.ID, it.Name)
	}
	for _, w := range f.t.Weapons {
		used[w.Name] = true
		assert.NotEmpty(t, f.t.Texts[w.Name], "weapon %s: key %d has no row", w.ID, w.Name)
	}
	for id, text := range f.t.Texts {
		assert.True(t, used[id], "text %d is keyed by no column", id)
		for lang, body := range text {
			assert.NotEmpty(t, body, "text %d %s", id, lang)
		}
	}
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

func recordedVersion(f *fakeStore) string {
	if f.t.Meta == nil {
		return ""
	}
	return f.t.Meta.Version
}

// A fresh database gets every artifact spell under its id, each text as one localized_texts row
// per language present (ADR 0023: no "fr" row for a spell without French, no key for a text in
// no language), the source and the typed stats (step 5.6), and the version is recorded (step
// 5.5).
func TestImportCatalogue_FreshDatabase(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	require.NoError(t, importOn(f, cat, nil))

	rows := catalogueRows(f)
	keyed := byKey(rows)
	assert.Len(t, rows, len(cat.Spells))
	assert.Len(t, keyed, len(cat.Spells))
	for _, s := range cat.Spells {
		r, ok := keyed[s.ID]
		if !assert.True(t, ok, s.ID) {
			continue
		}
		assert.Equal(t, s.Class, r.Class, s.ID)
		assert.Equal(t, int64(s.Level), r.Level, s.ID)
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
	assert.Equal(t, cat.Version, recordedVersion(f))
	assertTextIntegrity(t, f)
}

func TestImportCatalogue_Step6_TypedStatsParsedAtImport(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, importOn(f, fixtureCatalogue(t), nil))
	r := byKey(catalogueRows(f))["magic_missile"]
	var stats spellstats.Stats
	require.NoError(t, json.Unmarshal([]byte(r.Stats), &stats))
	assert.Equal(t, spellstats.Parse(spellstats.Raw{
		CastingTime: "1", Range: "60 yds. + 10 yds./level", Duration: "Instantaneous",
		AreaOfEffect: "1-5 targets", Components: "V, S", SavingThrow: "None",
	}), stats)
}

// setStats overwrites the stored typed stats of the spells with these keys.
func (f *fakeStore) setStats(stats sql.NullString, keys ...string) {
	for i := range f.t.Spells {
		for _, k := range keys {
			if f.t.Spells[i].Key.String == k {
				f.t.Spells[i].Stats = stats
			}
		}
	}
}

// Step 5.6 on a start that imports nothing: the stored typed stats are what an older parser made
// of the stored texts. The start re-types them from those texts, writes no other column and
// keeps the recorded version, so a parser change needs no migration and no new artifact
// version.
func TestImportCatalogue_Step6_AStartReTypesStatsAParserChangeMadeStale(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	require.NoError(t, importOn(f, cat, nil))
	fresh := byKey(catalogueRows(f))
	f.setStats(sql.NullString{String: `{"area":{"raw":"older parser","kind":"special"}}`, Valid: true}, "fireball", "magic_missile")
	f.setStats(sql.NullString{}, "sleep")

	require.NoError(t, importOn(f, cat, nil))
	after := byKey(catalogueRows(f))
	assert.Equal(t, fresh, after, "every stale row re-typed, nothing else written")
	var stats spellstats.Stats
	require.NoError(t, json.Unmarshal([]byte(after["magic_missile"].Stats), &stats))
	assert.Equal(t, "targets", stats.Area.Kind, "parsed again from the stored area text")
	assert.Equal(t, cat.Version, recordedVersion(f))
}

// Step 5.3: the recorded version equals the artifact's: nothing is written, so a row edited
// behind the importer's back stays edited.
func TestImportCatalogue_Step3_SameVersionWritesNothing(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	require.NoError(t, importOn(f, cat, nil))
	f.t.Texts[byKey(catalogueRows(f))["fireball"].TextKeys["description"]]["en"] = "edited"
	before := catalogueRows(f)

	require.NoError(t, importOn(f, cat, nil))
	assert.Equal(t, before, catalogueRows(f))
	assert.Equal(t, "edited", byKey(catalogueRows(f))["fireball"].Texts["description"].Default())
}

// Step 5.4.2: a new version overwrites every catalogue field by key and inserts new spells; row
// ids stay. Equality only: going back to the older version imports it again.
func TestImportCatalogue_Step4_NewVersionUpserts(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	require.NoError(t, importOn(f, cat, nil))
	first := byKey(catalogueRows(f))["fireball"]

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
	require.NoError(t, importOn(f, next, nil))
	rows := byKey(catalogueRows(f))
	assert.Len(t, rows, len(cat.Spells)+1)
	fb := rows["fireball"]
	assert.Equal(t, first.ID, fb.ID, "the row keeps its id")
	assert.Equal(t, first.TextKeys, fb.TextKeys, "and its text keys")
	assert.Equal(t, localized.Text{"en": "Fixture text, corrected."}, fb.Texts["description"])
	assert.Equal(t, localized.Text{"en": "Fireball"}, fb.Texts["name"], "a French name the artifact dropped is cleared")
	assert.NotContains(t, f.t.Texts[fb.TextKeys["name"]], "fr", "its row is deleted")
	assert.Contains(t, fb.Stats, "20 yds.", "stats parsed again")
	assert.Equal(t, "Nouveau sort", rows["new_spell"].Texts["name"]["fr"])
	assert.Equal(t, localized.Text{"en": "All"}, rows["new_spell"].Texts["sphere"])
	assert.Equal(t, "1970-01-02", recordedVersion(f))
	assertTextIntegrity(t, f)

	require.NoError(t, importOn(f, cat, nil), "an older version is an import like any other")
	assert.Equal(t, cat.ByID()["fireball"].Description, byKey(catalogueRows(f))["fireball"].Texts["description"])
	assert.NotContains(t, byKey(catalogueRows(f)), "new_spell", "unreferenced, so deleted")
	assertTextIntegrity(t, f)
}

// ADR 0023 at re-import: a text the artifact drops whole (a sphere) loses its key and every row;
// one it adds gets a new key; the texts of a deleted spell go with it.
func TestImportCatalogue_ReimportDropsAndAddsWholeTexts(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	require.NoError(t, importOn(f, cat, nil))
	priest := byKey(catalogueRows(f))["detect_magic_priest"]
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
	require.NoError(t, importOn(f, next, nil))
	rows := byKey(catalogueRows(f))
	assert.NotContains(t, rows["detect_magic_priest"].TextKeys, "sphere")
	assert.NotContains(t, f.t.Texts, priest.TextKeys["sphere"], "the dropped text's rows are deleted")
	assert.Equal(t, localized.Text{"en": "Protection"}, rows["armor"].Texts["sphere"])
	assert.NotContains(t, rows, "slow")
	assertTextIntegrity(t, f)
}

// plantOldRow stores a spells row as the SQLite schema left one seeded before the catalogue: no
// key, its English texts as localized_texts rows under keys of their own.
func (f *fakeStore) plantOldRow(id int64, s catalogue.Spell, description string) {
	texts := []string{
		s.Name.Default(), description, s.School.Default(), s.Sphere.Default(), s.CastingTime.Default(),
		s.Range.Default(), s.Duration.Default(), s.AreaOfEffect.Default(), s.Components.Default(), s.SavingThrow.Default(),
	}
	row := spellRow{ID: id, Class: s.Class, Level: int64(s.Level)}
	for i, body := range texts {
		if body == "" {
			continue
		}
		k := (id-1)*10 + int64(i) + 1
		row.Keys[i] = sql.NullInt64{Int64: k, Valid: true}
		f.t.Texts[k] = localized.Text{"en": body}
	}
	f.t.Spells = append(f.t.Spells, row)
}

// Step 5.4.1: a database seeded before the catalogue. Its rows are keyed by spellsvc.Keys and
// matched by (name, class, level); only the oldest row of a duplicated triple takes the key, the
// younger one keeps NULL and is left alone; no spell is inserted twice and row ids stay.
func TestImportCatalogue_Step4_1_BackfillsKeysOfAnExistingDatabase(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	for i, s := range cat.Spells {
		f.plantOldRow(int64(100+i), s, "old scraped text")
	}
	f.plantOldRow(900, cat.ByID()["fireball"], "younger duplicate")

	require.NoError(t, importOn(f, cat, nil))
	rows := catalogueRows(f)
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

// Step 5.4.1 verification: rows outside the set the ids were published from shift the Keys
// suffixes. Here a homebrew Wizard "Fortify" takes the slug "fortify" and pushes the Priest spell
// to "fortify_priest", while the artifact's "fortify" is the Priest spell. A key whose row is not
// the artifact's spell is refused, listing both triples, and nothing changes.
func TestImportCatalogue_Step4_1_RefusesKeysThatDoNotMatchTheArtifact(t *testing.T) {
	f := newFakeStore()
	cat := withVersion(fixtureCatalogue(t), "1970-01-03", func(in []catalogue.Spell) []catalogue.Spell {
		return append(in, catalogue.Spell{
			ID: "fortify", Class: "Priest", Level: 3, School: localized.Of("Alteration"), Sphere: localized.Of("War"),
			CastingTime: localized.Of("1"), Range: localized.Of("0"), Duration: localized.Of("1 rd."),
			AreaOfEffect: localized.Of("The caster"), Components: localized.Of("V"), SavingThrow: localized.Of("None"),
			Name: localized.Of("Fortify"), Description: localized.Of("Fixture text."), Source: "TOM",
		})
	})
	priest := cat.ByID()["fortify"]
	f.plantOldRow(1, priest, "old")
	homebrew := priest
	homebrew.Class, homebrew.Level = "Wizard", 1
	f.plantOldRow(2, homebrew, "homebrew")
	before := catalogueRows(f)

	err := importOn(f, cat, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fortify: database row 2 is (\"Fortify\", Wizard, 1), the artifact has (\"Fortify\", Priest, 3)")
	assert.Equal(t, before, catalogueRows(f), "nothing keyed, nothing written")
	assert.Empty(t, recordedVersion(f))
}

// Step 5.4.3: a spell the new artifact drops is deleted, with its texts, when nothing refers to
// it.
func TestImportCatalogue_Step4_3_DeletesAnUnreferencedRemovedSpell(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	refs := []catalogue.Ref{{ID: "fireball", Where: "spellbook seed"}}
	require.NoError(t, importOn(f, cat, refs))
	slow := byKey(catalogueRows(f))["slow"]
	require.NoError(t, importOn(f, withVersion(cat, "1970-01-02", without("slow")), refs))
	keyed := byKey(catalogueRows(f))
	assert.NotContains(t, keyed, "slow")
	assert.Len(t, keyed, len(cat.Spells)-1)
	for c, k := range slow.TextKeys {
		assert.NotContains(t, f.t.Texts, k, "the %s of the deleted spell is deleted", c)
	}
	assertTextIntegrity(t, f)
}

// plantState writes game state referring to spells in every place step 5.4.3 scans.
func (f *fakeStore) plantState() {
	f.t.Characters = append(f.t.Characters, 1)
	f.t.Spellbook[spellbookKey(1, "burning_hands")] = storeq.UpsertSpellbookEntryParams{CharacterID: 1, SpellKey: "burning_hands"}
	f.t.Slots[1] = `[{"level":1,"prepared_spell_id":"sleep","prepared_spell_name":"Sleep","used":false},{"level":1,"prepared_spell_name":"","used":false}]`
	f.t.Buffs[1] = `[{"id":"a","name":"Stoneskin","spell_id":"stoneskin"},{"id":"b","name":"Gem"}]`
	f.t.Log = []logRow{
		{1, "cast", `{"action":"cast","spellId":"magic_missile","spell":"Magic Missile","casterLevel":5}`},
		{2, "exposure", `{"spellId":"cloudkill","spell":"Cloudkill","casterLevel":9,"round":1,"onSelf":true}`},
		{3, "spell_prep", `{"spells":[{"id":"armor","name":"Armor","count":1}]}`},
		{4, "hp_change", `{}`},
		{5, "cast", `{"action":"cast","spellId":"haste","spell":"Haste","casterLevel":5}`},
		{6, "buff_change", `{"action":"dismiss","buff":"Detect Magic","spellId":"detect_magic"}`},
		{7, "round_advance", `{"action":"next_round","expired":["Wall of Ice","Gem"],"expiredSpellIds":["wall_of_ice",""]}`},
		{8, "time_passed", `{"action":"pass_time","expired":["Chromatic Orb"],"expiredSpellIds":["chromatic_orb"]}`},
		{9, "hp_change", `{"spellId":"not_a_spell_entry"}`},
	}
	f.t.Snapshots = []snapshotRow{
		{
			1, 3, "spell_slots", `[{"Level":1,"SpellID":"identify","PreparedSpellName":"Identify","Used":false,"Extra":false}]`,
			`[{"Level":1,"SpellID":"light","PreparedSpellName":"Light","Used":false,"Extra":false}]`,
		},
		{2, 4, "buffs", `[{"SpellID":"not_a_slot","spell_id":"improved_strength"}]`, `[{"spell_id":"mirror_image"}]`},
		{3, 5, "round", `[{"spell_id":"not_a_buff_list"}]`, `[]`},
	}
	f.t.Corrections = []correctionRow{{1, 5, `{"spellId":"enlarge","casterLevel":5}`}}
}

// Every place game state holds a spell id is scanned; a snapshot of another state key and
// entries of other kinds are not.
func TestStateSpellRefs_ScansEveryPlace(t *testing.T) {
	f := newFakeStore()
	f.plantState()
	refs, err := StateSpellRefs(context.Background(), f)
	require.NoError(t, err)
	got := map[string]string{}
	for _, r := range refs {
		got[r.ID] = r.Where
	}
	assert.Equal(t, map[string]string{
		"burning_hands": "spellbook of character 1",
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
	f := newFakeStore()
	f.plantState()
	f.t.Slots[1] = `{not json`
	_, err := StateSpellRefs(context.Background(), f)
	require.ErrorContains(t, err, "spell_slots of character 1")
}

// Step 5.4.3: an artifact that removes referenced spells is refused, listing every such key and
// where it is used (seeds, slots, snapshots, log entries, corrections), and the import rolls
// back whole.
func TestImportCatalogue_Step4_3_RefusesRemovingReferencedSpells(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	seeds := []catalogue.Ref{{ID: "strength", Where: "spellbook seed"}, {ID: "strength", Where: "spell effects seed"}}
	require.NoError(t, importOn(f, cat, seeds))
	f.plantState()
	before := copyTables(f.t)

	removed := []string{"strength", "burning_hands", "sleep", "identify", "light", "magic_missile", "cloudkill", "armor", "enlarge"}
	next := withVersion(cat, "1970-01-02", func(in []catalogue.Spell) []catalogue.Spell {
		for _, id := range removed {
			in = without(id)(in)
		}
		for i := range in {
			in[i].Description = localized.Of("Fixture text, changed.")
		}
		return in
	})
	err := importOn(f, next, seeds)
	require.Error(t, err)
	for _, want := range []string{
		"strength (spellbook seed, spell effects seed)", "burning_hands (spellbook of character 1)",
		"sleep (spell_slots of character 1)",
		"identify (action_snapshots 1)", "light (action_snapshots 1)", "magic_missile (action_log 1)",
		"cloudkill (action_log 2)", "armor (action_log 3)", "enlarge (action_corrections 1)",
	} {
		assert.Contains(t, err.Error(), want)
	}
	assert.Contains(t, err.Error(), "the artifact (version 1970-01-02)")
	assert.Contains(t, err.Error(), "This database was built from catalogue version 1970-01-01", "names the catalogue it expects")
	assert.Contains(t, err.Error(), "`just catalogue`")
	assert.Equal(t, before, f.t, "rolled back: no spell or text updated or deleted")
	assert.Equal(t, cat.Version, recordedVersion(f), "the old version stays recorded")
}

// Step 5.4.3's refusal on a database whose recorded version an upgrade of the SQLite schema
// cleared: the database was built from a catalogue (its spells are keyed), so the refusal says
// so instead of blaming the seeds that predate the catalogue artifact.
func TestImportCatalogue_RefusalWithoutARecordedVersionNamesAnEarlierCatalogue(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	require.NoError(t, importOn(f, cat, nil))
	f.plantState()
	f.t.Meta = nil

	err := importOn(f, withVersion(cat, "1970-01-02", without("magic_missile")), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "magic_missile (action_log 1)")
	assert.Contains(t, err.Error(), "This database was built from an earlier spell catalogue, which holds the spells listed, "+
		"whose version is not recorded")
	assert.NotContains(t, err.Error(), "migration")
	assert.NotContains(t, err.Error(), "predate")
}

// Without a recorded version or a keyed spell, the database holds the spell seeds that predate
// the catalogue artifact.
func TestRecordedCatalogue_NamesWhatTheDatabaseWasBuiltFrom(t *testing.T) {
	ctx := context.Background()
	f := newFakeStore()
	got, err := recordedCatalogue(ctx, f)
	require.NoError(t, err)
	assert.Equal(t, "the spell seeds that predate the catalogue artifact (the real catalogue's spells)", got)

	require.NoError(t, importOn(f, fixtureCatalogue(t), nil))
	got, err = recordedCatalogue(ctx, f)
	require.NoError(t, err)
	assert.Equal(t, "catalogue version 1970-01-01", got)

	f.t.Meta = nil
	got, err = recordedCatalogue(ctx, f)
	require.NoError(t, err)
	assert.Contains(t, got, "an earlier spell catalogue")
}

// A database built before the catalogue artifact (spells without keys, as the SQLite schema held
// them) holding a spellbook's worth of game state: after the
// seed and the import, nothing is lost and every reference (seeds and game state) resolves to a
// served spell; a restart changes nothing.
func TestImportCatalogue_UpgradesADatabaseSeededBeforeTheCatalogue(t *testing.T) {
	f := newFakeStore()
	cat := fixtureCatalogue(t)
	for i, s := range cat.Spells {
		f.plantOldRow(int64(i+1), s, "old scraped text")
	}
	f.plantState()

	require.NoError(t, seedOn(f, repoSeeds()))
	seeds := testdataSeedRefs(t)
	require.NoError(t, importOn(f, cat, seeds))

	rows := catalogueRows(f)
	keyed := byKey(rows)
	assert.Len(t, rows, len(cat.Spells), "no spell lost, none duplicated")
	for i, s := range cat.Spells {
		assert.Equal(t, int64(i+1), keyed[s.ID].ID, "%s keeps its row id", s.ID)
	}
	state, err := StateSpellRefs(context.Background(), f)
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
	assert.Len(t, f.t.Slots, 1, "game state untouched")
	assert.Len(t, f.t.Log, 9)

	before := catalogueRows(f)
	require.NoError(t, importOn(f, cat, seeds))
	assert.Equal(t, before, catalogueRows(f), "a restart changes nothing")
}

// testdataSeedRefs are the spell ids the testdata spellbook and spell effects refer to (what the
// composition root passes).
func testdataSeedRefs(t *testing.T) []catalogue.Ref {
	t.Helper()
	var refs []catalogue.Ref
	for _, name := range []string{"spellbook.json", "spell_effects.json"} {
		raw, err := os.ReadFile(filepath.Join(repoSeeds(), name)) //nolint:gosec // fixed test paths
		require.NoError(t, err)
		var entries []struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(raw, &entries))
		for _, e := range entries {
			refs = append(refs, catalogue.Ref{ID: e.ID, Where: name})
		}
	}
	return refs
}
