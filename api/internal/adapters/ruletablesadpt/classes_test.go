package ruletablesadpt_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// ADR 0040 step 1: the fixture's five classes parse into the registry with
// the fields read so far. Expected values are the PHB's, not the artifact's:
// the groups and prime requisites (chapter 3), the minimums (Table 13, p.25),
// the alignments (chapter 3: the thief any but lawful good, the bard partly
// neutral; the others any) and the XP of levels 5 and 6 (Tables 14, 20, 23
// and 25). A class without a field, or with a per-language name, is refused
// by the format-11 schema.
func TestClassCatalogue_Step1_EveryClassCarriesEveryField(t *testing.T) {
	defs, err := phb(t).Definitions()
	require.NoError(t, err)
	type want struct {
		group    string
		primes   []string
		mins     []classes.AbilityMinimum
		aligns   []string
		xp5, xp6 int
	}
	am := func(a string, m int) classes.AbilityMinimum { return classes.AbilityMinimum{Ability: a, Min: m} }
	page := map[string]want{
		"fighter": {"warrior", []string{"str"}, []classes.AbilityMinimum{am("str", 9)}, classes.Alignments, 16_000, 32_000},
		"mage":    {"wizard", []string{"int"}, []classes.AbilityMinimum{am("int", 9)}, classes.Alignments, 20_000, 40_000},
		"cleric":  {"priest", []string{"wis"}, []classes.AbilityMinimum{am("wis", 9)}, classes.Alignments, 13_000, 27_500},
		"thief":   {"rogue", []string{"dex"}, []classes.AbilityMinimum{am("dex", 9)}, classes.Alignments[1:], 10_000, 20_000},
		"bard": {
			"rogue",
			[]string{"dex", "cha"},
			[]classes.AbilityMinimum{am("dex", 12), am("int", 13), am("cha", 15)},
			[]string{"neutral_good", "lawful_neutral", "true_neutral", "chaotic_neutral", "neutral_evil"},
			10_000, 20_000,
		},
	}
	assert.Equal(t, []string{"fighter", "mage", "cleric", "thief", "bard"}, defs.ClassIDs())
	for id, w := range page {
		c, err := defs.Get(id)
		require.NoError(t, err)
		assert.Equal(t, w.group, c.Group, id)
		assert.Equal(t, w.primes, c.PrimeRequisites, id)
		assert.Equal(t, w.mins, c.AbilityMinimums, id)
		assert.Equal(t, w.aligns, c.Alignments, id)
		assert.Equal(t, w.xp5, c.Experience[4], id)
		assert.Equal(t, w.xp6, c.Experience[5], id)
		assert.NotEmpty(t, c.Name["en"], id)
		assert.NotEmpty(t, c.Name["fr"], id)
	}

	raw, err := os.ReadFile(cataloguetest.Path())
	require.NoError(t, err)
	for name, mutate := range map[string]func(c map[string]any){
		"no prime requisites":              func(c map[string]any) { delete(c, "prime_requisites") },
		"no experience":                    func(c map[string]any) { delete(c, "experience") },
		"a name in a language":             func(c map[string]any) { c["name"] = "Guerrier" },
		"a field the schema does not know": func(c map[string]any) { c["title"] = "Guerrier" },
	} {
		t.Run(name, func(t *testing.T) {
			var doc map[string]any
			require.NoError(t, json.Unmarshal(raw, &doc))
			mutate(doc["classes"].([]any)[0].(map[string]any))
			out, err := json.Marshal(doc)
			require.NoError(t, err)
			_, err = catalogueadpt.ParseArtifact(out)
			require.ErrorContains(t, err, "schema: at '/classes/0")
		})
	}
}

// ADR 0040 step 8: the import refuses to start on a row keyed by a class,
// group or race the definitions do not hold, and on a class whose group has no
// rows, naming the table.
func TestImport_Step8_RefusesARowForAnUnknownClass(t *testing.T) {
	tables := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"a level limit of an unknown class", func(d map[string]any) { classRow(d, levelLimits, 0)["class"] = "paladin" }, `unknown "paladin"`},
		{"a level limit of an unknown race", func(d map[string]any) { classRow(d, levelLimits, 0)["race"] = "orc" }, `unknown race "orc"`},
		{"a hit dice row of an unknown group", func(d map[string]any) { classRow(d, "hit_dice", 0)["group"] = "psionicist" }, "psionicist"},
		{"a save row of an unknown group", func(d map[string]any) { classRow(d, "saving_throws", 0)["group"] = "psionicist" }, "psionicist"},
	}
	for _, tc := range tables {
		t.Run(tc.name, func(t *testing.T) {
			_, err := classTablesOf(t, tc.mutate)
			require.ErrorContains(t, err, tc.want)
		})
	}
	defs := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"a class of an unknown group", func(d map[string]any) {
			d["classes"].([]any)[0].(map[string]any)["group"] = "psionicist"
		}, `class "fighter": unknown group "psionicist"`},
		{"a class twice", func(d map[string]any) {
			d["classes"] = append(d["classes"].([]any), d["classes"].([]any)[0])
		}, `"fighter" twice`},
	}
	for _, tc := range defs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := definitionsOf(t, tc.mutate)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
