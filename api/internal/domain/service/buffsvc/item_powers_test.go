package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// hammerPower is the Power Gem's thunder hammer as itemsvc serves it: on at
// effective STR 18, melee only, the weapon +5 and its dice x2.
var hammerPower = buffsvc.ItemPower{
	ItemID: "power_gem_dispel", ItemName: "Power Gem", PowerID: "thunder_hammer", MinStr: intPtr(18),
	Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(5), Multiplier: floatPtr(2), WeaponKind: buffsvc.WeaponMelee},
}

// aggregateWith runs Aggregate on buffs and powers at base STR for a roll
// with a weapon of kind.
func aggregateWith(t *testing.T, buffs []buffsvc.BuffEffect, powers []buffsvc.ItemPower, base int, kind buffsvc.WeaponKind) buffsvc.BuffModifiers {
	t.Helper()
	f := newFixture(t)
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(buffs, nil).Once()
	f.powers(powers...)
	got, err := f.svc.Aggregate(context.Background(), charID, sheetStr(base), kind)
	require.NoError(t, err)
	return got
}

// ADR 0016 step 10: a power's min_str trigger is read against the effective
// STR of the buffs: off at 17, on at 18, 19 and 23 (user: "not exactly 18 >=
// 18"); a power without a trigger is always on.
func TestItemPowers_Step10_TriggerAtEffectiveStr(t *testing.T) {
	on := func(str int) buffsvc.BuffModifiers {
		return buffsvc.BuffModifiers{
			SpellsBuff: 1, Multipliers: 2, EffectiveStr: str,
			WeaponBuff: 5, WeaponBuffFrom: "Power Gem", WeaponBuffItemID: "power_gem_dispel",
			MultipliersFrom: []buffsvc.Factor{{Buff: "Power Gem", ItemID: "power_gem_dispel", Value: 2}},
		}
	}
	always := buffsvc.ItemPower{ItemID: "ring", ItemName: "Ring", PowerID: "luck", Effects: buffsvc.BuffEffects{ToHit: intPtr(1)}}
	tests := []struct {
		name   string
		base   int
		buffs  []buffsvc.BuffEffect
		powers []buffsvc.ItemPower
		want   buffsvc.BuffModifiers
	}{
		{
			"17 (12 + Strength 5): off", 12,
			[]buffsvc.BuffEffect{strength(5)},
			[]buffsvc.ItemPower{hammerPower},
			buffsvc.BuffModifiers{SpellsBuff: 1, Multipliers: 1, EffectiveStr: 17},
		},
		{"18 (12 + Strength 2 + 4): on", 12, []buffsvc.BuffEffect{strength(2), strength(4)}, []buffsvc.ItemPower{hammerPower}, on(18)},
		{"19 (12 + Improved 7): on", 12, []buffsvc.BuffEffect{improvedStrength(7)}, []buffsvc.ItemPower{hammerPower}, on(19)},
		{"23 (14 + Improved 10, capped): on", 14, []buffsvc.BuffEffect{improvedStrength(10)}, []buffsvc.ItemPower{hammerPower}, on(23)},
		{
			"no trigger: on at the sheet's 12", 12, nil,
			[]buffsvc.ItemPower{always},
			buffsvc.BuffModifiers{
				SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12, ToHit: 1,
				ToHitFrom: []buffsvc.Contribution{{Buff: "Ring", ItemID: "ring", Value: 1}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, aggregateWith(t, tt.buffs, tt.powers, tt.base, buffsvc.WeaponMelee))
		})
	}
}

// ADR 0016 step 11: a power that is on joins the buffs' totals by the same
// rules: factors multiply, flat bonuses add, the best weapon +N wins across
// buffs and powers; the buffs come first, so a tie goes to the buff. Every
// power contribution carries its item's id.
func TestItemPowers_Step11_PowerJoinsTheAggregate(t *testing.T) {
	enlarge := buffsvc.BuffEffect{Name: "Enlarge", Effects: buffsvc.BuffEffects{SpellsBuff: floatPtr(1.5)}}
	bless := buffsvc.BuffEffect{Name: "Bless", Effects: buffsvc.BuffEffects{ToHit: intPtr(1), Multiplier: floatPtr(1.5)}}
	charm := buffsvc.ItemPower{
		ItemID: "charm", ItemName: "Charm", PowerID: "edge",
		Effects: buffsvc.BuffEffects{ToHit: intPtr(2), Damage: intPtr(1), SaveBonus: intPtr(1), AC: intPtr(-1), WeaponBuff: intPtr(3)},
	}

	t.Run("factors multiply, bonuses add, the best +N wins across both", func(t *testing.T) {
		buffPlus2 := buffsvc.BuffEffect{Name: "Bless", Effects: buffsvc.BuffEffects{ToHit: intPtr(1), Multiplier: floatPtr(1.5), WeaponBuff: intPtr(2)}}
		got := aggregateWith(t, []buffsvc.BuffEffect{enlarge, buffPlus2, strength(6)}, []buffsvc.ItemPower{charm, hammerPower}, 12, buffsvc.WeaponMelee)
		assert.Equal(t, buffsvc.BuffModifiers{
			SpellsBuff: 1.5, Multipliers: 3, SaveBonus: 1, AC: -1, ToHit: 3, Damage: 1, EffectiveStr: 18,
			WeaponBuff: 5, WeaponBuffFrom: "Power Gem", WeaponBuffItemID: "power_gem_dispel",
			SpellsBuffFrom: []buffsvc.Factor{{Buff: "Enlarge", Value: 1.5}},
			MultipliersFrom: []buffsvc.Factor{
				{Buff: "Bless", Value: 1.5}, {Buff: "Power Gem", ItemID: "power_gem_dispel", Value: 2},
			},
			ToHitFrom:     []buffsvc.Contribution{{Buff: "Bless", Value: 1}, {Buff: "Charm", ItemID: "charm", Value: 2}},
			DamageFrom:    []buffsvc.Contribution{{Buff: "Charm", ItemID: "charm", Value: 1}},
			SaveBonusFrom: []buffsvc.Contribution{{Buff: "Charm", ItemID: "charm", Value: 1}},
		}, got)
	})
	t.Run("a weapon +N tie goes to the buff", func(t *testing.T) {
		form := buffsvc.BuffEffect{Name: "Hammer form", Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(5)}}
		got := aggregateWith(t, []buffsvc.BuffEffect{form, strength(6)}, []buffsvc.ItemPower{hammerPower}, 12, buffsvc.WeaponMelee)
		assert.Equal(t, 5, got.WeaponBuff)
		assert.Equal(t, "Hammer form", got.WeaponBuffFrom)
		assert.Empty(t, got.WeaponBuffItemID)
	})
	t.Run("a power's +N above the buff's wins", func(t *testing.T) {
		got := aggregateWith(t, []buffsvc.BuffEffect{bless}, []buffsvc.ItemPower{charm}, 12, buffsvc.WeaponMelee)
		assert.Equal(t, 3, got.WeaponBuff)
		assert.Equal(t, "charm", got.WeaponBuffItemID)
	})
}

// ADR 0016 step 11 (ADR 0003 step 11: an item power that is off lists no
// term): below its trigger a power adds nothing anywhere and is named
// nowhere.
func TestItemPowers_Step11_PowerOffContributesNothing(t *testing.T) {
	bless := buffsvc.BuffEffect{Name: "Bless", Effects: buffsvc.BuffEffects{ToHit: intPtr(1)}}
	off := buffsvc.ItemPower{
		ItemID: "belt", ItemName: "Belt", PowerID: "giant", MinStr: intPtr(19),
		Effects: buffsvc.BuffEffects{ToHit: intPtr(2), Damage: intPtr(3), SaveBonus: intPtr(1), SpellsBuff: floatPtr(2), AC: intPtr(-2)},
	}
	got := aggregateWith(t, []buffsvc.BuffEffect{bless, strength(6)}, []buffsvc.ItemPower{hammerPower, off}, 12, buffsvc.WeaponMissile)
	assert.Equal(t, buffsvc.BuffModifiers{
		SpellsBuff: 1, Multipliers: 1, EffectiveStr: 18, ToHit: 1,
		ToHitFrom: []buffsvc.Contribution{{Buff: "Bless", Value: 1}},
	}, got, "the belt is off at 18; the hammer is on but melee only")
}

// ADR 0016 step 13: the powers listed are the ones that are on, in item and
// power order; below the trigger the list is empty. The buffs' status list
// never holds a power.
func TestItemPowers_Step13_ListedOnlyWhileOn(t *testing.T) {
	always := buffsvc.ItemPower{ItemID: "ring", ItemName: "Ring", PowerID: "luck", Effects: buffsvc.BuffEffects{ToHit: intPtr(1)}}
	tests := []struct {
		name  string
		buffs []buffsvc.BuffEffect
		want  []buffsvc.ItemPower
	}{
		{"STR 17: the hammer is not listed", []buffsvc.BuffEffect{strength(5)}, []buffsvc.ItemPower{always}},
		{"STR 18: listed after the ring", []buffsvc.BuffEffect{strength(6)}, []buffsvc.ItemPower{always, hammerPower}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.base(12)
			f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(tt.buffs, nil)
			f.powers(always, hammerPower)

			got, err := f.svc.ListPowers(context.Background(), charID)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			status, err := f.svc.ListStatus(context.Background(), charID)
			require.NoError(t, err)
			assert.Len(t, status, len(tt.buffs), "powers are never on the buff list")
		})
	}
}

// ADR 0004 step 18: effects with a weapon-kind condition count only for a
// roll made with a weapon of that kind (melee-only: a melee attack, not a
// missile one, a save or a spell to-hit, which ask with no kind); without
// the condition they count everywhere. It does not change whether the buff
// applies on the list.
func TestBuffs_Step18_WeaponKindFiltersTheEffects(t *testing.T) {
	meleeOnly := buffsvc.BuffEffect{ID: "m", Name: "Blade song", Effects: buffsvc.BuffEffects{
		ToHit: intPtr(2), Damage: intPtr(2), SaveBonus: intPtr(1), WeaponBuff: intPtr(4), Multiplier: floatPtr(2),
		WeaponKind: buffsvc.WeaponMelee,
	}}
	missileOnly := buffsvc.BuffEffect{ID: "r", Name: "True aim", Effects: buffsvc.BuffEffects{ToHit: intPtr(3), WeaponKind: buffsvc.WeaponMissile}}
	everywhere := buffsvc.BuffEffect{ID: "b", Name: "Bless", Effects: buffsvc.BuffEffects{ToHit: intPtr(1)}}
	buffs := []buffsvc.BuffEffect{meleeOnly, missileOnly, everywhere}
	melee := buffsvc.BuffModifiers{
		SpellsBuff: 1, Multipliers: 2, EffectiveStr: 12, ToHit: 3, Damage: 2, SaveBonus: 1,
		WeaponBuff: 4, WeaponBuffFrom: "Blade song",
		MultipliersFrom: []buffsvc.Factor{{Buff: "Blade song", Value: 2}},
		ToHitFrom:       []buffsvc.Contribution{{Buff: "Blade song", Value: 2}, {Buff: "Bless", Value: 1}},
		DamageFrom:      []buffsvc.Contribution{{Buff: "Blade song", Value: 2}},
		SaveBonusFrom:   []buffsvc.Contribution{{Buff: "Blade song", Value: 1}},
	}
	missile := buffsvc.BuffModifiers{
		SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12, ToHit: 4,
		ToHitFrom: []buffsvc.Contribution{{Buff: "True aim", Value: 3}, {Buff: "Bless", Value: 1}},
	}
	none := buffsvc.BuffModifiers{
		SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12, ToHit: 1,
		ToHitFrom: []buffsvc.Contribution{{Buff: "Bless", Value: 1}},
	}
	for kind, want := range map[buffsvc.WeaponKind]buffsvc.BuffModifiers{
		buffsvc.WeaponMelee: melee, buffsvc.WeaponMissile: missile, buffsvc.WeaponNone: none,
	} {
		t.Run("roll with "+string(kind)+" weapon", func(t *testing.T) {
			assert.Equal(t, want, aggregateWith(t, buffs, nil, 12, kind))
		})
	}

	t.Run("still applies on the list", func(t *testing.T) {
		f := newFixture(t)
		f.base(12)
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(buffs, nil)
		status, err := f.svc.ListStatus(context.Background(), charID)
		require.NoError(t, err)
		for _, s := range status {
			assert.True(t, s.Applies, s.Name)
		}
	})
}

// ADR 0004 step 18: a weapon kind other than melee or missile is refused,
// on an activated buff and on a saved template alike, before anything is
// read or written.
func TestBuffs_Step18_RejectsAnUnknownWeaponKind(t *testing.T) {
	bad := buffsvc.BuffEffects{ToHit: intPtr(1), WeaponKind: "thrown"}
	f := newFixture(t)
	_, err := f.svc.Activate(context.Background(), charID, buffsvc.BuffEffect{Name: "X", Effects: bad})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Contains(t, err.Error(), "weapon_kind")
	_, err = f.svc.SaveBuff(context.Background(), charID, buffsvc.SavedBuff{Name: "X", Effects: bad})
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	for _, ok := range []buffsvc.WeaponKind{buffsvc.WeaponMelee, buffsvc.WeaponMissile, buffsvc.WeaponNone} {
		require.NoError(t, buffsvc.ValidateEffects(buffsvc.BuffEffects{WeaponKind: ok}), ok)
	}
}

// sheetStr is a non-warrior's sheet Strength as Aggregate receives it.
func sheetStr(score int) buffsvc.BaseStrength {
	return buffsvc.BaseStrength{StrScore: buffsvc.StrScore{Score: score}}
}

// ADR 0033 step 11: a base_ac is an AC number, -10 to 10, on a buff and on
// a saved template alike.
func TestBuffs_ADR0033Step11_BaseACIsAnACNumber(t *testing.T) {
	for _, bad := range []int{-11, 11} {
		err := buffsvc.ValidateEffects(buffsvc.BuffEffects{BaseAC: intPtr(bad)})
		require.ErrorIs(t, err, domain.ErrInvalidInput, bad)
		assert.Contains(t, err.Error(), "base_ac")
	}
	for _, ok := range []int{-10, 6, 10} {
		require.NoError(t, buffsvc.ValidateEffects(buffsvc.BuffEffects{BaseAC: intPtr(ok), WithoutMagicArmour: true}), ok)
	}
	f := newFixture(t)
	_, err := f.svc.SaveBuff(context.Background(), charID, buffsvc.SavedBuff{Name: "X", Effects: buffsvc.BuffEffects{BaseAC: intPtr(12)}})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}
