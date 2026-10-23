// Package slots is the one owner of the slot figures (ADR 0046 step 4): ADR 0037 step 3's weapon
// and non-weapon proficiency pools over a Table 34 row, step 5's extra slots of a pick, and ADR
// 0008 step 9's splits of the spell levels a spell grants. It is pure: the caller reads the row
// and Table 4's value from the rule tables and passes them in.
package slots

// Row is one row of PHB Table 34 (ADR 0037 step 1): a class group's weapon and non-weapon
// slots. A pool's slots are Initial plus one at every level divisible by its Levels (step 3).
type Row struct {
	Group            string
	WeaponInitial    int
	WeaponLevels     int
	Penalty          int
	NonweaponInitial int
	NonweaponLevels  int
}

// NonweaponPool is ADR 0037 step 3's non-weapon pool with the terms of its formula: Available =
// Initial (Table 34) + LevelGained (one slot at every realized level divisible by PerLevels) +
// Intelligence (Table 4's "# of Lang." for the sheet's score). Spent is what the picks spend,
// the languages included, and Left = Available - Spent (negative when over budget).
type NonweaponPool struct {
	Initial      int
	PerLevels    int
	LevelGained  int
	Intelligence int
	Available    int
	Spent        int
	Left         int
	// Languages is the part of Spent the character's languages spend (ADR 0037 step 6: one slot
	// for each beyond the native one and Common).
	Languages int
}

// WeaponPool is ADR 0037 step 3's weapon pool with the terms of its formula: Available = Initial
// (Table 34) + LevelGained (one slot at every realized level divisible by PerLevels). Spent is
// what the weapon picks spend and Left = Available - Spent (negative when over budget).
type WeaponPool struct {
	Initial     int
	PerLevels   int
	LevelGained int
	Available   int
	Spent       int
	Left        int
}

// gained is one slot at every level divisible by per; none under level 1 or without a rate.
func gained(level, per int) int {
	if level > 0 && per > 0 {
		return level / per
	}
	return 0
}

// Nonweapon is ADR 0037 step 3 (ADR 0046 step 4.1) for the non-weapon pool at the realized level
// (never the target level), languages being Table 4's "# of Lang." for the sheet's Intelligence
// and the picks spending spent slots. A level under 1 gains no slot.
func Nonweapon(row Row, level, languages, spent int) NonweaponPool {
	out := NonweaponPool{
		Initial: row.NonweaponInitial, PerLevels: row.NonweaponLevels,
		LevelGained: gained(level, row.NonweaponLevels), Intelligence: languages, Spent: spent,
	}
	out.Available = out.Initial + out.LevelGained + out.Intelligence
	out.Left = out.Available - out.Spent
	return out
}

// Weapon is ADR 0037 step 3 (ADR 0046 step 4.1) for the weapon pool at the realized level (never
// the target level), the picks spending spent slots. A level under 1 gains no slot.
func Weapon(row Row, level, spent int) WeaponPool {
	out := WeaponPool{
		Initial: row.WeaponInitial, PerLevels: row.WeaponLevels,
		LevelGained: gained(level, row.WeaponLevels), Spent: spent,
	}
	out.Available = out.Initial + out.LevelGained
	out.Left = out.Available - out.Spent
	return out
}

// Extra is ADR 0037 step 5 (ADR 0046 step 4.1): the slots a pick spends beyond its cost, each +1
// to its check target (0 when it spends the cost or less).
func Extra(slots, cost int) int {
	return max(slots-cost, 0)
}

// Splits is ADR 0008 step 9 (ADR 0046 step 4.3): every way the caster may split levels granted
// spell levels into extra slots. Each split is ascending, and the first is all level 1 slots
// (the default). 3 levels: [1 1 1], [1 2], [3].
func Splits(levels int) [][]int {
	var out [][]int
	var walk func(left, lowest int, acc []int)
	walk = func(left, lowest int, acc []int) {
		if left == 0 {
			out = append(out, append([]int(nil), acc...))
			return
		}
		for l := lowest; l <= left; l++ {
			walk(left-l, l, append(acc, l))
		}
	}
	walk(levels, 1, nil)
	return out
}
