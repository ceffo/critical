package combatsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

func strPtr(s string) *string { return &s }

// formFixture: the stored form (nil: never saved), the catalog and the buffs.
// The dice mock has no expectation, so a preview that rolls fails the test,
// and itemsvc.Use is never expected either (nothing is spent).
func (f *fixture) withForm(stored *combatsvc.AttackForm, weapons []combatsvc.Weapon, items []itemsvc.Item, mods buffsvc.BuffModifiers) {
	if stored == nil {
		f.forms.EXPECT().Get(mock.Anything, charID).Return(combatsvc.AttackForm{}, false, nil).Once()
	} else {
		f.forms.EXPECT().Get(mock.Anything, charID).Return(*stored, true, nil).Once()
	}
	f.weapons.EXPECT().List(mock.Anything).Return(weapons, nil)
	f.items.EXPECT().List(mock.Anything).Return(items, nil).Maybe()
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(bard(), nil).Maybe()
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12, mock.Anything).Return(mods, nil).Maybe()
}

// ADR 0002 step 12: the preview lists the same to-hit modifiers and the same
// effective THAC0 as the attack the same choices make, for every attack case
// of TestResolveAttack, without rolling a die or spending ammunition.
func TestPreviewAttack_Step12_SameModifiersAsAttack(t *testing.T) {
	for _, c := range attackCases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			stored := combatsvc.AttackForm{
				WeaponID: c.weapon.ID, AmmoItemID: c.req.AmmoItemID, TargetAC: c.req.TargetAC,
				TargetSize: combatsvc.SizeSmallMedium,
			}
			if s := c.req.Situational; s != nil {
				// Stored as typed: the preview trims like the attack.
				stored.Situational = *s
			}
			var items []itemsvc.Item
			if c.ammo != nil {
				items = []itemsvc.Item{*c.ammo}
			}
			f.withForm(&stored, []combatsvc.Weapon{c.weapon}, items, c.mods)

			v, err := f.svc.GetAttackForm(t.Context(), charID)
			require.NoError(t, err)
			require.NotNil(t, v.Preview)
			assert.Equal(t, c.wantMods, v.Preview.Modifiers)
			assert.Equal(t, c.wantEffThac0, v.Preview.EffectiveThac0)
			assert.Equal(t, 13-c.wantEffThac0, v.Preview.ToHit)
			require.NotNil(t, v.Preview.RollNeeded)
			assert.Equal(t, c.wantEffThac0-*c.req.TargetAC, *v.Preview.RollNeeded)
		})
	}
}

// ADR 0002 step 12: the damage bonus sums the flat damage terms: weapon (or
// buff "+N"), magic ammunition, Strength and buff damage.
func TestPreviewAttack_Step12_DamageBonus(t *testing.T) {
	cases := []struct {
		name   string
		weapon combatsvc.Weapon
		items  []itemsvc.Item
		mods   buffsvc.BuffModifiers
		want   int
	}{
		// Sword +2, STR 18 damage +2, buff damage +1.
		{
			"melee: weapon + strength + buff damage", longsword, nil,
			buffsvc.BuffModifiers{SpellsBuff: 1, Multipliers: 1, EffectiveStr: 18, Damage: 1},
			5,
		},
		// Bow +2 and arrow +3; a bow gets no Strength.
		{
			"bow: weapon + arrow, no strength", shortbow,
			[]itemsvc.Item{{ID: "a3", Name: "Arrows +3", AmmoKind: "arrow", MagicBonus: 3, Count: 4}},
			strMods(18), 5,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.withForm(&combatsvc.AttackForm{WeaponID: c.weapon.ID, TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium},
				[]combatsvc.Weapon{c.weapon}, c.items, c.mods)
			v, err := f.svc.GetAttackForm(t.Context(), charID)
			require.NoError(t, err)
			assert.Equal(t, c.want, v.Preview.DamageBonus)
		})
	}
}

func TestGetAttackForm_DefaultsWhenNeverSaved(t *testing.T) {
	f := newFixture(t)
	f.withForm(nil, []combatsvc.Weapon{longsword, shortbow}, nil, strMods(12))
	v, err := f.svc.GetAttackForm(t.Context(), charID)
	require.NoError(t, err)
	assert.Equal(t, combatsvc.AttackForm{
		WeaponID: "longsword", TargetSize: combatsvc.SizeSmallMedium,
	}, v.Form, "the first weapon of the list, unknown AC (ADR 0012 step 1), small/medium")
	assert.Empty(t, v.AmmoChoices)
	assert.Equal(t, 11, v.Preview.EffectiveThac0, "THAC0 13 - sword +2")
	assert.Nil(t, v.Preview.RollNeeded, "no roll needed without an AC")
}

func TestGetAttackForm_NoWeaponNoPreview(t *testing.T) {
	f := newFixture(t)
	f.withForm(nil, nil, nil, strMods(12))
	v, err := f.svc.GetAttackForm(t.Context(), charID)
	require.NoError(t, err)
	assert.Empty(t, v.Form.WeaponID)
	assert.Nil(t, v.Preview)
}

func TestGetAttackForm_Ammunition(t *testing.T) {
	plain := itemsvc.Item{ID: "arrows", Name: "Arrows", AmmoKind: "arrow", Count: 0}
	plus1 := itemsvc.Item{ID: "arrows_plus1", Name: "Arrows +1", AmmoKind: "arrow", MagicBonus: 1, Count: 3}
	plus3 := itemsvc.Item{ID: "arrows_plus3", Name: "Arrows +3", AmmoKind: "arrow", MagicBonus: 3, Count: 2}
	bolts := itemsvc.Item{ID: "bolts", Name: "Bolts", AmmoKind: "bolt", Count: 9}
	flask := itemsvc.Item{ID: "flask", Name: "Flask", Count: 2}
	all := []itemsvc.Item{plain, plus1, plus3, bolts, flask}
	choice := func(i itemsvc.Item) combatsvc.AmmoChoice {
		return combatsvc.AmmoChoice{ItemID: i.ID, Name: i.Name, Magic: i.MagicBonus, Count: i.Count}
	}
	cases := []struct {
		name        string
		storedAmmo  string
		wantAmmo    string
		wantDefault bool
		wantChoices []combatsvc.AmmoChoice
		wantAmmoMod int
	}{
		{
			"none chosen: the first arrow left is offered", "", "arrows_plus1", true,
			[]combatsvc.AmmoChoice{choice(plus1), choice(plus3)},
			1,
		},
		{
			"a chosen arrow stays chosen", "arrows_plus3", "arrows_plus3", false,
			[]combatsvc.AmmoChoice{choice(plus1), choice(plus3)},
			3,
		},
		{
			"a chosen arrow stays chosen once spent, listed at 0", "arrows", "arrows", false,
			[]combatsvc.AmmoChoice{choice(plain), choice(plus1), choice(plus3)},
			0,
		},
		{
			"a chosen item that is gone falls back to the first left", "lost", "arrows_plus1", true,
			[]combatsvc.AmmoChoice{choice(plus1), choice(plus3)},
			1,
		},
		{
			"an item of another kind falls back too", "bolts", "arrows_plus1", true,
			[]combatsvc.AmmoChoice{choice(plus1), choice(plus3)},
			1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.withForm(&combatsvc.AttackForm{WeaponID: "shortbow", AmmoItemID: c.storedAmmo, TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium},
				[]combatsvc.Weapon{shortbow}, all, strMods(12))
			v, err := f.svc.GetAttackForm(t.Context(), charID)
			require.NoError(t, err)
			assert.Equal(t, c.wantAmmo, v.Form.AmmoItemID)
			assert.Equal(t, c.wantDefault, v.AmmoFromDefault)
			assert.Equal(t, c.wantChoices, v.AmmoChoices)
			// DEX 17 +2, bow +2, then the arrow's magic.
			want := 13 - 2 - 2 - c.wantAmmoMod
			assert.Equal(t, want, v.Preview.EffectiveThac0)
		})
	}

	t.Run("a melee weapon shows no ammunition but keeps the bow's choice", func(t *testing.T) {
		f := newFixture(t)
		f.withForm(&combatsvc.AttackForm{WeaponID: "longsword", AmmoItemID: "arrows_plus3", TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium},
			[]combatsvc.Weapon{longsword, shortbow}, all, strMods(12))
		v, err := f.svc.GetAttackForm(t.Context(), charID)
		require.NoError(t, err)
		assert.Empty(t, v.Form.AmmoItemID)
		assert.Empty(t, v.AmmoChoices)
		f.items.AssertNotCalled(t, "List", mock.Anything)
	})
}

func TestUpdateAttackForm_WeaponSwitch(t *testing.T) {
	weapons := []combatsvc.Weapon{longsword, shortbow, darts}
	cases := []struct {
		name   string
		stored *combatsvc.AttackForm
		to     string
		want   combatsvc.AttackForm // as saved
	}{
		{
			"from the default weapon: it becomes the previous one", nil, "shortbow",
			combatsvc.AttackForm{WeaponID: "shortbow", PreviousWeaponID: "longsword", TargetSize: combatsvc.SizeSmallMedium},
		},
		{
			"switching back swaps them",
			&combatsvc.AttackForm{WeaponID: "shortbow", PreviousWeaponID: "longsword", AmmoItemID: "a", TargetAC: new(3), TargetSize: combatsvc.SizeLarge},
			"longsword",
			combatsvc.AttackForm{WeaponID: "longsword", PreviousWeaponID: "shortbow", AmmoItemID: "a", TargetAC: new(3), TargetSize: combatsvc.SizeLarge},
		},
		{
			"the same weapon changes nothing",
			&combatsvc.AttackForm{WeaponID: "shortbow", PreviousWeaponID: "darts", TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium},
			"shortbow",
			combatsvc.AttackForm{WeaponID: "shortbow", PreviousWeaponID: "darts", TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.withForm(c.stored, weapons, nil, strMods(12))
			f.forms.EXPECT().Put(mock.Anything, charID, c.want).Return(nil).Once()
			v, err := f.svc.UpdateAttackForm(t.Context(), charID, combatsvc.AttackFormPatch{WeaponID: strPtr(c.to)})
			require.NoError(t, err)
			assert.Equal(t, c.want.WeaponID, v.Form.WeaponID)
			assert.Equal(t, c.want.PreviousWeaponID, v.Form.PreviousWeaponID)
		})
	}
}

func TestUpdateAttackForm_SavesTargetAndSituational(t *testing.T) {
	f := newFixture(t)
	f.withForm(nil, []combatsvc.Weapon{longsword}, nil, strMods(12))
	want := combatsvc.AttackForm{
		WeaponID: "", TargetAC: new(-2), TargetSize: combatsvc.SizeLarge,
		Situational: combatsvc.Situational{Value: -4, Label: "  cover "},
	}
	f.forms.EXPECT().Put(mock.Anything, charID, want).Return(nil).Once()
	size := combatsvc.SizeLarge
	v, err := f.svc.UpdateAttackForm(t.Context(), charID, combatsvc.AttackFormPatch{
		TargetAC: intPtr(-2), TargetSize: &size, Situational: &combatsvc.Situational{Value: -4, Label: "  cover "},
	})
	require.NoError(t, err)
	assert.Equal(t, "longsword", v.Form.WeaponID)
	assert.Equal(t, "  cover ", v.Form.Situational.Label, "kept as typed")
	assert.Equal(t, "cover", v.Preview.Modifiers[len(v.Preview.Modifiers)-1].Detail, "the preview trims like the attack")
	// THAC0 13 - (STR 0 + sword 2 - 4) = 15; vs AC -2 needs 17.
	assert.Equal(t, 15, v.Preview.EffectiveThac0)
	assert.Equal(t, new(17), v.Preview.RollNeeded)
}

// ADR 0012 step 1: "Inconnue" forgets the target AC; the preview has no roll
// needed then. Giving an AC and forgetting it at once is refused.
func TestUpdateAttackForm_UnknownTargetAC(t *testing.T) {
	f := newFixture(t)
	f.withForm(&combatsvc.AttackForm{WeaponID: "longsword", TargetAC: new(3), TargetSize: combatsvc.SizeSmallMedium},
		[]combatsvc.Weapon{longsword}, nil, strMods(12))
	f.forms.EXPECT().Put(mock.Anything, charID, combatsvc.AttackForm{WeaponID: "longsword", TargetSize: combatsvc.SizeSmallMedium}).Return(nil).Once()
	v, err := f.svc.UpdateAttackForm(t.Context(), charID, combatsvc.AttackFormPatch{UnknownTargetAC: true})
	require.NoError(t, err)
	assert.Nil(t, v.Form.TargetAC)
	assert.Nil(t, v.Preview.RollNeeded)
	assert.Equal(t, 11, v.Preview.EffectiveThac0, "the effective THAC0 does not need the AC")

	_, err = f.svc.UpdateAttackForm(t.Context(), charID, combatsvc.AttackFormPatch{TargetAC: intPtr(4), UnknownTargetAC: true})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "targetAc", fe.Field)
}

func TestUpdateAttackForm_ChoosesAmmunition(t *testing.T) {
	f := newFixture(t)
	arrows := itemsvc.Item{ID: "a3", Name: "Arrows +3", AmmoKind: "arrow", MagicBonus: 3, Count: 2}
	f.withForm(&combatsvc.AttackForm{WeaponID: "shortbow", TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium},
		[]combatsvc.Weapon{shortbow}, []itemsvc.Item{arrows}, strMods(12))
	f.forms.EXPECT().Put(mock.Anything, charID, combatsvc.AttackForm{
		WeaponID: "shortbow", AmmoItemID: "a3", TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium,
	}).Return(nil).Once()
	v, err := f.svc.UpdateAttackForm(t.Context(), charID, combatsvc.AttackFormPatch{AmmoItemID: strPtr("a3")})
	require.NoError(t, err)
	assert.False(t, v.AmmoFromDefault)
	assert.Equal(t, "a3", v.Form.AmmoItemID)
}

func TestUpdateAttackForm_Rejections(t *testing.T) {
	arrows := itemsvc.Item{ID: "a3", Name: "Arrows +3", AmmoKind: "arrow", Count: 2}
	bolts := itemsvc.Item{ID: "bolts", Name: "Bolts", AmmoKind: "bolt", Count: 2}
	bad := combatsvc.TargetSize("huge")
	cases := []struct {
		name      string
		held      string
		patch     combatsvc.AttackFormPatch
		wantField string
	}{
		{"unknown weapon", "longsword", combatsvc.AttackFormPatch{WeaponID: strPtr("nope")}, "weaponId"},
		{"unknown ammunition", "shortbow", combatsvc.AttackFormPatch{AmmoItemID: strPtr("nope")}, "ammoItemId"},
		{"ammunition of another kind", "shortbow", combatsvc.AttackFormPatch{AmmoItemID: strPtr("bolts")}, "ammoItemId"},
		{"ammunition for a melee weapon", "longsword", combatsvc.AttackFormPatch{AmmoItemID: strPtr("a3")}, "ammoItemId"},
		{"target AC above 20", "longsword", combatsvc.AttackFormPatch{TargetAC: intPtr(21)}, "targetAc"},
		{"target AC below -20", "longsword", combatsvc.AttackFormPatch{TargetAC: intPtr(-21)}, "targetAc"},
		{"unknown size", "longsword", combatsvc.AttackFormPatch{TargetSize: &bad}, "targetSize"},
		{"situational beyond 20", "longsword", combatsvc.AttackFormPatch{Situational: &combatsvc.Situational{Value: 21}}, "situational"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.withForm(&combatsvc.AttackForm{WeaponID: c.held, TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium},
				[]combatsvc.Weapon{longsword, shortbow}, []itemsvc.Item{arrows, bolts}, strMods(12))
			_, err := f.svc.UpdateAttackForm(t.Context(), charID, c.patch)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, c.wantField, fe.Field)
			f.forms.AssertNotCalled(t, "Put", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// The reason input sends every keystroke: a label saved mid-word ("Rear ")
// keeps its trailing space, so typing "Rear attack" one key at a time ends
// as "Rear attack", not "Rearattack".
func TestUpdateAttackForm_KeepsTheReasonAsTyped(t *testing.T) {
	label := ""
	for _, key := range "Rear attack" {
		f := newFixture(t)
		f.withForm(&combatsvc.AttackForm{
			WeaponID: "longsword", TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium,
			Situational: combatsvc.Situational{Value: 2, Label: label},
		}, []combatsvc.Weapon{longsword}, nil, strMods(12))
		f.forms.EXPECT().Put(mock.Anything, charID, mock.Anything).Return(nil).Once()
		v, err := f.svc.UpdateAttackForm(t.Context(), charID, combatsvc.AttackFormPatch{
			Situational: &combatsvc.Situational{Value: 2, Label: label + string(key)},
		})
		require.NoError(t, err)
		label = v.Form.Situational.Label
	}
	assert.Equal(t, "Rear attack", label)
}
