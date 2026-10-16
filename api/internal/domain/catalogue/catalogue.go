// Package catalogue holds the spell catalogue artifact (ADR 0021): reference
// data built offline in the rulebooks repo, loaded by catalogueadpt and
// imported into the database by the storage layer at startup.
package catalogue

import (
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The artifact formats this API reads (ADR 0021 steps 1 and 9.1). Format 2
// named a retracted shape and is never read.
const (
	// FormatSpells holds the spells only; the PHB rule tables come from the
	// seeds directory (step 9.6.1).
	FormatSpells = 1
	// FormatRuleTables is FormatSpells plus the PHB rule tables (step 9.1).
	FormatRuleTables = 3
)

// Formats lists the formats this API reads, in order.
var Formats = []int{FormatSpells, FormatRuleTables}

// Catalogue is one artifact: a version compared for equality only, its
// spells and, from format 3, the PHB rule tables.
type Catalogue struct {
	Format  int
	Version string
	Spells  []Spell
	// RuleTables are the artifact's PHB rule tables (format 3), validated;
	// nil in format 1, whose tables are the seeds' (ADR 0021 step 9.6.1).
	// They are read into memory at every start and never imported into
	// the database (step 9.4).
	RuleTables *ruletablesvc.Tables
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
