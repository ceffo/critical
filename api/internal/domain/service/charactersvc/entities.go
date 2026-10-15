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
