package saves_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/saves"
)

// figure has a distinct value everywhere, so a term read from the wrong
// field or category shows.
func figure() saves.Inputs {
	return saves.Inputs{
		Saves:        saves.Saves{ParaPoison: 11, RodStaffWand: 12, PetriPoly: 13, Breath: 14, Spell: 15},
		Race:         saves.Saves{RodStaffWand: 3, Spell: 4},
		RaceID:       "dwarf",
		MagicDefence: 2,
		Wis:          16,
		Class: []saves.ClassBonus{
			{Class: "monk", Against: classes.SaveAgainstMindAffecting, Bonus: 5},
			{Class: "other", Against: "fire", Bonus: 6},
		},
	}
}

// ADR 0046 step 3.3 (ADR 0030 step 6): the target is the category's Table
// 60 value, whatever the terms.
func TestTarget_TheCategorysValue(t *testing.T) {
	for i, cat := range saves.Categories {
		got, err := saves.Target(figure(), cat)
		require.NoError(t, err, cat)
		assert.Equal(t, 11+i, got, cat)
	}
}

// ADR 0046 step 3.3: a code that is no category is refused as invalid input.
func TestTarget_UnknownCategoryRefused(t *testing.T) {
	_, err := saves.Target(figure(), "poison")
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Contains(t, err.Error(), `unknown save category "poison"`)
	require.ErrorIs(t, saves.Check(""), domain.ErrInvalidInput)
	for _, cat := range saves.Categories {
		require.NoError(t, saves.Check(cat), cat)
	}
}

// ADR 0006 step 9.4 and ADR 0043 step 3: the Wisdom term and the class term
// against the mind-affecting tag come only on a flagged save, in any
// category, after the race's; a grant against another tag never comes.
func TestSaves_Step3_TermsWisdomOnlyWhenFlagged(t *testing.T) {
	race := saves.Term{Code: saves.TermRace, Source: "dwarf", Value: 4}
	wis := saves.Term{Code: saves.TermWisdom, Source: "16", Value: 2}
	class := saves.Term{Code: saves.TermClass, Source: "monk", Value: 5}

	assert.Equal(t, []saves.Term{race}, saves.Terms(figure(), "spell", saves.Flags{}))
	assert.Equal(t, []saves.Term{race, wis, class}, saves.Terms(figure(), "spell", saves.Flags{MindAffecting: true}))
	// No race bonus on breath: the flagged terms alone.
	assert.Nil(t, saves.Terms(figure(), "breath", saves.Flags{}))
	assert.Equal(t, []saves.Term{wis, class}, saves.Terms(figure(), "breath", saves.Flags{MindAffecting: true}))
}

// ADR 0046 step 3.3: a zero race bonus or Wisdom adjustment adds no term.
func TestTerms_ZeroValuesDropped(t *testing.T) {
	f := saves.Inputs{Saves: figure().Saves, RaceID: "human", Wis: 12}
	assert.Nil(t, saves.Terms(f, "spell", saves.Flags{MindAffecting: true}))
}
