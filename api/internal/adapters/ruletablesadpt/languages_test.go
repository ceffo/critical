package ruletablesadpt_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// languagesOf parses the fixture's tables, equipment and languages as a
// current artifact, with mutate applied to the decoded race_languages table
// and languages list first.
func languagesOf(t *testing.T, mutate func(rows, languages *[]any)) (*ruletablesvc.Tables, error) {
	t.Helper()
	var rules map[string]any
	require.NoError(t, json.Unmarshal(fixtureRuleTables(t), &rules))
	var languages []any
	require.NoError(t, json.Unmarshal(fixtureLanguages(t), &languages))
	rows := rules["race_languages"].([]any)
	mutate(&rows, &languages)
	rules["race_languages"] = rows
	rawRules, err := json.Marshal(rules)
	require.NoError(t, err)
	rawLanguages, err := json.Marshal(languages)
	require.NoError(t, err)
	return ruletablesadpt.Parse(rawRules, fixtureEquipment(t), rawLanguages)
}

// ADR 0021 step 14.1 (ADR 0032 step 4): the artifact hands on the language
// catalogue and each race's PHB list, the race's own language marked. The
// expected lists are read from PHB chapter 2, not derived from the artifact.
func TestParse_Step14_1_ReadsFormat8Languages(t *testing.T) {
	tables, err := languagesOf(t, func(*[]any, *[]any) {})
	require.NoError(t, err)
	l, ok := tables.Languages()
	require.True(t, ok)
	assert.True(t, l.Has("elvish"))
	assert.False(t, l.Has("klingon"))
	assert.Equal(t, localizedName("Elvish", "Elfe"), l.All()[2].Name)
	for race, want := range map[string]string{
		"human": "common", "dwarf": "dwarvish", "elf": "elvish", "gnome": "gnomish",
		"half_elf": "common", "halfling": "halfling",
	} {
		assert.Equal(t, want, l.Native(race), race)
		assert.Contains(t, l.RaceList(race), want, "%s: the native language is on its list", race)
	}
	assert.Equal(t, []string{"common", "dwarvish", "gnomish", "goblin", "kobold", "orcish"}, l.RaceList("dwarf"))
	assert.Equal(t, []string{"common", "elvish", "gnomish", "halfling", "goblin", "hobgoblin", "orcish", "gnoll"}, l.RaceList("elf"))
	assert.Equal(t, []string{"common", "dwarvish", "gnomish", "halfling", "goblin", "kobold", "burrowers"}, l.RaceList("gnome"))
	assert.Equal(t, []string{"common", "elvish", "gnomish", "halfling", "goblin", "hobgoblin", "orcish", "gnoll"}, l.RaceList("half_elf"))
	assert.Equal(t, []string{"common", "halfling", "dwarvish", "elvish", "gnomish", "goblin", "orcish"}, l.RaceList("halfling"))
	assert.Equal(t, []string{"common"}, l.RaceList("human"), "a human has no list of the book beyond Common")
}

// ADR 0021 step 14.2: the languages and race_languages are both required.
func TestParse_Step14_2_LanguagesAreRequired(t *testing.T) {
	var rules map[string]any
	require.NoError(t, json.Unmarshal(fixtureRuleTables(t), &rules))
	delete(rules, "race_languages")
	raw, err := json.Marshal(rules)
	require.NoError(t, err)
	_, err = ruletablesadpt.Parse(raw, fixtureEquipment(t), fixtureLanguages(t))
	require.ErrorContains(t, err, "the artifact needs the race_languages table")
	_, err = ruletablesadpt.Parse(fixtureRuleTables(t), fixtureEquipment(t), nil)
	require.ErrorContains(t, err, "the artifact needs the languages list")
}

// ADR 0021 step 14.3: the loader refuses a racial list naming an unknown
// language, and the other rules the schema cannot express; the refusal names
// the race or the language.
func TestParse_Step14_3_RefusesABrokenLanguageCatalogue(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(rows, languages *[]any)
		want   string
	}{
		{"a racial list naming an unknown language", func(rows, _ *[]any) {
			*rows = append(*rows, map[string]any{"race": "dwarf", "language": "draconic"})
		}, `dwarf names the unknown language "draconic"`},
		{"a language twice in a list", func(rows, _ *[]any) {
			*rows = append(*rows, map[string]any{"race": "dwarf", "language": "gnomish"})
		}, `dwarf lists "gnomish" twice`},
		{"a code twice in the catalogue", func(_, languages *[]any) {
			*languages = append(*languages, (*languages)[0])
		}, `code "common" twice`},
		{"a language without an English name", func(_, languages *[]any) {
			(*languages)[0].(map[string]any)["name"] = map[string]any{"fr": "Commun"}
		}, `"common" has no English name`},
		{"a race with two native languages", func(rows, _ *[]any) {
			for _, r := range *rows {
				if m := r.(map[string]any); m["race"] == "dwarf" && m["language"] == "orcish" {
					m["native"] = true
				}
			}
		}, "dwarf has two native languages"},
		{"a race with no native language", func(rows, _ *[]any) {
			kept := (*rows)[:0:0]
			for _, r := range *rows {
				if m := r.(map[string]any); m["race"] != "elf" || m["native"] != true {
					kept = append(kept, r)
				}
			}
			*rows = kept
		}, "elf has no native language"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := languagesOf(t, tc.mutate)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func localizedName(en, fr string) localized.Text { return localized.Text{"en": en, "fr": fr} }
