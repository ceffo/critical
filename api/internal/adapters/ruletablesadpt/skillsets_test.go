package ruletablesadpt_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// parseWithDefinitions parses the fixture's tables after change edits the decoded definitions.
func parseWithDefinitions(t *testing.T, change func(d map[string]any)) (*ruletablesvc.Tables, error) {
	t.Helper()
	var d map[string]any
	require.NoError(t, json.Unmarshal(fixtureDefinitions(t), &d))
	change(d)
	raw, err := json.Marshal(d)
	require.NoError(t, err)
	return ruletablesadpt.Parse(fixtureRuleTables(t), fixtureEquipment(t), fixtureLanguages(t), raw)
}

// setOf is the fixture definitions' skill set id in d.
func setOf(d map[string]any, id string) map[string]any {
	for _, s := range d["skill_sets"].([]any) {
		if m := s.(map[string]any); m["id"] == id {
			return m
		}
	}
	return nil
}

func skillEntry(id string, base int) map[string]any {
	return map[string]any{"id": id, "base": base, "armour_free": false}
}

func bardParams(d map[string]any) map[string]any {
	for _, c := range d["classes"].([]any) {
		if m := c.(map[string]any); m["id"] == "bard" {
			return m["features"].([]any)[0].(map[string]any)["params"].(map[string]any)
		}
	}
	return nil
}

// ADR 0040 step 12: a skill set a class names is read from the catalogue, no code change. A test
// set, test_scout, lists two skills with a base each; a class naming it gets those two skills at
// those bases, and none of the Go lists' others.
func TestSkillSets_Step12_ASetComesFromTheCatalogue(t *testing.T) {
	tbl, err := parseWithDefinitions(t, func(d map[string]any) {
		d["skill_sets"] = append(d["skill_sets"].([]any), map[string]any{
			"id": "test_scout", "name": setOf(d, "bard")["name"], "min_percent": 0,
			"skills": []any{skillEntry("detect_noise", 33), skillEntry("climb_walls", 44)},
		})
		bardParams(d)["set"] = "test_scout"
	})
	require.NoError(t, err)
	scout := classes.Class{}
	for _, c := range tblClasses(t, tbl) {
		if c.ID == "bard" {
			scout = c
		}
	}
	f, ok := scout.Lists(classes.KindRogueSkills)
	require.True(t, ok)
	assert.Equal(t, []string{"detect_noise", "climb_walls"}, ruletablesvc.SkillsOf(f.RogueSkills))
	got, err := tbl.RogueSkills(*f.RogueSkills, "", 9, "none", nil)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, 33, got[0].Base)
	assert.Equal(t, 44, got[1].Base)
}

// tblClasses are the classes of the definitions tbl was parsed with.
func tblClasses(t *testing.T, tbl *ruletablesvc.Tables) []classes.Class {
	t.Helper()
	reg, err := tbl.Definitions()
	require.NoError(t, err)
	return reg.Classes()
}

// ADR 0040 step 12: the start-up refuses a rogue_skills naming no skill set, and a skill of a set
// that a shared table does not cover; a Dexterity row for a skill no set lists is refused, not
// ignored (it could never be read, so it is a mistake of the data).
func TestImport_Step12_RefusesABrokenSkillSet(t *testing.T) {
	_, err := parseWithDefinitions(t, func(d map[string]any) { bardParams(d)["set"] = "test_set" })
	require.ErrorContains(t, err, `unknown skill set "test_set"`)

	_, err = parseWithDefinitions(t, func(d map[string]any) {
		s := setOf(d, "bard")
		s["skills"] = append(s["skills"].([]any), skillEntry("test_skill", 1))
	})
	require.ErrorContains(t, err, "skill_race table", "a skill the race table does not cover")

	_, err = parseWith(t, func(rt map[string]any) {
		rt["skill_dexterity"] = append(rows(rt, "skill_dexterity"),
			map[string]any{"dex_min": 1, "dex_max": 25, "skill": "test_skill", "adjustment": 5})
	})
	require.ErrorContains(t, err, `skill_dexterity table: unknown skill "test_skill"`)
}

// ADR 0040 step 12: Table 28 is rows keyed by skill; a thief at Dexterity 18 reads each skill's own
// row, and a skill with no row at a score adjusts by 0.
func TestSkillDexterity_Step12_RowsKeyedBySkill(t *testing.T) {
	got, err := rogueSkills(t, phb(t), "thief", "human", 18, "none", nil)
	require.NoError(t, err)
	assert.Equal(t, []int{10, 15, 5, 10, 10, 0, 0, 0}, dexterities(got))
	bard, err := rogueSkills(t, phb(t), "bard", "human", 9, "none", nil)
	require.NoError(t, err)
	assert.Equal(t, []int{0, 0, -15, 0}, dexterities(bard), "the bard's four skills, Dexterity on pick pockets only")
}

// specParams is the fixture fighter's weapon_specialization params in d.
func specParams(d map[string]any) map[string]any {
	for _, c := range d["classes"].([]any) {
		if m := c.(map[string]any); m["id"] == "fighter" {
			for _, f := range m["features"].([]any) {
				if fm := f.(map[string]any); fm["kind"] == "weapon_specialization" {
					return fm["params"].(map[string]any)
				}
			}
		}
	}
	return nil
}

// ADR 0040 step 13: the start-up refuses a column that is no Table 35 weapon, a weapon type in two
// rows, a rate_of_fire type with no Table 45 row, and a count under 1.
func TestImport_Step13_RefusesBrokenSpecializationParams(t *testing.T) {
	row := func(d map[string]any, i int) map[string]any {
		return specParams(d)["types"].([]any)[i].(map[string]any)
	}
	for name, tc := range map[string]struct {
		change func(d map[string]any)
		want   string
	}{
		"unknown column":     {func(d map[string]any) { specParams(d)["melee_column"] = "test_column" }, `column "test_column"`},
		"unknown row":        {func(d map[string]any) { row(d, 1)["column"] = "test_column" }, `column "test_column"`},
		"a type twice":       {func(d map[string]any) { row(d, 1)["weapon_types"] = []any{"long_bow"} }, `"long_bow" listed twice`},
		"no rate row":        {func(d map[string]any) { row(d, 0)["weapon_types"] = []any{"long_bow", "test_type"} }, `"test_type" keeps its rate of fire`},
		"default under 1":    {func(d map[string]any) { specParams(d)["default_slots"] = 0 }, "default_slots 0 under 1"},
		"type slots under 1": {func(d map[string]any) { row(d, 2)["slots"] = 0 }, "slots 0 under 1"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseWithDefinitions(t, tc.change)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
