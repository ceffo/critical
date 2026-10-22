package charactersvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// ADR 0042's multi-class steps on the derived sheet, derived on the fixture
// catalogue's real tables (the book's Tables 14 to 25, 53 and 60): a
// multi-class character reads every class entry, where the code before read
// the first one only. Each test lists the entries so that the first one alone
// gives another answer.

// multi is a multi-class character of race with the entries es (each active,
// at its realized level with its starting XP) and the sheet's abilities.
func multi(race string, es ...classes.Entry) *charactersvc.Character {
	return &charactersvc.Character{
		ID: 7, Name: "Multi", Race: race, Class: es[0].Class, RealizedLevel: es[0].RealizedLevel, XPTotal: es[0].StartingXP,
		Str: 16, Dex: 15, Con: 15, Int: 15, Wis: 15, Cha: 12, Classes: es,
	}
}

// entry is an active class entry at level with xp entered.
func entry(class string, level, xp int) classes.Entry {
	return classes.SingleEntry(class, level, xp)
}

// derived is c derived with the awards on the fixture's tables.
func derived(t *testing.T, c *charactersvc.Character, awards ...int) charactersvc.Derived {
	t.Helper()
	d, err := charactersvc.Derive(c, awards, "", cataloguetest.Tables(t))
	require.NoError(t, err)
	return d
}

// entryOf is d's block of the entry of class.
func entryOf(t *testing.T, d charactersvc.Derived, class string) charactersvc.EntryDerived {
	t.Helper()
	e, ok := d.Entry(class)
	require.True(t, ok, "entry %s", class)
	return e
}

// ADR 0042 step 5.1: "for a multi-class character, in equal shares to every
// active entry, even when an entry has reached its race's level limit [...].
// The share of an award is floor(amount / n) for n active entries, the
// remainder lost." A fighter/cleric's 1,001 XP award gives 500 to each; the
// half-elf's cleric at 14 (Table 7's limit) still takes its share.
func TestEntryXP_Step5_1_EvenSharesEvenAtTheLimit(t *testing.T) {
	assert.Equal(t, 500, charactersvc.EntryXP(entry("fighter", 1, 0), []int{1_001}, 2))
	assert.Equal(t, 1_000+333+33, charactersvc.EntryXP(entry("fighter", 1, 1_000), []int{1_000, 100}, 3), "floor per award")

	c := multi("half_elf", entry("fighter", 13, 1_300_000), entry("cleric", 14, 1_400_000))
	d := derived(t, c, 1_001)
	assert.Equal(t, 1_300_500, entryOf(t, d, "fighter").XP)
	assert.Equal(t, 1_400_500, entryOf(t, d, "cleric").XP, "a share still goes to the entry at its race's limit")
	assert.Equal(t, 1_300_500, d.XPTotal, "the character's XP is the first entry's")
}

// ADR 0042 step 5.2: "for a dual-class character, wholly to the active entry;
// a frozen entry's XP never changes."
func TestEntryXP_Step5_2_DualClassAllToTheActiveEntry(t *testing.T) {
	old := classes.Entry{Class: "fighter", RealizedLevel: 9, StartingXP: 250_000, State: classes.EntryFrozen, LeftAtLevel: 9}
	assert.Equal(t, 250_000, charactersvc.EntryXP(old, []int{1_001, 3_000}, 1), "the frozen entry's XP is unchanged")
	assert.Equal(t, 4_001, charactersvc.EntryXP(entry("mage", 1, 0), []int{1_001, 3_000}, 1), "every award whole")
}

// ADR 0042 step 7: "Each active entry has its own target level, from its XP
// and its class's thresholds, and its own pending level-ups [...]. The
// pending levels the server sends carry their class. Several pending levels
// in several entries are all offered, each lowest first within its entry."
// A cleric/fighter whose fighter has a pending level and cleric none; then
// both owing, in entry order. Each pending level carries the number of
// classes, which divides its hit points (step 10.2).
func TestDerive_Step7_PendingLevelUpsPerEntry(t *testing.T) {
	t.Run("the fighter owes a level, the cleric none", func(t *testing.T) {
		// Cleric 5 at 13,000 (Table 23: 5 from 13,000 to 27,499); fighter 4
		// at 8,000 plus half of 16,000 reaches 16,000 (Table 14's 5).
		c := multi("half_elf", entry("cleric", 5, 13_000), entry("fighter", 4, 8_000))
		d := derived(t, c, 16_000)
		cleric, fighter := entryOf(t, d, "cleric"), entryOf(t, d, "fighter")
		assert.Equal(t, 5, cleric.TargetLevel)
		assert.Empty(t, cleric.LevelUp.Pending)
		assert.Equal(t, 5, fighter.TargetLevel)
		want := []charactersvc.PendingLevel{{Class: "fighter", Classes: 2, Level: 5, Die: 10}}
		assert.Equal(t, want, fighter.LevelUp.Pending)
		assert.Equal(t, want, d.LevelUp.Pending, "the character's pending levels are the entries'")
	})
	t.Run("both owe levels: each lowest first, in entry order", func(t *testing.T) {
		c := multi("half_elf", entry("cleric", 3, 6_000), entry("fighter", 3, 8_000))
		d := derived(t, c, 14_000) // 7,000 each: cleric 13,000 (5), fighter 15,000 (4)
		assert.Equal(t, []charactersvc.PendingLevel{
			{Class: "cleric", Classes: 2, Level: 4, Die: 8},
			{Class: "cleric", Classes: 2, Level: 5, Die: 8},
			{Class: "fighter", Classes: 2, Level: 4, Die: 10},
		}, d.LevelUp.Pending)
	})
}

// ADR 0042 step 8: "A multi-class character uses the best (lowest) Table 53
// value among its active entries, each at its realized level." A cleric
// 7/fighter 6 uses the fighter's 15, not the cleric's 16.
func TestTHAC0_Step8_BestOfTheActiveEntries(t *testing.T) {
	c := multi("half_elf", entry("cleric", 7, 55_000), entry("fighter", 6, 32_000))
	assert.Equal(t, 15, derived(t, c).THAC0)
}

// ADR 0042 step 9: "For each of the five categories, the best (lowest) target
// among the active entries at their realized levels [...], category by
// category, so two classes' rows may mix." A mage 11/fighter 9: the fighter's
// 9-10 row is better on paralysation (8), breath (9); the mage's 11-15 row on
// rods (7) and spells (8); petrification is 9 in both.
func TestSaves_Step9_BestPerCategory(t *testing.T) {
	c := multi("half_elf", entry("mage", 11, 375_000), entry("fighter", 9, 250_000))
	assert.Equal(t, charactersvc.Saves{ParaPoison: 8, RodStaffWand: 7, PetriPoly: 9, Breath: 9, Spell: 8}, derived(t, c).Saves)
}

// ADR 0042 step 14: "Each entry whose class has a spell_progression has its
// own slots (its table at its realized level, plus the Wisdom bonus spells
// when its class has wisdom_bonus_spells) [...] and its own caster level, the
// entry's realized level." A fighter 7/cleric 5/mage 6 of Wisdom 15: Table 24
// at 5 with the bonus 2 first and 1 second, Table 21 at 6 without it; the
// fighter none. The character's slots and caster level are the first
// spellcasting entry's, the cleric's 5 (not the fighter's 7).
func TestDerive_Step14_SlotsPerSpellcastingEntry(t *testing.T) {
	c := multi("half_elf", entry("fighter", 7, 64_000), entry("cleric", 5, 13_000), entry("mage", 6, 40_000))
	d := derived(t, c)
	cleric, mage := entryOf(t, d, "cleric"), entryOf(t, d, "mage")
	assert.Equal(t, map[int]int{1: 3 + 2, 2: 3 + 1, 3: 1}, cleric.SpellSlots)
	assert.True(t, cleric.Casts)
	assert.Equal(t, map[int]int{1: 4, 2: 2, 3: 2}, mage.SpellSlots)
	assert.Empty(t, entryOf(t, d, "fighter").SpellSlots)
	assert.Equal(t, cleric.SpellSlots, d.SpellSlots)
	assert.Equal(t, 5, d.CasterLevel())
}

// ADR 0042 step 12 on the sheet: each feature is read through the entry that
// grants it, at that entry's level. A fighter 7/thief 9 backstabs at x4 (the
// thief's 9, Table 26's 9-12), not x3 (the first entry's 7); a fighter/cleric
// turns undead at the cleric's level.
func TestDerive_Step12_FeaturesAtTheGrantingEntrysLevel(t *testing.T) {
	ft := multi("half_elf", entry("fighter", 7, 64_000), entry("thief", 9, 110_000))
	d := derived(t, ft)
	assert.Equal(t, 4, d.Backstab)
	assert.Contains(t, d.Features, charactersvc.FeatureGrant{Kind: classes.KindBackstab, Class: "thief", Level: 9})
	assert.Equal(t, 9, d.FeatureLevel(classes.KindRogueSkills))

	fc := multi("half_elf", entry("fighter", 13, 1_300_000), entry("cleric", 3, 3_000))
	d = derived(t, fc)
	assert.Contains(t, d.Features, charactersvc.FeatureGrant{Kind: classes.KindTurnUndead, Class: "cleric", Level: 3})
	assert.True(t, d.ExceptionalStrength, "the fighter's")
	want, err := cataloguetest.Tables(t).TurningAt(3)
	require.NoError(t, err)
	assert.Equal(t, want, d.Turning, "turning at the cleric's 3, not the fighter's 13")
}

// ADR 0042 step 17 (scope): the party's two multi-class characters derive as
// the steps above say, each value traced to its step. The half-elf
// fighter 13/cleric 14 and the human thief 15/psionicist 15 (the human's
// multi-class under the ruleset's lift, ADR 0041 step 2.2; the psionicist
// entry is stubbed: its level, THAC0 and saves only, its powers are ADR 0043's).
func TestParty_Step17_TheMultiClassCharacters(t *testing.T) {
	t.Run("the half-elf fighter 13/cleric 14", func(t *testing.T) {
		c := multi("half_elf", entry("fighter", 13, 1_300_000), entry("cleric", 14, 1_400_000))
		d := derived(t, c)
		assert.Equal(t, 13, entryOf(t, d, "fighter").TargetLevel)
		assert.Equal(t, 14, entryOf(t, d, "cleric").TargetLevel)
		// Step 8: the warrior's 8 at 13 over the priest's 12 at 14.
		assert.Equal(t, 8, d.THAC0)
		// Step 9: the warrior's 13-14 row (5/7/6/5/8) beats the priest's
		// 13-15 (5/9/8/11/10) in every category.
		assert.Equal(t, charactersvc.Saves{ParaPoison: 5, RodStaffWand: 7, PetriPoly: 6, Breath: 5, Spell: 8}, d.Saves)
		// Step 14: Table 24 at 14 (6/6/6/5/3/2/1) with Wisdom 15's bonus
		// 2 first and 1 second; no 6th or 7th under Wisdom 17 and 18.
		assert.Equal(t, map[int]int{1: 8, 2: 7, 3: 6, 4: 5, 5: 3}, d.SpellSlots)
		assert.Equal(t, 14, d.CasterLevel())
		assert.Empty(t, d.LevelUp.Pending)
	})
	t.Run("the human thief 15/psionicist 15", func(t *testing.T) {
		c := multi("human", entry("thief", 15, 1_200_000), entry("psionicist", 15, 1_600_000))
		d := derived(t, c)
		assert.Equal(t, 15, entryOf(t, d, "thief").TargetLevel)
		assert.Equal(t, 15, entryOf(t, d, "psionicist").TargetLevel)
		// Step 8: both rows give 13 at 15.
		assert.Equal(t, 13, d.THAC0)
		// Step 9, rows mixed: the rogue's 13-16 (10/8/9/13/9) and the
		// psionicist's 13-16 (10/9/7/12/11).
		assert.Equal(t, charactersvc.Saves{ParaPoison: 10, RodStaffWand: 8, PetriPoly: 7, Breath: 12, Spell: 9}, d.Saves)
		// Step 14: neither class casts.
		assert.Empty(t, d.SpellSlots)
		// Step 12: the thief's backstab at 15 (x5).
		assert.Equal(t, 5, d.Backstab)
		assert.Empty(t, d.LevelUp.Pending)
	})
}
