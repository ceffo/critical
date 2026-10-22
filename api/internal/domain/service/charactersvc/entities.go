package charactersvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

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
	// Classes are the character's class entries (ADR 0042 step 1): the stored
	// truth of its class, level and starting XP. Set them with SetClasses;
	// Class, RealizedLevel and XPTotal below mirror the first active entry, the
	// one the single-class rules read until the multi-class math reads the
	// entries (ADR 0042 steps 7 to 15). Empty for a character built from the
	// mirror alone (Entries gives its one entry).
	Classes []classes.Entry
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
	// Proficiencies are the proficiency picks (ADR 0037 step 12): catalogue
	// id and slots spent, in the player's order.
	Proficiencies skills.Picks

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

// SetClasses sets the class entries and the mirror of the first active entry
// (Class, RealizedLevel, XPTotal) from them.
func (c *Character) SetClasses(entries []classes.Entry) {
	c.Classes = entries
	if first, ok := classes.Primary(entries); ok {
		c.Class, c.RealizedLevel, c.XPTotal = first.Class, first.RealizedLevel, first.StartingXP
	}
}

// SetPrimaryLevel moves the realized level of the first active entry and the
// mirror (a level-up, ADR 0030 step 7.3, and its undo).
func (c *Character) SetPrimaryLevel(level int) {
	c.RealizedLevel = level
	for i := range c.Classes {
		if c.Classes[i].State == classes.EntryActive {
			c.Classes = slices.Clone(c.Classes)
			c.Classes[i].RealizedLevel = level
			return
		}
	}
}

// Entries are the class entries: Classes, or the one active entry the mirror
// states for a character built without them (ADR 0042 step 1: today's columns
// are the one-entry case).
func (c *Character) Entries() []classes.Entry {
	if len(c.Classes) > 0 {
		return c.Classes
	}
	return []classes.Entry{classes.SingleEntry(c.Class, c.RealizedLevel, c.XPTotal)}
}

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
	// Group is the class's group (ADR 0040 step 1.2, its definition's).
	Group string
	// ExceptionalStrength is ADR 0030 steps 9 and 10 read through the class's
	// features (ADR 0040 step 3, exceptional_strength at the realized level):
	// the class enters an 18/xx at Strength 18 and reads the warrior's scale
	// of Strength spells above 18. The race's exclusion is the creation's
	// (ruletablesvc.ExceptionalStrengthAllowed).
	ExceptionalStrength bool
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
	// MagicDefence is ADR 0006 step 9.1: PHB Table 5's magical defence
	// adjustment at the sheet's Wisdom (any class), a named modifier on a
	// save the player flags as against magic that attacks the mind (0: none).
	MagicDefence int
	// SpellImmunities is ADR 0006 step 10.1: the catalogue ids of the spells
	// PHB Table 5 makes the sheet's Wisdom immune to (cumulative; empty below
	// 19). A save against one of them, or one cast on the character, rolls
	// nothing (steps 10.2 and 10.3).
	SpellImmunities []string
	// SpellSlots is step 8 at the realized level: the slots per day by
	// spell level, levels with no slot absent.
	SpellSlots map[int]int
	// LevelUp is step 7: the levels the target is above the realized one.
	LevelUp LevelUp
	// Specializes is step 12: the class may specialize in a weapon (ADR 0037
	// step 9: the weapon pick editor offers the specialization).
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
	// SkillArmor is the skill armour the thief's or the bard's skills read
	// (ADR 0033 step 14: ruletablesvc.SkillArmorFor the worn body armour's
	// Table 29 column; "none" when it wears none), "" for another class.
	SkillArmor string
	// SkillBudget is step 17's (a thief's) or step 18's (a bard's)
	// warnings: the entered points against the class's budget at the
	// realized level; empty when none.
	SkillBudget []ruletablesvc.BudgetWarning
	// SkillPoints is the same budget as figures at the realized level, with
	// the terms of its formula: available, spent, left and the per-skill cap
	// (steps 17 and 18); nil for a class without skill points.
	SkillPoints *ruletablesvc.SkillPointsBudget
	// Backstab is step 19: PHB Table 30's damage multiplier at the realized
	// level; 0 for a class without the backstab option.
	Backstab int
	// Proficiencies is ADR 0037 steps 3 to 5: the non-weapon pool at the
	// realized level and each pick's cost, extra slots and check; nil
	// without a Table 34 row for the class. Set by the service (it reads
	// the catalogue through skills.Read), not by Derive.
	Proficiencies *skills.ProficiencySlots
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
// above it a fixed gain (FixedGain, Die 0, step 7.2). Grants is what the
// level grants beyond hit points (ADR 0037 step 15), set by the service (the
// slots read the catalogue), not by Derive.
type PendingLevel struct {
	Level     int
	Die       int
	FixedGain int
	Grants    skills.LevelGrants
	// FreeSpells are the free spells the level grants a mage (ADR 0030 step
	// 20.8): one per spell level it opens, none above the Intelligence's
	// maximum. Set by the service; nil for none.
	FreeSpells []spellbook.FreeSpell
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
	// ToSpend is what is left to spend at the realized level: each pool's
	// left slots and the skill points left, 0 when none or over budget. The
	// card reminds the player while any is above 0; nothing blocks play. Set
	// by the service with the pools, not by Derive.
	ToSpend skills.LevelGrants
	// FreeSpellsOwed are the free spells the levels taken in the app granted
	// and the spellbook does not hold yet (ADR 0030 step 20.8), part of the
	// same reminder. Set by the service; nil for none.
	FreeSpellsOwed []spellbook.FreeSpell
}

// Summary is a character in its owner's list (ADR 0029 step 7.2).
type Summary struct {
	ID   int64
	Name string
	// Classes are the character's class entries (ADR 0042 step 1), each with
	// its realized level (ADR 0030 step 3): stored, advanced only by the
	// manual level-up step, not the target level derived from XP. Set by the
	// service.
	Classes []classes.Entry
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

// NonproficientPenalty is ADR 0037 step 8 for an attack of c with a weapon of weaponType, from its
// derived picks (skills.ProficiencySlots.NonproficientPenalty): none without them.
func (c *Character) NonproficientPenalty(weaponType string) int {
	return c.Derived.Proficiencies.NonproficientPenalty(weaponType)
}
