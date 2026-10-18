package charactersvc_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// kelsen is the seed's half-elf bard as the reader returns it: the entered
// values and the figures his sheet states (level 15, THAC0 13, saves
// 10/8/9/13/9 at 1,293,418 XP).
func kelsen() *charactersvc.Character {
	return &charactersvc.Character{
		ID: 3, Name: "Kelsen", Str: 12, Dex: 17, Con: 16, Int: 14, Wis: 8, Cha: 16, HPMax: 85,
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
	r.EXPECT().BardSpellsPerDay(15).Return(level15Slots, nil).Once()
}

func kelsenDerived() charactersvc.Derived {
	next := 1_320_000
	return charactersvc.Derived{
		XPTotal: 1_293_418, Level: 15, NextLevelXP: &next, THAC0: 13, Saves: kelsenSaves,
		SpellSlots: level15Slots, SheetDifferences: []charactersvc.SheetDifference{},
	}
}

// Kelsen's sheet matches the rules: every derived value is the sheet's and
// there is no difference to warn about (ADR 0030 Consequences).
func TestDerive_KelsenMatchesHisSheet(t *testing.T) {
	r := mocks.NewMockRules(t)
	expectKelsenRules(r, 1_293_418, 15)
	got, err := charactersvc.Derive(kelsen(), 0, r)
	require.NoError(t, err)
	assert.Equal(t, kelsenDerived(), got)
}

// ADR 0030 step 3: the level is the class's experience table at the XP
// total, the starting XP plus what the ledger adds (ADR 0015 step 1); the
// next level's threshold comes from the same table, none at level 20.
func TestDerive_Step3_LevelFromTheClassTableAtTheXPTotal(t *testing.T) {
	t.Run("starting XP plus the ledger", func(t *testing.T) {
		r := mocks.NewMockRules(t)
		r.EXPECT().LevelFor("bard", 1_293_418+26_582).Return(16, nil).Once()
		r.EXPECT().XPThreshold("bard", 17).Return(1_540_000, nil).Once()
		r.EXPECT().THAC0For("rogue", 16).Return(13, nil).Once()
		r.EXPECT().SavesFor("rogue", 16).Return(rogue13to16, nil).Once()
		r.EXPECT().BardSpellsPerDay(16).Return(map[int]int{1: 4}, nil).Once()
		// The sheet comparison reads the rules at the sheet's own XP.
		r.EXPECT().LevelFor("bard", 1_293_418).Return(15, nil).Once()
		r.EXPECT().THAC0For("rogue", 15).Return(13, nil).Once()
		r.EXPECT().SavesFor("rogue", 15).Return(rogue13to16, nil).Once()

		got, err := charactersvc.Derive(kelsen(), 26_582, r)
		require.NoError(t, err)
		assert.Equal(t, 1_320_000, got.XPTotal)
		assert.Equal(t, 16, got.Level)
		require.NotNil(t, got.NextLevelXP)
		assert.Equal(t, 1_540_000, *got.NextLevelXP)
	})
	t.Run("no next level at 20", func(t *testing.T) {
		c := kelsen()
		c.XPTotal, c.Sheet = 3_000_000, charactersvc.Sheet{Level: 20, THAC0: 11, Saves: kelsenSaves}
		r := mocks.NewMockRules(t)
		r.EXPECT().LevelFor("bard", 3_000_000).Return(20, nil).Once()
		r.EXPECT().THAC0For("rogue", 20).Return(11, nil).Once()
		r.EXPECT().SavesFor("rogue", 20).Return(rogue13to16, nil).Once()
		r.EXPECT().BardSpellsPerDay(20).Return(map[int]int{1: 4}, nil).Once()

		got, err := charactersvc.Derive(c, 0, r)
		require.NoError(t, err)
		assert.Equal(t, 20, got.Level)
		assert.Nil(t, got.NextLevelXP, "XPThreshold is never asked for level 21")
	})
}

// ADR 0030 step 2: the class group picks the THAC0 and saving throw rows
// (thief and bard rogues, fighter a warrior).
func TestDerive_Step2_TheClassGroupPicksTheRows(t *testing.T) {
	for class, group := range map[string]string{"bard": "rogue", "thief": "rogue", "fighter": "warrior"} {
		t.Run(class, func(t *testing.T) {
			c := kelsen()
			c.Class, c.XPTotal, c.Sheet = class, 0, charactersvc.Sheet{Level: 1, THAC0: 20}
			r := mocks.NewMockRules(t)
			r.EXPECT().LevelFor(class, 0).Return(1, nil).Once()
			r.EXPECT().XPThreshold(class, 2).Return(1_250, nil).Once()
			r.EXPECT().THAC0For(group, 1).Return(20, nil).Once()
			r.EXPECT().SavesFor(group, 1).Return(ruletablesvc.SavingThrowRow{}, nil).Once()
			if class == "bard" {
				r.EXPECT().BardSpellsPerDay(1).Return(map[int]int{}, nil).Once()
			}
			_, err := charactersvc.Derive(c, 0, r)
			require.NoError(t, err)
		})
	}
	t.Run("an unknown class is an error", func(t *testing.T) {
		c := kelsen()
		c.Class = "paladin"
		_, err := charactersvc.Derive(c, 0, mocks.NewMockRules(t))
		require.ErrorContains(t, err, "paladin")
	})
}

// ADR 0030 step 5: the THAC0 is PHB Table 53's for the group at the derived
// level (Table 53 answers 12 here, not the sheet's 13: the derived one wins).
func TestDerive_Step5_THAC0FromTable53(t *testing.T) {
	r := mocks.NewMockRules(t)
	r.EXPECT().LevelFor("bard", 1_293_418).Return(15, nil).Once()
	r.EXPECT().XPThreshold("bard", 16).Return(1_320_000, nil).Once()
	r.EXPECT().THAC0For("rogue", 15).Return(12, nil).Once()
	r.EXPECT().SavesFor("rogue", 15).Return(rogue13to16, nil).Once()
	r.EXPECT().BardSpellsPerDay(15).Return(level15Slots, nil).Once()
	got, err := charactersvc.Derive(kelsen(), 0, r)
	require.NoError(t, err)
	assert.Equal(t, 12, got.THAC0)
}

// ADR 0030 step 6: the five targets are the Table 60 row of the group's level
// band, each category from its own column (distinct values: a crossed column
// fails).
func TestDerive_Step6_SavesFromTable60(t *testing.T) {
	row := ruletablesvc.SavingThrowRow{Group: "rogue", LevelMin: 13, LevelMax: 16, ParaPoison: 1, RodStaffWand: 2, PetriPoly: 3, Breath: 4, Spell: 5}
	r := mocks.NewMockRules(t)
	r.EXPECT().LevelFor("bard", 1_293_418).Return(15, nil).Once()
	r.EXPECT().XPThreshold("bard", 16).Return(1_320_000, nil).Once()
	r.EXPECT().THAC0For("rogue", 15).Return(13, nil).Once()
	r.EXPECT().SavesFor("rogue", 15).Return(row, nil).Once()
	r.EXPECT().BardSpellsPerDay(15).Return(level15Slots, nil).Once()
	got, err := charactersvc.Derive(kelsen(), 0, r)
	require.NoError(t, err)
	assert.Equal(t, charactersvc.Saves{ParaPoison: 1, RodStaffWand: 2, PetriPoly: 3, Breath: 4, Spell: 5}, got.Saves)
}

// ADR 0030 step 8: a bard's slots are PHB Table 32 at the derived level,
// a mage's Table 21; fighters and thieves have none; the cleric's is not
// implemented yet (an error, not an empty list).
func TestDerive_Step8_SpellSlotsByClass(t *testing.T) {
	t.Run("bard: Table 32", func(t *testing.T) {
		r := mocks.NewMockRules(t)
		expectKelsenRules(r, 1_293_418, 15)
		got, err := charactersvc.Derive(kelsen(), 0, r)
		require.NoError(t, err)
		assert.Equal(t, level15Slots, got.SpellSlots)
	})
	for _, class := range []string{"fighter", "thief"} {
		t.Run(class+": none", func(t *testing.T) {
			c := kelsen()
			c.Class = class
			r := mocks.NewMockRules(t)
			r.EXPECT().LevelFor(class, 1_293_418).Return(14, nil).Once()
			r.EXPECT().XPThreshold(class, 15).Return(1_500_000, nil).Once()
			r.EXPECT().THAC0For(ruleGroup(t, class), 14).Return(7, nil).Once()
			r.EXPECT().SavesFor(ruleGroup(t, class), 14).Return(rogue13to16, nil).Once()
			got, err := charactersvc.Derive(c, 0, r)
			require.NoError(t, err)
			assert.Empty(t, got.SpellSlots)
		})
	}
	t.Run("mage: Table 21", func(t *testing.T) {
		c := kelsen()
		c.Class = "mage"
		r := mocks.NewMockRules(t)
		r.EXPECT().LevelFor("mage", 1_293_418).Return(13, nil).Once()
		r.EXPECT().XPThreshold("mage", 14).Return(1_500_000, nil).Once()
		r.EXPECT().THAC0For("wizard", 13).Return(16, nil).Once()
		r.EXPECT().SavesFor("wizard", 13).Return(rogue13to16, nil).Once()
		r.EXPECT().WizardSpellsPerDay(13).Return(map[int]int{1: 5, 6: 2}, nil).Once()
		got, err := charactersvc.Derive(c, 0, r)
		require.NoError(t, err)
		assert.Equal(t, map[int]int{1: 5, 6: 2}, got.SpellSlots)
	})
	t.Run("cleric: not implemented yet", func(t *testing.T) {
		c := kelsen()
		c.Class = "cleric"
		r := mocks.NewMockRules(t)
		r.EXPECT().LevelFor("cleric", 1_293_418).Return(13, nil).Once()
		r.EXPECT().XPThreshold("cleric", 14).Return(1_350_000, nil).Once()
		r.EXPECT().THAC0For("priest", 13).Return(12, nil).Once()
		r.EXPECT().SavesFor("priest", 13).Return(rogue13to16, nil).Once()
		_, err := charactersvc.Derive(c, 0, r)
		require.ErrorContains(t, err, "not implemented")
	})
}

func ruleGroup(t *testing.T, class string) string {
	t.Helper()
	g, err := ruletablesvc.ClassGroup(class)
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
			r := mocks.NewMockRules(t)
			expectKelsenRules(r, 1_293_418, 15)
			if tc.want != (charactersvc.Saves{}) {
				r.EXPECT().ConSaveBonusFor(16).Return(4, nil).Once()
			}
			got, err := charactersvc.Derive(c, 0, r)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.RaceSaveBonus)
			assert.Equal(t, kelsenSaves, got.Saves, "the bonus never moves the target")
		})
	}
}

// ADR 0030 step 24: each figure the imported sheet states differently from
// the rules is listed (the value, the sheet's figure, the computed one), in
// the sheet's order; the computed value is the one used. The comparison reads
// the rules at the sheet's own XP: XP awarded since does not make the sheet
// "wrong".
func TestDerive_Step24_SheetDifferences(t *testing.T) {
	t.Run("every differing figure", func(t *testing.T) {
		c := kelsen()
		c.Sheet = charactersvc.Sheet{Level: 14, THAC0: 14, Saves: charactersvc.Saves{ParaPoison: 11, RodStaffWand: 8, PetriPoly: 10, Breath: 14, Spell: 10}}
		r := mocks.NewMockRules(t)
		expectKelsenRules(r, 1_293_418, 15)
		got, err := charactersvc.Derive(c, 0, r)
		require.NoError(t, err)
		assert.Equal(t, []charactersvc.SheetDifference{
			{Value: charactersvc.SheetLevel, Sheet: 14, Computed: 15},
			{Value: charactersvc.SheetTHAC0, Sheet: 14, Computed: 13},
			{Value: charactersvc.SheetSaveParaPoison, Sheet: 11, Computed: 10},
			{Value: charactersvc.SheetSavePetriPoly, Sheet: 10, Computed: 9},
			{Value: charactersvc.SheetSaveBreath, Sheet: 14, Computed: 13},
			{Value: charactersvc.SheetSaveSpell, Sheet: 10, Computed: 9},
		}, got.SheetDifferences)
		assert.Equal(t, 15, got.Level, "the computed level is used")
		assert.Equal(t, 13, got.THAC0, "the computed THAC0 is used")
	})
	t.Run("XP awarded since the import is no difference", func(t *testing.T) {
		r := mocks.NewMockRules(t)
		r.EXPECT().LevelFor("bard", 1_293_418+300_000).Return(17, nil).Once()
		r.EXPECT().XPThreshold("bard", 18).Return(1_760_000, nil).Once()
		r.EXPECT().THAC0For("rogue", 17).Return(12, nil).Once()
		r.EXPECT().SavesFor("rogue", 17).Return(ruletablesvc.SavingThrowRow{ParaPoison: 9, RodStaffWand: 6, PetriPoly: 8, Breath: 12, Spell: 7}, nil).Once()
		r.EXPECT().BardSpellsPerDay(17).Return(map[int]int{1: 4}, nil).Once()
		r.EXPECT().LevelFor("bard", 1_293_418).Return(15, nil).Once()
		r.EXPECT().THAC0For("rogue", 15).Return(13, nil).Once()
		r.EXPECT().SavesFor("rogue", 15).Return(rogue13to16, nil).Once()
		got, err := charactersvc.Derive(kelsen(), 300_000, r)
		require.NoError(t, err)
		assert.Equal(t, 17, got.Level)
		assert.Empty(t, got.SheetDifferences)
	})
}

// A table error stops the derivation, naming the lookup.
func TestDerive_ATableErrorIsAnError(t *testing.T) {
	r := mocks.NewMockRules(t)
	boom := errors.New("boom")
	r.EXPECT().LevelFor("bard", 1_293_418).Return(0, boom).Once()
	_, err := charactersvc.Derive(kelsen(), 0, r)
	require.ErrorIs(t, err, boom)
}
