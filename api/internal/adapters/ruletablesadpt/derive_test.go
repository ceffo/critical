package ruletablesadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The lookups of the derived values (ADR 0030 steps 2, 3, 5, 6, 8 and 22.1)
// over the fixture's class tables, which carry the books' numbers. Expected
// values are read from the PHB pages, not from the artifact.

// ADR 0030 step 2: each class's group.
func TestClassGroup_Step2(t *testing.T) {
	for class, want := range map[string]string{
		"fighter": "warrior", "mage": "wizard", "cleric": "priest", "thief": "rogue", "bard": "rogue",
	} {
		got, err := phb(t).ClassGroup(class)
		require.NoError(t, err)
		assert.Equal(t, want, got, class)
	}
	_, err := phb(t).ClassGroup("paladin")
	require.Error(t, err)
}

// ADR 0030 step 3: the level is the highest one whose threshold is at or
// under the XP, from the class's table: PHB Table 25 (p.38) for thieves and
// bards, Table 14 (p.26) for fighters; at and above the last threshold the
// level stays 20.
func TestLevelFor_Step3_HighestThresholdAtOrUnderTheXP(t *testing.T) {
	tbl := phb(t)
	tests := []struct {
		class string
		xp    int
		want  int
	}{
		{"bard", 0, 1},
		{"bard", -5, 1},
		{"bard", 1_249, 1},
		{"bard", 1_250, 2},
		{"bard", 1_099_999, 14},
		{"bard", 1_100_000, 15},
		{"bard", 1_293_418, 15}, // Kelsen's sheet
		{"bard", 1_319_999, 15},
		{"bard", 1_320_000, 16},
		{"bard", 2_200_000, 20},
		{"bard", 9_999_999, 20},
		{"thief", 1_320_000, 16},
		{"fighter", 1_293_418, 13},
		{"fighter", 1_999_999, 15},
		{"fighter", 2_000_000, 16},
		{"fighter", 3_000_000, 20},
	}
	for _, tt := range tests {
		got, err := tbl.LevelFor(tt.class, tt.xp)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got, "%s at %d XP", tt.class, tt.xp)
	}
	_, err := tbl.LevelFor("paladin", 0)
	require.Error(t, err)
}

// The next level's threshold, from the same column (Tables 25 and 14).
func TestXPThreshold_ByClass(t *testing.T) {
	tbl := phb(t)
	for _, tt := range []struct {
		class       string
		level, want int
	}{
		{"bard", 16, 1_320_000}, {"thief", 17, 1_540_000}, {"fighter", 16, 2_000_000}, {"fighter", 1, 0},
	} {
		got, err := tbl.XPThreshold(tt.class, tt.level)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got, "%s level %d", tt.class, tt.level)
	}
	for _, bad := range []int{0, 21} {
		_, err := tbl.XPThreshold("bard", bad)
		require.Error(t, err, "level %d", bad)
	}
}

// ADR 0030 step 5: PHB Table 53 (p.91), by group and level.
func TestTHAC0For_Step5_Table53(t *testing.T) {
	tbl := phb(t)
	for _, tt := range []struct {
		group       string
		level, want int
	}{
		{"rogue", 1, 20},
		{"rogue", 14, 14},
		{"rogue", 15, 13},
		{"rogue", 16, 13},
		{"rogue", 17, 12},
		{"rogue", 20, 11},
		{"warrior", 1, 20},
		{"warrior", 13, 8},
		{"warrior", 20, 1},
		{"priest", 4, 18},
		{"priest", 13, 12},
		{"wizard", 4, 19},
		{"wizard", 20, 14},
	} {
		got, err := tbl.THAC0For(tt.group, tt.level)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got, "%s level %d", tt.group, tt.level)
	}
	for _, bad := range []int{0, 21} {
		_, err := tbl.THAC0For("rogue", bad)
		require.Error(t, err, "level %d", bad)
	}
	_, err := tbl.THAC0For("monk", 1)
	require.Error(t, err)
}

// ADR 0040 step 7: THAC0For reads the group definitions' rows; at every
// level they equal the format-10 Table 53 columns they replace.
func TestRuleTables_Step7_RowsKeyedByClassAndGroup(t *testing.T) {
	tbl := phb(t)
	c, ok := tbl.Classes()
	require.True(t, ok)
	require.Len(t, c.THAC0, 20)
	for _, r := range c.THAC0 {
		for group, want := range map[string]int{"warrior": r.Warrior, "priest": r.Priest, "rogue": r.Rogue, "wizard": r.Wizard} {
			got, err := tbl.THAC0For(group, r.Level)
			require.NoError(t, err)
			assert.Equal(t, want, got, "%s level %d", group, r.Level)
		}
	}
}

// ADR 0030 step 6: the PHB Table 60 (p.101) row of the group's level band,
// each column in its place.
func TestSavesFor_Step6_Table60(t *testing.T) {
	tbl := phb(t)
	type saves struct{ para, rod, petri, breath, spell int }
	for _, tt := range []struct {
		group string
		level int
		want  saves
	}{
		{"rogue", 13, saves{10, 8, 9, 13, 9}},
		{"rogue", 15, saves{10, 8, 9, 13, 9}}, // Kelsen's sheet
		{"rogue", 16, saves{10, 8, 9, 13, 9}},
		{"rogue", 17, saves{9, 6, 8, 12, 7}},
		{"rogue", 4, saves{13, 14, 12, 16, 15}},
		{"warrior", 1, saves{14, 16, 15, 17, 17}},
		{"priest", 15, saves{5, 9, 8, 11, 10}},
		{"wizard", 20, saves{10, 5, 7, 9, 6}},
	} {
		r, err := tbl.SavesFor(tt.group, tt.level)
		require.NoError(t, err)
		assert.Equal(t, tt.want, saves{r.ParaPoison, r.RodStaffWand, r.PetriPoly, r.Breath, r.Spell}, "%s level %d", tt.group, tt.level)
	}
	_, err := tbl.SavesFor("rogue", 21)
	require.Error(t, err)
}

// ADR 0030 step 22.1: PHB Table 9 (p.21); none below 4, the 18-19 bonus
// above 19.
func TestConSaveBonusFor_Step22_1_Table9(t *testing.T) {
	tbl := phb(t)
	for con, want := range map[int]int{3: 0, 4: 1, 6: 1, 7: 2, 10: 2, 11: 3, 13: 3, 14: 4, 16: 4, 17: 4, 18: 5, 19: 5, 22: 5} {
		got, err := tbl.ConSaveBonusFor(con)
		require.NoError(t, err)
		assert.Equal(t, want, got, "Constitution %d", con)
	}
}

// ADR 0030 step 8: PHB Table 21 (p.30), a mage's spells per day.
func TestWizardSpellsPerDay_Step8_Table21(t *testing.T) {
	tbl := phb(t)
	for level, want := range map[int]map[int]int{
		1:  {1: 1},
		5:  {1: 4, 2: 2, 3: 1},
		13: {1: 5, 2: 5, 3: 5, 4: 4, 5: 4, 6: 2},
		18: {1: 5, 2: 5, 3: 5, 4: 5, 5: 5, 6: 3, 7: 3, 8: 2, 9: 1},
	} {
		got, err := tbl.WizardSpellsPerDay(level)
		require.NoError(t, err)
		assert.Equal(t, want, got, "level %d", level)
	}
	_, err := tbl.WizardSpellsPerDay(21)
	require.Error(t, err)
}

// ADR 0030 step 7.1 and 7.2: each group's hit die, name level and fixed gain
// after it (PHB Tables 14 p.26, 20 p.30, 23 p.33 and 25 p.38, Hit Dice
// column, with chapter 3's hit points paragraphs).
func TestHitDiceFor_Step7_TheGroupsDieNameLevelAndFixedGain(t *testing.T) {
	tbl := phb(t)
	for group, want := range map[string]ruletablesvc.HitDiceRow{
		"warrior":    {Group: "warrior", Die: 10, NameLevel: 9, GainAfter: 3},
		"priest":     {Group: "priest", Die: 8, NameLevel: 9, GainAfter: 2},
		"rogue":      {Group: "rogue", Die: 6, NameLevel: 10, GainAfter: 2},
		"wizard":     {Group: "wizard", Die: 4, NameLevel: 10, GainAfter: 1},
		"psionicist": {Group: "psionicist", Die: 6, NameLevel: 9, GainAfter: 2},
	} {
		got, err := tbl.HitDiceFor(group)
		require.NoError(t, err)
		assert.Equal(t, want, got, group)
	}
	_, err := tbl.HitDiceFor("monk")
	require.Error(t, err)
}

// ADR 0030 step 7.1: PHB Table 3 (p.14), the hit point adjustment (the
// warriors' bracketed value) and the value a low die counts as from 20 up.
func TestConstitutionFor_Step7_1_Table3(t *testing.T) {
	tbl := phb(t)
	type row struct{ hp, warrior, lowDieMin int }
	for con, want := range map[int]row{
		3: {-2, -2, 0}, 6: {-1, -1, 0}, 7: {0, 0, 0}, 14: {0, 0, 0}, 15: {1, 1, 0}, 16: {2, 2, 0},
		17: {2, 3, 0}, 18: {2, 4, 0}, 19: {2, 5, 0}, 20: {2, 5, 2}, 21: {2, 6, 3}, 23: {2, 6, 4}, 25: {2, 7, 4},
	} {
		got, err := tbl.ConstitutionFor(con)
		require.NoError(t, err)
		assert.Equal(t, want, row{got.HP, got.HPWarrior, got.LowDieMin}, "Constitution %d", con)
	}
	_, err := tbl.ConstitutionFor(26)
	require.Error(t, err)
}

// PHB Table 4 (ADR 0030 step 20): the highest spell level, the chance to
// learn and the maximum spells per level ("All" from 19), against the book.
func TestIntelligenceFor_Step20_Table4(t *testing.T) {
	tbl := phb(t)
	type row struct {
		level, chance, max int
		all                bool
	}
	for score, want := range map[int]row{
		8: {0, 0, 0, false}, 9: {4, 35, 6, false}, 10: {5, 40, 7, false}, 13: {6, 55, 9, false},
		14: {7, 60, 9, false}, 17: {8, 75, 14, false}, 18: {9, 85, 18, false}, 19: {9, 95, 0, true},
		24: {9, 100, 0, true}, 25: {9, 100, 0, true},
	} {
		got, err := tbl.IntelligenceFor(score)
		require.NoError(t, err)
		assert.Equal(t, want, row{got.MaxSpellLevel, got.LearnChance, got.MaxSpells, got.NoMaximum}, "Intelligence %d", score)
	}
	_, err := tbl.IntelligenceFor(26)
	require.Error(t, err)
}

// Only mages and bards keep a spellbook, up to their highest spell level
// (ADR 0030 step 20.1).
func TestSpellbookMaxLevel_Step20_1(t *testing.T) {
	for class, want := range map[string]int{"mage": 9, "bard": 6, "fighter": 0, "cleric": 0, "thief": 0} {
		got, ok := ruletablesvc.SpellbookMaxLevel(class)
		assert.Equal(t, want, got, class)
		assert.Equal(t, want > 0, ok, class)
	}
}

// ADR 0032 step 6 shows PHB Table 8 (p.21) next to the rolled totals: each
// race's adjustment by ability, 0 elsewhere, a human's all 0.
func TestRaceAdjustmentsFor_Step6_Table8(t *testing.T) {
	tbl := phb(t)
	zero := map[string]int{"str": 0, "dex": 0, "con": 0, "int": 0, "wis": 0, "cha": 0}
	with := func(changes map[string]int) map[string]int {
		out := map[string]int{}
		for k, v := range zero {
			out[k] = v
		}
		for k, v := range changes {
			out[k] = v
		}
		return out
	}
	for race, want := range map[string]map[string]int{
		"human":    zero,
		"half_elf": zero,
		"dwarf":    with(map[string]int{"con": 1, "cha": -1}),
		"elf":      with(map[string]int{"dex": 1, "con": -1}),
		"gnome":    with(map[string]int{"int": 1, "wis": -1}),
		"halfling": with(map[string]int{"dex": 1, "str": -1}),
	} {
		got, err := tbl.RaceAdjustmentsFor(race)
		require.NoError(t, err)
		assert.Equal(t, want, got, race)
	}
	_, err := tbl.RaceAdjustmentsFor("orc")
	require.Error(t, err)
}
