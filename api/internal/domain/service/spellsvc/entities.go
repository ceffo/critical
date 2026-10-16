package spellsvc

import "github.com/ceffo/critical/api/internal/domain/spellstats"

// Spell is a catalogue spell (ADR 0021): English texts, with the French name
// and description when the catalogue has them (empty otherwise: the reader
// falls back to English).
type Spell struct {
	// ID is the spell's catalogue key, stable across catalogue versions.
	ID           string
	Name         string
	NameFR       string
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
	// DescriptionFR is in the catalogue's description subset, like Description.
	DescriptionFR string
	// Source is the book code of the English text (PHB, TOM, ...; WEB for
	// text no book was matched to).
	Source string
	// Stats is the typed form of the six stat fields above, parsed at import.
	Stats spellstats.Stats
}

// SpellName is a spell's id with its English name and its French one (empty
// when the catalogue has none).
type SpellName struct {
	ID     string
	Name   string
	NameFR string
}

// SpellSlot is one prepared-spell slot in the daily allotment.
type SpellSlot struct {
	Level int
	// SpellID is the snake_case slug of the English name (empty on slots
	// saved before spell preparation existed).
	SpellID           string
	PreparedSpellName string
	Used              bool
	// Extra marks a slot granted by a spell (grant_slots in the spell
	// effects data): blank until cast (the player picks the spell then) and
	// gone at the next long rest.
	Extra bool
}
