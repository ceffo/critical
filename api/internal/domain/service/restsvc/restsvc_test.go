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
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
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
	svc    restsvc.SVC
	spells *mocks.MockSpells
	items  *mocks.MockItems
	psp    *mocks.MockPSP
	// maintained is what the buff list ends on a rest (ADR 0043 step 7).
	maintained []buffsvc.BuffEffect
	buffSnap   actionlog.Snapshot
	tx         *fakeTx
	entries    []actionlog.NewEntry
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
	buffs := mocks.NewMockBuffs(t)
	buffs.EXPECT().EndMaintained(mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, int64) ([]buffsvc.BuffEffect, actionlog.Snapshot, error) {
			if len(f.maintained) == 0 {
				return nil, actionlog.Snapshot{}, nil
			}
			return f.maintained, f.buffSnap, nil
		}).Maybe()
	svc, err := restsvc.New(f.spells, f.items, f.psp, buffs, log, f.tx)
	require.NoError(t, err)
	f.svc = svc
	return f
}

func TestNew_NilDependency(t *testing.T) {
	_, err := restsvc.New(nil, mocks.NewMockItems(t), mocks.NewMockPSP(t), mocks.NewMockBuffs(t), mocks.NewMockLog(t), &fakeTx{})
	require.Error(t, err)
	_, err = restsvc.New(mocks.NewMockSpells(t), mocks.NewMockItems(t), nil, mocks.NewMockBuffs(t), mocks.NewMockLog(t), &fakeTx{})
	require.Error(t, err)
	_, err = restsvc.New(mocks.NewMockSpells(t), mocks.NewMockItems(t), mocks.NewMockPSP(t), nil, mocks.NewMockLog(t), &fakeTx{})
	require.Error(t, err)
	_, err = restsvc.New(mocks.NewMockSpells(t), mocks.NewMockItems(t), mocks.NewMockPSP(t), mocks.NewMockBuffs(t), nil, &fakeTx{})
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

// ADR 0043 step 7: the rest ends every maintained power (no maintenance
// asleep), its buff snapshot in the long_rest entry so undo puts the rows
// back, and names the powers in the payload.
func TestLongRest_Step7_EndsMaintainedPowers(t *testing.T) {
	f := newFixture(t)
	f.maintained = []buffsvc.BuffEffect{{ID: "r", Name: "Levitation", PowerID: "levitation", PowerName: localized.Text{"en": "Levitation", "fr": "Lévitation"}, MaintenanceCost: 2}}
	f.buffSnap = actionlog.Snapshot{Key: "buffs", Before: []byte(`[1]`), After: []byte(`[]`)}
	f.spells.EXPECT().GetSlots(mock.Anything, int64(1)).Return(nil, nil)
	f.items.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	f.spells.EXPECT().ResetSlots(mock.Anything, int64(1)).Return(nil, nil)
	f.items.EXPECT().ResetDaily(mock.Anything, int64(1)).Return(nil, nil)
	f.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return(nil, nil).Maybe()
	f.psp.EXPECT().NightsPSPRecovery(mock.Anything, int64(1)).Return(96, nil).Once()

	_, err := f.svc.LongRest(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, f.entries, 1)
	e := f.entries[0]
	assert.Contains(t, e.Snapshots, f.buffSnap)
	assert.JSONEq(t, `{"slotsReset":0,"itemsRefilled":[],"powersEnded":["levitation"],"powerNames":{"levitation":{"en":"Levitation","fr":"Lévitation"}}}`, string(e.Payload))
}

// ADR 0043 step 7 with ADR 0044 step 10: a contact row and the power kept up
// through it both end, the entry naming the power and the target.
func TestLongRest_Step7_EndsTheContactAndItsPower(t *testing.T) {
	f := newFixture(t)
	f.maintained = []buffsvc.BuffEffect{
		{ID: "c", Name: "Orc", Target: "Orc", MaintenanceCost: buffsvc.LinkMaintenanceCost, MaintenancePeriod: 1, ContactFrom: 1},
		{ID: "p", Name: "Inflict Pain", PowerID: "inflict_pain", PowerName: localized.Text{"en": "Inflict Pain", "fr": "Douleur"}, MaintenanceCost: 2, Target: "Orc"},
	}
	f.buffSnap = actionlog.Snapshot{Key: "buffs", Before: []byte(`[1,2]`), After: []byte(`[]`)}
	f.spells.EXPECT().GetSlots(mock.Anything, int64(1)).Return(nil, nil)
	f.items.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	f.spells.EXPECT().ResetSlots(mock.Anything, int64(1)).Return(nil, nil)
	f.items.EXPECT().ResetDaily(mock.Anything, int64(1)).Return(nil, nil)
	f.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return(nil, nil).Maybe()
	f.psp.EXPECT().NightsPSPRecovery(mock.Anything, int64(1)).Return(0, nil).Once()

	_, err := f.svc.LongRest(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, f.entries, 1)
	e := f.entries[0]
	assert.Contains(t, e.Snapshots, f.buffSnap)
	assert.JSONEq(t, `{"slotsReset":0,"itemsRefilled":[],"powersEnded":["inflict_pain"],"targetsEnded":["Orc"],
		"powerNames":{"inflict_pain":{"en":"Inflict Pain","fr":"Douleur"}}}`, string(e.Payload))
}
