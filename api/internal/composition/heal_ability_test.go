package composition_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// healTable is the seeded character, hurt for `hurt` points (one hp_change entry), holding a stored
// potion with the given effects (written straight to the store, past the editor's validation, so a
// test can hold what the editor refuses).
func healTable(t *testing.T, dice *attackDice, hurt int, effects []itemsvc.Effect) (*composition.BackingServices, context.Context, int64, int) {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(store), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id := rootCharacter(t, bs)
	require.NoError(t, store.Items().Insert(ctx, id, itemsvc.Item{
		ID: "potion_heal", Position: 90, Name: localized.Of("Potion of healing"), Type: "potion", Category: itemsvc.CategoryMagic,
		MagicKind: itemsvc.MagicPotion, State: itemsvc.StateCarried, Count: 3, Behaviour: itemsvc.BehaviourStock, MaxCount: 3,
		Abilities: []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}, Effects: effects}},
	}))
	if hurt > 0 {
		_, err = bs.Log.Record(ctx, actionlog.NewEntry{
			CharacterID: id, Kind: actionlog.KindHPChange, Payload: json.RawMessage(`{"note":"hurt"}`), HPDelta: -hurt,
		})
		require.NoError(t, err)
	}
	v, err := bs.Character.View(ctx, id)
	require.NoError(t, err)
	return bs, ctx, id, v.HPMax
}

func currentHP(t *testing.T, bs *composition.BackingServices, ctx context.Context, id int64, hpMax int) int {
	t.Helper()
	total, err := bs.Log.HPTotal(ctx, id)
	require.NoError(t, err)
	return actionlog.CurrentHP(hpMax, total)
}

// ADR 0049 steps 9 and 12: a heal effect goes through the HP ledger in the use's transaction. The
// potion's 2d4+1 rolls once per die (distinct values), the use's one item_use entry is the HP delta,
// the result on the item's effect says what was applied, and the quantity went down by one.
func TestUseAbility_ADR0049Step12_HealIsAppliedThroughTheHPLedger(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{3, 4}, calls: map[string]int{}}
	bs, ctx, id, hpMax := healTable(t, dice, 12, []itemsvc.Effect{{Kind: itemsvc.EffectHeal, Dice: "2d4+1"}})
	require.Equal(t, hpMax-12, currentHP(t, bs, ctx, id, hpMax))

	cctx, changes := actionlog.WithChanges(ctx)
	res, err := bs.Items.UseAbility(cctx, id, "potion_heal", "use")
	require.NoError(t, err)
	assert.Contains(t, changes.Keys(), "hp", "the answer tells the web the hit points moved")
	require.Len(t, res.Effects, 1)
	assert.Equal(t, []int{3, 4}, res.Effects[0].Rolls)
	assert.Equal(t, 8, res.Effects[0].Total, "3 + 4 + 1")
	assert.Equal(t, &itemsvc.Applied{HPDelta: 8, HPBefore: hpMax - 12, HPAfter: hpMax - 4}, res.Effects[0].Applied)
	assert.Equal(t, map[string]int{"die": 2}, dice.calls, "one call per die")
	assert.Equal(t, hpMax-4, currentHP(t, bs, ctx, id, hpMax), "the ledger holds the heal")
	assert.Equal(t, 2, itemOf(t, bs, ctx, id, "potion_heal").Count, "a dose is spent")

	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	require.Equal(t, actionlog.KindItemUse, entries[0].Kind)
	assert.Equal(t, 8, entries[0].HPDelta)
}

// ADR 0049 step 12 and ADR 0007 step 5: a heal never takes HP above max; the effect's applied is
// the capped amount and the roll is kept, and a heal at full HP applies 0. A full heal rolls
// nothing and fills the gap.
func TestUseAbility_ADR0049Step12_HealIsCappedAtMaxAndFullFillsTheGap(t *testing.T) {
	t.Run("capped", func(t *testing.T) {
		dice := &attackDice{t: t, dies: []int{4, 4}, calls: map[string]int{}}
		bs, ctx, id, hpMax := healTable(t, dice, 3, []itemsvc.Effect{{Kind: itemsvc.EffectHeal, Dice: "2d4"}})
		res, err := bs.Items.UseAbility(ctx, id, "potion_heal", "use")
		require.NoError(t, err)
		assert.Equal(t, 8, res.Effects[0].Total, "the roll")
		assert.Equal(t, 3, res.Effects[0].Applied.HPDelta, "capped to the gap")
		assert.Equal(t, hpMax, currentHP(t, bs, ctx, id, hpMax))
	})
	t.Run("at full HP", func(t *testing.T) {
		dice := &attackDice{t: t, dies: []int{4, 4}, calls: map[string]int{}}
		bs, ctx, id, hpMax := healTable(t, dice, 0, []itemsvc.Effect{{Kind: itemsvc.EffectHeal, Dice: "2d4"}})
		res, err := bs.Items.UseAbility(ctx, id, "potion_heal", "use")
		require.NoError(t, err)
		assert.Zero(t, res.Effects[0].Applied.HPDelta)
		assert.Equal(t, 2, itemOf(t, bs, ctx, id, "potion_heal").Count, "the dose is still spent")
		assert.Equal(t, hpMax, currentHP(t, bs, ctx, id, hpMax))
	})
	t.Run("full", func(t *testing.T) {
		dice := &attackDice{t: t, calls: map[string]int{}} // a roll fails the test
		bs, ctx, id, hpMax := healTable(t, dice, 17, []itemsvc.Effect{{Kind: itemsvc.EffectHeal, Full: true}})
		res, err := bs.Items.UseAbility(ctx, id, "potion_heal", "use")
		require.NoError(t, err)
		assert.True(t, res.Effects[0].Full)
		assert.Equal(t, 17, res.Effects[0].Applied.HPDelta)
		assert.Empty(t, dice.calls)
		assert.Equal(t, hpMax, currentHP(t, bs, ctx, id, hpMax))
	})
}

// ADR 0049 step 12: effects run in list order and the second heal sees what the first filled: with 4
// points missing, a heal of 4 then a heal of 4 applies 4 then 0, and the summed result is what the
// stored entry's ledger breakdown says it applied (hpRequested 4, hpApplied 4) and its HP delta.
func TestUseAbility_ADR0049Step12_TwoHealsShareTheGapInListOrder(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{4, 4}, calls: map[string]int{}}
	bs, ctx, id, hpMax := healTable(t, dice, 4, []itemsvc.Effect{
		{Kind: itemsvc.EffectHeal, Dice: "1d4"}, {Kind: itemsvc.EffectHeal, Dice: "1d4"},
	})
	res, err := bs.Items.UseAbility(ctx, id, "potion_heal", "use")
	require.NoError(t, err)
	assert.Equal(t, 4, res.Effects[0].Applied.HPDelta)
	assert.Zero(t, res.Effects[1].Applied.HPDelta, "the gap was filled by the first")
	assert.Equal(t, hpMax, currentHP(t, bs, ctx, id, hpMax))
	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	summed := res.Effects[0].Applied.HPDelta + res.Effects[1].Applied.HPDelta
	assert.Equal(t, summed, entries[0].HPDelta)
	var stored struct {
		Requested int `json:"hpRequested"`
		Applied   int `json:"hpApplied"`
	}
	require.NoError(t, json.Unmarshal(entries[0].Payload, &stored))
	assert.Equal(t, summed, stored.Applied, "the entry's own cap agrees with the effects' running total")
	assert.Equal(t, 4, stored.Requested)
}

// ADR 0049 step 12: a failure in any effect rolls the whole use back, the limit included. The
// potion holds a heal then a heal whose dice cannot be rolled (a stored value the editor would
// refuse): the first heal's roll happened, then the use fails, and neither the dose, the HP nor
// the log moved.
func TestUseAbility_ADR0049Step12_AFailingEffectRollsBackTheLimitAndTheHeal(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{4}, calls: map[string]int{}}
	bs, ctx, id, hpMax := healTable(t, dice, 12, []itemsvc.Effect{
		{Kind: itemsvc.EffectHeal, Dice: "1d4"}, {Kind: itemsvc.EffectHeal, Dice: "lots"},
	})
	before, err := bs.Log.List(ctx, id, 100)
	require.NoError(t, err)

	_, err = bs.Items.UseAbility(ctx, id, "potion_heal", "use")
	require.Error(t, err)
	assert.Equal(t, map[string]int{"die": 1}, dice.calls, "the first heal did roll")
	assert.Equal(t, 3, itemOf(t, bs, ctx, id, "potion_heal").Count, "the limit's payment is rolled back")
	assert.Equal(t, hpMax-12, currentHP(t, bs, ctx, id, hpMax))
	after, err := bs.Log.List(ctx, id, 100)
	require.NoError(t, err)
	assert.Len(t, after, len(before), "nothing is logged")
}

// ADR 0049 step 12: one undo pops the use: the dose comes back and the HP with it (the heal is the
// entry's own delta, no entry of its own); one redo applies the same heal again.
func TestUseAbility_ADR0049Step12_OneUndoPopsTheHealWithTheUse(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{3, 4}, calls: map[string]int{}}
	bs, ctx, id, hpMax := healTable(t, dice, 12, []itemsvc.Effect{{Kind: itemsvc.EffectHeal, Dice: "2d4"}})
	_, err := bs.Items.UseAbility(ctx, id, "potion_heal", "use")
	require.NoError(t, err)
	require.Equal(t, hpMax-5, currentHP(t, bs, ctx, id, hpMax))
	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)

	_, err = bs.Log.Undo(ctx, id, entries[0].ID)
	require.NoError(t, err)
	assert.Equal(t, hpMax-12, currentHP(t, bs, ctx, id, hpMax), "the heal is popped")
	assert.Equal(t, 3, itemOf(t, bs, ctx, id, "potion_heal").Count, "and the dose is back")

	redo, err := bs.Log.RedoTarget(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, redo)
	_, err = bs.Log.Redo(ctx, id, redo.ID)
	require.NoError(t, err)
	assert.Equal(t, hpMax-5, currentHP(t, bs, ctx, id, hpMax), "redo heals the same")
	assert.Equal(t, 2, itemOf(t, bs, ctx, id, "potion_heal").Count)
}
