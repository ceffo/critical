package buffsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

const charID = int64(1)

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }
func strPtr(v string) *string     { return &v }

// fakeTx runs the unit of work directly and counts transactions.
type fakeTx struct{ calls int }

func (f *fakeTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(ctx)
}

type fixture struct {
	svc   buffsvc.SVC
	dal   *mocks.MockDAL
	log   *mocks.MockLog
	chars *mocks.MockCharacters
	dice  *mocks.MockDice
	items *mocks.MockItems
	tx    *fakeTx
	// entries recorded through the log mock, in order.
	entries []actionlog.NewEntry
	// order of Record / Save* calls, to prove the entry precedes the writes.
	calls []string
	// initiatives written through SaveInitiative, in order (nil: cleared).
	initiatives []*buffsvc.Initiative
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		dal:   mocks.NewMockDAL(t),
		log:   mocks.NewMockLog(t),
		chars: mocks.NewMockCharacters(t),
		dice:  mocks.NewMockDice(t),
		items: mocks.NewMockItems(t),
		tx:    &fakeTx{},
	}
	svc, err := buffsvc.NewWithConfig(buffsvc.DefaultConfig(), f.dal, f.tx, f.log, f.chars, f.dice, f.items)
	require.NoError(t, err)
	f.svc = svc
	return f
}

// state sets up the reads of one mutation.
func (f *fixture) state(buffs []buffsvc.BuffEffect, round int) {
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(buffs, nil).Once()
	f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(round, nil).Once()
}

// records accepts one Record call and captures the entry.
func (f *fixture) records() {
	f.log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, e actionlog.NewEntry) (int64, error) {
			f.entries = append(f.entries, e)
			f.calls = append(f.calls, "record")
			return int64(len(f.entries)), nil
		}).Once()
}

// saves expects the buff list to be written as want.
func (f *fixture) saves(want []buffsvc.BuffEffect) {
	f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, want).RunAndReturn(
		func(context.Context, int64, []buffsvc.BuffEffect) error {
			f.calls = append(f.calls, "save buffs")
			return nil
		}).Once()
}

// initiativeIs answers one GetInitiative read with cur.
func (f *fixture) initiativeIs(cur *buffsvc.Initiative) {
	f.dal.EXPECT().GetInitiative(mock.Anything, charID).Return(cur, nil).Once()
}

// savesInitiative accepts one SaveInitiative call and captures it.
func (f *fixture) savesInitiative() {
	f.dal.EXPECT().SaveInitiative(mock.Anything, charID, mock.Anything).RunAndReturn(
		func(_ context.Context, _ int64, i *buffsvc.Initiative) error {
			f.initiatives = append(f.initiatives, i)
			f.calls = append(f.calls, "save initiative")
			return nil
		}).Once()
}

// rollsInitiative expects one initiative roll: the read of the state before
// it (none), exactly one d10 that shows d10, its entry, then the store.
func (f *fixture) rollsInitiative(d10 int) {
	f.initiativeIs(nil)
	f.dice.EXPECT().RollDie(10).Return(d10).Once()
	f.records()
	f.savesInitiative()
}

// powers answers the items port's reads with the carried items' powers.
func (f *fixture) powers(ps ...buffsvc.ItemPower) {
	f.items.EXPECT().ItemPowers(mock.Anything, charID).Return(ps, nil)
}

func (f *fixture) base(str int) {
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{ID: charID, Str: str}, nil)
}

func decodeSnap(t *testing.T, raw json.RawMessage) []map[string]any {
	t.Helper()
	var out []map[string]any
	require.NoError(t, json.Unmarshal([]byte(snaptest.State(t, raw)), &out))
	return out
}

func snapKeys(e actionlog.NewEntry) []string {
	keys := make([]string, len(e.Snapshots))
	for i, s := range e.Snapshots {
		keys[i] = s.Key
	}
	return keys
}

func payload(t *testing.T, e actionlog.NewEntry) map[string]any {
	t.Helper()
	var p map[string]any
	require.NoError(t, json.Unmarshal(e.Payload, &p))
	return p
}

func TestNew_NilDependencies(t *testing.T) {
	dal, log, chars, dice, items := mocks.NewMockDAL(t), mocks.NewMockLog(t), mocks.NewMockCharacters(t), mocks.NewMockDice(t), mocks.NewMockItems(t)
	tx := &fakeTx{}
	for name, build := range map[string]func() (buffsvc.SVC, error){
		"config": func() (buffsvc.SVC, error) { return buffsvc.NewWithConfig(nil, dal, tx, log, chars, dice, items) },
		"dal":    func() (buffsvc.SVC, error) { return buffsvc.New(nil, tx, log, chars, dice, items) },
		"tx":     func() (buffsvc.SVC, error) { return buffsvc.New(dal, nil, log, chars, dice, items) },
		"log":    func() (buffsvc.SVC, error) { return buffsvc.New(dal, tx, nil, chars, dice, items) },
		"chars":  func() (buffsvc.SVC, error) { return buffsvc.New(dal, tx, log, nil, dice, items) },
		"dice":   func() (buffsvc.SVC, error) { return buffsvc.New(dal, tx, log, chars, nil, items) },
		"items":  func() (buffsvc.SVC, error) { return buffsvc.New(dal, tx, log, chars, dice, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			_, err := build()
			require.Error(t, err)
		})
	}
}

func TestListActive(t *testing.T) {
	f := newFixture(t)
	buffs := []buffsvc.BuffEffect{{ID: "a", Name: "Enlarge"}}
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(buffs, nil)

	got, err := f.svc.ListActive(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, buffs, got)
}

func TestActivate(t *testing.T) {
	existing := buffsvc.BuffEffect{ID: "old", Name: "Old", Duration: 3}

	t.Run("logs the activation with before/after snapshots, then saves", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{existing}, 4)
		f.records()
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
			func(context.Context, int64, []buffsvc.BuffEffect) error {
				f.calls = append(f.calls, "save buffs")
				return nil
			})

		got, err := f.svc.Activate(context.Background(), charID,
			buffsvc.BuffEffect{Name: " Stoneskin ", Charges: 5})
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, existing, got[0])
		assert.Len(t, got[1].ID, 36, "uuid string expected")
		assert.Equal(t, "Stoneskin", got[1].Name)

		require.Len(t, f.entries, 1)
		e := f.entries[0]
		assert.Equal(t, actionlog.KindBuffChange, e.Kind)
		assert.Empty(t, e.Summary, "the UI words the entry from its payload")
		assert.Zero(t, e.Round, "stamped with the current round by the log")
		assert.Equal(t, []string{buffsvc.StateKeyBuffs}, snapKeys(e))
		assert.Len(t, decodeSnap(t, e.Snapshots[0].Before), 1)
		assert.Len(t, decodeSnap(t, e.Snapshots[0].After), 2)
		assert.Equal(t, map[string]any{"action": "activate", "buff": "Stoneskin", "charges": 5.0}, payload(t, e))
		assert.Equal(t, []string{"record", "save buffs"}, f.calls)
		assert.Equal(t, 1, f.tx.calls)
	})

	t.Run("keeps an explicit ID", func(t *testing.T) {
		f := newFixture(t)
		f.state(nil, 0)
		f.records()
		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).Return(nil)

		got, err := f.svc.Activate(context.Background(), charID, buffsvc.BuffEffect{ID: "mine", Name: "X"})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "mine", got[0].ID)
		assert.JSONEq(t, `{"action":"activate","buff":"X"}`, string(f.entries[0].Payload))
	})

	invalid := []struct {
		name string
		buff buffsvc.BuffEffect
	}{
		{"blank name", buffsvc.BuffEffect{Name: "  "}},
		{"negative rounds", buffsvc.BuffEffect{Name: "X", Duration: -1}},
		{"negative charges", buffsvc.BuffEffect{Name: "X", Charges: -1}},
		{"zero STR bonus", buffsvc.BuffEffect{Name: "X", Effects: buffsvc.BuffEffects{StrBonus: intPtr(0)}}},
		{"STR cap above 25", buffsvc.BuffEffect{Name: "X", Effects: buffsvc.BuffEffects{StrMax: intPtr(26)}}},
		{"STR cap below 1", buffsvc.BuffEffect{Name: "X", Effects: buffsvc.BuffEffects{StrMax: intPtr(0)}}},
		{"min STR above 25", buffsvc.BuffEffect{Name: "X", Effects: buffsvc.BuffEffects{MinStr: intPtr(30)}}},
		{"weapon +0", buffsvc.BuffEffect{Name: "X", Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(0)}}},
	}
	for _, tt := range invalid {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			f := newFixture(t) // no reads, no record, no writes
			_, err := f.svc.Activate(context.Background(), charID, tt.buff)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
		})
	}

	t.Run("record error: nothing is saved", func(t *testing.T) {
		f := newFixture(t)
		boom := errors.New("boom")
		f.state(nil, 0)
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(0, boom)

		_, err := f.svc.Activate(context.Background(), charID, buffsvc.BuffEffect{Name: "X"})
		assert.ErrorIs(t, err, boom)
	})

	t.Run("read error is wrapped", func(t *testing.T) {
		f := newFixture(t)
		boom := errors.New("boom")
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(nil, boom)

		_, err := f.svc.Activate(context.Background(), charID, buffsvc.BuffEffect{Name: "X"})
		assert.ErrorIs(t, err, boom)
	})
}

func TestEdit(t *testing.T) {
	str := buffsvc.BuffEffect{ID: "s", Name: "Strength", Duration: 10}
	mirror := buffsvc.BuffEffect{ID: "m", Name: "Mirror Image", Duration: 20, Charges: 4}
	gem := buffsvc.BuffEffect{ID: "g", Name: "Power Gem"}

	tests := []struct {
		name        string
		start       []buffsvc.BuffEffect
		id          string
		edit        buffsvc.BuffEdit
		want        []buffsvc.BuffEffect
		wantPayload string
		wantErr     error
	}{
		{
			name: "sets remaining rounds", start: []buffsvc.BuffEffect{str, mirror}, id: "s",
			edit:        buffsvc.BuffEdit{Duration: intPtr(3)},
			want:        []buffsvc.BuffEffect{{ID: "s", Name: "Strength", Duration: 3}, mirror},
			wantPayload: `{"action":"edit","buff":"Strength","fromRounds":10,"rounds":3}`,
		},
		{
			name: "sets charges only", start: []buffsvc.BuffEffect{mirror}, id: "m",
			edit:        buffsvc.BuffEdit{Charges: intPtr(2)},
			want:        []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 20, Charges: 2}},
			wantPayload: `{"action":"edit","buff":"Mirror Image","fromRounds":20,"fromCharges":4,"rounds":20,"charges":2}`,
		},
		{
			name: "rounds set to 0 ends a timed buff", start: []buffsvc.BuffEffect{str, mirror}, id: "s",
			edit:        buffsvc.BuffEdit{Duration: intPtr(0)},
			want:        []buffsvc.BuffEffect{mirror},
			wantPayload: `{"action":"edit","buff":"Strength","fromRounds":10,"ended":true}`,
		},
		{
			name: "charges set to 0 ends a charged buff", start: []buffsvc.BuffEffect{mirror}, id: "m",
			edit:        buffsvc.BuffEdit{Charges: intPtr(0)},
			want:        []buffsvc.BuffEffect{},
			wantPayload: `{"action":"edit","buff":"Mirror Image","fromRounds":20,"fromCharges":4,"rounds":20,"ended":true}`,
		},
		{
			name: "rounds on an untimed buff make it timed", start: []buffsvc.BuffEffect{gem}, id: "g",
			edit:        buffsvc.BuffEdit{Duration: intPtr(5)},
			want:        []buffsvc.BuffEffect{{ID: "g", Name: "Power Gem", Duration: 5}},
			wantPayload: `{"action":"edit","buff":"Power Gem","rounds":5}`,
		},
		{
			name: "note only", start: []buffsvc.BuffEffect{str}, id: "s",
			edit:        buffsvc.BuffEdit{Note: strPtr(" fire −50% ")},
			want:        []buffsvc.BuffEffect{{ID: "s", Name: "Strength", Duration: 10, Note: "fire −50%"}},
			wantPayload: `{"action":"edit","buff":"Strength","fromRounds":10,"rounds":10,"note":"fire −50%","noteChanged":true}`,
		},
		{
			name: "note with rounds", start: []buffsvc.BuffEffect{{ID: "s", Name: "Strength", Duration: 10, Note: "old"}}, id: "s",
			edit:        buffsvc.BuffEdit{Duration: intPtr(3), Note: strPtr("")},
			want:        []buffsvc.BuffEffect{{ID: "s", Name: "Strength", Duration: 3}},
			wantPayload: `{"action":"edit","buff":"Strength","fromRounds":10,"rounds":3,"fromNote":"old","noteChanged":true}`,
		},
		{name: "unknown id", start: []buffsvc.BuffEffect{str}, id: "nope", edit: buffsvc.BuffEdit{Duration: intPtr(1)}, wantErr: domain.ErrNotFound},
		{name: "negative rounds", start: []buffsvc.BuffEffect{str}, id: "s", edit: buffsvc.BuffEdit{Duration: intPtr(-1)}, wantErr: domain.ErrInvalidInput},
		{name: "negative charges", start: []buffsvc.BuffEffect{mirror}, id: "m", edit: buffsvc.BuffEdit{Charges: intPtr(-2)}, wantErr: domain.ErrInvalidInput},
		{name: "nothing to change", start: []buffsvc.BuffEffect{str}, id: "s", wantErr: domain.ErrInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.state(tt.start, 2)
			if tt.wantErr == nil {
				f.records()
				f.saves(tt.want)
			}

			got, err := f.svc.Edit(context.Background(), charID, tt.id, tt.edit)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			require.Len(t, f.entries, 1)
			assert.Equal(t, actionlog.KindBuffChange, f.entries[0].Kind)
			assert.Empty(t, f.entries[0].Summary, "the UI words the entry")
			assert.JSONEq(t, tt.wantPayload, string(f.entries[0].Payload))
			assert.Equal(t, []string{buffsvc.StateKeyBuffs}, snapKeys(f.entries[0]))
		})
	}
}

func TestTrigger(t *testing.T) {
	tests := []struct {
		name        string
		start       []buffsvc.BuffEffect
		id          string
		want        []buffsvc.BuffEffect
		wantPayload string
		wantErr     error
	}{
		{
			name:        "a Stoneskin layer absorbs a hit",
			start:       []buffsvc.BuffEffect{{ID: "c", Name: "Stoneskin", Charges: 3}},
			id:          "c",
			want:        []buffsvc.BuffEffect{{ID: "c", Name: "Stoneskin", Charges: 2}},
			wantPayload: `{"action":"absorb","buff":"Stoneskin","charges":2}`,
		},
		{
			name:        "the last layer ends the buff",
			start:       []buffsvc.BuffEffect{{ID: "c", Name: "Stoneskin", Charges: 1}, {ID: "d", Name: "Haste", Duration: 2}},
			id:          "c",
			want:        []buffsvc.BuffEffect{{ID: "d", Name: "Haste", Duration: 2}},
			wantPayload: `{"action":"absorb","buff":"Stoneskin","ended":true}`,
		},
		{
			name:        "Mirror Image loses an image, keeps its rounds",
			start:       []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 9, Charges: 2}},
			id:          "m",
			want:        []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 9, Charges: 1}},
			wantPayload: `{"action":"absorb","buff":"Mirror Image","charges":1}`,
		},
		{
			name:        "the last image ends Mirror Image though rounds are left",
			start:       []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 9, Charges: 1}, {ID: "d", Name: "Haste", Duration: 2}},
			id:          "m",
			want:        []buffsvc.BuffEffect{{ID: "d", Name: "Haste", Duration: 2}},
			wantPayload: `{"action":"absorb","buff":"Mirror Image","ended":true}`,
		},
		{name: "a buff without charges cannot absorb", start: []buffsvc.BuffEffect{{ID: "d", Name: "Haste", Duration: 2}}, id: "d", wantErr: domain.ErrInvalidInput},
		{name: "unknown id", start: []buffsvc.BuffEffect{{ID: "c", Charges: 3}}, id: "nope", wantErr: domain.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.state(tt.start, 1)
			if tt.wantErr == nil {
				f.records()
				f.saves(tt.want)
			}

			got, err := f.svc.Trigger(context.Background(), charID, tt.id)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Empty(t, f.entries[0].Summary, "the UI words the entry")
			assert.JSONEq(t, tt.wantPayload, string(f.entries[0].Payload))
			assert.Equal(t, []string{buffsvc.StateKeyBuffs}, snapKeys(f.entries[0]))
		})
	}
}

func TestDismiss(t *testing.T) {
	a := buffsvc.BuffEffect{ID: "a", Name: "Bless"}
	b := buffsvc.BuffEffect{ID: "b", Name: "Haste"}

	t.Run("removes the matching buff and logs it", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{a, b}, 0)
		f.records()
		f.saves([]buffsvc.BuffEffect{b})

		got, err := f.svc.Dismiss(context.Background(), charID, "a")
		require.NoError(t, err)
		assert.Equal(t, []buffsvc.BuffEffect{b}, got)
		assert.JSONEq(t, `{"action":"dismiss","buff":"Bless"}`, string(f.entries[0].Payload))
	})

	t.Run("unknown id is not found and nothing is logged", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{a, b}, 0)

		_, err := f.svc.Dismiss(context.Background(), charID, "zzz")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestGetRound(t *testing.T) {
	f := newFixture(t)
	f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(4, nil)

	got, err := f.svc.GetRound(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, 4, got)
}

func TestAdvanceRound(t *testing.T) {
	tests := []struct {
		name        string
		start       []buffsvc.BuffEffect
		round       int
		wantActive  []buffsvc.BuffEffect
		wantExpired []buffsvc.BuffEffect
		wantPayload string
	}{
		{
			name: "counts timed buffs down, expires at 0, leaves charges alone",
			start: []buffsvc.BuffEffect{
				{ID: "d2", Name: "Haste", Duration: 2},
				{ID: "d1", Name: "Strength", Duration: 1},
				{ID: "c", Name: "Stoneskin", Charges: 2},
				{ID: "m", Name: "Mirror Image", Duration: 3, Charges: 2},
			},
			round: 5,
			wantActive: []buffsvc.BuffEffect{
				{ID: "d2", Name: "Haste", Duration: 1},
				{ID: "c", Name: "Stoneskin", Charges: 2},
				{ID: "m", Name: "Mirror Image", Duration: 2, Charges: 2},
			},
			wantExpired: []buffsvc.BuffEffect{{ID: "d1", Name: "Strength", Duration: 0}},
			wantPayload: `{"action":"next_round","fromRound":5,"toRound":6,"expired":["Strength"]}`,
		},
		{
			name:        "from round 0 with no buffs",
			round:       0,
			wantActive:  []buffsvc.BuffEffect{},
			wantExpired: []buffsvc.BuffEffect{},
			wantPayload: `{"action":"next_round","fromRound":0,"toRound":1}`,
		},
		{
			name:        "Mirror Image with images left still expires with its rounds",
			start:       []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 1, Charges: 3}},
			round:       2,
			wantActive:  []buffsvc.BuffEffect{},
			wantExpired: []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 0, Charges: 3}},
			wantPayload: `{"action":"next_round","fromRound":2,"toRound":3,"expired":["Mirror Image"]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.state(tt.start, tt.round)
			f.records()
			f.saves(tt.wantActive)
			f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, tt.round+1).RunAndReturn(
				func(context.Context, int64, int) error {
					f.calls = append(f.calls, "save round")
					return nil
				})
			f.initiativeIs(nil) // no initiative to end
			f.rollsInitiative(7)

			active, expired, round, _, err := f.svc.AdvanceRound(context.Background(), charID)
			require.NoError(t, err)
			assert.Equal(t, tt.round+1, round)
			assert.Equal(t, tt.wantActive, active)
			assert.Equal(t, tt.wantExpired, expired)

			require.Len(t, f.entries, 2, "the advance, then the initiative")
			e := f.entries[0]
			assert.Equal(t, actionlog.KindRoundAdvance, e.Kind)
			assert.Equal(t, actionlog.KindInitiative, f.entries[1].Kind)
			assert.Empty(t, e.Summary, "the UI words the entry")
			assert.JSONEq(t, tt.wantPayload, string(e.Payload))
			assert.Equal(t, []string{buffsvc.StateKeyBuffs, buffsvc.StateKeyRound}, snapKeys(e))
			assert.JSONEq(t, jsonInt(tt.round), snaptest.State(t, e.Snapshots[1].Before))
			assert.JSONEq(t, jsonInt(tt.round+1), snaptest.State(t, e.Snapshots[1].After))
			assert.Len(t, decodeSnap(t, e.Snapshots[0].Before), len(tt.start))
			assert.Equal(t, []string{"record", "save buffs", "save round", "record", "save initiative"}, f.calls,
				"the entry is recorded before the round moves, so it belongs to the round it ends")
			assert.Equal(t, 1, f.tx.calls)
		})
	}

	t.Run("tx error is returned and results are zeroed", func(t *testing.T) {
		f := newFixture(t)
		boom := errors.New("boom")
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(nil, nil)
		f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(0, boom)

		active, expired, round, _, err := f.svc.AdvanceRound(context.Background(), charID)
		assert.ErrorIs(t, err, boom)
		assert.Nil(t, active)
		assert.Nil(t, expired)
		assert.Zero(t, round)
	})
}

func jsonInt(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// Strength is +1d6 for a bard, max 18, and stacks with itself; Improved
// Strength 1d6+4, max 23, replaces it. The Power Gem needs STR 18 or more.
var (
	powerGem = buffsvc.BuffEffect{ID: "g", Name: "Power Gem", Effects: buffsvc.BuffEffects{
		MinStr: intPtr(18), WeaponBuff: intPtr(5), Multiplier: floatPtr(2.0),
	}}
	strength = func(bonus int) buffsvc.BuffEffect {
		return buffsvc.BuffEffect{ID: "s", Name: "Strength", Duration: 90, Effects: buffsvc.BuffEffects{
			StrBonus: intPtr(bonus), StrMax: intPtr(18),
		}}
	}
	improvedStrength = func(bonus int) buffsvc.BuffEffect {
		return buffsvc.BuffEffect{ID: "i", Name: "Improved Strength", Duration: 15, Effects: buffsvc.BuffEffects{
			StrBonus: intPtr(bonus), StrMax: intPtr(23),
		}}
	}
	// handMade is a Strength bonus entered in the buff form: no cap, or a
	// custom one.
	handMade = func(bonus int, strMax *int) buffsvc.BuffEffect {
		return buffsvc.BuffEffect{ID: "h", Name: "Strength (hand-made)", Effects: buffsvc.BuffEffects{
			StrBonus: intPtr(bonus), StrMax: strMax,
		}}
	}
)

type strCase struct {
	name  string
	base  int
	buffs []buffsvc.BuffEffect
	want  int
}

func runEffectiveStr(t *testing.T, tests []strCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.base(tt.base)
			f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(tt.buffs, nil).Once()

			got, err := f.svc.GetEffectiveStr(context.Background(), charID)
			require.NoError(t, err)
			assert.Equal(t, buffsvc.Strength{Base: tt.base, Effective: tt.want}, got)
		})
	}
}

func TestGetEffectiveStr(t *testing.T) {
	runEffectiveStr(t, []strCase{
		{"no buffs: the sheet", 12, nil, 12},
		{"other buffs do not touch STR", 12, []buffsvc.BuffEffect{powerGem}, 12},
	})

	t.Run("character error is wrapped", func(t *testing.T) {
		f := newFixture(t)
		boom := errors.New("boom")
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(nil, boom)

		_, err := f.svc.GetEffectiveStr(context.Background(), charID)
		assert.ErrorIs(t, err, boom)
	})
}

// ADR 0004 step 7: with no Improved Strength, base + the sum of every
// Strength bonus, capped at 18 (user: "all str spells stack up to 18").
func TestStrength_Step7_StrengthStacksUpTo18(t *testing.T) {
	runEffectiveStr(t, []strCase{
		{"one cast: 12 + 4 = 16", 12, []buffsvc.BuffEffect{strength(4)}, 16},
		{"one cast reaching 18: 12 + 6", 12, []buffsvc.BuffEffect{strength(6)}, 18},
		{"two casts sum: 12 + 2 + 3 = 17", 12, []buffsvc.BuffEffect{strength(2), strength(3)}, 17},
		{"two casts sum to 18: 12 + 1 + 5", 12, []buffsvc.BuffEffect{strength(1), strength(5)}, 18},
		{"sum capped at 18: 12 + 4 + 5 = 21 -> 18", 12, []buffsvc.BuffEffect{strength(4), strength(5)}, 18},
		{"one cast capped at 18: 15 + 6", 15, []buffsvc.BuffEffect{strength(6)}, 18},
		{"never lowers a score above 18", 19, []buffsvc.BuffEffect{strength(3), strength(2)}, 19},
		{
			"hand-made +4 with no cap + received +3: 12 + 7 -> 18", 12,
			[]buffsvc.BuffEffect{handMade(4, nil), strength(3)},
			18,
		},
		{
			"hand-made +2 with no cap + received +1: 12 + 3 = 15", 12,
			[]buffsvc.BuffEffect{handMade(2, nil), strength(1)},
			15,
		},
		{
			"hand-made uncapped alone is still capped at 18: 12 + 9", 12,
			[]buffsvc.BuffEffect{handMade(9, nil)},
			18,
		},
		{
			"a custom cap below 18 is ignored, the total cap is 18: 12 + 5(max 16) + 2", 12,
			[]buffsvc.BuffEffect{handMade(5, intPtr(16)), strength(2)},
			18,
		},
	})
}

// ADR 0004 step 8: Improved Strength (a bonus with str_max above 18)
// replaces every other Strength bonus and can reach 23; it is not
// cumulative with itself (best single one).
func TestStrength_Step8_ImprovedReplacesStrength(t *testing.T) {
	runEffectiveStr(t, []strCase{
		{"Improved 1d6+4 alone: 12 + 10 = 22", 12, []buffsvc.BuffEffect{improvedStrength(10)}, 22},
		{"Improved caps at 23", 16, []buffsvc.BuffEffect{improvedStrength(9)}, 23},
		{
			"replaces a higher L2 sum: L2 12+6 = 18, Improved 12+5 = 17", 12,
			[]buffsvc.BuffEffect{strength(6), strength(3), improvedStrength(5)},
			17,
		},
		{
			"replaces a lower L2 sum: L2 12+2 = 14, Improved 12+7 = 19", 12,
			[]buffsvc.BuffEffect{strength(2), improvedStrength(7)},
			19,
		},
		{
			"two Improved: the best single one wins", 12,
			[]buffsvc.BuffEffect{improvedStrength(6), improvedStrength(8)},
			20,
		},
		{
			"replaces hand-made bonuses too: 12 + 4 + 3, Improved 12+5 = 17", 12,
			[]buffsvc.BuffEffect{handMade(4, nil), strength(3), improvedStrength(5)},
			17,
		},
	})
}

// ADR 0004 step 10 (Power Gem): a buff with min_str 18 applies at
// effective STR 18 or more (user: "not exactly 18 >= 18"): on at 18, 19 and
// 23, off at 17.
func TestStrength_Step10_PowerGemFrom18Up(t *testing.T) {
	on := func(str int) buffsvc.BuffModifiers {
		return buffsvc.BuffModifiers{
			SpellsBuff: 1.0, WeaponBuff: 5, WeaponBuffFrom: "Power Gem", Multipliers: 2.0, EffectiveStr: str,
			MultipliersFrom: []buffsvc.Factor{{Buff: "Power Gem", Value: 2.0}},
		}
	}
	tests := []struct {
		name  string
		base  int
		buffs []buffsvc.BuffEffect
		want  buffsvc.BuffModifiers
	}{
		{
			"17 (12 + Strength 5): off", 12,
			[]buffsvc.BuffEffect{strength(5), powerGem},
			buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: 17},
		},
		{"18 (12 + Strength 2 + 4): on", 12, []buffsvc.BuffEffect{strength(2), strength(4), powerGem}, on(18)},
		{"19 (12 + Improved 7): on", 12, []buffsvc.BuffEffect{improvedStrength(7), powerGem}, on(19)},
		{"23 (14 + Improved 10, capped): on", 14, []buffsvc.BuffEffect{improvedStrength(10), powerGem}, on(23)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(tt.buffs, nil).Once()
			f.powers()

			got, err := f.svc.Aggregate(context.Background(), charID, sheetStr(tt.base), buffsvc.WeaponNone)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ADR 0004 step 9: Strength effects are never gated by min_str: a Strength bonus that names a
// min_str far above the score still counts toward the effective STR, while
// the buff's other effects stay gated on that effective STR.
func TestStrength_Step9_StrengthEffectsAreNotGated(t *testing.T) {
	gated := func(id string, bonus, strMax int) buffsvc.BuffEffect {
		return buffsvc.BuffEffect{ID: id, Name: "Gated " + id, Effects: buffsvc.BuffEffects{
			StrBonus: intPtr(bonus), StrMax: intPtr(strMax), MinStr: intPtr(25),
		}}
	}
	runEffectiveStr(t, []strCase{
		{"a gated Strength bonus still adds: 12 + 3", 12, []buffsvc.BuffEffect{gated("a", 3, 18)}, 15},
		{"a gated Improved Strength still replaces: 12 + 7", 12, []buffsvc.BuffEffect{strength(2), gated("b", 7, 23)}, 19},
	})
}

func TestListStatus(t *testing.T) {
	tests := []struct {
		name  string
		buffs []buffsvc.BuffEffect
		want  []bool
	}{
		{"Power Gem alone at STR 12 does not apply", []buffsvc.BuffEffect{powerGem}, []bool{false}},
		{"Strength to 18 makes the Power Gem apply", []buffsvc.BuffEffect{strength(6), powerGem}, []bool{true, true}},
		{"Strength to 17 is not enough", []buffsvc.BuffEffect{strength(5), powerGem}, []bool{true, false}},
		// ADR 0004 step 8: while Improved Strength replaces them, Strength bonuses do not apply.
		{
			"Improved active: the Strength bonuses show as not applying",
			[]buffsvc.BuffEffect{strength(3), handMade(2, nil), improvedStrength(6), powerGem},
			[]bool{false, false, true, true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.base(12)
			f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(tt.buffs, nil)

			got, err := f.svc.ListStatus(context.Background(), charID)
			require.NoError(t, err)
			applies := make([]bool, len(got))
			for i, s := range got {
				applies[i] = s.Applies
				assert.Equal(t, tt.buffs[i], s.BuffEffect)
			}
			assert.Equal(t, tt.want, applies)
		})
	}
}

func TestAggregate(t *testing.T) {
	tests := []struct {
		name  string
		buffs []buffsvc.BuffEffect
		str   int
		want  buffsvc.BuffModifiers
	}{
		{
			name: "no buffs gives neutral modifiers",
			str:  10,
			want: buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: 10},
		},
		{
			name: "multipliers stack multiplicatively, flat bonuses additively, weapon +N: best wins",
			buffs: []buffsvc.BuffEffect{
				{Name: "A", Effects: buffsvc.BuffEffects{Multiplier: floatPtr(2.0), SpellsBuff: floatPtr(1.5), WeaponBuff: intPtr(5), SaveBonus: intPtr(1), AC: intPtr(-1), ToHit: intPtr(1), Damage: intPtr(2)}},
				{Name: "B", Effects: buffsvc.BuffEffects{Multiplier: floatPtr(1.5), SpellsBuff: floatPtr(2.0), WeaponBuff: intPtr(2), SaveBonus: intPtr(2), AC: intPtr(-4), ToHit: intPtr(-3), Damage: intPtr(1)}},
			},
			str: 12,
			want: buffsvc.BuffModifiers{
				SpellsBuff: 3.0, WeaponBuff: 5, WeaponBuffFrom: "A", Multipliers: 3.0, SaveBonus: 3, AC: -5,
				ToHit: -2, Damage: 3, EffectiveStr: 12,
				SpellsBuffFrom:  []buffsvc.Factor{{Buff: "A", Value: 1.5}, {Buff: "B", Value: 2.0}},
				MultipliersFrom: []buffsvc.Factor{{Buff: "A", Value: 2.0}, {Buff: "B", Value: 1.5}},
				ToHitFrom:       []buffsvc.Contribution{{Buff: "A", Value: 1}, {Buff: "B", Value: -3}},
				DamageFrom:      []buffsvc.Contribution{{Buff: "A", Value: 2}, {Buff: "B", Value: 1}},
				SaveBonusFrom:   []buffsvc.Contribution{{Buff: "A", Value: 1}, {Buff: "B", Value: 2}},
			},
		},
		{
			name: "to-hit and damage are gated by MinStr like every other effect",
			buffs: []buffsvc.BuffEffect{
				{Name: "Heavy", Effects: buffsvc.BuffEffects{ToHit: intPtr(2), Damage: intPtr(4), MinStr: intPtr(19)}},
			},
			str:  18,
			want: buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: 18},
		},
		{
			name:  "MinStr gates the buff when STR is too low",
			buffs: []buffsvc.BuffEffect{{Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(10), MinStr: intPtr(19)}}},
			str:   18,
			want:  buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: 18},
		},
		{
			name:  "MinStr met exactly applies",
			buffs: []buffsvc.BuffEffect{{Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(10), MinStr: intPtr(18)}}},
			str:   18,
			want:  buffsvc.BuffModifiers{SpellsBuff: 1.0, WeaponBuff: 10, Multipliers: 1.0, EffectiveStr: 18},
		},
		{
			name:  "Power Gem off at the sheet's STR 12",
			buffs: []buffsvc.BuffEffect{powerGem},
			str:   12,
			want:  buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: 12},
		},
		{
			name:  "Strength to 18 turns the Power Gem on",
			buffs: []buffsvc.BuffEffect{strength(6), powerGem},
			str:   12,
			want:  buffsvc.BuffModifiers{SpellsBuff: 1.0, WeaponBuff: 5, WeaponBuffFrom: "Power Gem", Multipliers: 2.0, EffectiveStr: 18},
		},
		{
			name:  "Strength to 16 leaves it off",
			buffs: []buffsvc.BuffEffect{strength(4), powerGem},
			str:   12,
			want:  buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: 16},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(tt.buffs, nil)
			f.powers()

			got, err := f.svc.Aggregate(context.Background(), charID, sheetStr(tt.str), buffsvc.WeaponNone)
			require.NoError(t, err)
			assert.InDelta(t, tt.want.SpellsBuff, got.SpellsBuff, 1e-9)
			assert.InDelta(t, tt.want.Multipliers, got.Multipliers, 1e-9)
			assert.Equal(t, tt.want.WeaponBuff, got.WeaponBuff)
			if tt.want.WeaponBuff != 0 && tt.want.WeaponBuffFrom != "" {
				assert.Equal(t, tt.want.WeaponBuffFrom, got.WeaponBuffFrom)
			}
			assert.Equal(t, tt.want.ToHit, got.ToHit)
			assert.Equal(t, tt.want.Damage, got.Damage)
			assert.Equal(t, tt.want.ToHitFrom, got.ToHitFrom)
			assert.Equal(t, tt.want.DamageFrom, got.DamageFrom)
			assert.Equal(t, tt.want.SaveBonusFrom, got.SaveBonusFrom)
			if tt.want.SpellsBuffFrom != nil {
				assert.Equal(t, tt.want.SpellsBuffFrom, got.SpellsBuffFrom)
				assert.Equal(t, tt.want.MultipliersFrom, got.MultipliersFrom)
			}
			assert.Equal(t, tt.want.SaveBonus, got.SaveBonus)
			assert.Equal(t, tt.want.AC, got.AC)
			assert.Equal(t, tt.want.EffectiveStr, got.EffectiveStr)
		})
	}
}

func TestAggregate_DALError(t *testing.T) {
	f := newFixture(t)
	boom := errors.New("boom")
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(nil, boom)

	_, err := f.svc.Aggregate(context.Background(), charID, sheetStr(10), buffsvc.WeaponNone)
	assert.ErrorIs(t, err, boom)
}

// Every state key any use case snapshots must have a Restorer, or voiding
// that entry fails ("no restorer registered").
func TestRestorers_CoverEverySnapshotKey(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	stone := buffsvc.BuffEffect{ID: "c", Name: "Stoneskin", Duration: 3, Charges: 3}
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{stone}, nil)
	f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(1, nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, e actionlog.NewEntry) (int64, error) {
			f.entries = append(f.entries, e)
			return 1, nil
		})
	f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).Return(nil)
	f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, mock.Anything).Return(nil)
	f.dal.EXPECT().GetInitiative(mock.Anything, charID).Return(&buffsvc.Initiative{D10: 3, LogEntryID: 1}, nil)
	f.dal.EXPECT().SaveInitiative(mock.Anything, charID, mock.Anything).Return(nil)
	f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(1, nil)
	f.dice.EXPECT().RollDie(10).Return(4)
	f.log.EXPECT().TopID(mock.Anything, charID).Return(1, true, nil)

	_, err := f.svc.Activate(ctx, charID, buffsvc.BuffEffect{Name: "X", Duration: 1})
	require.NoError(t, err)
	_, err = f.svc.Edit(ctx, charID, "c", buffsvc.BuffEdit{Duration: intPtr(2)})
	require.NoError(t, err)
	_, err = f.svc.Trigger(ctx, charID, "c")
	require.NoError(t, err)
	_, err = f.svc.Dismiss(ctx, charID, "c")
	require.NoError(t, err)
	_, _, _, _, err = f.svc.AdvanceRound(ctx, charID)
	require.NoError(t, err)
	_, err = f.svc.RerollInitiative(ctx, charID)
	require.NoError(t, err)
	_, _, err = f.svc.ResetRound(ctx, charID)
	require.NoError(t, err)
	_, _, _, _, err = f.svc.PassTime(ctx, charID, 1)
	require.NoError(t, err)

	module := f.svc.Module()
	seen := map[string]bool{}
	for _, e := range f.entries {
		for _, s := range e.Snapshots {
			seen[s.Key] = true
			assert.NotNil(t, module.Restorer(s.Key), "no Restorer for state key %q", s.Key)
		}
	}
	assert.Equal(t, map[string]bool{buffsvc.StateKeyBuffs: true, buffsvc.StateKeyRound: true, buffsvc.StateKeyInitiative: true}, seen)
}

func TestRestorers_WriteTheSnapshotBack(t *testing.T) {
	t.Run("buffs: the recorded before state round-trips", func(t *testing.T) {
		f := newFixture(t)
		before := []buffsvc.BuffEffect{
			strength(6),
			{ID: "m", Name: "Mirror Image", Duration: 4, Charges: 2},
			powerGem,
			{ID: "b", Name: "Bless", Duration: 6, Effects: buffsvc.BuffEffects{ToHit: intPtr(1), Damage: intPtr(2)}},
		}
		f.state(before, 3)
		f.records()
		f.saves([]buffsvc.BuffEffect{strength(6), powerGem, before[3]})
		_, err := f.svc.Dismiss(context.Background(), charID, "m")
		require.NoError(t, err)

		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, before).Return(nil).Once()
		err = f.svc.Module().Restorer(buffsvc.StateKeyBuffs).Restore(context.Background(), charID, f.entries[0].Snapshots[0].Before)
		require.NoError(t, err)
	})

	t.Run("round", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 7).Return(nil)
		require.NoError(t, f.svc.Module().Restorer(buffsvc.StateKeyRound).Restore(context.Background(), charID, snaptest.Env(`7`)))
	})

	bad := []struct {
		key   string
		state string
	}{
		{buffsvc.StateKeyBuffs, `{"not":"a list"}`},
		{buffsvc.StateKeyRound, `"seven"`},
		{buffsvc.StateKeyRound, `-1`},
	}
	for _, tt := range bad {
		t.Run("rejects "+tt.key+" "+tt.state, func(t *testing.T) {
			f := newFixture(t) // nothing is written
			err := f.svc.Module().Restorer(tt.key).Restore(context.Background(), charID, snaptest.Env(tt.state))
			assert.Error(t, err)
		})
	}
}

func TestRoundMoves(t *testing.T) {
	buffs := []buffsvc.BuffEffect{{ID: "h", Name: "Haste", Duration: 4}, {ID: "c", Name: "Stoneskin", Charges: 2}}
	tests := []struct {
		name        string
		kind        actionlog.Kind
		from, to    int
		call        func(buffsvc.SVC) ([]buffsvc.BuffEffect, int, error)
		wantPayload string
	}{
		{
			name: "End combat resets to 0", kind: actionlog.KindRoundReset, from: 5, to: 0,
			call: func(s buffsvc.SVC) ([]buffsvc.BuffEffect, int, error) {
				return s.ResetRound(context.Background(), charID)
			},
			wantPayload: `{"action":"end_combat","fromRound":5,"toRound":0}`,
		},
		{
			name: "End combat at round 0 still logs", kind: actionlog.KindRoundReset, from: 0, to: 0,
			call: func(s buffsvc.SVC) ([]buffsvc.BuffEffect, int, error) {
				return s.ResetRound(context.Background(), charID)
			},
			wantPayload: `{"action":"end_combat","fromRound":0,"toRound":0}`,
		},
		{
			name: "Set round up", kind: actionlog.KindRoundSet, from: 2, to: 9,
			call: func(s buffsvc.SVC) ([]buffsvc.BuffEffect, int, error) {
				return s.SetRound(context.Background(), charID, 9)
			},
			wantPayload: `{"action":"set_round","fromRound":2,"toRound":9}`,
		},
		{
			name: "Set round down to 0", kind: actionlog.KindRoundSet, from: 4, to: 0,
			call: func(s buffsvc.SVC) ([]buffsvc.BuffEffect, int, error) {
				return s.SetRound(context.Background(), charID, 0)
			},
			wantPayload: `{"action":"set_round","fromRound":4,"toRound":0}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.state(buffs, tt.from)
			f.records()
			if tt.to == 0 {
				f.initiativeIs(nil) // round 0 ends the initiative; there is none here
			}
			f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, tt.to).RunAndReturn(
				func(context.Context, int64, int) error { f.calls = append(f.calls, "save round"); return nil }).Once()
			// SaveActiveBuffs is not expected: the mock fails the test if buffs are written.

			active, round, err := tt.call(f.svc)
			require.NoError(t, err)
			assert.Equal(t, tt.to, round)
			assert.Equal(t, buffs, active, "buffs are untouched")

			require.Len(t, f.entries, 1)
			e := f.entries[0]
			assert.Equal(t, tt.kind, e.Kind)
			assert.Empty(t, e.Summary, "the UI words the entry")
			assert.JSONEq(t, tt.wantPayload, string(e.Payload))
			assert.Equal(t, []string{buffsvc.StateKeyRound}, snapKeys(e), "round key only")
			assert.JSONEq(t, jsonInt(tt.from), snaptest.State(t, e.Snapshots[0].Before))
			assert.JSONEq(t, jsonInt(tt.to), snaptest.State(t, e.Snapshots[0].After))
			assert.Equal(t, []string{"record", "save round"}, f.calls)
			assert.Equal(t, 1, f.tx.calls)
		})
	}

	t.Run("Set round rejects a negative round without touching anything", func(t *testing.T) {
		f := newFixture(t)
		_, _, err := f.svc.SetRound(context.Background(), charID, -1)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
		assert.Empty(t, f.entries)
		assert.Zero(t, f.tx.calls)
	})

	t.Run("voiding restores the previous round", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 5).Return(nil).Once()
		err := f.svc.Module().Restorer(buffsvc.StateKeyRound).Restore(context.Background(), charID, snaptest.Env(`5`))
		require.NoError(t, err)
	})
}

// ADR 0028 step 2: the score in effect of an ability is the sheet's score
// raised by the effects on that ability: Strength by the STR buffs (the same
// effective STR as GetEffectiveStr), every other ability the sheet's, other
// buffs active. An unknown key is refused.
func TestGetAbilityScore(t *testing.T) {
	sheet := &charactersvc.Character{ID: charID, Str: 12, Dex: 15, Con: 13, Int: 14, Wis: 9, Cha: 16}
	cases := []struct {
		name    string
		ability charactersvc.Ability
		buffs   []buffsvc.BuffEffect
		want    buffsvc.AbilityScore
	}{
		{"STR raised by a Strength buff", charactersvc.AbilityStr, []buffsvc.BuffEffect{strength(4)}, buffsvc.AbilityScore{Base: 12, Effective: 16}},
		{"STR with no buff", charactersvc.AbilityStr, nil, buffsvc.AbilityScore{Base: 12, Effective: 12}},
		{"DEX untouched by a Strength buff", charactersvc.AbilityDex, []buffsvc.BuffEffect{strength(4), powerGem}, buffsvc.AbilityScore{Base: 15, Effective: 15}},
		{"CHA is the sheet's", charactersvc.AbilityCha, nil, buffsvc.AbilityScore{Base: 16, Effective: 16}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(sheet, nil)
			f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(c.buffs, nil).Once()
			got, err := f.svc.GetAbilityScore(context.Background(), charID, c.ability)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}

	t.Run("unknown ability", func(t *testing.T) {
		f := newFixture(t)
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(sheet, nil)
		_, err := f.svc.GetAbilityScore(context.Background(), charID, "luck")
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})
}
