package itemsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNew_NilDAL(t *testing.T) {
	_, err := itemsvc.New(nil, itemsvc.Deps{})
	require.Error(t, err)
}

func TestUse_ReturnsUpdatedItem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, err := itemsvc.NewWithConfig(itemsvc.DefaultConfig(), dal, deps(t).Deps)
	require.NoError(t, err)
	dal.EXPECT().Decrement(mock.Anything, "wand").Return(itemsvc.Item{ID: "wand", Count: 3}, nil)

	got, err := svc.Use(context.Background(), "wand")
	require.NoError(t, err)
	assert.Equal(t, 3, got.Count)
}

func TestUse_PropagatesSentinels(t *testing.T) {
	for _, target := range []error{itemsvc.ErrItemEmpty, itemsvc.ErrItemNotFound} {
		dal := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		dal.EXPECT().Decrement(mock.Anything, "x").Return(itemsvc.Item{}, fmt.Errorf("%w: x", target))
		_, err := svc.Use(context.Background(), "x")
		assert.True(t, errors.Is(err, target))
	}
}

func TestResetDaily_ResetsThenLists(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().ResetPerDay(mock.Anything).Return(nil)
	dal.EXPECT().List(mock.Anything).Return([]itemsvc.Item{{ID: "gem", Count: 1}}, nil)

	got, err := svc.ResetDaily(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []itemsvc.Item{{ID: "gem", Count: 1}}, got)
}

func TestResetDaily_ResetErrorSkipsList(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().ResetPerDay(mock.Anything).Return(errors.New("boom"))
	_, err := svc.ResetDaily(context.Background())
	require.Error(t, err)
}

func TestUseSnapshot_BeforeIsOneMore(t *testing.T) {
	snap, err := itemsvc.UseSnapshot(itemsvc.Item{ID: "arrows_plus3", Count: 1})
	require.NoError(t, err)
	assert.Equal(t, "items:arrows_plus3", snap.Key)
	assert.JSONEq(t, `[{"id":"arrows_plus3","count":2}]`, string(snap.Before))
	assert.JSONEq(t, `[{"id":"arrows_plus3","count":1}]`, string(snap.After))
}

func TestRestorers_WriteTheBeforeCountsBack(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	snap, err := itemsvc.UseSnapshot(itemsvc.Item{ID: "arrows_plus3", Count: 0})
	require.NoError(t, err)

	dal.EXPECT().SetCount(mock.Anything, "arrows_plus3", 1).Return(nil).Once()
	r := svc.Restorers()[itemsvc.StateKeyItems]
	require.NotNil(t, r)
	require.NoError(t, r.Restore(context.Background(), 1, snap.Before))
}

func TestRestorers_RejectBadSnapshots(t *testing.T) {
	for _, state := range []string{`{"id":"x"}`, `[{"id":"","count":1}]`, `[{"id":"x","count":-1}]`} {
		dal := mocks.NewMockDAL(t) // nothing is written
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		err := svc.Restorers()[itemsvc.StateKeyItems].Restore(context.Background(), 1, json.RawMessage(state))
		assert.Error(t, err, state)
	}
}

// fixture holds the collaborators UseItem needs besides the DAL.
type fixture struct {
	itemsvc.Deps
	dice *mocks.MockDice
	log  *mocks.MockLog
}

// noTx runs the unit of work directly.
type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func deps(t *testing.T) fixture {
	dice, log := mocks.NewMockDice(t), mocks.NewMockLog(t)
	return fixture{Deps: itemsvc.Deps{Dice: dice, Log: log, Tx: noTx{}}, dice: dice, log: log}
}

func TestUseItem_RollsEffectDiceAndLogsWithSnapshot(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, err := itemsvc.New(dal, f.Deps)
	require.NoError(t, err)
	dal.EXPECT().Decrement(mock.Anything, "wand_lightning").Return(itemsvc.Item{
		ID: "wand_lightning", Name: "Wand of lightning", Kind: itemsvc.KindCharged, Count: 3, MaxCount: 4,
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
	assert.Equal(t, "6d6: 1 + 2 + 3 + 4 + 5 + 6 = 21", res.Breakdown)
	assert.Equal(t, int64(9), res.LogEntryID)

	assert.Equal(t, actionlog.KindItemUse, got.Kind)
	require.Len(t, got.Snapshots, 1)
	assert.Equal(t, "items:wand_lightning", got.Snapshots[0].Key)
	assert.JSONEq(t, `[{"id":"wand_lightning","count":4}]`, string(got.Snapshots[0].Before))
	assert.JSONEq(t, `[{"id":"wand_lightning","count":3}]`, string(got.Snapshots[0].After))
	assert.JSONEq(t, `{"itemId":"wand_lightning","name":"Wand of lightning","kind":"charged","remaining":3,"maxCount":4,
		"dice":"6d6","rolls":[1,2,3,4,5,6],"total":21,"save":"rod_staff_wand","breakdown":"6d6: 1 + 2 + 3 + 4 + 5 + 6 = 21"}`, string(got.Payload))
}

func TestUseItem_NoDiceRollsNothing(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t) // the dice mock fails the test on any call
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Decrement(mock.Anything, "gem").Return(itemsvc.Item{ID: "gem", Name: "Gem", Kind: itemsvc.KindCharged, Count: 0, MaxCount: 1}, nil)
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
		dal.EXPECT().Decrement(mock.Anything, "x").Return(itemsvc.Item{}, fmt.Errorf("%w: x", target))
		_, err := svc.UseItem(context.Background(), 1, "x")
		assert.True(t, errors.Is(err, target))
	}
}

func TestUseItem_AmmoIsRefused(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Decrement(mock.Anything, "arrows").Return(itemsvc.Item{ID: "arrows", Kind: itemsvc.KindAmmo, Count: 1}, nil)
	_, err := svc.UseItem(context.Background(), 1, "arrows")
	assert.True(t, errors.Is(err, domain.ErrInvalidInput))
}
