package composition_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
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

// ADR 0049 step 7 and ADR 0025 step 4 (the writer lock): two uses of a per_day ability with one use left, at
// the same instant (a double click, two tabs): exactly one is paid, the other is refused as exhausted, and
// the log holds one entry. Repeated on fresh tables so the race has many chances.
func TestUseAbility_ADR0049Step7_TwoConcurrentUsesOfTheLastDoseOnePays(t *testing.T) {
	for range 25 {
		dice := &attackDice{t: t, dies: []int{1, 2, 3, 4, 5, 6}, calls: map[string]int{}}
		bs, ctx, id := abilityTable(t, dice)
		require.Equal(t, 1, itemOf(t, bs, ctx, id, "power_gem_dispel").Abilities[0].Limit.Left)

		errs := make([]error, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for n := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[n] = bs.Items.UseAbility(ctx, id, "power_gem_dispel", "use")
			}()
		}
		close(start)
		wg.Wait()

		paid, refused := 0, 0
		for _, err := range errs {
			if err == nil {
				paid++
				continue
			}
			require.ErrorIs(t, err, itemsvc.ErrAbilityExhausted)
			refused++
		}
		assert.Equal(t, 1, paid, "exactly one use is paid")
		assert.Equal(t, 1, refused)
		assert.Equal(t, 0, itemOf(t, bs, ctx, id, "power_gem_dispel").Abilities[0].Limit.Left)
		entries, err := bs.Log.List(ctx, id, 100)
		require.NoError(t, err)
		assert.Len(t, entries, 1, "one entry")
	}
}

// ADR 0049 step 7: a use at the same time as an editor save of the same item loses neither write: the
// use's whole-item write does not undo the edit (the note), the edit's does not give the dose back.
func TestUseAbility_ADR0049Step7_AUseAndAnEditorSaveLoseNeitherWrite(t *testing.T) {
	for range 25 {
		dice := &attackDice{t: t, dies: []int{1, 2, 3, 4, 5, 6}, calls: map[string]int{}}
		bs, ctx, id := abilityTable(t, dice)
		note := "edited while used"

		var useErr, editErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, useErr = bs.Items.UseAbility(ctx, id, "power_gem_dispel", "use")
		}()
		go func() {
			defer wg.Done()
			<-start
			_, editErr = bs.Items.Update(ctx, id, "power_gem_dispel", itemsvc.ItemPatch{Notes: &note})
		}()
		close(start)
		wg.Wait()

		require.NoError(t, useErr)
		require.NoError(t, editErr)
		got := itemOf(t, bs, ctx, id, "power_gem_dispel")
		assert.Equal(t, 0, got.Abilities[0].Limit.Left, "the use is kept")
		assert.Equal(t, note, got.Notes, "the edit is kept")
	}
}

// ADR 0049 step 20, both old kinds on both shapes: an item_use or item_restock entry logged before
// abilities (a count-only snapshot) is popped now. On a wand (a charges ability) its before-count goes
// into the counter and the quantity stays 1; on the Fire flasks (a with_item ability: the stock is the
// count) it goes into the count, the ability untouched.
func TestUndo_ADR0049Step20_OldUseAndRestockEntriesOnAWandAndAFlask(t *testing.T) {
	tests := []struct {
		name          string
		kind          actionlog.Kind
		itemID        string
		before, after int
		wantLeft      int // the wand's counter after the pop, -1: no counter
		wantCount     int
	}{
		{name: "old use of a wand", kind: actionlog.KindItemUse, itemID: "wand_lightning", before: 4, after: 3, wantLeft: 4, wantCount: 1},
		{name: "old restock of a wand", kind: actionlog.KindItemRestock, itemID: "wand_lightning", before: 2, after: 4, wantLeft: 2, wantCount: 1},
		{name: "old use of a flask", kind: actionlog.KindItemUse, itemID: "fire_flask", before: 5, after: 4, wantLeft: -1, wantCount: 5},
		{name: "old restock of a flask", kind: actionlog.KindItemRestock, itemID: "fire_flask", before: 3, after: 5, wantLeft: -1, wantCount: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs, ctx, id := abilityTable(t, &attackDice{t: t, calls: map[string]int{}})
			snap, err := itemsvc.CountSnapshot(tt.itemID, tt.before, tt.after)
			require.NoError(t, err)
			entryID, err := bs.Log.Record(ctx, actionlog.NewEntry{
				CharacterID: id, Kind: tt.kind, Payload: json.RawMessage(`{"itemId":"` + tt.itemID + `"}`), Snapshots: []actionlog.Snapshot{snap},
			})
			require.NoError(t, err)
			before := itemOf(t, bs, ctx, id, tt.itemID)

			_, err = bs.Log.Undo(ctx, id, entryID)
			require.NoError(t, err)
			got := itemOf(t, bs, ctx, id, tt.itemID)
			assert.Equal(t, tt.wantCount, got.Count)
			if tt.wantLeft >= 0 {
				assert.Equal(t, tt.wantLeft, got.Abilities[0].Limit.Left)
			} else {
				assert.Equal(t, before.Abilities, got.Abilities, "a stock's ability is untouched")
			}
		})
	}
}

// ADR 0049 steps 2 and 7: the last dose of a potion through either endpoint (the ability's use, the
// inventory's minus) removes the potion in the same entry; undo brings it back whole, redo removes it again.
func TestUseLastDose_ADR0049Step2_BothEndpointsRemoveUndoBringsBackRedoRemoves(t *testing.T) {
	uses := map[string]func(bs *composition.BackingServices, ctx context.Context, id int64, itemID string) error{
		"ability endpoint": func(bs *composition.BackingServices, ctx context.Context, id int64, itemID string) error {
			_, err := bs.Items.UseAbility(ctx, id, itemID, "use")
			return err
		},
		"spend-one endpoint": func(bs *composition.BackingServices, ctx context.Context, id int64, itemID string) error {
			_, err := bs.Items.UseItem(ctx, id, itemID)
			return err
		},
	}
	for name, use := range uses {
		t.Run(name, func(t *testing.T) {
			bs, ctx, id := abilityTable(t, &attackDice{t: t, dies: []int{1, 2, 3, 4, 5, 6}, calls: map[string]int{}})
			created, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{
				Name: localized.Of("Draught"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicPotion,
				Behaviour: itemsvc.BehaviourStock, Count: 1,
				Abilities: []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}}},
			})
			require.NoError(t, err)
			itemID := created.Item.ID
			has := func() bool {
				items, err := bs.Items.List(ctx, id)
				require.NoError(t, err)
				for _, it := range items {
					if it.ID == itemID {
						return true
					}
				}
				return false
			}
			require.True(t, has())

			require.NoError(t, use(bs, ctx, id, itemID))
			assert.False(t, has(), "the last dose removes the potion")
			entries, err := bs.Log.List(ctx, id, 1)
			require.NoError(t, err)
			_, err = bs.Log.Undo(ctx, id, entries[0].ID)
			require.NoError(t, err)
			require.True(t, has(), "undo brings it back")
			back := itemOf(t, bs, ctx, id, itemID)
			assert.Equal(t, 1, back.Count)
			assert.Equal(t, created.Item.Abilities, back.Abilities)

			redo, err := bs.Log.RedoTarget(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, redo)
			_, err = bs.Log.Redo(ctx, id, redo.ID)
			require.NoError(t, err)
			assert.False(t, has(), "redo removes it again")
		})
	}
}

// ADR 0049 step 2: the inventory's minus on a potion or a scroll with no ability (the plain stock the
// old use spent) takes the last one out in the same entry; undo restores the whole item (name,
// category, kind, count 1), redo removes it again.
func TestUseItem_ADR0049Step2_PlainLastOneIsRemovedUndoRestoresWholeRedoRemoves(t *testing.T) {
	for _, kind := range []string{itemsvc.MagicPotion, itemsvc.MagicScroll} {
		t.Run(kind, func(t *testing.T) {
			bs, ctx, id := abilityTable(t, &attackDice{t: t, dies: []int{1}, calls: map[string]int{}})
			created, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{
				Name: localized.Of("Plain " + kind), Category: itemsvc.CategoryMagic, MagicKind: kind,
				Behaviour: itemsvc.BehaviourStock, Count: 1, Notes: "kept", Abilities: []itemsvc.Ability{},
			})
			require.NoError(t, err)
			itemID := created.Item.ID
			before := itemOf(t, bs, ctx, id, itemID)
			exists := func() bool {
				items, err := bs.Items.List(ctx, id)
				require.NoError(t, err)
				for _, it := range items {
					if it.ID == itemID {
						return true
					}
				}
				return false
			}

			res, err := bs.Items.UseItem(ctx, id, itemID)
			require.NoError(t, err)
			assert.True(t, res.Removed)
			assert.False(t, exists())
			entries, err := bs.Log.List(ctx, id, 1)
			require.NoError(t, err)
			_, err = bs.Log.Undo(ctx, id, entries[0].ID)
			require.NoError(t, err)
			require.True(t, exists(), "undo brings it back")
			assert.Equal(t, before, itemOf(t, bs, ctx, id, itemID), "the whole item")

			redo, err := bs.Log.RedoTarget(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, redo)
			_, err = bs.Log.Redo(ctx, id, redo.ID)
			require.NoError(t, err)
			assert.False(t, exists(), "redo removes it again")
		})
	}
}

// ADR 0049 step 2 and ADR 0025 step 4: two restocks of +9 on a stock of 990, at the same instant,
// leave the stock at 999 at most: one is paid, the other refused as above the bound; one log entry.
func TestRestock_ADR0049Step2_TwoConcurrentRestocksNeverPassTheBound(t *testing.T) {
	for range 25 {
		bs, ctx, id := abilityTable(t, &attackDice{t: t, dies: []int{1}, calls: map[string]int{}})
		created, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{
			Name: localized.Of("Bolts"), Category: itemsvc.CategoryAmmunition, AmmoKind: "arrow", Behaviour: itemsvc.BehaviourStock, Count: 990,
		})
		require.NoError(t, err)
		errs := make([]error, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for n := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[n] = bs.Items.Restock(ctx, id, created.Item.ID, 9)
			}()
		}
		close(start)
		wg.Wait()

		paid := 0
		for _, err := range errs {
			if err == nil {
				paid++
			} else {
				require.ErrorIs(t, err, domain.ErrInvalidInput)
			}
		}
		assert.Equal(t, 1, paid)
		assert.Equal(t, 999, itemOf(t, bs, ctx, id, created.Item.ID).Count)
	}
}
