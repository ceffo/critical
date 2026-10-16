package spellsvc

import (
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// Spell is a catalogue spell (ADR 0021). Its human-language texts are
// language maps (ADR 0023): the client picks its language and falls back to
// localized.Fallback per field. The six stat texts, the school and the
// sphere are the English books' text; the rules code reads the fallback
// body.
type Spell struct {
	// ID is the spell's catalogue key, stable across catalogue versions.
	ID           string
	Name         localized.Text
	Class        string
	Level        int
	School       localized.Text
	Sphere       localized.Text
	CastingTime  localized.Text
	Range        localized.Text
	Duration     localized.Text
	AreaOfEffect localized.Text
	Components   localized.Text
	SavingThrow  localized.Text
	// Description is in the catalogue's description subset in every language.
	Description localized.Text
	// Source is the book code of the English text (PHB, TOM, ...; WEB for
	// text no book was matched to).
	Source string
	// Stats is the typed form of the six stat fields above, parsed at import.
	Stats spellstats.Stats
}

// SpellName is a spell's id with its name in every language the catalogue
// has.
type SpellName struct {
	ID   string
	Name localized.Text
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
