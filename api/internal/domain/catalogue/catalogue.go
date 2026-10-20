// Package catalogue holds the spell catalogue artifact (ADR 0021): reference
// data built offline in the rulebooks repo, loaded by catalogueadpt and
// imported into the database by the storage layer at startup.
package catalogue

import (
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/localized"
)

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

// FormatProficiencyDescriptions is format 9 (ADR 0021 step 16, ADR 0037 step
// 19): format 8 plus the description of each non-weapon proficiency, in English
// and French. It is the only format the API reads (step 16.5): the formats 3
// to 8 are refused like any unknown format, with the advice to run `just
// catalogue`.
const FormatProficiencyDescriptions = 9

// Formats lists the formats this API reads: format 9 only (step 16.5).
var Formats = []int{FormatProficiencyDescriptions}

// Catalogue is one artifact's spells and proficiency descriptions: a version
// compared for equality only. A format-3 artifact's PHB rule tables are not part of it:
// they are read beside it at every start and never imported (ADR 0021 step
// 9.4).
type Catalogue struct {
	Format  int
	Version string
	Spells  []Spell
	// ProficiencyDescriptions are the format-9 descriptions of the non-weapon
	// proficiencies (ADR 0021 step 16): one row per proficiency id.
	ProficiencyDescriptions []ProficiencyDescription
}

// ProficiencyDescription is one non-weapon proficiency's description, a
// language map (ADR 0023) in the Markdown subset of ADR 0021 step 1. The
// artifact carries English and French for every row (step 16.1).
type ProficiencyDescription struct {
	ID          string
	Description localized.Text
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

// CheckProficiencyDescriptions is the import's check of ADR 0021 step 16.3: the descriptions and
// the proficiency ids of seeds/proficiencies.json agree. Each refusal names the id, and the
// language when it is one. It refuses a catalogue id with no description, a description for an
// unknown id, an id twice, and a description lacking English or French (both are required).
func CheckProficiencyDescriptions(descs []ProficiencyDescription, ids []string) error {
	known := make(map[string]bool, len(ids))
	for _, id := range ids {
		known[id] = true
	}
	seen := make(map[string]bool, len(descs))
	var problems []string
	for _, d := range descs {
		switch {
		case seen[d.ID]:
			problems = append(problems, "proficiency "+d.ID+": the description repeats")
			continue
		case !known[d.ID]:
			problems = append(problems, "proficiency "+d.ID+": no such proficiency in seeds/proficiencies.json")
		}
		seen[d.ID] = true
		for _, lang := range RequiredProficiencyLanguages {
			if d.Description[lang] == "" {
				problems = append(problems, "proficiency "+d.ID+": the "+lang+" description is missing or empty")
			}
		}
	}
	for _, id := range ids {
		if !seen[id] {
			problems = append(problems, "proficiency "+id+": no description in the catalogue")
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("proficiency descriptions: the catalogue and seeds/proficiencies.json disagree: %s", strings.Join(problems, "; "))
	}
	return nil
}

// RequiredProficiencyLanguages are the languages every proficiency description has (step 16.1).
var RequiredProficiencyLanguages = []string{"en", "fr"}
