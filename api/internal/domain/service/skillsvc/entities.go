package skillsvc

// Ability is the key of an ability score on the sheet.
type Ability string

// Ability keys, as stored in the proficiencies table.
const (
	AbilityStr Ability = "str"
	AbilityDex Ability = "dex"
	AbilityCon Ability = "con"
	AbilityInt Ability = "int"
	AbilityWis Ability = "wis"
	AbilityCha Ability = "cha"
)

// Kind tells a proficiency check from a bard skill check.
type Kind string

// Check kinds.
const (
	KindProficiency Kind = "proficiency"
	KindBardSkill   Kind = "bardSkill"
)

// Dice of each kind (ADR 0014 steps 1 and 3).
const (
	ProficiencyDie = 20
	BardSkillDie   = 100
)

// ProficiencyDef is a stored non-weapon proficiency: the ability it is
// checked against and the modifier of PHB Table 37.
type ProficiencyDef struct {
	ID       string
	Ability  Ability
	Modifier int
}

// BardSkillDef is a stored bard skill and its percentage (sheet value, all
// adjustments included).
type BardSkillDef struct {
	ID      string
	Percent int
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

// BardSkill is a bard skill with the last check rolled for it.
type BardSkill struct {
	ID      string
	Percent int
	Last    *LastCheck
}

// Skills is the whole Compétences tab.
type Skills struct {
	Proficiencies []Proficiency
	BardSkills    []BardSkill
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
