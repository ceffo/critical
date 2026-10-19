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

// ProficiencyDef is a stored non-weapon proficiency: the ability it is
// checked against and the modifier of PHB Table 37.
type ProficiencyDef struct {
	ID       string
	Ability  Ability
	Modifier int
}

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

// Proficiency is a proficiency with its target (ability score + modifier).
type Proficiency struct {
	ID           string
	Ability      Ability
	AbilityScore int
	Modifier     int
	Target       int
	Last         *LastCheck
}

// BardSkill is a bard skill (ADR 0030 step 18): its percentage, derived by
// the character service with its terms, and the last check rolled for it.
type BardSkill struct {
	ruletablesvc.SkillPercent
	Last *LastCheck
}

// ThiefSkill is a thieving skill (ADR 0030 step 17): its percentage, derived
// by the character service with its terms, and the last check rolled for it.
type ThiefSkill struct {
	ruletablesvc.SkillPercent
	Last *LastCheck
}

// Skills is the whole Compétences tab.
type Skills struct {
	Proficiencies []Proficiency
	BardSkills    []BardSkill
	ThiefSkills   []ThiefSkill
	// SkillArmor is the skill armour the bard or thief skills read (ADR
	// 0033 step 14: from the worn body armour), "" for another class.
	SkillArmor string
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
	LogEntryID       int64
}
