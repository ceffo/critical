package catalogue_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

func skillDesc(set, skill, en, fr string) catalogue.SkillDescription {
	t := localized.Text{}
	if en != "" {
		t["en"] = en
	}
	if fr != "" {
		t["fr"] = fr
	}
	return catalogue.SkillDescription{Set: set, Skill: skill, Description: t}
}

// ADR 0021 step 33.3: the import refuses a missing language, a skill no set has and a skill with
// no description, naming the set, the skill and the language; a skill two sets share has a text
// per set; matching sets pass.
func TestCheckSkillDescriptions_Step33_RefusesAMissingLanguageOrPairMismatch(t *testing.T) {
	sets := []catalogue.SkillSet{
		{ID: "thief", Skills: []catalogue.SkillSetSkill{{ID: "pick_pockets"}, {ID: "open_locks"}}},
		{ID: "bard", Skills: []catalogue.SkillSetSkill{{ID: "pick_pockets"}}},
	}
	good := []catalogue.SkillDescription{
		skillDesc("thief", "pick_pockets", "a", "b"), skillDesc("thief", "open_locks", "c", "d"), skillDesc("bard", "pick_pockets", "e", "f"),
	}
	require.NoError(t, catalogue.CheckSkillDescriptions(good, sets))

	tests := []struct {
		name  string
		descs []catalogue.SkillDescription
		want  string
	}{
		{
			"no French",
			[]catalogue.SkillDescription{good[0], skillDesc("thief", "open_locks", "c", ""), good[2]},
			"skill thief/open_locks: the fr description is missing or empty",
		},
		{
			"no English",
			[]catalogue.SkillDescription{good[0], good[1], skillDesc("bard", "pick_pockets", "", "f")},
			"skill bard/pick_pockets: the en description is missing or empty",
		},
		{
			"the bard's text missing though the thief's is there", good[:2],
			"skill bard/pick_pockets: no description in the catalogue",
		},
		{
			"a skill of no set", append([]catalogue.SkillDescription{skillDesc("bard", "open_locks", "x", "y")}, good...),
			"skill bard/open_locks: no such skill in the artifact's skill sets",
		},
		{
			"a set of none", append([]catalogue.SkillDescription{skillDesc("mage", "pick_pockets", "x", "y")}, good...),
			"skill mage/pick_pockets: no such skill in the artifact's skill sets",
		},
		{
			"a pair twice", append([]catalogue.SkillDescription{good[1]}, good...),
			"skill thief/open_locks: the description repeats",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := catalogue.CheckSkillDescriptions(tc.descs, sets)
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}
