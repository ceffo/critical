package ruletablesadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
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
		got, err := ruletablesvc.ClassGroup(class)
		require.NoError(t, err)
		assert.Equal(t, want, got, class)
	}
	_, err := ruletablesvc.ClassGroup("paladin")
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

// A format-3 artifact has no class tables: every lookup says so (the server
// refuses to start on one).
func TestDerivedLookups_Format3HasNoClassTables(t *testing.T) {
	three, err := classTablesOf(t, catalogue.FormatRuleTables, func(d map[string]any) {
		for k := range d {
			if k != "strength" && k != "dexterity" && k != "bard_spells" && k != "bard_experience" {
				delete(d, k)
			}
		}
	})
	require.NoError(t, err)
	_, err = three.LevelFor("bard", 0)
	require.ErrorIs(t, err, ruletablesvc.ErrNoClassTables)
	_, err = three.THAC0For("rogue", 1)
	require.ErrorIs(t, err, ruletablesvc.ErrNoClassTables)
	_, err = three.SavesFor("rogue", 1)
	require.ErrorIs(t, err, ruletablesvc.ErrNoClassTables)
	_, err = three.ConSaveBonusFor(16)
	require.ErrorIs(t, err, ruletablesvc.ErrNoClassTables)
	_, err = three.XPThreshold("bard", 1)
	require.ErrorIs(t, err, ruletablesvc.ErrNoClassTables)
	_, err = three.WizardSpellsPerDay(1)
	require.ErrorIs(t, err, ruletablesvc.ErrNoClassTables)
}
