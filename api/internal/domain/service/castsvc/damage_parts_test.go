package castsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
)

var forceMissiles = slot("mordenkainens_force_missiles", "Mordenkainen’s Force Missiles", 4)

// ADR 0008 step 13: Force Missiles is per missile 2d4 (no save) plus a burst
// of 1 per level that a save negates; one missile at level 7, one more every
// three levels, seven at level 25.
func TestDamageAt_ForceMissilesParts(t *testing.T) {
	tests := []struct{ level, missiles int }{{7, 1}, {15, 3}, {25, 7}}
	for _, tt := range tests {
		f := newFixture(t)
		got, err := f.svc.DamageAt(context.Background(), forceMissiles.SpellID, tt.level)
		require.NoError(t, err)
		require.NotNil(t, got)
		d2d4 := formula.Sum{Dice: []formula.Dice{{Count: 2, Sides: 4}}}
		assert.Equal(t, spelleffectsvc.DamageAt{
			CasterLevel: tt.level, Type: "force",
			Dice:      formula.Sum{Dice: d2d4.Dice, Constant: tt.level},
			Instances: formula.Sum{Constant: tt.missiles}, Rounds: formula.Sum{Constant: 1},
			Parts: []spelleffectsvc.DamagePartAt{
				{Dice: d2d4, Save: spelleffectsvc.SaveNone},
				{Dice: formula.Sum{Constant: tt.level}, Save: spelleffectsvc.SaveNegates},
			},
		}, *got, "level %d", tt.level)
	}
}

// ADR 0008 step 13: a cast rolls each part of each instance exactly once, in
// the data's order, and reports the total and the total on a save (the
// negated parts left out). The payload keeps every part roll.
func TestCast_DamageParts_RollsEachPartOnce(t *testing.T) {
	// Three missiles at level 15: 2d4 each, then the burst (no die).
	f := newFixture(t, 1, 2, 3, 4, 2, 3)
	f.prepared(forceMissiles)
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.Empty(t, f.dice.vals, "every queued die is rolled")
	assert.Equal(t, []int{4, 4, 4, 4, 4, 4}, f.dice.sides, "six d4, no other die")

	d := res.Damage
	require.NotNil(t, d)
	assert.Equal(t, []castsvc.DamagePart{
		{Formula: "2d4", Save: spelleffectsvc.SaveNone},
		{Formula: "level", Save: spelleffectsvc.SaveNegates},
	}, d.Parts)
	assert.Equal(t, "2d4+level", d.Dice)
	assert.Equal(t, 3, d.Instances)
	missile := func(a, b int) castsvc.Roll {
		dice := []castsvc.DieRoll{{Sides: 4, Value: a}, {Sides: 4, Value: b}}
		return castsvc.Roll{
			Formula: "2d4+level", Dice: dice, Total: a + b + 15,
			Parts: []castsvc.Roll{{Formula: "2d4", Dice: dice, Total: a + b}, {Formula: "level", Total: 15}},
		}
	}
	assert.Equal(t, []castsvc.Roll{missile(1, 2), missile(3, 4), missile(2, 3)}, d.Rolls)
	assert.Equal(t, 3+7+5+3*15, d.Total)
	require.NotNil(t, d.TotalOnSave)
	assert.Equal(t, 3+7+5, *d.TotalOnSave, "a save negates the three bursts")

	assert.JSONEq(t, `{"total":60,"type":"force","dice":"2d4+level","instances":3,"rounds":1,
		"parts":[{"formula":"2d4","save":"none"},{"formula":"level","save":"negates"}],"totalOnSave":15,
		"rolls":[
		{"formula":"2d4+level","dice":[{"sides":4,"value":1},{"sides":4,"value":2}],"total":18,"parts":[
			{"formula":"2d4","dice":[{"sides":4,"value":1},{"sides":4,"value":2}],"total":3},
			{"formula":"level","dice":[],"total":15}]},
		{"formula":"2d4+level","dice":[{"sides":4,"value":3},{"sides":4,"value":4}],"total":22,"parts":[
			{"formula":"2d4","dice":[{"sides":4,"value":3},{"sides":4,"value":4}],"total":7},
			{"formula":"level","dice":[],"total":15}]},
		{"formula":"2d4+level","dice":[{"sides":4,"value":2},{"sides":4,"value":3}],"total":20,"parts":[
			{"formula":"2d4","dice":[{"sides":4,"value":2},{"sides":4,"value":3}],"total":5},
			{"formula":"level","dice":[],"total":15}]}]}`,
		string(payloadField(t, f.entries[0].Payload, "damage")))
}

// A spell received from a level 25 caster rolls its seven missiles at that
// level.
func TestReceive_DamageParts_AtTheCastersLevel(t *testing.T) {
	f := newFixture(t, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1)
	res, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: forceMissiles.SpellID, CasterLevel: 25})
	require.NoError(t, err)
	assert.Empty(t, f.dice.vals)
	require.Len(t, res.Damage.Rolls, 7)
	assert.Equal(t, 7*(2+25), res.Damage.Total)
	assert.Equal(t, 7*2, *res.Damage.TotalOnSave)
}
