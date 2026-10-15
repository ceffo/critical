package actionlog_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// HP-1/HP-2: total is SUM(effective hp_delta); HP is at max
// when it is 0.
func TestCapHeal(t *testing.T) {
	cases := []struct {
		name               string
		total, delta, want int
	}{
		{"heal at max-1", -1, 6, 1},
		{"heal exactly to max", -7, 7, 7},
		{"heal bigger than the gap", -5, 12, 5},
		{"heal smaller than the gap", -9, 4, 4},
		{"heal at full HP logs 0", 0, 9, 0},
		{"heal above max (legacy rows) logs 0", 3, 8, 0},
		{"damage unchanged at max", 0, -3, -3},
		{"damage unchanged below max", -11, -2, -2},
		{"zero unchanged", -4, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, actionlog.CapHeal(c.total, c.delta))
		})
	}
}

// ADR 0011 step 1: current HP is maxHP+total and never goes below -10.
func TestFloorDamage(t *testing.T) {
	cases := []struct {
		name                      string
		maxHP, total, delta, want int
	}{
		{"damage at full HP unchanged", 40, 0, -13, -13},
		{"damage to exactly 0 unchanged", 40, -30, -10, -10},
		{"damage below 0 unchanged", 40, -38, -7, -7},
		{"damage exactly to the floor unchanged", 40, -30, -20, -20},
		{"damage past the floor clamped", 40, -30, -21, -20},
		{"one past the floor from -9", 40, -49, -2, -1},
		{"damage at the floor logs 0", 40, -50, -6, 0},
		{"damage below the floor (legacy rows) logs 0", 40, -60, -1, 0},
		{"heal unchanged", 40, -50, 5, 5},
		{"zero unchanged", 40, -50, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, actionlog.FloorDamage(c.maxHP, c.total, c.delta))
			assert.GreaterOrEqual(t, c.maxHP+c.total+actionlog.FloorDamage(c.maxHP, c.total, c.delta),
				min(actionlog.HPFloor, c.maxHP+c.total), "never lands below the floor")
		})
	}
}

func TestWithHPBreakdown(t *testing.T) {
	got, err := actionlog.WithHPBreakdown(nil, actionlog.HPBreakdown{Requested: 12, Applied: 5})
	require.NoError(t, err)
	assert.JSONEq(t, `{"hpRequested":12,"hpApplied":5}`, string(got))

	got, err = actionlog.WithHPBreakdown(json.RawMessage(`{"spell":"cure"}`), actionlog.HPBreakdown{Requested: 9, Applied: 0})
	require.NoError(t, err)
	assert.JSONEq(t, `{"spell":"cure","hpRequested":9,"hpApplied":0}`, string(got))

	for _, bad := range []string{`[1]`, `null`, `3`} {
		_, err = actionlog.WithHPBreakdown(json.RawMessage(bad), actionlog.HPBreakdown{Requested: 1, Applied: 1})
		require.Error(t, err, bad)
	}
}
