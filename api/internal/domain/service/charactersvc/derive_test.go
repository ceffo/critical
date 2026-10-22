package charactersvc_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	domainskills "github.com/ceffo/critical/api/internal/domain/skills"
)

// kelsen is the seed's half-elf bard as the reader returns it: the entered
// values and the figures his sheet states (level 15, THAC0 13, saves
// 10/8/9/13/9 at 1,293,418 XP), his max HP set for level 15.
func kelsen() *charactersvc.Character {
	return &charactersvc.Character{
		ID: 3, Name: "Kelsen", Str: 12, Dex: 17, Con: 16, Int: 14, Wis: 8, Cha: 16, HPMax: 85, RealizedLevel: 15,
		XPTotal: 1_293_418, Class: "bard", Race: "half_elf",
		Sheet: charactersvc.Sheet{Level: 15, THAC0: 13, Saves: kelsenSaves},
	}
}

var kelsenSaves = charactersvc.Saves{ParaPoison: 10, RodStaffWand: 8, PetriPoly: 9, Breath: 13, Spell: 9}

// rogue13to16 is PHB Table 60's rogue row for levels 13 to 16.
var rogue13to16 = ruletablesvc.SavingThrowRow{
	Group: "rogue", LevelMin: 13, LevelMax: 16, ParaPoison: 10, RodStaffWand: 8, PetriPoly: 9, Breath: 13, Spell: 9,
}

// level15Slots is PHB Table 32 at level 15.
var level15Slots = map[int]int{1: 3, 2: 3, 3: 3, 4: 3, 5: 2}

// expectKelsenRules answers the lookups of a level 15 bard: Table 25 at xp
// is level, then the book's numbers for level 15 (Tables 25, 53, 60 and 32).
func expectKelsenRules(r *mocks.MockRules, xp, level int) {
	r.EXPECT().LevelFor("bard", xp).Return(level, nil).Once()
	r.EXPECT().XPThreshold("bard", 16).Return(1_320_000, nil).Once()
	r.EXPECT().THAC0For("rogue", 15).Return(13, nil).Once()
	r.EXPECT().SavesFor("rogue", 15).Return(rogue13to16, nil).Once()
	r.EXPECT().SpellsPerDay(classDef("bard"), 15, mock.Anything).Return(level15Slots, nil).Once()
}

func kelsenDerived() charactersvc.Derived {
	next := 1_320_000
	return charactersvc.Derived{
		// His one class entry (ADR 0042 step 1) and the features it grants
		// at 15 (ADR 0042 step 12).
		Classes: []charactersvc.EntryDerived{{
			Class: "bard", Group: "rogue", State: classes.EntryActive, XP: 1_293_418, TargetLevel: 15, RealizedLevel: 15,
			NextLevelXP: &next, SpellSlots: level15Slots, Casts: true, LevelUp: charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{}},
		}},
		Features: []charactersvc.FeatureGrant{
			{Kind: classes.KindRogueSkills, Class: "bard", Level: 15}, {Kind: classes.KindSpellbook, Class: "bard", Level: 15},
		},
		XPTotal: 1_293_418, Group: "rogue", TargetLevel: 15, RealizedLevel: 15, NextLevelXP: &next, THAC0: 13, Saves: kelsenSaves,
		SpellSlots: level15Slots,
		LevelUp:    charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{}},
		Spellbook:  &kelsenSpellbook,
		// Nothing worn in these tests: the none column (ADR 0033 step 14).
		SkillColumn: "none",
		SkillSets: []domainskills.SkillSet{{
			Set: "bard", Armor: "none", Budget: []ruletablesvc.BudgetWarning{},
			Points: &ruletablesvc.SkillPointsBudget{Level: 15, First: 20, PerLevel: 15, Total: 230, Left: 230},
		}},
	}
}

// skillSetOf is the derived skill set with code set, nil when the character has none.
func skillSetOf(d charactersvc.Derived, set string) *domainskills.SkillSet {
	for i := range d.SkillSets {
		if d.SkillSets[i].Set == set {
			return &d.SkillSets[i]
		}
	}
	return nil
}

// kelsenSpellbook is a bard's spellbook rules at Intelligence 14 (step 20.1).
var kelsenSpellbook = charactersvc.SpellbookRules{
	ClassMaxSpellLevel: 6, IntMaxSpellLevel: 7, LearnChance: 60, MaxSpellsPerLevel: 9,
}

// rogueHitDice is the rogues' row of the hit dice (PHB Table 25, Hit Dice
// column): d6 through level 10, +2 a level after.
var rogueHitDice = ruletablesvc.HitDiceRow{Group: "rogue", Die: 6, NameLevel: 10, GainAfter: 2}

// Kelsen's sheet matches the rules: every derived value is the sheet's and
// there is no difference to warn about (ADR 0030 Consequences).
func TestDerive_KelsenMatchesHisSheet(t *testing.T) {
	r := newRules(t)
	expectKelsenRules(r, 1_293_418, 15)
	got, err := charactersvc.Derive(kelsen(), nil, "", r)
	require.NoError(t, err)
	assert.Equal(t, kelsenDerived(), got)
}

// ADR 0030 step 3: the level is the class's experience table at the XP
// total, the starting XP plus what the ledger adds (ADR 0015 step 1); the
// next level's threshold comes from the same table, none at level 20.
func TestDerive_Step3_LevelFromTheClassTableAtTheXPTotal(t *testing.T) {
	t.Run("starting XP plus the ledger", func(t *testing.T) {
		r := newRules(t)
		r.EXPECT().LevelFor("bard", 1_293_418+26_582).Return(16, nil).Once()
		r.EXPECT().XPThreshold("bard", 17).Return(1_540_000, nil).Once()
		// The benefits are the realized level's (15), not the target's.
		r.EXPECT().THAC0For("rogue", 15).Return(13, nil).Once()
		r.EXPECT().SavesFor("rogue", 15).Return(rogue13to16, nil).Once()
		r.EXPECT().SpellsPerDay(classDef("bard"), 15, mock.Anything).Return(map[int]int{1: 3}, nil).Once()
		r.EXPECT().HitDiceFor("rogue").Return(rogueHitDice, nil).Once()

		got, err := charactersvc.Derive(kelsen(), []int{26_582}, "", r)
		require.NoError(t, err)
		assert.Equal(t, 1_320_000, got.XPTotal)
		assert.Equal(t, 16, got.TargetLevel)
		assert.Equal(t, 15, got.RealizedLevel)
		require.NotNil(t, got.NextLevelXP)
		assert.Equal(t, 1_540_000, *got.NextLevelXP)
	})
	t.Run("no next level at 20", func(t *testing.T) {
		c := kelsen()
		c.XPTotal, c.RealizedLevel, c.Sheet = 3_000_000, 20, charactersvc.Sheet{Level: 20, THAC0: 11, Saves: kelsenSaves}
		r := newRules(t)
		r.EXPECT().LevelFor("bard", 3_000_000).Return(20, nil).Once()
		r.EXPECT().THAC0For("rogue", 20).Return(11, nil).Once()
		r.EXPECT().SavesFor("rogue", 20).Return(rogue13to16, nil).Once()
		r.EXPECT().SpellsPerDay(classDef("bard"), 20, mock.Anything).Return(map[int]int{1: 4}, nil).Once()

		got, err := charactersvc.Derive(c, nil, "", r)
		require.NoError(t, err)
		assert.Equal(t, 20, got.TargetLevel)
		assert.Nil(t, got.NextLevelXP, "XPThreshold is never asked for level 21")
	})
}

// ADR 0030 step 2: the class group picks the THAC0 and saving throw rows
// (thief and bard rogues, fighter a warrior).
func TestDerive_Step2_TheClassGroupPicksTheRows(t *testing.T) {
	for class, group := range map[string]string{"bard": "rogue", "thief": "rogue", "fighter": "warrior"} {
		t.Run(class, func(t *testing.T) {
			c := kelsen()
			c.Class, c.XPTotal, c.RealizedLevel, c.Sheet = class, 0, 1, charactersvc.Sheet{Level: 1, THAC0: 20}
			r := newRules(t)
			r.EXPECT().LevelFor(class, 0).Return(1, nil).Once()
			r.EXPECT().XPThreshold(class, 2).Return(1_250, nil).Once()
			r.EXPECT().THAC0For(group, 1).Return(20, nil).Once()
			r.EXPECT().SavesFor(group, 1).Return(ruletablesvc.SavingThrowRow{}, nil).Once()
			if class == "bard" {
				r.EXPECT().SpellsPerDay(classDef("bard"), 1, mock.Anything).Return(map[int]int{}, nil).Once()
			}
			if class == "thief" {
				r.EXPECT().RogueSkills(skillSet("thief"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()
				r.EXPECT().BackstabFor(1).Return(2, nil).Once()
			}
			_, err := charactersvc.Derive(c, nil, "", r)
			require.NoError(t, err)
		})
	}
	t.Run("an unknown class is an error", func(t *testing.T) {
		c := kelsen()
		c.Class = "paladin"
		_, err := charactersvc.Derive(c, nil, "", newRules(t))
		require.ErrorContains(t, err, "paladin")
	})
}

// ADR 0030 step 5: the THAC0 is PHB Table 53's for the group at the derived
// level (Table 53 answers 12 here, not the sheet's 13: the derived one wins).
func TestDerive_Step5_THAC0FromTable53(t *testing.T) {
	r := newRules(t)
	r.EXPECT().LevelFor("bard", 1_293_418).Return(15, nil).Once()
	r.EXPECT().XPThreshold("bard", 16).Return(1_320_000, nil).Once()
	r.EXPECT().THAC0For("rogue", 15).Return(12, nil).Once()
	r.EXPECT().SavesFor("rogue", 15).Return(rogue13to16, nil).Once()
	r.EXPECT().SpellsPerDay(classDef("bard"), 15, mock.Anything).Return(level15Slots, nil).Once()
	got, err := charactersvc.Derive(kelsen(), nil, "", r)
	require.NoError(t, err)
	assert.Equal(t, 12, got.THAC0)
}

// ADR 0030 step 6: the five targets are the Table 60 row of the group's level
// band, each category from its own column (distinct values: a crossed column
// fails).
func TestDerive_Step6_SavesFromTable60(t *testing.T) {
	row := ruletablesvc.SavingThrowRow{Group: "rogue", LevelMin: 13, LevelMax: 16, ParaPoison: 1, RodStaffWand: 2, PetriPoly: 3, Breath: 4, Spell: 5}
	r := newRules(t)
	r.EXPECT().LevelFor("bard", 1_293_418).Return(15, nil).Once()
	r.EXPECT().XPThreshold("bard", 16).Return(1_320_000, nil).Once()
	r.EXPECT().THAC0For("rogue", 15).Return(13, nil).Once()
	r.EXPECT().SavesFor("rogue", 15).Return(row, nil).Once()
	r.EXPECT().SpellsPerDay(classDef("bard"), 15, mock.Anything).Return(level15Slots, nil).Once()
	got, err := charactersvc.Derive(kelsen(), nil, "", r)
	require.NoError(t, err)
	assert.Equal(t, charactersvc.Saves{ParaPoison: 1, RodStaffWand: 2, PetriPoly: 3, Breath: 4, Spell: 5}, got.Saves)
}

// ADR 0030 step 8: a bard's slots are PHB Table 32 at the derived level,
// a mage's Table 21; fighters and thieves have none; the cleric's is not
// implemented yet (an error, not an empty list).
func TestDerive_Step8_SpellSlotsByClass(t *testing.T) {
	t.Run("bard: Table 32", func(t *testing.T) {
		r := newRules(t)
		expectKelsenRules(r, 1_293_418, 15)
		got, err := charactersvc.Derive(kelsen(), nil, "", r)
		require.NoError(t, err)
		assert.Equal(t, level15Slots, got.SpellSlots)
	})
	for _, class := range []string{"fighter", "thief"} {
		t.Run(class+": none", func(t *testing.T) {
			c := kelsen()
			c.Class, c.RealizedLevel = class, 14
			r := newRules(t)
			r.EXPECT().LevelFor(class, 1_293_418).Return(14, nil).Once()
			r.EXPECT().XPThreshold(class, 15).Return(1_500_000, nil).Once()
			r.EXPECT().THAC0For(ruleGroup(t, class), 14).Return(7, nil).Once()
			r.EXPECT().SavesFor(ruleGroup(t, class), 14).Return(rogue13to16, nil).Once()
			if class == "thief" {
				r.EXPECT().RogueSkills(skillSet("thief"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()
				r.EXPECT().BackstabFor(14).Return(5, nil).Once()
			}
			got, err := charactersvc.Derive(c, nil, "", r)
			require.NoError(t, err)
			assert.Empty(t, got.SpellSlots)
		})
	}
	t.Run("mage: Table 21", func(t *testing.T) {
		c := kelsen()
		c.Class, c.RealizedLevel = "mage", 13
		r := newRules(t)
		r.EXPECT().LevelFor("mage", 1_293_418).Return(13, nil).Once()
		r.EXPECT().XPThreshold("mage", 14).Return(1_500_000, nil).Once()
		r.EXPECT().THAC0For("wizard", 13).Return(16, nil).Once()
		r.EXPECT().SavesFor("wizard", 13).Return(rogue13to16, nil).Once()
		r.EXPECT().SpellsPerDay(classDef("mage"), 13, mock.Anything).Return(map[int]int{1: 5, 6: 2}, nil).Once()
		got, err := charactersvc.Derive(c, nil, "", r)
		require.NoError(t, err)
		assert.Equal(t, map[int]int{1: 5, 6: 2}, got.SpellSlots)
	})
	t.Run("cleric: Table 24 with the bonus spells at the sheet's Wisdom", func(t *testing.T) {
		c := kelsen()
		c.Class, c.RealizedLevel, c.Wis = "cleric", 13, 16
		r := clericRules(t, 13)
		r.EXPECT().SpellsPerDay(classDef("cleric"), 13, 16).Return(map[int]int{1: 8, 5: 2}, nil).Once()
		r.EXPECT().SpellFailureFor(16).Return(0, nil).Once()
		r.EXPECT().TurningAt(13).Return(nil, nil).Once()
		got, err := charactersvc.Derive(c, nil, "", r)
		require.NoError(t, err)
		assert.Equal(t, map[int]int{1: 8, 5: 2}, got.SpellSlots)
	})
}

// clericRules is a level-lvl cleric's level, THAC0 and saves.
func clericRules(t *testing.T, lvl int) *mocks.MockRules {
	t.Helper()
	r := newRules(t)
	r.EXPECT().LevelFor("cleric", 1_293_418).Return(lvl, nil).Once()
	r.EXPECT().XPThreshold("cleric", lvl+1).Return(1_350_000, nil).Once()
	r.EXPECT().THAC0For("priest", lvl).Return(12, nil).Once()
	r.EXPECT().SavesFor("priest", lvl).Return(rogue13to16, nil).Once()
	return r
}

// ADR 0030 steps 13, 14 and 16 on the derived values: a cleric's slots and
// spell failure follow the sheet's Wisdom and his turning the realized level
// (never the XP target); the other classes read none of it.
func TestDerive_Steps13to16_TheClericsWisdomAndTurning(t *testing.T) {
	turning := []ruletablesvc.TurningTarget{{Undead: "skeleton", HDMin: 1, HDMax: 1, Cell: ruletablesvc.TurningCell{Roll: 7}}}
	c := kelsen()
	c.Class, c.RealizedLevel, c.Wis = "cleric", 2, 9
	r := clericRules(t, 2)
	r.EXPECT().SpellsPerDay(classDef("cleric"), 2, 9).Return(map[int]int{1: 2}, nil).Once()
	r.EXPECT().SpellFailureFor(9).Return(20, nil).Once()
	r.EXPECT().TurningAt(2).Return(turning, nil).Once()
	got, err := charactersvc.Derive(c, nil, "", r)
	require.NoError(t, err)
	assert.Equal(t, 20, got.SpellFailure)
	assert.Equal(t, turning, got.Turning)

	m := kelsen()
	m.Class, m.RealizedLevel = "mage", 13
	rm := newRules(t)
	rm.EXPECT().LevelFor("mage", 1_293_418).Return(13, nil).Once()
	rm.EXPECT().XPThreshold("mage", 14).Return(1_500_000, nil).Once()
	rm.EXPECT().THAC0For("wizard", 13).Return(16, nil).Once()
	rm.EXPECT().SavesFor("wizard", 13).Return(rogue13to16, nil).Once()
	rm.EXPECT().SpellsPerDay(classDef("mage"), 13, mock.Anything).Return(map[int]int{1: 5}, nil).Once()
	got, err = charactersvc.Derive(m, nil, "", rm)
	require.NoError(t, err)
	assert.Zero(t, got.SpellFailure, "a mage has no spell failure")
	assert.Nil(t, got.Turning, "a mage does not turn undead")
}

func ruleGroup(t *testing.T, class string) string {
	t.Helper()
	g, err := cataloguetest.Tables(t).ClassGroup(class)
	require.NoError(t, err)
	return g
}

// ADR 0030 step 22.1: dwarves, gnomes and halflings add their PHB Table 9
// bonus, from the sheet's Constitution, to the saves against Rod/Staff/Wand
// and Spells only; humans, elves, half-elves and a character without a race
// add nothing (and read no table).
func TestDerive_Step22_1_RaceSaveBonus(t *testing.T) {
	for _, tc := range []struct {
		race string
		want charactersvc.Saves
	}{
		{"dwarf", charactersvc.Saves{RodStaffWand: 4, Spell: 4}},
		{"gnome", charactersvc.Saves{RodStaffWand: 4, Spell: 4}},
		{"halfling", charactersvc.Saves{RodStaffWand: 4, Spell: 4}},
		{"human", charactersvc.Saves{}},
		{"elf", charactersvc.Saves{}},
		{"half_elf", charactersvc.Saves{}},
		{"", charactersvc.Saves{}},
	} {
		t.Run("race "+tc.race, func(t *testing.T) {
			c := kelsen()
			c.Race = tc.race
			r := newRules(t)
			expectKelsenRules(r, 1_293_418, 15)
			if tc.want != (charactersvc.Saves{}) {
				r.EXPECT().ConSaveBonusFor(16).Return(4, nil).Once()
			}
			got, err := charactersvc.Derive(c, nil, "", r)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.RaceSaveBonus)
			assert.Equal(t, kelsenSaves, got.Saves, "the bonus never moves the target")
		})
	}
}

// ADR 0030 step 24 (retired): the sheet's figures are not compared with the
// rules. A sheet that disagrees changes nothing, and the rules are read once
// (at the realized level, the mocks refuse any other read): the computed
// values are the ones used.
func TestDerive_Step24_SheetIsNotCompared(t *testing.T) {
	c := kelsen()
	c.Sheet = charactersvc.Sheet{Level: 14, THAC0: 14, Saves: charactersvc.Saves{ParaPoison: 11, RodStaffWand: 8, PetriPoly: 10, Breath: 14, Spell: 10}}
	r := newRules(t)
	expectKelsenRules(r, 1_293_418, 15)
	got, err := charactersvc.Derive(c, nil, "", r)
	require.NoError(t, err)
	assert.Equal(t, 15, got.TargetLevel, "the computed level is used")
	assert.Equal(t, 13, got.THAC0, "the computed THAC0 is used")
	assert.Equal(t, kelsenSaves, got.Saves, "the computed saves are used")
}

// ADR 0030 step 7: a level-up is pending for each level above the realized
// one up to the target level (step 3), lowest first: the group's hit die up
// to its name level (step 7.1), the fixed gain above it (step 7.2).
func TestDerive_Step7_PendingLevelUps(t *testing.T) {
	// A bard realized at level 9 whose XP gives 12: d6 for 10 (the rogues'
	// name level), +2 for 11 and 12.
	t.Run("hit die through the name level, fixed gain after", func(t *testing.T) {
		c := kelsen()
		c.RealizedLevel, c.XPTotal, c.Sheet = 9, 500_000, charactersvc.Sheet{Level: 9, THAC0: 16, Saves: kelsenSaves}
		r := newRules(t)
		r.EXPECT().LevelFor("bard", 500_000+250_000).Return(12, nil).Once()
		r.EXPECT().XPThreshold("bard", 13).Return(880_000, nil).Once()
		r.EXPECT().THAC0For("rogue", 9).Return(16, nil).Once()
		r.EXPECT().SavesFor("rogue", 9).Return(rogue13to16, nil).Once()
		r.EXPECT().SpellsPerDay(classDef("bard"), 9, mock.Anything).Return(map[int]int{1: 3}, nil).Once()
		r.EXPECT().HitDiceFor("rogue").Return(rogueHitDice, nil).Once()
		got, err := charactersvc.Derive(c, []int{250_000}, "", r)
		require.NoError(t, err)
		assert.Equal(t, charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{
			{Class: "bard", Classes: 1, Level: 10, Die: 6},
			{Class: "bard", Classes: 1, Level: 11, FixedGain: 2},
			{Class: "bard", Classes: 1, Level: 12, FixedGain: 2},
		}}, got.LevelUp)
	})
	t.Run("the warriors' row for a fighter", func(t *testing.T) {
		c := kelsen()
		c.Class, c.RealizedLevel, c.XPTotal, c.Sheet = "fighter", 1, 2_000, charactersvc.Sheet{Level: 2, THAC0: 19}
		r := newRules(t)
		r.EXPECT().LevelFor("fighter", 2_000).Return(2, nil).Once()
		r.EXPECT().XPThreshold("fighter", 3).Return(4_000, nil).Once()
		// The benefits are level 1's.
		r.EXPECT().THAC0For("warrior", 1).Return(20, nil).Once()
		r.EXPECT().SavesFor("warrior", 1).Return(ruletablesvc.SavingThrowRow{}, nil).Once()
		r.EXPECT().HitDiceFor("warrior").Return(ruletablesvc.HitDiceRow{Group: "warrior", Die: 10, NameLevel: 9, GainAfter: 3}, nil).Once()
		got, err := charactersvc.Derive(c, nil, "", r)
		require.NoError(t, err)
		assert.Equal(t, []charactersvc.PendingLevel{{Class: "fighter", Classes: 1, Level: 2, Die: 10}}, got.LevelUp.Pending)
	})
	t.Run("none at the target level, and no lookup", func(t *testing.T) {
		r := newRules(t)
		expectKelsenRules(r, 1_293_418, 15)
		got, err := charactersvc.Derive(kelsen(), nil, "", r)
		require.NoError(t, err)
		assert.Equal(t, charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{}}, got.LevelUp)
	})
	// Only the create-time fallback: a stored character's NULL
	// realized_level is resolved in the DAL to its sheet level
	// (COALESCE(realized_level, level) in characterdal's characters.sql), so
	// it never reaches Derive as 0.
	t.Run("no realized level while creating is the target level (step 7.5)", func(t *testing.T) {
		c := kelsen()
		c.RealizedLevel = 0
		r := newRules(t)
		expectKelsenRules(r, 1_293_418, 15)
		got, err := charactersvc.Derive(c, nil, "", r)
		require.NoError(t, err)
		assert.Equal(t, charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{}}, got.LevelUp)
	})
}

// ADR 0030 step 7.4: an XP correction that brings the target level under
// the realized one removes nothing: the benefits stay the realized level's,
// no level is pending and the block flags it, with both levels, for the
// card's warning.
func TestDerive_Step7_4_ATargetUnderTheRealizedLevelIsFlagged(t *testing.T) {
	c := kelsen()
	c.RealizedLevel = 16
	r := newRules(t)
	r.EXPECT().LevelFor("bard", 1_293_418).Return(15, nil).Once()
	r.EXPECT().XPThreshold("bard", 16).Return(1_320_000, nil).Once()
	r.EXPECT().THAC0For("rogue", 16).Return(12, nil).Once()
	r.EXPECT().SavesFor("rogue", 16).Return(rogue13to16, nil).Once()
	r.EXPECT().SpellsPerDay(classDef("bard"), 16, mock.Anything).Return(map[int]int{1: 4}, nil).Once()
	got, err := charactersvc.Derive(c, nil, "", r)
	require.NoError(t, err)
	assert.Equal(t, charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{}, RealizedAboveTarget: true}, got.LevelUp)
	assert.Equal(t, 15, got.TargetLevel)
	assert.Equal(t, 16, got.RealizedLevel)
	assert.Equal(t, 12, got.THAC0, "the realized level's THAC0")
	assert.Equal(t, map[int]int{1: 4}, got.SpellSlots, "the realized level's slots")
}

// A table error stops the derivation, naming the lookup.
func TestDerive_ATableErrorIsAnError(t *testing.T) {
	r := newRules(t)
	boom := errors.New("boom")
	r.EXPECT().LevelFor("bard", 1_293_418).Return(0, boom).Once()
	_, err := charactersvc.Derive(kelsen(), nil, "", r)
	require.ErrorIs(t, err, boom)
}

// int14 is PHB Table 4 at Intelligence 14 (ADR 0030 step 20).
var int14 = ruletablesvc.IntelligenceRow{IntMin: 14, IntMax: 14, MaxSpellLevel: 7, LearnChance: 60, MaxSpells: 9}

// newRules is the rules mock; a class with a spellbook reads Table 4 (step
// 20), which the tests that are not about it leave at Intelligence 14.
func newRules(t *testing.T) *mocks.MockRules {
	r := mocks.NewMockRules(t)
	withDefinitions(t, r)
	r.EXPECT().MagicDefenceFor(mock.Anything).Return(0, nil).Maybe()
	r.EXPECT().SpellImmunitiesFor(mock.Anything).Return(nil, nil).Maybe()
	r.EXPECT().IntelligenceFor(mock.Anything).Return(int14, nil).Maybe()
	// A bard derives its skills (step 18), which the tests that are not
	// about them leave empty.
	r.EXPECT().RogueSkills(skillSet("bard"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	return r
}

// classDef matches the definition of the class id (ADR 0040 step 5), the
// argument of SpellsPerDay.
func classDef(id string) any {
	return mock.MatchedBy(func(d classes.Class) bool { return d.ID == id })
}

// skillSet matches rogue_skills params of the skill set (ADR 0040 step 3),
// the argument of RogueSkills.
func skillSet(set string) any {
	return mock.MatchedBy(func(rs classes.RogueSkills) bool { return rs.Set == set })
}

// withDefinitions backs r's class, group and race lookups (ADR 0040 step 5)
// with the fixture catalogue's definitions.
func withDefinitions(t *testing.T, r *mocks.MockRules) {
	t.Helper()
	tables := cataloguetest.Tables(t)
	r.EXPECT().Definitions().RunAndReturn(tables.Definitions).Maybe()
	r.EXPECT().Class(mock.Anything).RunAndReturn(tables.Class).Maybe()
	r.EXPECT().LevelLimit(mock.Anything, mock.Anything).RunAndReturn(tables.LevelLimit).Maybe()
	r.EXPECT().ClassGroup(mock.Anything).RunAndReturn(tables.ClassGroup).Maybe()
	// A class without a spell_progression has no slots (ADR 0040 step 1.9):
	// the tables answer it, a test expects only the slots of a caster.
	noSpells := mock.MatchedBy(func(d classes.Class) bool { return d.SpellProgression == "" })
	r.EXPECT().SpellsPerDay(noSpells, mock.Anything, mock.Anything).RunAndReturn(tables.SpellsPerDay).Maybe()
}

// Step 20.1: a class with a spellbook (mage, bard) derives the class's
// highest spell level and PHB Table 4 at the sheet's Intelligence; a fighter
// has none.
func TestDerive_Step20_1_SpellbookRules(t *testing.T) {
	int18 := ruletablesvc.IntelligenceRow{IntMin: 18, IntMax: 18, MaxSpellLevel: 9, LearnChance: 85, MaxSpells: 18}
	int19 := ruletablesvc.IntelligenceRow{IntMin: 19, IntMax: 19, MaxSpellLevel: 9, LearnChance: 95, NoMaximum: true}
	for _, tc := range []struct {
		class string
		score int
		row   ruletablesvc.IntelligenceRow
		want  *charactersvc.SpellbookRules
	}{
		{"mage", 18, int18, &charactersvc.SpellbookRules{ClassMaxSpellLevel: 9, IntMaxSpellLevel: 9, LearnChance: 85, MaxSpellsPerLevel: 18}},
		{"mage", 19, int19, &charactersvc.SpellbookRules{ClassMaxSpellLevel: 9, IntMaxSpellLevel: 9, LearnChance: 95, NoMaximum: true}},
		{"bard", 14, int14, &kelsenSpellbook},
		{"fighter", 18, int18, nil},
	} {
		t.Run(tc.class, func(t *testing.T) {
			c := kelsen()
			c.Class, c.Int, c.RealizedLevel = tc.class, tc.score, 13
			r := mocks.NewMockRules(t)
			withDefinitions(t, r)
			r.EXPECT().MagicDefenceFor(mock.Anything).Return(0, nil).Maybe()
			r.EXPECT().SpellImmunitiesFor(mock.Anything).Return(nil, nil).Maybe()
			r.EXPECT().LevelFor(tc.class, mock.Anything).Return(13, nil).Once()
			r.EXPECT().XPThreshold(tc.class, 14).Return(1_500_000, nil).Once()
			r.EXPECT().THAC0For(mock.Anything, 13).Return(16, nil).Once()
			r.EXPECT().SavesFor(mock.Anything, 13).Return(rogue13to16, nil).Once()
			r.EXPECT().SpellsPerDay(classDef("mage"), 13, mock.Anything).Return(map[int]int{1: 5}, nil).Maybe()
			r.EXPECT().SpellsPerDay(classDef("bard"), 13, mock.Anything).Return(map[int]int{1: 3}, nil).Maybe()
			r.EXPECT().RogueSkills(skillSet("bard"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
			if tc.want != nil {
				r.EXPECT().IntelligenceFor(tc.score).Return(tc.row, nil).Once()
			}
			got, err := charactersvc.Derive(c, nil, "", r)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.Spellbook)
		})
	}
}

// ADR 0030 steps 17 and 19: a thief's skills are Tables 26 to 29 on the
// sheet's Dexterity, its race, its entered points and the worn body armour's
// Table 29 column (ADR 0033 step 14: none worn is the none column, a metal
// armour reads the padded or studded column); the budget and the backstab
// multiplier follow the realized level, never the XP's target level (step
// 7). Other classes ask for neither.
func TestDerive_Steps17_19_TheThiefsSkillsBudgetAndBackstab(t *testing.T) {
	skills := []ruletablesvc.SkillPercent{{Skill: "open_locks", Base: 10, Points: 70, Percent: 80}}
	for _, tc := range []struct{ name, worn, asked string }{
		{"the worn armour's column", ruletablesvc.ColumnElvenChain, "elven_chain"},
		{"no armour worn is the none column", "", "none"},
		{"a metal armour reads the worst column", ruletablesvc.ColumnMetal, "padded_studded_leather"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := kelsen()
			c.Class, c.Race, c.Dex, c.XPTotal, c.RealizedLevel = "thief", "elf", 17, 0, 1
			c.SkillPoints = domainskills.SetPoints{"thief": {"open_locks": 70}}
			c.Sheet = charactersvc.Sheet{}
			r := newRules(t)
			// 40,000 XP is level 8 in Table 25: two levels pending; the
			// realized level 1 gives the backstab and the budget.
			r.EXPECT().LevelFor("thief", 40_000).Return(8, nil).Once()
			r.EXPECT().XPThreshold("thief", 9).Return(70_000, nil).Once()
			r.EXPECT().THAC0For("rogue", 1).Return(20, nil).Once()
			r.EXPECT().SavesFor("rogue", mock.Anything).Return(ruletablesvc.SavingThrowRow{}, nil)
			r.EXPECT().THAC0For("rogue", mock.Anything).Return(20, nil).Maybe()
			r.EXPECT().HitDiceFor("rogue").Return(rogueHitDice, nil).Maybe()
			r.EXPECT().RogueSkills(skillSet("thief"), "elf", 17, tc.asked, map[string]int{"open_locks": 70}).Return(skills, nil).Once()
			r.EXPECT().BackstabFor(1).Return(2, nil).Once()
			got, err := charactersvc.Derive(c, []int{40_000}, tc.worn, r)
			require.NoError(t, err)
			thief := skillSetOf(got, "thief")
			require.NotNil(t, thief)
			assert.Len(t, got.SkillSets, 1)
			assert.Equal(t, skills, thief.Skills)
			assert.Equal(t, tc.asked, thief.Armor)
			assert.Equal(t, 2, got.Backstab)
			assert.Equal(t, []ruletablesvc.BudgetWarning{
				{Code: ruletablesvc.BudgetTotal, Points: 70, Limit: 60},
				{Code: ruletablesvc.BudgetPerSkill, Skill: "open_locks", Points: 70, Limit: 30},
			}, thief.Budget, "the budget of level 1, not of level 8")
		})
	}
	t.Run("a bard has neither", func(t *testing.T) {
		r := newRules(t)
		expectKelsenRules(r, 1_293_418, 15)
		got, err := charactersvc.Derive(kelsen(), nil, "", r)
		require.NoError(t, err)
		assert.Nil(t, skillSetOf(got, "thief"))
		assert.Zero(t, got.Backstab)
	})
}

// ADR 0030 step 18: a bard's skills are the rules' on its race, the sheet's
// Dexterity, the entered points and the worn body armour's Table 29 column
// (ADR 0033 step 14: metal is the bard's, none worn the none column), and
// the budget is the bard's (20 + 15 a level after the first, no per-skill
// limit) at the realized level; a bard has no thieving skills or backstab.
func TestDerive_Step18_TheBardsSkillsAndBudget(t *testing.T) {
	skills := []ruletablesvc.SkillPercent{{Skill: "climb_walls", Base: 50, Points: 40, Percent: 90}}
	for _, tc := range []struct{ name, worn, asked string }{
		{"a worn metal armour", ruletablesvc.ColumnMetal, ruletablesvc.ArmorMetal},
		{"no armour worn is the none column", "", "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := kelsen()
			c.Race, c.Dex, c.XPTotal, c.RealizedLevel = "human", 16, 0, 2
			c.SkillPoints = domainskills.SetPoints{"bard": {"climb_walls": 40}}
			c.Sheet = charactersvc.Sheet{}
			r := mocks.NewMockRules(t)
			withDefinitions(t, r)
			r.EXPECT().MagicDefenceFor(mock.Anything).Return(0, nil).Maybe()
			r.EXPECT().SpellImmunitiesFor(mock.Anything).Return(nil, nil).Maybe()
			r.EXPECT().IntelligenceFor(mock.Anything).Return(int14, nil).Maybe()
			// 40,000 XP is level 7 in Table 25; the realized level 2 gives
			// the budget: 20 + 15 = 35.
			r.EXPECT().LevelFor("bard", 40_000).Return(7, nil).Once()
			r.EXPECT().XPThreshold("bard", 8).Return(70_000, nil).Once()
			r.EXPECT().THAC0For("rogue", mock.Anything).Return(20, nil)
			r.EXPECT().SavesFor("rogue", mock.Anything).Return(ruletablesvc.SavingThrowRow{}, nil)
			r.EXPECT().SpellsPerDay(classDef("bard"), mock.Anything, mock.Anything).Return(map[int]int{}, nil)
			r.EXPECT().HitDiceFor("rogue").Return(rogueHitDice, nil).Maybe()
			r.EXPECT().RogueSkills(skillSet("bard"), "human", 16, tc.asked, map[string]int{"climb_walls": 40}).Return(skills, nil).Once()
			got, err := charactersvc.Derive(c, []int{40_000}, tc.worn, r)
			require.NoError(t, err)
			bard := skillSetOf(got, "bard")
			require.NotNil(t, bard)
			assert.Len(t, got.SkillSets, 1)
			assert.Equal(t, skills, bard.Skills)
			assert.Equal(t, tc.asked, bard.Armor)
			assert.Nil(t, skillSetOf(got, "thief"))
			assert.Zero(t, got.Backstab)
			assert.Equal(t, []ruletablesvc.BudgetWarning{
				{Code: ruletablesvc.BudgetTotal, Points: 40, Limit: 35},
			}, bard.Budget, "the bard's budget at level 2, not at level 7, and no per-skill warning")
		})
	}
	t.Run("a thief has no bard skills", func(t *testing.T) {
		c := kelsen()
		c.Class, c.XPTotal, c.RealizedLevel, c.Sheet = "thief", 0, 1, charactersvc.Sheet{}
		r := mocks.NewMockRules(t)
		withDefinitions(t, r)
		r.EXPECT().MagicDefenceFor(mock.Anything).Return(0, nil).Maybe()
		r.EXPECT().SpellImmunitiesFor(mock.Anything).Return(nil, nil).Maybe()
		r.EXPECT().LevelFor("thief", 0).Return(1, nil)
		r.EXPECT().XPThreshold("thief", 2).Return(1_250, nil).Once()
		r.EXPECT().THAC0For("rogue", 1).Return(20, nil)
		r.EXPECT().SavesFor("rogue", 1).Return(ruletablesvc.SavingThrowRow{}, nil)
		r.EXPECT().RogueSkills(skillSet("thief"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()
		r.EXPECT().BackstabFor(1).Return(2, nil).Once()
		got, err := charactersvc.Derive(c, nil, "", r)
		require.NoError(t, err)
		assert.Nil(t, skillSetOf(got, "bard"))
	})
}
