package combatsvc_test

// One test per numbered step of docs/adr/0012-attack-against-unknown-ac.md.
// Every die returns a distinct value and is expected an exact number of
// times (d20 once, d100 twice, the damage dice once per attack): a roll
// reused, or rolled again by a verdict or a correction, fails the mocks.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// unknownSword is the long sword at STR 12 (effective THAC0 11) against an
// unknown AC, with a 1d8 of 5: a hit does ((5 × 1) + 2) × crit.
func unknownSword(d20, check, mult int) rolledAttack {
	return rolledAttack{
		weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{WeaponID: "longsword"},
		d20: d20, d100: check, mult: mult, sides: 8, dmg: []int{5},
	}
}

// Step 1: an attack may omit the target AC. It is rolled and logged, not
// settled: no hit, no crit, the damage of a hit at x1 shown as "if it hits".
func TestUnknownAC_Step1_AttackWithoutAC(t *testing.T) {
	f := newFixture(t)
	// d20 15 would hit AC 0 (needs 11) and crit it (15 - 3 = 12 >= 11): an
	// unknown AC read as 0 fails here.
	rollAttack(t, f, unknownSword(15, 3, 64))
	p := payloadOf(t, f.entries[0].Payload)
	assert.Nil(t, p["targetAc"], "no AC stored")
	assert.Equal(t, false, p["acKnown"])
	assert.Equal(t, true, p["rolledAcUnknown"])
	assert.Nil(t, p["verdict"], "no verdict yet")
	assert.Equal(t, false, p["hit"], "not settled: no hit")
	assert.Equal(t, false, p["isCrit"])
	assert.Equal(t, 1.0, p["critMultiplier"])
	assert.Equal(t, 0.0, p["hitMargin"])
	// The damage if it hits, at x1: ((5 × 1) + 2) × 1 = 7.
	assert.Equal(t, 7.0, p["totalDamage"])
	assert.Equal(t, 5.0, p["baseDamage"])
}

// Step 1, through the result: an attack without the AC is not settled, one
// with it is.
func TestUnknownAC_Step1_ResultNotSettled(t *testing.T) {
	for _, ac := range []*int{nil, new(0)} {
		f := newFixture(t)
		f.weapons.EXPECT().Get(mock.Anything, charID, "longsword").Return(longsword, nil).Once()
		f.withChar(bard())
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(strMods(12), nil).Once()
		f.dice.EXPECT().RollD20().Return(15).Once()
		f.d100s(t, 3, 64)
		f.dice.EXPECT().RollDie(8).Return(5).Once()
		f.records()
		res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: ac})
		require.NoError(t, err)
		if ac == nil {
			assert.False(t, res.ACKnown)
			assert.False(t, res.Settled())
			assert.False(t, res.Hit)
			assert.False(t, res.IsCrit)
			assert.Equal(t, 7, res.TotalDamage, "the damage if it hits, x1")
		} else {
			// AC 0 needs 11: 15 hits, 15 - 3 = 12 crits (x2 placeholder).
			assert.True(t, res.ACKnown)
			assert.True(t, res.Settled())
			assert.True(t, res.Hit)
			assert.True(t, res.IsCrit)
			assert.Equal(t, 14, res.TotalDamage)
		}
	}
}

// Step 2: hitsAc = effective THAC0 - d20, the lowest AC the roll hits; the
// AC it names hits and one lower misses (ADR 0002 step 2).
func TestUnknownAC_Step2_HitsACIsEffectiveThac0MinusD20(t *testing.T) {
	f := newFixture(t)
	// 11 - 13 = -2 (with the check d100 42 or the multiplier 87 instead of
	// the d20 it would be -31 or -76).
	e := rollAttack(t, f, unknownSword(13, 42, 87))
	assert.Equal(t, -2.0, payloadOf(t, f.entries[0].Payload)["hitsAc"])
	assert.Equal(t, -2.0, payloadOf(t, f.entries[0].Payload)["targetACNeeded"], "the same number as before")
	amends(f, e)

	res, err := f.svc.AdjustAttackAC(context.Background(), charID, 42, -2)
	require.NoError(t, err)
	assert.Equal(t, -2, res.HitsAC)
	assert.True(t, res.Hit, "AC hitsAc is hit")
	res, err = f.svc.AdjustAttackAC(context.Background(), charID, 42, -3)
	require.NoError(t, err)
	assert.False(t, res.Hit, "one AC lower is missed")
	assert.Equal(t, -2, res.HitsAC, "hitsAc does not depend on the AC")
}

// Step 3: critsFromAc = effective THAC0 - d20 + crit-check d100, the lowest
// AC the roll crits (ADR 0001 step 2 solved for the AC); the AC it names
// crits and one lower does not.
func TestUnknownAC_Step3_CritsFromACAddsTheCheckD100(t *testing.T) {
	f := newFixture(t)
	// 11 - 13 + 4 = 2 (with the multiplier d100 87 it would be 85).
	e := rollAttack(t, f, unknownSword(13, 4, 87))
	assert.Equal(t, 2.0, payloadOf(t, f.entries[0].Payload)["critsFromAc"])
	amends(f, e)

	res, err := f.svc.AdjustAttackAC(context.Background(), charID, 42, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, res.CritsFromAC)
	assert.True(t, res.IsCrit, "AC critsFromAc crits")
	// Placeholder table x2: ((5 × 1) + 2) × 2 = 14.
	assert.Equal(t, 14, res.TotalDamage)
	res, err = f.svc.AdjustAttackAC(context.Background(), charID, 42, 1)
	require.NoError(t, err)
	assert.False(t, res.IsCrit, "one AC lower is a plain hit")
	assert.True(t, res.Hit)
	assert.Equal(t, 7, res.TotalDamage)
}

// Step 4: a GM verdict without the AC settles hit or miss with no crit
// (x1), even when the numbers would crit any AC; a later verdict replaces
// it; nothing is rolled (the mocks expect each die once).
func TestUnknownAC_Step4_VerdictSettlesWithoutCrit(t *testing.T) {
	f := newFixture(t)
	// 11 - 20 + 1 = -8: any AC from -8 up would be a crit.
	e := rollAttack(t, f, unknownSword(20, 1, 64))
	amends(f, e)

	res, err := f.svc.SetAttackVerdict(context.Background(), charID, 42, true)
	require.NoError(t, err)
	assert.Equal(t, combatsvc.VerdictHit, res.Verdict)
	assert.True(t, res.Hit)
	assert.False(t, res.IsCrit, "no crit without the AC")
	assert.Equal(t, 1.0, res.CritMultiplier)
	assert.Zero(t, res.HitMargin)
	assert.Equal(t, 7, res.TotalDamage, "((5 × 1) + 2) × 1")
	assert.False(t, res.ACKnown)
	p := payloadOf(t, e.Corrections[0].Payload)
	assert.Equal(t, "hit", p["verdict"])
	assert.Nil(t, p["targetAc"])

	res, err = f.svc.SetAttackVerdict(context.Background(), charID, 42, false)
	require.NoError(t, err)
	assert.Equal(t, combatsvc.VerdictMiss, res.Verdict)
	assert.False(t, res.Hit)
	assert.Zero(t, res.TotalDamage, "a miss has no damage")
	assert.Empty(t, res.DamageTerms)
	assert.Equal(t, 20, res.D20Roll, "same d20")
}

// Step 4: a verdict on an attack whose AC is known is refused (the AC is
// corrected instead), and nothing is amended.
func TestUnknownAC_Step4_VerdictRefusedWhenACKnown(t *testing.T) {
	f := newFixture(t)
	e := rollAttack(t, f, sword(5, 15, 3))
	amends(f, e)
	res, err := f.svc.SetAttackVerdict(context.Background(), charID, 42, false)
	assert.Nil(t, res)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Empty(t, e.Corrections)
}

// Step 5: giving the AC runs ADR 0001 step 6 with the stored dice and
// replaces any verdict: here the verdict said hit x1, the AC makes it a x3
// crit from the stored multiplier d100.
func TestUnknownAC_Step5_GivenACReplacesTheVerdict(t *testing.T) {
	f := newFixture(t)
	// Multiplier d100 4 is in 1-4: x3; the check d100 10 would be x1.5.
	f.critRows = []combatsvc.CritRow{{From: 1, To: 4, Multiplier: 3}, {From: 5, To: 100, Multiplier: 1.5}}
	e := rollAttack(t, f, unknownSword(15, 10, 4))
	// Editing the table after the roll changes nothing (row stored at the roll).
	f.critRows = []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 5}}
	amends(f, e)

	res, err := f.svc.SetAttackVerdict(context.Background(), charID, 42, true)
	require.NoError(t, err)
	assert.Equal(t, 7, res.TotalDamage)

	// AC 6 needs 5: 15 hits, 15 - 10 = 5 >= 5 crits: ((5 × 1) + 2) × 3 = 21.
	res, err = f.svc.AdjustAttackAC(context.Background(), charID, 42, 6)
	require.NoError(t, err)
	assert.True(t, res.ACKnown)
	assert.Equal(t, combatsvc.VerdictNone, res.Verdict, "the AC replaces the verdict")
	assert.True(t, res.Hit)
	assert.True(t, res.IsCrit)
	assert.Equal(t, 3.0, res.CritMultiplier)
	assert.Equal(t, 21, res.TotalDamage)
	assert.Equal(t, 10, res.HitMargin)
	assert.Nil(t, res.RolledTargetAC, "no AC as rolled to keep")
	assert.True(t, res.RolledACUnknown)
	p := payloadOf(t, e.Corrections[len(e.Corrections)-1].Payload)
	assert.Nil(t, p["verdict"])
	assert.Equal(t, 6.0, p["targetAc"])
	assert.Equal(t, true, p["acKnown"])

	// Once the AC is known a verdict is refused; the AC can still change.
	_, err = f.svc.SetAttackVerdict(context.Background(), charID, 42, false)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	res, err = f.svc.AdjustAttackAC(context.Background(), charID, 42, -10)
	require.NoError(t, err)
	assert.False(t, res.Hit, "AC -10 needs 21")
	assert.Nil(t, res.RolledTargetAC)
}

// Step 5: a miss verdict, then the AC that the roll hits: a hit.
func TestUnknownAC_Step5_GivenACOverridesAMissVerdict(t *testing.T) {
	f := newFixture(t)
	e := rollAttack(t, f, unknownSword(12, 40, 64))
	amends(f, e)
	_, err := f.svc.SetAttackVerdict(context.Background(), charID, 42, false)
	require.NoError(t, err)
	// AC 5 needs 6: 12 hits, 12 - 40 < 6 no crit.
	res, err := f.svc.AdjustAttackAC(context.Background(), charID, 42, 5)
	require.NoError(t, err)
	assert.True(t, res.Hit)
	assert.False(t, res.IsCrit)
	assert.Equal(t, 7, res.TotalDamage)
}

// Step 6: an attack against an unknown AC rolls exactly the dice of one
// against a known AC, in the same order: d20, crit-check d100, multiplier
// d100, then the damage dice once.
func TestUnknownAC_Step6_SameDiceAsKnownAC(t *testing.T) {
	cases := []struct {
		name string
		ac   *int
	}{
		{"unknown AC", nil},
		{"known AC", new(5)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			dice := &seqDice{t: t, want: []string{"d20", "d100", "d100", "d4", "d4"}, vals: []int{14, 61, 37, 3, 2}}
			svc, err := combatsvc.New(combatsvc.Deps{
				Chars: f.chars, Buffs: f.buffs, Dice: dice, Weapons: f.weapons, Crit: f.crit,
				Items: f.items, Forms: f.forms, Rules: phb(t), Log: f.log, Tx: noTx{},
			})
			require.NoError(t, err)
			f.weapons.EXPECT().Get(mock.Anything, charID, "bastard_sword").Return(bastardSword, nil).Once()
			f.withChar(bard())
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(strMods(12), nil).Once()
			f.records()

			res, err := svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "bastard_sword", TargetAC: c.ac})
			require.NoError(t, err)
			assert.Equal(t, []string{"d20", "d100", "d100", "d4", "d4"}, dice.calls, "every die once, in order")
			assert.Equal(t, 14, res.D20Roll)
			assert.Equal(t, 61, res.D100CritRoll)
			assert.Equal(t, 37, res.D100MultiplierRoll)
			require.NotNil(t, res.Rolled)
			assert.Equal(t, 5, res.Rolled.Total)
			// 11 - 14 = -3 and -3 + 61 = 58, whatever the AC.
			assert.Equal(t, -3, res.HitsAC)
			assert.Equal(t, 58, res.CritsFromAC)
		})
	}
}

// An attack logged before unknown ACs existed has no acKnown: it reads as
// known (ADR 0012 consequences), so a verdict is refused and a correction
// keeps its AC as rolled.
func TestUnknownAC_EntryWithoutACKnownReadsAsKnown(t *testing.T) {
	f := newFixture(t)
	e := rollAttack(t, f, sword(5, 15, 30)) // AC 5 needs 6: a hit, 15 - 30 < 6 no crit
	var p map[string]any
	require.NoError(t, json.Unmarshal(e.Payload, &p))
	delete(p, "acKnown")
	delete(p, "hitsAc")
	delete(p, "critsFromAc")
	var err error
	e.Payload, err = json.Marshal(p)
	require.NoError(t, err)
	amends(f, e)

	res, err := f.svc.SetAttackVerdict(context.Background(), charID, 42, false)
	assert.Nil(t, res)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Empty(t, e.Corrections)

	res, err = f.svc.AdjustAttackAC(context.Background(), charID, 42, -10)
	require.NoError(t, err)
	assert.True(t, res.ACKnown)
	assert.False(t, res.Hit, "AC -10 needs 21")
	require.NotNil(t, res.RolledTargetAC, "the AC as rolled is kept")
	assert.Equal(t, 5, *res.RolledTargetAC)
	require.NotNil(t, res.RolledHit)
	assert.True(t, *res.RolledHit)
}

// Step 7: an attack can crit a real target only when critsFromAc is at most
// the highest AC the rules allow (10).
func TestUnknownAC_Step7_CritPossibleOnlyWithinRealACs(t *testing.T) {
	cases := []struct {
		name       string
		d20, check int
		crits      int
		possible   bool
	}{
		{"exactly the highest AC", 15, 14, 11 - 15 + 14, true},
		{"one beyond it", 14, 14, 11 - 14 + 14, false},
		{"far beyond", 2, 59, 11 - 2 + 59, false},
		{"negative", 20, 3, 11 - 20 + 3, true},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			e := rollAttack(t, f, unknownSword(c.d20, c.check, 50+i))
			amends(f, e)
			res, err := f.svc.SetAttackVerdict(context.Background(), charID, 42, true)
			require.NoError(t, err)
			assert.Equal(t, c.crits, res.CritsFromAC)
			assert.Equal(t, c.possible, res.CritPossible())
		})
	}
}

// Step 8: after the AC was given (or a verdict), the latest attack can go
// back to unknown: not settled again, the dice kept, nothing rolled.
func TestUnknownAC_Step8_BackToUnknownAfterTheACWasGiven(t *testing.T) {
	f := newFixture(t)
	e := rollAttack(t, f, unknownSword(15, 10, 64))
	amends(f, e)

	// Already unknown: refused.
	_, err := f.svc.ClearAttackAC(context.Background(), charID, 42)
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = f.svc.AdjustAttackAC(context.Background(), charID, 42, 6)
	require.NoError(t, err)
	res, err := f.svc.ClearAttackAC(context.Background(), charID, 42)
	require.NoError(t, err)
	assert.False(t, res.ACKnown)
	assert.False(t, res.Settled())
	assert.False(t, res.Hit)
	assert.False(t, res.IsCrit)
	assert.Equal(t, 7, res.TotalDamage, "the damage of a hit at x1")
	assert.Equal(t, -4, res.HitsAC)
	assert.True(t, res.RolledACUnknown)
	last := e.Corrections[len(e.Corrections)-1]
	p := payloadOf(t, last.Payload)
	assert.Nil(t, p["targetAc"])
	assert.Equal(t, false, p["acKnown"])

	// The verdict is available again, and the AC can be given again.
	_, err = f.svc.SetAttackVerdict(context.Background(), charID, 42, true)
	require.NoError(t, err)
	res, err = f.svc.AdjustAttackAC(context.Background(), charID, 42, 3)
	require.NoError(t, err)
	assert.True(t, res.ACKnown)
}

// Step 8: an attack rolled against a known AC can go unknown too; the AC
// and outcome as rolled stay for the "was" line.
func TestUnknownAC_Step8_KnownAtRollGoesUnknown(t *testing.T) {
	f := newFixture(t)
	r := unknownSword(15, 10, 64)
	r.req.TargetAC = new(4)
	e := rollAttack(t, f, r)
	amends(f, e)

	res, err := f.svc.ClearAttackAC(context.Background(), charID, 42)
	require.NoError(t, err)
	assert.False(t, res.ACKnown)
	require.NotNil(t, res.RolledTargetAC)
	assert.Equal(t, 4, *res.RolledTargetAC)
	assert.False(t, res.RolledACUnknown)
}
