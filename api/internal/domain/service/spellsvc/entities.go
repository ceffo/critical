package spellsvc

// Spell mirrors the seeds/spells.json schema.
type Spell struct {
	Name         string
	Class        string
	Level        int
	School       string
	Sphere       string
	CastingTime  string
	Range        string
	Duration     string
	AreaOfEffect string
	Components   string
	SavingThrow  string
	Description  string
}

// SpellSlot is one prepared-spell slot in the daily allotment.
type SpellSlot struct {
	Level int
	// SpellID is the snake_case slug of the English name (empty on slots
	// saved before spell preparation existed).
	SpellID           string
	PreparedSpellName string
	Used              bool
}
