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
	assert.Equal(t, catalogue.Format, c.Format)
	assert.Equal(t, "1970-01-01", c.Version)
	mm, ok := c.ByID()["magic_missile"]
	require.True(t, ok)
	assert.Equal(t, "Magic Missile", mm.Name)
	assert.Equal(t, "Projectile magique", mm.NameFR)
	assert.NotEmpty(t, mm.DescriptionFR)
	assert.Equal(t, "PHB", mm.Source)
	assert.Equal(t, "60 yds. + 10 yds./level", mm.Range)
	assert.Equal(t, "60 mètres + 10 mètres par niveau", mm.StatsFR.Range)
	assert.Equal(t, "1", mm.CastingTime)
	assert.Empty(t, mm.StatsFR.CastingTime, "a stat without French text is empty, per field")
	armor := c.ByID()["armor"]
	assert.Empty(t, armor.NameFR, "an omitted language is empty")
	assert.Empty(t, armor.DescriptionFR)
	assert.Equal(t, catalogue.StatsFR{}, armor.StatsFR)
	assert.Equal(t, "Divination", c.ByID()["detect_magic_priest"].Sphere)
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
		{"unknown format names it", func(d map[string]any) { d["format"] = 3 }, []string{"unknown format 3", "formats 1 and 2"}},
		{"format 1 with a format 2 stat", func(d map[string]any) {
			toFormat1(d)
			spells(d)[0].(map[string]any)["range"] = map[string]any{"en": "10 yds."}
		}, []string{"schema", "/spells/0/range/en"}},
		{"format 2 with a format 1 stat", func(d map[string]any) {
			spells(d)[0].(map[string]any)["range"] = "10 yds."
		}, []string{"schema", "/spells/0/range"}},
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

// toFormat1 rewrites a format 2 artifact as the builder wrote format 1: the
// six stats are their English strings.
func toFormat1(d map[string]any) {
	d["format"] = 1
	for _, sp := range spells(d) {
		m := sp.(map[string]any)
		for _, k := range statFields {
			m[k] = m[k].(map[string]any)["en"]
		}
	}
}

// A format 1 artifact (stats as English strings) is still read: same
// spells, the English stats, no French stat text, and its format recorded as 1.
func TestParse_ReadsFormat1(t *testing.T) {
	two, err := Parse(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	one, err := Parse(artifact(t, toFormat1))
	require.NoError(t, err)
	assert.Equal(t, catalogue.Format1, one.Format)
	assert.Equal(t, catalogue.Format, two.Format)
	require.Len(t, one.Spells, len(two.Spells))
	for i, s := range two.Spells {
		s.StatsFR = catalogue.StatsFR{}
		assert.Equal(t, s, one.Spells[i])
	}
	assert.NotEmpty(t, two.ByID()["magic_missile"].StatsFR.Range, "the fixture has French stats to lose")
}

func TestParse_NotJSON(t *testing.T) {
	_, err := Parse([]byte("[1, 2]"))
	require.Error(t, err)
}
