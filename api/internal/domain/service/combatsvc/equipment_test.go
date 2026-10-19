package combatsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// ADR 0033 step 8: picking a weapon that is not wielded wields it (one
// equipment_state entry, through Items.Wield) before the form changes; it is
// then the weapon in hand.
func TestUpdateAttackForm_Step8_PickingACarriedWeaponWieldsIt(t *testing.T) {
	f := newFixture(t)
	carried := shortbow
	carried.Wielded = false
	f.forms.EXPECT().Get(mock.Anything, charID).Return(combatsvc.AttackForm{WeaponID: "longsword"}, true, nil).Once()
	f.weapons.EXPECT().List(mock.Anything, charID).Return([]combatsvc.Weapon{longsword, carried}, nil).Once()
	wield := f.items.EXPECT().Wield(mock.Anything, charID, "shortbow").Return(&itemsvc.StateResult{LogEntryID: 4}, nil).Once()
	put := f.forms.EXPECT().Put(mock.Anything, charID, combatsvc.AttackForm{WeaponID: "shortbow", PreviousWeaponID: "longsword"}).
		Return(nil).Once().NotBefore(wield)
	f.weapons.EXPECT().List(mock.Anything, charID).Return([]combatsvc.Weapon{longsword, shortbow}, nil).Once().NotBefore(put)
	f.items.EXPECT().List(mock.Anything, charID).Return(nil, nil).Maybe()
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(bard(), nil).Maybe()
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(strMods(12), nil).Maybe()

	v, err := f.svc.UpdateAttackForm(t.Context(), charID, combatsvc.AttackFormPatch{WeaponID: strPtr("shortbow")})
	require.NoError(t, err)
	assert.Equal(t, "shortbow", v.Form.WeaponID)
	require.NotNil(t, v.Preview)
}

// ADR 0033 step 8: putting away the weapon in hand empties the form's
// weapon: no other wielded one takes its place, and no preview.
func TestGetAttackForm_Step8_PuttingAwayEmptiesTheHand(t *testing.T) {
	f := newFixture(t)
	away := longsword
	away.Wielded = false
	f.withForm(&combatsvc.AttackForm{WeaponID: "longsword", PreviousWeaponID: "shortbow"},
		[]combatsvc.Weapon{away, shortbow}, nil, strMods(12))
	v, err := f.svc.GetAttackForm(t.Context(), charID)
	require.NoError(t, err)
	assert.Empty(t, v.Form.WeaponID)
	assert.Nil(t, v.Preview)

	t.Run("no choice yet: the first wielded weapon", func(t *testing.T) {
		f := newFixture(t)
		f.withForm(nil, []combatsvc.Weapon{away, shortbow}, nil, strMods(12))
		v, err := f.svc.GetAttackForm(t.Context(), charID)
		require.NoError(t, err)
		assert.Equal(t, "shortbow", v.Form.WeaponID)
	})
}

// ADR 0033 step 8: the attack's weapon in hand is a wielded one; an attack
// with a weapon put away is refused before anything is spent or rolled.
func TestResolveAttack_Step8_WeaponMustBeWielded(t *testing.T) {
	f := newFixture(t) // no dice, no ammunition, no log
	away := longsword
	away.Wielded = false
	f.weapons.EXPECT().Get(mock.Anything, charID, "longsword").Return(away, nil)
	_, err := f.svc.ResolveAttack(t.Context(), charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(5)})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "weaponId", fe.Field)
}

// ADR 0033 steps 1 and 3: the weapons are the weapon pieces of the
// equipment, read with their fields (the ammo kind fired is the piece's
// fires); a piece of another category is not a weapon.
func TestItemWeapons_WeaponPiecesOnly(t *testing.T) {
	items := mocks.NewMockItems(t)
	bow := itemsvc.Item{
		ID: "shortbow", Name: localized.Of("Short Bow +2"), Category: itemsvc.CategoryWeapon, State: itemsvc.StateWielded,
		DamageSM: "1d6", DamageL: "1d6", MagicToHit: 2, MagicDamage: 2, WeaponKind: "missile", TwoHanded: true,
		Fires: "arrow", WeaponType: "short_bow", Speed: 7,
	}
	arrows := itemsvc.Item{ID: "arrows", Category: itemsvc.CategoryAmmunition, AmmoKind: "arrow"}
	items.EXPECT().List(mock.Anything, charID).Return([]itemsvc.Item{arrows, bow}, nil)
	w := combatsvc.ItemWeapons(items)
	got, err := w.List(t.Context(), charID)
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.Weapon{{
		ID: "shortbow", Name: localized.Of("Short Bow +2"), BaseDamage: "1d6", DamageLarge: "1d6", MagicToHit: 2,
		MagicDamage: 2, Kind: combatsvc.WeaponMissile, TwoHanded: true, AmmoKind: "arrow", Type: "short_bow",
		Speed: 7, Wielded: true,
	}}, got)
	_, err = w.Get(t.Context(), charID, "arrows")
	assert.ErrorIs(t, err, combatsvc.ErrWeaponNotFound)
}
