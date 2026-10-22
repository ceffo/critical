// Package catalogue holds the spell catalogue artifact (ADR 0021): reference
// data built offline in the rulebooks repo, loaded by catalogueadpt and
// imported into the database by the storage layer at startup.
package catalogue

import (
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/localized"
)

// FormatProficiencyDescriptions was format 9 (ADR 0021 step 16, ADR 0037 step
// 19): the spells, the PHB rule tables, the equipment, the languages and the
// description of each non-weapon proficiency, in English and French.
const FormatProficiencyDescriptions = 9

// FormatMagicDefence was format 10 (ADR 0021 step 17, ADR 0006 step 9): format
// 9 plus Table 5's magical defence adjustment on each wisdom row.
const FormatMagicDefence = 10

// FormatClassDefinitions was format 11 (ADR 0021 step 18, ADR 0040 steps 1,
// 2, 4 and 7): format 10 plus the class, group and race definitions,
// published and schema-checked; the API starts reading them in the next step
// of ADR 0040 step 11.
const FormatClassDefinitions = 11

// FormatSpellImmunities was format 12 (ADR 0021 step 19, ADR 0006 step 10):
// format 11 plus Table 5's spell immunities on each wisdom row.
const FormatSpellImmunities = 12

// FormatPsionics was format 13 (ADR 0021 step 20, ADR 0043): format 12 plus the
// psionicist's rows, the psionic tables, the disciplines and the powers.
const FormatPsionics = 13

// FormatNoSphereTable is format 14 (ADR 0021 step 21): format 13 without the
// per-class sphere_access rule table, which nothing read since a class's
// spheres became its sphere_access feature. It is the only format the API
// reads (step 21.2): the formats 1 to 13 are refused like any unknown format,
// with the advice to run `just catalogue`.
const FormatNoSphereTable = 14

// Formats lists the formats this API reads: format 14 only (step 21.2).
var Formats = []int{FormatNoSphereTable}

// Catalogue is one artifact's spells and proficiency descriptions: a version
// compared for equality only. The artifact's PHB rule tables are not part of it:
// they are read beside it at every start and never imported (ADR 0021 step
// 9.4).
type Catalogue struct {
	Format  int
	Version string
	Spells  []Spell
	// ProficiencyDescriptions are the descriptions (format 9 on) of the non-weapon
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
