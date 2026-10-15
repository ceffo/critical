package sqlite

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

type catalogueRow struct {
	ID                                      int64
	Key, NameFR, DescriptionFR, Source      sql.NullString
	Name, Class, Description, Stats, Sphere string
	Level                                   int
	StatsFR                                 statsFRRow
}

// statsFRRow is the six French stat columns, in catalogue.StatsFR's order.
type statsFRRow struct {
	CastingTime, Range, Duration, AreaOfEffect, Components, SavingThrow sql.NullString
}

func (r statsFRRow) texts() catalogue.StatsFR {
	return catalogue.StatsFR{
		CastingTime: r.CastingTime.String, Range: r.Range.String, Duration: r.Duration.String,
		AreaOfEffect: r.AreaOfEffect.String, Components: r.Components.String, SavingThrow: r.SavingThrow.String,
	}
}

func (r statsFRRow) nulls() []bool {
	return []bool{
		!r.CastingTime.Valid, !r.Range.Valid, !r.Duration.Valid,
		!r.AreaOfEffect.Valid, !r.Components.Valid, !r.SavingThrow.Valid,
	}
}

func emptyFR(s catalogue.StatsFR) []bool {
	return []bool{
		s.CastingTime == "", s.Range == "", s.Duration == "",
		s.AreaOfEffect == "", s.Components == "", s.SavingThrow == "",
	}
}

func catalogueRows(t *testing.T, db *DB) []catalogueRow {
	t.Helper()
	rows, err := db.Query(`SELECT id, key, name, name_fr, class, level, COALESCE(sphere, ''), COALESCE(description, ''),
		description_fr, source, COALESCE(stats, ''), casting_time_fr, range_fr, duration_fr, area_of_effect_fr,
		components_fr, saving_throw_fr FROM spells ORDER BY id`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []catalogueRow
	for rows.Next() {
		var r catalogueRow
		require.NoError(t, rows.Scan(&r.ID, &r.Key, &r.Name, &r.NameFR, &r.Class, &r.Level, &r.Sphere, &r.Description,
			&r.DescriptionFR, &r.Source, &r.Stats, &r.StatsFR.CastingTime, &r.StatsFR.Range, &r.StatsFR.Duration,
			&r.StatsFR.AreaOfEffect, &r.StatsFR.Components, &r.StatsFR.SavingThrow))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
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

// A fresh database gets every artifact spell under its id, with the French
// texts when present (NULL otherwise), the source and the typed stats (step
// 5.6), and the version is recorded (step 5.5).
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
		assert.Equal(t, s.Name, r.Name, s.ID)
		assert.Equal(t, s.Class, r.Class, s.ID)
		assert.Equal(t, s.Level, r.Level, s.ID)
		assert.Equal(t, s.Description, r.Description, s.ID)
		assert.Equal(t, s.NameFR, r.NameFR.String, s.ID)
		assert.Equal(t, s.NameFR != "", r.NameFR.Valid, "%s: an absent French name is NULL", s.ID)
		assert.Equal(t, s.DescriptionFR != "", r.DescriptionFR.Valid, s.ID)
		assert.Equal(t, s.Source, r.Source.String, s.ID)
		assert.Equal(t, s.StatsFR, r.StatsFR.texts(), s.ID)
		assert.Equal(t, emptyFR(s.StatsFR), r.StatsFR.nulls(), "%s: an absent French stat is NULL, per field", s.ID)
	}
	assert.Equal(t, cat.Version, recordedVersion(t, db))
	assert.Equal(t, "60 mètres + 10 mètres par niveau", keyed["magic_missile"].StatsFR.Range.String)
	assert.False(t, keyed["magic_missile"].StatsFR.CastingTime.Valid)
}

// Step 5.6: the typed stats are parsed from the English texts only; magic_missile
// has French stat texts, which must not change them.
func TestImportCatalogue_Step6_TypedStatsParsedAtImport(t *testing.T) {
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	require.NotEmpty(t, cat.ByID()["magic_missile"].StatsFR.Range)
	require.NoError(t, db.ImportCatalogue(context.Background(), cat, nil))
	r := byKey(catalogueRows(t, db))["magic_missile"]
	var stats spellstats.Stats
	require.NoError(t, json.Unmarshal([]byte(r.Stats), &stats))
	assert.Equal(t, spellstats.Parse(spellstats.Raw{
		CastingTime: "1", Range: "60 yds. + 10 yds./level", Duration: "Instantaneous",
		AreaOfEffect: "1-5 targets", Components: "V, S", SavingThrow: "None",
	}), stats)
}

// Step 5.3: the recorded version equals the artifact's: nothing is written,
// so a row edited behind the importer's back stays edited.
func TestImportCatalogue_Step3_SameVersionWritesNothing(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	_, err := db.Exec(`UPDATE spells SET description = 'edited' WHERE key = 'fireball'`)
	require.NoError(t, err)
	before := catalogueRows(t, db)

	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	assert.Equal(t, before, catalogueRows(t, db))
	assert.Equal(t, "edited", byKey(catalogueRows(t, db))["fireball"].Description)
}

// Step 5.4.2: a new version overwrites every catalogue field by key and
// inserts new spells; row ids stay. Equality only: going back to the older
// version imports it again.
func TestImportCatalogue_Step4_NewVersionUpserts(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cat := fixtureCatalogue(t)
	require.NoError(t, db.ImportCatalogue(ctx, cat, nil))
	fireballID := byKey(catalogueRows(t, db))["fireball"].ID

	next := withVersion(cat, "1970-01-02", func(in []catalogue.Spell) []catalogue.Spell {
		for i := range in {
			if in[i].ID == "fireball" {
				in[i].Description = "Fixture text, corrected."
				in[i].NameFR = ""
				in[i].Range = "20 yds."
				in[i].StatsFR.Duration = "Instantanée"
			}
			if in[i].ID == "magic_missile" {
				in[i].StatsFR.Range = ""
			}
		}
		return append(in, catalogue.Spell{
			ID: "new_spell", Class: "Priest", Level: 2, School: "Alteration", Sphere: "All",
			CastingTime: "1", Range: "0", Duration: "1 rd.", AreaOfEffect: "The caster", Components: "V",
			SavingThrow: "None", Name: "New Spell", NameFR: "Nouveau sort", Description: "Fixture text.", Source: "TOM",
		})
	})
	require.NoError(t, db.ImportCatalogue(ctx, next, nil))
	rows := byKey(catalogueRows(t, db))
	assert.Len(t, rows, len(cat.Spells)+1)
	fb := rows["fireball"]
	assert.Equal(t, fireballID, fb.ID, "the row keeps its id")
	assert.Equal(t, "Fixture text, corrected.", fb.Description)
	assert.False(t, fb.NameFR.Valid, "a French name the artifact dropped is cleared")
	assert.Contains(t, fb.Stats, "20 yds.", "stats parsed again")
	assert.Equal(t, "Instantanée", fb.StatsFR.Duration.String, "a French stat the artifact added is stored")
	assert.False(t, rows["magic_missile"].StatsFR.Range.Valid, "a French stat the artifact dropped is cleared")
	assert.Equal(t, "Instantanée", rows["magic_missile"].StatsFR.Duration.String, "the other French stats stay")
	assert.Equal(t, "Nouveau sort", rows["new_spell"].NameFR.String)
	assert.Equal(t, "All", rows["new_spell"].Sphere)
	assert.Equal(t, "1970-01-02", recordedVersion(t, db))

	require.NoError(t, db.ImportCatalogue(ctx, cat, nil), "an older version is an import like any other")
	assert.Equal(t, cat.ByID()["fireball"].Description, byKey(catalogueRows(t, db))["fireball"].Description)
	assert.NotContains(t, byKey(catalogueRows(t, db)), "new_spell", "unreferenced, so deleted")
}

// insertOldRow writes a spells row as the retired seed loader did: no key,
// no French texts.
func insertOldRow(t *testing.T, db *DB, id int64, s catalogue.Spell, description string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO spells (id, name, class, level, school, sphere, casting_time, range, duration,
		area_of_effect, components, saving_throw, description) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, s.Name, s.Class, s.Level, s.School, s.Sphere, s.CastingTime, s.Range, s.Duration, s.AreaOfEffect,
		s.Components, s.SavingThrow, description)
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
		assert.Equal(t, s.Description, keyed[s.ID].Description, "%s overwritten by the artifact", s.ID)
	}
	for _, r := range rows {
		if r.ID == 900 {
			assert.False(t, r.Key.Valid, "the younger duplicate keeps a NULL key")
			assert.Equal(t, "younger duplicate", r.Description, "and is never updated")
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
			ID: "fortify", Class: "Priest", Level: 3, School: "Alteration", Sphere: "War",
			CastingTime: "1", Range: "0", Duration: "1 rd.", AreaOfEffect: "The caster", Components: "V",
			SavingThrow: "None", Name: "Fortify", Description: "Fixture text.", Source: "TOM",
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
			in[i].Description = "Fixture text, changed."
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
		insertOldRow(t, db, int64(i+1), s, "old scraped text")
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
