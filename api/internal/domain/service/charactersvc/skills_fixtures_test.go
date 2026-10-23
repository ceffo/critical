package charactersvc_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/slots"
)

// The skills fixtures the service's tests share with the skills package's (domain/skills).

// profBook is a catalogue with PHB Table 34 and Table 38 as the seed holds them and a few Table 37
// rows: Tumbling (rogue, 1 slot, Dex), Tracking (warrior, 2, Wis), Healing (priest, 2, Wis -2),
// Cooking (general, 1, Int), Blind-fighting (rogue and warrior, 2, no check), Languages, Modern.
func profBook(t *testing.T) *ruletablesvc.Proficiencies {
	t.Helper()
	b, err := ruletablesvc.NewProficiencies([]ruletablesvc.Proficiency{
		{ID: "tumbling", Ability: "dex", Slots: 1, Groups: []string{"rogue"}},
		{ID: "tracking", Ability: "wis", Slots: 2, Groups: []string{"warrior"}},
		{ID: "healing", Ability: "wis", Modifier: -2, Slots: 2, Groups: []string{"priest"}},
		{ID: "cooking", Ability: "int", Slots: 1, Groups: []string{"general"}},
		{ID: "blind_fighting", Slots: 2, Groups: []string{"rogue", "warrior"}},
		{ID: "languages_modern", Ability: "int", Slots: 1, Groups: []string{"general"}},
	}, []ruletablesvc.Crossover{
		{Class: "fighter", Groups: []string{"warrior", "general"}},
		{Class: "mage", Groups: []string{"wizard", "general"}},
		{Class: "cleric", Groups: []string{"priest", "general"}},
		{Class: "thief", Groups: []string{"rogue", "general"}},
		{Class: "bard", Groups: []string{"rogue", "warrior", "wizard", "general"}},
		{Class: "psionicist", Groups: []string{"psionicist", "general"}},
	}, []slots.Row{
		{Group: "warrior", WeaponInitial: 4, WeaponLevels: 3, Penalty: -2, NonweaponInitial: 3, NonweaponLevels: 3},
		{Group: "wizard", WeaponInitial: 1, WeaponLevels: 6, Penalty: -5, NonweaponInitial: 4, NonweaponLevels: 3},
		{Group: "priest", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 4, NonweaponLevels: 3},
		{Group: "rogue", WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 3, NonweaponLevels: 4},
		{Group: "psionicist", WeaponInitial: 2, WeaponLevels: 5, Penalty: -4, NonweaponInitial: 3, NonweaponLevels: 3},
	}, cataloguetest.Definitions(t))
	require.NoError(t, err)
	return b
}

func picks(ps ...skills.Pick) skills.Picks {
	return skills.Picks{Nonweapon: append([]skills.Pick{}, ps...), Weapon: []skills.WeaponPick{}}
}

// weapons are weapon picks of types, 1 slot each, and no non-weapon pick.
func weapons(types ...string) skills.Picks {
	out := skills.Picks{Nonweapon: []skills.Pick{}, Weapon: []skills.WeaponPick{}}
	for _, t := range types {
		out.Weapon = append(out.Weapon, skills.WeaponPick{Type: t, Slots: 1})
	}
	return out
}

func p(id string, slots int) skills.Pick { return skills.Pick{ID: id, Slots: slots} }

func fieldErr(t *testing.T, err error) domain.FieldError {
	t.Helper()
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	return *fe
}

// allWeaponTypes are every weapon type of ADR 0030 step 23.
var allWeaponTypes = ruletablesvc.WeaponTypes

// phbLanguages is the language catalogue of the PHB racial lists (chapter 2) plus Common, with
// invented names: the six races' lists, each with its own language marked.
func phbLanguages(t *testing.T) *ruletablesvc.Languages {
	t.Helper()
	var list []ruletablesvc.Language
	for _, c := range []string{"common", "dwarvish", "elvish", "gnomish", "halfling", "goblin", "hobgoblin", "kobold", "orcish", "gnoll", "burrowers"} {
		list = append(list, ruletablesvc.Language{Code: c, Name: localized.Text{"en": c}})
	}
	lists := map[string][]string{
		"human":    {"common"},
		"dwarf":    {"common", "dwarvish", "gnomish", "goblin", "kobold", "orcish"},
		"elf":      {"common", "elvish", "gnomish", "halfling", "goblin", "hobgoblin", "orcish", "gnoll"},
		"gnome":    {"common", "dwarvish", "gnomish", "halfling", "goblin", "kobold", "burrowers"},
		"half_elf": {"common", "elvish", "gnomish", "halfling", "goblin", "hobgoblin", "orcish", "gnoll"},
		"halfling": {"common", "halfling", "dwarvish", "elvish", "gnomish", "goblin", "orcish"},
	}
	native := map[string]string{
		"human": "common", "dwarf": "dwarvish", "elf": "elvish", "gnome": "gnomish", "half_elf": "common", "halfling": "halfling",
	}
	var rows []ruletablesvc.RaceLanguageRow
	for _, race := range cataloguetest.Definitions(t).RaceIDs() {
		for _, l := range lists[race] {
			rows = append(rows, ruletablesvc.RaceLanguageRow{Race: race, Language: l, Native: l == native[race]})
		}
	}
	l, err := ruletablesvc.NewLanguages(list, rows, cataloguetest.Definitions(t))
	require.NoError(t, err)
	return l
}
