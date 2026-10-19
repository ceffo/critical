package charactersvc

import "github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"

// Character is the pure domain representation of a character row. No transport
// tags — handlers map to a DTO before serializing.
//
// The reader fills the entered values (ADR 0030 step 1) and Sheet; the
// service fills Derived (steps 3 to 8): level, THAC0, saves and slots are never
// entered.
type Character struct {
	ID   int64
	Name string
	Str  int
	// StrExceptional is a fighter's exceptional strength (ADR 0030 step 9):
	// the percentile of an 18/xx, 1 to 100 (100 = 18/00); 0 is none.
	StrExceptional int
	Dex            int
	Con            int
	Int            int
	Wis            int
	Cha            int
	HPMax          int
	// RealizedLevel is the level the character has taken (ADR 0030 step 7):
	// its benefits (max HP, THAC0, saves, slots) are this level's. It starts
	// at the level the entered max HP is for (step 7.5: an imported sheet's
	// level, a created character's starting level) and only the level-up
	// step moves it, one level per roll. 0 is none yet (being created).
	RealizedLevel int
	// XPTotal is the starting XP, the sheet's (ADR 0015 step 1); the XP total
	// adds the ledger's entries to it (Derived.XPTotal).
	XPTotal int

	// Class is the class code (e.g. "bard"), read-only.
	Class string
	// Race is the race code (one of Races), "" when none is set.
	Race string
	// SkillPoints are a thief's or a bard's entered skill points (ADR 0030
	// steps 1, 17 and 18), by skill code (ruletablesvc.SkillsOf the class);
	// nil or absent skills have 0.
	SkillPoints map[string]int
	// SkillArmor is the armour a thief or a bard wears for its skills (step
	// 1, one of ruletablesvc.SkillArmorsOf the class), "" for a character
	// without skill points or none entered (read as leather).
	SkillArmor string

	// Sheet is what the character sheet stated when the character was
	// imported; never an input. Only its level is read: the realized level of
	// a character with none stored (ADR 0030 step 7.5).
	Sheet Sheet
	// Derived are the values the rules derive from class, race, abilities and
	// XP; set by the service, zero as the reader returns it.
	Derived Derived

	Details Details
	// Warnings are the rules the stored character breaks without being
	// refused (ADR 0032 steps 4.2 and 8.4); set by the service.
	Warnings []Warning
}

// Races are the race codes of the six PHB races (ADR 0030 step 1).
var Races = ruletablesvc.RaceCodes

// Saves are the five saving throw numbers, one per category of ADR 0006 step
// 1 (targets, or a bonus per category).
type Saves struct {
	ParaPoison   int
	RodStaffWand int
	PetriPoly    int
	Breath       int
	Spell        int
}

// Sheet are the figures an imported character's sheet stated (Kelsen's seed):
// kept as imported, never read as inputs (ADR 0030 step 24, retired); the level
// stands in for a missing realized level (step 7.5).
type Sheet struct {
	Level int
	THAC0 int
	Saves Saves
}

// Derived are a character's values derived by the rules (ADR 0030 steps 3 to
// 8 and 22.1).
type Derived struct {
	// XPTotal is the starting XP plus every ledger entry (ADR 0015 step 1).
	XPTotal int
	// TargetLevel is ADR 0030 step 3: the class's experience table at
	// XPTotal. Never stored; it drives the pending level-ups (step 7) and the
	// XP progress, no benefit.
	TargetLevel int
	// RealizedLevel is the stored level the benefits follow (step 7):
	// THAC0, saves and slots below are this level's.
	RealizedLevel int
	// NextLevelXP is the XP that reaches TargetLevel + 1; nil at the last
	// level.
	NextLevelXP *int
	// THAC0 is step 5 (PHB Table 53) at the realized level.
	THAC0 int
	// Saves are the targets of step 6 (PHB Table 60) at the realized level.
	Saves Saves
	// RaceSaveBonus is step 22.1: the race's bonus per category, a named
	// modifier on the roll (0: none), never a change of the target.
	RaceSaveBonus Saves
	// SpellSlots is step 8 at the realized level: the slots per day by
	// spell level, levels with no slot absent.
	SpellSlots map[int]int
	// LevelUp is step 7: the levels the target is above the realized one.
	LevelUp LevelUp
	// Specializes is step 12: the class may specialize in a weapon (the
	// weapon editor offers the flag).
	Specializes bool
	// Spellbook is step 20: the learning rules of a class with a spellbook
	// (mage, bard); nil for the others.
	Spellbook *SpellbookRules
	// SpellFailure is step 14: the percent chance a cast fails (one d100 at
	// each cast, at or under it); 0 for no die (Wisdom 13 and up, a class
	// without it).
	SpellFailure int
	// Turning is step 16: PHB Table 61 at the realized level, the book's
	// rows in order; nil for a class that does not turn undead.
	Turning []ruletablesvc.TurningTarget
	// ThiefSkills is step 17: each thieving skill with its terms and
	// percentage; nil for a class without them.
	ThiefSkills []ruletablesvc.SkillPercent
	// BardSkills is step 18: each bard skill with its terms and
	// percentage; nil for a class without them.
	BardSkills []ruletablesvc.SkillPercent
	// SkillBudget is step 17's (a thief's) or step 18's (a bard's)
	// warnings: the entered points against the class's budget at the
	// realized level; empty when none.
	SkillBudget []ruletablesvc.BudgetWarning
	// Backstab is step 19: PHB Table 30's damage multiplier at the realized
	// level; 0 for a class without the backstab option.
	Backstab int
}

// SpellbookRules are ADR 0030 step 20.1 on a character: what it may write
// in its spellbook. The sheet's Intelligence picks the PHB Table 4 row.
type SpellbookRules struct {
	// ClassMaxSpellLevel is the class's highest spell level (bard 6th, mage
	// 9th).
	ClassMaxSpellLevel int
	// IntMaxSpellLevel is Table 4's highest spell level (0: none).
	IntMaxSpellLevel int
	// LearnChance is Table 4's chance to learn a spell, in percent.
	LearnChance int
	// MaxSpellsPerLevel is Table 4's maximum number of spells of one level;
	// it means nothing when NoMaximum (the book's "All").
	MaxSpellsPerLevel int
	NoMaximum         bool
}

// PendingLevel is one level whose hit points are owed (ADR 0030 step 7): up
// to the group's name level a hit die to roll (Die, FixedGain 0, step 7.1),
// above it a fixed gain (FixedGain, Die 0, step 7.2).
type PendingLevel struct {
	Level     int
	Die       int
	FixedGain int
}

// LevelUp is ADR 0030 step 7 on a character: its realized level against
// the target level its XP gives.
type LevelUp struct {
	// Pending are the levels still to take, lowest first: RealizedLevel+1
	// to TargetLevel, one prompt each. Empty when none.
	Pending []PendingLevel
	// RealizedAboveTarget is step 7.4: an XP correction brought the target
	// under the realized level. Nothing is removed; the card warns until the
	// roll is undone or the XP comes back.
	RealizedAboveTarget bool
}

// Summary is a character in its owner's list (ADR 0029 step 7.2).
type Summary struct {
	ID    int64
	Name  string
	Class string
	// Level is the derived level (ADR 0030 step 3), set by the service.
	Level int
	// Race is the race code, "" when none is set.
	Race string
	// Alignment and Sex are codes; "" when still to choose (a value that is no
	// code reads as such, ADR 0032 step 8.2). Missing lists those fields.
	Alignment string
	Sex       string
	Missing   []string
	// MaxHP and CurrentHP are the max HP and the ledger's current HP (ADR 0007
	// step 6, ADR 0011 step 1), set by the service.
	MaxHP     int
	CurrentHP int
}

// Details are the facts about the character that the player edits. Alignment
// and Sex are codes (ADR 0032 steps 2 and 3), "" when a migrated character
// lacks them (Missing); hair and eyes are stored as entered, Height (cm),
// Weight (kg) and Age 0 mean not set (ADR 0032 step 5) and Languages are
// catalogue codes.
type Details struct {
	Alignment string
	Age       int
	Sex       string
	Height    float64
	Weight    float64
	Hair      string
	Eyes      string
	Languages []string
}

// Ability is the key of an ability score on the sheet.
type Ability string

// Ability keys, as the API and the stored proficiencies name them.
const (
	AbilityStr Ability = "str"
	AbilityDex Ability = "dex"
	AbilityCon Ability = "con"
	AbilityInt Ability = "int"
	AbilityWis Ability = "wis"
	AbilityCha Ability = "cha"
)

// Abilities are the six ability keys, in the sheet's order.
var Abilities = []Ability{AbilityStr, AbilityDex, AbilityCon, AbilityInt, AbilityWis, AbilityCha}

// Score is the sheet's score in ability a; ok is false for an unknown key.
func (c *Character) Score(a Ability) (score int, ok bool) {
	switch a {
	case AbilityStr:
		return c.Str, true
	case AbilityDex:
		return c.Dex, true
	case AbilityCon:
		return c.Con, true
	case AbilityInt:
		return c.Int, true
	case AbilityWis:
		return c.Wis, true
	case AbilityCha:
		return c.Cha, true
	}
	return 0, false
}
