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

// ADR 0007 step 2: the total is the sum of the effective HP deltas: a plain
// sum, the latest amend in effect replacing an entry's delta, a popped entry
// counting 0 (amended or not), a summary-only or note-only amend keeping the
// delta. ADR 0018 step 4: every kind counts. ADR 0019 step 1: a redone entry
// counts again. ADR 0010 step 6: an amend whose correction step is popped is
// out of effect.
func TestHPTotal(t *testing.T) {
	amend := func(d int) actionlog.Correction {
		return actionlog.Correction{Kind: actionlog.CorrectionAmend, HPDelta: &d}
	}
	void := actionlog.Correction{Kind: actionlog.CorrectionVoid}
	redo := actionlog.Correction{Kind: actionlog.CorrectionRedo}
	note := "DM ruling"
	entry := func(kind actionlog.Kind, delta int, cs ...actionlog.Correction) actionlog.Entry {
		return actionlog.Entry{Kind: kind, HPDelta: delta, Corrections: cs}
	}
	hp := func(delta int, cs ...actionlog.Correction) actionlog.Entry {
		return entry(actionlog.KindHPChange, delta, cs...)
	}
	stepAmend := func(d int, undone bool) actionlog.Correction {
		c := amend(d)
		step := int64(9)
		c.StepID, c.StepUndone = &step, undone
		return c
	}
	cases := []struct {
		name    string
		entries []actionlog.Entry
		want    int
	}{
		{"empty", nil, 0},
		{"plain sum", []actionlog.Entry{hp(-8), hp(3), hp(-2)}, -7},
		{"amend replaces delta", []actionlog.Entry{hp(-8, amend(-12)), hp(3)}, -9},
		{"latest amend wins", []actionlog.Entry{hp(-8, amend(-12), amend(-1))}, -1},
		{"amend to zero", []actionlog.Entry{hp(-8, amend(0)), hp(-2)}, -2},
		{"void counts zero", []actionlog.Entry{hp(-8, void), hp(3)}, 3},
		{"void after amend counts zero", []actionlog.Entry{hp(-8, amend(-20), void), hp(3)}, 3},
		{"summary-only amend keeps delta", []actionlog.Entry{hp(-8, actionlog.Correction{Kind: actionlog.CorrectionAmend, Summary: "orc"})}, -8},
		{"note-only amend keeps delta", []actionlog.Entry{hp(-8, actionlog.Correction{Kind: actionlog.CorrectionAmend, Note: &note})}, -8},
		{"redone entry counts again", []actionlog.Entry{hp(-8, void, redo)}, -8},
		{"popped again counts zero", []actionlog.Entry{hp(-8, void, redo, void), hp(-1)}, -1},
		{"amend of a popped step is out of effect", []actionlog.Entry{hp(-8, stepAmend(-3, true))}, -8},
		{"amend of a live step is in effect", []actionlog.Entry{hp(-8, stepAmend(-3, false))}, -3},
		{"every kind counts", []actionlog.Entry{hp(-8), entry(actionlog.KindCast, -4), entry(actionlog.KindAttack, 0)}, -12},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, actionlog.HPTotal(c.entries))
		})
	}
}

// ADR 0007 steps 4 and 5, ADR 0011 step 1, on the total of an amended log:
// the heal cap and the damage floor see the amended delta, not the original.
func TestHPTotal_CapAndFloorSeeTheAmend(t *testing.T) {
	d := -3
	amended := []actionlog.Entry{{
		Kind: actionlog.KindHPChange, HPDelta: -12,
		Corrections: []actionlog.Correction{{Kind: actionlog.CorrectionAmend, HPDelta: &d}},
	}}
	total := actionlog.HPTotal(amended)
	require.Equal(t, -3, total)
	assert.Equal(t, 3, actionlog.CapHeal(total, 10), "the gap to max is 3 after the amend, not 12")
	assert.Equal(t, -17, actionlog.FloorDamage(10, total, -40), "from 7 HP down to -10, not from -2")
}
