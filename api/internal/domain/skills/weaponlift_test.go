package skills_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// ADR 0041 step 2.5 on the weapon picks (ADR 0042 step 16.1, "unless the ruleset lifts it"): with
// the weapon lift on, a multi-class cleric/fighter picks a long sword, as the fighter allows, and
// its stored long sword reads allowed; off, both are refused as the book says. A single-class
// cleric is refused either way (the lift is multi-class only), and so is a dual-class cleric
// whose old class was a fighter (one active entry: the cleric's own limit decides).
func TestWeaponPick_Step2_5_TheWeaponLiftFreesAMulticlassCleric(t *testing.T) {
	book := profBook(t)
	refused := domain.FieldError{Field: "weapon.long_sword", Message: "weapon_not_allowed"}
	clericFighter := skills.Classes{{Class: "cleric", Level: 3}, {Class: "fighter", Level: 3}}
	for _, tc := range []struct {
		name    string
		classes skills.Classes
		lifted  bool
		allowed bool
	}{
		{"multi-class cleric/fighter, lift off", clericFighter, false, false},
		{"multi-class cleric/fighter, lift on", clericFighter, true, true},
		{"single-class cleric, lift off", skills.Classes{{Class: "cleric", Level: 3}}, false, false},
		{"single-class cleric, lift on", skills.Classes{{Class: "cleric", Level: 3}}, true, false},
		{"dual-class fighter 9 to cleric 7, lift on", skills.Classes{{Class: "fighter", Level: 9, Frozen: true}, {Class: "cleric", Level: 7}}, true, false},
		{"dual-class fighter 9 to cleric 10, lift on", skills.Classes{{Class: "fighter", Level: 9, Frozen: true}, {Class: "cleric", Level: 10}}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := skills.Sheet{Classes: tc.classes, Intelligence: 12, Picks: weapons(), MulticlassAnyWeapon: tc.lifted}
			_, err := skills.Apply(book, nil, "human", before, skills.Edit{Picks: skills.Picks{Weapon: weapons("long_sword").Weapon}})
			if tc.allowed {
				require.NoError(t, err, "a new pick")
			} else {
				assert.Equal(t, refused, fieldErr(t, err), "a new pick")
			}

			kept := skills.Sheet{Classes: tc.classes, Intelligence: 12, Picks: weapons("long_sword"), MulticlassAnyWeapon: tc.lifted}
			slots, _ := skills.Read(book, kept)
			require.Len(t, slots.WeaponPicks, 1)
			assert.Equal(t, tc.allowed, slots.WeaponPicks[0].Allowed, "the stored pick")
		})
	}
}

// ADR 0041 step 2.5 lifts the blunt-only limit, nothing else: a multi-class fighter/cleric is
// still refused a specialization with the lift on (PHB, Weapon Specialization: single-class
// fighters only).
func TestWeaponPick_Step2_5_TheLiftGivesNoSpecialization(t *testing.T) {
	book := profBook(t)
	spec := skills.Picks{Weapon: []skills.WeaponPick{{Type: "long_sword", Slots: ruletablesvc.SpecializationSlots("long_sword")}}}
	before := skills.Sheet{
		Classes: skills.Classes{{Class: "fighter", Level: 3}, {Class: "cleric", Level: 3}}, Intelligence: 12,
		Picks: weapons(), MulticlassAnyWeapon: true,
	}
	_, err := skills.Apply(book, nil, "human", before, skills.Edit{Picks: spec})
	assert.Equal(t, domain.FieldError{Field: "weapon.long_sword", Message: skills.CodeNotAllowed}, fieldErr(t, err))
}
