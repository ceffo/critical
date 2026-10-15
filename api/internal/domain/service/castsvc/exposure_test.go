package castsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/pkg/formula"
)

var cloudkill = slot("cloudkill", "Cloudkill", 5)

func once(count, sides int) formula.Sum {
	return formula.Sum{Dice: []formula.Dice{{Count: count, Sides: sides}}}
}

func exposureDamage(level int, d formula.Sum, typ string) *spelleffectsvc.DamageAt {
	return &spelleffectsvc.DamageAt{
		CasterLevel: level, Dice: d, Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1}, Type: typ,
	}
}

// ADR 0008 step 16: the exposure data of the seeds at caster levels 9 and
// 15, per the PHB. Cloudkill: 1d10 poison each round. Wall of Fire: 2d4
// within 10 ft, 1d4 within 20 ft, 2d6 + 1 per level passing through, fire.
// Death Fog: 1, 2, 4, then 8 acid by round of exposure.
func TestExposureAt_MatchesThePHB(t *testing.T) {
	f := newFixture(t)
	for _, level := range []int{9, 15} {
		got, err := f.svc.ExposureAt(context.Background(), "cloudkill", level)
		require.NoError(t, err)
		assert.Equal(t, &spelleffectsvc.ExposureAt{Damage: exposureDamage(level, once(1, 10), "poison")}, got)

		got, err = f.svc.ExposureAt(context.Background(), "wall_of_fire", level)
		require.NoError(t, err)
		through := once(2, 6)
		through.Constant = level
		assert.Equal(t, &spelleffectsvc.ExposureAt{Modes: []spelleffectsvc.ModeAt{
			{Key: "within_10_ft", Damage: exposureDamage(level, once(2, 4), "fire")},
			{Key: "within_20_ft", Damage: exposureDamage(level, once(1, 4), "fire")},
			{Key: "passing_through", Damage: exposureDamage(level, through, "fire")},
		}}, got)

		got, err = f.svc.ExposureAt(context.Background(), "death_fog", level)
		require.NoError(t, err)
		fog := exposureDamage(level, formula.Sum{}, "acid")
		fog.Rounds, fog.StepsPer = formula.Sum{Constant: 4}, spelleffectsvc.StepPerRound
		fog.Steps = []spelleffectsvc.DamageStepAt{
			{Step: 1, Dice: formula.Sum{Constant: 1}},
			{Step: 2, Dice: formula.Sum{Constant: 2}},
			{Step: 3, Dice: formula.Sum{Constant: 4}},
			{Step: 4, Dice: formula.Sum{Constant: 8}},
		}
		assert.Equal(t, &spelleffectsvc.ExposureAt{Damage: fog}, got)
	}
	for _, id := range []string{"fireball", "tashas_uncontrollable_hideous_laughter", "no_such_spell"} {
		got, err := f.svc.ExposureAt(context.Background(), id, bardLevel)
		require.NoError(t, err)
		assert.Nil(t, got, "%s has no exposure", id)
	}
	_, err := f.svc.ExposureAt(context.Background(), "cloudkill", castsvc.MaxCasterLevel+1)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

// ADR 0008 step 16: an exposure rolls exactly the dice served for its round,
// once (every queued die is a distinct value and must be thrown, no other),
// and is one exposure entry that records the roll and changes no HP when
// the Bard is not the one exposed.
func TestExpose_RollsTheServedDiceOnce(t *testing.T) {
	tests := []struct {
		name  string
		req   castsvc.ExposureRequest
		dice  []int
		sides []int
		mode  string
		round int
		step  int // the dice step rolled (0: no steps)
		total int
	}{
		{"cloudkill 1d10", castsvc.ExposureRequest{SpellID: "cloudkill", CasterLevel: 15}, []int{7}, []int{10}, "", 1, 0, 7},
		{
			"wall of fire, the first mode by default",
			castsvc.ExposureRequest{SpellID: "wall_of_fire", CasterLevel: 12},
			[]int{2, 3},
			[]int{4, 4},
			"within_10_ft", 1, 0, 5,
		},
		{
			"wall of fire passing through: 2d6 + level",
			castsvc.ExposureRequest{SpellID: "wall_of_fire", CasterLevel: 12, Mode: "passing_through", Round: 3},
			[]int{4, 5},
			[]int{6, 6},
			"passing_through", 3, 0, 4 + 5 + 12,
		},
		{"death fog round 1", castsvc.ExposureRequest{SpellID: "death_fog", CasterLevel: 15, Round: 1}, nil, nil, "", 1, 1, 1},
		{"death fog round 3", castsvc.ExposureRequest{SpellID: "death_fog", CasterLevel: 15, Round: 3}, nil, nil, "", 3, 3, 4},
		{"death fog round 7: the last step", castsvc.ExposureRequest{SpellID: "death_fog", CasterLevel: 15, Round: 7}, nil, nil, "", 7, 4, 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.dice...)
			res, err := f.svc.Expose(context.Background(), charID, tt.req)
			require.NoError(t, err)
			assert.Empty(t, f.dice.vals, "every queued die is rolled")
			assert.Equal(t, tt.sides, f.dice.sides, "exactly the served dice")
			assert.Equal(t, tt.mode, res.Mode)
			assert.Equal(t, tt.round, res.Round)
			require.NotNil(t, res.Damage)
			assert.Equal(t, tt.total, res.Damage.Total)
			require.Len(t, res.Damage.Rolls, 1, "one round, one roll")
			assert.Equal(t, tt.step, res.Damage.Rolls[0].Step)

			require.Len(t, f.entries, 1, "one entry")
			e := f.entries[0]
			assert.Equal(t, actionlog.KindExposure, e.Kind)
			assert.Zero(t, e.HPDelta, "not the Bard: the DM's roll")
			assert.Empty(t, e.Snapshots)
			assert.Equal(t, int64(1), res.EntryID)
			var p struct {
				SpellID     string `json:"spellId"`
				CasterLevel int    `json:"casterLevel"`
				Mode        string `json:"mode"`
				Round       int    `json:"round"`
				OnSelf      bool   `json:"onSelf"`
				Damage      struct {
					Total int `json:"total"`
					Rolls []struct {
						Total int `json:"total"`
						Step  int `json:"step"`
					} `json:"rolls"`
				} `json:"damage"`
			}
			require.NoError(t, json.Unmarshal(e.Payload, &p))
			assert.Equal(t, tt.req.SpellID, p.SpellID)
			assert.Equal(t, tt.req.CasterLevel, p.CasterLevel)
			assert.Equal(t, tt.mode, p.Mode)
			assert.Equal(t, tt.round, p.Round)
			assert.False(t, p.OnSelf)
			assert.Equal(t, tt.total, p.Damage.Total)
			require.Len(t, p.Damage.Rolls, 1)
			assert.Equal(t, tt.step, p.Damage.Rolls[0].Step)
		})
	}
}

// ADR 0008 step 16: what an exposure rolls is what is served, for every
// spell of the seeds with an exposure, every mode and rounds 1 to 6, at
// levels 9 and 15: one die per served die, of its sides, and the served
// constant on top.
func TestExposureAt_IsWhatAnExposureRolls(t *testing.T) {
	cat := catalog(t)
	n := 0
	for _, e := range cat.All() {
		for _, level := range []int{9, 15} {
			x, err := e.ExposureAt(level)
			require.NoError(t, err)
			if x == nil {
				continue
			}
			served := map[string]*spelleffectsvc.DamageAt{"": x.Damage}
			if x.Modes != nil {
				served = map[string]*spelleffectsvc.DamageAt{}
				for _, m := range x.Modes {
					served[m.Key] = m.Damage
				}
			}
			for mode, d := range served {
				for round := 1; round <= 6; round++ {
					sum := d.Dice
					if d.Steps != nil {
						sum = d.Steps[min(round, len(d.Steps))-1].Dice
					}
					var vals, sides []int
					want := sum.Constant
					for _, g := range sum.Dice {
						for range g.Count {
							vals = append(vals, 1)
							sides = append(sides, g.Sides)
							want++
						}
					}
					f := newFixture(t, vals...)
					res, err := f.svc.Expose(context.Background(), charID, castsvc.ExposureRequest{
						SpellID: e.ID, CasterLevel: level, Mode: mode, Round: round,
					})
					require.NoError(t, err, "%s %s round %d", e.ID, mode, round)
					assert.Empty(t, f.dice.vals, "%s %s round %d: every served die rolled", e.ID, mode, round)
					assert.Equal(t, sides, f.dice.sides, "%s %s round %d: the served dice", e.ID, mode, round)
					assert.Equal(t, want, res.Damage.Total, "%s %s round %d", e.ID, mode, round)
					n++
				}
			}
		}
	}
	assert.Equal(t, 2*6*(1+3+1), n, "cloudkill, wall of fire's three modes and death fog")
}

// ADR 0008 step 16 (user ruling 2026-10-15): exposed himself, the Bard
// takes the total off his HP in the same entry, through the HP ledger.
func TestExpose_OnTheBardTakesHP(t *testing.T) {
	f := newFixture(t, 9)
	res, err := f.svc.Expose(context.Background(), charID, castsvc.ExposureRequest{SpellID: "cloudkill", CasterLevel: 12, OnSelf: true})
	require.NoError(t, err)
	assert.True(t, res.OnSelf)
	assert.Equal(t, 9, res.Damage.Total)
	assert.Equal(t, &spelleffectsvc.ExposureAt{Damage: exposureDamage(12, once(1, 10), "poison")}, res.Exposure,
		"served for the next round")
	require.Len(t, f.entries, 1, "one entry: the roll and the HP change")
	assert.Equal(t, actionlog.KindExposure, f.entries[0].Kind)
	assert.Equal(t, -9, f.entries[0].HPDelta)
	assert.Contains(t, string(f.entries[0].Payload), `"onSelf":true`)
	assert.Contains(t, string(f.entries[0].Payload), `"exposure":{"damage":{"caster_level":12,`, "the served exposure, for the next round")
}

// ADR 0008 step 16: a spell without a usable exposure, an unknown mode, a
// caster level or round out of range, or no spell is refused, with nothing
// rolled or logged.
func TestExpose_Refused(t *testing.T) {
	tests := map[string]struct {
		req  castsvc.ExposureRequest
		want error
	}{
		"no exposure":                {castsvc.ExposureRequest{SpellID: "fireball", CasterLevel: 9}, spelleffectsvc.ErrNoExposure},
		"low confidence":             {castsvc.ExposureRequest{SpellID: "tashas_uncontrollable_hideous_laughter", CasterLevel: 9}, spelleffectsvc.ErrNoExposure},
		"not in the catalogue":       {castsvc.ExposureRequest{SpellID: "no_such_spell", CasterLevel: 9}, spelleffectsvc.ErrNoExposure},
		"unknown mode":               {castsvc.ExposureRequest{SpellID: "wall_of_fire", CasterLevel: 9, Mode: "far_away"}, spelleffectsvc.ErrUnknownMode},
		"a mode without modes":       {castsvc.ExposureRequest{SpellID: "cloudkill", CasterLevel: 9, Mode: "within_10_ft"}, spelleffectsvc.ErrUnknownMode},
		"no spell":                   {castsvc.ExposureRequest{CasterLevel: 9}, domain.ErrInvalidInput},
		"caster level 0":             {castsvc.ExposureRequest{SpellID: "cloudkill"}, domain.ErrInvalidInput},
		"caster level above the max": {castsvc.ExposureRequest{SpellID: "cloudkill", CasterLevel: castsvc.MaxCasterLevel + 1}, domain.ErrInvalidInput},
		"negative round":             {castsvc.ExposureRequest{SpellID: "cloudkill", CasterLevel: 9, Round: -1}, domain.ErrInvalidInput},
		"round above the max":        {castsvc.ExposureRequest{SpellID: "cloudkill", CasterLevel: 9, Round: spelleffectsvc.MaxExposureRound + 1}, domain.ErrInvalidInput},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.svc.Expose(context.Background(), charID, tt.req)
			require.ErrorIs(t, err, tt.want)
			assert.Empty(t, f.entries, "nothing logged")
			assert.Empty(t, f.dice.sides, "nothing rolled")
		})
	}
}

// ADR 0008 step 16: casting a spell whose only damage is its exposure rolls
// nothing, spends the slot and serves the exposure at the caster's level on
// the result; a spell received serves it at its caster's level. Spells
// without an exposure serve none.
func TestCast_Exposure_ServedOnTheResult(t *testing.T) {
	f := newFixture(t)
	f.prepared(cloudkill)
	r, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.Empty(t, f.dice.sides, "a cast rolls no exposure")
	assert.Nil(t, r.Damage)
	assert.Equal(t, castsvc.TextOnlyNone, r.TextOnly, "not a text-only spell")
	assert.Equal(t, &spelleffectsvc.ExposureAt{Damage: exposureDamage(bardLevel, once(1, 10), "poison")}, r.Exposure)
	require.Len(t, f.saved, 1)
	assert.Equal(t, []spellsvc.SpellSlot{{Level: 5, SpellID: "cloudkill", PreparedSpellName: "Cloudkill", Used: true}}, f.saved[0])
	require.Len(t, f.entries, 1)
	assert.Equal(t, actionlog.KindCast, f.entries[0].Kind)
	assert.Zero(t, f.entries[0].HPDelta)
	// The entry records the served exposure in the lookup's wire shape, so
	// the journal's cast card rolls an exposure from it.
	var p struct {
		Exposure json.RawMessage `json:"exposure"`
	}
	require.NoError(t, json.Unmarshal(f.entries[0].Payload, &p))
	assert.JSONEq(t, `{"damage":{"caster_level":15,"type":"poison","dice":{"groups":[{"count":1,"sides":10}],"modifier":0},
		"instances":{"groups":[],"modifier":1},"rounds":{"groups":[],"modifier":1}}}`, string(p.Exposure))

	r, err = f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "wall_of_fire", CasterLevel: 11})
	require.NoError(t, err)
	require.NotNil(t, r.Exposure)
	require.Len(t, r.Exposure.Modes, 3)
	assert.Equal(t, 11, r.Exposure.Modes[2].Damage.Dice.Constant, "2d6 + the caster's level")
	assert.Empty(t, f.dice.sides)

	f.prepared(slot("fireball", "Fireball", 3))
	f.dice.vals = []int{1, 2, 3, 4, 5, 6, 1, 2, 3, 4}
	r, err = f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.Nil(t, r.Exposure, "fireball has none")
}
