package skills

import (
	"fmt"
	"maps"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// A thief's or a bard's skill points (ADR 0030 steps 17 and 18) and their budget (ADR 0037 step
// 10).

// MaxSkillPoints bounds a thief's points in one skill (ADR 0030 step 17): more could never
// count, a skill being capped at 95%. The budget of PHB p.39 is a warning (PointsFigures.Budget),
// not a bound.
const MaxSkillPoints = ruletablesvc.MaxSkillPercent

// SetPoints are a character's entered skill points per skill set: the set code (the
// rogue_skills `set` param of the feature that grants it, ADR 0040 step 3.5) to the points of
// each of its skills. Each set has its own budget, so the points of two sets that share a skill
// (a thief's and a bard's pick pockets) never mix.
type SetPoints map[string]map[string]int

// Of are the points of a set, nil for none.
func (p SetPoints) Of(set string) map[string]int { return p[set] }

// With is p with the points of set replaced by points (nil or empty: none), without mutating p.
// nil when no set holds a point.
func (p SetPoints) With(set string, points map[string]int) SetPoints {
	var out SetPoints
	for k, v := range p {
		if k != set && len(v) > 0 {
			if out == nil {
				out = SetPoints{}
			}
			out[k] = maps.Clone(v)
		}
	}
	if len(points) > 0 {
		if out == nil {
			out = SetPoints{}
		}
		out[set] = maps.Clone(points)
	}
	return out
}

// LegacySet is the key the reader files points under when only the legacy flat
// characters.skill_points holds them (a release from before skill sets kept writing it): they
// belong to the class's first set, which only the class definition names (Resolve).
const LegacySet = ""

// Resolve is p with the points filed under LegacySet moved to the first skill set def lists,
// unless that set already has points (or def lists none: they are dropped). p when it has none.
func (p SetPoints) Resolve(def classes.Class) SetPoints {
	legacy, ok := p[LegacySet]
	if !ok {
		return p
	}
	out := p.Clone()
	delete(out, LegacySet)
	if set := FirstSet(def); set != "" && len(out[set]) == 0 && len(legacy) > 0 {
		out[set] = legacy
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Clone is a deep copy of p.
func (p SetPoints) Clone() SetPoints {
	var out SetPoints
	for k, v := range p {
		if out == nil {
			out = SetPoints{}
		}
		out[k] = maps.Clone(v)
	}
	return out
}

// RogueSkillsAt is ADR 0040 step 5 for rogue_skills: the params of the first skill set the
// class grants at level, nil when it grants none there. A class that grants several is read
// with RogueSetsAt.
func RogueSkillsAt(def classes.Class, level int) *classes.RogueSkills {
	if sets := RogueSetsAt(def, level); len(sets) > 0 {
		return &sets[0]
	}
	return nil
}

// RogueSetsAt is ADR 0040 steps 3.5 and 5 for rogue_skills: the params of every skill set the
// class grants at level, in the class's order (one per rogue_skills feature); nil for none.
func RogueSetsAt(def classes.Class, level int) []classes.RogueSkills {
	var out []classes.RogueSkills
	for _, f := range def.FeaturesAt(classes.KindRogueSkills, level) {
		out = append(out, *f.RogueSkills)
	}
	return out
}

// FirstSet is the code of the first skill set the class lists at any level, "" for a class
// without one: the set a creation enters its points in.
func FirstSet(def classes.Class) string {
	if f, ok := def.Lists(classes.KindRogueSkills); ok {
		return f.RogueSkills.Set
	}
	return ""
}

// rogueSkillsListed are the rogue_skills params of set that the class lists at any level, nil
// for none: what an entry of points is checked against before the level is read.
func rogueSkillsListed(def classes.Class, set string) *classes.RogueSkills {
	for _, f := range def.ListsAll(classes.KindRogueSkills) {
		if f.RogueSkills.Set == set {
			return f.RogueSkills
		}
	}
	return nil
}

// rogueSkillsAtFor are the params of set the class grants at level, nil for none.
func rogueSkillsAtFor(def classes.Class, set string, level int) *classes.RogueSkills {
	for _, rs := range RogueSetsAt(def, level) {
		if rs.Set == set {
			return &rs
		}
	}
	return nil
}

// ValidSkillPoints is ADR 0030 steps 17 and 18's entry check on the points of one skill set of
// a thief, a bard or any class with rogue_skills, at creation and when the player spends them
// later: each skill of the class's rogue_skills feature of that set
// (ADR 0040 step 3) from 0 to MaxSkillPoints. The budget is checked apart, at the realized level
// (SkillPointsWithinBudget). The points come back without the zeros, nil for none. A class
// without that set enters none: any point is refused with field skillPoints; another
// refusal names the field skillPoints.<skill>.
func ValidSkillPoints(def classes.Class, set string, points map[string]int) (map[string]int, error) {
	rs := rogueSkillsListed(def, set)
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
// budget: nil. The budget reads the grant ValidSkillPoints checked the entry against (the
// listed rogue_skills, ADR 0040 step 3): below its from_level the class has the skills but no
// points to spend yet (the derive shows none), so the budget is 0 and any spend above before
// is refused.
func SkillPointsWithinBudget(def classes.Class, set string, level int, before, after map[string]int) error {
	listed := rogueSkillsListed(def, set)
	if listed == nil || level < 1 {
		return nil
	}
	rs := rogueSkillsAtFor(def, set, level)
	if rs == nil {
		spent, was := 0, 0
		for _, skill := range ruletablesvc.SkillsOf(listed) {
			spent, was = spent+after[skill], was+before[skill]
		}
		if spent > 0 && spent > was {
			return &domain.BudgetError{Field: FieldSkillPoints, Spent: spent, Limit: 0}
		}
		return nil
	}
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

// PointsSheet is what a character's skill sets read of the character: its class, race,
// Dexterity, realized level, the worn body armour's Table 29 column (ADR 0033 step 14: each set
// reads it through its own armour entries, ruletablesvc.SkillArmorFor) and entered points per set.
// MultiClass is set for a character of several class entries (ADR 0042 step 16.3).
type PointsSheet struct {
	Class       classes.Class
	Race        string
	Dex, Level  int
	ArmorColumn string
	Points      SetPoints
	MultiClass  bool
}

// ArmourBlocks is ADR 0042 step 16.3: a multi-class character wearing body armour of Table 29
// column column that its skill class's equipment (ADR 0040 step 1.6) does not allow uses only
// the set's armour-free skills (RogueSkills.ArmourFree). A single-class thief or bard is unchanged (its armour
// outside the list is ADR 0033 step 20's warning), and no body armour never blocks.
func (s PointsSheet) ArmourBlocks() bool {
	column := s.ArmorColumn
	return s.MultiClass && column != "" && column != ruletablesvc.ColumnNone && !s.Class.Equipment.AllowsArmour(column)
}

// SkillSet is one skill set of a character (ADR 0040 step 3.5: one per rogue_skills feature
// its classes grant, ADR 0030 steps 17 and 18): the set code, each skill's percentage with its
// terms, the skill armour the set reads, the budget's warnings and the budget as figures at the
// realized level. Each set has its own budget.
type SkillSet struct {
	Set string
	// Name is the set's heading and MinPercent the least percentage a skill of it is used at
	// (ADR 0040 step 15), from its rogue_skills params.
	Name       localized.Text
	MinPercent int
	Skills     []ruletablesvc.SkillPercent
	Armor      string
	Budget     []ruletablesvc.BudgetWarning
	Points     *ruletablesvc.SkillPointsBudget
	// Dormant: the set is a dual-class's old class's before the crossover
	// (ADR 0042 step 13): rolled, warning dual_class_old_ability. Set by the
	// character's derivation from its grant. Domain only, not on the wire.
	Dormant bool `json:"-"`
}

// SkillSetsOf is ADR 0030 steps 17 and 18 on s: for each skill set the class grants at s.Level
// (ADR 0040 step 3.5), the percentages from the tables, the budget and its warnings, through
// that set's rogue_skills params. nil for a class without rogue_skills. When the armour blocks
// the set (PointsSheet.ArmourBlocks, ADR 0042 step 16.3), the set's armour-free skills take the
// no-armour percentage (Q16) and every other skill is Blocked.
func SkillSetsOf(t SkillTables, s PointsSheet) ([]SkillSet, error) {
	var out []SkillSet
	for _, rs := range RogueSetsAt(s.Class, s.Level) {
		set := SkillSet{Set: rs.Set, Name: rs.Name, MinPercent: rs.MinPercent, Armor: ruletablesvc.SkillArmorFor(&rs, s.ArmorColumn)}
		points := s.Points.Of(rs.Set)
		var err error
		if set.Skills, err = t.RogueSkills(rs, s.Race, s.Dex, set.Armor, points); err != nil {
			return nil, fmt.Errorf("%s skills (%s): %w", s.Class.ID, rs.Set, err)
		}
		if s.ArmourBlocks() {
			if err := blockSkills(t, s, rs, points, set.Skills); err != nil {
				return nil, err
			}
		}
		set.Budget = ruletablesvc.SkillBudgetOf(&rs, s.Level, points)
		if b, ok := ruletablesvc.SkillPointsFigures(&rs, s.Level, points); ok {
			set.Points = &b
		}
		out = append(out, set)
	}
	return out, nil
}

// blockSkills is ADR 0042 step 16.3 on the percentages skills of set rs read in armour that
// blocks it: the set's armour-free skills take their no-armour percentage (Q16), every other
// skill is Blocked.
func blockSkills(t SkillTables, s PointsSheet, rs classes.RogueSkills, points map[string]int, skills []ruletablesvc.SkillPercent) error {
	free, err := t.RogueSkills(rs, s.Race, s.Dex, ruletablesvc.SkillArmorFor(&rs, ruletablesvc.ColumnNone), points)
	if err != nil {
		return fmt.Errorf("%s skills (%s) without armour: %w", s.Class.ID, rs.Set, err)
	}
	for i := range skills {
		if !slices.Contains(rs.ArmourFree, skills[i].Skill) {
			skills[i].Blocked = true
			continue
		}
		for _, f := range free {
			if f.Skill == skills[i].Skill {
				skills[i] = f
			}
		}
	}
	return nil
}
