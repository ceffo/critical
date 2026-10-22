package skills_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/skills"
)

// ADR 0042 step 15 (amending ADR 0037 step 3): "the largest initial number of
// slots of the classes, then new slots at the fastest rate. Read per pool,
// each column of Table 34 on its own [...]. The levels gained are counted on
// the level of the entry whose rate is used (the highest such level when
// several share it)." The cleric is listed first: a pool read on the first
// class's row fails.
func TestProficiencySlots_Step15_LargestInitialFastestRate(t *testing.T) {
	book := profBook(t)
	for _, tc := range []struct {
		name                        string
		fighter, cleric             int
		weaponAvail, nonweaponAvail int
	}{
		// Weapon: the warrior's 4 initial, every 3 fighter levels. Non-weapon:
		// the priest's 4 initial; both rows every 3, on the higher level.
		{"both at 1", 1, 1, 4, 4},
		{"both at 3", 3, 3, 5, 5},
		{"both at 4", 4, 4, 5, 5},
		{"both at 12", 12, 12, 8, 8},
		{"fighter 6, cleric 9: the weapon rate on the fighter's level, the shared non-weapon rate on the higher", 6, 9, 6, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cls := skills.Classes{{Class: "cleric", Level: tc.cleric}, {Class: "fighter", Level: tc.fighter}}
			got, ok := skills.FiguresOf(book, cls, 0, skills.Spending{})
			require.True(t, ok)
			assert.Equal(t, 4, got.Weapon.Initial)
			assert.Equal(t, 3, got.Weapon.PerLevels)
			assert.Equal(t, tc.weaponAvail, got.Weapon.Available)
			assert.Equal(t, 4, got.Nonweapon.Initial)
			assert.Equal(t, 3, got.Nonweapon.PerLevels)
			assert.Equal(t, tc.nonweaponAvail, got.Nonweapon.Available)
		})
	}
	t.Run("one class is its own row (ADR 0037 step 3 unchanged)", func(t *testing.T) {
		got, ok := skills.FiguresOf(book, skills.One("cleric", 12), 0, skills.Spending{})
		require.True(t, ok)
		assert.Equal(t, 2+12/4, got.Weapon.Available)
		assert.Equal(t, 4+12/3, got.Nonweapon.Available)
	})
}

// ADR 0042 step 15, Q12: "The non-proficiency penalty (ADR 0037 step 8) is
// the smallest among the active entries' groups": a mage/fighter takes the
// warrior's -2, not the wizard's -5.
func TestNonproficientPenalty_Step15_SmallestOfTheGroups(t *testing.T) {
	book := profBook(t)
	got, ok := skills.FiguresOf(book, skills.Classes{{Class: "mage", Level: 5}, {Class: "fighter", Level: 5}}, 0, skills.Spending{})
	require.True(t, ok)
	assert.Equal(t, -2, got.Penalty)
	assert.Equal(t, -2, got.NonproficientPenalty("long_sword"))
}

// ADR 0042 step 15, Q13: "A non-weapon proficiency costs Table 37's slots when
// one of its groups is among any active entry's Table 38 groups": a
// fighter/cleric pays healing (priest) at its 2 slots and tracking (warrior)
// at its 2, where the fighter alone pays healing 3 (one more, ADR 0037 step 4).
func TestNonweaponCost_Step15_AnyActiveClassesGroups(t *testing.T) {
	book := profBook(t)
	spend := skills.Spending{Picks: picks(p("healing", 2), p("tracking", 2))}
	got, ok := skills.FiguresOf(book, skills.Classes{{Class: "fighter", Level: 3}, {Class: "cleric", Level: 3}}, 0, spend)
	require.True(t, ok)
	require.Len(t, got.Picks, 2)
	assert.Equal(t, 2, got.Picks[0].Cost, "healing is in the cleric's groups")
	assert.Equal(t, 2, got.Picks[1].Cost, "tracking is in the fighter's groups")

	alone, ok := skills.FiguresOf(book, skills.One("fighter", 3), 0, spend)
	require.True(t, ok)
	assert.Equal(t, 3, alone.Picks[0].Cost, "healing is out of the fighter's groups")
}

// ADR 0042 steps 7 and 15 on ADR 0037 step 15: a level of one entry grants
// the combined pools' slots at that entry's new level over its level below.
// A fighter/cleric both at 2 taking fighter 3 gains a weapon slot (the
// warrior's every 3, on the fighter's level) and a non-weapon slot (the
// shared every 3 on the higher level, now 3); taking cleric 3 instead gains
// the non-weapon slot only (the weapon rate is counted on the fighter's 2).
func TestLevelGrants_Step15_TheCombinedPoolsAtTheEntrysLevel(t *testing.T) {
	book := profBook(t)
	cls := skills.Classes{{Class: "fighter", Level: 2}, {Class: "cleric", Level: 2}}
	assert.Equal(t, skills.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1}, skills.GrantsOf(book, cls, "fighter", 3))
	assert.Equal(t, skills.LevelGrants{NonweaponSlots: 1}, skills.GrantsOf(book, cls, "cleric", 3))
}
