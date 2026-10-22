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
		g.SkillPoints = ruletablesvc.SkillPointsGainedAt(RogueSkillsAt(def, level), level)
	}
	return g
}

// ToSpend is what the pools f and the skill points b leave to spend at the realized level (ADR
// 0037 step 15's reminder): the pools' left slots and the skill points left, each 0 when none
// (nil) or over budget.
func ToSpend(f *ProficiencySlots, b *ruletablesvc.SkillPointsBudget) LevelGrants {
	var g LevelGrants
	if f != nil {
		g.WeaponSlots, g.NonweaponSlots = max(f.Weapon.Left, 0), max(f.Nonweapon.Left, 0)
	}
	if b != nil {
		g.SkillPoints = max(b.Left, 0)
	}
	return g
}
