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
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// abilityTable is the seeded character (the fixture seeds are in the old shape: the boot conversion
// of the seed gives the Wand of lightning a charges ability, the Power Gem a per_day one and the
// Fire flasks a with_item one) with scripted dice.
func abilityTable(t *testing.T, dice *attackDice) (*composition.BackingServices, context.Context, int64) {
	t.Helper()
	ctx := context.Background()
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	return bs, ctx, rootCharacter(t, bs)
}

func itemOf(t *testing.T, bs *composition.BackingServices, ctx context.Context, id int64, itemID string) itemsvc.Item {
	t.Helper()
	items, err := bs.Items.List(ctx, id)
	require.NoError(t, err)
	for _, it := range items {
		if it.ID == itemID {
			return it
		}
	}
	require.Failf(t, "item not found", "%s", itemID)
	return itemsvc.Item{}
}

// ADR 0049 steps 5, 6 and 7 through the real services: using the Wand of lightning's ability pays a
// charge and rolls 6d6 in one transaction, logs one item_use entry naming the ability and the state
// keys of what it changed; one undo gives the charge back, one redo pays it again; the quantity of the
// wand stays 1 throughout.
func TestUseAbility_ADR0049Step6_PaysRollsLogsAndUndoRedoRestoreTheLimit(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{1, 2, 3, 4, 5, 6}, calls: map[string]int{}}
	bs, ctx, id := abilityTable(t, dice)
	wand := itemOf(t, bs, ctx, id, "wand_lightning")
	require.Equal(t, 1, wand.Count, "the seed holds a quantity of one object, the four charges are the ability's")
	require.Equal(t, itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 4, Left: 4, Cost: 1}, wand.Abilities[0].Limit)

	cctx, changes := actionlog.WithChanges(ctx)
	res, err := bs.Items.UseAbility(cctx, id, "wand_lightning", "use")
	require.NoError(t, err)
	assert.Equal(t, []string{"items:wand_lightning", "log"}, changes.Keys())
	require.Len(t, res.Effects, 1)
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6}, res.Effects[0].Rolls)
	assert.Equal(t, 21, res.Effects[0].Total)
	assert.Equal(t, map[string]int{"die": 6}, dice.calls, "one call per die")
	got := itemOf(t, bs, ctx, id, "wand_lightning")
	assert.Equal(t, 3, got.Abilities[0].Limit.Left)
	assert.Equal(t, 1, got.Count)

	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, actionlog.KindItemUse, entries[0].Kind)
	var payload struct {
		AbilityID string `json:"abilityId"`
		Left      int    `json:"left"`
	}
	require.NoError(t, json.Unmarshal(entries[0].Payload, &payload))
	assert.Equal(t, "use", payload.AbilityID)
	assert.Equal(t, 3, payload.Left)

	_, err = bs.Log.Undo(ctx, id, entries[0].ID)
	require.NoError(t, err)
	assert.Equal(t, 4, itemOf(t, bs, ctx, id, "wand_lightning").Abilities[0].Limit.Left, "undo gives the charge back")
	redo, err := bs.Log.RedoTarget(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, redo)
	_, err = bs.Log.Redo(ctx, id, redo.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, itemOf(t, bs, ctx, id, "wand_lightning").Abilities[0].Limit.Left, "redo pays it again")
}

// ADR 0049 steps 5 and 6: the Power Gem's dispel is once a day: used, the second use is refused with
// nothing logged, the long rest refills it in its own entry, and undoing the rest gives back the used
// state (the whole item is in the snapshot); the wand is not recharged by the rest.
func TestUseAbility_ADR0049Step6_LongRestRefillsPerDayOnly(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{1, 2, 3, 4, 5, 6}, calls: map[string]int{}}
	bs, ctx, id := abilityTable(t, dice)
	_, err := bs.Items.UseAbility(ctx, id, "power_gem_dispel", "use")
	require.NoError(t, err)
	_, err = bs.Items.UseAbility(ctx, id, "wand_lightning", "use")
	require.NoError(t, err)
	before, err := bs.Log.List(ctx, id, 100)
	require.NoError(t, err)

	_, err = bs.Items.UseAbility(ctx, id, "power_gem_dispel", "use")
	assert.ErrorIs(t, err, itemsvc.ErrAbilityExhausted)
	after, err := bs.Log.List(ctx, id, 100)
	require.NoError(t, err)
	assert.Len(t, after, len(before), "a refusal logs nothing")
	gem := itemOf(t, bs, ctx, id, "power_gem_dispel")
	assert.Equal(t, 0, gem.Abilities[0].Limit.Left)
	assert.Equal(t, itemsvc.ReasonExhausted, gem.UseRefusal(gem.Abilities[0]))

	rest, err := bs.Rest.LongRest(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 1, itemOf(t, bs, ctx, id, "power_gem_dispel").Abilities[0].Limit.Left, "refilled")
	assert.Equal(t, 3, itemOf(t, bs, ctx, id, "wand_lightning").Abilities[0].Limit.Left, "a wand is never recharged")
	var refilled struct {
		ItemsRefilled []string `json:"itemsRefilled"`
	}
	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(entries[0].Payload, &refilled))
	assert.Equal(t, []string{"Power Gem"}, refilled.ItemsRefilled)

	_, err = bs.Log.Undo(ctx, id, rest.EntryID)
	require.NoError(t, err)
	assert.Equal(t, 0, itemOf(t, bs, ctx, id, "power_gem_dispel").Abilities[0].Limit.Left, "undoing the rest: used again")
}

// ADR 0049 step 2: the inventory's minus is spend one: no ability, no dice (the Fire flasks hold a
// 5d6 dice ability, and the scripted dice are empty: a roll fails the test), no initiative; the entry
// carries no abilityId; undo gives the flask back.
func TestUseItem_ADR0049Step2_SpendOneRollsNothingAndLogsNoAbility(t *testing.T) {
	dice := &attackDice{t: t, calls: map[string]int{}}
	bs, ctx, id := abilityTable(t, dice)

	res, err := bs.Items.UseItem(ctx, id, "fire_flask")
	require.NoError(t, err)
	assert.Equal(t, 4, res.Remaining)
	assert.Empty(t, dice.calls)
	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	assert.NotContains(t, string(entries[0].Payload), "abilityId")
	_, err = bs.Log.Undo(ctx, id, entries[0].ID)
	require.NoError(t, err)
	assert.Equal(t, 5, itemOf(t, bs, ctx, id, "fire_flask").Count)
}

// ADR 0049 step 15 out of combat: an ability used in round 0 records no initiative correction; in
// combat the wand's use corrects the round (the unit tests of itemsvc cover the kinds).
func TestUseAbility_ADR0049Step15_NothingOutOfCombat(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{1, 2, 3, 4, 5, 6}, calls: map[string]int{}}
	tb := newInitiativeTable(t, dice, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Bram", Class: "fighter", Race: "human", Str: 16, Dex: 16, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 10,
	})
	wand, err := tb.bs.Items.Create(tb.ctx, tb.id, itemsvc.ItemInput{
		Name: localized.Of("Wand of frost"), Category: itemsvc.CategoryMagic, MagicKind: "wand", Behaviour: itemsvc.BehaviourStock, Count: 1,
		Abilities: useAbility(itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 5, Left: 5, Cost: 1}),
	})
	require.NoError(t, err)
	_, err = tb.bs.Items.UseAbility(tb.ctx, tb.id, wand.Item.ID, "use")
	require.NoError(t, err)
	assert.Equal(t, 0, tb.countKind(t, actionlog.KindInitiativeCorrect), "round 0: nothing")
	assert.Nil(t, tb.initiative(t))
}

// ADR 0049 step 20: entries the previous releases logged stay decodable and undoable, never rewritten:
// an old use of the wand (a count-only snapshot of 4 to 3, an old-shape payload) popped now puts its
// before-count into the ability's counter, and the wand stays one object. A counter the entry cannot
// reach (above the max) is clamped.
func TestUndo_ADR0049Step20_AnOldUseOfAWandGivesBackItsChargeThroughTheAbility(t *testing.T) {
	dice := &attackDice{t: t, calls: map[string]int{}}
	bs, ctx, id := abilityTable(t, dice)
	wand := itemOf(t, bs, ctx, id, "wand_lightning")
	wand.Abilities[0].Limit.Left = 3 // what the old use left
	_, err := bs.Items.Update(ctx, id, "wand_lightning", itemsvc.ItemPatch{Abilities: &wand.Abilities})
	require.NoError(t, err)

	snap, err := itemsvc.CountSnapshot("wand_lightning", 4, 3)
	require.NoError(t, err)
	oldPayload := `{"itemId":"wand_lightning","name":"Wand of lightning","type":"wand","behaviour":"charges","removed":false,"remaining":3,"maxCount":4,"dice":"6d6","rolls":[1,2,3,4,5,6],"total":21,"save":"rod_staff_wand"}`
	entryID, err := bs.Log.Record(ctx, actionlog.NewEntry{
		CharacterID: id, Kind: actionlog.KindItemUse, Payload: json.RawMessage(oldPayload), Snapshots: []actionlog.Snapshot{snap},
	})
	require.NoError(t, err)
	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	assert.JSONEq(t, oldPayload, string(entries[0].Payload), "the payload is as it was written")

	_, err = bs.Log.Undo(ctx, id, entryID)
	require.NoError(t, err)
	got := itemOf(t, bs, ctx, id, "wand_lightning")
	assert.Equal(t, 4, got.Abilities[0].Limit.Left, "the old before-count went into the counter")
	assert.Equal(t, 1, got.Count, "the quantity is left alone")
}
