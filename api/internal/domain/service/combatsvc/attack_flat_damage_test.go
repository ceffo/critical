package combatsvc_test

// ADR 0003 steps 1 and 2 (amended): a damage as printed (1d4+1, 1d3-1, a
// flat 1). Its flat part K is part of the weapon's standard damage: Enlarge
// (PHB p.134), the backstab (PHB p.40) and the crit multiply it with the
// dice; only the Power Gem's ×2 stays on the raw dice (user ruling). Each multiplier has its own test, with numbers
// where the other order gives another total.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// mace is a catalogue footman's mace: 1d6+1, no magic.
var mace = combatsvc.Weapon{
	Wielded: true, ID: "footmans_mace", Name: localized.Of("Footman's mace"),
	BaseDamage: "1d6+1", DamageLarge: "1d6", Kind: combatsvc.WeaponMelee,
}

// str12Term is the bard's Strength 12 damage term (+0, listed).
var str12Term = combatsvc.DamageTerm{Term: combatsvc.TermStrength, Kind: combatsvc.ModStrength, Detail: "12"}

func flatTerm(dice string, k int) combatsvc.DamageTerm {
	return combatsvc.DamageTerm{Term: combatsvc.TermWeapon, Kind: combatsvc.ModDiceModifier, Detail: dice, Value: float64(k)}
}

// flatAttack runs one hit of w by char, the damage die returning die (none
// expected when sides is 0).
func flatAttack(t *testing.T, char charactersvc.View, w combatsvc.Weapon, mods buffsvc.BuffModifiers, crit []combatsvc.CritRow, d20, critCheck, sides, die int, req combatsvc.AttackRequest) (*combatsvc.RollResult, *fixture) {
	t.Helper()
	f := newFixture(t)
	f.critRows = crit
	f.weapons.EXPECT().Get(mock.Anything, charID, w.ID).Return(w, nil).Once()
	f.withChar(char)
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(mods, nil).Once()
	f.dice.EXPECT().RollD20().Return(d20).Once()
	f.d100s(t, critCheck, noCritMult)
	if sides > 0 {
		f.dice.EXPECT().RollDie(sides).Return(die).Once()
	}
	f.records()
	req.WeaponID, req.TargetAC = w.ID, new(5)
	res, err := f.svc.ResolveAttack(context.Background(), charID, req)
	require.NoError(t, err)
	require.True(t, res.Hit)
	return res, f
}

// ADR 0003 step 1: the dice of a damage as printed are rolled, its flat part
// is kept apart (logged, so a correction recomputes it); a flat damage rolls
// no die. A negative total is 0 (step 10).
func TestDamage_Step1_DamageAsPrinted(t *testing.T) {
	cases := []struct {
		name      string
		sm        string
		sides     int
		die       int
		wantK     int
		wantTotal int
	}{
		{"NdM+K: one d6, +1", "1d6+1", 6, 4, 1, 5},
		{"NdM-K: one d3, -1, never below 0", "1d3-1", 3, 1, -1, 0},
		{"flat K: no die", "1", 0, 0, 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := mace
			w.BaseDamage = c.sm
			res, f := flatAttack(t, bard(), w, strMods(12), nil, 17, 92, c.sides, c.die, combatsvc.AttackRequest{TargetSize: combatsvc.SizeSmallMedium})
			assert.Equal(t, c.sm, res.DamageDice)
			assert.Equal(t, c.die, res.BaseDamage, "the dice total, without K")
			assert.Equal(t, c.wantK, res.DiceModifier)
			assert.Equal(t, c.wantTotal, res.TotalDamage)
			assert.Equal(t, []combatsvc.DamageTerm{flatTerm(c.sm, c.wantK), str12Term}, res.DamageTerms)
			var p map[string]any
			require.NoError(t, json.Unmarshal(f.entries[0].Payload, &p))
			assert.Equal(t, float64(c.wantK), p["diceModifier"])
			assert.Equal(t, float64(c.wantK), p["rolled"].(map[string]any)["diceModifier"])
		})
	}
}

// ADR 0003 steps 2 and 3: Enlarge (×1.5) multiplies the standard damage,
// dice and K (PHB p.134 "damage rolls increase proportionately").
// 1d6 = 5: (5 + 1) × 1.5 = 9, not 5 × 1.5 + 1 = 8.5 → 8.
func TestDamage_Step2_FlatPartMultipliedBySpellsBuff(t *testing.T) {
	mods := strMods(12)
	mods.SpellsBuff, mods.SpellsBuffFrom = 1.5, []buffsvc.Factor{{Buff: "Enlarge", Value: 1.5}}
	res, _ := flatAttack(t, bard(), mace, mods, nil, 17, 92, 6, 5, combatsvc.AttackRequest{})
	assert.Equal(t, 9, res.TotalDamage)
	assert.Equal(t, []combatsvc.DamageTerm{
		flatTerm("1d6+1", 1),
		{Term: combatsvc.TermSpells, Kind: combatsvc.ModBuff, Detail: "Enlarge", Value: 1.5},
		str12Term,
	}, res.DamageTerms)
}

// ADR 0003 steps 2 and 4: the Power Gem's ×2 is on the raw dice only (user,
// 2026-10-14), not on K. 1d6 = 5: 5 × 2 + 1 = 11, not (5 + 1) × 2 = 12.
func TestDamage_Step2_FlatPartAfterDiceMultiplier(t *testing.T) {
	mods := strMods(12)
	mods.Multipliers, mods.MultipliersFrom = 2, []buffsvc.Factor{{Buff: "Power Gem", Value: 2}}
	res, _ := flatAttack(t, bard(), mace, mods, nil, 17, 92, 6, 5, combatsvc.AttackRequest{})
	assert.Equal(t, 11, res.TotalDamage)
	assert.Equal(t, []combatsvc.DamageTerm{
		{Term: combatsvc.TermDiceMult, Kind: combatsvc.ModBuff, Detail: "Power Gem", Value: 2},
		flatTerm("1d6+1", 1),
		str12Term,
	}, res.DamageTerms)
}

// ADR 0003 step 2 and ADR 0030 step 19: the backstab multiplies the
// weapon's standard damage, dice and K (PHB p.40), not Strength. L5 thief
// STR 16 (+1 damage), 1d6 = 3: (3 + 1) × 3 + 1 = 13, not 3 × 3 + 1 + 1 = 11.
func TestDamage_Step2_FlatPartMultipliedByBackstab(t *testing.T) {
	res, _ := flatAttack(t, thief(), mace, str16Mods, nil, 20, 90, 6, 3, combatsvc.AttackRequest{Backstab: true})
	assert.Equal(t, 13, res.TotalDamage)
	assert.Equal(t, 3.0, res.BackstabMultiplier)
	assert.Equal(t, 1.0, res.DiceMultiplier, "the backstab is not a raw-dice factor")
	assert.Equal(t, []combatsvc.DamageTerm{
		flatTerm("1d6+1", 1),
		{Term: combatsvc.TermStandardMult, Kind: combatsvc.ModBackstab, Value: 3},
		{Term: combatsvc.TermStrength, Kind: combatsvc.ModStrength, Detail: "16", Value: 1},
	}, res.DamageTerms)
}

// ADR 0003 steps 2 and 6: the crit multiplies K with the dice and the
// weapon's magic. 1d6 = 5, crit ×2: (5 + 1) × 2 = 12, not 5 × 2 + 1 = 11.
func TestDamage_Step2_FlatPartMultipliedByCrit(t *testing.T) {
	res, _ := flatAttack(t, bard(), mace, strMods(12), []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 2}}, 17, 2, 6, 5, combatsvc.AttackRequest{})
	require.True(t, res.IsCrit)
	assert.Equal(t, 12, res.TotalDamage)
	assert.Equal(t, []combatsvc.DamageTerm{
		flatTerm("1d6+1", 1),
		{Term: combatsvc.TermMultiplier, Kind: combatsvc.ModCrit, Value: 2},
		str12Term,
	}, res.DamageTerms)
}

// ADR 0003 step 2 and ADR 0030 step 19: a corrected target AC recomputes a
// backstab from the logged factor, K inside it: (3 + 1) × 3 + 1 = 13 again.
// An attack logged before the backstab had its own factor (inside
// buffMultipliers, no backstabMultiplier, no K) keeps its old total.
func TestAdjustAttackAC_Step2_BackstabFactorIsLogged(t *testing.T) {
	t.Run("logged apart", func(t *testing.T) {
		_, f := flatAttack(t, thief(), mace, str16Mods, nil, 20, 90, 6, 3, combatsvc.AttackRequest{Backstab: true})
		n := f.entries[0]
		e := &actionlog.Entry{ID: 42, CharacterID: charID, Kind: n.Kind, Summary: n.Summary, Payload: n.Payload}
		amends(f, e)
		res, err := f.svc.AdjustAttackAC(context.Background(), charID, 42, 7)
		require.NoError(t, err)
		assert.Equal(t, 3.0, res.BackstabMultiplier)
		assert.Equal(t, 13, res.TotalDamage)
	})
	t.Run("logged inside the dice multiplier", func(t *testing.T) {
		_, f := flatAttack(t, thief(), dagger1, str16Mods, nil, 20, 90, 4, 3, combatsvc.AttackRequest{Backstab: true})
		n := f.entries[0]
		var p map[string]any
		require.NoError(t, json.Unmarshal(n.Payload, &p))
		delete(p, "backstabMultiplier")
		p["diceMultiplier"] = 3.0
		rolled := p["rolled"].(map[string]any)
		delete(rolled, "backstabMultiplier")
		rolled["buffMultipliers"] = 3.0
		payload, err := json.Marshal(p)
		require.NoError(t, err)
		e := &actionlog.Entry{ID: 42, CharacterID: charID, Kind: n.Kind, Summary: n.Summary, Payload: payload}
		amends(f, e)
		res, err := f.svc.AdjustAttackAC(context.Background(), charID, 42, 7)
		require.NoError(t, err)
		assert.Equal(t, 0.0, res.BackstabMultiplier)
		assert.Equal(t, 3.0, res.DiceMultiplier)
		// 3 × 3 + 1 (dagger +1) + 1 (STR 16) = 11.
		assert.Equal(t, 11, res.TotalDamage)
	})
}
