package spellsvc

import "github.com/ceffo/critical/api/internal/domain/spellstats"

// Spell mirrors the seeds/spells.json schema.
type Spell struct {
	// ID is the spell's unique key (see Keys); the database does not store it.
	ID           string
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
	// Stats is the typed form of the six stat fields above, parsed at seed time.
	Stats spellstats.Stats
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
