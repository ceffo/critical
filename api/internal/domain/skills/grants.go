package skills

import "github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"

// LevelGrants are slots and skill points beyond hit points (ADR 0037 step 15): what a level
// grants, or what is left to spend. WeaponSlots and NonweaponSlots are ADR 0037 step 3's pools,
// SkillPoints a thief's or a bard's budget (ADR 0030 steps 17 and 18). Never negative.
type LevelGrants struct {
	WeaponSlots    int
	NonweaponSlots int
	SkillPoints    int
}

// GrantsAt is ADR 0037 step 15: what level grants a character of class beyond hit points, the
// slots of step 3's pools gained over the level below (Table 34 from book; none without a row)
// and a thief's or a bard's skill points (ADR 0030 steps 17 and 18). Zero at level 1.
func GrantsAt(book Book, class string, level int) LevelGrants {
	var g LevelGrants
	if row, ok := book.SlotRowOf(class); ok {
		g.WeaponSlots, g.NonweaponSlots = ruletablesvc.SlotsGainedAt(row, level)
	}
	if def, ok := book.ClassDef(class); ok {
		for _, rs := range RogueSetsAt(def, level) {
			g.SkillPoints += ruletablesvc.SkillPointsGainedAt(&rs, level)
		}
	}
	return g
}

// GrantsOf is GrantsAt for a level of the entry of class among a character's active entries
// cls (ADR 0042 steps 7 and 15): the slots of the combined pools at that entry's level over the
// level below, and the skill points the level grants when class has them.
func GrantsOf(book Book, cls Classes, class string, level int) LevelGrants {
	var g LevelGrants
	if level >= 2 {
		at, ok := FiguresOf(book, cls.with(class, level), 0, Spending{})
		below, _ := FiguresOf(book, cls.with(class, level-1), 0, Spending{})
		if ok {
			g.WeaponSlots = at.Weapon.Available - below.Weapon.Available
			g.NonweaponSlots = at.Nonweapon.Available - below.Nonweapon.Available
		}
	}
	if def, ok := book.ClassDef(class); ok {
		for _, rs := range RogueSetsAt(def, level) {
			g.SkillPoints += ruletablesvc.SkillPointsGainedAt(&rs, level)
		}
	}
	return g
}

// ToSpend is what the pools f and the skill sets leave to spend at the realized level (ADR
// 0037 step 15's reminder): the pools' left slots and the skill points left in all the sets
// (each set's budget is its own: a set over budget leaves 0, it does not eat into another's),
// each 0 when none (nil) or over budget.
func ToSpend(f *ProficiencySlots, sets []SkillSet) LevelGrants {
	var g LevelGrants
	if f != nil {
		g.WeaponSlots, g.NonweaponSlots = max(f.Weapon.Left, 0), max(f.Nonweapon.Left, 0)
	}
	for _, set := range sets {
		if set.Points != nil {
			g.SkillPoints += max(set.Points.Left, 0)
		}
	}
	return g
}
