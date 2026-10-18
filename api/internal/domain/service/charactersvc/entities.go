package charactersvc

// Character is the pure domain representation of a character row. No transport
// tags — handlers map to a DTO before serializing.
//
// The reader fills the entered values (ADR 0030 step 1) and Sheet; the
// service fills Derived (steps 3 to 8): level, THAC0, saves and slots are never
// entered.
type Character struct {
	ID    int64
	Name  string
	Str   int
	Dex   int
	Con   int
	Int   int
	Wis   int
	Cha   int
	HPMax int
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

	// Sheet is what the character sheet stated when the character was
	// imported; never an input (ADR 0030 step 24).
	Sheet Sheet
	// Derived are the values the rules derive from class, race, abilities and
	// XP; set by the service, zero as the reader returns it.
	Derived Derived

	Details Details
}

// Races are the race codes of the six PHB races (ADR 0030 step 1).
var Races = []string{"human", "dwarf", "elf", "gnome", "half_elf", "halfling"}

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
// kept to compare them with the derived values (ADR 0030 step 24), never read
// as inputs.
type Sheet struct {
	Level int
	THAC0 int
	Saves Saves
}

// Derived are a character's values derived by the rules (ADR 0030 steps 3 to
// 8, 22.1 and 24).
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
	// SheetDifferences are step 24: each value the imported sheet states
	// differently from the rules; the derived value is the one used.
	SheetDifferences []SheetDifference
	// LevelUp is step 7: the levels the target is above the realized one.
	LevelUp LevelUp
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

// SheetValue names a value of the sheet comparison (ADR 0030 step 24).
type SheetValue string

// The values the sheet comparison names.
const (
	SheetLevel            SheetValue = "level"
	SheetTHAC0            SheetValue = "thac0"
	SheetSaveParaPoison   SheetValue = "save_para_poison"
	SheetSaveRodStaffWand SheetValue = "save_rod_staff_wand"
	SheetSavePetriPoly    SheetValue = "save_petri_poly"
	SheetSaveBreath       SheetValue = "save_breath"
	SheetSaveSpell        SheetValue = "save_spell"
)

// SheetDifference is a value the sheet states differently from the rules:
// the sheet's figure and the computed one (structured: the web words it).
type SheetDifference struct {
	Value    SheetValue
	Sheet    int
	Computed int
}

// Summary is a character in its owner's list (ADR 0029 step 7.2).
type Summary struct {
	ID    int64
	Name  string
	Class string
	// Level is the derived level (ADR 0030 step 3), set by the service.
	Level int
}

// Details are the free-text facts about the character that the player edits.
// Text fields are stored as entered, Age 0 means not set and Languages are the
// names as entered.
type Details struct {
	Race      string
	Alignment string
	Age       int
	Sex       string
	Height    string
	Weight    string
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
