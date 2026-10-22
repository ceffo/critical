package ruletablesadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// Expected values below are read from the PHB pages (Table 5 on p.17, Table
// 24 on p.33, Table 61 on p.103, the spheres on p.33-34), not derived from
// the artifact.

// ADR 0030 step 13: Table 24 at the level plus Table 5's bonus spells,
// cumulative from Wisdom 13, each only at a level the cleric has a slot of;
// 6th-level spells need Wisdom 17, 7th-level ones 18.
func TestPriestSpellsPerDay_Step13_WisdomBonusSpells(t *testing.T) {
	tbl := phb(t)
	tests := []struct {
		name       string
		level, wis int
		want       map[int]int
	}{
		{"level 1, Wisdom 12: Table 24 alone", 1, 12, map[int]int{1: 1}},
		{"level 1, Wisdom 13: one bonus 1st", 1, 13, map[int]int{1: 2}},
		{"level 1, Wisdom 15: the 2nd-level bonus waits for a 2nd-level slot", 1, 15, map[int]int{1: 3}},
		{"level 3, Wisdom 15: two 1st and one 2nd", 3, 15, map[int]int{1: 4, 2: 2}},
		{"level 3, Wisdom 18: no 3rd or 4th slot to add to", 3, 18, map[int]int{1: 4, 2: 3}},
		{"level 9, Wisdom 18: 1st 1st 2nd 2nd 3rd 4th", 9, 18, map[int]int{1: 6, 2: 6, 3: 4, 4: 3, 5: 1}},
		{"level 11, Wisdom 16: no 6th level below Wisdom 17", 11, 16, map[int]int{1: 7, 2: 6, 3: 4, 4: 3, 5: 2}},
		{"level 11, Wisdom 17: 6th level opens", 11, 17, map[int]int{1: 7, 2: 6, 3: 5, 4: 3, 5: 2, 6: 1}},
		{"level 14, Wisdom 17: no 7th level below Wisdom 18", 14, 17, map[int]int{1: 8, 2: 8, 3: 7, 4: 5, 5: 3, 6: 2}},
		{"level 14, Wisdom 18: 7th level opens", 14, 18, map[int]int{1: 8, 2: 8, 3: 7, 4: 6, 5: 3, 6: 2, 7: 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tbl.PriestSpellsPerDay(tc.level, tc.wis)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
	_, err := tbl.PriestSpellsPerDay(21, 12)
	require.Error(t, err)
	_, err = tbl.PriestSpellsPerDay(1, 26)
	require.Error(t, err)
}

// ADR 0030 step 14: Table 5's chance of spell failure for a cleric, Wisdom 12
// and below; none from 13 nor for a class without it.
func TestSpellFailureFor_Step14_LowWisdom(t *testing.T) {
	tbl := phb(t)
	for _, tc := range []struct {
		class     string
		wis, want int
	}{
		{"cleric", 3, 50},
		{"cleric", 9, 20},
		{"cleric", 12, 5},
		{"cleric", 13, 0},
		{"cleric", 18, 0},
		{"mage", 3, 0},
		{"bard", 9, 0},
	} {
		got := 0
		if fixtureClass(t, tc.class).Has(classes.KindSpellFailure, 1) {
			var err error
			got, err = tbl.SpellFailureFor(tc.wis)
			require.NoError(t, err)
		}
		assert.Equal(t, tc.want, got, "%s Wisdom %d", tc.class, tc.wis)
	}
}

// ADR 0006 step 9.1: Table 5's magical defence adjustment column, read for
// the Wisdom of every class, from -6 at Wisdom 1 to +4 at 18 and up; a score
// outside the table is refused.
func TestWisdomFor_Step9_MagicDefence(t *testing.T) {
	tbl := phb(t)
	for wis, want := range map[int]int{1: -6, 3: -3, 7: -1, 8: 0, 14: 0, 15: 1, 17: 3, 18: 4, 25: 4} {
		got, err := tbl.MagicDefenceFor(wis)
		require.NoError(t, err)
		assert.Equal(t, want, got, "Wisdom %d", wis)
		row, err := tbl.WisdomFor(wis)
		require.NoError(t, err)
		assert.Equal(t, want, row.MagicDefence, "Wisdom %d row", wis)
	}
	for _, wis := range []int{0, 26} {
		_, err := tbl.MagicDefenceFor(wis)
		assert.Error(t, err, "Wisdom %d", wis)
	}
}

// ADR 0030 step 16: Table 61 at a cleric's level, the shared columns above
// level 9 (10-11, 12-13, 14 and up).
func TestTurningAt_Step16_Table61(t *testing.T) {
	tbl := phb(t)
	cell := func(level int, undead string) ruletablesvc.TurningCell {
		rows, err := tbl.TurningAt(level)
		require.NoError(t, err)
		require.Len(t, rows, len(ruletablesvc.Undead))
		for _, r := range rows {
			if r.Undead == undead {
				return r.Cell
			}
		}
		t.Fatalf("no row %s", undead)
		return ruletablesvc.TurningCell{}
	}
	assert.Equal(t, ruletablesvc.TurningCell{Roll: 10}, cell(1, "skeleton"))
	assert.Equal(t, ruletablesvc.TurningCell{Roll: 20}, cell(1, "wight"))
	assert.Equal(t, ruletablesvc.TurningCell{Outcome: "cannot"}, cell(1, "ghast"))
	assert.Equal(t, ruletablesvc.TurningCell{Outcome: "turn"}, cell(4, "skeleton"))
	assert.Equal(t, ruletablesvc.TurningCell{Outcome: "destroy"}, cell(6, "skeleton"))
	assert.Equal(t, ruletablesvc.TurningCell{Outcome: "destroy_more"}, cell(8, "skeleton"))
	assert.Equal(t, ruletablesvc.TurningCell{Roll: 13}, cell(10, "ghost"))
	assert.Equal(t, ruletablesvc.TurningCell{Roll: 13}, cell(11, "ghost"))
	assert.Equal(t, ruletablesvc.TurningCell{Roll: 10}, cell(12, "ghost"))
	assert.Equal(t, ruletablesvc.TurningCell{Roll: 13}, cell(20, "special"))
	rows, err := tbl.TurningAt(1)
	require.NoError(t, err)
	assert.Equal(t, 3, rows[3].HDMin, "shadow or 3-4 HD")
	assert.Equal(t, 4, rows[3].HDMax)
	_, err = tbl.TurningAt(0)
	require.Error(t, err)
}

// ADR 0030 step 15.1 and 15.2 on the fixture's sphere access: the cleric's
// twelve major spheres open every level, Elemental (each subsphere) levels 1
// to 3 only, every other sphere is closed.
func TestSphereAccess_Step15_TheClericsSpheres(t *testing.T) {
	access, err := phb(t).SphereAccess()
	require.NoError(t, err)
	assert.True(t, access.HasSpheres("cleric"))
	assert.False(t, access.HasSpheres("mage"))
	for _, s := range []string{
		"all", "astral", "charm", "combat", "creation", "divination", "guardian", "healing",
		"necromantic", "protection", "summoning", "sun",
	} {
		assert.True(t, access.Opens("cleric", 7, []string{s}), "major %s at 7th", s)
	}
	for _, s := range []string{"elemental_air", "elemental_earth", "elemental_fire", "elemental_water"} {
		assert.True(t, access.Opens("cleric", 3, []string{s}), "minor %s at 3rd", s)
		assert.False(t, access.Opens("cleric", 4, []string{s}), "minor %s at 4th", s)
	}
	for _, s := range []string{"animal", "plant", "weather", "thought", "chaos", "law", "numbers", "time", "travelers", "war", "wards"} {
		assert.False(t, access.Opens("cleric", 1, []string{s}), "closed %s", s)
	}
	assert.True(t, access.Opens("cleric", 5, []string{"elemental_fire", "combat"}), "Elemental (Fire) and Combat: open through Combat")
	assert.False(t, access.Opens("mage", 1, []string{"all"}), "no access, no spell")
}

// ADR 0006 step 10.1: Table 5's spell immunities, cumulative from the lowest
// row up to the score: none at Wisdom 18, charm person but not hold person at
// 19, hold person (both spells of the name) at 20, every row's at 25; a score
// outside the table is refused.
func TestSpellImmunitiesFor_Step10_Cumulative(t *testing.T) {
	tbl := phb(t)
	at := func(wis int) []string {
		got, err := tbl.SpellImmunitiesFor(wis)
		require.NoError(t, err, "Wisdom %d", wis)
		return got
	}
	for _, wis := range []int{1, 12, 18} {
		assert.Empty(t, at(wis), "Wisdom %d", wis)
	}
	assert.Equal(t, []string{"charm_person", "command", "friends", "hypnotism"}, at(19))
	assert.NotContains(t, at(19), "hold_person")
	assert.Equal(t, []string{
		"charm_person", "command", "friends", "hypnotism",
		"forget", "hold_person", "hold_person_priest", "ray_of_enfeeblement", "scare",
	}, at(20))
	all := at(25)
	assert.Len(t, all, 26)
	for _, id := range []string{"charm_person", "hold_person", "fear", "confusion_priest", "quest", "geas", "mass_charm"} {
		assert.Contains(t, all, id)
	}
	for _, wis := range []int{0, 26} {
		_, err := tbl.SpellImmunitiesFor(wis)
		assert.Error(t, err, "Wisdom %d", wis)
	}
}
