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

// SkillUsable reports whether a skill of a set whose least percentage is minPercent can be
// rolled at percent (ADR 0030 step 17, PHB p.39: a thieving skill under 1% cannot be used until
// points raise it, whatever the DM's adjustment). The server owns this rule; the web reads it.
func SkillUsable(minPercent, percent int) bool {
	return percent >= minPercent
}

// CodeSkillUnusable is the FieldError code of a roll of a skill under its
// set's least percentage. The web owns the wording.
const CodeSkillUnusable = "skill_unusable"

// CodeSkillBlockedByArmour is the FieldError code of a roll of a skill the
// worn body armour blocks (ADR 0042 step 16.3): a multi-class thief in armour
// his thief class may not wear uses only Open Locks and Detect Noise. The web
// owns the wording.
const CodeSkillBlockedByArmour = "skill_blocked_by_armour"

// The skill set codes of the check kinds stored before the single rogueSkill kind (ADR 0040
// step 15, Q1): an entry logged as thiefSkill or bardSkill carries no set, and these two
// literals say which set it was rolled in. Reading only: nothing logs those kinds any more.
const (
	setThief = "thief"
	setBard  = "bard"
)

// setOf is the skill set a check was rolled in: the one it logged, or for an entry stored
// under a per-set kind the set that kind stood for.
func setOf(rec checkRecord) string {
	switch {
	case rec.Set != "":
		return rec.Set
	case rec.Kind == KindThiefSkill:
		return setThief
	case rec.Kind == KindBardSkill:
		return setBard
	}
	return ""
}

// percentKind reports a check on a d100 against a percentage: a skill of a skill set, or one of
// the kinds stored before them (ADR 0014 step 3, ADR 0030 step 17).
func percentKind(k Kind) bool {
	return k == KindRogueSkill || k == KindBardSkill || k == KindThiefSkill
}

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
