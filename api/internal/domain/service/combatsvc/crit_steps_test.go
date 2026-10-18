package combatsvc_test

// One test per numbered step of docs/adr/0001-critical-hits.md. Every die
// returns a distinct value and is expected an exact number of times (d20
// once, d100 twice per attack, via fixture.d100s), so a roll reused for two
// purposes, or not rolled, fails.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// stepAttack rolls the long sword (STR 12: effective THAC0 11) against ac
// with d20, check and multiplier d100s and a 1d8 of 5; mods default to none.
func stepAttack(t *testing.T, f *fixture, ac, d20, check, mult int, mods *buffsvc.BuffModifiers) *combatsvc.RollResult {
	t.Helper()
	m := strMods(12)
	if mods != nil {
		m = *mods
	}
	f.weapons.EXPECT().Get(mock.Anything, charID, "longsword").Return(longsword, nil).Once()
	f.withChar(bard())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12, mock.Anything).Return(m, nil).Once()
	f.dice.EXPECT().RollD20().Return(d20).Once()
	f.d100s(t, check, mult)
	f.dice.EXPECT().RollDie(8).Return(5).Once()
	f.records()
	res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(ac)})
	require.NoError(t, err)
	return res
}

// Step 1: every attack rolls the d20 and the crit-check d100 together (the
// first d100 is the check).
func TestCrit_Step1_RollsD20AndCheckD100(t *testing.T) {
	f := newFixture(t)
	res := stepAttack(t, f, 5, 13, 42, 87, nil)
	assert.Equal(t, 13, res.D20Roll)
	assert.Equal(t, 42, res.D100CritRoll, "the first d100 is the crit check")
	p := payloadOf(t, f.entries[0].Payload)
	assert.Equal(t, 13.0, p["d20Roll"])
	assert.Equal(t, 42.0, p["d100CritRoll"])
}

// Step 2: crit when d20 - crit-check d100 >= effective THAC0 - target AC
// (11 - 5 = 6 here); the multiplier d100 plays no part in it.
func TestCrit_Step2_CritWhenD20MinusCheckD100ReachesNeeded(t *testing.T) {
	cases := []struct {
		name            string
		d20, check, mul int
		want            bool
	}{
		// 15 - 9 = 6 >= 6; with the multiplier d100 as the check (15 - 95) it would not be.
		{"exactly the roll needed is a crit", 15, 9, 95, true},
		// 15 - 10 = 5 < 6; with the multiplier d100 as the check (15 - 1) it would be.
		{"one short is no crit", 15, 10, 1, false},
		// 20 - 15 = 5 < 6: a natural 20 is plain arithmetic, nothing more.
		{"natural 20 needs the check like any roll", 20, 15, 2, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			res := stepAttack(t, f, 5, c.d20, c.check, c.mul, nil)
			assert.Equal(t, c.want, res.IsCrit)
			assert.True(t, res.Hit, "every case hits (needs 6)")
		})
	}
}

// Step 3: a second, independent d100 is rolled on every attack (miss, hit,
// crit) and stored.
func TestCrit_Step3_MultiplierD100RolledOnEveryAttack(t *testing.T) {
	cases := []struct {
		name            string
		d20, check, mul int
		hit, crit       bool
	}{
		{"miss", 3, 40, 63, false, false},
		{"hit, no crit", 12, 40, 63, true, false},
		{"crit", 18, 4, 63, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			res := stepAttack(t, f, 5, c.d20, c.check, c.mul, nil)
			assert.Equal(t, c.hit, res.Hit)
			assert.Equal(t, c.crit, res.IsCrit)
			assert.Equal(t, c.mul, res.D100MultiplierRoll)
			assert.Equal(t, float64(c.mul), payloadOf(t, f.entries[0].Payload)["d100MultiplierRoll"])
		})
	}
}

// Step 4: on a crit the multiplier is the crit-table row of the multiplier
// d100 (not of the check d100); the default table is a x2 placeholder; no
// crit is x1.
func TestCrit_Step4_MultiplierUsesSecondD100(t *testing.T) {
	table := rows([3]int{1, 50, 2}, [3]int{51, 90, 3}, [3]int{91, 100, 4})
	cases := []struct {
		name       string
		table      []combatsvc.CritRow
		d20, check int
		mult       int
		wantCrit   bool
		wantMult   float64
		wantFrom   int
		wantTo     int
		wantPlaceh bool
		wantTotal  int
	}{
		// Check 7 would pick x2 (1-50); the multiplier d100 63 picks x3 (51-90).
		// ((5 × 1) + 2) × 3 = 21.
		{
			name: "crit: row of the multiplier d100", table: table, d20: 18, check: 7, mult: 63,
			wantCrit: true, wantMult: 3, wantFrom: 51, wantTo: 90, wantTotal: 21,
		},
		// Check 60 (no crit at 18 - 60) would have picked x3; the multiplier d100 95 picks x4.
		{
			name: "crit on a big roll: x4 from 95", table: table, d20: 20, check: 12, mult: 95,
			wantCrit: true, wantMult: 4, wantFrom: 91, wantTo: 100, wantTotal: 28,
		},
		{
			name: "default table: x2 placeholder", table: nil, d20: 18, check: 7, mult: 63,
			wantCrit: true, wantMult: 2, wantFrom: 1, wantTo: 100, wantPlaceh: true, wantTotal: 14,
		},
		{
			name: "no crit: x1 whatever the multiplier d100", table: table, d20: 12, check: 40, mult: 95,
			wantMult: 1, wantTotal: 7,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.critRows = c.table
			res := stepAttack(t, f, 5, c.d20, c.check, c.mult, nil)
			assert.Equal(t, c.wantCrit, res.IsCrit)
			assert.InDelta(t, c.wantMult, res.CritMultiplier, 0)
			assert.Equal(t, c.wantFrom, res.CritFrom)
			assert.Equal(t, c.wantTo, res.CritTo)
			assert.Equal(t, c.wantPlaceh, res.CritPlaceholder)
			assert.Equal(t, c.wantTotal, res.TotalDamage)
			if c.wantCrit {
				// The crit term carries the multiplier; the UI labels it from
				// the roll's crit range (CritFrom..CritTo, CritPlaceholder).
				var crit []combatsvc.DamageTerm
				for _, d := range res.DamageTerms {
					if d.Kind == combatsvc.ModCrit {
						crit = append(crit, d)
					}
				}
				require.Len(t, crit, 1)
				assert.Equal(t, combatsvc.DamageTerm{Term: combatsvc.TermMultiplier, Kind: combatsvc.ModCrit, Value: c.wantMult}, crit[0])
			}
		})
	}
}

// Step 5: the crit multiplier (from the multiplier d100's row) multiplies
// the dice part and the weapon bonus, after the buff multipliers, which
// apply to the raw dice only; full precision, floored once at the end,
// never below 0.
func TestCrit_Step5_MultipliersStackFlooredOnce(t *testing.T) {
	cases := []struct {
		name      string
		mods      buffsvc.BuffModifiers
		wantTotal int
		wantBreak float64
	}{
		{
			// ((5 × 3 dice buff) + 2) × 1.5 crit = 25.5 → 25, floored once at the end.
			name: "x1.5 crit times x3 buff floors once, not after the crit",
			mods: buffsvc.BuffModifiers{
				SpellsBuff: 1, Multipliers: 3, EffectiveStr: 12,
				MultipliersFrom: []buffsvc.Factor{{Buff: "Gem", Value: 3}},
			},
			wantTotal: 25, wantBreak: 25.5,
		},
		{
			// 7 × 1.5 - 20 = -9.5 → 0.
			name: "never below 0",
			mods: buffsvc.BuffModifiers{
				SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12, Damage: -20,
				DamageFrom: []buffsvc.Contribution{{Buff: "Curse", Value: -20}},
			},
			wantTotal: 0, wantBreak: -9.5,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			// Check d100 3 would pick x5; the multiplier d100 60 picks x1.5.
			f.critRows = []combatsvc.CritRow{{From: 1, To: 50, Multiplier: 5}, {From: 51, To: 100, Multiplier: 1.5}}
			res := stepAttack(t, f, 5, 18, 3, 60, &c.mods)
			require.True(t, res.IsCrit)
			assert.Equal(t, c.wantTotal, res.TotalDamage)
			assert.InDelta(t, c.wantBreak, res.UnroundedDamage, 1e-9)
		})
	}
}

// Step 6: a target AC correction re-runs the crit check and the multiplier
// with the stored d20, check d100, multiplier d100 and its row; nothing is
// rolled (the dice mocks allow exactly the attack's rolls).
func TestCrit_Step6_CorrectionReusesStoredD100s(t *testing.T) {
	f := newFixture(t)
	f.critRows = rows([3]int{1, 50, 2}, [3]int{51, 100, 3})
	// Rolled vs AC -10 (needs 21): d20 15 misses. Check 4, multiplier 63.
	e := rollAttack(t, f, withMult(sword(-10, 15, 4), 63))
	// Edited after the roll: the correction keeps the row as rolled.
	f.critRows = rows([3]int{1, 100, 9})
	amends(f, e)

	// AC 5 needs 6: 15 - 4 = 11 >= 6, a crit; the multiplier d100 63 is in
	// 51-100 (x3): ((5 × 1) + 2) × 3 = 21.
	res, err := f.svc.AdjustAttackAC(context.Background(), charID, 42, 5)
	require.NoError(t, err)
	assert.True(t, res.IsCrit)
	assert.Equal(t, 4, res.D100CritRoll)
	assert.Equal(t, 63, res.D100MultiplierRoll)
	assert.InDelta(t, 3, res.CritMultiplier, 0)
	assert.Equal(t, 21, res.TotalDamage)

	// AC 0 needs 11: 15 - 4 = 11, still a crit; AC -1 needs 12: a hit, no crit.
	res, err = f.svc.AdjustAttackAC(context.Background(), charID, 42, -1)
	require.NoError(t, err)
	assert.True(t, res.Hit)
	assert.False(t, res.IsCrit)
	assert.Equal(t, 7, res.TotalDamage)
}
