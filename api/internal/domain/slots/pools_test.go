package slots_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/slots"
)

// table34 are PHB Table 34's rows (p.50), as seeds/proficiencies.json holds them.
var table34 = map[string]slots.Row{
	"warrior": {Group: "warrior", WeaponInitial: 4, WeaponLevels: 3, Penalty: -2, NonweaponInitial: 3, NonweaponLevels: 3},
	"wizard":  {Group: "wizard", WeaponInitial: 1, WeaponLevels: 6, Penalty: -5, NonweaponInitial: 4, NonweaponLevels: 3},
	"priest":  {Group: "priest", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 4, NonweaponLevels: 3},
	"rogue":   {Group: "rogue", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 3, NonweaponLevels: 4},
}

// ADR 0037 step 3 (non-weapon pool): initial (Table 34) + one slot at every level divisible by
// #Levels + Table 4's "# of Lang." for the Intelligence. Each group at level 1, one below its first
// gain level, the first gain level and 20; Intelligence 10 (2 languages) throughout, so the
// Intelligence term is the same and the level term moves.
func TestProficiencySlots_Step3_ByGroupAndLevel(t *testing.T) {
	cases := []struct {
		group                       string
		level, initial, gained, all int
	}{
		{"warrior", 1, 3, 0, 5},
		{"warrior", 2, 3, 0, 5},
		{"warrior", 3, 3, 1, 6},
		{"warrior", 6, 3, 2, 7},
		{"warrior", 9, 3, 3, 8},
		{"warrior", 20, 3, 6, 11},
		{"wizard", 1, 4, 0, 6},
		{"wizard", 2, 4, 0, 6},
		{"wizard", 3, 4, 1, 7},
		{"wizard", 20, 4, 6, 12},
		{"priest", 1, 4, 0, 6},
		{"priest", 2, 4, 0, 6},
		{"priest", 3, 4, 1, 7},
		{"priest", 20, 4, 6, 12},
		{"rogue", 1, 3, 0, 5},
		{"rogue", 3, 3, 0, 5},
		{"rogue", 4, 3, 1, 6},
		{"rogue", 8, 3, 2, 7},
		{"rogue", 12, 3, 3, 8},
		{"rogue", 20, 3, 5, 10},
	}
	for _, c := range cases {
		got := slots.Nonweapon(table34[c.group], c.level, ruletablesvc.LanguageLimit(10), 1)
		assert.Equal(t, slots.NonweaponPool{
			Initial: c.initial, PerLevels: table34[c.group].NonweaponLevels, LevelGained: c.gained,
			Intelligence: 2, Available: c.all, Spent: 1, Left: c.all - 1,
		}, got, "%s level %d", c.group, c.level)
	}
	// No level yet gains nothing.
	assert.Equal(t, 0, slots.Nonweapon(table34["rogue"], 0, ruletablesvc.LanguageLimit(10), 0).LevelGained)
}

// ADR 0037 step 3's Intelligence term: PHB Table 4's "# of Lang." for the sheet's score (the
// PHB's dwarf fighter at Intelligence 11: 3 + 0 + 2 = 5; Kelsen, rogue level 15, Intelligence
// 14: 3 + 3 + 4 = 10).
func TestProficiencySlots_Step3_IntelligenceBonusPerTable4(t *testing.T) {
	for _, c := range []struct{ score, bonus int }{{3, 1}, {8, 1}, {9, 2}, {11, 2}, {12, 3}, {14, 4}, {16, 5}, {18, 7}, {19, 8}} {
		assert.Equal(t, c.bonus, slots.Nonweapon(table34["warrior"], 1, ruletablesvc.LanguageLimit(c.score), 0).Intelligence, "Int %d", c.score)
	}
	assert.Equal(t, 5, slots.Nonweapon(table34["warrior"], 1, ruletablesvc.LanguageLimit(11), 0).Available)
	kelsen := slots.Nonweapon(table34["rogue"], 15, ruletablesvc.LanguageLimit(14), 8)
	assert.Equal(t, slots.NonweaponPool{Initial: 3, PerLevels: 4, LevelGained: 3, Intelligence: 4, Available: 10, Spent: 8, Left: 2}, kelsen)
	over := slots.Nonweapon(table34["rogue"], 1, ruletablesvc.LanguageLimit(9), 7)
	assert.Equal(t, -2, over.Left, "over budget: left is negative")
}

// ADR 0037 step 5: each slot spent beyond the cost is one extra slot (+1 to the check).
func TestExtraSlots_Step5_BeyondTheCost(t *testing.T) {
	for _, c := range []struct{ slots, cost, want int }{{1, 1, 0}, {2, 1, 1}, {4, 1, 3}, {2, 2, 0}, {5, 2, 3}, {1, 2, 0}} {
		assert.Equal(t, c.want, slots.Extra(c.slots, c.cost), "%d slots at cost %d", c.slots, c.cost)
	}
}

// ADR 0037 step 3 (weapon pool): initial (Table 34) + one slot at every realized level divisible
// by #Levels. Each group at level 1, one below its first gain level, the first gain level and 20.
func TestWeaponSlots_Step3_ByGroupAndLevel(t *testing.T) {
	cases := []struct {
		group                       string
		level, initial, gained, all int
	}{
		{"warrior", 1, 4, 0, 4},
		{"warrior", 2, 4, 0, 4},
		{"warrior", 3, 4, 1, 5},
		{"warrior", 20, 4, 6, 10},
		{"wizard", 1, 1, 0, 1},
		{"wizard", 5, 1, 0, 1},
		{"wizard", 6, 1, 1, 2},
		{"wizard", 20, 1, 3, 4},
		{"priest", 1, 2, 0, 2},
		{"priest", 3, 2, 0, 2},
		{"priest", 4, 2, 1, 3},
		{"priest", 20, 2, 5, 7},
		{"rogue", 1, 2, 0, 2},
		{"rogue", 3, 2, 0, 2},
		{"rogue", 4, 2, 1, 3},
		{"rogue", 15, 2, 3, 5},
		{"rogue", 20, 2, 5, 7},
	}
	for _, c := range cases {
		got := slots.Weapon(table34[c.group], c.level, 1)
		assert.Equal(t, slots.WeaponPool{
			Initial: c.initial, PerLevels: table34[c.group].WeaponLevels, LevelGained: c.gained,
			Available: c.all, Spent: 1, Left: c.all - 1,
		}, got, "%s level %d", c.group, c.level)
	}
	assert.Equal(t, 0, slots.Weapon(table34["warrior"], 0, 0).LevelGained, "no level yet gains nothing")
}
