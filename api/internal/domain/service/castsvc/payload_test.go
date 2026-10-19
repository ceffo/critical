package castsvc_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

func damageJSON(t *testing.T, d *castsvc.DamageResult) string {
	t.Helper()
	raw, err := json.Marshal(castsvc.NewCastBreakdown(castsvc.CastResult{Damage: d}).Damage)
	require.NoError(t, err)
	return string(raw)
}

// ADR 0008 step 13: a cast's damage made of parts carries the parts, each
// instance's part rolls and the total on a save; without parts none of them.
func TestCastBreakdown_DamageParts(t *testing.T) {
	onSave := 3
	dice := []castsvc.DieRoll{{Sides: 4, Value: 1}, {Sides: 4, Value: 2}}
	got := damageJSON(t, &castsvc.DamageResult{
		Dice: "2d4+level", Type: "force", Instances: 1, Rounds: 1, Total: 18, TotalOnSave: &onSave,
		Parts: []castsvc.DamagePart{{Formula: "2d4", Save: "none"}, {Formula: "level", Save: "negates"}},
		Rolls: []castsvc.Roll{{
			Formula: "2d4+level", Dice: dice, Total: 18,
			Parts: []castsvc.Roll{{Formula: "2d4", Dice: dice, Total: 3}, {Formula: "level", Total: 15}},
		}},
	})
	assert.JSONEq(t, `{"dice":"2d4+level","type":"force","instances":1,"rounds":1,"total":18,"totalOnSave":3,
		"parts":[{"formula":"2d4","save":"none"},{"formula":"level","save":"negates"}],
		"rolls":[{"formula":"2d4+level","dice":[{"sides":4,"value":1},{"sides":4,"value":2}],"total":18,"parts":[
			{"formula":"2d4","dice":[{"sides":4,"value":1},{"sides":4,"value":2}],"total":3},
			{"formula":"level","dice":[],"total":15}]}]}`, got)

	got = damageJSON(t, &castsvc.DamageResult{Rolls: []castsvc.Roll{{Formula: "1d4"}}})
	assert.NotContains(t, got, "parts")
	assert.NotContains(t, got, "totalOnSave")
}

// ADR 0008 step 15: a cast's damage made of steps carries the steps' unit and
// each roll's step (no instance dice); without steps neither.
func TestCastBreakdown_DamageSteps(t *testing.T) {
	got := damageJSON(t, &castsvc.DamageResult{
		Type: "acid", Instances: 1, Rounds: 2, Total: 9, StepsPer: spelleffectsvc.StepPerRound,
		Rolls: []castsvc.Roll{
			{Formula: "(max(4-2*(step-1),0))d4", Step: 1, Total: 7, Dice: []castsvc.DieRoll{{Sides: 4, Value: 3}, {Sides: 4, Value: 4}}},
			{Formula: "(max(4-2*(step-1),0))d4", Step: 2, Total: 2, Dice: []castsvc.DieRoll{{Sides: 4, Value: 2}}},
		},
	})
	assert.JSONEq(t, `{"type":"acid","instances":1,"rounds":2,"total":9,"stepsPer":"round","rolls":[
		{"formula":"(max(4-2*(step-1),0))d4","step":1,"dice":[{"sides":4,"value":3},{"sides":4,"value":4}],"total":7},
		{"formula":"(max(4-2*(step-1),0))d4","step":2,"dice":[{"sides":4,"value":2}],"total":2}]}`, got)

	assert.NotContains(t, damageJSON(t, &castsvc.DamageResult{Rolls: []castsvc.Roll{{Formula: "1d4"}}}), "step")
}

// The spell attack's die is d20 and only d20: the previous release's d20Roll
// alias is gone.
func TestCastBreakdown_AttackHasNoD20RollAlias(t *testing.T) {
	raw, err := json.Marshal(castsvc.NewCastBreakdown(castsvc.CastResult{Attack: toHit()}).Attack)
	require.NoError(t, err)
	assert.JSONEq(t, `{"d20":17,"targetAc":5,"hit":true,"baseThac0":13,"modifiers":[{"kind":"dexterity","detail":"17","value":2}],
		"effectiveThac0":11,"targetACNeeded":-6,"hitMargin":11}`, string(raw))
}

func toHit() *combatsvc.ToHitRoll {
	return &combatsvc.ToHitRoll{
		D20Roll: 17, BaseThac0: 13, Modifiers: []combatsvc.Modifier{{Kind: "dexterity", Detail: "17", Value: 2}},
		EffectiveThac0: 11, TargetAC: 5, TargetACNeeded: -6, Hit: true, HitMargin: 11,
	}
}
