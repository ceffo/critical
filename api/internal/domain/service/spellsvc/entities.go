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
	Level             int
	PreparedSpellName string
	Used              bool
}
