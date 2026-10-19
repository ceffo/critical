package ruletablesvc

import (
	"fmt"
	"slices"
)

// The thief's rules (ADR 0030 steps 17 and 19): the thieving skills from the
// entered points with the race, Dexterity and armour adjustments, the budget
// of the points, and the backstab multiplier. The numbers are the class
// tables' (PHB Tables 26 to 30); this file says which row and column a thief
// reads.

// thiefSkillClasses are the classes with the eight thieving skills of PHB
// Table 26 (ADR 0030 step 17). The bard's four skills are step 18's.
var thiefSkillClasses = []string{"thief"}

// backstabClasses are the classes whose attack has the backstab option (ADR
// 0030 step 19: a thief's, not a bard's).
var backstabClasses = []string{"thief"}

// ArmorLeather is Table 29's baseline (no column: every adjustment 0).
const ArmorLeather = "leather"

// ThiefArmors are the armours a thief enters for its skills (ADR 0030 step
// 1): Table 29's columns and leather, its baseline.
var ThiefArmors = []string{"none", ArmorLeather, "padded_studded_leather", "elven_chain"}

// Thieving skill limits (ADR 0030 step 17, PHB p.39).
const (
	// MaxSkillPercent caps a skill with every adjustment; MinSkillPercent
	// floors it.
	MaxSkillPercent = 95
	MinSkillPercent = 0
	// SkillPointsFirstLevel and SkillPointsPerLevel are the points a thief
	// spends at level 1 and at each level after; SkillPointsPerSkillFirst
	// and SkillPointsPerSkillPerLevel the most it puts in one skill.
	SkillPointsFirstLevel       = 60
	SkillPointsPerLevel         = 30
	SkillPointsPerSkillFirst    = 30
	SkillPointsPerSkillPerLevel = 15
)

// HasThiefSkills reports whether class has the thieving skills (ADR 0030
// step 17).
func HasThiefSkills(class string) bool { return slices.Contains(thiefSkillClasses, class) }

// Backstabs reports whether class's attack has the backstab option (ADR
// 0030 step 19).
func Backstabs(class string) bool { return slices.Contains(backstabClasses, class) }

// SkillPercent is one thieving skill of ADR 0030 step 17 with its terms:
// Percent = Base + Points + Race + Dexterity + Armor, capped at
// MaxSkillPercent and floored at MinSkillPercent.
type SkillPercent struct {
	Skill                                string
	Base, Points, Race, Dexterity, Armor int
	Percent                              int
}

// ThiefSkills is ADR 0030 step 17 for every skill of ThiefSkills, in order:
// Table 26's base, the entered points (a skill absent from points has 0),
// Table 27 for race ("" or human: 0), Table 28 for dex (a score outside its
// rows or a skill without a column: 0) and Table 29 for armor (leather: 0).
// An armor that is none of ThiefArmors is an error.
func (t *Tables) ThiefSkills(race string, dex int, armor string, points map[string]int) ([]SkillPercent, error) {
	c, err := t.classTables()
	if err != nil {
		return nil, err
	}
	if !slices.Contains(ThiefArmors, armor) {
		return nil, fmt.Errorf("thief skills: unknown armour %q", armor)
	}
	return rogueSkills(c, ThiefSkills, c.ThiefSkillBase, race, dex, armor, 0, points), nil
}

// rogueSkills is the sum of ADR 0030 steps 17 and 18 for each of skills, in
// order: the base from base (the class's table), the entered points, Table 27
// for race, Table 28 for dex (a skill without a column: 0), Table 29's armor
// column (leather: 0) plus extra (the bard's metal armour penalty, step 18),
// capped at MaxSkillPercent and floored at MinSkillPercent.
func rogueSkills(c *ClassTables, skills []string, base []SkillBaseRow, race string, dex int, armor string, extra int, points map[string]int) []SkillPercent {
	var dexRow SkillDexterityRow
	for _, r := range c.ThiefSkillDexterity {
		if dex >= r.DexMin && dex <= r.DexMax {
			dexRow = r
			break
		}
	}
	out := make([]SkillPercent, 0, len(skills))
	for _, skill := range skills {
		p := SkillPercent{Skill: skill, Points: points[skill], Dexterity: dexterityColumn(dexRow, skill), Armor: extra}
		for _, r := range base {
			if r.Skill == skill {
				p.Base = r.Base
			}
		}
		for _, r := range c.ThiefSkillRace {
			if r.Race == race && r.Skill == skill {
				p.Race = r.Adjustment
			}
		}
		for _, r := range c.ThiefSkillArmor {
			if r.Armor == armor && r.Skill == skill {
				p.Armor += r.Adjustment
			}
		}
		p.Percent = min(max(p.Base+p.Points+p.Race+p.Dexterity+p.Armor, MinSkillPercent), MaxSkillPercent)
		out = append(out, p)
	}
	return out
}

// dexterityColumn is Table 28's column of skill (0 for a skill without one).
func dexterityColumn(r SkillDexterityRow, skill string) int {
	switch skill {
	case "pick_pockets":
		return r.PickPockets
	case "open_locks":
		return r.OpenLocks
	case "find_remove_traps":
		return r.FindRemoveTraps
	case "move_silently":
		return r.MoveSilently
	case "hide_in_shadows":
		return r.HideInShadows
	}
	return 0
}

// Skill point budget warning codes (ADR 0030 step 17).
const (
	// BudgetTotal: the points of every skill add up to more than the level's
	// budget (Skill is empty).
	BudgetTotal = "skill_points_total"
	// BudgetPerSkill: one skill holds more than the level allows in one
	// skill.
	BudgetPerSkill = "skill_points_per_skill"
)

// BudgetWarning is a breach of the skill point budget (ADR 0030 step 17): a
// warning shown on the skills card, never a refusal.
type BudgetWarning struct {
	Code   string
	Skill  string
	Points int
	Limit  int
}

// SkillPointBudget checks the entered points against PHB p.39's budget at
// level (ADR 0030 step 17): at most SkillPointsFirstLevel plus
// SkillPointsPerLevel a level after the first in all, at most
// SkillPointsPerSkillFirst plus SkillPointsPerSkillPerLevel a level after
// the first in one skill. The points are entered as one total per skill, so
// the check is on the totals the levels allow. Skills in ThiefSkills order.
func SkillPointBudget(level int, points map[string]int) []BudgetWarning {
	return budget(level, points, ThiefSkills, SkillPointsFirstLevel, SkillPointsPerLevel, SkillPointsPerSkillFirst, SkillPointsPerSkillPerLevel)
}

// SkillBudgetLimits are the limits SkillBudgetOf checks at level (ADR 0030
// steps 17 and 18): the points in all, and in one skill (0: no limit on one
// skill, the bard's). ok is false for a class without skill points or a level
// under 1.
func SkillBudgetLimits(class string, level int) (total, perSkill int, ok bool) {
	if level < 1 {
		return 0, 0, false
	}
	switch {
	case HasThiefSkills(class):
		return SkillPointsFirstLevel + SkillPointsPerLevel*(level-1),
			SkillPointsPerSkillFirst + SkillPointsPerSkillPerLevel*(level-1), true
	case HasBardSkills(class):
		return BardSkillPointsFirstLevel + BardSkillPointsPerLevel*(level-1), 0, true
	}
	return 0, 0, false
}

// budget checks points over skills against first plus perLevel a level
// after the first in all, and perSkillFirst plus perSkillPerLevel a level
// after the first in one skill; a perSkillFirst of 0 checks no skill on its
// own (the bard's, step 18). No warning under level 1.
func budget(level int, points map[string]int, skills []string, first, perLevel, perSkillFirst, perSkillPerLevel int) []BudgetWarning {
	out := []BudgetWarning{}
	if level < 1 {
		return out
	}
	total, totalLimit := 0, first+perLevel*(level-1)
	skillLimit := perSkillFirst + perSkillPerLevel*(level-1)
	for _, skill := range skills {
		total += points[skill]
		if perSkillFirst > 0 && points[skill] > skillLimit {
			out = append(out, BudgetWarning{Code: BudgetPerSkill, Skill: skill, Points: points[skill], Limit: skillLimit})
		}
	}
	if total > totalLimit {
		out = append([]BudgetWarning{{Code: BudgetTotal, Points: total, Limit: totalLimit}}, out...)
	}
	return out
}

// BackstabFor is PHB Table 30 at a thief's level (ADR 0030 step 19): the
// multiplier on the weapon's damage dice. An error for a level outside the
// table.
func (t *Tables) BackstabFor(level int) (int, error) {
	c, err := t.classTables()
	if err != nil {
		return 0, err
	}
	for _, r := range c.Backstab {
		if level >= r.LevelMin && level <= r.LevelMax {
			return r.Multiplier, nil
		}
	}
	return 0, fmt.Errorf("backstab: level %d outside Table 30", level)
}
