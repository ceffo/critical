// Package catalogue holds the spell catalogue artifact (ADR 0021): reference
// data built offline in the rulebooks repo, loaded by catalogueadpt and
// imported into the database by the storage layer at startup.
package catalogue

import "github.com/ceffo/critical/api/internal/domain/localized"

// FormatRuleTables is the one artifact format this API reads (ADR 0021 step
// 9.1): the spells and the PHB rule tables. Format 1 (spells only, the tables
// in the seeds) is retired (step 9.6.3) and format 2 named a retracted shape:
// both are refused like any unknown format.
const FormatRuleTables = 3

// Formats lists the formats this API reads, in order.
var Formats = []int{FormatRuleTables}

// Catalogue is one artifact's spells: a version compared for equality only,
// and the spells. A format-3 artifact's PHB rule tables are not part of it:
// they are read beside it at every start and never imported (ADR 0021 step
// 9.4).
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
