package skillsvc

import (
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// abilityScore is the sheet's score for an ability key.
func abilityScore(c *charactersvc.Character, a Ability) (int, error) {
	score, ok := c.Score(a)
	if !ok {
		return 0, fmt.Errorf("unknown ability %q", a)
	}
	return score, nil
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

// Adjustment limits (ADR 0014 step 7): a proficiency's adjustment is in d20
// points, a bard skill's in percentage points.
const (
	MaxProficiencyAdjustment = 20
	MaxBardSkillAdjustment   = 100
	MaxAdjustmentReasonLen   = 80
)

// MaxAdjustment is the largest adjustment, either sign, of a check of kind.
func MaxAdjustment(k Kind) int {
	if k == KindBardSkill {
		return MaxBardSkillAdjustment
	}
	return MaxProficiencyAdjustment
}

// AdjustedTarget moves the target by the adjustment, + easier, - harder
// (ADR 0014 step 5). Plain arithmetic, no clamp: a target at or under 0
// always fails, one of 100 or more on a d100 always succeeds.
func AdjustedTarget(target, adjustment int) int {
	return target + adjustment
}

// Success is the verdict of a roll of a check of kind against target: a
// proficiency per ProficiencySuccess (a natural 20 fails whatever the
// target), a bard skill per BardSkillSuccess.
func Success(k Kind, roll, target int) bool {
	if k == KindBardSkill {
		return BardSkillSuccess(roll, target)
	}
	return ProficiencySuccess(roll, target)
}
