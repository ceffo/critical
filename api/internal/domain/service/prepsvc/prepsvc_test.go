package prepsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// passTx runs fn directly: the transaction itself is storage's job.
type passTx struct{ calls int }

func (p *passTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	p.calls++
	return fn(ctx)
}

type fixture struct {
	svc   prepsvc.SVC
	slots *mocks.MockSlots
	log   *mocks.MockLog
	tx    *passTx
}

// bardTable is PHB Table 32 at level 15.
var bardTable = map[int]int{1: 3, 2: 3, 3: 3, 4: 3, 5: 2}

var book = map[string]prepsvc.SpellbookEntry{
	"magic_missile": {ID: "magic_missile", Name: "Magic Missile", Level: 1},
	"alarm":         {ID: "alarm", Name: "Alarm", Level: 1},
	"identify":      {ID: "identify", Name: "Identify", Level: 1},
	"enlarge":       {ID: "enlarge", Name: "Enlarge", Level: 1},
	"mirror_image":  {ID: "mirror_image", Name: "Mirror Image", Level: 2},
	"fireball":      {ID: "fireball", Name: "Fireball", Level: 3},
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{slots: mocks.NewMockSlots(t), log: mocks.NewMockLog(t), tx: &passTx{}}

	b := mocks.NewMockSpellbook(t)
	b.EXPECT().Get(mock.Anything).RunAndReturn(func(id string) (prepsvc.SpellbookEntry, bool) {
		e, ok := book[id]
		return e, ok
	}).Maybe()
	b.EXPECT().All().Return([]prepsvc.SpellbookEntry{book["alarm"], book["fireball"]}).Maybe()

	limits := mocks.NewMockLimits(t)
	limits.EXPECT().BardSpellsPerDay(15).Return(bardTable, nil).Maybe()

	chars := mocks.NewMockCharacters(t)
	chars.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(&charactersvc.Character{Level: 15}, nil).Maybe()

	svc, err := prepsvc.New(prepsvc.Deps{
		Slots: f.slots, Spellbook: b, Limits: limits, Characters: chars, Log: f.log, Tx: f.tx,
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

func TestNew_NilDeps(t *testing.T) {
	_, err := prepsvc.New(prepsvc.Deps{})
	require.Error(t, err)
}

func TestInfo(t *testing.T) {
	f := newFixture(t)
	info, err := f.svc.Info(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []prepsvc.LevelLimit{{1, 3}, {2, 3}, {3, 3}, {4, 3}, {5, 2}}, info.Limits)
	assert.Len(t, info.Spellbook, 2)
}

func TestPrepare_Refused(t *testing.T) {
	tests := []struct {
		name    string
		ids     []string
		wantErr error
	}{
		{"4 level-1 spells at level 15", []string{"magic_missile", "alarm", "identify", "enlarge"}, prepsvc.ErrOverLimit},
		{"5 copies of one level-1 spell", []string{"alarm", "alarm", "alarm", "alarm", "alarm"}, prepsvc.ErrOverLimit},
		{"spell outside the spellbook", []string{"shield"}, prepsvc.ErrNotInSpellbook},
		{"valid and invalid mixed", []string{"alarm", "shield"}, prepsvc.ErrNotInSpellbook},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t) // no slot store or log call is expected: nothing is written
			_, err := f.svc.Prepare(context.Background(), 1, tc.ids)
			require.ErrorIs(t, err, tc.wantErr)
			assert.Zero(t, f.tx.calls)
		})
	}
}

func TestPrepare_AcceptsTheLimit_KeepsOrderAndDuplicates(t *testing.T) {
	f := newFixture(t)
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return([]spellsvc.SpellSlot{}, nil)
	var saved []spellsvc.SpellSlot
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).
		Run(func(_ context.Context, _ int64, s []spellsvc.SpellSlot) { saved = s }).Return(nil)
	var rec actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).
		Run(func(_ context.Context, e actionlog.NewEntry) { rec = e }).Return(int64(7), nil)

	got, err := f.svc.Prepare(context.Background(), 1,
		[]string{"fireball", "magic_missile", "mirror_image", "alarm", "mirror_image", "identify"})
	require.NoError(t, err)

	want := []spellsvc.SpellSlot{
		{Level: 1, SpellID: "magic_missile", PreparedSpellName: "Magic Missile"},
		{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm"},
		{Level: 1, SpellID: "identify", PreparedSpellName: "Identify"},
		{Level: 2, SpellID: "mirror_image", PreparedSpellName: "Mirror Image"},
		{Level: 2, SpellID: "mirror_image", PreparedSpellName: "Mirror Image"},
		{Level: 3, SpellID: "fireball", PreparedSpellName: "Fireball"},
	}
	assert.Equal(t, want, got)
	assert.Equal(t, want, saved)

	// Recorded in the same transaction, with a snapshot under the state key.
	assert.Equal(t, 1, f.tx.calls)
	assert.Equal(t, actionlog.KindSpellPrep, rec.Kind)
	assert.Equal(t, "Prepared: Magic Missile, Alarm, Identify, Mirror Image ×2, Fireball", rec.Summary)
	require.Len(t, rec.Snapshots, 1)
	assert.Equal(t, prepsvc.StateKey, rec.Snapshots[0].Key)
	assert.JSONEq(t, `[]`, string(rec.Snapshots[0].Before))
	var after []spellsvc.SpellSlot
	require.NoError(t, json.Unmarshal(rec.Snapshots[0].After, &after))
	assert.Equal(t, want, after)
}

func TestPrepare_ThreeLevelOneSpellsAccepted(t *testing.T) {
	f := newFixture(t)
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return(nil, nil)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).Return(nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
	got, err := f.svc.Prepare(context.Background(), 1, []string{"magic_missile", "alarm", "identify"})
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestPrepare_KeepsUsedFlagOfSpellsStillPrepared(t *testing.T) {
	f := newFixture(t)
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return([]spellsvc.SpellSlot{
		{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm", Used: true},
		{Level: 2, SpellID: "mirror_image", PreparedSpellName: "Mirror Image", Used: true},
		{Level: 2, SpellID: "mirror_image", PreparedSpellName: "Mirror Image"},
	}, nil)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).Return(nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)

	got, err := f.svc.Prepare(context.Background(), 1, []string{"mirror_image", "mirror_image", "mirror_image", "magic_missile"})
	require.NoError(t, err)
	used := []bool{}
	for _, s := range got {
		used = append(used, s.Used)
	}
	// magic_missile (new, unused), then mirror_image: one used cast carried over, two fresh.
	assert.Equal(t, []bool{false, true, false, false}, used)
}

func TestPrepare_RecordFailureFailsTheCall(t *testing.T) {
	f := newFixture(t)
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return(nil, nil)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).Return(nil)
	boom := errors.New("boom")
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(0), boom)
	_, err := f.svc.Prepare(context.Background(), 1, []string{"alarm"})
	require.ErrorIs(t, err, boom) // the real TxRunner rolls the slot write back
}

func TestRestore_WritesTheSnapshotBack(t *testing.T) {
	f := newFixture(t)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), []spellsvc.SpellSlot{
		{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm", Used: true},
	}).Return(nil)
	raw, err := json.Marshal([]spellsvc.SpellSlot{{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm", Used: true}})
	require.NoError(t, err)
	require.NoError(t, f.svc.Restore(context.Background(), 1, raw))

	require.Error(t, f.svc.Restore(context.Background(), 1, json.RawMessage(`{`)))
}

// critical-qki.41: Rary's extra slots are not part of the preparation, so
// preparing again keeps them as they are.
func TestPrepare_KeepsRarysExtraSlots(t *testing.T) {
	f := newFixture(t)
	extra := spellsvc.SpellSlot{Level: 2, SpellID: "mirror_image", PreparedSpellName: "Mirror Image", Used: true, Extra: true}
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return([]spellsvc.SpellSlot{
		{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm"}, extra,
	}, nil)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).Return(nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)

	got, err := f.svc.Prepare(context.Background(), 1, []string{"mirror_image"})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.False(t, got[0].Used, "a fresh mirror_image: the extra slot's cast does not count against it")
	assert.Equal(t, extra, got[1])
}
