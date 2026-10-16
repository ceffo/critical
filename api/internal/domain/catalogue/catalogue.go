// Package catalogue holds the spell catalogue artifact (ADR 0021): reference
// data built offline in the rulebooks repo, loaded by catalogueadpt and
// imported into the database by the storage layer at startup.
package catalogue

import "github.com/ceffo/critical/api/internal/domain/localized"

// Format is the one artifact format this API reads (ADR 0021 step 1).
const Format = 1

// Catalogue is one artifact: a version compared for equality only, and its
// spells.
type Catalogue struct {
	Format  int
	Version string
	Spells  []Spell
}

// Spell is one catalogue spell. ID is the stable key game state refers to
// (the builder's published-id ledger, ADR 0021 step 3). Every human-language
// text is a language map (ADR 0023): the six stat texts, the school and the
// sphere are the English books' text only; the typed stats are parsed at
// import from the fallback-language stat texts. An absent language has no
// entry; an absent text (a spell without a sphere) is empty.
type Spell struct {
	ID           string
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
	Name         localized.Text
	Description  localized.Text
	Source       string
}

// ByID indexes the spells by ID.
func (c Catalogue) ByID() map[string]Spell {
	out := make(map[string]Spell, len(c.Spells))
	for _, s := range c.Spells {
		out[s.ID] = s
	}
	return out
}

// Ref is a spell id that game state or a seed refers to, with where it is
// used (for the import's refusal message).
type Ref struct {
	ID    string
	Where string
}
