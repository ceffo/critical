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
	// SkillPoints are the entered skill points (ADR 0030 steps 1, 17 and
	// 18) of each skill set the class lists (ADR 0040 step 3.5), by skill
	// code (ruletablesvc.SkillsOf the set); nil or absent skills have 0.
	SkillPoints skills.SetPoints
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

// SetEntryLevel moves the realized level of the active entry of class (""
// the first active one) and, for the first active one, the mirror (a
// level-up, ADR 0030 step 7.3 and ADR 0042 step 7, and its undo); false when
// there is no such entry.
func (c *Character) SetEntryLevel(class string, level int) bool {
	if len(c.Classes) == 0 {
		if class != "" && class != c.Class {
			return false
		}
		c.RealizedLevel = level
		return true
	}
	first, _ := classes.Primary(c.Classes)
	if class == "" {
		class = first.Class
	}
	for i := range c.Classes {
		if c.Classes[i].State == classes.EntryActive && c.Classes[i].Class == class {
			c.Classes = slices.Clone(c.Classes)
			c.Classes[i].RealizedLevel = level
			if class == first.Class {
				c.RealizedLevel = level
			}
			return true
		}
	}
	return false
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

// ActiveClasses are the active entries of the stored row, each class at its
// realized level as stored (no derived block needed): what the attack's
// non-proficiency penalty reads (ADR 0042 step 15).
func (c *Character) ActiveClasses() skills.Classes {
	var out skills.Classes
	for _, e := range c.Entries() {
		if e.State == classes.EntryActive {
			out = append(out, skills.ClassLevel{Class: e.Class, Level: e.RealizedLevel})
		}
	}
	return out
}

// ActiveClassIDs are the classes of the active entries of the stored row, in
// order: what the class equipment warnings judge together (ADR 0042 step 16).
func (c *Character) ActiveClassIDs() []string {
	var out []string
	for _, e := range c.ActiveClasses() {
		out = append(out, e.Class)
	}
	return out
}

// ProficiencyClasses are the entries as the proficiency pools read them (ADR
// 0042 step 15): each class at its realized level, a dual-class's frozen ones
// at their left-at level and Frozen (their slots are kept), from the derived
// block (Derived must be set).
func (c *Character) ProficiencyClasses() skills.Classes {
	out := make(skills.Classes, 0, len(c.Derived.Classes))
	for _, e := range c.Derived.Classes {
		out = append(out, skills.ClassLevel{Class: e.Class, Level: e.RealizedLevel, Frozen: e.State == classes.EntryFrozen})
	}
	return out
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
// 8 and 22.1) through its class entries (ADR 0042 steps 5 to 14). The level
// and XP fields below are the first active entry's (the mirror of
// Character.Class); Classes holds every entry's.
type Derived struct {
	// Classes are the class entries' own figures (ADR 0042 steps 5, 7 and
	// 14), in the entries' order.
	Classes []EntryDerived
	// Features are the feature kinds the entries grant (ADR 0042 step 12),
	// each with the class and the level it is read at, in the kinds' order
	// then the entries'.
	Features []FeatureGrant
	// XPTotal is the first active entry's XP: its starting XP plus its share
	// of every ledger entry (ADR 0015 step 1, ADR 0042 step 5).
	XPTotal int
	// Group is the first active entry's class group (ADR 0040 step 1.2, its
	// definition's).
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
	// spell level, levels with no slot absent. A multi-class character's
	// are the first active spellcasting entry's (ADR 0042 step 14; each
	// entry's are in Classes).
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
	// SkillColumn is the worn body armour's Table 29 column (ADR 0033 step
	// 14: "none" when it wears none), the one every skill set reads through
	// its own armour entries; "" for a class without skill sets.
	SkillColumn string
	// SkillSets is steps 17 and 18 (ADR 0040 step 3.5): one set per
	// rogue_skills feature each entry's class grants at the entry's level
	// (ADR 0042 step 12: a thief/bard has both classes' sets), each with
	// its skills' percentages and terms, the skill armour it reads (ADR 0033
	// step 14), its budget's warnings and its budget as figures; nil for a
	// class without skill sets.
	SkillSets []skills.SkillSet
	// Backstab is step 19: PHB Table 30's damage multiplier at the realized
	// level; 0 for a class without the backstab option.
	Backstab int
	// Proficiencies is ADR 0037 steps 3 to 5: the non-weapon pool at the
	// realized level and each pick's cost, extra slots and check; nil
	// without a Table 34 row for the class. Set by the service (it reads
	// the catalogue through skills.Read), not by Derive.
	Proficiencies *skills.ProficiencySlots
}

// EntryDerived are one class entry's own figures (ADR 0042 steps 5, 7 and
// 14).
type EntryDerived struct {
	Class string
	// Group is the class's group (ADR 0040 step 1.2).
	Group string
	State classes.EntryState
	// XP is the entry's starting XP plus its share of the ledger (ADR 0042
	// step 5).
	XP int
	// TargetLevel is the class's experience table at XP (ADR 0030 step 3);
	// 0 for a frozen entry (ADR 0042 step 7).
	TargetLevel int
	// RealizedLevel is the level taken in the class (a frozen entry's
	// left-at level).
	RealizedLevel int
	// NextLevelXP is the XP that reaches TargetLevel + 1; nil at the last
	// level and for a frozen entry.
	NextLevelXP *int
	// SpellSlots are the class's slots at RealizedLevel (ADR 0042 step 14),
	// empty for a class without a spell progression.
	SpellSlots map[int]int
	// Casts is a class with a spell progression (ADR 0040 step 1.9).
	Casts bool
	// NoArmour is a class that allows no body armour (its equipment's
	// armour list is only none, ADR 0040 step 1.6): its spells warn
	// spell_in_armour while body armour is worn (ADR 0042 step 16.2).
	NoArmour bool
	// SpellFailure is the chance (percent) that a spell of this entry's slots
	// fails (ADR 0030 step 14 per entry, ADR 0042 step 14): its class's
	// spell_failure at the feature's ability score; 0 without the feature.
	SpellFailure int
	// LevelUp are the entry's pending levels (ADR 0042 step 7).
	LevelUp LevelUp
	// Dormant is a dual-class's frozen entry before the crossover (ADR 0042
	// step 13): its features, spells and equipment may be used, each use
	// warning dual_class_old_ability; its THAC0 and saves do not count.
	Dormant bool
}

// CasterLevel is ADR 0042 step 14: the realized level of the first active
// entry with a spell progression, the one whose slots SpellSlots are; the
// realized level when no entry casts (a single-class character's).
func (d *Derived) CasterLevel() int {
	if e, ok := d.CasterEntry(""); ok {
		return e.RealizedLevel
	}
	return d.RealizedLevel
}

// FeatureGrant is one feature kind granted through a class entry (ADR 0042
// step 12): the class granting it and the level its rule reads.
type FeatureGrant struct {
	Kind  string
	Class string
	Level int
	// Dormant is a grant of a dual-class's frozen entry before the
	// crossover (ADR 0042 step 13): usable, with the warning
	// dual_class_old_ability.
	Dormant bool
}

// Grant is the first grant of kind (ADR 0042 step 12), false when no entry
// grants it.
func (d *Derived) Grant(kind string) (FeatureGrant, bool) {
	for _, g := range d.Features {
		if g.Kind == kind {
			return g, true
		}
	}
	return FeatureGrant{}, false
}

// SkillGrants are the rogue_skills grants in entry order (ADR 0042 step 12),
// each granting class with its own skill sets read at its own level; with
// none, the one of class at the realized level (a character derived
// without entries).
func (d *Derived) SkillGrants(class string) []FeatureGrant {
	var out []FeatureGrant
	for _, g := range d.Features {
		if g.Kind == classes.KindRogueSkills {
			out = append(out, g)
		}
	}
	if len(out) == 0 {
		out = []FeatureGrant{{Kind: classes.KindRogueSkills, Class: class, Level: d.RealizedLevel}}
	}
	return out
}

// FeatureLevel is the level the rule of kind reads (ADR 0042 step 12): the
// level of the entry granting it, the realized level when none does.
func (d *Derived) FeatureLevel(kind string) int {
	if g, ok := d.Grant(kind); ok {
		return g.Level
	}
	return d.RealizedLevel
}

// FeatureClass is the class granting kind (ADR 0042 step 12), fallback when
// no entry grants it.
func (d *Derived) FeatureClass(kind, fallback string) string {
	if g, ok := d.Grant(kind); ok {
		return g.Class
	}
	return fallback
}

// CasterEntry is the entry whose slots, preparation and caster level a
// spell uses (ADR 0042 step 14): the spellcasting entry of class, or the
// first one when class is empty (an active one before a frozen one). False when there is none (a character
// with no derived entries: the caller falls back to the character's level).
func (d *Derived) CasterEntry(class string) (EntryDerived, bool) {
	for _, e := range d.casterEntries() {
		if class == "" || e.Class == class {
			return e, true
		}
	}
	return EntryDerived{}, false
}

// Casters is the number of spellcasting entries, a dual-class's frozen ones
// included: above one, each prepared slot names its entry (ADR 0042 step 14).
func (d *Derived) Casters() int {
	return len(d.casterEntries())
}

// casterEntries are the spellcasting entries, the active ones first, then a
// dual-class's frozen ones: a frozen entry keeps its spells at its left-at
// level (ADR 0042 step 14) and casts them, with the warning
// dual_class_old_ability while it is Dormant (step 13).
func (d *Derived) casterEntries() []EntryDerived {
	var out []EntryDerived
	for _, state := range []classes.EntryState{classes.EntryActive, classes.EntryFrozen} {
		for _, e := range d.Classes {
			if e.State == state && e.Casts {
				out = append(out, e)
			}
		}
	}
	return out
}

// Groups are the class groups of the active entries, in entry order without
// repeats (ADR 0042: a by-group rule reads every active class's group); the
// character's Group when no entry is derived.
func (d *Derived) Groups() []string {
	var out []string
	for _, e := range d.Classes {
		if e.State == classes.EntryActive && e.Group != "" && !slices.Contains(out, e.Group) {
			out = append(out, e.Group)
		}
	}
	if len(out) == 0 && d.Group != "" {
		out = []string{d.Group}
	}
	return out
}

// Entry is the derived block of the entry of class, false when the character
// has no such entry.
func (d *Derived) Entry(class string) (EntryDerived, bool) {
	for _, e := range d.Classes {
		if e.Class == class {
			return e, true
		}
	}
	return EntryDerived{}, false
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
	// Class is the entry the level is taken in (ADR 0042 step 7).
	Class string
	// Classes is the character's number of classes, what the hit die, the
	// fixed gain and the Constitution adjustment are divided by (ADR 0042
	// step 10.2 and 10.3); 1 for a single-class character.
	Classes   int
	Level     int
	Die       int
	FixedGain int
	// NoHitPoints is a dual-class's level not above its old class's (ADR
	// 0042 step 11): taken with its other benefits, it gains no hit points
	// (Die and FixedGain 0).
	NoHitPoints bool
	Grants      skills.LevelGrants
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
