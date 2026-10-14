package combatsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	buffmocks "github.com/ceffo/critical/api/internal/domain/service/buffsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Weapons built by hand for these tests (independent of seeds/weapons.json).
var (
	longsword = combatsvc.Weapon{
		ID: "longsword", Name: "Long Sword +2", BaseDamage: "1d8", DamageLarge: "1d12",
		MagicToHit: 2, MagicDamage: 2, Kind: combatsvc.WeaponMelee,
	}
	bastardSword = combatsvc.Weapon{
		ID: "bastard_sword", Name: "Bastard Sword +2", BaseDamage: "2d4", DamageLarge: "2d8",
		MagicToHit: 2, MagicDamage: 2, Kind: combatsvc.WeaponMelee, TwoHanded: true,
	}
	shortbow = combatsvc.Weapon{
		ID: "shortbow", Name: "Short Bow +2", BaseDamage: "1d6", DamageLarge: "1d6",
		MagicToHit: 2, MagicDamage: 2, Kind: combatsvc.WeaponMissile, TwoHanded: true, AmmoKind: "arrow",
	}
	darts = combatsvc.Weapon{
		ID: "darts", Name: "Darts", BaseDamage: "1d3", DamageLarge: "1d2", Kind: combatsvc.WeaponMissile,
	}
	// arrows3Left1 is what itemsvc.Use returns after firing one of two +3 arrows.
	arrows3Left1 = itemsvc.Item{
		ID: "arrows_plus3", Name: "Arrows +3", Type: "arrow", Behaviour: itemsvc.BehaviourStock, MagicBonus: 3, AmmoKind: "arrow", Count: 1,
	}
)

func modStr(str, v int) combatsvc.Modifier {
	s := fmt.Sprint(str)
	return combatsvc.Modifier{Kind: combatsvc.ModStrength, Source: "Strength " + s, Detail: s, Value: v}
}

func modDex(dex, v int) combatsvc.Modifier {
	s := fmt.Sprint(dex)
	return combatsvc.Modifier{Kind: combatsvc.ModDexterity, Source: "Dexterity " + s, Detail: s, Value: v}
}

func modWeapon(w combatsvc.Weapon) combatsvc.Modifier {
	return combatsvc.Modifier{Kind: combatsvc.ModWeaponMagic, Source: w.Name, Detail: w.Name, Value: w.MagicToHit}
}

func modAmmo(i itemsvc.Item) combatsvc.Modifier {
	return combatsvc.Modifier{Kind: combatsvc.ModAmmoMagic, Source: i.Name, Detail: i.Name, Value: i.MagicBonus}
}

func modBuff(name string, v int) combatsvc.Modifier {
	return combatsvc.Modifier{Kind: combatsvc.ModBuff, Source: name, Detail: name, Value: v}
}

func modSituational(label string, v int) combatsvc.Modifier {
	return combatsvc.Modifier{Kind: combatsvc.ModSituational, Source: label, Detail: label, Value: v}
}

type attackCase struct {
	name   string
	weapon combatsvc.Weapon
	// mods is what the buffs port returns: EffectiveStr is the STR with
	// buffs (the sheet says 12).
	mods  buffsvc.BuffModifiers
	ammo  *itemsvc.Item // returned by itemsvc.Use for a bow
	req   combatsvc.AttackRequest
	d20   int
	d100  int
	sides int   // damage die
	dmg   []int // damage dice, rolled on a hit

	wantMods      []combatsvc.Modifier
	wantEffThac0  int
	wantHit       bool
	wantMargin    int
	wantCrit      bool
	wantDice      string
	wantTotal     int
	wantBreakdown string
}

// All expected values are worked by hand from PHB Table 1 (Strength: 8-15
// -> hit 0 / dmg 0; 17 -> +1/+1; 18 -> +1/+2) and Table 2 (Dexterity 17 ->
// missile +2) with THAC0 13. Hit when d20 >= effective THAC0 - target AC.
var attackCases = []attackCase{
	{
		// Mods: STR 12 +0, sword +2 = +2. Eff. THAC0 13-2 = 11; vs AC 5 needs 6.
		// Damage 1d8=5: ((5 × 1) + 2) × 1 + 0 = 7.
		name: "long sword +2 vs AC 5 at STR 12 hits on 6", weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5}, d20: 6, d100: 50, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(longsword)},
		wantEffThac0: 11, wantHit: true, wantMargin: 0, wantDice: "1d8", wantTotal: 7,
		wantBreakdown: "((5 × 1.0) + 2) × 1.0 + 0 = 7",
	},
	{
		name: "long sword +2 vs AC 5 at STR 12 misses on 5", weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5}, d20: 5, d100: 50, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(longsword)},
		wantEffThac0: 11, wantHit: false, wantMargin: -1,
	},
	{
		// Mods: STR 18 +1, sword +2 = +3. Eff. THAC0 10; vs AC 5 needs 5.
		// Damage 1d8=5: ((5 × 1) + 2) × 1 + 2 (STR 18) = 9.
		name: "long sword +2 vs AC 5 at effective STR 18 hits on 5", weapon: longsword, mods: strMods(18),
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5}, d20: 5, d100: 50, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(18, 1), modWeapon(longsword)},
		wantEffThac0: 10, wantHit: true, wantMargin: 0, wantDice: "1d8", wantTotal: 9,
		wantBreakdown: "((5 × 1.0) + 2) × 1.0 + 2 = 9",
	},
	{
		name: "long sword +2 vs AC 5 at effective STR 18 misses on 4", weapon: longsword, mods: strMods(18),
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5}, d20: 4, d100: 50, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(18, 1), modWeapon(longsword)},
		wantEffThac0: 10, wantHit: false, wantMargin: -1,
	},
	{
		// Bow: no STR at all (PHB: STR only with a strength bow), even at 18.
		// DMG 2e "Bow +1": magic bow + magic arrows -> total both bonuses.
		// Mods: DEX 17 +2, bow +2, arrow +3 = +7. Eff. THAC0 6; vs AC 2 needs 4.
		// Damage 1d6=4: ((4 × 1) + (2 + 3)) × 1 + 0 = 9.
		name: "short bow +2 with a +3 arrow at DEX 17 hits on 4", weapon: shortbow, mods: strMods(18), ammo: &arrows3Left1,
		req: combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus3", TargetAC: 2}, d20: 4, d100: 50,
		sides: 6, dmg: []int{4},
		wantMods:     []combatsvc.Modifier{modDex(17, 2), modWeapon(shortbow), modAmmo(arrows3Left1)},
		wantEffThac0: 6, wantHit: true, wantMargin: 0, wantDice: "1d6", wantTotal: 9,
		wantBreakdown: "((4 × 1.0) + 5) × 1.0 + 0 = 9",
	},
	{
		name: "short bow +2 with a +3 arrow at DEX 17 misses on 3", weapon: shortbow, mods: strMods(18), ammo: &arrows3Left1,
		req: combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus3", TargetAC: 2}, d20: 3, d100: 50, sides: 6, dmg: []int{4},
		wantMods:     []combatsvc.Modifier{modDex(17, 2), modWeapon(shortbow), modAmmo(arrows3Left1)},
		wantEffThac0: 6, wantHit: false, wantMargin: -1,
	},
	{
		// Mods: STR 12 +0, sword +2. Eff. THAC0 11; vs AC 5 needs 6; 14 hits by 8.
		// Large target: 2d8 = 3 + 6 = 9; ((9 × 1) + 2) × 1 + 0 = 11.
		name: "bastard sword +2 two-handed vs a large target rolls 2d8", weapon: bastardSword, mods: strMods(12),
		req: combatsvc.AttackRequest{WeaponID: "bastard_sword", TargetAC: 5, TargetSize: combatsvc.SizeLarge},
		d20: 14, d100: 50, sides: 8, dmg: []int{3, 6},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(bastardSword)},
		wantEffThac0: 11, wantHit: true, wantMargin: 8, wantDice: "2d8", wantTotal: 11,
		wantBreakdown: "((9 × 1.0) + 2) × 1.0 + 0 = 11",
	},
	{
		// Small/medium: 2d4 = 3 + 4 = 7; ((7 × 1) + 2) × 1 + 0 = 9.
		name: "bastard sword +2 vs small/medium rolls 2d4", weapon: bastardSword, mods: strMods(12),
		req: combatsvc.AttackRequest{WeaponID: "bastard_sword", TargetAC: 5, TargetSize: combatsvc.SizeSmallMedium},
		d20: 14, d100: 50, sides: 4, dmg: []int{3, 4},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(bastardSword)},
		wantEffThac0: 11, wantHit: true, wantMargin: 8, wantDice: "2d4", wantTotal: 9,
		wantBreakdown: "((7 × 1.0) + 2) × 1.0 + 0 = 9",
	},
	{
		// Mods: STR 12 +0, sword +2, Bless +1 = +3. Eff. THAC0 10; vs AC 5
		// needs 5: Bless turns the 5 (a miss above) into a hit.
		name: "buff to-hit (Bless +1) turns a 5 into a hit", weapon: longsword,
		mods: buffsvc.BuffModifiers{
			SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12, ToHit: 1,
			ToHitFrom: []buffsvc.Contribution{{Buff: "Bless", Value: 1}},
		},
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5}, d20: 5, d100: 50, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(longsword), modBuff("Bless", 1)},
		wantEffThac0: 10, wantHit: true, wantMargin: 0, wantDice: "1d8", wantTotal: 7,
		wantBreakdown: "((5 × 1.0) + 2) × 1.0 + 0 = 7",
	},
	{
		// Buff damage is added after the multipliers: ((5 × 1) + 2) × 1 + 0 + 2 = 9.
		name: "buff damage is a flat bonus after the multipliers", weapon: longsword,
		mods: buffsvc.BuffModifiers{
			SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12, Damage: 2,
			DamageFrom: []buffsvc.Contribution{{Buff: "Rage", Value: 2}},
		},
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5}, d20: 6, d100: 50, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(longsword)},
		wantEffThac0: 11, wantHit: true, wantMargin: 0, wantDice: "1d8", wantTotal: 9,
		wantBreakdown: "((5 × 1.0) + 2) × 1.0 + 0 + 2 = 9",
	},
	{
		// Mods: +2 (sword) +2 (rear attack) = +4. Eff. THAC0 9; vs AC 5 needs 4.
		name: "situational +2 (rear attack)", weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{
			WeaponID: "longsword", TargetAC: 5,
			Situational: &combatsvc.Situational{Value: 2, Label: " Rear attack "},
		},
		d20: 4, d100: 50, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(longsword), modSituational("Rear attack", 2)},
		wantEffThac0: 9, wantHit: true, wantMargin: 0, wantDice: "1d8", wantTotal: 7,
		wantBreakdown: "((5 × 1.0) + 2) × 1.0 + 0 = 7",
	},
	{
		// Mods: +2 (sword) -4 (cover) = -2. Eff. THAC0 15; vs AC 5 needs 10.
		name: "situational -4 without a label", weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{
			WeaponID: "longsword", TargetAC: 5,
			Situational: &combatsvc.Situational{Value: -4},
		},
		d20: 9, d100: 50, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(longsword), modSituational("Situational", -4)},
		wantEffThac0: 15, wantHit: false, wantMargin: -1,
	},
	{
		name: "situational 0 is not listed", weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{
			WeaponID: "longsword", TargetAC: 5,
			Situational: &combatsvc.Situational{Value: 0, Label: "nothing"},
		},
		d20: 5, d100: 50, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(longsword)},
		wantEffThac0: 11, wantHit: false, wantMargin: -1,
	},
	{
		// Needs 6. House crit: 15 - 9 = 6 >= 6 -> crit, x2 placeholder.
		// ((5 × 1) + 2) × 2 + 0 = 14.
		name: "crit: d20 - d100 reaches the roll needed (placeholder x2)", weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5}, d20: 15, d100: 9, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(longsword)},
		wantEffThac0: 11, wantHit: true, wantMargin: 9, wantCrit: true, wantDice: "1d8", wantTotal: 14,
		wantBreakdown: "((5 × 1.0) + 2) × 2.0 + 0 = 14",
	},
	{
		// 15 - 10 = 5 < 6: a plain hit.
		name: "no crit one short", weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5}, d20: 15, d100: 10, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(longsword)},
		wantEffThac0: 11, wantHit: true, wantMargin: 9, wantDice: "1d8", wantTotal: 7,
		wantBreakdown: "((5 × 1.0) + 2) × 1.0 + 0 = 7",
	},
	{
		// No special natural rolls: eff. THAC0 11 vs AC 10 needs 1, so a 1 hits.
		name: "natural 1 is plain THAC0 math: hits when 1 is enough", weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 10}, d20: 1, d100: 50, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(longsword)},
		wantEffThac0: 11, wantHit: true, wantMargin: 0, wantDice: "1d8", wantTotal: 7,
		wantBreakdown: "((5 × 1.0) + 2) × 1.0 + 0 = 7",
	},
	{
		// Eff. THAC0 11 vs AC -10 needs 21, so a 20 misses.
		name: "natural 20 is plain THAC0 math: misses when 21 is needed", weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: -10}, d20: 20, d100: 50, sides: 8, dmg: []int{5},
		wantMods:     []combatsvc.Modifier{modStr(12, 0), modWeapon(longsword)},
		wantEffThac0: 11, wantHit: false, wantMargin: -1,
	},
	{
		// Hurled weapon: STR (always for hurled, PHB) and DEX both apply.
		// Mods: STR 18 +1, DEX 17 +2 = +3. Eff. THAC0 10; vs AC 5 needs 5.
		// 1d3 = 2: ((2 × 1) + 0) × 1 + 2 = 4.
		name: "hurled darts take STR and DEX", weapon: darts, mods: strMods(18),
		req: combatsvc.AttackRequest{WeaponID: "darts", TargetAC: 5}, d20: 5, d100: 50, sides: 3, dmg: []int{2},
		wantMods:     []combatsvc.Modifier{modStr(18, 1), modDex(17, 2)},
		wantEffThac0: 10, wantHit: true, wantMargin: 0, wantDice: "1d3", wantTotal: 4,
		wantBreakdown: "((2 × 1.0) + 0) × 1.0 + 2 = 4",
	},
}

func TestResolveAttack(t *testing.T) {
	for _, tt := range attackCases {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.weapons.EXPECT().Get(mock.Anything, tt.req.WeaponID).Return(tt.weapon, nil)
			f.withChar(bard())
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(tt.mods, nil)
			if tt.ammo != nil {
				f.items.EXPECT().Use(mock.Anything, tt.req.AmmoItemID).Return(*tt.ammo, nil).Once()
			}
			f.dice.EXPECT().RollD20().Return(tt.d20).Once()
			f.d100s(t, tt.d100, noCritMult)
			for _, d := range tt.dmg {
				f.dice.EXPECT().RollDie(tt.sides).Return(d).Once()
			}
			f.records()

			res, err := f.svc.ResolveAttack(context.Background(), charID, tt.req)
			require.NoError(t, err)

			assert.Equal(t, tt.wantMods, res.Modifiers)
			assert.Equal(t, 13, res.BaseThac0)
			assert.Equal(t, tt.wantEffThac0, res.EffectiveThac0)
			assert.Equal(t, tt.wantEffThac0-tt.d20, res.TargetACNeeded)
			assert.Equal(t, tt.d20, res.D20Roll)
			assert.Equal(t, tt.d100, res.D100CritRoll)
			assert.Equal(t, tt.wantHit, res.Hit)
			assert.Equal(t, tt.wantMargin, res.HitMargin)
			assert.Equal(t, tt.wantCrit, res.IsCrit)
			assert.Equal(t, tt.wantCrit, res.CritPlaceholder)
			if tt.wantCrit {
				assert.InDelta(t, 2, res.CritMultiplier, 0)
			} else {
				assert.InDelta(t, 1, res.CritMultiplier, 0)
			}
			assert.Equal(t, tt.wantTotal, res.TotalDamage)
			assert.Equal(t, tt.wantBreakdown, res.DamageBreakdown)
			assert.Equal(t, tt.wantDice, res.DamageDice)
			assert.Equal(t, tt.weapon.Name, res.WeaponName)
			assert.Equal(t, tt.req.TargetAC, res.TargetAC)
			assert.Equal(t, int64(42), res.LogEntryID)
			if !tt.wantHit {
				assert.Zero(t, res.BaseDamage)
				assert.Empty(t, res.DamageTerms)
			}
			// The damage dice are rolled and stored on every attack, a miss too.
			sum := 0
			for _, d := range tt.dmg {
				sum += d
			}
			require.NotNil(t, res.Rolled)
			assert.Equal(t, sum, res.Rolled.Total)
			require.Len(t, f.entries, 1)
			assert.Equal(t, actionlog.KindAttack, f.entries[0].Kind)
			p := payloadOf(t, f.entries[0].Payload)
			require.Contains(t, p, "rolled")
			assert.Equal(t, float64(sum), p["rolled"].(map[string]any)["total"])
			assert.Contains(t, p, "critRow")
		})
	}
}

// Every contribution to the damage formula is listed, in formula order.
func TestResolveAttack_DamageTermsListEveryModifier(t *testing.T) {
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil)
	f.withChar(bard())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(buffsvc.BuffModifiers{
		SpellsBuff: 1.5, Multipliers: 2, EffectiveStr: 18, Damage: 1,
		SpellsBuffFrom:  []buffsvc.Factor{{Buff: "Enlarge", Value: 1.5}},
		MultipliersFrom: []buffsvc.Factor{{Buff: "Haste blade", Value: 2}},
		DamageFrom:      []buffsvc.Contribution{{Buff: "Rage", Value: 1}},
	}, nil)
	f.dice.EXPECT().RollD20().Return(15)
	f.d100s(t, 5, noCritMult) // 15 - 5 = 10 >= 5 needed: crit
	f.dice.EXPECT().RollDie(8).Return(5)
	f.records()

	res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5})
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.DamageTerm{
		{Term: combatsvc.TermSpells, Kind: combatsvc.ModBuff, Source: "Enlarge", Detail: "Enlarge", Value: 1.5},
		{Term: combatsvc.TermDiceMult, Kind: combatsvc.ModBuff, Source: "Haste blade", Detail: "Haste blade", Value: 2},
		{Term: combatsvc.TermWeapon, Kind: combatsvc.ModWeaponMagic, Source: "Long Sword +2", Detail: "Long Sword +2", Value: 2},
		{Term: combatsvc.TermMultiplier, Kind: combatsvc.ModCrit, Source: "Critical hit (check d100 5, multiplier d100 77, placeholder)", Detail: "placeholder", Value: 2},
		{Term: combatsvc.TermStrength, Kind: combatsvc.ModStrength, Source: "Strength 18", Detail: "18", Value: 2},
		{Term: combatsvc.TermBonus, Kind: combatsvc.ModBuff, Source: "Rage", Detail: "Rage", Value: 1},
	}, res.DamageTerms)
	// floor(((5 × 1.5 × 2 buff) + 2) × 2 crit) + 2 + 1 = 34 + 3 = 37.
	assert.Equal(t, 37, res.TotalDamage)
	assert.Equal(t, "((5 × 1.5 × 2.0) + 2) × 2.0 + 2 + 1 = 37", res.DamageBreakdown)
}

// The Power Gem's effects (weapon becomes +5, damage x2, min_str 18) apply
// only at effective STR 18. The real buffsvc sits behind the Buffs port so
// the effective STR comes from the Strength buff over the sheet's STR 12.
func TestResolveAttack_PowerGem(t *testing.T) {
	gem := buffsvc.BuffEffect{ID: "g", Name: "Power Gem", Effects: buffsvc.BuffEffects{
		MinStr: intPtr(18), WeaponBuff: intPtr(5), Multiplier: floatPtr(2.0),
	}}
	strength := func(bonus int) buffsvc.BuffEffect {
		return buffsvc.BuffEffect{ID: "s", Name: "Strength", Duration: 10, Effects: buffsvc.BuffEffects{
			StrBonus: intPtr(bonus), StrMax: intPtr(18),
		}}
	}
	gemMod := combatsvc.Modifier{Kind: combatsvc.ModWeaponBuff, Source: "Power Gem (weapon +5)", Detail: "Power Gem", Value: 5}

	tests := []struct {
		name      string
		bonus     int
		d100      int
		wantMods  []combatsvc.Modifier
		wantEff   int
		wantTotal int
		die       int // the 1d8; 0 = 5
		wantDiceX bool
	}{
		{
			// docs/backend.md "Damage Formula": the gem doubles the raw dice
			// only. 1d8=6: (6 × 2) + 5 + 2 = 19, NOT ((6 + 5) × 2) + 2 = 24.
			name: "x2 on the raw dice only, not on the +5", bonus: 6, d100: 50, die: 6, wantDiceX: true,
			wantMods: []combatsvc.Modifier{modStr(18, 1), gemMod}, wantEff: 7, wantTotal: 19,
		},
		{
			// 12 + 6 = 18. Mods: STR 18 +1, weapon becomes +5 (replaces the
			// sword's +2) = +6. Eff. THAC0 7. 1d8=5: (5 × 2 gem dice) + 5 + 2 = 17 (the +5 and STR are not doubled).
			name: "active at effective STR 18", bonus: 6, d100: 50,
			wantMods: []combatsvc.Modifier{modStr(18, 1), gemMod}, wantEff: 7, wantTotal: 17,
		},
		{
			// 12 + 5 = 17: the gem is ignored. Mods: STR 17 +1, sword +2 = +3.
			// Eff. THAC0 10. 1d8=5: ((5 × 1) + 2) × 1 + 1 = 8.
			name: "ignored at effective STR 17", bonus: 5, d100: 50,
			wantMods: []combatsvc.Modifier{modStr(17, 1), modWeapon(longsword)}, wantEff: 10, wantTotal: 8,
		},
		{
			// Needs 7 - 5 = 2; 10 - 2 = 8 >= 2: crit. Multipliers stack:
			// ((5 × 2 gem) + 5) × 2 crit + 2 = 32.
			name: "crit with the gem: gem on the dice, crit on dice and weapon", bonus: 6, d100: 2,
			wantMods: []combatsvc.Modifier{modStr(18, 1), gemMod}, wantEff: 7, wantTotal: 32,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dal := buffmocks.NewMockDAL(t)
			dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{strength(tt.bonus), gem}, nil)
			realBuffs, err := buffsvc.New(dal, noTx{}, buffmocks.NewMockLog(t), buffmocks.NewMockCharacters(t))
			require.NoError(t, err)

			f := newFixture(t)
			svc, err := combatsvc.New(combatsvc.Deps{
				Chars: f.chars, Buffs: realBuffs, Dice: f.dice, Weapons: f.weapons, Crit: f.crit,
				Items: f.items, Rules: phb(t), Log: f.log, Tx: noTx{},
			})
			require.NoError(t, err)
			f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil)
			f.withChar(bard())
			f.dice.EXPECT().RollD20().Return(10)
			f.d100s(t, tt.d100, noCritMult)
			die := tt.die
			if die == 0 {
				die = 5
			}
			f.dice.EXPECT().RollDie(8).Return(die)
			f.records()

			res, err := svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5})
			require.NoError(t, err)
			if tt.wantDiceX {
				assert.Equal(t, 2.0, res.DiceMultiplier)
				assert.Equal(t, 1.0, res.Multipliers, "no crit: the crit multiplier is 1")
				assert.Contains(t, res.DamageTerms, combatsvc.DamageTerm{Term: combatsvc.TermDiceMult, Kind: combatsvc.ModBuff, Source: "Power Gem", Detail: "Power Gem", Value: 2})
				assert.Equal(t, "((6 × 1.0 × 2.0) + 5) × 1.0 + 2 = 19", res.DamageBreakdown)
			}
			assert.Equal(t, tt.wantMods, res.Modifiers)
			assert.Equal(t, tt.wantEff, res.EffectiveThac0)
			assert.True(t, res.Hit)
			assert.Equal(t, tt.wantTotal, res.TotalDamage)
		})
	}
}

// docs/backend.md "Attack Modifiers", weapon magic: the weapon's own +N, or a
// buff's "weapon becomes +N" when higher; never stacked. Here the own +3
// beats the buff's +2, so the breakdown names the weapon, not the buff. Fails
// if the buff always replaced the weapon (it would show +2 from the buff).
func TestResolveAttack_OwnWeaponMagicBeatsLowerWeaponBuff(t *testing.T) {
	dagger := combatsvc.Weapon{
		ID: "dagger3", Name: "Dagger +3", BaseDamage: "1d4", DamageLarge: "1d3",
		MagicToHit: 3, MagicDamage: 3, Kind: combatsvc.WeaponMelee,
	}
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, "dagger3").Return(dagger, nil)
	f.withChar(bard())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(buffsvc.BuffModifiers{
		SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12, WeaponBuff: 2, WeaponBuffFrom: "Magic Weapon",
	}, nil)
	f.dice.EXPECT().RollD20().Return(10).Once()
	f.d100s(t, 33, noCritMult)
	f.dice.EXPECT().RollDie(4).Return(3).Once()
	f.records()

	res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "dagger3", TargetAC: 5})
	require.NoError(t, err)

	// To hit: STR 12 +0, dagger +3 (the buff's +2 is not added). Eff. THAC0 10; needs 5.
	assert.Equal(t, []combatsvc.Modifier{modStr(12, 0), modWeapon(dagger)}, res.Modifiers)
	assert.Equal(t, 10, res.EffectiveThac0)
	assert.True(t, res.Hit)
	// Damage 1d4=3: ((3 × 1) + 3) × 1 + 0 = 6; one weapon term, the dagger's.
	assert.Equal(t, 3, res.WeaponBuff)
	assert.Equal(t, 6, res.TotalDamage)
	assert.Equal(t, []combatsvc.DamageTerm{
		{Term: combatsvc.TermWeapon, Kind: combatsvc.ModWeaponMagic, Source: "Dagger +3", Detail: "Dagger +3", Value: 3},
		{Term: combatsvc.TermStrength, Kind: combatsvc.ModStrength, Source: "Strength 12", Detail: "12", Value: 0},
	}, res.DamageTerms)
}

// docs/backend.md "Attack Modifiers", ammo magic: a bow with no magic of its
// own, a buff making it +5, and a +3 arrow total +8 (to hit and damage) as two
// separate entries. Fails if the buff replaced the ammo bonus (+5 only).
func TestResolveAttack_WeaponBuffPlusMagicAmmo(t *testing.T) {
	bow := combatsvc.Weapon{
		ID: "plainbow", Name: "Short Bow", BaseDamage: "1d6", DamageLarge: "1d6",
		Kind: combatsvc.WeaponMissile, TwoHanded: true, AmmoKind: "arrow",
	}
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, "plainbow").Return(bow, nil)
	f.withChar(bard())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(buffsvc.BuffModifiers{
		SpellsBuff: 1, Multipliers: 1, EffectiveStr: 18, WeaponBuff: 5, WeaponBuffFrom: "Magic Weapon",
	}, nil)
	f.items.EXPECT().Use(mock.Anything, "arrows_plus3").Return(arrows3Left1, nil).Once()
	f.dice.EXPECT().RollD20().Return(9).Once()
	f.d100s(t, 33, noCritMult)
	f.dice.EXPECT().RollDie(6).Return(4).Once()
	f.records()

	res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "plainbow", AmmoItemID: "arrows_plus3", TargetAC: 5})
	require.NoError(t, err)

	buffMod := combatsvc.Modifier{Kind: combatsvc.ModWeaponBuff, Source: "Magic Weapon (weapon +5)", Detail: "Magic Weapon", Value: 5}
	// To hit: DEX 17 +2 (no STR for a bow), buff +5, arrow +3 = +10. Eff. THAC0 3; needs -2.
	assert.Equal(t, []combatsvc.Modifier{modDex(17, 2), buffMod, modAmmo(arrows3Left1)}, res.Modifiers)
	assert.Equal(t, 3, res.EffectiveThac0)
	assert.True(t, res.Hit)
	// Damage 1d6=4: ((4 × 1) + (5 + 3)) × 1 + 0 = 12; two separate weapon terms.
	assert.Equal(t, 8, res.WeaponBuff)
	assert.Equal(t, 12, res.TotalDamage)
	assert.Equal(t, []combatsvc.DamageTerm{
		{Term: combatsvc.TermWeapon, Kind: combatsvc.ModWeaponBuff, Source: "Magic Weapon (weapon +5)", Detail: "Magic Weapon", Value: 5},
		{Term: combatsvc.TermWeapon, Kind: combatsvc.ModAmmoMagic, Source: "Arrows +3", Detail: "Arrows +3", Value: 3},
	}, res.DamageTerms)
}

// A missile attack spends the arrow and records the attack with the full
// result and an items snapshot in the same transaction.
func TestResolveAttack_RecordsAttackWithAmmoSnapshot(t *testing.T) {
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, "shortbow").Return(shortbow, nil)
	f.withChar(bard())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(strMods(12), nil)
	f.items.EXPECT().Use(mock.Anything, "arrows_plus3").Return(arrows3Left1, nil).Once()
	f.dice.EXPECT().RollD20().Return(4)
	f.d100s(t, 50, noCritMult)
	f.dice.EXPECT().RollDie(6).Return(4)
	f.records()

	res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{
		WeaponID: "shortbow", AmmoItemID: "arrows_plus3", TargetAC: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, &combatsvc.AmmoUse{ItemID: "arrows_plus3", Name: "Arrows +3", Magic: 3, Remaining: 1}, res.Ammo)

	require.Len(t, f.entries, 1)
	e := f.entries[0]
	assert.Equal(t, charID, e.CharacterID)
	assert.Equal(t, actionlog.KindAttack, e.Kind)
	assert.Zero(t, e.HPDelta)
	assert.Equal(t, "Short Bow +2 (Arrows +3) vs AC 2: hit, 9 damage", e.Summary)
	require.Len(t, e.Snapshots, 1)
	assert.Equal(t, "items:arrows_plus3", e.Snapshots[0].Key)
	assert.JSONEq(t, `[{"id":"arrows_plus3","count":2}]`, string(e.Snapshots[0].Before))
	assert.JSONEq(t, `[{"id":"arrows_plus3","count":1}]`, string(e.Snapshots[0].After))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(e.Payload, &payload))
	assert.Equal(t, true, payload["hit"])
	assert.EqualValues(t, 9, payload["totalDamage"])
	assert.EqualValues(t, 6, payload["effectiveThac0"])
	assert.Equal(t, "Short Bow +2", payload["weaponName"])
	assert.Len(t, payload["modifiers"], 3)
	assert.Len(t, payload["damageTerms"], 2)
	assert.Equal(t, map[string]any{"itemId": "arrows_plus3", "name": "Arrows +3", "magic": 3.0, "remaining": 1.0}, payload["ammo"])
}

func TestResolveAttack_MeleeIsLoggedWithoutSnapshot(t *testing.T) {
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil)
	f.withChar(bard())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(strMods(12), nil)
	f.dice.EXPECT().RollD20().Return(3)
	f.d100s(t, 50, noCritMult)
	f.dice.EXPECT().RollDie(8).Return(5)
	f.records()

	_, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5})
	require.NoError(t, err)
	require.Len(t, f.entries, 1)
	assert.Equal(t, "Long Sword +2 vs AC 5: miss", f.entries[0].Summary)
	assert.Empty(t, f.entries[0].Snapshots)
}

func TestResolveAttack_Rejections(t *testing.T) {
	boom := errors.New("boom")
	ctx := context.Background()

	t.Run("unknown target size", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.svc.ResolveAttack(ctx, charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetSize: "huge"})
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("unknown weapon", func(t *testing.T) {
		f := newFixture(t)
		f.weapons.EXPECT().Get(mock.Anything, "nope").Return(combatsvc.Weapon{}, combatsvc.ErrWeaponNotFound)
		res, err := f.svc.ResolveAttack(ctx, charID, combatsvc.AttackRequest{WeaponID: "nope"})
		assert.Nil(t, res)
		assert.ErrorIs(t, err, combatsvc.ErrWeaponNotFound)
	})

	t.Run("character lookup error keeps the sentinel", func(t *testing.T) {
		f := newFixture(t)
		f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil)
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(nil, domain.ErrCharacterNotFound)
		_, err := f.svc.ResolveAttack(ctx, charID, combatsvc.AttackRequest{WeaponID: "longsword"})
		assert.ErrorIs(t, err, domain.ErrCharacterNotFound)
	})

	t.Run("aggregate error", func(t *testing.T) {
		f := newFixture(t)
		f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil)
		f.withChar(bard())
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(buffsvc.BuffModifiers{}, boom)
		_, err := f.svc.ResolveAttack(ctx, charID, combatsvc.AttackRequest{WeaponID: "longsword"})
		assert.ErrorIs(t, err, boom)
	})

	ammoCases := []struct {
		name    string
		weapon  combatsvc.Weapon
		ammoID  string
		use     *itemsvc.Item
		useErr  error
		wantErr error
	}{
		{name: "a bow without ammunition", weapon: shortbow, wantErr: domain.ErrInvalidInput},
		{name: "ammunition for a sword", weapon: longsword, ammoID: "arrows_plus3", wantErr: domain.ErrInvalidInput},
		{
			name: "no arrows left", weapon: shortbow, ammoID: "arrows_plus3",
			useErr: fmt.Errorf("%w: arrows_plus3", itemsvc.ErrItemEmpty), wantErr: itemsvc.ErrItemEmpty,
		},
		{
			name: "not ammunition", weapon: shortbow, ammoID: "wand",
			use: &itemsvc.Item{ID: "wand", Name: "Wand", Behaviour: itemsvc.BehaviourCharges, Count: 3}, wantErr: domain.ErrInvalidInput,
		},
		{
			name: "ammunition of another kind", weapon: shortbow, ammoID: "bolts",
			use: &itemsvc.Item{ID: "bolts", Name: "Bolts", Behaviour: itemsvc.BehaviourStock, AmmoKind: "bolt"}, wantErr: domain.ErrInvalidInput,
		},
	}
	for _, tt := range ammoCases {
		// Nothing is rolled or recorded; the transaction rolls the Use back.
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.weapons.EXPECT().Get(mock.Anything, tt.weapon.ID).Return(tt.weapon, nil)
			f.withChar(bard())
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(strMods(12), nil)
			if tt.use != nil || tt.useErr != nil {
				var item itemsvc.Item
				if tt.use != nil {
					item = *tt.use
				}
				f.items.EXPECT().Use(mock.Anything, tt.ammoID).Return(item, tt.useErr)
			}
			_, err := f.svc.ResolveAttack(ctx, charID, combatsvc.AttackRequest{WeaponID: tt.weapon.ID, AmmoItemID: tt.ammoID})
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}

	t.Run("record error fails the attack", func(t *testing.T) {
		f := newFixture(t)
		f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil)
		f.withChar(bard())
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(strMods(12), nil)
		f.dice.EXPECT().RollD20().Return(3)
		f.d100s(t, 50, noCritMult)
		f.dice.EXPECT().RollDie(8).Return(5)
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(0, boom)
		res, err := f.svc.ResolveAttack(ctx, charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5})
		assert.Nil(t, res)
		assert.ErrorIs(t, err, boom)
	})

	t.Run("bad damage dice", func(t *testing.T) {
		f := newFixture(t)
		bad := longsword
		bad.BaseDamage = "d8"
		f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(bad, nil)
		f.withChar(bard())
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(strMods(12), nil)
		f.dice.EXPECT().RollD20().Return(20)
		f.d100s(t, 99, noCritMult)
		_, err := f.svc.ResolveAttack(ctx, charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5})
		assert.ErrorContains(t, err, `invalid damage dice "d8"`)
	})
}

func TestListWeapons(t *testing.T) {
	f := newFixture(t)
	f.weapons.EXPECT().List(mock.Anything).Return([]combatsvc.Weapon{longsword}, nil)
	got, err := f.svc.ListWeapons(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.Weapon{longsword}, got)
}
