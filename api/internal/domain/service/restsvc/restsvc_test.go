package restsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/restsvc"
	"github.com/ceffo/critical/api/internal/domain/service/restsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// fakeTx runs fn directly, the way InTx does when it commits.
type fakeTx struct{ calls int }

func (f *fakeTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(ctx)
}

type fixture struct {
	svc     restsvc.SVC
	spells  *mocks.MockSpells
	items   *mocks.MockItems
	psp     *mocks.MockPSP
	tx      *fakeTx
	entries []actionlog.NewEntry
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{spells: mocks.NewMockSpells(t), items: mocks.NewMockItems(t), psp: mocks.NewMockPSP(t), tx: &fakeTx{}}
	log := mocks.NewMockLog(t)
	log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, e actionlog.NewEntry) (int64, error) {
			f.entries = append(f.entries, e)
			return 7, nil
		}).Maybe()
	svc, err := restsvc.New(f.spells, f.items, f.psp, log, f.tx)
	require.NoError(t, err)
	f.svc = svc
	return f
}

func TestNew_NilDependency(t *testing.T) {
	_, err := restsvc.New(nil, mocks.NewMockItems(t), mocks.NewMockPSP(t), mocks.NewMockLog(t), &fakeTx{})
	require.Error(t, err)
	_, err = restsvc.New(mocks.NewMockSpells(t), mocks.NewMockItems(t), nil, mocks.NewMockLog(t), &fakeTx{})
	require.Error(t, err)
	_, err = restsvc.New(mocks.NewMockSpells(t), mocks.NewMockItems(t), mocks.NewMockPSP(t), nil, &fakeTx{})
	require.Error(t, err)
}

// The rest is one logged entry: the slot list always (so it blocks voiding
// an older cast), plus each item whose count changed.
func TestLongRest_ResetsBothInOneLoggedTransaction(t *testing.T) {
	f := newFixture(t)
	before := []spellsvc.SpellSlot{{Level: 1, SpellID: "sleep", Used: true}, {Level: 3, SpellID: "haste"}}
	after := []spellsvc.SpellSlot{{Level: 1, SpellID: "sleep"}, {Level: 3, SpellID: "haste"}}
	f.spells.EXPECT().GetSlots(mock.Anything, int64(1)).Return(before, nil)
	f.spells.EXPECT().ResetSlots(mock.Anything, int64(1)).Return(after, nil)
	f.items.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{{ID: "gem", Name: localized.Of("Gem"), Count: 0}, {ID: "arrows", Count: 9}}, nil)
	f.items.EXPECT().ResetDaily(mock.Anything, int64(1)).Return([]itemsvc.Item{{ID: "gem", Name: localized.Of("Gem"), Count: 1}, {ID: "arrows", Count: 9}}, nil)
	warn := []itemsvc.Warning{{ItemID: "gem", Code: itemsvc.WarnHands}}
	f.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return(warn, nil)
	f.psp.EXPECT().NightsPSPRecovery(mock.Anything, int64(1)).Return(0, nil).Once()

	got, err := f.svc.LongRest(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, warn, got.Warnings, "the items answer carries their class warnings (ADR 0033 step 20)")
	assert.Equal(t, 1, f.tx.calls)
	assert.Equal(t, after, got.Slots)
	assert.Equal(t, int64(7), got.EntryID)
	require.Len(t, f.entries, 1)
	e := f.entries[0]
	assert.Equal(t, actionlog.KindLongRest, e.Kind)
	slotSnap, err := prepsvc.SlotsSnapshot(before, after)
	require.NoError(t, err)
	gemSnap, err := itemsvc.CountSnapshot("gem", 0, 1)
	require.NoError(t, err)
	assert.Equal(t, []actionlog.Snapshot{slotSnap, gemSnap}, e.Snapshots, "arrows did not change")
	assert.JSONEq(t, `{"slotsReset":1,"itemsRefilled":["Gem"]}`, string(e.Payload))
}

func TestLongRest_ItemFailureFailsTheRest(t *testing.T) {
	f := newFixture(t)
	f.spells.EXPECT().GetSlots(mock.Anything, int64(1)).Return(nil, nil)
	f.items.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	f.spells.EXPECT().ResetSlots(mock.Anything, int64(1)).Return(nil, nil)
	boom := errors.New("boom")
	f.items.EXPECT().ResetDaily(mock.Anything, int64(1)).Return(nil, boom)

	_, err := f.svc.LongRest(context.Background(), 1)
	require.ErrorIs(t, err, boom)
	assert.Empty(t, f.entries)
}

func TestLongRest_SlotFailureSkipsItems(t *testing.T) {
	f := newFixture(t)
	f.spells.EXPECT().GetSlots(mock.Anything, int64(1)).Return(nil, nil)
	f.items.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	boom := errors.New("boom")
	f.spells.EXPECT().ResetSlots(mock.Anything, int64(1)).Return(nil, boom)

	_, err := f.svc.LongRest(context.Background(), 1)
	require.ErrorIs(t, err, boom)
	assert.Empty(t, f.entries)
}

// ADR 0043 step 7: the long rest recovers a night's sleep of PSPs, carried as
// the long_rest entry's PSP delta (the recording path caps it at the PSPs
// spent, step 5), in the rest's one transaction; a character with no
// psionics gets none, and a failed read fails the rest with nothing logged.
func TestLongRest_Step7_RecoversANightsSleep(t *testing.T) {
	rest := func(f *fixture) error {
		f.spells.EXPECT().GetSlots(mock.Anything, int64(1)).Return(nil, nil)
		f.items.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
		f.spells.EXPECT().ResetSlots(mock.Anything, int64(1)).Return(nil, nil)
		f.items.EXPECT().ResetDaily(mock.Anything, int64(1)).Return(nil, nil)
		f.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return(nil, nil).Maybe()
		_, err := f.svc.LongRest(context.Background(), 1)
		return err
	}
	f := newFixture(t)
	f.psp.EXPECT().NightsPSPRecovery(mock.Anything, int64(1)).Return(96, nil).Once()
	require.NoError(t, rest(f))
	require.Len(t, f.entries, 1)
	assert.Equal(t, actionlog.KindLongRest, f.entries[0].Kind)
	assert.Equal(t, 96, f.entries[0].PSPDelta)
	assert.Equal(t, 1, f.tx.calls, "one transaction")

	f = newFixture(t)
	boom := errors.New("boom")
	f.psp.EXPECT().NightsPSPRecovery(mock.Anything, int64(1)).Return(0, boom).Once()
	require.ErrorIs(t, rest(f), boom)
	assert.Empty(t, f.entries)
}
