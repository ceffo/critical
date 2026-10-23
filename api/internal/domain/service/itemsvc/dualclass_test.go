package itemsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

// catalogueClasses reads the fixture catalogue's class definitions.
type catalogueClasses struct{ r *classes.Registry }

func (c catalogueClasses) Class(id string) (classes.Class, error) {
	def, ok := c.r.Class(id)
	if !ok {
		return classes.Class{}, classes.ErrUnknownClass
	}
	return def, nil
}

// dualClass is a character who left old at leftAt and is now newClass at level.
// The old entry is dormant until the active one passes the left-at level (ADR 0042 step 13).
func dualClass(old string, leftAt int, newClass string, level int) charactersvc.View {
	v := viewtest.Multi(viewtest.WithFrozen(old, leftAt), viewtest.WithEntry(newClass, level))
	v.Entries[0].Dormant = level <= leftAt
	return v
}

// ADR 0042 step 13: "an action that uses a frozen entry's [...] equipment allowance before the
// crossover is allowed and carries the warning `dual_class_old_ability`"; after it nothing is
// warned (step 16: a frozen class's allowance counts). The elf, fighter left at 9: as a mage 7
// his chain mail, shield and long sword are the fighter's allowance, warned; as a mage 10 not.
// A piece no class allows keeps its own warning; an old blunt_only class restricts nothing. Each
// action follows the limits of the class using it (step 13): an active blunt_only cleric's sword is
// warned weapon_not_allowed whatever its old fighter allows, past the crossover too.
func TestClassWarnings_Step13_AnOldClassAllowanceWarnsBeforeTheCrossover(t *testing.T) {
	chain, buckler := armour("p", "metal"), shield("s")
	sword := weapon("l", "long_sword", "long_sword", "s")
	for _, tc := range []struct {
		name   string
		c      charactersvc.View
		pieces []itemsvc.Item
		want   []itemsvc.Warning
	}{
		{
			name: "fighter 9 to mage 7: the fighter's allowance warns", c: dualClass("fighter", 9, "mage", 7),
			pieces: []itemsvc.Item{chain, buckler, sword},
			want:   []itemsvc.Warning{w("p", itemsvc.WarnOldAbility), w("s", itemsvc.WarnOldAbility), w("l", itemsvc.WarnOldAbility)},
		},
		{
			name: "fighter 9 to mage 10: past the crossover", c: dualClass("fighter", 9, "mage", 10),
			pieces: []itemsvc.Item{chain, buckler, sword},
		},
		{
			name: "mage 9 to thief 4: metal neither allows", c: dualClass("mage", 9, "thief", 4),
			pieces: []itemsvc.Item{chain}, want: []itemsvc.Warning{w("p", itemsvc.WarnArmourNotAllowed)},
		},
		{
			name: "cleric 5 to fighter 3: the old blunt_only restricts nothing", c: dualClass("cleric", 5, "fighter", 3),
			pieces: []itemsvc.Item{sword},
		},
		{
			name: "fighter 9 to cleric 7: the active blunt_only decides", c: dualClass("fighter", 9, "cleric", 7),
			pieces: []itemsvc.Item{sword}, want: []itemsvc.Warning{w("l", itemsvc.WarnWeaponNotAllowed)},
		},
		{
			name: "fighter 9 to cleric 10: still, past the crossover", c: dualClass("fighter", 9, "cleric", 10),
			pieces: []itemsvc.Item{sword}, want: []itemsvc.Warning{w("l", itemsvc.WarnWeaponNotAllowed)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := deps(t)
			f.Classes = catalogueClasses{cataloguetest.Definitions(t)}
			f.chars.EXPECT().View(mock.Anything, int64(1)).Return(tc.c, nil)
			svc, err := itemsvc.New(mocks.NewMockDAL(t), f.Deps)
			require.NoError(t, err)

			got, err := svc.Warnings(context.Background(), 1, tc.pieces)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
