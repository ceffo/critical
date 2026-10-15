// Package catalogue holds the spell catalogue artifact (ADR 0021): reference
// data built offline in the rulebooks repo, loaded by catalogueadpt and
// imported into the database by the storage layer at startup.
package catalogue

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
// (the builder's published-id ledger, ADR 0021 step 3). The six stat fields
// are raw book text; the typed form is parsed at import. A French text that
// the artifact omits is empty here.
type Spell struct {
	ID            string
	Class         string
	Level         int
	School        string
	Sphere        string
	CastingTime   string
	Range         string
	Duration      string
	AreaOfEffect  string
	Components    string
	SavingThrow   string
	Name          string
	NameFR        string
	Description   string
	DescriptionFR string
	Source        string
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
