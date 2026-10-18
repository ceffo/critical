package ruletablesvc

import (
	"fmt"
	"slices"
)

// The bard's skills (ADR 0030 step 18): the four skills of PHB Table 33 from
// the entered points with the thief's race, Dexterity and armour adjustments
// (Tables 27 to 29), and the budget of the points. Step 17's machinery
// (rogueSkills, budget) with the bard's base table, armours and budget.

// bardSkillClasses are the classes with the four bard skills (step 18).
var bardSkillClasses = []string{"bard"}

// ArmorMetal is the bard's "metal armour up to chain mail" (ADR 0030 step 1:
// chain, ring, scale, hide or brigandine): Table 29's elven chain column
// plus the extra adjustment of the table's note (BardArmorPenalty), a
// ruling, not book text.
const ArmorMetal = "metal"

// armorElvenChain is the Table 29 column ArmorMetal reads.
const armorElvenChain = "elven_chain"

// BardArmors are the armours a bard enters for its skills (step 1): the
// thief's and ArmorMetal.
var BardArmors = append(slices.Clone(ThiefArmors), ArmorMetal)

// Bard skill point budget (ADR 0030 step 18, PHB p.42): 20 points at level
// 1 and 15 at each level after; the PHB gives no per-skill limit.
const (
	BardSkillPointsFirstLevel = 20
	BardSkillPointsPerLevel   = 15
)

// HasBardSkills reports whether class has the bard skills (step 18).
func HasBardSkills(class string) bool { return slices.Contains(bardSkillClasses, class) }

// HasSkillPoints reports whether class enters skill points and a skill
// armour (steps 1, 17 and 18): a thief or a bard.
func HasSkillPoints(class string) bool { return HasThiefSkills(class) || HasBardSkills(class) }

// SkillsOf are the skills class enters points in, in its table's order (nil
// for a class without skill points).
func SkillsOf(class string) []string {
	switch {
	case HasThiefSkills(class):
		return ThiefSkills
	case HasBardSkills(class):
		return BardSkills
	}
	return nil
}

// SkillArmorsOf are the armours class picks for its skills (nil for a class
// without skill points).
func SkillArmorsOf(class string) []string {
	switch {
	case HasThiefSkills(class):
		return ThiefArmors
	case HasBardSkills(class):
		return BardArmors
	}
	return nil
}

// SkillBudgetOf is SkillPointBudget or BardSkillPointBudget by class (an
// empty list for a class without skill points).
func SkillBudgetOf(class string, level int, points map[string]int) []BudgetWarning {
	switch {
	case HasThiefSkills(class):
		return SkillPointBudget(level, points)
	case HasBardSkills(class):
		return BardSkillPointBudget(level, points)
	}
	return []BudgetWarning{}
}

// BardSkills is ADR 0030 step 18 for every skill of BardSkills, in order:
// Table 33's base, the entered points (absent: 0), Table 27 for race, Table
// 28 for dex (Pick Pockets only: the bard's other skills have no column),
// and Table 29 for armor (leather: 0; ArmorMetal: the elven chain column
// plus Table 29's note), capped at MaxSkillPercent and floored at
// MinSkillPercent. An armor that is none of BardArmors is an error.
func (t *Tables) BardSkills(race string, dex int, armor string, points map[string]int) ([]SkillPercent, error) {
	c, err := t.classTables()
	if err != nil {
		return nil, err
	}
	if !slices.Contains(BardArmors, armor) {
		return nil, fmt.Errorf("bard skills: unknown armour %q", armor)
	}
	extra := 0
	if armor == ArmorMetal {
		armor = armorElvenChain
		for _, r := range c.BardArmorPenalty {
			if r.Armor == ArmorMetal {
				extra = r.Adjustment
			}
		}
	}
	return rogueSkills(c, BardSkills, c.BardSkillBase, race, dex, armor, extra, points), nil
}

// BardSkillPointBudget checks the entered points against the bard's budget
// at level (ADR 0030 step 18): at most BardSkillPointsFirstLevel plus
// BardSkillPointsPerLevel a level after the first in all (code BudgetTotal);
// no per-skill check. A warning, never a refusal.
func BardSkillPointBudget(level int, points map[string]int) []BudgetWarning {
	return budget(level, points, BardSkills, BardSkillPointsFirstLevel, BardSkillPointsPerLevel, 0, 0)
}
