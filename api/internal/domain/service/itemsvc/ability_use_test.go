package itemsvc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

// zombieSword is a magical sword whose one ability, "turn zombies", is used twice a day and rolls nothing.
func zombieSword(left int) itemsvc.Item {
	return itemsvc.Item{
		ID: "sword", Position: 2, Name: localized.Of("Sword of the dead"), Type: "sword", Category: itemsvc.CategoryGear,
		State: itemsvc.StateCarried, Count: 1,
		Abilities: []itemsvc.Ability{{
			ID: "turn", Name: localized.Text{"en": "Turn zombies"}, Note: "Zombies within 30 feet",
			Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 2, Left: left},
		}},
	}
}

// lightningWand is the Wand of lightning: 6d6 with a save, four charges, one per use.
func lightningWand(left int) itemsvc.Item {
	return itemsvc.Item{
		ID: "wand", Position: 3, Name: localized.Of("Wand of lightning"), Type: "wand", Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicWand,
		State: itemsvc.StateCarried, Count: 1,
		Abilities: []itemsvc.Ability{{
			ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 4, Left: left, Cost: 1},
			Effects: []itemsvc.Effect{{Kind: itemsvc.EffectDice, Dice: "6d6", Save: "rod_staff_wand"}},
		}},
	}
}

// useOnce runs UseAbility on before with the DAL as expectUse sets it, the log answering 9, and
// returns what it logged.
func useOnce(t *testing.T, f fixture, dal *mocks.MockDAL, before itemsvc.Item, abilityID string) (*itemsvc.AbilityUseResult, actionlog.NewEntry, error) {
	t.Helper()
	svc, err := itemsvc.New(dal, f.Deps)
	require.NoError(t, err)
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(9), nil).Maybe()
	res, err := svc.UseAbility(context.Background(), 1, before.ID, abilityID)
	return res, got, err
}

// ADR 0049 step 5: one use pays the limit by its kind: unlimited nothing, per_day one of left,
// charges its cost of left, with_item one of the quantity.
func TestUseAbility_ADR0049Step5_PaysTheLimitByKind(t *testing.T) {
	charges2 := lightningWand(5)
	charges2.Abilities[0].Limit = itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 6, Left: 5, Cost: 2}
	tests := []struct {
		name string
		item itemsvc.Item
		want itemsvc.Limit // the limit written, when a counter is
	}{
		{"per_day: 1 of left", zombieSword(2), itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 2, Left: 1}},
		{"per_day: the last one", zombieSword(1), itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 2, Left: 0}},
		{"charges: cost 1", lightningWand(4), itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 4, Left: 3, Cost: 1}},
		{"charges: cost 2", charges2, itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 6, Left: 3, Cost: 2}},
		{"charges: the last charge keeps the item listed", lightningWand(1), itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 4, Left: 0, Cost: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t) // Delete fails the strict mock: a counter item is never removed
			f := readsCharacter(t)
			read := tt.item.Abilities[0].Limit
			for _, v := range []int{1, 2, 3, 4, 5, 6} {
				f.dice.EXPECT().RollDie(6).Return(v).Maybe()
			}
			dal.EXPECT().Get(mock.Anything, int64(1), tt.item.ID).Return(tt.item, nil)
			var written itemsvc.Item
			dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { written = it }).Return(nil).Once()

			res, _, err := useOnce(t, f, dal, tt.item, tt.item.Abilities[0].ID)
			require.NoError(t, err)
			assert.Equal(t, tt.want, written.Abilities[0].Limit)
			assert.Equal(t, 1, written.Count, "the quantity of one object never moves")
			require.NotNil(t, res.Left)
			assert.Equal(t, tt.want.Left, *res.Left)
			assert.Equal(t, tt.want.Max, *res.Max)
			assert.Equal(t, tt.want.Left, res.Remaining)
			assert.False(t, res.Removed)
			assert.Equal(t, read, tt.item.Abilities[0].Limit, "the item read is not mutated")
		})
	}

	t.Run("unlimited: nothing is written", func(t *testing.T) {
		rope := itemsvc.Item{
			ID: "rope", Name: localized.Of("Rope"), Category: itemsvc.CategoryGear, Count: 2,
			Abilities: []itemsvc.Ability{{ID: "climb", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}}},
		}
		dal := mocks.NewMockDAL(t) // an Update, a Decrement or a Delete fails the strict mock
		dal.EXPECT().Get(mock.Anything, int64(1), "rope").Return(rope, nil)
		res, _, err := useOnce(t, deps(t), dal, rope, "climb")
		require.NoError(t, err)
		assert.Nil(t, res.Left)
		assert.Zero(t, res.Remaining)
	})

	t.Run("with_item: one of the quantity, atomically", func(t *testing.T) {
		flask := itemsvc.Item{
			ID: "flask", Name: localized.Of("Fire flasks"), Category: itemsvc.CategoryGear, Count: 5,
			Abilities: []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}}},
		}
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Get(mock.Anything, int64(1), "flask").Return(flask, nil)
		after := flask
		after.Count = 4
		dal.EXPECT().Decrement(mock.Anything, int64(1), "flask").Return(after, nil).Once()
		res, _, err := useOnce(t, deps(t), dal, flask, "use")
		require.NoError(t, err)
		assert.Equal(t, 4, res.Remaining)
		assert.Nil(t, res.Left)
		assert.False(t, res.Removed)
	})
}

// ADR 0049 step 5: an ability whose limit cannot pay is refused (exhausted) and nothing changes:
// nothing written, nothing rolled, nothing logged, no initiative (the mocks fail on any call).
func TestUseAbility_ADR0049Step5_ExhaustedIsRefusedAndChangesNothing(t *testing.T) {
	short := lightningWand(1)
	short.Abilities[0].Limit = itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 6, Left: 1, Cost: 2}
	tests := map[string]itemsvc.Item{
		"per_day at 0":           zombieSword(0),
		"charges below the cost": short,
		"charges at 0":           lightningWand(0),
		"with_item at quantity 0": {
			ID: "flask", Name: localized.Of("Flasks"), Category: itemsvc.CategoryGear, Count: 0,
			Abilities: []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}}},
		},
	}
	for name, it := range tests {
		t.Run(name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t) // only the read is expected
			f := readsCharacter(t)
			dal.EXPECT().Get(mock.Anything, int64(1), it.ID).Return(it, nil)
			svc, err := itemsvc.New(dal, f.Deps)
			require.NoError(t, err)
			_, err = svc.UseAbility(context.Background(), 1, it.ID, it.Abilities[0].ID)
			assert.ErrorIs(t, err, itemsvc.ErrAbilityExhausted)
			assert.Equal(t, itemsvc.ReasonExhausted, it.UseRefusal(it.Abilities[0]), "the wire serves the reason")
		})
	}
}

// ADR 0049 steps 5 and 14: the server says per ability whether a use is paid now: nothing when it
// is, the reason `exhausted` when not; the item's state does not matter (step 7).
func TestItem_ADR0049Step14_UseRefusalPerAbility(t *testing.T) {
	assert.Empty(t, zombieSword(1).UseRefusal(zombieSword(1).Abilities[0]))
	assert.Equal(t, itemsvc.ReasonExhausted, zombieSword(0).UseRefusal(zombieSword(0).Abilities[0]))
	worn := zombieSword(1)
	worn.State = itemsvc.StateWorn
	assert.Empty(t, worn.UseRefusal(worn.Abilities[0]))
	sheathed := lightningWand(2)
	sheathed.State = itemsvc.StateCarried
	assert.Empty(t, sheathed.UseRefusal(sheathed.Abilities[0]), "carried, not wielded: still usable")
}

// ADR 0049 step 7: an item or an ability that does not exist is refused with its own error and
// changes nothing.
func TestUseAbility_ADR0049Step7_UnknownItemOrAbility(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := readsCharacter(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "nope").Return(itemsvc.Item{}, fmt.Errorf("%w: nope", itemsvc.ErrItemNotFound))
	_, err := svc.UseAbility(context.Background(), 1, "nope", "use")
	assert.ErrorIs(t, err, itemsvc.ErrItemNotFound)

	sword := zombieSword(2)
	dal.EXPECT().Get(mock.Anything, int64(1), "sword").Return(sword, nil)
	_, err = svc.UseAbility(context.Background(), 1, "sword", "fly")
	assert.ErrorIs(t, err, itemsvc.ErrAbilityNotFound)
	_, err = svc.UseAbility(context.Background(), 1, "sword", "use")
	assert.ErrorIs(t, err, itemsvc.ErrAbilityNotFound, "an id of another item's ability")
}

// ADR 0049 steps 6 and 7: one use pays the limit, rolls the dice effect (every roll distinct, one
// call each), and logs one item_use entry: the ability, the limit as it stands, the effect's rolls
// and the flat fields the journal words, with the whole item's snapshot before and after.
func TestUseAbility_ADR0049Step6_RollsTheDiceEffectAndLogsOneEntry(t *testing.T) {
	wand := lightningWand(4)
	dal := mocks.NewMockDAL(t)
	f := readsCharacter(t)
	for _, v := range []int{1, 2, 3, 4, 5, 6} {
		f.dice.EXPECT().RollDie(6).Return(v).Once()
	}
	dal.EXPECT().Get(mock.Anything, int64(1), "wand").Return(wand, nil)
	dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil).Once()

	res, got, err := useOnce(t, f, dal, wand, "use")
	require.NoError(t, err)
	assert.Equal(t, int64(9), res.LogEntryID)
	require.Len(t, res.Effects, 1)
	assert.Equal(t, itemsvc.EffectResult{
		Kind: itemsvc.EffectDice, Dice: "6d6", Rolls: []int{1, 2, 3, 4, 5, 6}, Total: 21, Save: "rod_staff_wand",
	}, res.Effects[0])

	assert.Equal(t, actionlog.KindItemUse, got.Kind)
	assert.True(t, actionlog.IsRoll(got.Kind))
	assert.JSONEq(t, `{"itemId":"wand","abilityId":"use","name":"Wand of lightning","type":"wand","limit":"charges","left":3,"max":4,
		"removed":false,"remaining":3,"maxCount":4,"dice":"6d6","rolls":[1,2,3,4,5,6],"total":21,"save":"rod_staff_wand",
		"effects":[{"kind":"dice","dice":"6d6","rolls":[1,2,3,4,5,6],"total":21,"save":"rod_staff_wand"}]}`, string(got.Payload))
	require.Len(t, got.Snapshots, 1)
	assert.Equal(t, "items:wand", got.Snapshots[0].Key)
	assert.Contains(t, snaptest.State(t, got.Snapshots[0].Before), `"limit":{"kind":"charges","max":4,"left":4,"cost":1}`)
	assert.Contains(t, snaptest.State(t, got.Snapshots[0].After), `"limit":{"kind":"charges","max":4,"left":3,"cost":1}`)
}

// ADR 0049 steps 6 and 12: the effects run in list order, each rolling its own dice (a modifier
// kept), and a failing effect rolls the whole use back (the entry is never written).
func TestUseAbility_ADR0049Step6_EffectsRunInListOrder(t *testing.T) {
	item := lightningWand(3)
	item.Abilities[0].Effects = []itemsvc.Effect{
		{Kind: itemsvc.EffectDice, Dice: "2d6", Label: "blast"},
		{Kind: itemsvc.EffectDice, Dice: "1d8+2", Label: "burn"},
	}
	dal := mocks.NewMockDAL(t)
	f := readsCharacter(t)
	f.dice.EXPECT().RollDie(6).Return(3).Once()
	f.dice.EXPECT().RollDie(6).Return(5).Once()
	f.dice.EXPECT().RollDie(8).Return(7).Once()
	dal.EXPECT().Get(mock.Anything, int64(1), "wand").Return(item, nil)
	dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil).Once()

	res, got, err := useOnce(t, f, dal, item, "use")
	require.NoError(t, err)
	require.Len(t, res.Effects, 2)
	assert.Equal(t, []int{3, 5}, res.Effects[0].Rolls)
	assert.Equal(t, 8, res.Effects[0].Total)
	assert.Equal(t, "burn", res.Effects[1].Label)
	assert.Equal(t, []int{7}, res.Effects[1].Rolls)
	assert.Equal(t, 2, res.Effects[1].Modifier, "the flat +2 of 1d8+2")
	assert.Equal(t, 9, res.Effects[1].Total)
	assert.Contains(t, string(got.Payload), `"dice":"2d6","rolls":[3,5],"total":8`, "the journal's flat fields are the first dice effect")
}

// ADR 0049 step 7: an ability with no effect (turn zombies) pays its limit, rolls nothing (the dice
// mock fails on any call) and shows the ability's note; the entry names the ability by its language map.
func TestUseAbility_ADR0049Step7_AnAbilityWithNoEffectOnlyPaysAndShowsTheNote(t *testing.T) {
	sword := zombieSword(2)
	dal := mocks.NewMockDAL(t)
	f := readsCharacter(t)
	dal.EXPECT().Get(mock.Anything, int64(1), "sword").Return(sword, nil)
	dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil).Once()

	res, got, err := useOnce(t, f, dal, sword, "turn")
	require.NoError(t, err)
	assert.Empty(t, res.Effects)
	assert.Equal(t, "Zombies within 30 feet", res.Notes)
	assert.Equal(t, localized.Text{"en": "Turn zombies"}, res.AbilityName)
	assert.JSONEq(t, `{"itemId":"sword","abilityId":"turn","name":"Sword of the dead","abilityName":{"en":"Turn zombies"},"type":"sword",
		"limit":"per_day","left":1,"max":2,"removed":false,"remaining":1,"maxCount":2,"rolls":[],"total":0,"notes":"Zombies within 30 feet"}`, string(got.Payload))
}

// ADR 0049 step 2 and 7: a potion's last dose, used through its with_item ability, removes the
// potion in the same entry; the snapshot carries the whole potion before and none after, so undo
// puts it back.
func TestUseAbility_ADR0049Step7_LastDoseRemovesThePotionAndUndoPutsItBack(t *testing.T) {
	dose := withUse(itemsvc.Item{
		ID: "potion", Position: 1, Name: localized.Of("Potion of healing"), Type: "potion", Category: itemsvc.CategoryMagic,
		MagicKind: itemsvc.MagicPotion, State: itemsvc.StateCarried, Count: 1,
	}, itemsvc.Limit{Kind: itemsvc.LimitWithItem})
	dal := mocks.NewMockDAL(t)
	f := readsCharacter(t)
	expectUse(dal, dose)

	res, got, err := useOnce(t, f, dal, dose, "use")
	require.NoError(t, err)
	assert.True(t, res.Removed)
	assert.Equal(t, 0, res.Remaining)
	assert.Contains(t, string(got.Payload), `"removed":true`)
	assert.True(t, snaptest.IsAbsent(t, got.Snapshots[0].After))

	back := mocks.NewMockDAL(t)
	backSvc, _ := itemsvc.New(back, deps(t).Deps)
	back.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(itemsvc.ErrItemNotFound).Once()
	var inserted itemsvc.Item
	back.EXPECT().Insert(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { inserted = it }).Return(nil).Once()
	back.EXPECT().Get(mock.Anything, int64(1), "potion").Return(itemsvc.Item{}, itemsvc.ErrItemNotFound).Maybe()
	require.NoError(t, backSvc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, got.Snapshots[0].Before))
	assert.Equal(t, 1, inserted.Count)
	assert.Equal(t, "potion", inserted.ID)
	assert.Equal(t, dose.Abilities, inserted.Abilities)
}

// ADR 0049 step 6: undo restores the counter the use paid, and redo pays it again: the snapshot's
// before and after are each the whole item, written back by the same restorer.
func TestUseAbility_ADR0049Step6_UndoAndRedoRestoreTheLimit(t *testing.T) {
	sword := zombieSword(2)
	dal := mocks.NewMockDAL(t)
	f := readsCharacter(t)
	dal.EXPECT().Get(mock.Anything, int64(1), "sword").Return(sword, nil)
	dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil).Once()
	_, got, err := useOnce(t, f, dal, sword, "turn")
	require.NoError(t, err)

	restore := func(state json.RawMessage) itemsvc.Item {
		d := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(d, readsCharacter(t).Deps)
		var written itemsvc.Item
		d.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { written = it }).Return(nil).Once()
		require.NoError(t, svc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, state))
		return written
	}
	undone := restore(got.Snapshots[0].Before)
	assert.Equal(t, 2, undone.Abilities[0].Limit.Left, "undo: the use is given back")
	assert.Equal(t, localized.Text{"en": "Turn zombies"}, undone.Abilities[0].Name)
	redone := restore(got.Snapshots[0].After)
	assert.Equal(t, 1, redone.Abilities[0].Limit.Left, "redo: paid again")
}

// ADR 0049 step 6: a failing use writes nothing it could not take back: the dice failing (an
// effect that does not parse) fails the use before the entry, and the transaction rolls the limit
// payment back (noTx here: no entry is written).
func TestUseAbility_ADR0049Step12_AFailingEffectWritesNoEntry(t *testing.T) {
	item := lightningWand(3)
	item.Abilities[0].Effects = []itemsvc.Effect{{Kind: itemsvc.EffectDice, Dice: "lots"}}
	dal := mocks.NewMockDAL(t)
	f := readsCharacter(t) // the log mock fails on any call
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "wand").Return(item, nil)
	dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil).Once()
	_, err := svc.UseAbility(context.Background(), 1, "wand", "use")
	require.Error(t, err)
}

// ADR 0049 step 6: the long rest refills every per_day ability to its max and nothing else: charges,
// with_item and unlimited are untouched, an ability already full writes nothing, and the list comes
// back whole.
func TestResetDaily_ADR0049Step6_RefillsPerDayOnly(t *testing.T) {
	gem := zombieSword(0)
	gem.ID = "gem"
	full := zombieSword(2)
	full.ID = "full"
	wand := lightningWand(1)
	flask := itemsvc.Item{
		ID: "flask", Category: itemsvc.CategoryGear, Count: 3,
		Abilities: []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}}},
	}
	plain := itemsvc.Item{ID: "plain", Category: itemsvc.CategoryGear, Count: 1}
	mixed := zombieSword(1)
	mixed.ID = "mixed"
	mixed.Abilities = append(mixed.Abilities, lightningWand(1).Abilities...)

	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{gem, full, wand, flask, plain, mixed}, nil)
	var written []itemsvc.Item
	dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { written = append(written, it) }).Return(nil).Twice()

	got, err := svc.ResetDaily(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, got, 6)
	require.Len(t, written, 2, "only the gem and the mixed item changed")
	assert.Equal(t, 2, got[0].Abilities[0].Limit.Left)
	assert.Equal(t, 2, got[5].Abilities[0].Limit.Left)
	assert.Equal(t, 1, got[5].Abilities[1].Limit.Left, "the charges ability of the same item is untouched")
	assert.Equal(t, 1, got[2].Abilities[0].Limit.Left, "a wand is never recharged")
	assert.Equal(t, 0, gem.Abilities[0].Limit.Left, "the list read is not mutated")
	assert.False(t, gem.SameCounters(got[0]))
	assert.True(t, full.SameCounters(got[1]))
	assert.True(t, wand.SameCounters(got[2]))
}

// ADR 0049 step 6: a failure refilling an item changes nothing the caller keeps (the error comes
// back, the rest's transaction rolls the other writes back).
func TestResetDaily_ADR0049Step6_AFailureIsReturned(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{zombieSword(0)}, nil)
	dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(assert.AnError).Once()
	_, err := svc.ResetDaily(context.Background(), 1)
	require.ErrorIs(t, err, assert.AnError)
}

// ADR 0049 steps 2 and 20: an entry logged before abilities counted a wand's charges or a gem's daily
// use in the item's count; popping it now writes that count into the ability's left (never above
// its max) and leaves the quantity alone. The payload is not rewritten: the pop reads it anew.
func TestRestore_ADR0049Step20_AnOldCountOnlyEntryGoesIntoTheCounter(t *testing.T) {
	tests := []struct {
		name  string
		item  itemsvc.Item
		state string
		want  int
	}{
		{"a wand's old use given back", lightningWand(2), `{"count":3}`, 3},
		{"a gem's old rest undone: back to used", zombieSword(2), `{"count":0}`, 0},
		{"above the max is clamped", lightningWand(2), `{"count":9}`, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t)
			f := deps(t)
			svc, _ := itemsvc.New(dal, f.Deps)
			dal.EXPECT().Get(mock.Anything, int64(1), tt.item.ID).Return(tt.item, nil)
			var written itemsvc.Item
			dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { written = it }).Return(nil).Once()
			require.NoError(t, svc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, snaptest.EnvID(tt.item.ID, tt.state)))
			assert.Equal(t, tt.want, written.Abilities[0].Limit.Left)
			assert.Equal(t, 1, written.Count, "the quantity is left alone")
			assert.Equal(t, tt.item.Abilities[0].Limit.Max, written.Abilities[0].Limit.Max)
			assert.Equal(t, tt.item.Abilities[0].Name, written.Abilities[0].Name)
		})
	}
}
