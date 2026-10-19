package castsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
)

var (
	chainLightning  = slot("chain_lightning", "Chain Lightning", 6)
	vitriolicSphere = slot("vitriolic_sphere", "Vitriolic Sphere", 4)
	incendiaryCloud = slot("incendiary_cloud", "Incendiary Cloud", 8)
)

// stepsOf builds served steps 1..n, step i rolling counts[i-1] dice of sides
// (0: no damage).
func stepsOf(sides int, counts ...int) []spelleffectsvc.DamageStepAt {
	out := make([]spelleffectsvc.DamageStepAt, len(counts))
	for i, c := range counts {
		out[i] = spelleffectsvc.DamageStepAt{Step: i + 1}
		if c > 0 {
			out[i].Dice = formula.Sum{Dice: []formula.Dice{{Count: c, Sides: sides}}}
		}
	}
	return out
}

// ADR 0008 step 15: the dice served for the stepped spells at caster levels
// 12, 15 and 20. Chain Lightning (PHB): (min(level,12))d6 for the first
// strike, 1d6 fewer each jump, as many strikes as the caster has levels: 12
// strikes from 12d6 down to 1d6 at all three levels (the dice run out before
// the strikes). Incendiary Cloud (PHB): no damage the first two rounds, then
// 1d2, 1d4 and 1d2 per caster level. Vitriolic Sphere (not in the PHB; the
// numbers of its data entry): (min(level,12))d4, two dice fewer each round
// until none are left.
func TestDamageAt_StepsMatchThePHB(t *testing.T) {
	chain := stepsOf(6, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1)
	sphere := stepsOf(4, 12, 10, 8, 6, 4, 2)
	cloud := func(level int) []spelleffectsvc.DamageStepAt {
		return []spelleffectsvc.DamageStepAt{
			{Step: 1},
			{Step: 2},
			{Step: 3, Dice: formula.Sum{Dice: []formula.Dice{{Count: level, Sides: 2}}}},
			{Step: 4, Dice: formula.Sum{Dice: []formula.Dice{{Count: level, Sides: 4}}}},
			{Step: 5, Dice: formula.Sum{Dice: []formula.Dice{{Count: level, Sides: 2}}}},
		}
	}
	for _, level := range []int{12, 15, 20} {
		f := newFixture(t)
		tests := []struct {
			slot      string
			per       spelleffectsvc.StepPer
			typ       string
			steps     []spelleffectsvc.DamageStepAt
			instances int
			rounds    int
		}{
			{chainLightning.SpellID, spelleffectsvc.StepPerInstance, "electricity", chain, 12, 1},
			{vitriolicSphere.SpellID, spelleffectsvc.StepPerRound, "acid", sphere, 1, 6},
			{incendiaryCloud.SpellID, spelleffectsvc.StepPerRound, "fire", cloud(level), 1, 5},
		}
		for _, tt := range tests {
			got, err := f.svc.DamageAt(context.Background(), tt.slot, level)
			require.NoError(t, err)
			require.NotNil(t, got, "%s at %d", tt.slot, level)
			assert.Equal(t, spelleffectsvc.DamageAt{
				CasterLevel: level, Type: tt.typ, StepsPer: tt.per, Steps: tt.steps,
				Instances: formula.Sum{Constant: tt.instances}, Rounds: formula.Sum{Constant: tt.rounds},
			}, *got, "%s at level %d", tt.slot, level)
		}
	}
}

// ADR 0008 step 15: a cast rolls every served step once, in order, each roll
// numbered by its step; the payload keeps each step's roll.
func TestCast_DamageSteps_RollsEachStepOnce(t *testing.T) {
	// At level 15 the Bard's bolt strikes 12 times, 12d6 down to 1d6: 78 dice.
	vals := make([]int, 78)
	for i := range vals {
		vals[i] = i%6 + 1
	}
	f := newFixture(t, vals...)
	f.prepared(chainLightning)
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.Empty(t, f.dice.vals, "every queued die is rolled")
	require.Len(t, f.dice.sides, 78, "no other die")

	d := res.Damage
	require.NotNil(t, d)
	assert.Equal(t, spelleffectsvc.StepPerInstance, d.StepsPer)
	assert.Equal(t, 12, d.Instances)
	assert.Equal(t, 1, d.Rounds)
	assert.Empty(t, d.Dice, "each step has its own dice")
	assert.Nil(t, d.TotalOnSave)
	require.Len(t, d.Rolls, 12)
	next, total := 0, 0
	for i, r := range d.Rolls {
		assert.Equal(t, i+1, r.Step)
		require.Len(t, r.Dice, 12-i, "strike %d rolls %dd6", i+1, 12-i)
		want := 0
		for _, die := range r.Dice {
			assert.Equal(t, castsvc.DieRoll{Sides: 6, Value: vals[next]}, die)
			want += vals[next]
			next++
		}
		assert.Equal(t, want, r.Total)
		total += want
	}
	assert.Equal(t, total, d.Total)

	var rec struct {
		StepsPer  string `json:"stepsPer"`
		Instances int    `json:"instances"`
		Rolls     []struct {
			Step  int `json:"step"`
			Total int `json:"total"`
			Dice  []struct{ Sides, Value int }
		} `json:"rolls"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(payloadField(t, f.entries[0].Payload, "damage"), &rec))
	assert.Equal(t, "instance", rec.StepsPer)
	assert.Equal(t, 12, rec.Instances)
	assert.Equal(t, total, rec.Total)
	require.Len(t, rec.Rolls, 12)
	for i, r := range rec.Rolls {
		assert.Equal(t, i+1, r.Step)
		assert.Equal(t, d.Rolls[i].Total, r.Total)
		assert.Len(t, r.Dice, 12-i)
	}
}

// ADR 0008 step 15: a spell received rolls its steps at the caster's level;
// the dice show distinct values, so a reused or skipped die fails.
func TestReceive_DamageSteps_AtTheCastersLevel(t *testing.T) {
	// Chain Lightning from a level 3 caster: three strikes, 3d6, 2d6, 1d6.
	f := newFixture(t, 1, 2, 3, 4, 5, 6)
	_, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: chainLightning.SpellID, CasterLevel: 3})
	require.NoError(t, err)
	assert.Empty(t, f.dice.vals)
	assert.Equal(t, []int{6, 6, 6, 6, 6, 6}, f.dice.sides)
	assert.JSONEq(t, `{"total":21,"type":"electricity","instances":3,"rounds":1,"stepsPer":"instance","rolls":[
		{"step":1,"formula":"(max(min(level,12)-(step-1),0))d6","total":6,"dice":[{"sides":6,"value":1},{"sides":6,"value":2},{"sides":6,"value":3}]},
		{"step":2,"formula":"(max(min(level,12)-(step-1),0))d6","total":9,"dice":[{"sides":6,"value":4},{"sides":6,"value":5}]},
		{"step":3,"formula":"(max(min(level,12)-(step-1),0))d6","total":6,"dice":[{"sides":6,"value":6}]}]}`,
		string(payloadField(t, f.entries[0].Payload, "damage")))

	// Incendiary Cloud from a level 2 caster: two rounds of smoke (no die),
	// then 2d2, 2d4 and 2d2; then its row's 4 + 1d6 rounds (ADR 0004 step
	// 20).
	f = newFixture(t, 2, 1, 3, 4, 1, 2, 5)
	res, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: incendiaryCloud.SpellID, CasterLevel: 2})
	require.NoError(t, err)
	assert.Empty(t, f.dice.vals)
	assert.Equal(t, []int{2, 2, 4, 4, 2, 2, 6}, f.dice.sides)
	require.Len(t, f.added, 1)
	assert.Equal(t, 9, f.added[0].Duration)
	d := res.Damage
	assert.Equal(t, spelleffectsvc.StepPerRound, d.StepsPer)
	assert.Equal(t, 5, d.Rounds)
	assert.Equal(t, []castsvc.Roll{
		{Formula: "0", Step: 1},
		{Formula: "0", Step: 2},
		{Formula: "leveld2", Step: 3, Total: 3, Dice: []castsvc.DieRoll{{Sides: 2, Value: 2}, {Sides: 2, Value: 1}}},
		{Formula: "leveld4", Step: 4, Total: 7, Dice: []castsvc.DieRoll{{Sides: 4, Value: 3}, {Sides: 4, Value: 4}}},
		{Formula: "leveld2", Step: 5, Total: 3, Dice: []castsvc.DieRoll{{Sides: 2, Value: 1}, {Sides: 2, Value: 2}}},
	}, d.Rolls)
	assert.Equal(t, 13, d.Total)
}
