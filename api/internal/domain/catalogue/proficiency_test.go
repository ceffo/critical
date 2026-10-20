package catalogue_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

func desc(id, en, fr string) catalogue.ProficiencyDescription {
	t := localized.Text{}
	if en != "" {
		t["en"] = en
	}
	if fr != "" {
		t["fr"] = fr
	}
	return catalogue.ProficiencyDescription{ID: id, Description: t}
}

// ADR 0021 step 16.3: the import refuses a missing language, an id the seeds lack and a seed id
// with no description, naming the id and the language; matching sets pass.
func TestCheckProficiencyDescriptions_Step16_RefusesAMissingLanguageOrIdMismatch(t *testing.T) {
	ids := []string{"tumbling", "tracking"}
	require.NoError(t, catalogue.CheckProficiencyDescriptions(
		[]catalogue.ProficiencyDescription{desc("tracking", "a", "b"), desc("tumbling", "c", "d")}, ids))

	tests := []struct {
		name  string
		descs []catalogue.ProficiencyDescription
		want  string
	}{
		{
			"no French",
			[]catalogue.ProficiencyDescription{desc("tumbling", "c", ""), desc("tracking", "a", "b")},
			"proficiency tumbling: the fr description is missing or empty",
		},
		{
			"no English",
			[]catalogue.ProficiencyDescription{desc("tumbling", "c", "d"), desc("tracking", "", "b")},
			"proficiency tracking: the en description is missing or empty",
		},
		{
			"an id with no description",
			[]catalogue.ProficiencyDescription{desc("tumbling", "c", "d")},
			"proficiency tracking: no description in the catalogue",
		},
		{
			"an unknown id",
			[]catalogue.ProficiencyDescription{desc("tumbling", "c", "d"), desc("tracking", "a", "b"), desc("swordplay", "x", "y")},
			"proficiency swordplay: no such proficiency in seeds/proficiencies.json",
		},
		{
			"an id twice",
			[]catalogue.ProficiencyDescription{desc("tumbling", "c", "d"), desc("tumbling", "c", "d"), desc("tracking", "a", "b")},
			"proficiency tumbling: the description repeats",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := catalogue.CheckProficiencyDescriptions(tc.descs, ids)
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}
