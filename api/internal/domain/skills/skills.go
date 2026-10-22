// Package skills holds the rules of a character's skills (ADR 0037 step 17, GLOSSARY "Skills"):
// the proficiency pools and their picks (ADR 0037 steps 3 to 9), the languages' slots (step 6),
// the budget refusal (step 10), the warnings (step 11), the level grants (step 15), and a thief's
// or a bard's skill points with their budget (ADR 0030 steps 17 and 18). It is rules only, as
// ruletablesvc holds the tables: it depends on no service but ruletablesvc. skillsvc writes and
// previews the skills over it; charactersvc derives the sheet's figures through it.
package skills

import (
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// Book is the non-weapon catalogue and PHB Table 34 the picks are read against (ADR 0037 steps 1
// to 5); ruletablesvc.Proficiencies satisfies it, the empty one included.
type Book interface {
	Get(id string) (ruletablesvc.Proficiency, bool)
	GroupsOf(class string) []string
	SlotRowOf(class string) (ruletablesvc.SlotRow, bool)
	// ClassDef is the class's definition (ADR 0040 step 5: its features), false for a class the
	// catalogue does not define.
	ClassDef(class string) (classes.Class, bool)
}

// LanguageBook is the language catalogue the languages are checked against (ADR 0032 step 4);
// ruletablesvc.Languages satisfies it, the empty one of a catalogue before format 8 included.
type LanguageBook interface {
	// Has reports whether code is a catalogue language.
	Has(code string) bool
	// Native is the race's own language, "" without a catalogue.
	Native(race string) string
	// All are the catalogue's languages: none for an empty catalogue.
	All() []ruletablesvc.Language
}

// Refusal codes of the skills' FieldErrors. The web owns the wording.
const (
	CodeEmpty      = "empty"
	CodeUnknown    = "unknown"
	CodeDuplicate  = "duplicate"
	CodeOutOfRange = "out_of_range"
	CodeNotAllowed = "not_allowed"
	// CodeWeaponNotAllowed refuses a weapon pick of a type the class may not pick (ADR 0037 step
	// 7).
	CodeWeaponNotAllowed = "weapon_not_allowed"
)

// Warning codes of the slots (ADR 0037 step 11) and of a stored pick the catalogue does not
// hold (step 16, kept as a pick of its stored slots). The web words them.
const (
	CodeSlotsOverBudget    = "slots_over_budget"
	CodeSlotsToSpend       = "slots_to_spend"
	CodeProficiencyUnknown = "proficiency_unknown"
)

// Wire names of the skills' fields and pools.
const (
	// FieldNonweapon is the non-weapon picks: the budget refusal's field; a pick's field is
	// FieldNonweapon + "." + id.
	FieldNonweapon = "nonweapon"
	// PoolNonweapon names the non-weapon pool in the slot warnings.
	PoolNonweapon = "nonweapon"
	// FieldWeapon is the weapon picks: the budget refusal's field; a pick's field is
	// FieldWeapon + "." + type.
	FieldWeapon = "weapon"
	// PoolWeapon names the weapon pool in the slot warnings.
	PoolWeapon = "weapon"
	// FieldLanguages is the languages: a refused code's field and the budget refusal's when the
	// languages overspend (step 6).
	FieldLanguages = "languages"
	// FieldSkillPoints is a thief's or a bard's skill points; one skill's field is
	// FieldSkillPoints + "." + skill.
	FieldSkillPoints = "skillPoints"
	// FieldSet is the skill set a spend of skill points addresses, refused as empty when the
	// class has several (ADR 0040 step 3.5).
	FieldSet = "set"
)

// Warning is a skills rule the stored character breaks without being refused (ADR 0037 step
// 11): a code, the field it is about and, for a slot warning, the pool and its figures
// (slots_over_budget spent and available, slots_to_spend left; nil for the others).
type Warning struct {
	Code                   string
	Field                  string
	Pool                   string
	Spent, Available, Left *int
}
