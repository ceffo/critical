package saves_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/saves"
)

// ADR 0046 step 3.1: every category code names its own field, in the order
// of the one list, and a code that is none has no value.
func TestOf_EveryCategoryNamesItsOwnField(t *testing.T) {
	s := saves.Saves{ParaPoison: 1, RodStaffWand: 2, PetriPoly: 3, Breath: 4, Spell: 5}
	assert.Equal(t, classes.SaveCategories, saves.Categories)
	for i, cat := range saves.Categories {
		v, ok := s.Of(cat)
		assert.True(t, ok, cat)
		assert.Equal(t, i+1, v, cat)
	}
	_, ok := s.Of("poison")
	assert.False(t, ok)
}

// ADR 0042 step 9: category by category the lowest target, two rows mixing;
// one row is itself, none the zero value.
func TestBest_TheLowestPerCategory(t *testing.T) {
	warrior := saves.Saves{ParaPoison: 11, RodStaffWand: 13, PetriPoly: 12, Breath: 13, Spell: 14}
	wizard := saves.Saves{ParaPoison: 14, RodStaffWand: 11, PetriPoly: 13, Breath: 15, Spell: 12}
	assert.Equal(t, saves.Saves{ParaPoison: 11, RodStaffWand: 11, PetriPoly: 12, Breath: 13, Spell: 12}, saves.Best(warrior, wizard))
	assert.Equal(t, saves.Best(warrior, wizard), saves.Best(wizard, warrior))
	assert.Equal(t, warrior, saves.Best(warrior))
	assert.Equal(t, saves.Saves{}, saves.Best())
}

// ADR 0030 step 22.1: the bonus on each of the trait's categories only; a
// code that is no category adds nothing.
func TestRaceBonus_OnTheTraitsCategories(t *testing.T) {
	assert.Equal(t, saves.Saves{RodStaffWand: 4, Spell: 4}, saves.RaceBonus([]string{"rod_staff_wand", "spell"}, 4))
	assert.Equal(t, saves.Saves{}, saves.RaceBonus([]string{"poison"}, 4))
	assert.Equal(t, saves.Saves{}, saves.RaceBonus(nil, 4))
}

// ADR 0043 step 3: per effect tag the best bonus, the first grant's on a tie,
// never the sum of two classes' bonuses; no grant is nil.
func TestClassBonuses_TheBestPerTagNeverSummed(t *testing.T) {
	b := func(class, against string, bonus int) saves.ClassBonus {
		return saves.ClassBonus{Class: class, Against: against, Bonus: bonus}
	}
	for _, tc := range []struct {
		name string
		in   []saves.ClassBonus
		want []saves.ClassBonus
	}{
		{"a tie keeps the first entry's, not the sum", []saves.ClassBonus{b("a", "mind", 2), b("b", "mind", 2)}, []saves.ClassBonus{b("a", "mind", 2)}},
		{"the better bonus wins, whichever entry has it", []saves.ClassBonus{b("a", "mind", 2), b("b", "mind", 3)}, []saves.ClassBonus{b("b", "mind", 3)}},
		{"the first's stays when it is the better", []saves.ClassBonus{b("a", "mind", 3), b("b", "mind", 2)}, []saves.ClassBonus{b("a", "mind", 3)}},
		{"two tags each keep their own", []saves.ClassBonus{b("a", "mind", 2), b("b", "poison", 1)}, []saves.ClassBonus{b("a", "mind", 2), b("b", "poison", 1)}},
		{"no grant", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, saves.ClassBonuses(tc.in))
		})
	}
}
