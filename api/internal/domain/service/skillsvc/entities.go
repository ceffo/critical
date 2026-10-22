package skillsvc

import (
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// Ability is the key of an ability score on the sheet.
type Ability = charactersvc.Ability

// Ability keys, as stored in the proficiencies table.
const (
	AbilityStr = charactersvc.AbilityStr
	AbilityDex = charactersvc.AbilityDex
	AbilityCon = charactersvc.AbilityCon
	AbilityInt = charactersvc.AbilityInt
	AbilityWis = charactersvc.AbilityWis
	AbilityCha = charactersvc.AbilityCha
)

// Kind tells a proficiency check from a bard or thief skill check.
type Kind string

// Check kinds.
const (
	KindProficiency Kind = "proficiency"
	KindBardSkill   Kind = "bardSkill"
	// KindThiefSkill is a thieving skill (ADR 0030 step 17): a d100 at or
	// under the derived percentage, as a bard skill (ADR 0014 step 3).
	KindThiefSkill Kind = "thiefSkill"
)

// Dice of each kind (ADR 0014 steps 1 and 3).
const (
	ProficiencyDie = 20
	BardSkillDie   = 100
	ThiefSkillDie  = 100
)

// LastCheck is the newest non-voided check of a skill, with the adjustment
// and verdict in effect. Target is the unadjusted target.
type LastCheck struct {
	EntryID          int64
	Roll             int
	Target           int
	Adjustment       int
	AdjustmentReason string
	AdjustedTarget   int
	Success          bool
}

// Proficiency is a picked proficiency with a check and its target: ability
// score + the catalogue's modifier + the slots spent beyond the cost (ADR
// 0014 step 1 as amended by ADR 0037 step 5).
type Proficiency struct {
	ID           string
	Ability      Ability
	AbilityScore int
	Modifier     int
	ExtraSlots   int
	Target       int
	Last         *LastCheck
}

// SetSkill is a skill of a skill set (ADR 0030 steps 17 and 18): its percentage, derived by the
// character service with its terms, and the last check rolled for it in that set.
type SetSkill struct {
	ruletablesvc.SkillPercent
	Last *LastCheck
}

// SkillSet is one skill set of the Skills tab (ADR 0040 step 3.5): its code, the check kind it
// logs (ADR 0040 step 3.2), the skill armour it reads (ADR 0033 step 14), its budget with the
// terms of its formula (nil below the feature's level of first points, ADR 0030 steps 17 and 18)
// and its skills.
type SkillSet struct {
	Set    string
	Kind   Kind
	Armor  string
	Budget *ruletablesvc.SkillPointsBudget
	Skills []SetSkill
}

// Skills is the whole Compétences tab: the proficiencies and one skill set per rogue_skills
// feature the character's class grants.
type Skills struct {
	Proficiencies []Proficiency
	SkillSets     []SkillSet
}

// Adjustment is the DM's adjustment of one check (ADR 0014 steps 5 to 7):
// it moves the target, + easier, - harder; the roll never changes. Reason is
// optional ("slippery rope").
type Adjustment struct {
	Value  int
	Reason string
}

// CheckResult is one check with the adjustment in effect. Target is the
// unadjusted target, AdjustedTarget = Target + Adjustment the one the roll is
// judged against; Success is the verdict in effect, RolledSuccess the verdict
// as rolled (with the adjustment set before the roll). Ability, AbilityScore
// and Modifier are set for a proficiency only.
type CheckResult struct {
	ID               string
	Kind             Kind
	Die              int
	Roll             int
	Target           int
	Adjustment       int
	AdjustmentReason string
	AdjustedTarget   int
	Success          bool
	RolledSuccess    bool
	Ability          Ability
	AbilityScore     int
	Modifier         int
	// ExtraSlots are a proficiency's slots beyond its cost, each +1 to the
	// target (ADR 0037 step 5); 0 for a skill and in older entries.
	ExtraSlots int
	LogEntryID int64
	// Buff is the buff the verdict in effect put on (ADR 0037 step 20), nil
	// when it put none.
	Buff *AppliedBuff
	// Warnings are the check's class warnings, logged with it, never a
	// refusal (ADR 0042 step 13: dual_class_old_ability).
	Warnings []string
}
