package skillsvc

import (
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// abilityScore is the sheet's score for an ability key.
func abilityScore(c *charactersvc.Character, a Ability) (int, error) {
	switch a {
	case AbilityStr:
		return c.Str, nil
	case AbilityDex:
		return c.Dex, nil
	case AbilityCon:
		return c.Con, nil
	case AbilityInt:
		return c.Int, nil
	case AbilityWis:
		return c.Wis, nil
	case AbilityCha:
		return c.Cha, nil
	}
	return 0, fmt.Errorf("unknown ability %q", a)
}

// ProficiencyTarget is the ability score plus the proficiency's modifier
// (ADR 0014 step 1).
func ProficiencyTarget(score, modifier int) int {
	return score + modifier
}

// ProficiencySuccess reports a d20 at or under the target (ADR 0014 step
// 1), except a natural 20, which always fails (step 2).
func ProficiencySuccess(roll, target int) bool {
	return roll != ProficiencyDie && roll <= target
}

// BardSkillSuccess reports a d100 at or under the percentage (ADR 0014
// step 3).
func BardSkillSuccess(roll, percent int) bool {
	return roll <= percent
}
