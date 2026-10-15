package xpsvc_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc/mocks"
)

// recordingTx runs the unit of work directly and counts the transactions.
type recordingTx struct{ n int }

func (r *recordingTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	r.n++
	return fn(ctx)
}

type fixture struct {
	svc   xpsvc.SVC
	dal   *mocks.MockDAL
	chars *mocks.MockCharacters
	tx    *recordingTx
}

var today = time.Date(2026, 10, 17, 21, 30, 0, 0, time.Local)

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{dal: mocks.NewMockDAL(t), chars: mocks.NewMockCharacters(t), tx: &recordingTx{}}
	svc, err := xpsvc.New(xpsvc.Deps{
		DAL: f.dal, Characters: f.chars, Thresholds: table25{}, Tx: f.tx,
		Now: func() time.Time { return today },
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

func (f fixture) sheet(dex, cha, level, xp int) {
	f.chars.EXPECT().GetCharacter(mock.Anything, int64(1)).
		Return(&charactersvc.Character{ID: 1, Dex: dex, Cha: cha, Level: level, XPTotal: xp}, nil)
}

func TestNew_RejectsNilDependencies(t *testing.T) {
	_, err := xpsvc.New(xpsvc.Deps{})
	require.Error(t, err)
}

func TestGet_TotalsEntriesOverTheSheetAndReportsEligibility(t *testing.T) {
	f := newFixture(t)
	f.sheet(17, 16, 15, 1_100_000)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return([]xpsvc.Entry{
		{ID: 2, Date: "2026-10-17", Reason: "dragon", Base: 15, Bonus: true},
		{ID: 1, Date: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1000},
	}, nil)

	got, err := f.svc.Get(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1_100_000, got.Opening)
	assert.Equal(t, 1_101_017, got.Total)
	assert.True(t, got.BonusEligible)
	assert.Equal(t, 1_100_000, got.CurrentFloor)
	assert.Equal(t, 1_320_000-1_101_017, got.ToNext)
	assert.Equal(t, []xpsvc.Entry{
		{ID: 2, Date: "2026-10-17", Reason: "dragon", Base: 15, Bonus: true, BonusAmount: 2, Total: 17},
		{ID: 1, Date: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1000, Total: 1000},
	}, got.Entries)
}

func TestGet_NotEligibleBelow16(t *testing.T) {
	f := newFixture(t)
	f.sheet(15, 18, 15, 0)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	got, err := f.svc.Get(context.Background(), 1)
	require.NoError(t, err)
	assert.False(t, got.BonusEligible)
}

func TestAdd_DatesTodayTrimsAndReturnsTheLedger(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().Insert(mock.Anything, int64(1), xpsvc.Entry{
		Date: "2026-10-17", Session: "S42", Reason: "troll", Base: 1200, Bonus: true,
	}).Return(7, nil).Once()
	f.sheet(17, 16, 15, 0)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return([]xpsvc.Entry{
		{ID: 7, Date: "2026-10-17", Session: "S42", Reason: "troll", Base: 1200, Bonus: true},
	}, nil)

	got, err := f.svc.Add(context.Background(), 1, xpsvc.EntryInput{
		Base: 1200, Bonus: true, Reason: "  troll ", Session: " S42",
	})
	require.NoError(t, err)
	assert.Equal(t, 1320, got.Total)
}

func TestAdd_RejectsInvalidInput(t *testing.T) {
	tests := map[string]struct {
		in    xpsvc.EntryInput
		field string
	}{
		"zero base":        {xpsvc.EntryInput{Base: 0}, "base"},
		"negative base":    {xpsvc.EntryInput{Base: -5}, "base"},
		"base too large":   {xpsvc.EntryInput{Base: xpsvc.MaxBase + 1}, "base"},
		"reason too long":  {xpsvc.EntryInput{Base: 1, Reason: strings.Repeat("é", xpsvc.MaxReasonLen+1)}, "reason"},
		"session too long": {xpsvc.EntryInput{Base: 1, Session: strings.Repeat("x", xpsvc.MaxSessionLen+1)}, "session"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t) // no DAL call expected
			_, err := f.svc.Add(context.Background(), 1, tt.in)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, tt.field, fe.Field)
		})
	}
}

func TestUpdate_AppliesOnlyTheSetFieldsInOneTransaction(t *testing.T) {
	f := newFixture(t)
	stored := xpsvc.Entry{ID: 3, Date: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1000, Bonus: true}
	f.dal.EXPECT().Get(mock.Anything, int64(1), int64(3)).Return(stored, nil).Once()
	f.dal.EXPECT().Update(mock.Anything, int64(1), xpsvc.Entry{
		ID: 3, Date: "2026-10-10", Session: "S41", Reason: "ogre chief", Base: 1100, Bonus: false,
	}).Return(nil).Once()
	f.sheet(17, 16, 15, 0)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)

	base, bonus, reason := 1100, false, " ogre chief "
	_, err := f.svc.Update(context.Background(), 1, 3, xpsvc.EntryPatch{Base: &base, Bonus: &bonus, Reason: &reason})
	require.NoError(t, err)
	assert.Equal(t, 1, f.tx.n)
}

func TestUpdate_UnknownEntryAndInvalidBase(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().Get(mock.Anything, int64(1), int64(9)).Return(xpsvc.Entry{}, xpsvc.ErrEntryNotFound).Once()
	_, err := f.svc.Update(context.Background(), 1, 9, xpsvc.EntryPatch{})
	require.ErrorIs(t, err, domain.ErrNotFound)

	f.dal.EXPECT().Get(mock.Anything, int64(1), int64(3)).Return(xpsvc.Entry{ID: 3, Base: 10}, nil).Once()
	zero := 0
	_, err = f.svc.Update(context.Background(), 1, 3, xpsvc.EntryPatch{Base: &zero})
	require.ErrorIs(t, err, domain.ErrInvalidInput) // and no Update call
}

func TestDelete_RemovesOrReportsNotFound(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().Delete(mock.Anything, int64(1), int64(3)).Return(nil).Once()
	f.sheet(17, 16, 15, 0)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	got, err := f.svc.Delete(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Empty(t, got.Entries)

	f.dal.EXPECT().Delete(mock.Anything, int64(1), int64(4)).Return(xpsvc.ErrEntryNotFound).Once()
	_, err = f.svc.Delete(context.Background(), 1, 4)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPreview_IsStep1AndValidatesTheBase(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.Preview(1205, true)
	require.NoError(t, err)
	assert.Equal(t, xpsvc.Amount{Base: 1205, BonusAmount: 121, Total: 1326}, got)
	_, err = f.svc.Preview(0, true)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}
