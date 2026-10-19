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

// artifact is a valid format 3 document around the given spells: the fixture's
// rule tables stand in for the required ones.
func artifact(t *testing.T, spells ...any) map[string]any {
	t.Helper()
	return map[string]any{"format": 3, "version": "2026-10-15", "spells": spells, "rule_tables": ruleTables(t)}
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

// ruleTables is the fixture's four PHB rule tables, the format-3 shape (ADR
// 0021 step 9.7).
func ruleTables(t *testing.T) map[string]any {
	t.Helper()
	tables := allTables(t)
	for k := range tables {
		if k != "strength" && k != "dexterity" && k != "bard_spells" && k != "bard_experience" {
			delete(tables, k)
		}
	}
	return tables
}

// format4Tables is the fixture's rule_tables without the racial level
// limits, the format-4 shape (ADR 0021 step 10).
func format4Tables(t *testing.T) map[string]any {
	t.Helper()
	tables := format5Tables(t)
	delete(tables, "race_level_limits")
	return tables
}

// format5Tables is the fixture's rule_tables without the sphere access, the
// format-5 shape (ADR 0021 step 11).
func format5Tables(t *testing.T) map[string]any {
	t.Helper()
	tables := format6Tables(t)
	delete(tables, "sphere_access")
	return tables
}

// format6Tables is the fixture's rule_tables without Tables 52 and 56, the
// format-6 shape (ADR 0021 step 12).
func format6Tables(t *testing.T) map[string]any {
	t.Helper()
	tables := allTables(t)
	delete(tables, "weapon_vs_armor")
	delete(tables, "initiative_item")
	return tables
}

// allTables reads the fixture's rule_tables as format 7 has them (the four
// PHB rule tables, the class and race tables, the racial level limits, the
// sphere access and Tables 52 and 56, ADR 0021 steps 10 to 13): without the
// race_languages table of format 8.
func allTables(t *testing.T) map[string]any {
	t.Helper()
	tables := format8Tables(t)
	delete(tables, "race_languages")
	return tables
}

// format8Tables reads the fixture's rule_tables whole (format 8, ADR 0021
// step 14).
func format8Tables(t *testing.T) map[string]any {
	t.Helper()
	f, err := os.Open("../../testdata/catalogue.json")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	doc, err := jsonschema.UnmarshalJSON(f)
	require.NoError(t, err)
	artifact, ok := doc.(map[string]any)
	require.True(t, ok)
	tables, ok := artifact["rule_tables"].(map[string]any)
	require.True(t, ok, "the fixture carries rule_tables")
	return tables
}

func format3(t *testing.T, tables any) map[string]any {
	a := artifact(t, spell("a", map[string]any{"en": "x"}))
	a["rule_tables"] = tables
	return a
}

// mutate returns a copy of the tables with one change made to a deep copy.
func mutate(t *testing.T, change func(map[string]any)) map[string]any {
	t.Helper()
	raw, err := json.Marshal(ruleTables(t))
	require.NoError(t, err)
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
	require.NoError(t, err)
	tables, ok := doc.(map[string]any)
	require.True(t, ok)
	change(tables)
	return tables
}

func row(tables map[string]any, table string, i int) map[string]any {
	rows, _ := tables[table].([]any)
	r, _ := rows[i].(map[string]any)
	return r
}

// TestSchema_Step9_Format3RuleTables (ADR 0021 steps 9.1 to 9.3): format 3 is
// the spells plus the four PHB rule tables, numbers only; format 1 is retired
// (step 9.6.3) and format 2 is never valid.
func TestSchema_Step9_Format3RuleTables(t *testing.T) {
	sch := compile(t)
	assert.NoError(t, sch.Validate(format3(t, ruleTables(t))), "format 3 with the four tables")
	assert.NoError(t, sch.Validate(format3(t, mutate(t, func(map[string]any) {}))), "an unchanged copy")

	format1 := format3(t, ruleTables(t))
	format1["format"] = 1
	assert.Error(t, sch.Validate(format1), "format 1 is retired")

	format1NoTables := artifact(t, spell("a", map[string]any{"en": "x"}))
	format1NoTables["format"] = 1
	delete(format1NoTables, "rule_tables")
	assert.Error(t, sch.Validate(format1NoTables), "format 1 without tables, the retired shape")

	noTables := artifact(t, spell("a", map[string]any{"en": "x"}))
	delete(noTables, "rule_tables")
	assert.Error(t, sch.Validate(noTables), "format 3 requires the rule tables")

	format2 := format3(t, ruleTables(t))
	format2["format"] = 2
	assert.Error(t, sch.Validate(format2), "format 2 is never valid")

	refused := map[string]func(map[string]any){
		"a missing table":       func(m map[string]any) { delete(m, "bard_experience") },
		"an unknown table":      func(m map[string]any) { m["crit"] = []any{} },
		"a text in the tables":  func(m map[string]any) { m["_source"] = "PHB" },
		"an unknown row field":  func(m map[string]any) { row(m, "strength", 0)["note"] = "x" },
		"a missing row field":   func(m map[string]any) { delete(row(m, "dexterity", 0), "missile") },
		"a text value":          func(m map[string]any) { row(m, "strength", 0)["hit"] = "-5" },
		"a fractional value":    func(m map[string]any) { row(m, "dexterity", 0)["reaction"] = 0.5 },
		"a score above 25":      func(m map[string]any) { row(m, "strength", 0)["str_max"] = 26 },
		"a percentile over 100": func(m map[string]any) { row(m, "strength", 13)["exc_max"] = 101 },
		"a bard level over 20":  func(m map[string]any) { row(m, "bard_spells", 0)["level"] = 21 },
		"five spell levels": func(m map[string]any) {
			row(m, "bard_spells", 0)["per_spell_level"] = []any{0, 0, 0, 0, 0}
		},
		"a negative spell count": func(m map[string]any) {
			row(m, "bard_spells", 0)["per_spell_level"] = []any{-1, 0, 0, 0, 0, 0}
		},
		"negative XP": func(m map[string]any) { row(m, "bard_experience", 0)["xp"] = -1 },
		"21 bard levels": func(m map[string]any) {
			rows, _ := m["bard_spells"].([]any)
			m["bard_spells"] = append(rows, rows[0])
		},
		"19 experience levels": func(m map[string]any) {
			rows, _ := m["bard_experience"].([]any)
			m["bard_experience"] = rows[:19]
		},
	}
	for name, change := range refused {
		assert.Error(t, sch.Validate(format3(t, mutate(t, change))), name)
	}
}

// TestSchema_Step10_Format4ClassTables (ADR 0021 steps 10.1 to 10.3): format
// 4 is format 3 plus every class and race table, numbers and language-neutral
// codes only; format 3 may not carry them.
func TestSchema_Step10_Format4ClassTables(t *testing.T) {
	sch := compile(t)
	format4 := func(tables any) map[string]any {
		a := format3(t, tables)
		a["format"] = 4
		return a
	}
	assert.NoError(t, sch.Validate(format4(format4Tables(t))), "format 4 with every table")
	assert.Error(t, sch.Validate(format4(ruleTables(t))), "format 4 without the class tables")
	assert.Error(t, sch.Validate(format3(t, format4Tables(t))), "format 3 with the class tables")

	edit := func(change func(map[string]any)) map[string]any {
		tables := format4Tables(t)
		raw, err := json.Marshal(tables)
		require.NoError(t, err)
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
		require.NoError(t, err)
		out, _ := doc.(map[string]any)
		change(out)
		return out
	}
	refused := map[string]func(map[string]any){
		"a missing table":       func(m map[string]any) { delete(m, "thac0") },
		"an unknown class code": func(m map[string]any) { row(m, "hit_dice", 0)["group"] = "paladin" },
		"a race name in French": func(m map[string]any) { row(m, "race_requirements", 0)["race"] = "nain" },
		"an unknown skill":      func(m map[string]any) { row(m, "thief_skill_base", 0)["skill"] = "Pick Pockets" },
		"a text value":          func(m map[string]any) { row(m, "thac0", 0)["rogue"] = "20" },
		"a missing row field":   func(m map[string]any) { delete(row(m, "saving_throws", 0), "breath") },
		"an unknown row field":  func(m map[string]any) { row(m, "backstab", 0)["note"] = "x" },
		"a level over 20":       func(m map[string]any) { row(m, "backstab", 3)["level_max"] = 21 },
		"a chance over 100":     func(m map[string]any) { row(m, "wisdom", 0)["spell_failure"] = 101 },
		"19 THAC0 levels":       func(m map[string]any) { m["thac0"] = m["thac0"].([]any)[:19] },
		"8 wizard spell levels": func(m map[string]any) { row(m, "wizard_spells", 0)["per_spell_level"] = []any{1, 0, 0, 0, 0, 0, 0, 0} },
		"a turning roll over 20": func(m map[string]any) {
			row(m, "turning_undead", 0)["by_level"].([]any)[0] = map[string]any{"roll": 21}
		},
		"an unknown turning cell": func(m map[string]any) {
			row(m, "turning_undead", 0)["by_level"].([]any)[0] = map[string]any{"outcome": "T"}
		},
		"a roll and an outcome": func(m map[string]any) {
			row(m, "turning_undead", 0)["by_level"].([]any)[0] = map[string]any{"roll": 10, "outcome": "turn"}
		},
		"19 turning levels": func(m map[string]any) {
			row(m, "turning_undead", 0)["by_level"] = row(m, "turning_undead", 0)["by_level"].([]any)[1:]
		},
		"an unknown weapon type": func(m map[string]any) { row(m, "rate_of_fire", 0)["weapon_type"] = "Longbow" },
	}
	for name, change := range refused {
		assert.Error(t, sch.Validate(format4(edit(change))), name)
	}
}

// TestSchema_Step11_Format5LevelLimits (ADR 0021 steps 11.1 to 11.3): format
// 5 is format 4 plus the racial level limits, 25 rows of language-neutral
// codes and a level of 0 to 20 or none; formats 3 and 4 may not carry them.
func TestSchema_Step11_Format5LevelLimits(t *testing.T) {
	sch := compile(t)
	format5 := func(tables any) map[string]any {
		a := format3(t, tables)
		a["format"] = 5
		return a
	}
	assert.NoError(t, sch.Validate(format5(format5Tables(t))), "format 5 with every table")
	assert.Error(t, sch.Validate(format5(format4Tables(t))), "format 5 without the level limits")
	assert.Error(t, sch.Validate(format3(t, format5Tables(t))), "format 3 with the level limits")
	four := format5(format5Tables(t))
	four["format"] = 4
	assert.Error(t, sch.Validate(four), "format 4 with the level limits")
	onlyLimits := ruleTables(t)
	onlyLimits["race_level_limits"] = format5Tables(t)["race_level_limits"]
	assert.Error(t, sch.Validate(format5(onlyLimits)), "format 5 without the class tables")
	assert.Error(t, sch.Validate(format3(t, onlyLimits)), "format 3 with only the level limits")

	edit := func(change func(map[string]any)) map[string]any {
		tables := format5Tables(t)
		change(tables)
		return tables
	}
	unlimited := edit(func(m map[string]any) { delete(row(m, "race_level_limits", 0), "max_level") })
	assert.NoError(t, sch.Validate(format5(unlimited)), "a level limit may be absent (no limit)")
	refused := map[string]func(map[string]any){
		"a human row":           func(m map[string]any) { row(m, "race_level_limits", 0)["race"] = "human" },
		"a race name in French": func(m map[string]any) { row(m, "race_level_limits", 0)["race"] = "nain" },
		"a class the app lacks": func(m map[string]any) { row(m, "race_level_limits", 0)["class"] = "paladin" },
		"a level over 20":       func(m map[string]any) { row(m, "race_level_limits", 0)["max_level"] = 21 },
		"a negative level":      func(m map[string]any) { row(m, "race_level_limits", 0)["max_level"] = -1 },
		"the book's U as text":  func(m map[string]any) { row(m, "race_level_limits", 0)["max_level"] = "U" },
		"a missing class":       func(m map[string]any) { delete(row(m, "race_level_limits", 0), "class") },
		"an unknown row field":  func(m map[string]any) { row(m, "race_level_limits", 0)["note"] = "x" },
		"24 rows":               func(m map[string]any) { m["race_level_limits"] = m["race_level_limits"].([]any)[1:] },
		"26 rows": func(m map[string]any) {
			rows := m["race_level_limits"].([]any)
			m["race_level_limits"] = append(rows, rows[0])
		},
	}
	for name, change := range refused {
		assert.Error(t, sch.Validate(format5(edit(change))), name)
	}
}

// TestSchema_Step12_Format6Spheres (ADR 0021 steps 12.1 to 12.3): format 6 is
// format 5 plus the sphere access and every priest spell's sphere codes;
// formats before 6 may carry neither, and a wizard spell never has codes.
func TestSchema_Step12_Format6Spheres(t *testing.T) {
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
	doc := func(format int, tables any, spells ...any) map[string]any {
		a := format3(t, tables)
		a["format"] = format
		a["spells"] = spells
		return a
	}
	assert.NoError(t, sch.Validate(doc(6, format6Tables(t), priest("combat"), spell("a", map[string]any{"en": "x"}))), "format 6 with every table")
	assert.Error(t, sch.Validate(doc(6, format5Tables(t), priest("combat"))), "format 6 without the sphere access")
	assert.Error(t, sch.Validate(doc(6, format6Tables(t), priest())), "format 6 with a priest spell without codes")
	wizard := spell("a", map[string]any{"en": "x"})
	wizard["spheres"] = []any{"combat"}
	assert.Error(t, sch.Validate(doc(6, format6Tables(t), wizard)), "format 6 with a wizard spell with codes")
	assert.Error(t, sch.Validate(doc(5, format6Tables(t), priest())), "format 5 with the sphere access")
	assert.Error(t, sch.Validate(doc(5, format5Tables(t), priest("combat"))), "format 5 with sphere codes")
	assert.NoError(t, sch.Validate(doc(5, format5Tables(t), priest())), "format 5 as before")

	empty := priest()
	empty["spheres"] = []any{}
	for name, p := range map[string]map[string]any{
		"a sphere name, not a code": priest("combat", "Combat"),
		"a sphere twice":            priest("combat", "combat"),
		"no sphere":                 empty,
	} {
		assert.Error(t, sch.Validate(doc(6, format6Tables(t), p)), name)
	}
	editAccess := func(change func(map[string]any)) map[string]any {
		tables := format6Tables(t)
		change(row(tables, "sphere_access", 0))
		return tables
	}
	for name, change := range map[string]func(map[string]any){
		"a class the app lacks": func(r map[string]any) { r["class"] = "druid" },
		"an unknown access":     func(r map[string]any) { r["access"] = "full" },
		"a missing sphere":      func(r map[string]any) { delete(r, "sphere") },
		"an unknown row field":  func(r map[string]any) { r["level"] = 3 },
	} {
		assert.Error(t, sch.Validate(doc(6, editAccess(change), priest("combat"))), name)
	}
}

// TestSchema_Step13_Format7Equipment (ADR 0021 steps 13.1 and 13.2, ADR 0033
// steps 21 to 23): format 7 is format 6 plus the equipment object and Tables 52
// and 56; formats before 7 may carry none of them; codes and numbers only.
func TestSchema_Step13_Format7Equipment(t *testing.T) {
	sch := compile(t)
	raw, err := os.ReadFile("../../testdata/catalogue.json")
	require.NoError(t, err)
	fixtureEquipment := func() map[string]any {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
		require.NoError(t, err)
		return doc.(map[string]any)["equipment"].(map[string]any)
	}
	doc := func(format int, tables, equipment any) map[string]any {
		a := format3(t, tables)
		a["format"] = format
		if equipment != nil {
			a["equipment"] = equipment
		}
		return a
	}
	assert.NoError(t, sch.Validate(doc(7, allTables(t), fixtureEquipment())), "format 7 with every table and the equipment")
	assert.Error(t, sch.Validate(doc(7, allTables(t), nil)), "format 7 without the equipment")
	assert.Error(t, sch.Validate(doc(7, format6Tables(t), fixtureEquipment())), "format 7 without Tables 52 and 56")
	assert.Error(t, sch.Validate(doc(6, format6Tables(t), fixtureEquipment())), "format 6 with the equipment")
	assert.Error(t, sch.Validate(doc(6, allTables(t), nil)), "format 6 with Tables 52 and 56")
	assert.NoError(t, sch.Validate(doc(6, format6Tables(t), nil)), "format 6 as before")

	edit := func(change func(map[string]any)) map[string]any {
		e := fixtureEquipment()
		change(e)
		return e
	}
	item := func(e map[string]any, list string, i int) map[string]any { return row(e, list, i) }
	refusedEquipment := map[string]func(map[string]any){
		"a missing list":          func(e map[string]any) { delete(e, "shields") },
		"an unknown list":         func(e map[string]any) { e["helmets"] = []any{} },
		"an empty list":           func(e map[string]any) { e["ammunition"] = []any{} },
		"a code with a space":     func(e map[string]any) { item(e, "weapons", 0)["code"] = "battle axe" },
		"a weapon without a name": func(e map[string]any) { delete(item(e, "weapons", 0), "name") },
		"a name text, not a map":  func(e map[string]any) { item(e, "weapons", 0)["name"] = "Battle axe" },
		"a cost":                  func(e map[string]any) { item(e, "weapons", 0)["cost"] = "5 gp" },
		"a size in capitals":      func(e map[string]any) { item(e, "weapons", 0)["size"] = "M" },
		"a kind of 'thrown'":      func(e map[string]any) { item(e, "weapons", 0)["kind"] = "thrown" },
		"a negative speed":        func(e map[string]any) { item(e, "weapons", 0)["speed"] = -1 },
		"two-handed as text":      func(e map[string]any) { item(e, "weapons", 0)["two_handed"] = "yes" },
		"a launcher firing nothing": func(e map[string]any) {
			item(e, "weapons", 2)["fires"] = []any{}
		},
		"a launcher firing a thing twice": func(e map[string]any) {
			item(e, "weapons", 2)["fires"] = []any{"arrow", "arrow"}
		},
		"a damage with a letter":  func(e map[string]any) { item(e, "weapons", 0)["damage_l"] = "d8" },
		"an armour without a row": func(e map[string]any) { delete(item(e, "armour", 0), "armor_row") },
		"a shield kind in French": func(e map[string]any) { item(e, "shields", 0)["kind"] = "bouclier" },
	}
	for name, change := range refusedEquipment {
		assert.Error(t, sch.Validate(doc(7, allTables(t), edit(change))), name)
	}
	editTables := func(change func(map[string]any)) map[string]any {
		tables := allTables(t)
		change(tables)
		return tables
	}
	refusedTables := map[string]func(map[string]any){
		"a Table 52 row in French":   func(m map[string]any) { row(m, "weapon_vs_armor", 0)["armor_row"] = "bandes" },
		"a Table 52 cell as text":    func(m map[string]any) { row(m, "weapon_vs_armor", 0)["s"] = "+2" },
		"a Table 52 row missing 'b'": func(m map[string]any) { delete(row(m, "weapon_vs_armor", 0), "b") },
		"a Table 56 kind outside":    func(m map[string]any) { row(m, "initiative_item", 0)["kind"] = "stave" },
		"a Table 56 kind missing":    func(m map[string]any) { delete(row(m, "initiative_item", 0), "kind") },
		"a Table 56 text modifier":   func(m map[string]any) { row(m, "initiative_item", 0)["modifier"] = "+4" },
	}
	for name, change := range refusedTables {
		assert.Error(t, sch.Validate(doc(7, editTables(change), fixtureEquipment())), name)
	}
}

// TestSchema_Step14_Format8Languages (ADR 0021 steps 14.1 and 14.2, ADR 0032
// step 4): format 8 is format 7 plus the languages list and the
// race_languages table; formats before 8 may carry neither; codes, names as
// language maps, the six races only.
func TestSchema_Step14_Format8Languages(t *testing.T) {
	sch := compile(t)
	raw, err := os.ReadFile("../../testdata/catalogue.json")
	require.NoError(t, err)
	fixture := func() map[string]any {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
		require.NoError(t, err)
		return doc.(map[string]any)
	}
	languages := func() any { return fixture()["languages"] }
	equipment := func() any { return fixture()["equipment"] }
	doc := func(format int, tables, langs any) map[string]any {
		a := format3(t, tables)
		a["format"] = format
		a["equipment"] = equipment()
		if langs != nil {
			a["languages"] = langs
		}
		return a
	}
	assert.NoError(t, sch.Validate(doc(8, format8Tables(t), languages())), "format 8 with the languages and the table")
	assert.Error(t, sch.Validate(doc(8, format8Tables(t), nil)), "format 8 without the languages")
	assert.Error(t, sch.Validate(doc(8, allTables(t), languages())), "format 8 without race_languages")
	assert.Error(t, sch.Validate(doc(7, format8Tables(t), nil)), "format 7 with race_languages")
	assert.Error(t, sch.Validate(doc(7, allTables(t), languages())), "format 7 with the languages")
	assert.NoError(t, sch.Validate(doc(7, allTables(t), nil)), "format 7 as before")

	refused := map[string]func(map[string]any){
		"a missing list":     func(a map[string]any) { a["languages"] = []any{} },
		"a code in capitals": func(a map[string]any) { a["languages"].([]any)[0].(map[string]any)["code"] = "Common" },
		"a name text, not a map": func(a map[string]any) {
			a["languages"].([]any)[0].(map[string]any)["name"] = "Common"
		},
		"a name without English": func(a map[string]any) {
			a["languages"].([]any)[0].(map[string]any)["name"] = map[string]any{"fr": "Commun"}
		},
		"a family":           func(a map[string]any) { a["languages"].([]any)[0].(map[string]any)["family"] = "x" },
		"a race of the book": func(a map[string]any) { row(a["rule_tables"].(map[string]any), "race_languages", 0)["race"] = "orc" },
		"native as a word":   func(a map[string]any) { row(a["rule_tables"].(map[string]any), "race_languages", 0)["native"] = "yes" },
		"a row without a language": func(a map[string]any) {
			delete(row(a["rule_tables"].(map[string]any), "race_languages", 0), "language")
		},
	}
	for name, change := range refused {
		a := doc(8, format8Tables(t), languages())
		change(a)
		assert.Error(t, sch.Validate(a), name)
	}
}
