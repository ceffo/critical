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

// fighter is a level 13 fighter with Strength 18/50 and Dexterity 17 (the
// THAC0 is set, not derived: combat reads Derived.THAC0).
func fighter() *charactersvc.Character {
	return &charactersvc.Character{
		ID: charID, Str: 18, StrExceptional: 50, Dex: 17, Class: "fighter", Race: "human",
		Derived: charactersvc.Derived{RealizedLevel: 13, THAC0: 8, Specializes: true},
	}
}

var (
	str1850     = buffsvc.BaseStrength{StrScore: buffsvc.StrScore{Score: 18, Exceptional: 50}, Warrior: true}
	str1850Mods = buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: 18, EffectiveStrExceptional: 50}

	specSword = combatsvc.Weapon{
		ID: "long_sword", Name: localized.Of("Long Sword"), BaseDamage: "1d8", DamageLarge: "1d12",
		Kind: combatsvc.WeaponMelee, Type: "long_sword", Specialized: true,
	}
	specCrossbow = combatsvc.Weapon{
		ID: "heavy_crossbow", Name: localized.Of("Heavy Crossbow"), BaseDamage: "1d4", DamageLarge: "1d6",
		Kind: combatsvc.WeaponMissile, AmmoKind: "bolt", Type: "heavy_crossbow", Specialized: true,
	}
	bolts = itemsvc.Item{ID: "bolts", Name: localized.Of("Bolts"), AmmoKind: "bolt", Count: 9}
)

// ADR 0030 steps 9 and 12: a specialized long sword in the hands of an 18/50
// fighter reads PHB Table 1 at 18/50 (+1 to hit, +3 damage, detail "18/50")
// and adds specialization's +1 to hit and +2 damage, the damage after the
// multipliers with Strength: 1d8 (5) + 3 + 2 = 10.
func TestResolveAttack_Step12_SpecializedMeleeAnd18_50(t *testing.T) {
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, charID, "long_sword").Return(specSword, nil)
	f.withChar(fighter())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, str1850, buffsvc.WeaponKind("melee")).Return(str1850Mods, nil)
	f.dice.EXPECT().RollD20().Return(15).Once()
	f.d100s(t, 90, noCritMult)
	f.dice.EXPECT().RollDie(8).Return(5).Once()
	f.records()

	res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "long_sword", TargetAC: intPtr(0)})
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.Modifier{
		{Kind: combatsvc.ModStrength, Detail: "18/50", Value: 1},
		{Kind: combatsvc.ModSpecialization, Value: 1},
	}, res.Modifiers)
	assert.Equal(t, 6, res.EffectiveThac0)
	assert.True(t, res.Hit)
	assert.Equal(t, 10, res.TotalDamage)
	assert.Contains(t, res.Rolled.Terms, combatsvc.DamageTerm{Term: combatsvc.TermStrength, Kind: combatsvc.ModSpecialization, Value: 2})
	assert.Contains(t, string(f.entries[0].Payload), `"kind":"specialization"`)
}

// ADR 0030 step 12 (PHB "Effects of Specialization"): a specialized launcher
// gets no damage bonus and no to-hit bonus by default; with the point-blank
// option, +2 to hit. 1d4 (3), no Strength for a crossbow.
func TestResolveAttack_Step12_LauncherPointBlank(t *testing.T) {
	for _, pb := range []bool{false, true} {
		f := newFixture(t)
		f.weapons.EXPECT().Get(mock.Anything, charID, "heavy_crossbow").Return(specCrossbow, nil)
		f.withChar(fighter())
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, str1850, buffsvc.WeaponKind("missile")).Return(str1850Mods, nil)
		f.items.EXPECT().Use(mock.Anything, charID, "bolts").Return(bolts, nil).Once()
		f.dice.EXPECT().RollD20().Return(15).Once()
		f.d100s(t, 90, noCritMult)
		f.dice.EXPECT().RollDie(4).Return(3).Once()
		f.records()

		res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{
			WeaponID: "heavy_crossbow", AmmoItemID: "bolts", TargetAC: intPtr(0), PointBlank: pb,
		})
		require.NoError(t, err)
		want := []combatsvc.Modifier{{Kind: combatsvc.ModDexterity, Detail: "17", Value: 2}}
		if pb {
			want = append(want, combatsvc.Modifier{Kind: combatsvc.ModPointBlank, Value: 2})
		}
		assert.Equal(t, want, res.Modifiers, "point blank %v", pb)
		assert.Equal(t, 3, res.TotalDamage, "no specialization damage for a launcher")
	}
}

// ADR 0030 step 12: the point-blank option is refused for a weapon that is
// not a specialized launcher, before anything is rolled or logged.
func TestResolveAttack_Step12_PointBlankRefused(t *testing.T) {
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, charID, "long_sword").Return(specSword, nil)
	f.withChar(fighter())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, str1850, mock.Anything).Return(str1850Mods, nil)

	_, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "long_sword", TargetAC: intPtr(0), PointBlank: true})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

// ADR 0030 step 11 at the realized level 13: a specialized long sword shows
// 5/2, 2 in the odd round 3 (step 11.4); a specialized heavy crossbow 3/2
// and a point-blank preview with +2 more to hit (step 12). A bard's weapon
// shows no rate.
func TestGetAttackForm_Step11_AttackRateAndPointBlank(t *testing.T) {
	cases := []struct {
		name      string
		char      *charactersvc.Character
		weapon    combatsvc.Weapon
		round     int
		wantRate  *combatsvc.AttackRateView
		wantPoint bool
	}{
		{"specialized long sword", fighter(), specSword, 3, &combatsvc.AttackRateView{Attacks: 5, Rounds: 2, ThisRound: 2}, false},
		{"specialized heavy crossbow", fighter(), specCrossbow, 4, &combatsvc.AttackRateView{Attacks: 3, Rounds: 2, ThisRound: 2}, true},
		{"bard", bard(), combatsvc.Weapon{ID: "long_sword", Name: localized.Of("Long Sword"), BaseDamage: "1d8", DamageLarge: "1d12", Kind: combatsvc.WeaponMelee, Type: "long_sword"}, 0, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.forms.EXPECT().Get(mock.Anything, charID).Return(combatsvc.AttackForm{}, false, nil).Once()
			f.weapons.EXPECT().List(mock.Anything, charID).Return([]combatsvc.Weapon{c.weapon}, nil)
			f.items.EXPECT().List(mock.Anything, charID).Return([]itemsvc.Item{bolts}, nil).Maybe()
			f.withChar(c.char)
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(str1850Mods, nil)
			if c.wantRate != nil {
				f.buffs.EXPECT().GetRound(mock.Anything, charID).Return(c.round, nil)
			}

			v, err := f.svc.GetAttackForm(context.Background(), charID)
			require.NoError(t, err)
			assert.Equal(t, c.wantRate, v.AttackRate)
			if !c.wantPoint {
				assert.Nil(t, v.Preview.PointBlank)
				return
			}
			require.NotNil(t, v.Preview.PointBlank)
			assert.Equal(t, v.Preview.EffectiveThac0-2, v.Preview.PointBlank.EffectiveThac0)
			assert.Equal(t, v.Preview.DamageBonus, v.Preview.PointBlank.DamageBonus)
		})
	}
}
