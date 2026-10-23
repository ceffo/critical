package ruletablesvc

import (
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// The thief's rules (ADR 0030 steps 17 and 19): the thieving skills from the
// entered points with the race, Dexterity and armour adjustments, the budget
// of the points, and the backstab multiplier. The numbers are the class
// tables' (PHB Tables 26 to 30); this file says which row and column a thief
// reads.

// ArmorLeather is Table 29's baseline (no column: every adjustment 0).
const ArmorLeather = "leather"

// Thieving skill limits (ADR 0030 step 17, PHB p.39), the same for every
// class with rogue_skills (ADR 0040 step 3.3).
const (
	// MaxSkillPercent caps a skill with every adjustment; MinSkillPercent
	// floors it.
	MaxSkillPercent = 95
	MinSkillPercent = 0
)

// SkillPercent is one thieving skill of ADR 0030 step 17 with its terms:
// Percent = Base + Points + Race + Dexterity + Armor, capped at
// MaxSkillPercent and floored at MinSkillPercent. Raw is that sum before the
// cap and the floor: it differs from Percent when one applied.
type SkillPercent struct {
	Skill                                string
	Base, Points, Race, Dexterity, Armor int
	Raw                                  int
	Percent                              int
	// Blocked: the skill is not used while the worn body armour blocks it
	// (ADR 0042 step 16.3): its check is refused before the die. Omitted
	// from JSON when false, so a single-class sheet reads as before.
	Blocked bool `json:",omitempty"`
}

// rogueSkills is the sum of ADR 0030 steps 17 and 18 for each of skills, in
// order: the base from set (the class's skill set), the entered points, Table 27
// for race, Table 28 for dex (a skill without a column: 0), Table 29's armor
// column (leather: 0) plus extra (the bard's metal armour penalty, step 18),
// capped at MaxSkillPercent and floored at MinSkillPercent.
func rogueSkills(c *ClassTables, skills []string, set catalogue.SkillSet, race string, dex int, armor string, extra int, points map[string]int) []SkillPercent {
	out := make([]SkillPercent, 0, len(skills))
	for _, skill := range skills {
		p := SkillPercent{Skill: skill, Points: points[skill], Dexterity: dexterityAdjustment(c, dex, skill), Armor: extra}
		for _, k := range set.Skills {
			if k.ID == skill {
				p.Base = k.Base
			}
		}
		for _, r := range c.SkillRace {
			if r.Race == race && r.Skill == skill {
				p.Race = r.Adjustment
			}
		}
		for _, r := range c.SkillArmor {
			if r.Armor == armor && r.Skill == skill {
				p.Armor += r.Adjustment
			}
		}
		p.Raw = p.Base + p.Points + p.Race + p.Dexterity + p.Armor
		p.Percent = min(max(p.Raw, MinSkillPercent), MaxSkillPercent)
		out = append(out, p)
	}
	return out
}

// dexterityAdjustment is Table 28's adjustment of skill at dex (0 for a skill
// without a row at that score).
func dexterityAdjustment(c *ClassTables, dex int, skill string) int {
	for _, r := range c.SkillDexterity {
		if r.Skill == skill && dex >= r.DexMin && dex <= r.DexMax {
			return r.Adjustment
		}
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

// BudgetWarning is a breach of the skill point budget (ADR 0030 step 17) by
// the points stored: a warning shown on the skills card. A save that grows
// it is refused (ADR 0037 step 10: skills.SkillPointsWithinBudget).
type BudgetWarning struct {
	Code   string
	Skill  string
	Points int
	Limit  int
}

// SkillBudgetLimits are the limits SkillBudgetOf checks at level (ADR 0030
// steps 17 and 18), the one source of both: the points in all, and in one
// skill (0: no limit on one skill, the bard's). ok is false without
// rogue_skills (rs nil) or for a level under 1.
func SkillBudgetLimits(rs *classes.RogueSkills, level int) (total, perSkill int, ok bool) {
	b, ok := SkillPointsFigures(rs, level, nil)
	return b.Total, b.PerSkill, ok
}

// SkillPointsBudget is the skill point budget of a thief or a bard at Level
// (ADR 0030 steps 17 and 18) with the terms of its formula, for the UI to
// show it: Total = First + PerLevel x (Level - 1) points in all, Spent
// entered, Left = Total - Spent (negative when over). PerSkill = PerSkillFirst
// + PerSkillPerLevel x (Level - 1) is the most one skill holds, all 0 for no
// such limit (the bard's).
type SkillPointsBudget struct {
	Level, First, PerLevel, Total, Spent, Left int
	PerSkillFirst, PerSkillPerLevel, PerSkill  int
}

// SkillPointsFigures is the budget of the rogue_skills params rs at level for
// the entered points (ADR 0030 steps 17 and 18, ADR 0040 step 3: the budget is
// the params'), the one source of the limits. ok is false without rogue_skills
// (rs nil) or for a level under 1.
func SkillPointsFigures(rs *classes.RogueSkills, level int, points map[string]int) (b SkillPointsBudget, ok bool) {
	if rs == nil || level < 1 {
		return SkillPointsBudget{}, false
	}
	b.Level = level
	b.First, b.PerLevel = rs.Budget.First, rs.Budget.PerLevel
	b.PerSkillFirst, b.PerSkillPerLevel = rs.Budget.PerSkillFirst, rs.Budget.PerSkillPerLevel
	if b.PerSkillFirst > 0 || b.PerSkillPerLevel > 0 {
		b.PerSkill = b.PerSkillFirst + b.PerSkillPerLevel*(level-1)
	}
	b.Total = b.First + b.PerLevel*(level-1)
	for _, skill := range SkillsOf(rs) {
		b.Spent += points[skill]
	}
	b.Left = b.Total - b.Spent
	return b, true
}

// SkillBudgetOf checks the entered points against the budget of rs at level
// (ADR 0030 steps 17 and 18): at most the budget's total in all and, when the
// budget has one, its per-skill limit in one skill. The points are entered as
// one total per skill, so the check is on the totals the levels allow. Skills
// in the set's order; an empty list without rogue_skills. The warning of
// points stored over it (a save that grows it is refused, ADR 0037 step 10:
// skills.SkillPointsWithinBudget).
func SkillBudgetOf(rs *classes.RogueSkills, level int, points map[string]int) []BudgetWarning {
	total, perSkill, ok := SkillBudgetLimits(rs, level)
	if !ok {
		return []BudgetWarning{}
	}
	return budget(level, points, SkillsOf(rs), total, perSkill)
}

// SkillPointsGainedAt is ADR 0037 step 15 on ADR 0030 steps 17 and 18: the
// skill points level grants with rogue_skills rs, the budget at level minus
// the budget at the level below. None at level 1 (its points are the first
// level's) or under, nor without rogue_skills.
func SkillPointsGainedAt(rs *classes.RogueSkills, level int) int {
	if level < 2 {
		return 0
	}
	now, ok := SkillPointsFigures(rs, level, nil)
	if !ok {
		return 0
	}
	before, _ := SkillPointsFigures(rs, level-1, nil)
	return now.Total - before.Total
}

// budget checks points over skills against totalLimit in all and perSkill in
// one skill; a perSkill of 0 checks no skill on its own (the bard's, step 18).
// No warning under level 1.
func budget(level int, points map[string]int, skills []string, totalLimit, perSkill int) []BudgetWarning {
	out := []BudgetWarning{}
	if level < 1 {
		return out
	}
	total := 0
	for _, skill := range skills {
		total += points[skill]
		if perSkill > 0 && points[skill] > perSkill {
			out = append(out, BudgetWarning{Code: BudgetPerSkill, Skill: skill, Points: points[skill], Limit: perSkill})
		}
	}
	if total > totalLimit {
		out = append([]BudgetWarning{{Code: BudgetTotal, Points: total, Limit: totalLimit}}, out...)
	}
	return out
}

// BackstabFor is PHB Table 30 at the level of a class with backstab (ADR 0030 step 19): the
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
