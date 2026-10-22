package itemsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

// liftRuleset resolves every character to the default ruleset with the weapon lift set to on.
type liftRuleset struct{ on bool }

func (l liftRuleset) ForCharacter(context.Context, int64) (ruleset.Ruleset, error) {
	rs := ruleset.Default()
	rs.WeaponLift.MulticlassAnyWeapon = l.on
	return rs, nil
}

// multiClass is a character of the active classes cls, level 3 each.
func multiClass(cls ...string) *charactersvc.Character {
	out := &charactersvc.Character{ID: 1, Class: cls[0]}
	for _, c := range cls {
		out.Classes = append(out.Classes, classes.Entry{Class: c, RealizedLevel: 3, State: classes.EntryActive})
	}
	return out
}

// ADR 0041 step 2.5 on the items (ADR 0042 step 16.1, "unless the ruleset lifts it"): the
// ruleset's weapon lift reaches the warnings. A multi-class cleric/fighter wielding a long sword
// is warned weapon_not_allowed with the lift off and not with it on. A single-class cleric is
// warned either way, and so is a dual-class cleric whose old class was a fighter (one active
// entry).
func TestWarnings_Step2_5_TheWeaponLiftReachesTheItems(t *testing.T) {
	sword := weapon("l", "long_sword", "long_sword", "s")
	warned := []itemsvc.Warning{w("l", itemsvc.WarnWeaponNotAllowed)}
	single := &charactersvc.Character{ID: 1, Class: "cleric", Classes: []classes.Entry{{Class: "cleric", RealizedLevel: 3, State: classes.EntryActive}}}
	for _, tc := range []struct {
		name string
		c    *charactersvc.Character
		on   bool
		want []itemsvc.Warning
	}{
		{"multi-class cleric/fighter, lift off", multiClass("cleric", "fighter"), false, warned},
		{"multi-class cleric/fighter, lift on", multiClass("cleric", "fighter"), true, nil},
		{"single-class cleric, lift off", single, false, warned},
		{"single-class cleric, lift on", single, true, warned},
		{"dual-class fighter 9 to cleric 7, lift on", dualClass("fighter", 9, "cleric", 7), true, warned},
		{"dual-class fighter 9 to cleric 10, lift on", dualClass("fighter", 9, "cleric", 10), true, warned},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := deps(t)
			f.Classes = catalogueClasses{cataloguetest.Definitions(t)}
			f.Rulesets = liftRuleset{on: tc.on}
			f.chars.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(tc.c, nil)
			svc, err := itemsvc.New(mocks.NewMockDAL(t), f.Deps)
			require.NoError(t, err)

			got, err := svc.Warnings(context.Background(), 1, []itemsvc.Item{sword})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
