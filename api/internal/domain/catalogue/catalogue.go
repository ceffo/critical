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

// FormatNoSphereTable was format 14 (ADR 0021 step 21): format 13 without the
// per-class sphere_access rule table, which nothing read since a class's
// spheres became its sphere_access feature.
const FormatNoSphereTable = 14

// FormatBracketedConstitution was format 15 (ADR 0021 step 22): a class's hit
// dice carry the boolean bracketed_constitution instead of the Table 3 column
// name.
const FormatBracketedConstitution = 15

// FormatPowerSlot was format 16 (ADR 0021 step 23): a psionic power may carry
// the slot its pick takes where its kind does not say it.
const FormatPowerSlot = 16

// FormatMultiClassFlag was format 17 (ADR 0021 step 24): a class may say it is
// never one of a multi-class character's classes (`multiclass: false`).
const FormatMultiClassFlag = 17

// FormatPsionicCombat was format 18 (ADR 0021 step 25, ADR 0044 step 1): format
// 17 plus the psionic combat tables and the table-named and contact-plus
// initial costs.
const FormatPsionicCombat = 18

// FormatSkillSets was format 19 (ADR 0021 step 27, ADR 0040 steps 12 to 14):
// format 18 plus the top-level skill sets, the shared skill tables renamed
// skill_race, skill_dexterity (rows keyed by skill) and skill_armor, without
// the per-set base and penalty tables, and the parameters of the
// weapon_specialization feature.
const FormatSkillSets = 19

// FormatMeditating is format 20 (ADR 0021 step 28, ADR 0047 item 6): format
// 19 plus the meditating row of psp_recovery, whose requires_proficiency
// names the proficiency that gates it, and that proficiency's description
// (66 in all). It is the only format the API reads (step 28.4): the formats 1
// to 19 are refused like any unknown format, with the advice to run `just
// catalogue`.
const FormatMeditating = 20

// Formats lists the formats this API reads: format 20 only (step 28.4).
var Formats = []int{FormatMeditating}

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

// SkillSet is one skill set of the artifact (format 19, ADR 0040 step 12): the
// skills a class's rogue_skills feature names by ID, with their base
// percentages.
type SkillSet struct {
	// ID is the language-neutral code rogue_skills.set and characters.skill_sets name.
	ID string
	// Name is the heading the client shows for the set.
	Name localized.Text
	// MinPercent is the least percentage a skill of the set is used at.
	MinPercent int
	// Skills are the set's skills in order.
	Skills []SkillSetSkill
}

// SkillSetSkill is one skill of a set: its base percentage (PHB Table 26 or
// 33) and whether a multi-class character in body armour its class may not
// wear still uses it (ADR 0042 step 16.3).
type SkillSetSkill struct {
	ID         string
	Base       int
	ArmourFree bool
}

// CheckSkillSets refuses a set id twice and a skill twice in one set (the
// schema has checked the shapes); the rules that tie the sets to the classes
// and the shared tables are the rule tables service's.
func CheckSkillSets(sets []SkillSet) error {
	seen := make(map[string]bool, len(sets))
	for _, s := range sets {
		if seen[s.ID] {
			return fmt.Errorf("skill sets: set %q twice", s.ID)
		}
		seen[s.ID] = true
		skills := make(map[string]bool, len(s.Skills))
		for _, k := range s.Skills {
			if skills[k.ID] {
				return fmt.Errorf("skill sets: set %q has skill %q twice", s.ID, k.ID)
			}
			skills[k.ID] = true
		}
	}
	return nil
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
