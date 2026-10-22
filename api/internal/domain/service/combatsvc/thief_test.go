package combatsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// thief is a level 5 thief, Strength 16 (Table 1: +0 to hit, +1 damage):
// THAC0 18 and PHB Table 30's x3, as the character service derives them.
func thief() *charactersvc.Character {
	return &charactersvc.Character{
		ID: charID, Str: 16, Dex: 17, Class: "thief", Race: "halfling",
		Derived: charactersvc.Derived{RealizedLevel: 5, Group: "rogue", THAC0: 18, Backstab: 3},
	}
}

var (
	str16Mods = buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: 16}
	dagger1   = combatsvc.Weapon{
		Wielded: true,
		ID:      "dagger", Name: localized.Of("Dagger +1"), BaseDamage: "1d4", DamageLarge: "1d3",
		Kind: combatsvc.WeaponMelee, Type: "dagger", MagicToHit: 1, MagicDamage: 1,
	}
)

// ADR 0030 step 19: the backstab is +4 to hit (a named modifier, after the
// weapon's magic) and the weapon's dice multiplied by Table 30 as a
// DiceMultiplier factor (ADR 0003 step 4): before the weapon's magic, the
// crit and Strength. One d20, two d100 and one d4, exactly.
func TestResolveAttack_Step19_Backstab(t *testing.T) {
	cases := []struct {
		name                  string
		backstab              bool
		mods                  buffsvc.BuffModifiers
		d20, critCheck        int
		critRows              []combatsvc.CritRow
		wantThac0, wantDamage int
		wantDiceMult          float64
		wantBackstab          float64
	}{
		// (3) + 1 + 1 = 5.
		{"without", false, str16Mods, 12, 90, nil, 17, 5, 1, 0},
		// (3 x 3) + 1 + 1 = 11.
		{"with", true, str16Mods, 12, 90, nil, 13, 11, 1, 3},
		// A buff's x2 on the dice multiplies with it, each its own factor:
		// (3 x 2 x 3) + 1 + 1 = 20.
		{"with a buff's dice multiplier", true, func() buffsvc.BuffModifiers {
			m := str16Mods
			m.Multipliers = 2
			m.MultipliersFrom = []buffsvc.Factor{{Buff: "gem", Value: 2}}
			return m
		}(), 12, 90, nil, 13, 20, 2, 3},
		// The crit multiplies the backstabbed dice and the magic, not
		// Strength: ((3 x 3) + 1) x 2 + 1 = 21.
		{"with a crit", true, str16Mods, 20, 1, []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 2}}, 13, 21, 1, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.critRows = c.critRows
			if f.critRows == nil {
				f.critRows = []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 1.5}}
			}
			f.weapons.EXPECT().Get(mock.Anything, charID, "dagger").Return(dagger1, nil)
			f.withChar(thief())
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, buffsvc.WeaponKind("melee")).Return(c.mods, nil)
			f.dice.EXPECT().RollD20().Return(c.d20).Once()
			f.d100s(t, c.critCheck, noCritMult)
			f.dice.EXPECT().RollDie(4).Return(3).Once()
			f.records()

			res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{
				WeaponID: "dagger", TargetAC: intPtr(5), TargetSize: combatsvc.SizeSmallMedium, Backstab: c.backstab,
			})
			require.NoError(t, err)
			want := []combatsvc.Modifier{
				{Kind: combatsvc.ModStrength, Detail: "16", Value: 0},
				{Kind: combatsvc.ModWeaponMagic, Detail: "Dagger +1", Value: 1},
			}
			if c.backstab {
				want = append(want, combatsvc.Modifier{Kind: combatsvc.ModBackstab, Value: 4})
			}
			assert.Equal(t, want, res.Modifiers)
			assert.Equal(t, c.wantThac0, res.EffectiveThac0)
			assert.True(t, res.Hit)
			assert.Equal(t, c.wantDiceMult, res.DiceMultiplier)
			assert.Equal(t, c.wantBackstab, res.BackstabMultiplier)
			assert.Equal(t, c.wantDamage, res.TotalDamage)
			term := combatsvc.DamageTerm{Term: combatsvc.TermStandardMult, Kind: combatsvc.ModBackstab, Value: 3}
			if c.backstab {
				assert.Contains(t, res.DamageTerms, term, "the multiplier is in the breakdown")
				assert.Contains(t, string(f.entries[0].Payload), `"kind":"backstab"`)
			} else {
				assert.NotContains(t, res.DamageTerms, term)
			}
		})
	}
}

// ADR 0030 step 19: a class without the backstab (a bard's, a fighter's)
// is refused before anything is rolled, spent or logged.
func TestResolveAttack_Step19_BackstabRefusedBeforeAnyDie(t *testing.T) {
	for _, char := range []*charactersvc.Character{bard(), fighter()} {
		t.Run(char.Class, func(t *testing.T) {
			f := newFixture(t)
			f.weapons.EXPECT().Get(mock.Anything, charID, "dagger").Return(dagger1, nil)
			f.withChar(char)
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(str16Mods, nil)
			_, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "dagger", TargetAC: intPtr(5), Backstab: true})
			require.ErrorIs(t, err, domain.ErrInvalidInput)
		})
	}
}

// ADR 0030 step 19 (and ADR 0002 step 12): the attack form previews the
// backstab's effective THAC0 for a thief, none for another class.
func TestGetAttackForm_Step19_BackstabPreview(t *testing.T) {
	for _, tc := range []struct {
		char *charactersvc.Character
		want bool
	}{{thief(), true}, {bard(), false}} {
		t.Run(tc.char.Class, func(t *testing.T) {
			f := newFixture(t)
			f.forms.EXPECT().Get(mock.Anything, charID).Return(combatsvc.AttackForm{}, false, nil).Once()
			f.weapons.EXPECT().List(mock.Anything, charID).Return([]combatsvc.Weapon{dagger1}, nil)
			f.items.EXPECT().List(mock.Anything, charID).Return([]itemsvc.Item{}, nil).Maybe()
			f.withChar(tc.char)
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(str16Mods, nil)
			v, err := f.svc.GetAttackForm(context.Background(), charID)
			require.NoError(t, err)
			if !tc.want {
				assert.Nil(t, v.Preview.Backstab)
				return
			}
			require.NotNil(t, v.Preview.Backstab)
			assert.Equal(t, v.Preview.EffectiveThac0-4, v.Preview.Backstab.EffectiveThac0)
			assert.Equal(t, combatsvc.ModBackstab, v.Preview.Backstab.Modifiers[len(v.Preview.Backstab.Modifiers)-1].Kind)
		})
	}
}

// ADR 0030 step 19 (PHB p.40): a thief backstabs with a melee weapon only.
// With darts the backstab is refused before any die (the dice mock fails a
// roll), and the attack form previews none.
func TestBackstab_Step19_MeleeWeaponOnly(t *testing.T) {
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, charID, "darts").Return(darts, nil)
	f.withChar(thief())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(str16Mods, nil).Maybe()
	_, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "darts", TargetAC: intPtr(5), Backstab: true})
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	g := newFixture(t)
	g.forms.EXPECT().Get(mock.Anything, charID).Return(combatsvc.AttackForm{}, false, nil).Once()
	g.weapons.EXPECT().List(mock.Anything, charID).Return([]combatsvc.Weapon{darts}, nil)
	g.items.EXPECT().List(mock.Anything, charID).Return([]itemsvc.Item{}, nil).Maybe()
	g.withChar(thief())
	g.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(str16Mods, nil)
	v, err := g.svc.GetAttackForm(context.Background(), charID)
	require.NoError(t, err)
	assert.Nil(t, v.Preview.Backstab)

	assert.True(t, combatsvc.BackstabAllowed(thief(), dagger1))
	assert.False(t, combatsvc.BackstabAllowed(thief(), darts))
	assert.False(t, combatsvc.BackstabAllowed(bard(), dagger1))
}
