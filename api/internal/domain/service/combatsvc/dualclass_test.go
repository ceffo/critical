package combatsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// ADR 0042 step 13 on the attack: "an action that uses a frozen entry's
// feature [...] or equipment allowance before the crossover is allowed and
// carries the warning dual_class_old_ability [...]. After the crossover
// nothing is warned". Before it, a figure the character has without choosing
// to use it is the active class's own (no old attack rate).

// dualClass is a character whose old class (left at 9) is dormant until
// crossed, with the old class's feature kind granted at 9 and the active
// class at level.
func dualClass(old, active string, level int, kind string, crossed bool) *charactersvc.Character {
	return &charactersvc.Character{
		ID: charID, Str: 16, Dex: 17, Class: active, Race: "human",
		Derived: charactersvc.Derived{
			RealizedLevel: level, THAC0: 18,
			Classes: []charactersvc.EntryDerived{
				{Class: old, State: classes.EntryFrozen, RealizedLevel: 9, Dormant: !crossed},
				{Class: active, State: classes.EntryActive, RealizedLevel: level},
			},
			Features: []charactersvc.FeatureGrant{{Kind: kind, Class: old, Level: 9, Dormant: !crossed}},
		},
	}
}

// The elf, a fighter left at 9: at mage 7 its long sword shows the mage's
// rate (none; the code before read the dormant fighter's 3/2), at mage 10
// the fighter's 3/2 at 9.
func TestGetAttackForm_Step13_NoOldAttackRateBeforeTheCrossover(t *testing.T) {
	sword := combatsvc.Weapon{Wielded: true, ID: "long_sword", Name: localized.Of("Long Sword"), BaseDamage: "1d8", DamageLarge: "1d12", Kind: combatsvc.WeaponMelee, Type: "long_sword"}
	for _, c := range []struct {
		name  string
		level int
		want  *combatsvc.AttackRateView
	}{
		{"mage 7", 7, nil},
		{"mage 10", 10, &combatsvc.AttackRateView{Attacks: 3, Rounds: 2, ThisRound: 1}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.forms.EXPECT().Get(mock.Anything, charID).Return(combatsvc.AttackForm{}, false, nil).Once()
			f.weapons.EXPECT().List(mock.Anything, charID).Return([]combatsvc.Weapon{sword}, nil)
			f.withChar(dualClass("fighter", "mage", c.level, classes.KindMeleeAttacksPerRound, c.level > 9))
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(str16Mods, nil)
			if c.want != nil {
				f.buffs.EXPECT().GetRound(mock.Anything, charID).Return(1, nil)
			}
			v, err := f.svc.GetAttackForm(context.Background(), charID)
			require.NoError(t, err)
			assert.Equal(t, c.want, v.AttackRate)
		})
	}
}

// A thief left at 9, now a fighter 5, backstabs: allowed at the thief's
// multiplier, warned on the answer and in the journal payload; past the
// crossover, not warned (and no item is read).
func TestResolveAttack_Step13_AnOldClassBackstabWarnsBeforeTheCrossover(t *testing.T) {
	for _, crossed := range []bool{false, true} {
		t.Run(map[bool]string{false: "fighter 5", true: "fighter 10"}[crossed], func(t *testing.T) {
			f := newFixture(t)
			f.critRows = []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 1.5}}
			char := dualClass("thief", "fighter", 5, classes.KindBackstab, crossed)
			char.Derived.Backstab = 4
			f.weapons.EXPECT().Get(mock.Anything, charID, "dagger").Return(dagger1, nil)
			f.withChar(char)
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, buffsvc.WeaponKind("melee")).Return(str16Mods, nil)
			f.dice.EXPECT().RollD20().Return(12).Once()
			f.d100s(t, 90, noCritMult)
			f.dice.EXPECT().RollDie(4).Return(3).Once()
			f.records()

			res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "dagger", TargetAC: intPtr(5), Backstab: true})
			require.NoError(t, err)
			assert.InDelta(t, 4.0, res.BackstabMultiplier, 0, "the thief's at 9")
			if crossed {
				assert.Empty(t, res.Warnings)
				assert.NotContains(t, string(f.entries[0].Payload), "warnings")
				return
			}
			assert.Equal(t, []string{classes.WarnOldAbility}, res.Warnings)
			assert.Contains(t, string(f.entries[0].Payload), `"warnings":["dual_class_old_ability"]`)
		})
	}
}

// The elf at mage 7 attacks with a weapon only its old fighter allows
// (itemsvc warns it dual_class_old_ability): the attack carries the warning.
func TestResolveAttack_Step13_AnOldClassWeaponWarnsBeforeTheCrossover(t *testing.T) {
	f := newFixture(t)
	f.critRows = []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 1.5}}
	f.weapons.EXPECT().Get(mock.Anything, charID, "dagger").Return(dagger1, nil)
	f.withChar(dualClass("fighter", "mage", 7, classes.KindMeleeAttacksPerRound, false))
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, buffsvc.WeaponKind("melee")).Return(str16Mods, nil)
	f.dice.EXPECT().RollD20().Return(12).Once()
	f.d100s(t, 90, noCritMult)
	f.dice.EXPECT().RollDie(4).Return(3).Once()
	items := []itemsvc.Item{{ID: "dagger"}}
	f.items.EXPECT().List(mock.Anything, charID).Return(items, nil).Once()
	f.items.EXPECT().Warnings(mock.Anything, charID, items).Return([]itemsvc.Warning{{ItemID: "dagger", Code: classes.WarnOldAbility}}, nil).Once()
	f.records()

	res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "dagger", TargetAC: intPtr(5)})
	require.NoError(t, err)
	assert.Equal(t, []string{classes.WarnOldAbility}, res.Warnings)
	assert.Contains(t, string(f.entries[0].Payload), `"warnings":["dual_class_old_ability"]`)
}
