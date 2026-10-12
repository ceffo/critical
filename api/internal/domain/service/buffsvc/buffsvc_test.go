package buffsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const charID = int64(1)

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

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
	tx    *fakeTx
	// entries recorded through the log mock, in order.
	entries []actionlog.NewEntry
	// order of Record / Save* calls, to prove the entry precedes the writes.
	calls []string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		dal:   mocks.NewMockDAL(t),
		log:   mocks.NewMockLog(t),
		chars: mocks.NewMockCharacters(t),
		tx:    &fakeTx{},
	}
	svc, err := buffsvc.NewWithConfig(buffsvc.DefaultConfig(), f.dal, f.tx, f.log, f.chars)
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

func (f *fixture) base(str int) {
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{ID: charID, Str: str}, nil)
}

func decodeSnap(t *testing.T, raw json.RawMessage) []map[string]any {
	t.Helper()
	var out []map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
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
	dal, log, chars := mocks.NewMockDAL(t), mocks.NewMockLog(t), mocks.NewMockCharacters(t)
	tx := &fakeTx{}
	for name, build := range map[string]func() (buffsvc.SVC, error){
		"config": func() (buffsvc.SVC, error) { return buffsvc.NewWithConfig(nil, dal, tx, log, chars) },
		"dal":    func() (buffsvc.SVC, error) { return buffsvc.New(nil, tx, log, chars) },
		"tx":     func() (buffsvc.SVC, error) { return buffsvc.New(dal, nil, log, chars) },
		"log":    func() (buffsvc.SVC, error) { return buffsvc.New(dal, tx, nil, chars) },
		"chars":  func() (buffsvc.SVC, error) { return buffsvc.New(dal, tx, log, nil) },
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
		assert.Equal(t, "Activated Stoneskin (5 ch)", e.Summary)
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
		assert.Equal(t, "Activated X (until dismissed)", f.entries[0].Summary)
	})

	invalid := []struct {
		name string
		buff buffsvc.BuffEffect
	}{
		{"blank name", buffsvc.BuffEffect{Name: "  "}},
		{"negative rounds", buffsvc.BuffEffect{Name: "X", Duration: -1}},
		{"negative charges", buffsvc.BuffEffect{Name: "X", Charges: -1}},
		{"zero STR bonus", buffsvc.BuffEffect{Name: "X", Effects: buffsvc.BuffEffects{StrBonus: intPtr(0)}}},
		{"STR set above 25", buffsvc.BuffEffect{Name: "X", Effects: buffsvc.BuffEffects{StrSet: intPtr(26)}}},
		{"STR cap below 1", buffsvc.BuffEffect{Name: "X", Effects: buffsvc.BuffEffects{StrMax: intPtr(0)}}},
		{"min STR above 25", buffsvc.BuffEffect{Name: "X", Effects: buffsvc.BuffEffects{MinStr: intPtr(30)}}},
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
		wantSummary string
		wantErr     error
	}{
		{
			name: "sets remaining rounds", start: []buffsvc.BuffEffect{str, mirror}, id: "s",
			edit:        buffsvc.BuffEdit{Duration: intPtr(3)},
			want:        []buffsvc.BuffEffect{{ID: "s", Name: "Strength", Duration: 3}, mirror},
			wantSummary: "Edited Strength: 10 rd → 3 rd",
		},
		{
			name: "sets charges only", start: []buffsvc.BuffEffect{mirror}, id: "m",
			edit:        buffsvc.BuffEdit{Charges: intPtr(2)},
			want:        []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 20, Charges: 2}},
			wantSummary: "Edited Mirror Image: 20 rd, 4 ch → 20 rd, 2 ch",
		},
		{
			name: "rounds set to 0 ends a timed buff", start: []buffsvc.BuffEffect{str, mirror}, id: "s",
			edit:        buffsvc.BuffEdit{Duration: intPtr(0)},
			want:        []buffsvc.BuffEffect{mirror},
			wantSummary: "Edited Strength: 10 rd → ended",
		},
		{
			name: "charges set to 0 ends a charged buff", start: []buffsvc.BuffEffect{mirror}, id: "m",
			edit:        buffsvc.BuffEdit{Charges: intPtr(0)},
			want:        []buffsvc.BuffEffect{},
			wantSummary: "Edited Mirror Image: 20 rd, 4 ch → ended",
		},
		{
			name: "rounds on an untimed buff make it timed", start: []buffsvc.BuffEffect{gem}, id: "g",
			edit:        buffsvc.BuffEdit{Duration: intPtr(5)},
			want:        []buffsvc.BuffEffect{{ID: "g", Name: "Power Gem", Duration: 5}},
			wantSummary: "Edited Power Gem: until dismissed → 5 rd",
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
			assert.Equal(t, tt.wantSummary, f.entries[0].Summary)
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
		wantSummary string
		wantErr     error
	}{
		{
			name:        "a Stoneskin layer absorbs a hit",
			start:       []buffsvc.BuffEffect{{ID: "c", Name: "Stoneskin", Charges: 3}},
			id:          "c",
			want:        []buffsvc.BuffEffect{{ID: "c", Name: "Stoneskin", Charges: 2}},
			wantSummary: "Stoneskin absorbed a hit (2 left)",
		},
		{
			name:        "the last layer ends the buff",
			start:       []buffsvc.BuffEffect{{ID: "c", Name: "Stoneskin", Charges: 1}, {ID: "d", Name: "Haste", Duration: 2}},
			id:          "c",
			want:        []buffsvc.BuffEffect{{ID: "d", Name: "Haste", Duration: 2}},
			wantSummary: "Stoneskin absorbed a hit, ended",
		},
		{
			name:        "Mirror Image loses an image, keeps its rounds",
			start:       []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 9, Charges: 2}},
			id:          "m",
			want:        []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 9, Charges: 1}},
			wantSummary: "Mirror Image absorbed a hit (1 left)",
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
			assert.Equal(t, tt.wantSummary, f.entries[0].Summary)
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
		assert.Equal(t, "Dismissed Bless", f.entries[0].Summary)
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
		wantSummary string
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
			wantSummary: "Round 5 → 6 (expired: Strength)",
		},
		{
			name:        "from round 0 with no buffs",
			round:       0,
			wantActive:  []buffsvc.BuffEffect{},
			wantExpired: []buffsvc.BuffEffect{},
			wantSummary: "Round 0 → 1",
		},
		{
			name:        "Mirror Image with images left still expires with its rounds",
			start:       []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 1, Charges: 3}},
			round:       2,
			wantActive:  []buffsvc.BuffEffect{},
			wantExpired: []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 0, Charges: 3}},
			wantSummary: "Round 2 → 3 (expired: Mirror Image)",
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

			active, expired, round, err := f.svc.AdvanceRound(context.Background(), charID)
			require.NoError(t, err)
			assert.Equal(t, tt.round+1, round)
			assert.Equal(t, tt.wantActive, active)
			assert.Equal(t, tt.wantExpired, expired)

			require.Len(t, f.entries, 1)
			e := f.entries[0]
			assert.Equal(t, actionlog.KindRoundAdvance, e.Kind)
			assert.Equal(t, tt.wantSummary, e.Summary)
			assert.Equal(t, []string{buffsvc.StateKeyBuffs, buffsvc.StateKeyRound}, snapKeys(e))
			assert.JSONEq(t, jsonInt(tt.round), string(e.Snapshots[1].Before))
			assert.JSONEq(t, jsonInt(tt.round+1), string(e.Snapshots[1].After))
			assert.Len(t, decodeSnap(t, e.Snapshots[0].Before), len(tt.start))
			assert.Equal(t, []string{"record", "save buffs", "save round"}, f.calls,
				"the entry is recorded before the round moves, so it belongs to the round it ends")
			assert.Equal(t, 1, f.tx.calls)
		})
	}

	t.Run("tx error is returned and results are zeroed", func(t *testing.T) {
		f := newFixture(t)
		boom := errors.New("boom")
		f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(nil, nil)
		f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(0, boom)

		active, expired, round, err := f.svc.AdvanceRound(context.Background(), charID)
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

// Strength is +1d6 for a bard, max 18; Improved Strength 1d6+4, max 23;
// Strength magic is not cumulative. The Power Gem needs STR 18.
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
	gauntlets = buffsvc.BuffEffect{ID: "o", Name: "Ogre gauntlets", Effects: buffsvc.BuffEffects{StrSet: intPtr(18)}}
)

func TestGetEffectiveStr(t *testing.T) {
	tests := []struct {
		name  string
		base  int
		buffs []buffsvc.BuffEffect
		want  int
	}{
		{"no buffs: the sheet", 12, nil, 12},
		{"Strength +6 from 12 reaches 18", 12, []buffsvc.BuffEffect{strength(6)}, 18},
		{"Strength +4 from 12 gives 16", 12, []buffsvc.BuffEffect{strength(4)}, 16},
		{"Strength never confers more than 18", 15, []buffsvc.BuffEffect{strength(6)}, 18},
		{"Strength on an 18 stays 18", 18, []buffsvc.BuffEffect{strength(3)}, 18},
		{"Improved Strength 1d6+4 from 12", 12, []buffsvc.BuffEffect{improvedStrength(10)}, 22},
		{"Improved Strength caps at 23", 16, []buffsvc.BuffEffect{improvedStrength(10)}, 23},
		{"not cumulative: the best one wins", 12, []buffsvc.BuffEffect{strength(6), improvedStrength(5)}, 18},
		{"not cumulative: Improved beats Strength", 12, []buffsvc.BuffEffect{strength(6), improvedStrength(9)}, 21},
		{"a set raises the score", 12, []buffsvc.BuffEffect{gauntlets}, 18},
		{"a set never lowers it", 19, []buffsvc.BuffEffect{gauntlets}, 19},
		{"other buffs do not touch STR", 12, []buffsvc.BuffEffect{powerGem}, 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.base(tt.base)
			f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(tt.buffs, nil)

			got, err := f.svc.GetEffectiveStr(context.Background(), charID)
			require.NoError(t, err)
			assert.Equal(t, buffsvc.Strength{Base: tt.base, Effective: tt.want}, got)
		})
	}

	t.Run("character error is wrapped", func(t *testing.T) {
		f := newFixture(t)
		boom := errors.New("boom")
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(nil, boom)

		_, err := f.svc.GetEffectiveStr(context.Background(), charID)
		assert.ErrorIs(t, err, boom)
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
			name: "multipliers stack multiplicatively, flat bonuses additively",
			buffs: []buffsvc.BuffEffect{
				{Effects: buffsvc.BuffEffects{Multiplier: floatPtr(2.0), SpellsBuff: floatPtr(1.5), WeaponBuff: intPtr(5), SaveBonus: intPtr(1), AC: intPtr(-1)}},
				{Effects: buffsvc.BuffEffects{Multiplier: floatPtr(1.5), SpellsBuff: floatPtr(2.0), WeaponBuff: intPtr(2), SaveBonus: intPtr(2), AC: intPtr(-4)}},
			},
			str:  12,
			want: buffsvc.BuffModifiers{SpellsBuff: 3.0, WeaponBuff: 7, Multipliers: 3.0, SaveBonus: 3, AC: -5, EffectiveStr: 12},
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
			want:  buffsvc.BuffModifiers{SpellsBuff: 1.0, WeaponBuff: 5, Multipliers: 2.0, EffectiveStr: 18},
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

			got, err := f.svc.Aggregate(context.Background(), charID, tt.str)
			require.NoError(t, err)
			assert.InDelta(t, tt.want.SpellsBuff, got.SpellsBuff, 1e-9)
			assert.InDelta(t, tt.want.Multipliers, got.Multipliers, 1e-9)
			assert.Equal(t, tt.want.WeaponBuff, got.WeaponBuff)
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

	_, err := f.svc.Aggregate(context.Background(), charID, 10)
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

	_, err := f.svc.Activate(ctx, charID, buffsvc.BuffEffect{Name: "X", Duration: 1})
	require.NoError(t, err)
	_, err = f.svc.Edit(ctx, charID, "c", buffsvc.BuffEdit{Duration: intPtr(2)})
	require.NoError(t, err)
	_, err = f.svc.Trigger(ctx, charID, "c")
	require.NoError(t, err)
	_, err = f.svc.Dismiss(ctx, charID, "c")
	require.NoError(t, err)
	_, _, _, err = f.svc.AdvanceRound(ctx, charID)
	require.NoError(t, err)

	restorers := f.svc.Restorers()
	seen := map[string]bool{}
	for _, e := range f.entries {
		for _, s := range e.Snapshots {
			seen[s.Key] = true
			assert.Contains(t, restorers, s.Key, "no Restorer for state key %q", s.Key)
		}
	}
	assert.Equal(t, map[string]bool{buffsvc.StateKeyBuffs: true, buffsvc.StateKeyRound: true}, seen)
}

func TestRestorers_WriteTheSnapshotBack(t *testing.T) {
	t.Run("buffs: the recorded before state round-trips", func(t *testing.T) {
		f := newFixture(t)
		before := []buffsvc.BuffEffect{
			strength(6),
			{ID: "m", Name: "Mirror Image", Duration: 4, Charges: 2},
			powerGem,
		}
		f.state(before, 3)
		f.records()
		f.saves([]buffsvc.BuffEffect{strength(6), powerGem})
		_, err := f.svc.Dismiss(context.Background(), charID, "m")
		require.NoError(t, err)

		f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, before).Return(nil).Once()
		err = f.svc.Restorers()[buffsvc.StateKeyBuffs].Restore(context.Background(), charID, f.entries[0].Snapshots[0].Before)
		require.NoError(t, err)
	})

	t.Run("round", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 7).Return(nil)
		require.NoError(t, f.svc.Restorers()[buffsvc.StateKeyRound].Restore(context.Background(), charID, json.RawMessage(`7`)))
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
			err := f.svc.Restorers()[tt.key].Restore(context.Background(), charID, json.RawMessage(tt.state))
			assert.Error(t, err)
		})
	}
}
