package charactersvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	rulesetmocks "github.com/ceffo/critical/api/internal/domain/ruleset/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// liftEnv is classesEnv with the proficiency book and a ruleset whose weapon lift is on.
func liftEnv(t *testing.T, on bool) *env {
	t.Helper()
	e := newEnv(t)
	res := rulesetmocks.NewMockResolver(t)
	rs := ruleset.Default()
	rs.WeaponLift.MulticlassAnyWeapon = on
	res.EXPECT().ForCharacter(mock.Anything, mock.Anything).Return(rs, nil).Maybe()
	svc, err := charactersvc.New(charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: cataloguetest.Tables(t), Owners: e.owners, Tx: e.tx, Sessions: e.sessions,
		Armour: e.armour, HP: e.hp, Spells: e.spells, Book: e.book, FreeSpells: e.free, Rulesets: res,
		Proficiencies: profBook(t),
	})
	require.NoError(t, err)
	e.svc = svc
	return e
}

// ADR 0041 step 2.5 at creation (ADR 0037 step 14): a half-elf cleric/fighter's long sword pick
// is refused weapon_not_allowed with the weapon lift off; with it on the draft refuses nothing
// and reads the pick allowed.
func TestDraft_Step2_5_TheWeaponLiftDecidesAMulticlassClericsPick(t *testing.T) {
	n := entered("half_elf", charactersvc.NewClass{Class: "cleric", XP: 9_000}, charactersvc.NewClass{Class: "fighter", XP: 9_000})
	n.Proficiencies = weapons("long_sword")

	assert.Equal(t, []string{"weapon.long_sword: weapon_not_allowed"}, refused(t, liftEnv(t, false), n))

	on := liftEnv(t, true)
	assert.Empty(t, refused(t, on, n))
	d, err := on.svc.Draft(context.Background(), n, nil)
	require.NoError(t, err)
	require.NotNil(t, d.Character)
	require.Len(t, d.Character.Derived.Proficiencies.WeaponPicks, 1)
	assert.True(t, d.Character.Derived.Proficiencies.WeaponPicks[0].Allowed)
}
