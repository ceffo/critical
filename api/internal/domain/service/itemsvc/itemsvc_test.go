package itemsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

func TestNew_NilDAL(t *testing.T) {
	_, err := itemsvc.New(nil, itemsvc.Deps{})
	require.Error(t, err)
}

func TestUse_ReturnsUpdatedItem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, err := itemsvc.NewWithConfig(itemsvc.DefaultConfig(), dal, deps(t).Deps)
	require.NoError(t, err)
	dal.EXPECT().Decrement(mock.Anything, int64(1), "wand").Return(itemsvc.Item{ID: "wand", Count: 3}, nil)

	got, err := svc.Use(context.Background(), 1, "wand")
	require.NoError(t, err)
	assert.Equal(t, 3, got.Count)
}

func TestUse_PropagatesSentinels(t *testing.T) {
	for _, target := range []error{itemsvc.ErrItemEmpty, itemsvc.ErrItemNotFound} {
		dal := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		dal.EXPECT().Decrement(mock.Anything, int64(1), "x").Return(itemsvc.Item{}, fmt.Errorf("%w: x", target))
		_, err := svc.Use(context.Background(), 1, "x")
		assert.True(t, errors.Is(err, target))
	}
}

func TestResetDaily_ResetsThenLists(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().ResetDaily(mock.Anything, int64(1)).Return(nil)
	dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{{ID: "gem", Count: 1}}, nil)

	got, err := svc.ResetDaily(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []itemsvc.Item{{ID: "gem", Count: 1}}, got)
}

func TestResetDaily_ResetErrorSkipsList(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().ResetDaily(mock.Anything, int64(1)).Return(errors.New("boom"))
	_, err := svc.ResetDaily(context.Background(), 1)
	require.Error(t, err)
}

func TestBehaviour_Rules(t *testing.T) {
	tests := []struct {
		b                    itemsvc.Behaviour
		valid, restock, gone bool
	}{
		{itemsvc.BehaviourStock, true, true, false},
		{itemsvc.BehaviourCharges, true, false, true},
		{itemsvc.BehaviourDaily, true, false, false},
		{"charged", false, false, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.b), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.b.Valid())
			assert.Equal(t, tt.restock, tt.b.Restockable())
			assert.Equal(t, tt.gone, tt.b.RemovedAtZero())
		})
	}
}

// Use at 0: a charges item is removed, a stock or daily item stays listed.
func TestUse_RemovesChargesItemAtZeroOnly(t *testing.T) {
	tests := []struct {
		name    string
		item    itemsvc.Item
		removed bool
	}{
		{"wand spends its last charge", itemsvc.Item{ID: "x", Behaviour: itemsvc.BehaviourCharges, Count: 0}, true},
		{"wand with charges left", itemsvc.Item{ID: "x", Behaviour: itemsvc.BehaviourCharges, Count: 1}, false},
		{"arrows at 0 stay", itemsvc.Item{ID: "x", Behaviour: itemsvc.BehaviourStock, Count: 0}, false},
		{"gem at 0 stays", itemsvc.Item{ID: "x", Behaviour: itemsvc.BehaviourDaily, Count: 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t) // Delete fails the test unless expected
			svc, _ := itemsvc.New(dal, deps(t).Deps)
			dal.EXPECT().Decrement(mock.Anything, int64(1), "x").Return(tt.item, nil)
			if tt.removed {
				dal.EXPECT().Delete(mock.Anything, int64(1), "x").Return(nil).Once()
			}
			got, err := svc.Use(context.Background(), 1, "x")
			require.NoError(t, err)
			assert.Equal(t, tt.removed, got.Removed())
			assert.Equal(t, tt.item.Count, got.Count)
		})
	}
}

func TestUse_RemoveErrorFails(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Decrement(mock.Anything, int64(1), "x").Return(itemsvc.Item{ID: "x", Behaviour: itemsvc.BehaviourCharges}, nil)
	dal.EXPECT().Delete(mock.Anything, int64(1), "x").Return(errors.New("boom"))
	_, err := svc.Use(context.Background(), 1, "x")
	require.Error(t, err)
}

var wand = itemsvc.Item{
	ID: "wand_lightning", Position: 4, Name: localized.Of("Wand of lightning"), Type: "wand", Behaviour: itemsvc.BehaviourCharges,
	Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicWand, State: itemsvc.StateCarried,
	Count: 0, MaxCount: 4, EffectDice: "6d6", EffectSave: "rod_staff_wand",
}

const wandBefore = `{"count":1,"item":{"position":4,"name":{"en":"Wand of lightning"},"type":"wand",
	"behaviour":"charges","magicalBonus":0,"ammoKind":"","maxCount":4,"effectDice":"6d6","effectSave":"rod_staff_wand","notes":"","powers":[],
	"piece":{"category":"magic","magicKind":"wand","state":"carried"}}}`

func TestUseSnapshot_BeforeIsOneMore(t *testing.T) {
	for _, b := range []itemsvc.Behaviour{itemsvc.BehaviourStock, itemsvc.BehaviourDaily} {
		snap, err := itemsvc.UseSnapshot(itemsvc.Item{ID: "arrows_plus3", Name: localized.Of("Arrows +3"), Behaviour: b, Count: 0})
		require.NoError(t, err)
		assert.Equal(t, "items:arrows_plus3", snap.Key)
		assert.Equal(t, "arrows_plus3", snaptest.ID(t, snap.Before))
		assert.JSONEq(t, `{"count":1}`, snaptest.State(t, snap.Before), "kept at 0: no item data")
		assert.JSONEq(t, `{"count":0}`, snaptest.State(t, snap.After))
	}
}

func TestUseSnapshot_MarksARemovedItem(t *testing.T) {
	snap, err := itemsvc.UseSnapshot(wand)
	require.NoError(t, err)
	assert.JSONEq(t, wandBefore, snaptest.State(t, snap.Before))
	assert.Equal(t, snaptest.Absent("wand_lightning"), snap.After, "the use removed it")
}

func TestRestorers_WriteTheBeforeCountsBack(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	snap, err := itemsvc.UseSnapshot(itemsvc.Item{ID: "arrows_plus3", Count: 0})
	require.NoError(t, err)

	dal.EXPECT().SetCount(mock.Anything, int64(1), "arrows_plus3", 1).Return(nil).Once()
	r := svc.Restorers()[itemsvc.StateKeyItems]
	require.NotNil(t, r)
	require.NoError(t, r.Restore(context.Background(), 1, snap.Before))
}

// Popping the use that removed a wand inserts it back whole, with 1 charge.
func TestRestorers_PutBackARemovedItem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	snap, err := itemsvc.UseSnapshot(wand)
	require.NoError(t, err)

	dal.EXPECT().SetCount(mock.Anything, int64(1), "wand_lightning", 1).Return(fmt.Errorf("%w: wand_lightning", itemsvc.ErrItemNotFound)).Once()
	back := wand
	back.Count = 1
	dal.EXPECT().Insert(mock.Anything, int64(1), back).Return(nil).Once()
	require.NoError(t, svc.Restorers()[itemsvc.StateKeyItems].Restore(context.Background(), 1, snap.Before))
}

// A count-only state (a use that kept the item) cannot recreate a missing item.
func TestRestorers_MissingItemWithoutDataFails(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().SetCount(mock.Anything, int64(1), "x", 2).Return(fmt.Errorf("%w: x", itemsvc.ErrItemNotFound))
	err := svc.Restorers()[itemsvc.StateKeyItems].Restore(context.Background(), 1, snaptest.EnvID("x", `{"count":2}`))
	assert.ErrorIs(t, err, itemsvc.ErrItemNotFound)
}

func TestRestorers_RejectBadSnapshots(t *testing.T) {
	for _, state := range []json.RawMessage{
		snaptest.EnvID("x", `[{"count":1}]`), snaptest.Env(`{"count":1}`), snaptest.EnvID("x", `{"count":-1}`), snaptest.Absent(""),
	} {
		dal := mocks.NewMockDAL(t) // nothing is written
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		err := svc.Restorers()[itemsvc.StateKeyItems].Restore(context.Background(), 1, state)
		assert.Error(t, err, string(state))
	}
}

// fixture holds the collaborators UseItem needs besides the DAL.
type fixture struct {
	itemsvc.Deps
	dice  *mocks.MockDice
	log   *mocks.MockLog
	chars *mocks.MockCharacters
}

// noTx runs the unit of work directly.
type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func deps(t *testing.T) fixture {
	dice, log, chars := mocks.NewMockDice(t), mocks.NewMockLog(t), mocks.NewMockCharacters(t)
	return fixture{Deps: itemsvc.Deps{Dice: dice, Log: log, Tx: noTx{}, Chars: chars}, dice: dice, log: log, chars: chars}
}

func TestUseItem_RollsEffectDiceAndLogsWithSnapshot(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, err := itemsvc.New(dal, f.Deps)
	require.NoError(t, err)
	dal.EXPECT().Decrement(mock.Anything, int64(1), "wand_lightning").Return(itemsvc.Item{
		ID: "wand_lightning", Name: localized.Of("Wand of lightning"), Type: "wand", Behaviour: itemsvc.BehaviourCharges, Count: 3, MaxCount: 4,
		EffectDice: "6d6", EffectSave: "rod_staff_wand",
	}, nil)
	for _, v := range []int{1, 2, 3, 4, 5, 6} {
		f.dice.EXPECT().RollDie(6).Return(v).Once()
	}
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(9), nil)

	res, err := svc.UseItem(context.Background(), 1, "wand_lightning")
	require.NoError(t, err)
	assert.Equal(t, 3, res.Remaining)
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6}, res.Rolls)
	assert.Equal(t, 21, res.Total)
	assert.Equal(t, "rod_staff_wand", res.Save)
	assert.Zero(t, res.Modifier)
	assert.Equal(t, int64(9), res.LogEntryID)

	assert.Equal(t, actionlog.KindItemUse, got.Kind)
	require.Len(t, got.Snapshots, 1)
	assert.Equal(t, "items:wand_lightning", got.Snapshots[0].Key)
	assert.JSONEq(t, `{"count":4}`, snaptest.State(t, got.Snapshots[0].Before))
	assert.JSONEq(t, `{"count":3}`, snaptest.State(t, got.Snapshots[0].After))
	assert.JSONEq(t, `{"itemId":"wand_lightning","name":"Wand of lightning","type":"wand","behaviour":"charges","removed":false,"remaining":3,"maxCount":4,
		"dice":"6d6","rolls":[1,2,3,4,5,6],"total":21,"save":"rod_staff_wand"}`, string(got.Payload))
}

func TestUseItem_NoDiceRollsNothing(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t) // the dice mock fails the test on any call
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Decrement(mock.Anything, int64(1), "gem").Return(itemsvc.Item{ID: "gem", Name: localized.Of("Gem"), Behaviour: itemsvc.BehaviourDaily, Count: 0, MaxCount: 1}, nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)

	res, err := svc.UseItem(context.Background(), 1, "gem")
	require.NoError(t, err)
	assert.Empty(t, res.Rolls)
	assert.Zero(t, res.Total)
	assert.Equal(t, 0, res.Remaining)
}

func TestUseItem_EmptyAndUnknownLogNothing(t *testing.T) {
	for _, target := range []error{itemsvc.ErrItemEmpty, itemsvc.ErrItemNotFound} {
		dal := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(dal, deps(t).Deps) // log and dice mocks fail on any call
		dal.EXPECT().Decrement(mock.Anything, int64(1), "x").Return(itemsvc.Item{}, fmt.Errorf("%w: x", target))
		_, err := svc.UseItem(context.Background(), 1, "x")
		assert.True(t, errors.Is(err, target))
	}
}

// The inventory's − spends arrows too (lost, broken): any behaviour is usable.
func TestUseItem_SpendsStockAmmo(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Decrement(mock.Anything, int64(1), "arrows").Return(itemsvc.Item{ID: "arrows", Name: localized.Of("Arrows"), Behaviour: itemsvc.BehaviourStock, AmmoKind: "arrow", Count: 0}, nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(3), nil)
	res, err := svc.UseItem(context.Background(), 1, "arrows")
	require.NoError(t, err)
	assert.Equal(t, 0, res.Remaining)
	assert.False(t, res.Removed, "stock stays listed at 0")
}

// The use that spends a wand's last charge removes it in the same entry,
// whose snapshot carries the whole wand so popping it brings it back.
func TestUseItem_LastChargeRemovesTheItem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Decrement(mock.Anything, int64(1), "wand_lightning").Return(wand, nil)
	dal.EXPECT().Delete(mock.Anything, int64(1), "wand_lightning").Return(nil).Once()
	for _, v := range []int{6, 5, 4, 3, 2, 1} {
		f.dice.EXPECT().RollDie(6).Return(v).Once()
	}
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(5), nil)

	res, err := svc.UseItem(context.Background(), 1, "wand_lightning")
	require.NoError(t, err)
	assert.True(t, res.Removed)
	assert.Equal(t, 0, res.Remaining)
	assert.Empty(t, got.Summary, "the UI words the use from its payload")
	require.Len(t, got.Snapshots, 1)
	assert.JSONEq(t, wandBefore, snaptest.State(t, got.Snapshots[0].Before))
	assert.True(t, snaptest.IsAbsent(t, got.Snapshots[0].After))
	assert.Contains(t, string(got.Payload), `"removed":true`)
}

func TestRestock_AddsToStockAndLogs(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Increment(mock.Anything, int64(1), "arrows_plus3", 1).Return(itemsvc.Item{
		ID: "arrows_plus3", Name: localized.Of("Arrows +3"), Type: "arrow", Behaviour: itemsvc.BehaviourStock, Count: 1,
	}, nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(7), nil)

	res, err := svc.Restock(context.Background(), 1, "arrows_plus3", 1)
	require.NoError(t, err)
	assert.Equal(t, &itemsvc.RestockResult{ItemID: "arrows_plus3", Name: "Arrows +3", Type: "arrow", Added: 1, Count: 1, LogEntryID: 7}, res)
	assert.Equal(t, actionlog.KindItemRestock, got.Kind)
	assert.False(t, actionlog.IsRoll(got.Kind), "a restock leaves the result card alone")
	assert.JSONEq(t, `{"itemId":"arrows_plus3","name":"Arrows +3","type":"arrow","added":1,"count":1}`, string(got.Payload))
	require.Len(t, got.Snapshots, 1)
	assert.Equal(t, "items:arrows_plus3", got.Snapshots[0].Key)
	assert.JSONEq(t, `{"count":0}`, snaptest.State(t, got.Snapshots[0].Before))
	assert.JSONEq(t, `{"count":1}`, snaptest.State(t, got.Snapshots[0].After))
	assert.JSONEq(t, `{"itemId":"arrows_plus3","name":"Arrows +3","type":"arrow","added":1,"count":1}`, string(got.Payload))
}

// Charges and daily items are only used: the restock is refused and nothing
// is logged (the transaction rolls the increment back).
func TestRestock_RejectsChargesAndDaily(t *testing.T) {
	for _, b := range []itemsvc.Behaviour{itemsvc.BehaviourCharges, itemsvc.BehaviourDaily} {
		t.Run(string(b), func(t *testing.T) {
			dal := mocks.NewMockDAL(t)
			svc, _ := itemsvc.New(dal, deps(t).Deps) // log mock fails on any call
			dal.EXPECT().Increment(mock.Anything, int64(1), "x", 1).Return(itemsvc.Item{ID: "x", Behaviour: b, Count: 2}, nil)
			_, err := svc.Restock(context.Background(), 1, "x", 1)
			assert.ErrorIs(t, err, itemsvc.ErrNotRestockable)
		})
	}
}

func TestRestock_RejectsOutOfRangeCounts(t *testing.T) {
	for _, n := range []int{0, -1, itemsvc.MaxRestock + 1} {
		dal := mocks.NewMockDAL(t) // nothing is written
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		_, err := svc.Restock(context.Background(), 1, "x", n)
		assert.ErrorIs(t, err, domain.ErrInvalidInput, n)
	}
}

func TestRestock_UnknownItem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Increment(mock.Anything, int64(1), "nope", 2).Return(itemsvc.Item{}, fmt.Errorf("%w: nope", itemsvc.ErrItemNotFound))
	_, err := svc.Restock(context.Background(), 1, "nope", 2)
	assert.ErrorIs(t, err, itemsvc.ErrItemNotFound)
}

// ADR 0033 step 1.1: a spell spends a piece by its component code, never by
// the free-text type label, whatever its category.
func TestItem_Step1_1_Spendable(t *testing.T) {
	base := itemsvc.Item{Component: itemsvc.ComponentGem, Category: itemsvc.CategoryGear, Behaviour: itemsvc.BehaviourStock, Count: 1}
	tests := []struct {
		name string
		mod  func(*itemsvc.Item)
		want bool
	}{
		{"stock with some left", func(*itemsvc.Item) {}, true},
		{"charges with some left", func(i *itemsvc.Item) { i.Behaviour = itemsvc.BehaviourCharges }, true},
		{"any category", func(i *itemsvc.Item) { i.Category, i.MagicKind = itemsvc.CategoryMagic, itemsvc.MagicMisc }, true},
		{"no component", func(i *itemsvc.Item) { i.Component = "" }, false},
		{"the type label alone", func(i *itemsvc.Item) { i.Component, i.Type = "", "gem" }, false},
		{"daily item", func(i *itemsvc.Item) { i.Behaviour = itemsvc.BehaviourDaily }, false},
		{"none left", func(i *itemsvc.Item) { i.Count = 0 }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it := base
			tt.mod(&it)
			assert.Equal(t, tt.want, it.Spendable("gem"))
		})
	}
}
