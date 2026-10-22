package charactersvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// ADR 0042's dual-class steps on the derived sheet, on the fixture
// catalogue's real tables: the party's elf, a fighter left at 9 who became a
// mage. Each test puts the mage on both sides of the crossover (step 13: above
// the fighter's 9), where the code before read the active entry alone.

// frozen is a dual-class's old class, left at level with xp.
func frozen(class string, level, xp int) classes.Entry {
	return classes.Entry{Class: class, RealizedLevel: level, StartingXP: xp, State: classes.EntryFrozen, LeftAtLevel: level}
}

// elf is the party's elf: the fighter left at 9 (250,000 XP), then a mage at
// mageLevel with mageXP.
func elf(mageLevel, mageXP int) *charactersvc.Character {
	c := multi("elf", frozen("fighter", 9, 250_000), entry("mage", mageLevel, mageXP))
	c.Class, c.RealizedLevel, c.XPTotal = "mage", mageLevel, mageXP
	return c
}

// ADR 0042 step 8: "A dual-class character uses the active entry's value while
// its realized level is not above every frozen entry's left_at_level [...].
// After the crossover, the best of the active entry and each frozen entry at
// its left_at_level (Q5)." The wizard's Table 53 is 18 at 7 to 9 and 17 at 10;
// the warrior's at 9 is 12.
func TestTHAC0_Step8_DualClassNewClassUntilTheCrossover(t *testing.T) {
	assert.Equal(t, 18, derived(t, elf(7, 60_000)).THAC0, "mage 7: the mage's")
	assert.Equal(t, 18, derived(t, elf(9, 135_000)).THAC0, "mage 9, equal to the fighter's 9: still the mage's")
	assert.Equal(t, 12, derived(t, elf(10, 250_000)).THAC0, "mage 10: the fighter's 9 is better")
}

// ADR 0042 step 9: "A dual-class character as step 8: the active entry's rows
// until the crossover, then the best per category (Q5)." The wizard's 6-10 row
// is 13/9/11/13/10, the warrior's 9-10 row 8/10/9/9/11: past the crossover
// the fighter's paralysation, petrification and breath, the mage's rods and
// spells.
func TestSaves_Step9_DualClassBestPerCategoryAfterTheCrossover(t *testing.T) {
	mage := charactersvc.Saves{ParaPoison: 13, RodStaffWand: 9, PetriPoly: 11, Breath: 13, Spell: 10}
	assert.Equal(t, mage, derived(t, elf(9, 135_000)).Saves, "before: the mage's row")
	assert.Equal(t, charactersvc.Saves{ParaPoison: 8, RodStaffWand: 9, PetriPoly: 9, Breath: 9, Spell: 10},
		derived(t, elf(10, 250_000)).Saves, "after: the best per category")
}

// ADR 0042 steps 12 and 13: a frozen entry's features are granted at its
// left-at level; before the crossover they are dormant (usable, warning
// dual_class_old_ability), after it they are not. The active mage's never are.
func TestDerive_Step13_TheOldClassIsDormantUntilTheCrossover(t *testing.T) {
	before := derived(t, elf(7, 60_000))
	assert.True(t, entryOf(t, before, "fighter").Dormant)
	assert.False(t, entryOf(t, before, "mage").Dormant)
	assert.Contains(t, before.Features, charactersvc.FeatureGrant{Kind: classes.KindExceptionalStrength, Class: "fighter", Level: 9, Dormant: true})
	assert.Contains(t, before.Features, charactersvc.FeatureGrant{Kind: classes.KindSpellbook, Class: "mage", Level: 7})

	after := derived(t, elf(10, 250_000))
	assert.False(t, entryOf(t, after, "fighter").Dormant)
	assert.Contains(t, after.Features, charactersvc.FeatureGrant{Kind: classes.KindExceptionalStrength, Class: "fighter", Level: 9})
}

// ADR 0042 step 17: the party's elf as entered, the fighter left at 9 and
// the mage above it: both classes' features and the better THAC0 and saves
// apply; the same elf with the mage below the fighter's level: the fighter's
// features are dormant and its numbers do not count.
func TestParty_Step17_TheDualClassElf(t *testing.T) {
	t.Run("mage 10 past the fighter's 9", func(t *testing.T) {
		d := derived(t, elf(10, 250_000))
		fighter, mage := entryOf(t, d, "fighter"), entryOf(t, d, "mage")
		// Step 5.2: the frozen entry's XP as entered, no target, nothing pending.
		assert.Equal(t, 250_000, fighter.XP)
		assert.Equal(t, 0, fighter.TargetLevel)
		assert.Equal(t, 9, fighter.RealizedLevel)
		assert.Equal(t, 10, mage.TargetLevel)
		assert.Equal(t, 12, d.THAC0, "step 8: the fighter's 9")
		assert.Equal(t, charactersvc.Saves{ParaPoison: 8, RodStaffWand: 9, PetriPoly: 9, Breath: 9, Spell: 10}, d.Saves, "step 9")
		// Step 12: both classes' features, none dormant.
		for _, g := range d.Features {
			assert.False(t, g.Dormant, "%s of %s", g.Kind, g.Class)
		}
		assert.True(t, d.ExceptionalStrength, "the fighter's")
		assert.NotNil(t, d.Spellbook, "the mage's")
		// Step 14: the mage's Table 21 at 10.
		assert.Equal(t, map[int]int{1: 4, 2: 4, 3: 3, 4: 2, 5: 2}, d.SpellSlots)
		assert.Equal(t, 10, d.CasterLevel())
		assert.Empty(t, d.LevelUp.Pending)
	})
	t.Run("mage 7 under the fighter's 9", func(t *testing.T) {
		d := derived(t, elf(7, 60_000))
		assert.Equal(t, 18, d.THAC0, "step 8: the mage's alone")
		assert.Equal(t, charactersvc.Saves{ParaPoison: 13, RodStaffWand: 9, PetriPoly: 11, Breath: 13, Spell: 10}, d.Saves, "step 9")
		for _, g := range d.Features {
			assert.Equal(t, g.Class == "fighter", g.Dormant, "%s of %s", g.Kind, g.Class)
		}
		assert.Equal(t, 7, d.CasterLevel())
	})
}

// ADR 0042 step 11 on the pending levels: "While the active entry's level is
// not above every frozen entry's left_at_level, a level-up of it gains no hit
// points [...]. From the first level above it, ADR 0030 step 7.1 and 7.2 for
// the new class." The elf's mage at 7 with mage 10's XP owes 8 and 9 without
// hit points (no die, no fixed gain) and 10 with the wizard's d4.
func TestDerive_Step11_NoHitPointsUntilTheCrossover(t *testing.T) {
	d := derived(t, elf(7, 250_000))
	assert.Equal(t, []charactersvc.PendingLevel{
		{Class: "mage", Classes: 1, Level: 8, NoHitPoints: true},
		{Class: "mage", Classes: 1, Level: 9, NoHitPoints: true},
		{Class: "mage", Classes: 1, Level: 10, Die: 4},
	}, d.LevelUp.Pending)
}

// ADR 0042 step 15 (Q14) on the sheet: the proficiency pools read the old
// class at its left-at level, frozen (its slots kept, its rate and penalty
// not combined), beside the new class at its realized level
// (skills.TestProficiencySlots_Step15_DualClassKeepsAndGains has the pools).
func TestProficiencyClasses_Step15_TheOldClassKeptAtItsLeftAtLevel(t *testing.T) {
	c := elf(7, 60_000)
	c.Derived = derived(t, c)
	assert.Equal(t, skills.Classes{
		{Class: "fighter", Level: 9, Frozen: true},
		{Class: "mage", Level: 7},
	}, c.ProficiencyClasses())
}
