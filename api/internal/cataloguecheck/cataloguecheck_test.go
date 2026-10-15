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
		"casting_time": en("1"), "range": map[string]any{"en": "10 yards", "fr": "10 mètres"}, "duration": en("1 round"),
		"area_of_effect": en("One creature"), "components": en("V, S"), "saving_throw": en("None"),
		"name": map[string]any{"en": "Spell"}, "description": desc, "source": "PHB",
	}
}

func en(text string) map[string]any { return map[string]any{"en": text} }

func artifact(spells ...any) map[string]any {
	return map[string]any{"format": 2, "version": "2026-10-15", "spells": spells}
}

func withDesc(en string) map[string]any {
	return artifact(spell("a", map[string]any{"en": en}))
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
	Range       map[string]string `json:"range"`
	CastingTime map[string]string `json:"casting_time"`
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
// one has a French stat text next to an English-only one (the per-field
// fallback), one description has a bullet list and one a table; every
// description is invented placeholder text, never book text.
func TestFixture_CoversTheFormat(t *testing.T) {
	var withFR, withoutFR, lists, tables, partialStatsFR int
	for _, s := range readFixture(t) {
		if s.Range["fr"] != "" && s.CastingTime["fr"] == "" {
			partialStatsFR++
		}
		if s.Name["fr"] != "" {
			withFR++
			assert.NotEmpty(t, s.Description["fr"], "%s: a French name comes with a French description", s.ID)
		} else {
			withoutFR++
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
	assert.Positive(t, withFR)
	assert.Positive(t, withoutFR)
	assert.Positive(t, lists, "one description with a bullet list")
	assert.Positive(t, tables, "one description with a table")
	assert.Positive(t, partialStatsFR, "one spell with a French range and an English-only casting time")
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
	for _, f := range []int{1, 3} {
		assert.Error(t, sch.Validate(map[string]any{"format": f, "version": "2026-10-15", "spells": []any{spell("a", map[string]any{"en": "x"})}}), "format %d", f)
	}
	assert.Error(t, sch.Validate(artifact(spell("a", map[string]any{"en": "x", "fr": ""}))), "empty fr")
	assert.Error(t, sch.Validate(artifact(spell("A-b", map[string]any{"en": "x"}))), "bad id")
	// The six stats are localized since format 2: English required, French optional and never empty.
	for name, v := range map[string]any{
		"a format 1 string": "10 yards",
		"no en":             map[string]any{"fr": "10 mètres"},
		"empty fr":          map[string]any{"en": "10 yards", "fr": ""},
		"empty en":          map[string]any{"en": ""},
		"a third language":  map[string]any{"en": "10 yards", "de": "10 Meter"},
	} {
		sp := spell("a", map[string]any{"en": "x"})
		sp["range"] = v
		assert.Error(t, sch.Validate(artifact(sp)), "stat: %s", name)
	}
	extra := spell("a", map[string]any{"en": "x"})
	extra["extra"] = 1
	assert.Error(t, sch.Validate(artifact(extra)), "unknown property")
}
