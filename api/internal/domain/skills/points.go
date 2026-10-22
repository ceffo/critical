package skills

import (
	"fmt"
	"maps"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// A thief's or a bard's skill points (ADR 0030 steps 17 and 18) and their budget (ADR 0037 step
// 10).

// MaxSkillPoints bounds a thief's points in one skill (ADR 0030 step 17): more could never
// count, a skill being capped at 95%. The budget of PHB p.39 is a warning (PointsFigures.Budget),
// not a bound.
const MaxSkillPoints = ruletablesvc.MaxSkillPercent

// RogueSkillsAt is ADR 0040 step 5 for rogue_skills: the params the class grants at level, nil
// when it grants none there.
func RogueSkillsAt(def classes.Class, level int) *classes.RogueSkills {
	f, ok := def.Feature(classes.KindRogueSkills, level)
	if !ok {
		return nil
	}
	return f.RogueSkills
}

// rogueSkillsListed are the rogue_skills params the class lists at any level, nil for none: what an
// entry of points is checked against before the level is read.
func rogueSkillsListed(def classes.Class) *classes.RogueSkills {
	f, ok := def.Lists(classes.KindRogueSkills)
	if !ok {
		return nil
	}
	return f.RogueSkills
}

// ValidSkillPoints is ADR 0030 steps 17 and 18's entry check on a thief's or a bard's points, at
// creation and when the player spends them later: each skill of the class's rogue_skills set
// (ADR 0040 step 3) from 0 to MaxSkillPoints. The budget is checked apart, at the realized level
// (SkillPointsWithinBudget). The points come back without the zeros, nil for none. A class
// without skill points enters none: any point is refused with field skillPoints; another
// refusal names the field skillPoints.<skill>.
func ValidSkillPoints(def classes.Class, points map[string]int) (map[string]int, error) {
	rs := rogueSkillsListed(def)
	if rs == nil {
		if len(points) > 0 {
			return nil, &domain.FieldError{Field: FieldSkillPoints, Message: CodeNotAllowed}
		}
		return nil, nil
	}
	var out map[string]int
	for _, skill := range slices.Sorted(maps.Keys(points)) {
		pts := points[skill]
		switch {
		case !slices.Contains(ruletablesvc.SkillsOf(rs), skill):
			return nil, &domain.FieldError{Field: FieldSkillPoints + "." + skill, Message: CodeUnknown}
		case pts < 0 || pts > MaxSkillPoints:
			return nil, &domain.FieldError{Field: FieldSkillPoints + "." + skill, Message: CodeOutOfRange}
		case pts == 0:
			continue
		}
		if out == nil {
			out = map[string]int{}
		}
		out[skill] = pts
	}
	return out, nil
}

// SkillPointsWithinBudget is ADR 0037 step 10 on a thief's or a bard's skill points (amending
// ADR 0030 steps 17 and 18): a save from before to after at level is refused when it leaves the
// total over the level's budget and above the total before, or (the thief's per-skill cap of
// step 17) one skill over the cap and above its points before. Any other save passes: an
// existing overspend that shrinks or stays keeps the derived warning, and points left to spend
// are never refused. At creation before is nil (0 spent). The refusal is a *domain.BudgetError
// (field skillPoints for the total, skillPoints.<skill> for one skill), the total first, then
// the skills in SkillsOf order. A class without skill points, or a level under 1, has no
// budget: nil.
func SkillPointsWithinBudget(def classes.Class, level int, before, after map[string]int) error {
	rs := RogueSkillsAt(def, level)
	b, ok := ruletablesvc.SkillPointsFigures(rs, level, after)
	if !ok {
		return nil
	}
	was, _ := ruletablesvc.SkillPointsFigures(rs, level, before)
	if b.Spent > b.Total && b.Spent > was.Spent {
		return &domain.BudgetError{Field: FieldSkillPoints, Spent: b.Spent, Limit: b.Total}
	}
	if b.PerSkill == 0 {
		return nil
	}
	for _, skill := range ruletablesvc.SkillsOf(rs) {
		if pts := after[skill]; pts > b.PerSkill && pts > before[skill] {
			return &domain.BudgetError{Field: FieldSkillPoints + "." + skill, Spent: pts, Limit: b.PerSkill}
		}
	}
	return nil
}

// SkillTables are PHB Tables 26 to 29 and 33 on the entered points (ADR 0030 steps 17 and 18),
// read through a class's rogue_skills params (ADR 0040 step 3); ruletablesvc.Tables satisfies it.
type SkillTables interface {
	RogueSkills(rs classes.RogueSkills, race string, dex int, armor string, points map[string]int) ([]ruletablesvc.SkillPercent, error)
}

// PointsSheet is what a thief's or a bard's skills read of the character: its class, race,
// Dexterity, realized level, skill armour (ruletablesvc.SkillArmorFor the worn body armour, ADR
// 0033 step 14) and entered points.
type PointsSheet struct {
	Class      classes.Class
	Race       string
	Dex, Level int
	Armor      string
	Points     map[string]int
}

// PointsFigures are the skills a thief's or a bard's points give (ADR 0030 steps 17 and 18):
// each skill's percentage with its terms (Thief or Bard), the budget's warnings and the budget
// as figures at the realized level.
type PointsFigures struct {
	Thief, Bard []ruletablesvc.SkillPercent
	Budget      []ruletablesvc.BudgetWarning
	Points      *ruletablesvc.SkillPointsBudget
}

// PointsFiguresOf is ADR 0030 steps 17 and 18 on s: the percentages from the tables, the budget
// and its warnings, through the rogue_skills params the class grants at s.Level (ADR 0040 step
// 3); the set picks the field the percentages fill (Thief or Bard, the derived block's). The zero
// figures for a class without rogue_skills.
func PointsFiguresOf(t SkillTables, s PointsSheet) (PointsFigures, error) {
	var out PointsFigures
	rs := RogueSkillsAt(s.Class, s.Level)
	if rs == nil {
		return out, nil
	}
	percents, err := t.RogueSkills(*rs, s.Race, s.Dex, s.Armor, s.Points)
	if err != nil {
		return PointsFigures{}, fmt.Errorf("%s skills: %w", s.Class.ID, err)
	}
	switch rs.Set {
	case ruletablesvc.SkillSetThief:
		out.Thief = percents
	case ruletablesvc.SkillSetBard:
		out.Bard = percents
	}
	out.Budget = ruletablesvc.SkillBudgetOf(rs, s.Level, s.Points)
	if b, ok := ruletablesvc.SkillPointsFigures(rs, s.Level, s.Points); ok {
		out.Points = &b
	}
	return out, nil
}
