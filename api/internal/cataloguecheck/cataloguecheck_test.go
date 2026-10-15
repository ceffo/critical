// Package cataloguecheck holds the tests of the spell catalogue artifact's
// JSON Schema (ADR 0021): the schema compiles with the Go engine the importer
// uses, accepts the testdata spells converted to the artifact shape, and
// enforces the description subset.
package cataloguecheck_test

import (
	"encoding/json"
	"os"
	"regexp"
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

func artifact(spells ...any) map[string]any {
	return map[string]any{"format": 1, "version": "2026-10-15", "spells": spells}
}

func withDesc(en string) map[string]any {
	return artifact(spell("a", map[string]any{"en": en}))
}

// TestSchema_TestdataSpellsConvertedToArtifact: every spell of the testdata
// seed, converted to the artifact shape, is valid.
func TestSchema_TestdataSpellsConvertedToArtifact(t *testing.T) {
	sch := compile(t)
	raw, err := os.ReadFile("../../testdata/seeds/spells.json")
	require.NoError(t, err)
	var seeds []map[string]any
	require.NoError(t, json.Unmarshal(raw, &seeds))
	require.NotEmpty(t, seeds)

	ws := regexp.MustCompile(`\s+`)
	var spells []any
	for i, s := range seeds {
		str := func(k string) string { v, _ := s[k].(string); return v }
		class, _ := s["class"].(string)
		level, _ := s["level"].(float64)
		school := str("school")
		if school == "" { // one fixture spell has no school; the artifact requires one
			school = "Unknown"
		}
		m := spell("id_"+strings.Repeat("a", i+1), map[string]any{"en": strings.TrimSpace(ws.ReplaceAllString(str("description"), " "))})
		m["class"], m["level"], m["school"], m["name"] = class, int(level), school, map[string]any{"en": str("name")}
		for _, k := range []string{"casting_time", "range", "duration", "area_of_effect", "components", "saving_throw"} {
			m[k] = str(k)
		}
		if sp := str("sphere"); sp != "" {
			m["sphere"] = sp
		}
		spells = append(spells, m)
	}
	assert.NoError(t, sch.Validate(artifact(spells...)))
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
		assert.NoError(t, sch.Validate(withDesc(d)), "good %q", d)
	}
	for _, d := range bad {
		assert.Error(t, sch.Validate(withDesc(d)), "bad %q", d)
	}
}

func TestSchema_Shape(t *testing.T) {
	sch := compile(t)
	assert.NoError(t, sch.Validate(artifact(spell("a", map[string]any{"en": "x", "fr": "y"}))))
	assert.Error(t, sch.Validate(map[string]any{"format": 2, "version": "2026-10-15", "spells": []any{spell("a", map[string]any{"en": "x"})}}), "unknown format")
	assert.Error(t, sch.Validate(artifact(spell("a", map[string]any{"en": "x", "fr": ""}))), "empty fr")
	assert.Error(t, sch.Validate(artifact(spell("A-b", map[string]any{"en": "x"}))), "bad id")
	extra := spell("a", map[string]any{"en": "x"})
	extra["extra"] = 1
	assert.Error(t, sch.Validate(artifact(extra)), "unknown property")
}
