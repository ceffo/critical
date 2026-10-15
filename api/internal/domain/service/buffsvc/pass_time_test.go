package buffsvc_test

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRoundsForHours(t *testing.T) {
	tests := []struct {
		hours float64
		want  int
	}{
		{1.5, 90},
		{0.02, 1},    // 1.2 rounds, down
		{0.25, 15},   // a quarter of an hour
		{1.15, 69},   // 68.99999999999999 in floating point
		{0.1, 6},     // 6.000000000000001
		{2.999, 179}, // 179.94, down
		{8, 480},
		{720, 43200}, // the cap is allowed
	}
	for _, tt := range tests {
		got, err := buffsvc.RoundsForHours(tt.hours)
		require.NoError(t, err, "%g h", tt.hours)
		assert.Equal(t, tt.want, got, "%g h", tt.hours)
	}

	for name, hours := range map[string]float64{
		"zero":            0,
		"negative":        -1,
		"under one round": 0.01, // 0.6 rounds
		"over the cap":    720.01,
		"NaN":             math.NaN(),
		"infinite":        math.Inf(1),
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			_, err := buffsvc.RoundsForHours(hours)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
		})
	}

	t.Run("0.01 h says it is less than one round", func(t *testing.T) {
		_, err := buffsvc.RoundsForHours(0.01)
		assert.ErrorContains(t, err, "less than one round")
	})
}

func TestPassTime(t *testing.T) {
	stoneskin := buffsvc.BuffEffect{ID: "c", Name: "Stoneskin", Charges: 5}
	gem := buffsvc.BuffEffect{ID: "g", Name: "Power Gem"}
	tests := []struct {
		name        string
		hours       float64
		start       []buffsvc.BuffEffect
		round       int
		wantRounds  int
		wantActive  []buffsvc.BuffEffect
		wantExpired []buffsvc.BuffEffect
		wantPayload string
	}{
		{
			name:  "1.5 h: 100 rounds keeps 10, 60 expires, charge-only and untimed untouched",
			hours: 1.5,
			start: []buffsvc.BuffEffect{
				{ID: "a", Name: "Long", Duration: 100},
				{ID: "b", Name: "Short", Duration: 60},
				stoneskin,
				gem,
			},
			round:      4,
			wantRounds: 90,
			wantActive: []buffsvc.BuffEffect{{ID: "a", Name: "Long", Duration: 10}, stoneskin, gem},
			wantExpired: []buffsvc.BuffEffect{
				{ID: "b", Name: "Short", Duration: 0},
			},
			wantPayload: `{"action":"pass_time","hours":1.5,"rounds":90,"fromRound":4,"toRound":0,"expired":["Short"]}`,
		},
		{
			name:        "exactly the rounds left expires (edge at 0)",
			hours:       0.5,
			start:       []buffsvc.BuffEffect{{ID: "a", Name: "Bless", Duration: 30}, {ID: "b", Name: "Haste", Duration: 31}},
			round:       2,
			wantRounds:  30,
			wantActive:  []buffsvc.BuffEffect{{ID: "b", Name: "Haste", Duration: 1}},
			wantExpired: []buffsvc.BuffEffect{{ID: "a", Name: "Bless", Duration: 0}},
			wantPayload: `{"action":"pass_time","hours":0.5,"rounds":30,"fromRound":2,"toRound":0,"expired":["Bless"]}`,
		},
		{
			name:        "a timed buff with charges (Mirror Image) expires with its rounds",
			hours:       0.02,
			start:       []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 1, Charges: 3}},
			round:       0,
			wantRounds:  1,
			wantActive:  []buffsvc.BuffEffect{},
			wantExpired: []buffsvc.BuffEffect{{ID: "m", Name: "Mirror Image", Duration: 0, Charges: 3}},
			wantPayload: `{"action":"pass_time","hours":0.02,"rounds":1,"fromRound":0,"toRound":0,"expired":["Mirror Image"]}`,
		},
		{
			name:        "no buffs",
			hours:       8,
			round:       7,
			wantRounds:  480,
			wantActive:  []buffsvc.BuffEffect{},
			wantExpired: []buffsvc.BuffEffect{},
			wantPayload: `{"action":"pass_time","hours":8,"rounds":480,"fromRound":7,"toRound":0}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.state(tt.start, tt.round)
			f.records()
			f.saves(tt.wantActive)
			f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 0).Return(nil).Once()

			active, expired, rounds, round, err := f.svc.PassTime(context.Background(), charID, tt.hours)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRounds, rounds)
			assert.Zero(t, round, "the round counter goes to 0")
			assert.Equal(t, tt.wantActive, active)
			assert.Equal(t, tt.wantExpired, expired)

			require.Len(t, f.entries, 1)
			e := f.entries[0]
			assert.Equal(t, actionlog.KindTimePassed, e.Kind)
			assert.Empty(t, e.Summary, "the UI words the entry")
			assert.JSONEq(t, tt.wantPayload, string(e.Payload))
			assert.Equal(t, []string{buffsvc.StateKeyBuffs, buffsvc.StateKeyRound}, snapKeys(e),
				"void restores both the buffs and the round")
			assert.JSONEq(t, jsonInt(tt.round), string(e.Snapshots[1].Before))
			assert.JSONEq(t, `0`, string(e.Snapshots[1].After))
			assert.Len(t, decodeSnap(t, e.Snapshots[0].Before), len(tt.start))

			var p map[string]any
			require.NoError(t, json.Unmarshal(e.Payload, &p))
			assert.Equal(t, "pass_time", p["action"])
			assert.InDelta(t, tt.hours, p["hours"], 1e-12)
			assert.EqualValues(t, tt.wantRounds, p["rounds"])
			assert.EqualValues(t, tt.round, p["fromRound"])
			assert.EqualValues(t, 0, p["toRound"])
			assert.Equal(t, 1, f.tx.calls)
		})
	}

	t.Run("invalid hours write nothing", func(t *testing.T) {
		f := newFixture(t) // no DAL or log expectations: any call fails the test
		for _, h := range []float64{0, -2, 0.01, 721} {
			_, _, _, _, err := f.svc.PassTime(context.Background(), charID, h)
			assert.ErrorIs(t, err, domain.ErrInvalidInput, "%g h", h)
		}
		assert.Zero(t, f.tx.calls)
	})
}
