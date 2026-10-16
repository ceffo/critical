package itemsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

func ptr[T any](v T) *T { return &v }

// Validation: every bad field is rejected with a *domain.FieldError naming
// the wire field, before anything is stored or logged.
func TestCreate_ValidationNamesTheField(t *testing.T) {
	good := itemsvc.ItemInput{Name: "Arrows +3", Type: "arrow", Behaviour: itemsvc.BehaviourStock, Count: 6}
	cases := map[string]struct {
		mut   func(*itemsvc.ItemInput)
		field string
	}{
		"empty name":          {func(i *itemsvc.ItemInput) { i.Name = "  " }, "name"},
		"unknown behaviour":   {func(i *itemsvc.ItemInput) { i.Behaviour = "x" }, "behaviour"},
		"negative count":      {func(i *itemsvc.ItemInput) { i.Count = -1 }, "count"},
		"huge count":          {func(i *itemsvc.ItemInput) { i.Count = itemsvc.MaxItemCount + 1 }, "count"},
		"bad dice 2x6":        {func(i *itemsvc.ItemInput) { i.EffectDice = "2x6" }, "effectDice"},
		"bad dice 0d6":        {func(i *itemsvc.ItemInput) { i.EffectDice = "0d6" }, "effectDice"},
		"bad dice 1d1":        {func(i *itemsvc.ItemInput) { i.EffectDice = "1d1" }, "effectDice"},
		"bad dice trailing":   {func(i *itemsvc.ItemInput) { i.EffectDice = "2d6+" }, "effectDice"},
		"charges at zero":     {func(i *itemsvc.ItemInput) { i.Behaviour = itemsvc.BehaviourCharges; i.Count = 0 }, "count"},
		"unknown save":        {func(i *itemsvc.ItemInput) { i.EffectSave = "fire" }, "effectSave"},
		"bonus out of range":  {func(i *itemsvc.ItemInput) { i.MagicBonus = 11 }, "magicalBonus"},
		"daily without daily": {func(i *itemsvc.ItemInput) { i.Behaviour = itemsvc.BehaviourDaily }, "maxCount"},
		"daily count > daily": {func(i *itemsvc.ItemInput) { i.Behaviour = itemsvc.BehaviourDaily; i.MaxCount = 2; i.Count = 3 }, "count"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := itemsvc.New(mocks.NewMockDAL(t), deps(t).Deps) // nothing is touched
			in := good
			c.mut(&in)
			_, err := svc.Create(context.Background(), 1, in)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, c.field, fe.Field)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.NotEmpty(t, fe.Message)
		})
	}
}

func TestParseDice(t *testing.T) {
	for in, want := range map[string][3]int{"2d6": {2, 6, 0}, "1D8+2": {1, 8, 2}, " 3d4-1 ": {3, 4, -1}} {
		n, s, m, err := itemsvc.ParseDice(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, [3]int{n, s, m}, in)
	}
	for _, in := range []string{"", "d6", "2d", "2x6", "2d6+", "2d6+x", "0d6", "101d6", "1d1", "1d1001"} {
		_, _, _, err := itemsvc.ParseDice(in)
		assert.Error(t, err, in)
	}
}

// A new item gets a unique id from its name and the next sheet position, and
// one item_add entry whose snapshot says "did not exist" before.
func TestCreate_InsertsLogsAndSnapshots(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().List(mock.Anything).Return([]itemsvc.Item{
		{ID: "arrows_3", Position: 4}, {ID: "arrows_3_2", Position: 7},
	}, nil)
	want := itemsvc.Item{
		ID: "arrows_3_3", Position: 8, Name: "Arrows +3", Type: "arrow", Behaviour: itemsvc.BehaviourStock,
		MagicBonus: 3, AmmoKind: "arrow", Count: 6, MaxCount: 6,
	}
	dal.EXPECT().Insert(mock.Anything, want).Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(5), nil)

	// The id slug of "Arrows +3" is arrows_3; two of those exist already.
	res, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{
		Name: " Arrows +3 ", Type: "arrow", Behaviour: itemsvc.BehaviourStock, Count: 6, MagicBonus: 3, AmmoKind: "arrow",
	})
	require.NoError(t, err)
	assert.Equal(t, &itemsvc.ChangeResult{Item: want, LogEntryID: 5}, res)
	assert.Equal(t, actionlog.KindItemAdd, got.Kind)
	assert.False(t, actionlog.IsRoll(got.Kind))
	require.Len(t, got.Snapshots, 1)
	assert.Equal(t, "items:arrows_3_3", got.Snapshots[0].Key)
	assert.JSONEq(t, `[{"id":"arrows_3_3","count":0,"removed":true,"absent":true}]`, string(got.Snapshots[0].Before))
	assert.JSONEq(t, `[{"id":"arrows_3_3","count":6,"replace":true,"item":{"position":8,"name":"Arrows +3","type":"arrow",
		"behaviour":"stock","magicalBonus":3,"ammoKind":"arrow","maxCount":6,"effectDice":"","effectSave":"","notes":"","powers":[]}}]`, string(got.Snapshots[0].After))
	var p map[string]any
	require.NoError(t, json.Unmarshal(got.Payload, &p))
	assert.Equal(t, "Arrows +3", p["name"])
	assert.EqualValues(t, 6, p["count"])
}

var wand4 = itemsvc.Item{
	ID: "wand_lightning", Position: 4, Name: "Wand of lightning", Type: "wand", Behaviour: itemsvc.BehaviourCharges,
	Count: 4, MaxCount: 4, EffectDice: "6d6", EffectSave: "rod_staff_wand",
}

// Editing the wand 4 -> 3: one item_edit entry with both whole items, the
// changed field listed, only the patched field touched.
func TestUpdate_ChangesOneFieldAndLogsBeforeAfter(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, "wand_lightning").Return(wand4, nil)
	after := wand4
	after.Count = 3
	dal.EXPECT().Update(mock.Anything, after).Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(8), nil)

	res, err := svc.Update(context.Background(), 1, "wand_lightning", itemsvc.ItemPatch{Count: ptr(3)})
	require.NoError(t, err)
	assert.Equal(t, after, res.Item)
	assert.Equal(t, actionlog.KindItemEdit, got.Kind)
	assert.Empty(t, got.Summary, "the UI words the change from its payload")
	assert.Contains(t, string(got.Payload), `"changes":[{"field":"count","before":"4","after":"3"}]`)
	assert.JSONEq(t, `[{"id":"wand_lightning","count":4,"replace":true,"item":{"position":4,"name":"Wand of lightning","type":"wand",
		"behaviour":"charges","magicalBonus":0,"ammoKind":"","maxCount":4,"effectDice":"6d6","effectSave":"rod_staff_wand","notes":"","powers":[]}}]`,
		string(got.Snapshots[0].Before))
	assert.Contains(t, string(got.Snapshots[0].After), `"count":3`)
	assert.Contains(t, string(got.Payload), `"changes":[{"field":"count","before":"4","after":"3"}]`)
}

// A stock item turned daily keeps its count and takes the daily number sent with the change.
func TestUpdate_StockToDailyKeepsCountAndTakesDailyNumber(t *testing.T) {
	flask := itemsvc.Item{ID: "fire_flask", Position: 1, Name: "Fire flasks", Type: "flask", Behaviour: itemsvc.BehaviourStock, Count: 5, MaxCount: 1, EffectDice: "5d6"}
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(9), nil)
	dal.EXPECT().Get(mock.Anything, "fire_flask").Return(flask, nil)
	after := flask
	after.Behaviour = itemsvc.BehaviourDaily
	after.MaxCount = 5
	dal.EXPECT().Update(mock.Anything, after).Return(nil).Once()

	res, err := svc.Update(context.Background(), 1, "fire_flask", itemsvc.ItemPatch{Behaviour: ptr(itemsvc.BehaviourDaily), MaxCount: ptr(5)})
	require.NoError(t, err)
	assert.Equal(t, 5, res.Item.Count)
	assert.Equal(t, 5, res.Item.MaxCount)
	assert.Equal(t, itemsvc.BehaviourDaily, res.Item.Behaviour)
}

func TestUpdate_RejectsBadValuesAndNoOps(t *testing.T) {
	for name, c := range map[string]struct {
		patch itemsvc.ItemPatch
		field string
	}{
		"empty name":         {itemsvc.ItemPatch{Name: ptr("")}, "name"},
		"negative count":     {itemsvc.ItemPatch{Count: ptr(-1)}, "count"},
		"bad dice":           {itemsvc.ItemPatch{EffectDice: ptr("2x6")}, "effectDice"},
		"daily 0":            {itemsvc.ItemPatch{Behaviour: ptr(itemsvc.BehaviourDaily), MaxCount: ptr(0)}, "maxCount"},
		"charges to 0":       {itemsvc.ItemPatch{Count: ptr(0)}, "count"},
		"stock 0 to charges": {itemsvc.ItemPatch{Behaviour: ptr(itemsvc.BehaviourCharges), Count: ptr(0)}, "count"},
		"nothing":            {itemsvc.ItemPatch{Count: ptr(4)}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t) // no Update
			svc, _ := itemsvc.New(dal, deps(t).Deps)
			dal.EXPECT().Get(mock.Anything, "wand_lightning").Return(wand4, nil)
			_, err := svc.Update(context.Background(), 1, "wand_lightning", c.patch)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			var fe *domain.FieldError
			if c.field != "" {
				require.ErrorAs(t, err, &fe)
				assert.Equal(t, c.field, fe.Field)
			}
		})
	}
}

func TestUpdate_UnknownItem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Get(mock.Anything, "x").Return(itemsvc.Item{}, itemsvc.ErrItemNotFound)
	_, err := svc.Update(context.Background(), 1, "x", itemsvc.ItemPatch{Count: ptr(1)})
	assert.ErrorIs(t, err, itemsvc.ErrItemNotFound)
}

func TestDelete_RemovesAndLogsWholeItem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, "wand_lightning").Return(wand4, nil)
	dal.EXPECT().Delete(mock.Anything, "wand_lightning").Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(2), nil)

	res, err := svc.Delete(context.Background(), 1, "wand_lightning")
	require.NoError(t, err)
	assert.Equal(t, wand4, res.Item)
	assert.Equal(t, actionlog.KindItemRemove, got.Kind)
	assert.Contains(t, string(got.Snapshots[0].Before), `"replace":true`)
	assert.Contains(t, string(got.Snapshots[0].Before), `"count":4`)
	assert.JSONEq(t, `[{"id":"wand_lightning","count":0,"removed":true,"absent":true}]`, string(got.Snapshots[0].After))
}

// Restoring: an item added by hand is deleted, an edited one overwritten
// whole, a deleted one inserted back whole. These snapshots predate powers
// (no "powers" key): the item keeps the powers it has now.
func TestRestorers_UndoCreateEditDelete(t *testing.T) {
	restore := func(dal *mocks.MockDAL, before json.RawMessage) error {
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		return svc.Restorers()[itemsvc.StateKeyItems].Restore(context.Background(), 1, before)
	}
	t.Run("create", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Delete(mock.Anything, "new").Return(nil).Once()
		require.NoError(t, restore(dal, json.RawMessage(`[{"id":"new","count":0,"removed":true,"absent":true}]`)))
	})
	before := json.RawMessage(`[{"id":"wand_lightning","count":4,"replace":true,"item":{"position":4,"name":"Wand of lightning","type":"wand",
		"behaviour":"charges","magicalBonus":0,"ammoKind":"","maxCount":4,"effectDice":"6d6","effectSave":"rod_staff_wand","notes":""}}]`)
	t.Run("edit", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Get(mock.Anything, "wand_lightning").Return(wand4, nil).Once()
		dal.EXPECT().Update(mock.Anything, wand4).Return(nil).Once()
		require.NoError(t, restore(dal, before))
	})
	t.Run("delete", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Get(mock.Anything, "wand_lightning").Return(itemsvc.Item{}, itemsvc.ErrItemNotFound).Once()
		dal.EXPECT().Update(mock.Anything, wand4).Return(itemsvc.ErrItemNotFound).Once()
		dal.EXPECT().Insert(mock.Anything, wand4).Return(nil).Once()
		require.NoError(t, restore(dal, before))
	})
}

// Dice may carry a flat modifier: 1d8+2 rolls one die and adds 2.
func TestUseItem_DiceWithModifier(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Decrement(mock.Anything, "x").Return(itemsvc.Item{ID: "x", Name: "X", Behaviour: itemsvc.BehaviourStock, Count: 1, EffectDice: "1d8+2"}, nil)
	f.dice.EXPECT().RollDie(8).Return(5).Once()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
	res, err := svc.UseItem(context.Background(), 1, "x")
	require.NoError(t, err)
	assert.Equal(t, []int{5}, res.Rolls)
	assert.Equal(t, 7, res.Total)
	assert.Equal(t, 2, res.Modifier, "the flat +2 of 1d8+2")
}
