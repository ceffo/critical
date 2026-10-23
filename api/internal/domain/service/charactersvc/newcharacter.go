package charactersvc

import (
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// Every class the catalogue defines is creatable (ADR 0040 step 8): a class
// that passed the start-up check is fully supported, and the races are the
// catalogue's too.

// The ranges a new character's entered values must fall in.
const (
	// MinAbility and MaxAbility bound each ability score as it stands on the
	// sheet, after the racial adjustments (ADR 0030 step 1): 3d6, one more
	// or less for a race's adjustment (PHB Table 8) the rolled 18 or 3 takes.
	MinAbility = 3
	MaxAbility = 19
	// MaxNameLen is the longest name, in characters.
	MaxNameLen = 40
	// MinHPMax and MaxHPMax bound the max HP entered.
	MinHPMax = 1
	MaxHPMax = 999
	// MaxPSPMax bounds the PSP maximum entered (ADR 0043 step 4.4), which runs 1 to this.
	MaxPSPMax = 999
	// MinStartingXP and MaxStartingXP bound the starting XP entered (the
	// highest is more than any class's level 20 threshold).
	MinStartingXP = 0
	MaxStartingXP = 100_000_000
	// MinStrExceptional and MaxStrExceptional bound an exceptional strength
	// (ADR 0030 step 9): 18/01 to 18/00, entered as 1 to 100.
	MinStrExceptional = 1
	MaxStrExceptional = 100
	// MaxSkillPoints bounds a thief's points in one skill (ADR 0030 step 17):
	// more could never count, a skill being capped at 95%. The budget of
	// PHB p.39 is a warning (Derived.SkillBudget), not a bound.
	MaxSkillPoints = skills.MaxSkillPoints
)

// Error codes carried by the creation's FieldError messages. The web owns
// the wording.
const (
	CodeEmpty      = "empty"
	CodeTooLong    = "too_long"
	CodeOutOfRange = "out_of_range"
	// CodeNotCreatable is a class no character may be created with yet.
	CodeNotCreatable = "not_creatable"
	// CodeUnknown is a race that is none of Races.
	CodeUnknown = "unknown"
	// CodeRaceNotAllowed is a race the class does not admit (DMG Table 7).
	CodeRaceNotAllowed = "race_not_allowed"
	// CodeNotAllowed is an exceptional strength for a character that has
	// none (ADR 0030 step 9: not a warrior, a Strength other than 18, or a
	// halfling), or skill points or a skill armour for a class without
	// skill points (steps 17 and 18).
	CodeNotAllowed = "not_allowed"
	// The refusals of the class entries (ADR 0042 steps 1 and 2).
	// CodeMultiClassNotAllowed is a multi-class character of a race the ruleset does not lift the
	// book's limit for, CodeDualClassNotAllowed a dual-class one.
	CodeMultiClassNotAllowed = "multiclass_not_allowed"
	CodeDualClassNotAllowed  = "dualclass_not_allowed"
	// CodeMultiClassClassNotAllowed is a class of a multi-class character that the catalogue marks as
	// never part of one (`multiclass: false`, the bard).
	CodeMultiClassClassNotAllowed = "multiclass_class_not_allowed"
	// CodePrimeRequisiteTooLow is a dual-class character with a prime requisite under 15 in a class
	// it left or under 17 in its new class.
	CodePrimeRequisiteTooLow = "prime_requisite_too_low"
	// CodeLevelTooLow is a class left under level 2.
	CodeLevelTooLow = "level_too_low"
	// CodeClassAlreadyLeft is the new class of a dual-class character that it already left.
	CodeClassAlreadyLeft = "class_already_left"
	// CodeDuplicateClass is a class listed twice.
	CodeDuplicateClass = "duplicate_class"
	// CodeTooManyClasses is a multi-class character of more than three classes.
	CodeTooManyClasses = "too_many_classes"
	// CodeMultiAndDualClass is a character both multi-class and dual-class.
	CodeMultiAndDualClass = "multi_and_dual_class"
	// CodeNoActiveClass is a character with no class it is still advancing in.
	CodeNoActiveClass = "no_active_class"
)

// Prime requisite scores a dual-class character needs (ADR 0042 step 2.2; PHB multi/dual, Dual-Class
// Benefits and Restrictions): in the class it left, and in its new class.
const (
	DualClassOldPrime = 15
	DualClassNewPrime = 17
)

// FieldClasses is the creation's field of the class entries as a whole; a refusal of one entry
// names FieldClasses + "." + its index + "." + the entry's field.
const FieldClasses = "classes"

// NewClass is one class of a new character (ADR 0042 steps 1 and 4): the class, its XP and level
// as the sheet states them and, for a class the character left (a dual-class's old one), the level
// it was left at.
type NewClass struct {
	Class string
	// XP is the XP entered for the class; for a class left, 0 is the XP that reaches its left-at level.
	XP int
	// Level is the level taken in the class (ADR 0042 step 4); 0 is the level its XP reaches.
	Level int
	// LeftAtLevel is the level a frozen class was left at; 0 for a class the character advances in.
	LeftAtLevel int
}

// NewCharacter are the values entered for a new character (ADR 0030 step 1):
// never its level, THAC0, saves or slots, which the rules derive.
type NewCharacter struct {
	Name  string
	Class string
	Race  string
	// Classes are the class entries entered (ADR 0042 steps 1 and 4); when set they replace Class
	// and XP, which enter the one-class case.
	Classes                      []NewClass
	Str, Dex, Con, Int, Wis, Cha int
	// StrExceptional is a fighter's 18/xx percentile (step 9), 0 for none.
	StrExceptional int
	HPMax          int
	// PSPMax is the PSP maximum entered from the sheet (ADR 0043 step 4.4), kept as entered; nil
	// enters none and the rules compute it (an entered 0 is refused: the maximum runs 1 to
	// MaxPSPMax). Only a character with a psionics entry has one.
	PSPMax *int
	XP     int
	// SkillPoints are a thief's or a bard's (steps 1, 17 and 18): the points
	// by skill code (0 to MaxSkillPoints each, absent is 0). Empty for the
	// other classes. The armour the skills read is the worn one (ADR 0033
	// step 14): nothing is worn at creation.
	SkillPoints map[string]int
	// Proficiencies are the picks (ADR 0037 step 14), checked as the
	// editor checks them with nothing spent before.
	Proficiencies skills.Picks
	// Details are the details entered with the character, checked as the
	// card's edit checks them (Details.Apply); the alignment and the sex are
	// required, the other nil fields stay unset.
	Details DetailsPatch
	// Languages are the languages entered with the character (nil: none),
	// checked as the Skills tab checks them (skills.ValidLanguages, ADR 0037
	// step 18); its native language is always added.
	Languages []string
	// Spellbook is a mage's or a bard's starting spellbook (ADR 0030 step
	// 20.7): the catalogue ids of the spells the DM gave, in order, written
	// without a roll; nil for none. Another class enters none.
	Spellbook []string
	// PsionicPicks are a psionicist's known powers (ADR 0043 step 9): the disciplines and powers
	// in the player's order, checked as the known-powers editor checks them at the psionics
	// entry's starting level and stored with the character; nil for none. A character with no
	// psionics entry enters none.
	PsionicPicks []psionics.Pick
}

// Change is a rename (Name) or a retirement (Retired true) of a character;
// nil leaves a field as it is, and Retired false retires nothing (a retired
// character is not found, so none is brought back). The skill armour is no
// entry any more: it is the worn body armour (ADR 0033 step 14).
type Change struct {
	Name    *string
	Retired *bool
}
