package combatsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// rolledAttack is an attack to roll before correcting its target AC.
type rolledAttack struct {
	weapon combatsvc.Weapon
	ammo   *itemsvc.Item
	mods   buffsvc.BuffModifiers
	req    combatsvc.AttackRequest
	d20    int
	d100   int // crit check
	mult   int // multiplier d100 (0: noCritMult)
	sides  int
	dmg    []int
}

// rollAttack rolls a and returns the entry the log recorded, as the log
// would return it (id 42, no corrections). Every die and the ammunition are
// expected exactly once: a correction that rolled or spent again would fail
// the mocks.
func rollAttack(t *testing.T, f *fixture, a rolledAttack) *actionlog.Entry {
	t.Helper()
	f.weapons.EXPECT().Get(mock.Anything, a.req.WeaponID).Return(a.weapon, nil).Once()
	f.withChar(bard())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12, mock.Anything).Return(a.mods, nil).Once()
	if a.ammo != nil {
		f.items.EXPECT().Use(mock.Anything, a.req.AmmoItemID).Return(*a.ammo, nil).Once()
	}
	f.dice.EXPECT().RollD20().Return(a.d20).Once()
	mult := a.mult
	if mult == 0 {
		mult = noCritMult
	}
	f.d100s(t, a.d100, mult)
	for _, d := range a.dmg {
		f.dice.EXPECT().RollDie(a.sides).Return(d).Once()
	}
	f.records()
	_, err := f.svc.ResolveAttack(context.Background(), charID, a.req)
	require.NoError(t, err)
	require.Len(t, f.entries, 1)
	n := f.entries[0]
	return &actionlog.Entry{ID: 42, CharacterID: charID, Kind: n.Kind, Summary: n.Summary, Payload: n.Payload, Snapshots: n.Snapshots}
}

// swordMinus7 is the long sword at STR 12 with a -7 situational: effective
// THAC0 13 - (2 - 7) = 18, so AC 5 needs 13 and AC 7 needs 11.
func swordMinus7(d20, d100 int) rolledAttack {
	return rolledAttack{
		weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{
			WeaponID: "longsword", TargetAC: new(5),
			Situational: &combatsvc.Situational{Value: -7, Label: "Blinded"},
		},
		d20: d20, d100: d100, sides: 8, dmg: []int{5},
	}
}

// sword is the long sword at STR 12: effective THAC0 11, AC 5 needs 6.
func sword(ac, d20, d100 int) rolledAttack {
	return rolledAttack{
		weapon: longsword, mods: strMods(12),
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(ac)},
		d20: d20, d100: d100, sides: 8, dmg: []int{5},
	}
}

// withMult sets the multiplier d100 of a.
func withMult(a rolledAttack, mult int) rolledAttack {
	a.mult = mult
	return a
}

func TestAdjustAttackAC_Recompute(t *testing.T) {
	// Crit rows: a multiplier d100 of 1-10 gives x3, 11-100 x1.5.
	critRows := []combatsvc.CritRow{{From: 1, To: 10, Multiplier: 3}, {From: 11, To: 100, Multiplier: 1.5}}
	cases := []struct {
		name     string
		attack   rolledAttack
		critRows []combatsvc.CritRow
		acs      []int // corrections in order

		wantHit       bool
		wantMargin    int
		wantCrit      bool
		wantMult      float64
		wantTotal     int
		wantBreakdown float64
		wantTerms     int
		// wantRolled is the outcome as rolled (MISS, HIT, CRIT), kept while
		// a correction differs from the rolled AC.
		wantRolled string
	}{
		{
			// Rolled: d20 11 vs AC 5 needs 13, a miss. AC 7 needs 11: a hit
			// with the 1d8 = 5 rolled with the attack: ((5 × 1) + 2) × 1 + 0 = 7.
			name: "miss to hit shows the damage rolled with the attack", attack: swordMinus7(11, 50), acs: []int{7},
			wantHit: true, wantMargin: 0, wantMult: 1, wantTotal: 7, wantTerms: 2,
			wantBreakdown: 7,
			wantRolled:    "MISS",
		},
		{
			// Rolled: d20 13 vs AC 5 needs 13, a hit. AC 3 needs 15: a miss, no damage.
			name: "hit to miss hides the damage", attack: swordMinus7(13, 50), acs: []int{3},
			wantHit: false, wantMargin: -2, wantMult: 1, wantTotal: 0, wantTerms: 0,
			wantRolled: "HIT",
		},
		{
			// Rolled: d20 15, d100 10 vs AC 5 (needs 6): 15 - 10 = 5 < 6, a
			// plain hit. AC 6 needs 5: 5 >= 5, a crit; the multiplier d100 4 is
			// in 1-10, x3 (the check d100 10 is not looked up):
			// ((5 × 1) + 2) × 3 = 21.
			name: "a crit appears from the stored d100", attack: withMult(sword(5, 15, 10), 4), critRows: critRows, acs: []int{6},
			wantHit: true, wantMargin: 10, wantCrit: true, wantMult: 3, wantTotal: 21, wantTerms: 3,
			wantBreakdown: 21,
			wantRolled:    "HIT",
		},
		{
			// Rolled: d20 15, d100 10 vs AC 6 (needs 5): a x3 crit. AC 5 needs
			// 6: 15 - 10 = 5 < 6, a plain hit: 7.
			name: "a crit disappears", attack: withMult(sword(6, 15, 10), 4), critRows: critRows, acs: []int{5},
			wantHit: true, wantMargin: 9, wantMult: 1, wantTotal: 7, wantTerms: 2,
			wantBreakdown: 7,
			wantRolled:    "CRIT",
		},
		{
			// Rolled: d20 15 vs AC -10 needs 21, a miss. AC 10 needs 1:
			// 15 - 12 = 3 >= 1, a crit; the multiplier d100 77 is in 11-100, x1.5:
			// ((5 × 1) + 2) × 1.5 = 10.5, floored once: 10.
			name: "miss to decimal crit floors once", attack: sword(-10, 15, 12), critRows: critRows, acs: []int{10},
			wantHit: true, wantMargin: 14, wantCrit: true, wantMult: 1.5, wantTotal: 10, wantTerms: 3,
			wantBreakdown: 10.5,
			wantRolled:    "MISS",
		},
		{
			// A natural 20 is plain arithmetic: vs AC -10 (needs 21) it misses,
			// vs AC -9 (needs 20) it hits.
			name: "natural 20 stays plain arithmetic", attack: sword(-10, 20, 50), acs: []int{-9},
			wantHit: true, wantMargin: 0, wantMult: 1, wantTotal: 7, wantTerms: 2,
			wantBreakdown: 7,
			wantRolled:    "MISS",
		},
		{
			// A natural 1 is plain arithmetic: vs AC 10 (needs 1) it hits.
			name: "natural 1 stays plain arithmetic", attack: sword(5, 1, 50), acs: []int{10},
			wantHit: true, wantMargin: 0, wantMult: 1, wantTotal: 7, wantTerms: 2,
			wantBreakdown: 7,
			wantRolled:    "MISS",
		},
		{
			// AC 5 → 7 → 9: the line reads from the rolled AC and outcome.
			// AC 9 needs 9: 11 hits by 2.
			name: "a second correction keeps the rolled AC and outcome", attack: swordMinus7(11, 50), acs: []int{7, 9},
			wantHit: true, wantMargin: 2, wantMult: 1, wantTotal: 7, wantTerms: 2,
			wantBreakdown: 7,
			wantRolled:    "MISS",
		},
		{
			// AC 5 → 7 → 5: back to the rolled AC reads as rolled (still amended).
			name: "back to the rolled AC reads as rolled", attack: swordMinus7(11, 50), acs: []int{7, 5},
			wantHit: false, wantMargin: -2, wantMult: 1, wantTotal: 0, wantTerms: 0,
			wantRolled: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.critRows = c.critRows
			e := rollAttack(t, f, c.attack)
			// The crit table edited after the roll changes nothing: the
			// correction uses the row the d100 fell in when rolling.
			f.critRows = []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 5}}
			amends(f, e)

			var res *combatsvc.RollResult
			for _, ac := range c.acs {
				var err error
				res, err = f.svc.AdjustAttackAC(context.Background(), charID, 42, ac)
				require.NoError(t, err)
			}
			last := c.acs[len(c.acs)-1]
			assert.Equal(t, c.attack.d20, res.D20Roll, "same d20")
			assert.Equal(t, c.attack.d100, res.D100CritRoll, "same d100")
			assert.Equal(t, last, res.TargetAC)
			assert.Equal(t, c.wantHit, res.Hit)
			assert.Equal(t, c.wantMargin, res.HitMargin)
			assert.Equal(t, c.wantCrit, res.IsCrit)
			assert.InDelta(t, c.wantMult, res.CritMultiplier, 0)
			assert.Equal(t, c.wantTotal, res.TotalDamage)
			assert.InDelta(t, c.wantBreakdown, res.UnroundedDamage, 1e-9)
			assert.Len(t, res.DamageTerms, c.wantTerms)
			if c.wantHit {
				assert.Equal(t, 5, res.BaseDamage, "damage dice rolled with the attack")
				assert.Equal(t, "1d8", res.DamageDice)
			} else {
				assert.Zero(t, res.BaseDamage)
			}
			backToRolled := last == *c.attack.req.TargetAC
			if backToRolled {
				assert.Nil(t, res.RolledTargetAC, "back to the rolled AC: nothing to show")
			} else {
				require.NotNil(t, res.RolledTargetAC)
				assert.Equal(t, *c.attack.req.TargetAC, *res.RolledTargetAC, "the AC as rolled, not the previous step")
			}
			assert.Equal(t, int64(42), res.LogEntryID)

			require.Len(t, e.Corrections, len(c.acs), "one audited amend per correction")
			p := payloadOf(t, e.EffectivePayload())
			assert.Equal(t, float64(last), p["targetAc"])
			assert.Equal(t, c.wantHit, p["hit"])
			assert.Equal(t, float64(c.wantTotal), p["totalDamage"])
			if backToRolled {
				assert.NotContains(t, p, "rolledTargetAc")
			} else {
				assert.Equal(t, float64(*c.attack.req.TargetAC), p["rolledTargetAc"])
				assert.Equal(t, c.wantRolled != "MISS", p["rolledHit"])
				assert.Equal(t, c.wantRolled == "CRIT", p["rolledCrit"])
			}
			assert.Contains(t, p, "rolled", "the next correction can recompute again")
			assert.Equal(t, float64(*c.attack.req.TargetAC), payloadOf(t, e.Payload)["targetAc"], "the original entry is untouched")
		})
	}
}

// A bow attack corrected after the roll spends no second arrow (the items
// mock allows one Use) and keeps the arrow count the attack left.
func TestAdjustAttackAC_SpendsNoAmmo(t *testing.T) {
	f := newFixture(t)
	// Short bow +2 with a +3 arrow at DEX 17: effective THAC0 6, AC 2 needs 4.
	e := rollAttack(t, f, rolledAttack{
		weapon: shortbow, ammo: &arrows3Left1, mods: strMods(12),
		req: combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus3", TargetAC: new(2)},
		d20: 3, d100: 50, sides: 6, dmg: []int{4},
	})
	amends(f, e)
	res, err := f.svc.AdjustAttackAC(context.Background(), charID, 42, 3)
	require.NoError(t, err)
	assert.True(t, res.Hit)
	// ((4 × 1) + (2 + 3)) × 1 + 0 = 9.
	assert.Equal(t, 9, res.TotalDamage)
	assert.Equal(t, &combatsvc.AmmoUse{ItemID: "arrows_plus3", Name: "Arrows +3", Magic: 3, Remaining: 1}, res.Ammo)
	require.NotNil(t, res.RolledHit)
	assert.False(t, *res.RolledHit, "rolled as a miss vs AC 2")
	require.Len(t, e.Snapshots, 1, "the amend keeps the attack's items snapshot for void")
}

// A miss stores its damage dice and terms, so the correction lists every
// damage term in formula order with the crit where it belongs.
func TestAdjustAttackAC_DamageTermsInFormulaOrder(t *testing.T) {
	f := newFixture(t)
	e := rollAttack(t, f, rolledAttack{
		weapon: longsword,
		mods: buffsvc.BuffModifiers{
			SpellsBuff: 1.5, Multipliers: 2, EffectiveStr: 18, Damage: 1,
			SpellsBuffFrom:  []buffsvc.Factor{{Buff: "Enlarge", Value: 1.5}},
			MultipliersFrom: []buffsvc.Factor{{Buff: "Haste blade", Value: 2}},
			DamageFrom:      []buffsvc.Contribution{{Buff: "Rage", Value: 1}},
		},
		// STR 18 +1, sword +2: effective THAC0 10; AC -10 needs 20, a miss.
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(-10)}, d20: 15, d100: 5, sides: 8, dmg: []int{5},
	})
	amends(f, e)
	// AC 5 needs 5: 15 - 5 = 10 >= 5, a crit (placeholder x2).
	res, err := f.svc.AdjustAttackAC(context.Background(), charID, 42, 5)
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.DamageTerm{
		{Term: combatsvc.TermSpells, Kind: combatsvc.ModBuff, Detail: "Enlarge", Value: 1.5},
		{Term: combatsvc.TermDiceMult, Kind: combatsvc.ModBuff, Detail: "Haste blade", Value: 2},
		{Term: combatsvc.TermWeapon, Kind: combatsvc.ModWeaponMagic, Detail: "Long Sword +2", Value: 2},
		{Term: combatsvc.TermMultiplier, Kind: combatsvc.ModCrit, Value: 2},
		{Term: combatsvc.TermStrength, Kind: combatsvc.ModStrength, Detail: "18", Value: 2},
		{Term: combatsvc.TermBonus, Kind: combatsvc.ModBuff, Detail: "Rage", Value: 1},
	}, res.DamageTerms)
	// Same as rolling a crit directly: floor(((5 × 1.5 × 2 buff) + 2) × 2 crit) + 2 + 1 = 37.
	assert.Equal(t, 37, res.TotalDamage)
	assert.True(t, res.CritPlaceholder)
}

func TestAdjustAttackAC_Rejections(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name    string
		ac      int
		logErr  error // returned by AmendLatest; nil: not reached
		wantErr error
	}{
		{"above 20", 21, nil, domain.ErrInvalidInput},
		{"below -20", -21, nil, domain.ErrInvalidInput},
		{"not the latest", 7, actionlogsvc.ErrNotLatest, actionlogsvc.ErrNotLatest},
		{"voided", 7, actionlogsvc.ErrAlreadyVoided, actionlogsvc.ErrAlreadyVoided},
		{"not an attack", 7, actionlogsvc.ErrNotAmendable, actionlogsvc.ErrNotAmendable},
		{"storage", 7, boom, boom},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			if c.logErr != nil {
				f.log.EXPECT().AmendLatest(mock.Anything, charID, int64(9), actionlog.KindAttack, mock.Anything).Return(nil, c.logErr)
			}
			res, err := f.svc.AdjustAttackAC(context.Background(), charID, 9, c.ac)
			assert.Nil(t, res)
			assert.ErrorIs(t, err, c.wantErr)
		})
	}

	t.Run("attack logged without its damage dice", func(t *testing.T) {
		f := newFixture(t)
		// An attack entry from before corrections: a miss without rolled dice.
		e := &actionlog.Entry{ID: 9, Kind: actionlog.KindAttack, Summary: "x", Payload: json.RawMessage(
			`{"d20Roll":3,"d100CritRoll":50,"hit":false,"effectiveThac0":11,"targetAc":5,"weaponName":"Long Sword +2","modifiers":[],"damageTerms":[]}`)}
		amends(f, e)
		res, err := f.svc.AdjustAttackAC(context.Background(), charID, 9, 7)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
		assert.ErrorContains(t, err, "undo it and roll again")
		assert.Empty(t, e.Corrections)
	})

	t.Run("attack logged before the multiplier d100", func(t *testing.T) {
		f := newFixture(t)
		// Rolled with damage dice and a crit row but one d100 (before ADR
		// 0001): its crit can't be re-evaluated with two d100s.
		e := &actionlog.Entry{ID: 9, Kind: actionlog.KindAttack, Summary: "x", Payload: json.RawMessage(
			`{"d20Roll":15,"d100CritRoll":4,"hit":false,"effectiveThac0":11,"targetAc":-10,"weaponName":"Long Sword +2",` +
				`"modifiers":[],"damageTerms":[],` +
				`"rolled":{"dice":"1d8","total":5,"spellsBuff":1,"weaponBuff":2,"buffMultipliers":1,"strengthBonus":0,"bonusDamage":0,"terms":[]},` +
				`"critRow":{"from":1,"to":100,"multiplier":2,"placeholder":true}}`)}
		amends(f, e)
		res, err := f.svc.AdjustAttackAC(context.Background(), charID, 9, 5)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
		assert.ErrorContains(t, err, "undo it and roll again")
		assert.Empty(t, e.Corrections)
	})

	t.Run("undecodable payload", func(t *testing.T) {
		f := newFixture(t)
		e := &actionlog.Entry{ID: 9, Kind: actionlog.KindAttack, Summary: "x", Payload: json.RawMessage(`[]`)}
		amends(f, e)
		res, err := f.svc.AdjustAttackAC(context.Background(), charID, 9, 7)
		assert.Nil(t, res)
		assert.ErrorContains(t, err, "decode attack #9")
		assert.Empty(t, e.Corrections)
	})
}

// An attack logged before the dice-only gem multiplier has no top-level
// diceMultiplier and an old multipliers value (crit × buff): correcting its
// AC does not fail and re-settles with the new formula from the stored rolled
// damage (BuffMultipliers).
func TestAdjustAttackAC_PreChangeEntryResettles(t *testing.T) {
	f := newFixture(t)
	m := strMods(12)
	m.Multipliers = 2
	m.MultipliersFrom = []buffsvc.Factor{{Buff: "Power Gem", Value: 2}}
	e := rollAttack(t, f, rolledAttack{
		weapon: longsword, mods: m,
		req: combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(-10)}, // a miss
		d20: 15, d100: 50, sides: 8, dmg: []int{5},
	})
	var p map[string]any
	require.NoError(t, json.Unmarshal(e.Payload, &p))
	delete(p, "diceMultiplier")
	p["multipliers"] = 2.0
	var err error
	e.Payload, err = json.Marshal(p)
	require.NoError(t, err)
	amends(f, e)

	res, err := f.svc.AdjustAttackAC(context.Background(), charID, 42, 7)
	require.NoError(t, err)
	assert.True(t, res.Hit)
	assert.Equal(t, 2.0, res.DiceMultiplier)
	// ((5 × 1 × 2) + 2) × 1 = 12.
	assert.Equal(t, 12, res.TotalDamage)
}
