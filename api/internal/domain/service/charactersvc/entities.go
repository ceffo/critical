package charactersvc

// Character is the pure domain representation of a character row. No transport
// tags — handlers map to a DTO before serializing.
type Character struct {
	ID        int64
	Name      string
	Str       int
	Dex       int
	Con       int
	Int       int
	Wis       int
	Cha       int
	HPMax     int
	Thac0Base int
	Level     int
	XPTotal   int

	SaveParaPoison   int
	SaveRodStaffWand int
	SavePetriPoly    int
	SaveBreath       int
	SaveSpell        int

	// Class is the class key (e.g. "bard"), read-only.
	Class   string
	Details Details
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
