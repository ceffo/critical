package spellbook

import "github.com/ceffo/critical/api/internal/domain/catalogue"

// Spell is what the spellbook's rules read of a catalogue spell: its spell
// level and whether it is a wizard spell (only those go in a spellbook).
type Spell struct {
	Level  int
	Wizard bool
}

// wizardClass is the catalogue's class of a wizard spell.
const wizardClass = "Wizard"

// FromCatalogue is the spellbook's reading of a catalogue spell: the one
// mapping of the catalogue's class to a wizard spell.
func FromCatalogue(s catalogue.Spell) Spell {
	return Spell{Level: s.Level, Wizard: s.Class == wizardClass}
}

// Limits are step 20.1's limits of one spellbook (ADR 0030): the class's
// highest spell level, PHB Table 4's highest spell level for the
// Intelligence and its maximum number of spells of one level (NoMaximum: the
// book's "All"). Which class maximum applies is the caller's: the class's
// highest for a character's book (Learn, Ajouter), the highest of its slots
// at the starting level for a starting book (step 20.7).
type Limits struct {
	ClassMaxSpellLevel int
	IntMaxSpellLevel   int
	MaxSpellsPerLevel  int
	NoMaximum          bool
}

// Refusal is the reason step 20.1 refuses a spell in a spellbook; "" (Fits)
// when it may go in.
type Refusal string

// The refusals of step 20.1, in the order Check tries them.
const (
	// Fits is no refusal: the spell may go in the spellbook.
	Fits Refusal = ""
	// RefusedUnknown is a spell the catalogue does not hold.
	RefusedUnknown Refusal = "unknown"
	// RefusedNotWizard is a catalogue spell that is not a wizard spell.
	RefusedNotWizard Refusal = "not_a_wizard_spell"
	// RefusedAlreadyIn is a spell already in the spellbook.
	RefusedAlreadyIn Refusal = "already_in_spellbook"
	// RefusedAboveClass is a spell level above the class's highest.
	RefusedAboveClass Refusal = "spell_level_above_class"
	// RefusedAboveIntelligence is a spell level above Table 4's for the
	// Intelligence.
	RefusedAboveIntelligence Refusal = "spell_level_above_intelligence"
	// RefusedLevelFull is a spellbook holding Table 4's maximum of the
	// spell's level.
	RefusedLevelFull Refusal = "spellbook_level_full"
)

// Check is step 20.1 (ADR 0030): whether spell sp (found: the catalogue
// holds it) may go in a spellbook with limits l, which already holds it
// (inBook) or not and holds sameLevel spells of its level. The refusals are
// tried in the order of the constants above; Fits when none applies.
func Check(l Limits, sp Spell, found, inBook bool, sameLevel int) Refusal {
	switch {
	case !found:
		return RefusedUnknown
	case !sp.Wizard:
		return RefusedNotWizard
	case inBook:
		return RefusedAlreadyIn
	case sp.Level > l.ClassMaxSpellLevel:
		return RefusedAboveClass
	case sp.Level > l.IntMaxSpellLevel:
		return RefusedAboveIntelligence
	case !l.NoMaximum && sameLevel >= l.MaxSpellsPerLevel:
		return RefusedLevelFull
	}
	return Fits
}
