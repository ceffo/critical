package catalogueadpt

import (
	"bytes"
	"encoding/json"
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
	c, err := Load(fixture)
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatRuleTables, c.Format)
	assert.Equal(t, "1970-01-01", c.Version)
	require.NotNil(t, c.RuleTables, "the fixture carries the rule tables")
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

// Step 5.2: unknown format, schema violations (the first ones listed) and
// duplicate ids are refused.
func TestParse_Step2_Refusals(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   []string
	}{
		{"unknown format names it", func(d map[string]any) { d["format"] = 4 }, []string{"unknown format 4", "formats 1 and 3"}},
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

// format1 is the fixture as a format-1 artifact: the same spells, no tables.
func format1(d map[string]any) {
	d["format"] = 1
	delete(d, "rule_tables")
}

// ADR 0021 step 9.1: this API reads formats 1 and 3. Format 3 brings its
// rule tables; format 1 has none (its tables are the seeds', step 9.6.1);
// format 2, the retracted stat shape, is refused naming it.
func TestParse_Step9_1_ReadsFormats1And3(t *testing.T) {
	three, err := Parse(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatRuleTables, three.Format)
	require.NotNil(t, three.RuleTables)
	hit, dmg, err := three.RuleTables.StrengthMods(18, 100)
	require.NoError(t, err)
	assert.Equal(t, [2]int{3, 6}, [2]int{hit, dmg}, "PHB Table 1, 18/00")

	one, err := Parse(artifact(t, format1))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatSpells, one.Format)
	assert.Nil(t, one.RuleTables)
	assert.Len(t, one.Spells, len(three.Spells), "the same spells in either format")

	_, err = Parse(artifact(t, func(d map[string]any) { d["format"] = 2 }))
	require.ErrorContains(t, err, "unknown format 2: this API reads formats 1 and 3")
}

// ADR 0021 steps 9.1 and 9.2: format 3 requires the four tables, format 1
// carries none; both are schema refusals naming the format found.
func TestParse_Step9_2_TablesBelongToFormat3Only(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   []string
	}{
		{"format 3 without tables", func(d map[string]any) { delete(d, "rule_tables") }, []string{"format 3 schema", "rule_tables"}},
		{"format 3 missing a table", func(d map[string]any) {
			delete(d["rule_tables"].(map[string]any), "dexterity")
		}, []string{"format 3 schema", "dexterity"}},
		{"format 3 with a text in the tables", func(d map[string]any) {
			d["rule_tables"].(map[string]any)["_source"] = "PHB"
		}, []string{"format 3 schema", "_source"}},
		{"format 1 with tables", func(d map[string]any) { d["format"] = 1 }, []string{"format 1 schema", "rule_tables"}},
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

// ADR 0021 step 9.3: what the schema cannot check (scores covered in order,
// no gap, no overlap, levels once, XP increasing) refuses the artifact,
// naming the table.
func TestParse_Step9_3_RefusesTablesThatBreakTheRulesNamingTheTable(t *testing.T) {
	rows := func(d map[string]any, table string) []any {
		return d["rule_tables"].(map[string]any)[table].([]any)
	}
	row := func(d map[string]any, table string, i int) map[string]any {
		return rows(d, table)[i].(map[string]any)
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"a gap in strength", func(d map[string]any) {
			r := rows(d, "strength")
			d["rule_tables"].(map[string]any)["strength"] = append(r[:1:1], r[2:]...)
		}, "rule_tables: strength table: row 1: score 2 expected, got 3"},
		{"an overlap in dexterity", func(d map[string]any) { row(d, "dexterity", 1)["dex_min"] = 1 }, "rule_tables: dexterity table"},
		{"a bard level twice", func(d map[string]any) { row(d, "bard_spells", 1)["level"] = 1 }, "rule_tables: bard table"},
		{"XP not increasing", func(d map[string]any) { row(d, "bard_experience", 2)["xp"] = 1250 }, "rule_tables: experience table"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(artifact(t, tc.mutate))
			require.ErrorContains(t, err, tc.want)
		})
	}
}
