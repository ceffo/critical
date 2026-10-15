// Package catalogue holds the spell catalogue artifact (ADR 0021): reference
// data built offline in the rulebooks repo, loaded by catalogueadpt and
// imported into the database by the storage layer at startup.
package catalogue

// Format is the artifact format the builder writes and this API validates
// (ADR 0021 step 1). Format 2 made the six stat texts bilingual.
const Format = 2

// Format1 is the previous format, still read: its stat texts are English
// strings, read as stats with no French text.
const Format1 = 1

// Catalogue is one artifact: a version compared for equality only, and its
// spells.
type Catalogue struct {
	Format  int
	Version string
	Spells  []Spell
}

// Spell is one catalogue spell. ID is the stable key game state refers to
// (the builder's published-id ledger, ADR 0021 step 3). The six stat fields
// are raw book text; the typed form is parsed at import from the English
// ones only. A French text that the artifact omits is empty here.
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
	StatsFR       StatsFR
	Name          string
	NameFR        string
	Description   string
	DescriptionFR string
	Source        string
}

// StatsFR are the French texts of the six stats, each empty when the
// artifact has none (the reader falls back to the English one, per field).
type StatsFR struct {
	CastingTime  string
	Range        string
	Duration     string
	AreaOfEffect string
	Components   string
	SavingThrow  string
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
