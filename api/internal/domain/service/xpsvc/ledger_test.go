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
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/classes"
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
	log   *mocks.MockLog
	tx    *recordingTx
}

var today = time.Date(2026, 10, 17, 21, 30, 0, 0, time.Local)

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{dal: mocks.NewMockDAL(t), chars: mocks.NewMockCharacters(t), log: mocks.NewMockLog(t), tx: &recordingTx{}}
	svc, err := xpsvc.New(xpsvc.Deps{
		DAL: f.dal, Characters: f.chars, Thresholds: xpTables{}, Log: f.log, Tx: f.tx,
		Now: func() time.Time { return today },
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

func (f fixture) sheet(dex, cha, level, xp int) {
	f.chars.EXPECT().GetCharacter(mock.Anything, int64(1)).
		Return(&charactersvc.Character{ID: 1, Class: "bard", Dex: dex, Cha: cha, XPTotal: xp, Derived: charactersvc.Derived{TargetLevel: level}}, nil)
}

// entries answers the stored class entries read (a single bard unless told).
func (f fixture) entries(es ...classes.Entry) {
	if len(es) == 0 {
		es = []classes.Entry{classes.SingleEntry("bard", 15, 0)}
	}
	f.chars.EXPECT().ClassEntries(mock.Anything, int64(1)).Return(es, nil).Once()
}

// recorded captures the one action-log entry a write records.
func (f fixture) recorded() *actionlog.NewEntry {
	got := &actionlog.NewEntry{}
	f.log.EXPECT().Record(mock.Anything, mock.Anything).
		Run(func(_ context.Context, e actionlog.NewEntry) { *got = e }).Return(42, nil).Once()
	return got
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

// ADR 0030 step 3: the progress reads the character's class table. A mage
// at 2,600 XP is level 2 of Table 20 (2,500 / 5,000), not the bard's 1,250 /
// 2,500.
func TestGet_ProgressFromTheCharactersClassTable(t *testing.T) {
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, int64(1)).
		Return(&charactersvc.Character{ID: 1, Class: "mage", XPTotal: 2_600, Derived: charactersvc.Derived{TargetLevel: 2}}, nil)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)

	got, err := f.svc.Get(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 2_500, got.CurrentFloor)
	assert.Equal(t, 5_000, *got.NextThreshold)
	assert.Equal(t, 2_400, got.ToNext)
	assert.Equal(t, 4, got.ProgressPercent)
	assert.Equal(t, mageCells(1, xpsvc.CellPast, xpsvc.CellCurrent, xpsvc.CellNext,
		xpsvc.CellAhead, xpsvc.CellAhead, xpsvc.CellAhead, xpsvc.CellAhead, xpsvc.CellAhead), got.Levels)
}

// ADR 0015 step 4 (amended by ADR 0018): an add is one transaction whose
// entry carries the award's "xp:<id>" snapshot, absent before.
func TestAdd_RecordsOneTransactionWithTheAwardsSnapshot(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().Insert(mock.Anything, int64(1), xpsvc.Entry{
		Date: "2026-10-17", Session: "S42", Reason: "troll", Base: 1200, Bonus: true,
	}).Return(7, nil).Once()
	rec := f.recorded()
	f.entries()
	f.sheet(17, 16, 15, 0)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return([]xpsvc.Entry{
		{ID: 7, Date: "2026-10-17", Session: "S42", Reason: "troll", Base: 1200, Bonus: true},
	}, nil)

	got, err := f.svc.Add(context.Background(), 1, xpsvc.EntryInput{
		Base: 1200, Bonus: true, Reason: "  troll ", Session: " S42",
	})
	require.NoError(t, err)
	assert.Equal(t, 1320, got.Total)
	assert.Equal(t, 1, f.tx.n)
	assert.Equal(t, actionlog.KindXPAdd, rec.Kind)
	assert.Equal(t, int64(1), rec.CharacterID)
	assert.JSONEq(t, `{"entry":{"id":7,"date":"2026-10-17","session":"S42","reason":"troll","base":1200,"bonus":true,"bonusAmount":120,"total":1320}}`, string(rec.Payload))
	require.Len(t, rec.Snapshots, 1)
	assert.Equal(t, "xp:7", rec.Snapshots[0].Key)
	assert.Equal(t, snaptest.Absent("7"), rec.Snapshots[0].Before)
	assert.Equal(t, "7", snaptest.ID(t, rec.Snapshots[0].After))
	assert.JSONEq(t, `{"date":"2026-10-17","session":"S42","reason":"troll","base":1200,"bonus":true}`, snaptest.State(t, rec.Snapshots[0].After))
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
			f := newFixture(t) // no DAL call, no Record expected
			_, err := f.svc.Add(context.Background(), 1, tt.in)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, tt.field, fe.Field)
		})
	}
}

// ADR 0015 step 4 (amended by ADR 0018): an edit is one transaction whose
// entry keeps the award before and after, in its payload and its snapshot.
func TestUpdate_AppliesOnlyTheSetFieldsInOneTransaction(t *testing.T) {
	f := newFixture(t)
	stored := xpsvc.Entry{ID: 3, Date: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1000, Bonus: true}
	f.dal.EXPECT().Get(mock.Anything, int64(1), int64(3)).Return(stored, nil).Once()
	f.dal.EXPECT().Update(mock.Anything, int64(1), xpsvc.Entry{
		ID: 3, Date: "2026-10-10", Session: "S41", Reason: "ogre chief", Base: 1100, Bonus: false,
	}).Return(nil).Once()
	rec := f.recorded()
	f.entries()
	f.sheet(17, 16, 15, 0)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)

	base, bonus, reason := 1100, false, " ogre chief "
	_, err := f.svc.Update(context.Background(), 1, 3, xpsvc.EntryPatch{Base: &base, Bonus: &bonus, Reason: &reason})
	require.NoError(t, err)
	assert.Equal(t, 1, f.tx.n)
	assert.Equal(t, actionlog.KindXPEdit, rec.Kind)
	assert.JSONEq(t, `{
		"entry":{"id":3,"date":"2026-10-10","session":"S41","reason":"ogre chief","base":1100,"bonus":false,"bonusAmount":0,"total":1100},
		"before":{"id":3,"date":"2026-10-10","session":"S41","reason":"ogre","base":1000,"bonus":true,"bonusAmount":100,"total":1100}
	}`, string(rec.Payload))
	require.Len(t, rec.Snapshots, 1)
	assert.Equal(t, "xp:3", rec.Snapshots[0].Key)
	assert.JSONEq(t, `{"date":"2026-10-10","session":"S41","reason":"ogre","base":1000,"bonus":true}`, snaptest.State(t, rec.Snapshots[0].Before))
	assert.JSONEq(t, `{"date":"2026-10-10","session":"S41","reason":"ogre chief","base":1100,"bonus":false}`, snaptest.State(t, rec.Snapshots[0].After))
}

// ADR 0018 step 1: an edit that changes nothing writes nothing and records
// no transaction.
func TestUpdate_NothingChangedRecordsNothing(t *testing.T) {
	f := newFixture(t) // no Update, no Record expected
	stored := xpsvc.Entry{ID: 3, Date: "2026-10-10", Reason: "ogre", Base: 1000}
	f.dal.EXPECT().Get(mock.Anything, int64(1), int64(3)).Return(stored, nil).Once()
	f.sheet(17, 16, 15, 0)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return([]xpsvc.Entry{stored}, nil)

	base, reason := 1000, " ogre"
	got, err := f.svc.Update(context.Background(), 1, 3, xpsvc.EntryPatch{Base: &base, Reason: &reason})
	require.NoError(t, err)
	assert.Equal(t, 1000, got.Total)
}

func TestUpdate_UnknownEntryAndInvalidBase(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().Get(mock.Anything, int64(1), int64(9)).Return(xpsvc.Entry{}, xpsvc.ErrEntryNotFound).Once()
	_, err := f.svc.Update(context.Background(), 1, 9, xpsvc.EntryPatch{})
	require.ErrorIs(t, err, domain.ErrNotFound)

	f.dal.EXPECT().Get(mock.Anything, int64(1), int64(3)).Return(xpsvc.Entry{ID: 3, Base: 10}, nil).Once()
	zero := 0
	_, err = f.svc.Update(context.Background(), 1, 3, xpsvc.EntryPatch{Base: &zero})
	require.ErrorIs(t, err, domain.ErrInvalidInput) // and no Update, no Record
}

// ADR 0015 step 4 (amended by ADR 0018): a delete is one transaction whose
// entry keeps the removed award in its payload and as its snapshot's "before".
func TestDelete_RecordsTheRemovedAwardOrReportsNotFound(t *testing.T) {
	f := newFixture(t)
	stored := xpsvc.Entry{ID: 3, Date: "2026-10-10", Reason: "ogre", Base: 15, Bonus: true}
	f.dal.EXPECT().Get(mock.Anything, int64(1), int64(3)).Return(stored, nil).Once()
	f.dal.EXPECT().Delete(mock.Anything, int64(1), int64(3)).Return(nil).Once()
	rec := f.recorded()
	f.entries()
	f.sheet(17, 16, 15, 0)
	f.dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	got, err := f.svc.Delete(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Empty(t, got.Entries)
	assert.Equal(t, 1, f.tx.n)
	assert.Equal(t, actionlog.KindXPRemove, rec.Kind)
	assert.JSONEq(t, `{"entry":{"id":3,"date":"2026-10-10","session":"","reason":"ogre","base":15,"bonus":true,"bonusAmount":2,"total":17}}`, string(rec.Payload))
	require.Len(t, rec.Snapshots, 1)
	assert.Equal(t, "3", snaptest.ID(t, rec.Snapshots[0].Before))
	assert.JSONEq(t, `{"date":"2026-10-10","session":"","reason":"ogre","base":15,"bonus":true}`, snaptest.State(t, rec.Snapshots[0].Before))
	assert.Equal(t, snaptest.Absent("3"), rec.Snapshots[0].After)

	f.dal.EXPECT().Get(mock.Anything, int64(1), int64(4)).Return(xpsvc.Entry{}, xpsvc.ErrEntryNotFound).Once()
	_, err = f.svc.Delete(context.Background(), 1, 4)
	require.ErrorIs(t, err, domain.ErrNotFound) // and no Delete, no Record
}

// The restorer writes a snapshot back: absent deletes, present overwrites,
// or writes the entry back under its id when it is gone.
func TestRestorer_WritesTheSnapshotBack(t *testing.T) {
	ctx := context.Background()
	entry := xpsvc.Entry{ID: 3, Date: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1000, Bonus: true}
	present := snaptest.EnvID("3", `{"date":"2026-10-10","session":"S41","reason":"ogre","base":1000,"bonus":true}`)
	restorer := func(f fixture) actionlog.Restorer { return f.svc.Module().Restorer(xpsvc.StateKeyXP) }

	t.Run("absent deletes", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().Delete(mock.Anything, int64(1), int64(3)).Return(nil).Once()
		require.NoError(t, restorer(f).Restore(ctx, 1, snaptest.Absent("3")))
	})
	t.Run("present overwrites", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().Update(mock.Anything, int64(1), entry).Return(nil).Once()
		require.NoError(t, restorer(f).Restore(ctx, 1, present))
	})
	t.Run("gone is written back under its id", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().Update(mock.Anything, int64(1), entry).Return(xpsvc.ErrEntryNotFound).Once()
		f.dal.EXPECT().Reinsert(mock.Anything, int64(1), entry).Return(nil).Once()
		require.NoError(t, restorer(f).Restore(ctx, 1, present))
	})
	t.Run("no id is refused", func(t *testing.T) {
		f := newFixture(t)
		require.Error(t, restorer(f).Restore(ctx, 1, snaptest.Absent("")))
	})
}

func TestPreview_IsStep1AndValidatesTheBase(t *testing.T) {
	f := newFixture(t)
	f.entries()
	got, err := f.svc.Preview(context.Background(), 1, 1205, true)
	require.NoError(t, err)
	assert.Equal(t, xpsvc.Amount{Base: 1205, BonusAmount: 121, Total: 1326}, got)
	_, err = f.svc.Preview(context.Background(), 1, 0, true)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

// Totals are what the entries add to the starting XP (ADR 0015 step 1), each
// award with its bonus: what the character's level is derived from (ADR 0030
// step 3), award by award (ADR 0042 step 5).
func TestTotals_Awards(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	dal.EXPECT().List(mock.Anything, int64(1)).Return([]xpsvc.Entry{{ID: 7, Base: 1000, Bonus: true}, {ID: 5, Base: 25}}, nil).Once()
	got, err := xpsvc.Totals{DAL: dal}.Awards(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []charactersvc.Award{{ID: 7, Amount: 1100}, {ID: 5, Amount: 25}}, got)
}
