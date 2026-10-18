package catalogueadpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

const fixture = "../../../testdata/catalogue.json"

// The embedded schema is the committed one, byte for byte.
func TestEmbeddedSchemaIsTheCommittedOne(t *testing.T) {
	committed, err := os.ReadFile("../../../../schemas/catalogue.schema.json")
	require.NoError(t, err)
	assert.True(t, bytes.Equal(committed, schemaJSON), "copy schemas/catalogue.schema.json to api/internal/adapters/catalogueadpt/")
}

func TestLoad_Fixture(t *testing.T) {
	a, err := LoadArtifact(fixture)
	require.NoError(t, err)
	c := a.Catalogue
	assert.Equal(t, catalogue.FormatClassTables, c.Format)
	assert.Equal(t, "1970-01-01", c.Version)
	assert.NotEmpty(t, a.RuleTables, "the fixture carries the rule tables")
	mm, ok := c.ByID()["magic_missile"]
	require.True(t, ok)
	assert.Equal(t, localized.Text{"en": "Magic Missile", "fr": "Projectile magique"}, mm.Name)
	assert.NotEmpty(t, mm.Description["fr"])
	assert.Equal(t, "PHB", mm.Source)
	assert.Equal(t, localized.Text{"en": "60 yds. + 10 yds./level"}, mm.Range, "the plain texts are the English books'")
	armor := c.ByID()["armor"]
	assert.Equal(t, localized.Text{"en": "Armor"}, armor.Name, "an omitted language has no entry")
	assert.NotContains(t, armor.Description, "fr")
	assert.Equal(t, localized.Text{"en": "Divination"}, c.ByID()["detect_magic_priest"].Sphere)
	assert.Empty(t, armor.Sphere, "no sphere: an empty text")
}

// Step 5.1: no file -> an error naming the path and `just catalogue`.
func TestLoad_Step1_MissingFileNamesThePathAndTheRecipe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.json")
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
	assert.Contains(t, err.Error(), "just catalogue")
}

func artifact(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	raw, err := os.ReadFile(fixture)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	mutate(doc)
	out, err := json.Marshal(doc)
	require.NoError(t, err)
	return out
}

func spells(doc map[string]any) []any { return doc["spells"].([]any) }

// classTables are the keys format 4 adds to rule_tables (ADR 0021 step 10).
var classTables = []string{
	"experience", "hit_dice", "constitution", "intelligence", "wisdom", "con_save_bonus", "race_requirements",
	"race_adjustments", "thac0", "saving_throws", "warrior_attacks", "specialist_attacks", "rate_of_fire",
	"wizard_spells", "priest_spells", "priest_spell_wisdom", "thief_skill_base", "thief_skill_race",
	"thief_skill_dexterity", "thief_skill_armor", "bard_armor_penalty", "bard_skill_base", "backstab",
	"turning_undead",
}

// asFormat3 turns the format-4 fixture into a format-3 artifact: the same
// spells and the four PHB rule tables of step 9.
func asFormat3(d map[string]any) {
	d["format"] = 3
	for _, k := range classTables {
		delete(d["rule_tables"].(map[string]any), k)
	}
}

// Step 5.2: unknown format, schema violations (the first ones listed) and
// duplicate ids are refused.
func TestParse_Step2_Refusals(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   []string
	}{
		{"unknown format names it", func(d map[string]any) { d["format"] = 5 }, []string{"unknown format 5", "formats 3 and 4", "just catalogue"}},
		{"missing format", func(d map[string]any) { delete(d, "format") }, []string{"unknown format none"}},
		{"schema violation lists the offence", func(d map[string]any) {
			spells(d)[0].(map[string]any)["level"] = 12
		}, []string{"schema", "/spells/0/level"}},
		{"several offences, capped", func(d map[string]any) {
			for _, s := range spells(d) {
				s.(map[string]any)["class"] = "Bard"
			}
		}, []string{"/spells/0/class", "more"}},
		{"duplicate id", func(d map[string]any) {
			d["spells"] = append(spells(d), spells(d)[0], spells(d)[1])
		}, []string{"duplicate spell ids: alarm, armor"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(artifact(t, tc.mutate))
			require.Error(t, err)
			for _, w := range tc.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}
}

func TestParse_NotJSON(t *testing.T) {
	_, err := Parse([]byte("[1, 2]"))
	require.Error(t, err)
}

// ADR 0021 step 9.1: format 3 hands on its rule tables as written
// (ruletablesadpt parses them).
func TestParse_Step9_1_ReadsFormat3(t *testing.T) {
	three, err := ParseArtifact(artifact(t, asFormat3))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatRuleTables, three.Catalogue.Format)
	assert.NotEmpty(t, three.Catalogue.Spells)
	var tables map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(three.RuleTables, &tables))
	assert.ElementsMatch(t, []string{"strength", "dexterity", "bard_spells", "bard_experience"}, keys(tables))
}

// ADR 0021 step 10.1: format 4 hands on the four PHB rule tables and the
// class and race tables as written (ruletablesadpt parses them).
func TestParse_Step10_1_ReadsFormat4(t *testing.T) {
	four, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatClassTables, four.Catalogue.Format)
	var tables map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(four.RuleTables, &tables))
	assert.ElementsMatch(t, append([]string{"strength", "dexterity", "bard_spells", "bard_experience"}, classTables...), keys(tables))
}

// ADR 0021 step 9.6.3: format 1 is retired. A format-1 artifact (spells
// only, its tables in the seeds) is refused like format 2, the retracted stat
// shape, and any other unknown format: the refusal names the format found,
// the one this API reads and `just catalogue`, before the schema is checked.
func TestParse_Step9_6_3_RefusesTheRetiredFormat1(t *testing.T) {
	for _, format := range []int{1, 2} {
		_, err := Parse(artifact(t, func(d map[string]any) {
			d["format"] = format
			delete(d, "rule_tables")
		}))
		require.ErrorContains(t, err, fmt.Sprintf(
			"unknown format %d: this API reads formats 3 and 4; run `just catalogue` to fetch a current artifact", format))
	}
}

// ADR 0021 steps 9.1, 9.2 and 10.2: format 3 requires the four tables and
// nothing else in them, format 4 every class and race table too; a schema
// refusal names the format found.
func TestParse_Step9_2_TablesBelongToFormat3Only(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   []string
	}{
		{"format 3 without tables", func(d map[string]any) { asFormat3(d); delete(d, "rule_tables") }, []string{"format 3 schema", "rule_tables"}},
		{"format 3 missing a table", func(d map[string]any) {
			asFormat3(d)
			delete(d["rule_tables"].(map[string]any), "dexterity")
		}, []string{"format 3 schema", "dexterity"}},
		{"format 3 with a text in the tables", func(d map[string]any) {
			asFormat3(d)
			d["rule_tables"].(map[string]any)["_source"] = "PHB"
		}, []string{"format 3 schema", "_source"}},
		{"format 3 with a class table", func(d map[string]any) {
			d["format"] = 3
			for _, k := range classTables[1:] {
				delete(d["rule_tables"].(map[string]any), k)
			}
		}, []string{"format 3 schema", "/rule_tables/experience"}},
		{"format 4 missing a class table", func(d map[string]any) {
			delete(d["rule_tables"].(map[string]any), "turning_undead")
		}, []string{"format 4 schema", "turning_undead"}},
		{"format 4 with a class name in a language", func(d map[string]any) {
			d["rule_tables"].(map[string]any)["hit_dice"].([]any)[0].(map[string]any)["group"] = "guerrier"
		}, []string{"format 4 schema", "hit_dice/0/group"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(artifact(t, tc.mutate))
			require.Error(t, err)
			for _, w := range tc.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
