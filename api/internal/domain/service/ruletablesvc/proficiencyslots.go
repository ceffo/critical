package ruletablesvc

import "slices"

// NonweaponPool is ADR 0037 step 3's non-weapon pool with the terms of its
// formula: Available = Initial (Table 34) + LevelGained (one slot at every
// realized level divisible by PerLevels) + Intelligence (Table 4's
// "# of Lang." for the sheet's score). Spent is what the picks spend and Left
// = Available - Spent (negative when over budget).
type NonweaponPool struct {
	Initial      int
	PerLevels    int
	LevelGained  int
	Intelligence int
	Available    int
	Spent        int
	Left         int
}

// NonweaponSlots is ADR 0037 step 3 for the non-weapon pool at the realized
// level (never the target level) with intelligence the sheet's score, the
// picks spending spent slots. A level under 1 gains no slot.
func NonweaponSlots(row SlotRow, level, intelligence, spent int) NonweaponPool {
	gained := 0
	if level > 0 && row.NonweaponLevels > 0 {
		gained = level / row.NonweaponLevels
	}
	out := NonweaponPool{
		Initial: row.NonweaponInitial, PerLevels: row.NonweaponLevels, LevelGained: gained,
		Intelligence: LanguageLimit(intelligence), Spent: spent,
	}
	out.Available = out.Initial + out.LevelGained + out.Intelligence
	out.Left = out.Available - out.Spent
	return out
}

// NonweaponCost is ADR 0037 step 4: Table 37's slots when one of the
// proficiency's groups is among the class's Table 38 groups, one more
// otherwise.
func NonweaponCost(p Proficiency, classGroups []string) int {
	for _, g := range p.Groups {
		if slices.Contains(classGroups, g) {
			return p.Slots
		}
	}
	return p.Slots + 1
}

// ExtraSlots is ADR 0037 step 5: the slots a pick spends beyond its cost, each
// +1 to its check target (0 when it spends the cost or less).
func ExtraSlots(slots, cost int) int {
	return max(slots-cost, 0)
}
