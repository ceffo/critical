package ruletablesvc

import (
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// The rogue_skills feature kind (ADR 0040 step 3 on ADR 0030 steps 17 and
// 18): a skill set's skills from the entered points with the race, Dexterity
// and armour adjustments (PHB Tables 27 to 29), the set's base table (Table
// 26 or 33), and the armour entries of the class's params. Which class has
// which set is the catalogue's: this file reads the params.

// Skill set codes, the rogue_skills `set` param (ADR 0040 step 3): the
// skill list and base rows a set reads (and the check kind it logs, step
// 3.2).
const (
	// SkillSetThief is the eight thieving skills of PHB Table 26.
	SkillSetThief = "thief"
	// SkillSetBard is the four bard skills of PHB Table 33.
	SkillSetBard = "bard"
)

// ArmorMetal is the skill armour of the bard's "metal armour up to chain
// mail" (ADR 0030 step 1): an armour entry with an extra adjustment is its
// own skill armour, named by its code (SkillArmorFor).
const ArmorMetal = "metal"

// SkillsOf are the skills of the rogue_skills params rs, in its set's table
// order; nil without rogue_skills (rs nil) or for an unknown set.
func SkillsOf(rs *classes.RogueSkills) []string {
	if rs == nil {
		return nil
	}
	switch rs.Set {
	case SkillSetThief:
		return ThiefSkills
	case SkillSetBard:
		return BardSkills
	}
	return nil
}

// CheckSkillSet refuses a rogue_skills set this package does not know (ADR
// 0040 step 8).
func CheckSkillSet(set string) error {
	if set != SkillSetThief && set != SkillSetBard {
		return fmt.Errorf("unknown skill set %q", set)
	}
	return nil
}

// RogueSkills is ADR 0030 steps 17 and 18 for every skill of rs's set, in
// order: the set's base table (Table 26 or 33), the entered points (absent:
// 0), Table 27 for race ("" or a race without rows: 0), Table 28 for dex (a
// skill without a column: 0), and the armour entry whose skill armour
// (SkillArmorFor) is armor: Table 29's column it reads (leather: 0) plus its
// extra; capped at MaxSkillPercent and floored at MinSkillPercent. An armor
// that is no entry's skill armour is an error.
func (t *Tables) RogueSkills(rs classes.RogueSkills, race string, dex int, armor string, points map[string]int) ([]SkillPercent, error) {
	c, err := t.classTables()
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(c.SkillSets, func(s catalogue.SkillSet) bool { return s.ID == rs.Set })
	if i < 0 {
		return nil, fmt.Errorf("rogue skills: unknown skill set %q", rs.Set)
	}
	set := c.SkillSets[i]
	for _, a := range rs.Armour {
		if skillArmorOf(a) == armor {
			column := a.Column
			if column == "" {
				column = ArmorLeather
			}
			return rogueSkills(c, SkillsOf(&rs), set, race, dex, column, a.Extra, points), nil
		}
	}
	return nil, fmt.Errorf("%s skills: unknown armour %q", rs.Set, armor)
}
