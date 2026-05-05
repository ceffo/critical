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
}
