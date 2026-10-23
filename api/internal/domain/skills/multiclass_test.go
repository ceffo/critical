package skills_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
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

// ADR 0042 step 15 (Q14): "A dual-class character keeps every pick of the old class; its slots
// are the frozen entries' available at their `left_at_level` plus the new class's gained slots
// (`floor(L / #Levels)`, no second initial count)." The elf, fighter left at 9 (Table 34 warrior:
// weapon 4 + 9/3, non-weapon 3 + 9/3), now a mage (wizard: weapon every 6, non-weapon every 3):
// the mage's levels add their gained slots only, at the mage's rate, and the penalty is the
// active mage's. A level-up of the mage grants the mage's gained slots.
func TestProficiencySlots_Step15_DualClassKeepsAndGains(t *testing.T) {
	book := profBook(t)
	elf := func(mage int) skills.Classes {
		return skills.Classes{{Class: "fighter", Level: 9, Frozen: true}, {Class: "mage", Level: mage}}
	}
	for _, tc := range []struct {
		name                        string
		mage                        int
		weaponAvail, nonweaponAvail int
	}{
		{"mage 1: the fighter's slots only", 1, 7, 6},
		{"mage 7", 7, 7 + 7/6, 6 + 7/3},
		{"mage 10", 10, 7 + 10/6, 6 + 10/3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := skills.FiguresOf(book, elf(tc.mage), 0, skills.Spending{})
			require.True(t, ok)
			assert.Equal(t, 4, got.Weapon.Initial, "the fighter's initial count, once")
			assert.Equal(t, 6, got.Weapon.PerLevels, "the mage's rate")
			assert.Equal(t, tc.weaponAvail, got.Weapon.Available)
			assert.Equal(t, 3, got.Nonweapon.Initial)
			assert.Equal(t, 3, got.Nonweapon.PerLevels)
			assert.Equal(t, tc.nonweaponAvail, got.Nonweapon.Available)
			assert.Equal(t, -5, got.Penalty, "the active mage's penalty")
		})
	}
	t.Run("a level of the mage grants the mage's gained slots", func(t *testing.T) {
		assert.Equal(t, skills.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1}, skills.GrantsOf(book, elf(5), "mage", 6))
		assert.Equal(t, skills.LevelGrants{WeaponSlots: 0, NonweaponSlots: 1}, skills.GrantsOf(book, elf(8), "mage", 9))
	})
}

// ADR 0042 step 16.1 with steps 13 and 15: the weapon picks follow the items'
// rule, so a dual-class's old class adds the types it may pick, before and
// after the crossover (a pick is not a use; wielding warns, itemsvc): the
// elf's kept long-sword pick is allowed and a new one accepted (the code
// before read the active mage alone). Each use follows the limits of the
// class using it: an active blunt_only cleric keeps its limit whatever its
// old fighter allows.
func TestWeaponPick_Step16_1_TheOldClassAllowsThePicks(t *testing.T) {
	book := profBook(t)
	refused := domain.FieldError{Field: "weapon.long_sword", Message: "weapon_not_allowed"}
	for _, tc := range []struct {
		name    string
		classes skills.Classes
		want    *domain.FieldError
	}{
		{"elf at mage 7", skills.Classes{{Class: "fighter", Level: 9, Frozen: true}, {Class: "mage", Level: 7}}, nil},
		{"elf at mage 10", skills.Classes{{Class: "fighter", Level: 9, Frozen: true}, {Class: "mage", Level: 10}}, nil},
		{"cleric 10 after fighter 9", skills.Classes{{Class: "fighter", Level: 9, Frozen: true}, {Class: "cleric", Level: 10}}, &refused},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kept := skills.Sheet{Classes: tc.classes, Intelligence: 12, Picks: weapons("long_sword")}
			slots, _ := skills.Read(book, kept)
			require.Len(t, slots.WeaponPicks, 1)
			assert.Equal(t, tc.want == nil, slots.WeaponPicks[0].Allowed, "the kept pick")

			before := skills.Sheet{Classes: tc.classes, Intelligence: 12, Picks: weapons()}
			_, err := skills.Apply(book, nil, "human", before, skills.Edit{Picks: skills.Picks{Weapon: weapons("long_sword").Weapon}})
			if tc.want == nil {
				require.NoError(t, err, "a new pick")
				return
			}
			assert.Equal(t, *tc.want, fieldErr(t, err))
		})
	}
}

// ADR 0042 step 13: a specialization kept from the old fighter gives the elf
// nothing before the crossover (the active mage's own figures apply) and
// counts past it; the kept pick survives a save either way (the code before
// refused it not_allowed, the mage not specializing).
func TestWeaponProficiency_Step13_AKeptSpecializationCountsPastTheCrossover(t *testing.T) {
	book := profBook(t)
	spec := skills.Picks{Nonweapon: []skills.Pick{}, Weapon: []skills.WeaponPick{{Type: "long_sword", Slots: fighterSlots(t, book, "long_sword")}}}
	elf := func(mage int) skills.Classes {
		return skills.Classes{{Class: "fighter", Level: 9, Frozen: true}, {Class: "mage", Level: mage}}
	}
	assert.Empty(t, skills.WeaponProficiencyOf(book, elf(7), spec).Specialized, "mage 7")
	assert.Empty(t, skills.WeaponProficiencyOf(book, elf(9), spec).Specialized, "mage 9, not above the fighter's 9")
	assert.Equal(t, "long_sword", skills.WeaponProficiencyOf(book, elf(10), spec).Specialized, "mage 10")

	for _, mage := range []int{7, 10} {
		before := skills.Sheet{Classes: elf(mage), Intelligence: 12, Picks: spec}
		got, err := skills.Apply(book, nil, "elf", before, skills.Edit{Picks: skills.Picks{Weapon: spec.Weapon}})
		require.NoError(t, err, "mage %d keeps the pick", mage)
		assert.Equal(t, spec.Weapon, got.Picks.Weapon)
	}
	// A new specialization is still the active class's: refused to the mage.
	before := skills.Sheet{Classes: elf(10), Intelligence: 12, Picks: weapons()}
	_, err := skills.Apply(book, nil, "elf", before, skills.Edit{Picks: skills.Picks{Weapon: spec.Weapon}})
	assert.Equal(t, domain.FieldError{Field: "weapon.long_sword", Message: "not_allowed"}, fieldErr(t, err))
}
