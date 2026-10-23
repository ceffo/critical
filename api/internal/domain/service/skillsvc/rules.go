package skillsvc

import (
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// abilityScore is the sheet's score for an ability key.
func abilityScore(c charactersvc.View, a Ability) (int, error) {
	score, ok := c.Score(a)
	if !ok {
		return 0, fmt.Errorf("unknown ability %q", a)
	}
	return score, nil
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

// MinThiefSkillPercent is the least percentage a thieving skill is used at
// (ADR 0030 step 17, PHB p.39): under it the player must spend points first.
const MinThiefSkillPercent = 1

// SkillUsable reports whether a skill of kind at percent can be rolled: a
// thieving skill under MinThiefSkillPercent cannot, whatever else (the
// server owns this rule; the web reads it).
func SkillUsable(kind Kind, percent int) bool {
	return kind != KindThiefSkill || percent >= MinThiefSkillPercent
}

// CodeSkillUnusable is the FieldError code of a roll of a thieving skill
// under MinThiefSkillPercent. The web owns the wording.
const CodeSkillUnusable = "skill_unusable"

// CodeSkillBlockedByArmour is the FieldError code of a roll of a skill the
// worn body armour blocks (ADR 0042 step 16.3): a multi-class thief in armour
// his thief class may not wear uses only Open Locks and Detect Noise. The web
// owns the wording.
const CodeSkillBlockedByArmour = "skill_blocked_by_armour"

// The skill set codes whose check kind is still one per set (ADR 0040 step 3.2): the single
// rogueSkill kind (step 15) removes them.
const (
	setThief = "thief"
	setBard  = "bard"
)

// KindOfSet is the check kind a skill set logs (ADR 0040 step 3.2, ADR 0014 step 3): the set
// code is a rogue_skills param, not a class id. ok is false for a set this package does not
// know.
func KindOfSet(set string) (Kind, bool) {
	switch set {
	case setThief:
		return KindThiefSkill, true
	case setBard:
		return KindBardSkill, true
	}
	return "", false
}

// percentKind reports a check on a d100 against a percentage: a bard skill
// (ADR 0014 step 3) or a thieving skill (ADR 0030 step 17).
func percentKind(k Kind) bool { return k == KindBardSkill || k == KindThiefSkill }

// MaxAdjustment is the largest adjustment, either sign, of a check of kind:
// a percentage check's is in percentage points.
func MaxAdjustment(k Kind) int {
	if percentKind(k) {
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
// target), a bard or thieving skill per BardSkillSuccess.
func Success(k Kind, roll, target int) bool {
	if percentKind(k) {
		return BardSkillSuccess(roll, target)
	}
	return ProficiencySuccess(roll, target)
}

// skillDie is the die of a skill set's check of kind (ADR 0014 step 3).
func skillDie(k Kind) int {
	if k == KindThiefSkill {
		return ThiefSkillDie
	}
	return BardSkillDie
}
