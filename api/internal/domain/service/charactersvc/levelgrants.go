package charactersvc

import "github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"

// GrantsAt is ADR 0037 step 15: what level grants a character of class beyond hit points, the
// slots of step 3's pools gained over the level below (Table 34 from book; none without a row)
// and a thief's or a bard's skill points (ADR 0030 steps 17 and 18). Zero at level 1.
func GrantsAt(book ProficiencyBook, class string, level int) LevelGrants {
	var g LevelGrants
	if row, ok := book.SlotRowOf(class); ok {
		g.WeaponSlots, g.NonweaponSlots = ruletablesvc.SlotsGainedAt(row, level)
	}
	g.SkillPoints = ruletablesvc.SkillPointsGainedAt(class, level)
	return g
}

// ToSpend is what d leaves to spend at the realized level (ADR 0037 step 15's reminder): the
// pools' left slots and the skill points left, each 0 when none or over budget.
func ToSpend(d Derived) LevelGrants {
	var g LevelGrants
	if f := d.Proficiencies; f != nil {
		g.WeaponSlots, g.NonweaponSlots = max(f.Weapon.Left, 0), max(f.Nonweapon.Left, 0)
	}
	if b := d.SkillPoints; b != nil {
		g.SkillPoints = max(b.Left, 0)
	}
	return g
}

// setGrants fills c's level-up with ADR 0037 step 15 once its pools are derived: each pending
// level's grants and what is left to spend.
func setGrants(book ProficiencyBook, c *Character) {
	for i := range c.Derived.LevelUp.Pending {
		p := &c.Derived.LevelUp.Pending[i]
		p.Grants = GrantsAt(book, c.Class, p.Level)
	}
	c.Derived.LevelUp.ToSpend = ToSpend(c.Derived)
}
