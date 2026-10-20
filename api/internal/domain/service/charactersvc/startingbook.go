package charactersvc

import (
	"context"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// The starting spellbook of a mage or a bard (ADR 0030 step 20.7): the spells the DM gave, entered
// at creation without a learn roll, each checked by the limits of step 20.1 read at the starting
// level and stored with the origin starting.

// FieldSpellbook is the creation's field of the starting spellbook; a refusal of one spell names
// FieldSpellbook + "." + its id.
const FieldSpellbook = "spellbook"

// The refusals of a starting spellbook: step 20.1's (spellbook.Check), the codes of the Learn
// endpoint (the web words them as it words those), plus CodeUnknown for a spell the catalogue
// does not hold, and CodeNoSpellbook for a class without a spellbook.
const (
	// CodeNoSpellbook is a spellbook entered for a class without one.
	CodeNoSpellbook = "no_spellbook"
	// CodeNotAWizardSpell is a catalogue spell that is not a wizard spell.
	CodeNotAWizardSpell = string(spellbook.RefusedNotWizard)
	// CodeAlreadyInSpellbook is a spell entered twice.
	CodeAlreadyInSpellbook = string(spellbook.RefusedAlreadyIn)
	// CodeSpellLevelAboveClass is a spell level above the class's highest at the starting level.
	CodeSpellLevelAboveClass = string(spellbook.RefusedAboveClass)
	// CodeSpellLevelAboveIntelligence is a spell level above PHB Table 4's for the Intelligence.
	CodeSpellLevelAboveIntelligence = string(spellbook.RefusedAboveIntelligence)
	// CodeSpellbookLevelFull is a spell over Table 4's maximum number of spells of its level.
	CodeSpellbookLevelFull = string(spellbook.RefusedLevelFull)
)

// SpellCatalogue reads the spell catalogue and the starting spellbook's data.
type SpellCatalogue interface {
	// Spell is the catalogue spell id; ok is false when the catalogue holds none.
	Spell(id string) (s spellbook.Spell, ok bool)
	// WizardSpells are the catalogue's wizard spells, by spell level then id.
	WizardSpells() []SpellOption
	// StartingSpells are the catalogue ids a new character of class is offered pre-ticked in
	// his starting spellbook (step 20.7), wizard spells of the catalogue, in the data's order;
	// none for a class without.
	StartingSpells(class string) []string
}

// SpellOption is one catalogue wizard spell the creation offers: its id and spell level.
type SpellOption struct {
	ID    string
	Level int
}

// StartingBook writes a new character's spellbook. It joins the ctx transaction.
type StartingBook interface {
	// AddEntry writes the entry at the end of the character's spellbook.
	AddEntry(ctx context.Context, characterID int64, e spellbook.Entry) error
}

// StartingSpellbook is a draft's starting spellbook (ADR 0030 step 20.7): the limits of step 20.1
// read at the starting level, and the spells entered at each spell level the character may enter.
type StartingSpellbook struct {
	// ClassMaxSpellLevel is the class's highest spell level at the starting level: the highest of
	// its slot allotment there (step 8), 0 for none (a bard at level 1).
	ClassMaxSpellLevel int
	// IntMaxSpellLevel is PHB Table 4's highest spell level for the Intelligence.
	IntMaxSpellLevel int
	// MaxSpellsPerLevel is Table 4's maximum number of spells of one level; it means nothing when
	// NoMaximum (the book's "All").
	MaxSpellsPerLevel int
	NoMaximum         bool
	// Levels are the spell levels the character may enter, 1 up to the lower of the class's and the
	// Intelligence's highest, each with the spells entered of that level and not refused; empty
	// when he may enter none.
	Levels []SpellLevelCount
}

// SpellLevelCount is a spell level and the number of spells entered of it.
type SpellLevelCount struct {
	Level, Count int
}

// classMaxAt is the class's highest spell level at the derived level: the highest spell level of
// its slot allotment (ADR 0030 step 8), within the spellbook's highest (step 20.1).
func classMaxAt(d Derived, classMax int) int {
	top := 0
	for level, slots := range d.SpellSlots {
		if slots > 0 && level > top {
			top = level
		}
	}
	return min(top, classMax)
}

// startingBook is step 20.7 on a derived new character: the starting spellbook's limits at its
// starting level, the entries it keeps in the order entered, and a refusal for each other entry
// (its field FieldSpellbook + "." + the id, in the order entered). A class without a spellbook
// refuses any entry on FieldSpellbook (and answers no limits). Nothing is rolled.
func (s *service) startingBook(c *Character, ids []string) (*StartingSpellbook, []string, []error) {
	rules := c.Derived.Spellbook
	if rules == nil {
		if len(ids) > 0 {
			return nil, nil, []error{&domain.FieldError{Field: FieldSpellbook, Message: CodeNoSpellbook}}
		}
		return nil, nil, nil
	}
	classMax, _ := ruletablesvc.SpellbookMaxLevel(c.Class)
	out := &StartingSpellbook{
		ClassMaxSpellLevel: classMaxAt(c.Derived, classMax), IntMaxSpellLevel: rules.IntMaxSpellLevel,
		MaxSpellsPerLevel: rules.MaxSpellsPerLevel, NoMaximum: rules.NoMaximum, Levels: []SpellLevelCount{},
	}
	counts := map[int]int{}
	var kept []string
	var refusals []error
	for _, id := range ids {
		code, level := s.startingRefusal(out, id, kept, counts)
		if code != "" {
			refusals = append(refusals, &domain.FieldError{Field: FieldSpellbook + "." + id, Message: code})
			continue
		}
		kept = append(kept, id)
		counts[level]++
	}
	for level := 1; level <= min(out.ClassMaxSpellLevel, out.IntMaxSpellLevel); level++ {
		out.Levels = append(out.Levels, SpellLevelCount{Level: level, Count: counts[level]})
	}
	return out, kept, refusals
}

// startingRefusal is the code refusing spell id after the entries kept (counts: kept by spell
// level), "" with its spell level when it is kept: step 20.1's check (spellbook.Check) with the
// class's highest level read at the starting level (b.ClassMaxSpellLevel, from the slots), not
// the class's table maximum the Learn endpoint passes.
func (s *service) startingRefusal(b *StartingSpellbook, id string, kept []string, counts map[int]int) (string, int) {
	sp, found := s.spells.Spell(id)
	limits := spellbook.Limits{
		ClassMaxSpellLevel: b.ClassMaxSpellLevel, IntMaxSpellLevel: b.IntMaxSpellLevel,
		MaxSpellsPerLevel: b.MaxSpellsPerLevel, NoMaximum: b.NoMaximum,
	}
	if r := spellbook.Check(limits, sp, found, slices.Contains(kept, id), counts[sp.Level]); r != spellbook.Fits {
		return string(r), 0
	}
	return "", sp.Level
}

// startingEntries are the spellbook entries a new character is stored with: each spell kept, in the
// order entered, with the origin starting (step 20.5).
func startingEntries(kept []string) []spellbook.Entry {
	out := make([]spellbook.Entry, 0, len(kept))
	for _, id := range kept {
		out = append(out, spellbook.Entry{Key: id, Origin: spellbook.OriginStarting})
	}
	return out
}
