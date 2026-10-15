package dicesvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const charID int64 = 1

type countingTx struct{ calls int }

func (c *countingTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	c.calls++
	return fn(ctx)
}

type fixture struct {
	svc     dicesvc.SVC
	dal     *mocks.MockDAL
	dice    *mocks.MockDice
	log     *mocks.MockLog
	tx      *countingTx
	entries []actionlog.NewEntry
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{dal: mocks.NewMockDAL(t), dice: mocks.NewMockDice(t), log: mocks.NewMockLog(t), tx: &countingTx{}}
	svc, err := dicesvc.New(dicesvc.Deps{DAL: f.dal, Dice: f.dice, Log: f.log, Tx: f.tx})
	require.NoError(t, err)
	f.svc = svc
	return f
}

func (f *fixture) records() {
	f.log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, e actionlog.NewEntry) (int64, error) {
			f.entries = append(f.entries, e)
			return int64(99 + len(f.entries)), nil
		})
}

func TestNew_RefusesNilDependencies(t *testing.T) {
	_, err := dicesvc.New(dicesvc.Deps{})
	require.Error(t, err)
}

// Every die of the formula is one RollDie call, group by group in order of
// first appearance, each with a distinct value: a reused or missing roll
// breaks the totals and the exact call counts.
func TestRoll_OneCallPerDieInOrder(t *testing.T) {
	f := newFixture(t)
	f.records()
	f.dice.EXPECT().RollDie(6).Return(2).Once()
	f.dice.EXPECT().RollDie(6).Return(5).Once()
	f.dice.EXPECT().RollDie(4).Return(3).Once()
	f.dice.EXPECT().RollDie(100).Return(41).Once()

	res, err := f.svc.Roll(context.Background(), charID, "2d6 + d% - d4 + 3 - 1")
	require.NoError(t, err)

	assert.Equal(t, "2d6+d%-d4+2", res.Formula)
	assert.Equal(t, []dicesvc.RolledGroup{
		{Sides: 6, Rolls: []int{2, 5}, Subtotal: 7},
		{Sides: 100, Rolls: []int{41}, Subtotal: 41},
		{Sides: 4, Rolls: []int{3}, Negative: true, Subtotal: -3},
	}, res.Groups)
	assert.Equal(t, 2, res.Modifier)
	assert.Equal(t, 7+41-3+2, res.Total)
	assert.Equal(t, int64(100), res.LogEntryID)
	assert.Equal(t, 1, f.tx.calls, "the entry is recorded in one transaction")
}

func TestRoll_RecordsADiceEntryWithTheBreakdown(t *testing.T) {
	f := newFixture(t)
	f.records()
	f.dice.EXPECT().RollDie(20).Return(14).Once()

	_, err := f.svc.Roll(context.Background(), charID, "d20+5")
	require.NoError(t, err)

	require.Len(t, f.entries, 1)
	e := f.entries[0]
	assert.Equal(t, actionlog.KindDice, e.Kind)
	assert.Equal(t, charID, e.CharacterID)
	assert.Empty(t, e.Snapshots, "a free roll changes no state")
	assert.Empty(t, e.Summary, "the backend composes no text")
	assert.JSONEq(t, `{"formula":"d20+5","groups":[{"sides":20,"rolls":[14],"negative":false,"subtotal":14}],
		"modifier":5,"total":19}`, string(e.Payload))
}

func TestRoll_InvalidFormulaRollsAndRecordsNothing(t *testing.T) {
	f := newFixture(t) // the mocks fail the test on any dice or log call
	_, err := f.svc.Roll(context.Background(), charID, "2d6*3")
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Zero(t, f.tx.calls)
}

func TestRoll_LogFailureIsReturned(t *testing.T) {
	f := newFixture(t)
	f.dice.EXPECT().RollDie(6).Return(3).Once()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(0, errors.New("disk full"))
	_, err := f.svc.Roll(context.Background(), charID, "d6")
	require.ErrorContains(t, err, "disk full")
}

func TestRoll_PayloadIsDecodable(t *testing.T) {
	f := newFixture(t)
	f.records()
	f.dice.EXPECT().RollDie(8).Return(8).Once()
	_, err := f.svc.Roll(context.Background(), charID, "d8")
	require.NoError(t, err)
	var back map[string]any
	require.NoError(t, json.Unmarshal(f.entries[0].Payload, &back))
	assert.Equal(t, "d8", back["formula"])
}

func TestAddFavourite_SavesTheNormalForm(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().List(mock.Anything, charID).Return(nil, nil).Once()
	f.dal.EXPECT().Insert(mock.Anything, charID, "2d6+d4+1").Return(7, nil).Once()
	got, err := f.svc.AddFavourite(context.Background(), charID, "d6 + d4 + d6 + 1")
	require.NoError(t, err)
	assert.Equal(t, &dicesvc.Favourite{ID: 7, Formula: "2d6+d4+1"}, got)
	assert.Equal(t, 1, f.tx.calls)
}

func TestAddFavourite_AlreadySavedReturnsTheExistingOne(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().List(mock.Anything, charID).Return([]dicesvc.Favourite{{ID: 3, Formula: "d20+5"}}, nil).Once()
	got, err := f.svc.AddFavourite(context.Background(), charID, "1d20 + 5")
	require.NoError(t, err)
	assert.Equal(t, &dicesvc.Favourite{ID: 3, Formula: "d20+5"}, got)
}

func TestAddFavourite_InvalidFormulaIsRefused(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.AddFavourite(context.Background(), charID, "banana")
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestAddFavourite_RefusedBeyondTheLimit(t *testing.T) {
	f := newFixture(t)
	full := make([]dicesvc.Favourite, dicesvc.MaxFavourites)
	for i := range full {
		full[i] = dicesvc.Favourite{ID: int64(i + 1), Formula: "d6+" + string(rune('a'+i%26))}
	}
	f.dal.EXPECT().List(mock.Anything, charID).Return(full, nil).Once()
	_, err := f.svc.AddFavourite(context.Background(), charID, "d4")
	require.ErrorIs(t, err, domain.ErrConflict)
}

func TestFavourites_ListsFromTheDAL(t *testing.T) {
	f := newFixture(t)
	want := []dicesvc.Favourite{{ID: 1, Formula: "d4"}, {ID: 2, Formula: "d%"}}
	f.dal.EXPECT().List(mock.Anything, charID).Return(want, nil).Once()
	got, err := f.svc.Favourites(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestDeleteFavourite(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().Delete(mock.Anything, charID, int64(3)).Return(true, nil).Once()
	f.dal.EXPECT().Delete(mock.Anything, charID, int64(4)).Return(false, nil).Once()
	require.NoError(t, f.svc.DeleteFavourite(context.Background(), charID, 3))
	require.ErrorIs(t, f.svc.DeleteFavourite(context.Background(), charID, 4), domain.ErrNotFound)
}

func TestReorderFavourites_WritesEachPositionInOneTransaction(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().List(mock.Anything, charID).Return([]dicesvc.Favourite{{ID: 1}, {ID: 2}, {ID: 3}}, nil).Once()
	f.dal.EXPECT().SetPosition(mock.Anything, charID, int64(3), int64(1)).Return(true, nil).Once()
	f.dal.EXPECT().SetPosition(mock.Anything, charID, int64(1), int64(2)).Return(true, nil).Once()
	f.dal.EXPECT().SetPosition(mock.Anything, charID, int64(2), int64(3)).Return(true, nil).Once()
	require.NoError(t, f.svc.ReorderFavourites(context.Background(), charID, []int64{3, 1, 2}))
	assert.Equal(t, 1, f.tx.calls)
}

func TestReorderFavourites_RefusesAnythingButTheFullSet(t *testing.T) {
	for name, ids := range map[string][]int64{
		"unknown id":   {1, 2, 9},
		"repeated id":  {1, 2, 2},
		"missing one":  {1, 2},
		"extra one":    {1, 2, 3, 4},
		"empty":        {},
		"out of range": {0, -1, 3},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.dal.EXPECT().List(mock.Anything, charID).Return([]dicesvc.Favourite{{ID: 1}, {ID: 2}, {ID: 3}}, nil).Once()
			err := f.svc.ReorderFavourites(context.Background(), charID, ids)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, "ids", fe.Field)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
		})
	}
}
