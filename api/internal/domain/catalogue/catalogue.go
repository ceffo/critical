// Package catalogue holds the spell catalogue artifact (ADR 0021): reference
// data built offline in the rulebooks repo, loaded by catalogueadpt and
// imported into the database by the storage layer at startup.
package catalogue

import "github.com/ceffo/critical/api/internal/domain/localized"

// FormatRuleTables is format 3 (ADR 0021 step 9.1): the spells and the PHB
// rule tables. Format 1 (spells only, the tables in the seeds) is retired
// (step 9.6.3) and format 2 named a retracted shape: both are refused like
// any unknown format.
const FormatRuleTables = 3

// FormatClassTables is format 4 (ADR 0021 step 10): format 3 plus the class
// and race tables of ADR 0030 step 23. The API reads formats 3 and 4 until
// the published artifact and the dev checkouts are format 4 (step 10.4).
const FormatClassTables = 4

// FormatLevelLimits is format 5 (ADR 0021 step 11): format 4 plus the racial
// level limits of DMG Table 7. The API reads it beside formats 3 and 4 until
// the published artifact and the dev checkouts are format 5 (step 11.4).
const FormatLevelLimits = 5

// FormatSpheres is format 6 (ADR 0021 step 12): format 5 plus each priest
// spell's sphere codes and the classes' sphere access (ADR 0030 steps 15 and
// 23). The API reads it beside format 5 until the published artifact and the
// dev checkouts are format 6 (step 12.4).
const FormatSpheres = 6

// FormatEquipment is format 7 (ADR 0021 step 13, ADR 0033 steps 21 to 23):
// format 6 plus the equipment reference data and its two rule tables. The API
// reads it beside format 6 until the published artifact and the dev checkouts
// are format 7 (step 13.4).
const FormatEquipment = 7

// FormatLanguages is format 8 (ADR 0021 step 14, ADR 0032 step 4): format 7
// plus the language catalogue and each race's language list. The API reads it
// beside format 7 until the published artifact and the dev checkouts are
// format 8 (step 14.4).
const FormatLanguages = 8

// Formats lists the formats this API reads, in order.
var Formats = []int{FormatRuleTables, FormatClassTables, FormatLevelLimits, FormatSpheres, FormatEquipment, FormatLanguages}

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
	ID     string
	Class  string
	Level  int
	School localized.Text
	Sphere localized.Text
	// Spheres are the sphere codes of a priest spell (format 6, ADR 0030
	// step 23): what the rules read, never Sphere. Empty before format 6
	// and for wizard spells.
	Spheres      []string
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
