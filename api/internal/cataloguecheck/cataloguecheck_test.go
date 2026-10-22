// Package cataloguecheck holds the tests of the spell catalogue artifact's
// JSON Schema (ADR 0021): the schema compiles with the Go engine the importer
// uses, accepts the fixture api/testdata/catalogue.json, and enforces the
// description subset; the fixture covers what CI and e2e need.
package cataloguecheck_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func compile(t *testing.T) *jsonschema.Schema {
	t.Helper()
	f, err := os.Open("../../../schemas/catalogue.schema.json")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	doc, err := jsonschema.UnmarshalJSON(f)
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	require.NoError(t, c.AddResource("catalogue.schema.json", doc))
	sch, err := c.Compile("catalogue.schema.json")
	require.NoError(t, err, "the schema must compile with Go's RE2 regexp engine")
	return sch
}

func spell(id string, desc map[string]any) map[string]any {
	return map[string]any{
		"id": id, "class": "Wizard", "level": 1, "school": "Evocation",
		"casting_time": "1", "range": "10 yards", "duration": "1 round",
		"area_of_effect": "One creature", "components": "V, S", "saving_throw": "None",
		"name": map[string]any{"en": "Spell"}, "description": desc, "source": "PHB",
	}
}

// fixtureDoc is a fresh copy of the fixture, parsed the way the schema engine reads a document, for
// a test to edit.
func fixtureDoc(t *testing.T) map[string]any {
	t.Helper()
	f, err := os.Open("../../testdata/catalogue.json")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	doc, err := jsonschema.UnmarshalJSON(f)
	require.NoError(t, err)
	a, ok := doc.(map[string]any)
	require.True(t, ok)
	return a
}

// artifact is a valid current-format document around the given spells: the fixture's other parts stand in
// for the required ones.
func artifact(t *testing.T, spells ...any) map[string]any {
	t.Helper()
	a := fixtureDoc(t)
	a["spells"] = spells
	return a
}

// tables is the artifact's rule_tables, for a test to edit.
func tables(a map[string]any) map[string]any {
	m, _ := a["rule_tables"].(map[string]any)
	return m
}

func withDesc(t *testing.T, en string) map[string]any {
	return artifact(t, spell("a", map[string]any{"en": en}))
}

// TestSchema_FixtureIsValid: the fixture CI and e2e run on is a valid artifact.
func TestSchema_FixtureIsValid(t *testing.T) {
	sch := compile(t)
	f, err := os.Open("../../testdata/catalogue.json")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	doc, err := jsonschema.UnmarshalJSON(f)
	require.NoError(t, err)
	assert.NoError(t, sch.Validate(doc))
}

type fixtureSpell struct {
	ID          string            `json:"id"`
	Name        map[string]string `json:"name"`
	Description map[string]string `json:"description"`
}

func readFixture(t *testing.T) []fixtureSpell {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/catalogue.json")
	require.NoError(t, err)
	var a struct {
		Spells []fixtureSpell `json:"spells"`
	}
	require.NoError(t, json.Unmarshal(raw, &a))
	require.NotEmpty(t, a.Spells)
	return a.Spells
}

// TestFixture_HoldsEveryReferencedSpell (ADR 0021 step 4): every spell id the
// testdata spellbook and spell effects refer to is in the fixture, so the
// e2e suite and the service tests resolve them without the private artifact.
func TestFixture_HoldsEveryReferencedSpell(t *testing.T) {
	ids := map[string]bool{}
	for _, s := range readFixture(t) {
		ids[s.ID] = true
	}
	for _, file := range []string{"spellbook.json", "spell_effects.json"} {
		raw, err := os.ReadFile("../../testdata/seeds/" + file) //nolint:gosec // fixed test paths
		require.NoError(t, err)
		var refs []struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(raw, &refs))
		require.NotEmpty(t, refs, file)
		for _, r := range refs {
			assert.True(t, ids[r.ID], "%s refers to %s, which the fixture lacks", file, r.ID)
		}
	}
}

// TestFixture_CoversTheFormat: some spells have French texts and some do not,
// one description has a bullet list and one a table; every description is
// invented placeholder text, never book text.
func TestFixture_CoversTheFormat(t *testing.T) {
	var translated, untranslated, lists, tables int
	for _, s := range readFixture(t) {
		if s.Name["fr"] != "" {
			translated++
			assert.NotEmpty(t, s.Description["fr"], "%s: a French name comes with a French description", s.ID)
		} else {
			untranslated++
		}
		if strings.Contains(s.Description["en"], "\n- ") {
			lists++
		}
		if strings.Contains(s.Description["en"], "\n| --- |") {
			tables++
		}
		assert.True(t, strings.HasPrefix(s.Description["en"], "Fixture text for "), "%s: invented text only", s.ID)
		if fr := s.Description["fr"]; fr != "" {
			assert.True(t, strings.HasPrefix(fr, "Texte de test pour "), "%s: invented French text only", s.ID)
		}
	}
	assert.Positive(t, translated)
	assert.Positive(t, untranslated)
	assert.Positive(t, lists, "one description with a bullet list")
	assert.Positive(t, tables, "one description with a table")
}

func TestSchema_DescriptionSubset(t *testing.T) {
	sch := compile(t)
	good := []string{
		"a", "Hello world.", "A\n\nB", "-1 penalty to hit.", "- a\n- b",
		"Intro.\n\n- a\n- b\n\nOutro.", "| a | b |\n| - | - |\n| 1 | 2 |", "x\n\n| a |\n| - |\n\ny", "-x",
	}
	bad := []string{
		"", "a\nb", "a ", " a", "a\n\n\nb", "a\n\n", "a \n\nb", "- a\n\n- b\nplain",
		"- a\nplain", "| a |\ntext", "# H", "A.\n\n# H", "A.\n\n## H\n\nB",
	}
	for _, d := range good {
		assert.NoError(t, sch.Validate(withDesc(t, d)), "good %q", d)
	}
	for _, d := range bad {
		assert.Error(t, sch.Validate(withDesc(t, d)), "bad %q", d)
	}
}

func TestSchema_Shape(t *testing.T) {
	sch := compile(t)
	assert.NoError(t, sch.Validate(artifact(t, spell("a", map[string]any{"en": "x", "fr": "y"}))))
	assert.Error(t, sch.Validate(map[string]any{"format": 2, "version": "2026-10-15", "spells": []any{spell("a", map[string]any{"en": "x"})}}), "unknown format")
	assert.Error(t, sch.Validate(artifact(t, spell("a", map[string]any{"en": "x", "fr": ""}))), "empty fr")
	assert.Error(t, sch.Validate(artifact(t, spell("A-b", map[string]any{"en": "x"}))), "bad id")
	extra := spell("a", map[string]any{"en": "x"})
	extra["extra"] = 1
	assert.Error(t, sch.Validate(artifact(t, extra)), "unknown property")
}

func row(tables map[string]any, table string, i int) map[string]any {
	rows, _ := tables[table].([]any)
	r, _ := rows[i].(map[string]any)
	return r
}

// refuseEach checks that every change makes the fixture artifact invalid.
func refuseEach(t *testing.T, changes map[string]func(a map[string]any)) {
	t.Helper()
	sch := compile(t)
	for name, change := range changes {
		a := fixtureDoc(t)
		change(a)
		assert.Error(t, sch.Validate(a), name)
	}
}

// TestSchema_Step17_OnlyFormat10IsValid (ADR 0021 steps 16.5 and 17.3): the schema accepts format 10
// only; every earlier format, retired, is refused whatever it carries (the API refuses it as an
// unknown format), and so is a wisdom row without the magical defence adjustment.
func TestSchema_Step17_OnlyFormat10IsValid(t *testing.T) {
	sch := compile(t)
	assert.NoError(t, sch.Validate(fixtureDoc(t)), "the fixture is format 10")
	for _, f := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 11} {
		a := fixtureDoc(t)
		a["format"] = f
		assert.Error(t, sch.Validate(a), "format %d", f)
	}
	for _, part := range []string{"equipment", "languages", "proficiency_descriptions", "rule_tables"} {
		a := fixtureDoc(t)
		delete(a, part)
		assert.Error(t, sch.Validate(a), "format 10 requires %s", part)
	}
	a := fixtureDoc(t)
	delete(row(tables(a), "wisdom", 7), "magic_defence")
	assert.Error(t, sch.Validate(a), "format 10 requires the magical defence adjustment")
}

// TestSchema_Step9_RuleTables (ADR 0021 steps 9.1 to 9.3): the four PHB rule tables are numbers
// only.
func TestSchema_Step9_RuleTables(t *testing.T) {
	refuseEach(t, map[string]func(map[string]any){
		"a missing table":       func(a map[string]any) { delete(tables(a), "bard_experience") },
		"an unknown table":      func(a map[string]any) { tables(a)["crit"] = []any{} },
		"a text in the tables":  func(a map[string]any) { tables(a)["_source"] = "PHB" },
		"an unknown row field":  func(a map[string]any) { row(tables(a), "strength", 0)["note"] = "x" },
		"a missing row field":   func(a map[string]any) { delete(row(tables(a), "dexterity", 0), "missile") },
		"a text value":          func(a map[string]any) { row(tables(a), "strength", 0)["hit"] = "-5" },
		"a fractional value":    func(a map[string]any) { row(tables(a), "dexterity", 0)["reaction"] = 0.5 },
		"a score above 25":      func(a map[string]any) { row(tables(a), "strength", 0)["str_max"] = 26 },
		"a percentile over 100": func(a map[string]any) { row(tables(a), "strength", 13)["exc_max"] = 101 },
		"a bard level over 20":  func(a map[string]any) { row(tables(a), "bard_spells", 0)["level"] = 21 },
		"five spell levels": func(a map[string]any) {
			row(tables(a), "bard_spells", 0)["per_spell_level"] = []any{0, 0, 0, 0, 0}
		},
		"a negative spell count": func(a map[string]any) {
			row(tables(a), "bard_spells", 0)["per_spell_level"] = []any{-1, 0, 0, 0, 0, 0}
		},
		"negative XP": func(a map[string]any) { row(tables(a), "bard_experience", 0)["xp"] = -1 },
		"21 bard levels": func(a map[string]any) {
			rows, _ := tables(a)["bard_spells"].([]any)
			tables(a)["bard_spells"] = append(rows, rows[0])
		},
		"19 experience levels": func(a map[string]any) {
			rows, _ := tables(a)["bard_experience"].([]any)
			tables(a)["bard_experience"] = rows[:19]
		},
	})
}

// TestSchema_Step10_ClassTables (ADR 0021 steps 10.1 to 10.3): every class and race table is
// numbers and language-neutral codes only.
func TestSchema_Step10_ClassTables(t *testing.T) {
	refuseEach(t, map[string]func(map[string]any){
		"a missing table":          func(a map[string]any) { delete(tables(a), "thac0") },
		"an unknown class code":    func(a map[string]any) { row(tables(a), "hit_dice", 0)["group"] = "paladin" },
		"a race name in French":    func(a map[string]any) { row(tables(a), "race_requirements", 0)["race"] = "nain" },
		"an unknown skill":         func(a map[string]any) { row(tables(a), "thief_skill_base", 0)["skill"] = "Pick Pockets" },
		"a text value":             func(a map[string]any) { row(tables(a), "thac0", 0)["rogue"] = "20" },
		"a missing row field":      func(a map[string]any) { delete(row(tables(a), "saving_throws", 0), "breath") },
		"an unknown row field":     func(a map[string]any) { row(tables(a), "backstab", 0)["note"] = "x" },
		"a level over 20":          func(a map[string]any) { row(tables(a), "backstab", 3)["level_max"] = 21 },
		"a chance over 100":        func(a map[string]any) { row(tables(a), "wisdom", 0)["spell_failure"] = 101 },
		"a magic defence below -6": func(a map[string]any) { row(tables(a), "wisdom", 0)["magic_defence"] = -7 },
		"a magic defence above +4": func(a map[string]any) { row(tables(a), "wisdom", 24)["magic_defence"] = 5 },
		"19 THAC0 levels":          func(a map[string]any) { tables(a)["thac0"] = tables(a)["thac0"].([]any)[:19] },
		"8 wizard spell levels": func(a map[string]any) {
			row(tables(a), "wizard_spells", 0)["per_spell_level"] = []any{1, 0, 0, 0, 0, 0, 0, 0}
		},
		"a turning roll over 20": func(a map[string]any) {
			row(tables(a), "turning_undead", 0)["by_level"].([]any)[0] = map[string]any{"roll": 21}
		},
		"an unknown turning cell": func(a map[string]any) {
			row(tables(a), "turning_undead", 0)["by_level"].([]any)[0] = map[string]any{"outcome": "T"}
		},
		"a roll and an outcome": func(a map[string]any) {
			row(tables(a), "turning_undead", 0)["by_level"].([]any)[0] = map[string]any{"roll": 10, "outcome": "turn"}
		},
		"19 turning levels": func(a map[string]any) {
			row(tables(a), "turning_undead", 0)["by_level"] = row(tables(a), "turning_undead", 0)["by_level"].([]any)[1:]
		},
		"an unknown weapon type": func(a map[string]any) { row(tables(a), "rate_of_fire", 0)["weapon_type"] = "Longbow" },
	})
}

// TestSchema_Step11_LevelLimits (ADR 0021 steps 11.1 to 11.3): the racial level limits are 25 rows
// of language-neutral codes and a level of 0 to 20 or none.
func TestSchema_Step11_LevelLimits(t *testing.T) {
	sch := compile(t)
	unlimited := fixtureDoc(t)
	delete(row(tables(unlimited), "race_level_limits", 0), "max_level")
	assert.NoError(t, sch.Validate(unlimited), "a level limit may be absent (no limit)")
	refuseEach(t, map[string]func(map[string]any){
		"no level limits":       func(a map[string]any) { delete(tables(a), "race_level_limits") },
		"a human row":           func(a map[string]any) { row(tables(a), "race_level_limits", 0)["race"] = "human" },
		"a race name in French": func(a map[string]any) { row(tables(a), "race_level_limits", 0)["race"] = "nain" },
		"a class the app lacks": func(a map[string]any) { row(tables(a), "race_level_limits", 0)["class"] = "paladin" },
		"a level over 20":       func(a map[string]any) { row(tables(a), "race_level_limits", 0)["max_level"] = 21 },
		"a negative level":      func(a map[string]any) { row(tables(a), "race_level_limits", 0)["max_level"] = -1 },
		"the book's U as text":  func(a map[string]any) { row(tables(a), "race_level_limits", 0)["max_level"] = "U" },
		"a missing class":       func(a map[string]any) { delete(row(tables(a), "race_level_limits", 0), "class") },
		"an unknown row field":  func(a map[string]any) { row(tables(a), "race_level_limits", 0)["note"] = "x" },
		"24 rows": func(a map[string]any) {
			tables(a)["race_level_limits"] = tables(a)["race_level_limits"].([]any)[1:]
		},
		"26 rows": func(a map[string]any) {
			rows := tables(a)["race_level_limits"].([]any)
			tables(a)["race_level_limits"] = append(rows, rows[0])
		},
	})
}

// TestSchema_Step12_Spheres (ADR 0021 steps 12.1 to 12.3): the sphere access, and every priest
// spell's sphere codes (a wizard spell never has codes).
func TestSchema_Step12_Spheres(t *testing.T) {
	sch := compile(t)
	priest := func(spheres ...any) map[string]any {
		s := spell("p", map[string]any{"en": "x"})
		s["class"] = "Priest"
		s["sphere"] = "Combat"
		if spheres != nil {
			s["spheres"] = spheres
		}
		return s
	}
	assert.NoError(t, sch.Validate(artifact(t, priest("combat"), spell("a", map[string]any{"en": "x"}))), "a priest spell with codes")
	assert.Error(t, sch.Validate(artifact(t, priest())), "a priest spell without codes")
	wizard := spell("a", map[string]any{"en": "x"})
	wizard["spheres"] = []any{"combat"}
	assert.Error(t, sch.Validate(artifact(t, wizard)), "a wizard spell with codes")

	empty := priest()
	empty["spheres"] = []any{}
	for name, p := range map[string]map[string]any{
		"a sphere name, not a code": priest("combat", "Combat"),
		"a sphere twice":            priest("combat", "combat"),
		"no sphere":                 empty,
	} {
		assert.Error(t, sch.Validate(artifact(t, p)), name)
	}
	refuseEach(t, map[string]func(map[string]any){
		"no sphere access":      func(a map[string]any) { delete(tables(a), "sphere_access") },
		"a class the app lacks": func(a map[string]any) { row(tables(a), "sphere_access", 0)["class"] = "druid" },
		"an unknown access":     func(a map[string]any) { row(tables(a), "sphere_access", 0)["access"] = "full" },
		"a missing sphere":      func(a map[string]any) { delete(row(tables(a), "sphere_access", 0), "sphere") },
		"an unknown row field":  func(a map[string]any) { row(tables(a), "sphere_access", 0)["level"] = 3 },
	})
}

// TestSchema_Step13_Equipment (ADR 0021 steps 13.1 and 13.2, ADR 0033 steps 21 to 23): the
// equipment object and Tables 52 and 56; codes and numbers only.
func TestSchema_Step13_Equipment(t *testing.T) {
	equipment := func(a map[string]any) map[string]any {
		e, _ := a["equipment"].(map[string]any)
		return e
	}
	item := func(a map[string]any, list string, i int) map[string]any { return row(equipment(a), list, i) }
	refuseEach(t, map[string]func(map[string]any){
		"a missing list":          func(a map[string]any) { delete(equipment(a), "shields") },
		"an unknown list":         func(a map[string]any) { equipment(a)["helmets"] = []any{} },
		"an empty list":           func(a map[string]any) { equipment(a)["ammunition"] = []any{} },
		"a code with a space":     func(a map[string]any) { item(a, "weapons", 0)["code"] = "battle axe" },
		"a weapon without a name": func(a map[string]any) { delete(item(a, "weapons", 0), "name") },
		"a name text, not a map":  func(a map[string]any) { item(a, "weapons", 0)["name"] = "Battle axe" },
		"a cost":                  func(a map[string]any) { item(a, "weapons", 0)["cost"] = "5 gp" },
		"a size in capitals":      func(a map[string]any) { item(a, "weapons", 0)["size"] = "M" },
		"a kind of 'thrown'":      func(a map[string]any) { item(a, "weapons", 0)["kind"] = "thrown" },
		"a negative speed":        func(a map[string]any) { item(a, "weapons", 0)["speed"] = -1 },
		"two-handed as text":      func(a map[string]any) { item(a, "weapons", 0)["two_handed"] = "yes" },
		"a launcher firing nothing": func(a map[string]any) {
			item(a, "weapons", 2)["fires"] = []any{}
		},
		"a launcher firing a thing twice": func(a map[string]any) {
			item(a, "weapons", 2)["fires"] = []any{"arrow", "arrow"}
		},
		"a damage with a letter":     func(a map[string]any) { item(a, "weapons", 0)["damage_l"] = "d8" },
		"an armour without a row":    func(a map[string]any) { delete(item(a, "armour", 0), "armor_row") },
		"a shield kind in French":    func(a map[string]any) { item(a, "shields", 0)["kind"] = "bouclier" },
		"no equipment table 52":      func(a map[string]any) { delete(tables(a), "weapon_vs_armor") },
		"no equipment table 56":      func(a map[string]any) { delete(tables(a), "initiative_item") },
		"a Table 52 row in French":   func(a map[string]any) { row(tables(a), "weapon_vs_armor", 0)["armor_row"] = "bandes" },
		"a Table 52 cell as text":    func(a map[string]any) { row(tables(a), "weapon_vs_armor", 0)["s"] = "+2" },
		"a Table 52 row missing 'b'": func(a map[string]any) { delete(row(tables(a), "weapon_vs_armor", 0), "b") },
		"a Table 56 kind outside":    func(a map[string]any) { row(tables(a), "initiative_item", 0)["kind"] = "stave" },
		"a Table 56 kind missing":    func(a map[string]any) { delete(row(tables(a), "initiative_item", 0), "kind") },
		"a Table 56 text modifier":   func(a map[string]any) { row(tables(a), "initiative_item", 0)["modifier"] = "+4" },
	})
}

// TestSchema_Step14_Languages (ADR 0021 steps 14.1 and 14.2, ADR 0032 step 4): the languages list
// and the race_languages table; codes, names as language maps, the six races only.
func TestSchema_Step14_Languages(t *testing.T) {
	lang := func(a map[string]any) map[string]any { return a["languages"].([]any)[0].(map[string]any) }
	refuseEach(t, map[string]func(map[string]any){
		"a missing list":         func(a map[string]any) { a["languages"] = []any{} },
		"a code in capitals":     func(a map[string]any) { lang(a)["code"] = "Common" },
		"a name text, not a map": func(a map[string]any) { lang(a)["name"] = "Common" },
		"a name without English": func(a map[string]any) { lang(a)["name"] = map[string]any{"fr": "Commun"} },
		"a family":               func(a map[string]any) { lang(a)["family"] = "x" },
		"no race_languages":      func(a map[string]any) { delete(tables(a), "race_languages") },
		"a race of the book":     func(a map[string]any) { row(tables(a), "race_languages", 0)["race"] = "orc" },
		"native as a word":       func(a map[string]any) { row(tables(a), "race_languages", 0)["native"] = "yes" },
		"a row without a language": func(a map[string]any) {
			delete(row(tables(a), "race_languages", 0), "language")
		},
	})
}

// TestSchema_Step16_Format9ProficiencyDescriptions (ADR 0021 steps 16.1 and 16.4): the 65 rows, each
// {id, description} with both languages in step 1's subset, and nothing else; an earlier format may
// not carry the list (TestSchema_Step17_OnlyFormat10IsValid).
func TestSchema_Step16_Format9ProficiencyDescriptions(t *testing.T) {
	descs := func(a map[string]any) []any { return a["proficiency_descriptions"].([]any) }
	desc := func(a map[string]any, i int) map[string]any {
		return descs(a)[i].(map[string]any)["description"].(map[string]any)
	}
	assert.Len(t, descs(fixtureDoc(t)), 65, "the fixture holds 65 rows")
	refuseEach(t, map[string]func(map[string]any){
		"64 rows": func(a map[string]any) { a["proficiency_descriptions"] = descs(a)[1:] },
		"66 rows": func(a map[string]any) {
			a["proficiency_descriptions"] = append(descs(a), map[string]any{"id": "extra", "description": map[string]any{"en": "x", "fr": "y"}})
		},
		"a row twice":            func(a map[string]any) { descs(a)[1] = descs(a)[0] },
		"no French":              func(a map[string]any) { delete(desc(a, 0), "fr") },
		"no English":             func(a map[string]any) { delete(desc(a, 0), "en") },
		"an empty French":        func(a map[string]any) { desc(a, 0)["fr"] = "" },
		"a third language":       func(a map[string]any) { desc(a, 0)["de"] = "x" },
		"surrounding space":      func(a map[string]any) { desc(a, 0)["en"] = " x" },
		"a heading":              func(a map[string]any) { desc(a, 0)["en"] = "x\n\n# H" },
		"a hard wrap":            func(a map[string]any) { desc(a, 0)["fr"] = "x\ny" },
		"a name beside the id":   func(a map[string]any) { descs(a)[0].(map[string]any)["name"] = "x" },
		"an id in capitals":      func(a map[string]any) { descs(a)[0].(map[string]any)["id"] = "Agriculture" },
		"a row without its id":   func(a map[string]any) { delete(descs(a)[0].(map[string]any), "id") },
		"a row without its text": func(a map[string]any) { delete(descs(a)[0].(map[string]any), "description") },
		"a text, not a map":      func(a map[string]any) { descs(a)[0].(map[string]any)["description"] = "x" },
	})
}

// TestFixture_Step16_ProficiencyDescriptionsAreInventedForEveryId (ADR 0021 steps 16.4 and 7): the
// fixture holds a description in English and French for each id of the testdata proficiencies seed,
// invented placeholder text and never book text, with at least one list and one table.
func TestFixture_Step16_ProficiencyDescriptionsAreInventedForEveryId(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/seeds/proficiencies.json")
	require.NoError(t, err)
	var seed struct {
		Nonweapon []struct {
			ID string `json:"id"`
		} `json:"nonweapon"`
	}
	require.NoError(t, json.Unmarshal(raw, &seed))
	raw, err = os.ReadFile("../../testdata/catalogue.json")
	require.NoError(t, err)
	var cat struct {
		Rows []struct {
			ID          string            `json:"id"`
			Description map[string]string `json:"description"`
		} `json:"proficiency_descriptions"`
	}
	require.NoError(t, json.Unmarshal(raw, &cat))
	require.Len(t, cat.Rows, len(seed.Nonweapon))
	var lists, tables int
	for i, p := range seed.Nonweapon {
		r := cat.Rows[i]
		assert.Equal(t, p.ID, r.ID, "the rows follow the seed's order")
		assert.True(t, strings.HasPrefix(r.Description["en"], "Invented fixture text for "), "%s: invented English only", r.ID)
		assert.True(t, strings.HasPrefix(r.Description["fr"], "Texte inventé pour "), "%s: invented French only", r.ID)
		if strings.Contains(r.Description["en"], "\n- ") {
			lists++
		}
		if strings.Contains(r.Description["en"], "\n| --- |") {
			tables++
		}
	}
	assert.Positive(t, lists, "one description with a bullet list")
	assert.Positive(t, tables, "one description with a table")
}
