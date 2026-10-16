package combatsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	buffmocks "github.com/ceffo/critical/api/internal/domain/service/buffsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// gemHammer is the Power Gem's thunder hammer as itemsvc serves it (ADR 0016
// step 16): on at effective STR 18, melee only, the weapon +5 and its dice
// x2.
var gemHammer = buffsvc.ItemPower{
	ItemID: "power_gem_dispel", ItemName: "Power Gem", PowerID: "thunder_hammer", MinStr: intPtr(18),
	Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(5), Multiplier: floatPtr(2), WeaponKind: buffsvc.WeaponMelee},
}

// strengthTo18 is a Strength buff that takes the sheet's 12 to 18.
var strengthTo18 = buffsvc.BuffEffect{ID: "s", Name: "Strength", Duration: 90, Effects: buffsvc.BuffEffects{
	StrBonus: intPtr(6), StrMax: intPtr(18),
}}

// withRealBuffs is the fixture with the real buffsvc aggregate over buffs
// and the carried items' powers, so the weapon kind combatsvc asks for is
// the one the powers are filtered by.
func withRealBuffs(t *testing.T, buffs []buffsvc.BuffEffect, powers []buffsvc.ItemPower) *fixture {
	t.Helper()
	dal := buffmocks.NewMockDAL(t)
	dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(buffs, nil)
	items := buffmocks.NewMockItems(t)
	items.EXPECT().ItemPowers(mock.Anything).Return(powers, nil)
	real, err := buffsvc.New(dal, noTx{}, buffmocks.NewMockLog(t), buffmocks.NewMockCharacters(t), buffmocks.NewMockDice(t), items)
	require.NoError(t, err)

	f := newFixture(t)
	svc, err := combatsvc.New(combatsvc.Deps{
		Chars: f.chars, Buffs: real, Dice: f.dice, Weapons: f.weapons, Crit: f.crit,
		Items: f.items, Forms: f.forms, Rules: phb(t), Log: f.log, Tx: noTx{},
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

// gemMod is the hammer's "weapon becomes +5", named by its item.
var gemMod = combatsvc.Modifier{Kind: combatsvc.ModWeaponBuff, Detail: "Power Gem", ItemID: "power_gem_dispel", Value: 5}

// ADR 0016 step 12 (ADR 0002 step 6, ADR 0003 step 4): at effective STR 18
// the hammer makes a melee weapon +5 (not +2 + 5) and doubles its raw dice;
// with a bow or hurled darts the power is still on but adds nothing: the
// weapon's own magic, dice x1.
func TestItemPowers_Step12_HammerOnMeleeOnly(t *testing.T) {
	arrows := itemsvc.Item{ID: "arrows_plus3", Name: "Arrows +3", AmmoKind: "arrow", MagicBonus: 3, Count: 5}
	tests := []struct {
		name      string
		weapon    combatsvc.Weapon
		ammo      string
		d20, die  int
		sides     int
		wantMods  []combatsvc.Modifier
		wantDiceX float64
		// wantDamage is ((die x dice multiplier) + weapon) + STR damage.
		wantDamage int
	}{
		{
			// STR 18: +1 to hit, +2 damage. 1d8 = 6: 6 x 2 + 5 + 2 = 19.
			name: "long sword +2: +5 and x2", weapon: longsword, d20: 11, die: 6, sides: 8,
			wantMods: []combatsvc.Modifier{modStr(18, 1), gemMod}, wantDiceX: 2, wantDamage: 19,
		},
		{
			// A bow gets no STR; DEX 17 +2, bow +2, arrow +3. 1d6 = 4: 4 + 2 + 3 = 9.
			name: "short bow +2: its own +2, x1", weapon: shortbow, ammo: "arrows_plus3", d20: 12, die: 4, sides: 6,
			wantMods: []combatsvc.Modifier{modDex(17, 2), modWeapon(shortbow), modAmmo(arrows)}, wantDiceX: 1, wantDamage: 9,
		},
		{
			// Hurled: STR 18 +1/+2 and DEX 17 +2, no magic. 1d3 = 3: 3 + 2 = 5.
			name: "darts: no +5, x1", weapon: darts, d20: 13, die: 3, sides: 3,
			wantMods: []combatsvc.Modifier{modStr(18, 1), modDex(17, 2)}, wantDiceX: 1, wantDamage: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := withRealBuffs(t, []buffsvc.BuffEffect{strengthTo18}, []buffsvc.ItemPower{gemHammer})
			f.weapons.EXPECT().Get(mock.Anything, tt.weapon.ID).Return(tt.weapon, nil)
			f.withChar(bard())
			if tt.ammo != "" {
				left := arrows
				left.Count--
				f.items.EXPECT().Use(mock.Anything, tt.ammo).Return(left, nil).Once()
			}
			f.dice.EXPECT().RollD20().Return(tt.d20).Once()
			f.d100s(t, 90, noCritMult)
			f.dice.EXPECT().RollDie(tt.sides).Return(tt.die).Once()
			f.records()

			res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{
				WeaponID: tt.weapon.ID, AmmoItemID: tt.ammo, TargetAC: new(10),
			})
			require.NoError(t, err)
			require.True(t, res.Hit)
			assert.Equal(t, tt.wantMods, res.Modifiers)
			assert.InDelta(t, tt.wantDiceX, res.DiceMultiplier, 1e-9)
			assert.Equal(t, tt.wantDamage, res.TotalDamage)
		})
	}
}

// ADR 0016 step 12 (ADR 0002 step 12): the preview follows the weapon in
// hand: the sword's lists the hammer's +5, the bow's its own +2.
func TestItemPowers_Step12_PreviewFollowsTheWeaponInHand(t *testing.T) {
	arrows := itemsvc.Item{ID: "arrows_plus1", Name: "Arrows +1", AmmoKind: "arrow", MagicBonus: 1, Count: 9}
	tests := []struct {
		name     string
		form     combatsvc.AttackForm
		wantMods []combatsvc.Modifier
		wantDmg  int
	}{
		{
			"long sword",
			combatsvc.AttackForm{WeaponID: "longsword", TargetSize: combatsvc.SizeSmallMedium},
			[]combatsvc.Modifier{modStr(18, 1), gemMod},
			5 + 2,
		},
		{
			"short bow",
			combatsvc.AttackForm{WeaponID: "shortbow", AmmoItemID: "arrows_plus1", TargetSize: combatsvc.SizeSmallMedium},
			[]combatsvc.Modifier{modDex(17, 2), modWeapon(shortbow), modAmmo(arrows)},
			2 + 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := withRealBuffs(t, []buffsvc.BuffEffect{strengthTo18}, []buffsvc.ItemPower{gemHammer})
			f.forms.EXPECT().Get(mock.Anything, charID).Return(tt.form, true, nil).Once()
			f.weapons.EXPECT().List(mock.Anything).Return([]combatsvc.Weapon{longsword, shortbow}, nil)
			f.items.EXPECT().List(mock.Anything).Return([]itemsvc.Item{arrows}, nil).Maybe()
			f.withChar(bard())

			v, err := f.svc.GetAttackForm(context.Background(), charID)
			require.NoError(t, err)
			require.NotNil(t, v.Preview)
			assert.Equal(t, tt.wantMods, v.Preview.Modifiers)
			assert.Equal(t, tt.wantDmg, v.Preview.DamageBonus)
		})
	}
}

// ADR 0016 step 14 (ADR 0002 step 6, ADR 0003 step 11): every contribution
// of a power carries its item's id next to the item's stored name, in the
// result and in the logged payload: the weapon +N modifier, the weapon and
// the dice-multiplier damage terms. A buff's contributions carry none.
func TestItemPowers_Step14_BreakdownCarriesTheItemID(t *testing.T) {
	bless := buffsvc.BuffEffect{ID: "b", Name: "Bless", Effects: buffsvc.BuffEffects{ToHit: intPtr(1)}}
	f := withRealBuffs(t, []buffsvc.BuffEffect{strengthTo18, bless}, []buffsvc.ItemPower{gemHammer})
	f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil)
	f.withChar(bard())
	f.dice.EXPECT().RollD20().Return(14).Once()
	f.d100s(t, 95, noCritMult)
	f.dice.EXPECT().RollDie(8).Return(7).Once()
	f.records()

	res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(5)})
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.Modifier{modStr(18, 1), gemMod, modBuff("Bless", 1)}, res.Modifiers)
	assert.Equal(t, []combatsvc.DamageTerm{
		{Term: combatsvc.TermDiceMult, Kind: combatsvc.ModBuff, Detail: "Power Gem", ItemID: "power_gem_dispel", Value: 2},
		{Term: combatsvc.TermWeapon, Kind: combatsvc.ModWeaponBuff, Detail: "Power Gem", ItemID: "power_gem_dispel", Value: 5},
		{Term: combatsvc.TermStrength, Kind: combatsvc.ModStrength, Detail: "18", Value: 2},
	}, res.DamageTerms)
	assert.Equal(t, 7*2+5+2, res.TotalDamage)

	require.Len(t, f.entries, 1)
	var p struct {
		Modifiers   []map[string]any `json:"modifiers"`
		DamageTerms []map[string]any `json:"damageTerms"`
	}
	require.NoError(t, json.Unmarshal(f.entries[0].Payload, &p))
	assert.Equal(t, map[string]any{"kind": "weapon_buff", "detail": "Power Gem", "itemId": "power_gem_dispel", "value": 5.0}, p.Modifiers[1])
	assert.NotContains(t, p.Modifiers[2], "itemId", "a buff's modifier has no item")
	assert.Equal(t, "power_gem_dispel", p.DamageTerms[0]["itemId"])
	assert.Equal(t, "power_gem_dispel", p.DamageTerms[1]["itemId"])
}
