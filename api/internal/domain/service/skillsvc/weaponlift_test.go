package skillsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
)

// ADR 0041 step 2.5: the proficiency editor checks the weapon picks under the character's
// ruleset. A multi-class cleric/fighter's long sword pick previews allowed with the weapon lift
// on, and is refused weapon_not_allowed with it off (the book's rule, ADR 0042 step 16.1).
func TestPreviewProficiencies_Step2_5_ReadsTheWeaponLift(t *testing.T) {
	clericFighter := &charactersvc.Character{
		ID: charID, Class: "cleric", Int: 12, RealizedLevel: 3, Proficiencies: nonweapon(),
		Derived: charactersvc.Derived{RealizedLevel: 3, Classes: []charactersvc.EntryDerived{
			{Class: "cleric", State: classes.EntryActive, RealizedLevel: 3},
			{Class: "fighter", State: classes.EntryActive, RealizedLevel: 3},
		}},
	}
	edit := skillsvc.Edit{Picks: withWeapons(nonweapon(), "long_sword")}

	t.Run("lift on", func(t *testing.T) {
		f := newSheetFixture(t, clericFighter, nil)
		f.rs.WeaponLift.MulticlassAnyWeapon = true
		got, err := f.svc.PreviewProficiencies(context.Background(), charID, edit)
		require.NoError(t, err)
		require.Len(t, got.Slots.WeaponPicks, 1)
		assert.True(t, got.Slots.WeaponPicks[0].Allowed)
	})
	t.Run("lift off", func(t *testing.T) {
		f := newSheetFixture(t, clericFighter, nil)
		_, err := f.svc.PreviewProficiencies(context.Background(), charID, edit)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, domain.FieldError{Field: "weapon.long_sword", Message: "weapon_not_allowed"}, *fe)
	})
}
