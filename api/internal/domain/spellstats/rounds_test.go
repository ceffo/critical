package spellstats_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/pkg/formula"
)

// ADR 0004 step 20.1: a catalogue duration (the Priest spells have no
// effect entry) as a formula in rounds. The inputs are forms the catalogue
// uses; every die rolls its top face so the totals show the conversion.
func TestBuffs_Step20_DurationInRounds(t *testing.T) {
	top := func(sides int) int { return sides }
	tests := []struct {
		raw     string
		formula string
		at15    int
	}{
		{"1 rd./level", "level", 15},
		{"1 round/level", "level", 15},
		{"2 rds./level", "2*level", 30},
		{"1 turn/level", "10*level", 150},
		{"1 hour/level", "60*level", 900},
		{"1 day/level", "1440*level", 21600},
		{"1 week", "10080", 10080},
		{"1 turn", "10", 10},
		{"6 rds.", "6", 6},
		{"1 turn + 1 rd./level", "10+level", 25},
		{"4 rds. + 1 rd./2 levels", "4+(level/2)", 11},
		{"1 turn/3 levels", "10*(level/3)", 50},
		{"1 1/2 hrs./level", "90*level", 1350},
		{"4 hrs. + 1 hr./2 levels", "240+60*(level/2)", 660},
		{"2d10 rounds", "2d10", 20},
		{"4d12 hours", "60*4d12", 2880},
		{"1d4+8 rounds", "(1d4+8)", 12},
		{"8 rds. + 1d8 rds.", "8+1d8", 16},
		{"3d4 turns + 1/level", "10*3d4+10*level", 270},
		{"1d12 rounds + 2 turns/level", "1d12+20*level", 312},
		{"1-4 days", "1440*1d4", 5760},
		{"1-4 rounds", "1d4", 4},
		{"3-4 rounds", "(2+1d2)", 4},
		{"up to 8 hrs.", "480", 480},
		// A cap caps the total, in the unit of the cap (20.1): below it the
		// total stands, above it the cap does, and the cap follows the
		// segment round-up base.
		{"1 turn/level, max. 10 turns", "min(10*level,100)", 100},
		{"1 hr./level, max. 12 hrs.", "min(60*level,720)", 720},
		{"1 rd./level, max. 2 turns", "min(level,20)", 15},
		// A dice term with a flat count and a step, and a range per level:
		// everything in the term scales with the levels (20.1).
		{"1d4+2 rds./2 levels", "((1*(level/2))d4+2*(level/2))", 7*4 + 14},
		{"1d4+1 rds./level", "((1*level)d4+level)", 15*4 + 15},
		{"1-4 rds./level", "1d4*level", 4 * 15},
		{"1-4 rds./2 levels", "1d4*(level/2)", 4 * 7},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, ok := spellstats.ParseDuration(tt.raw).RoundsFormula()
			require.True(t, ok)
			assert.Equal(t, tt.formula, got)
			f, err := formula.Parse(got)
			require.NoError(t, err)
			v, err := f.Eval(15, top)
			require.NoError(t, err)
			assert.Equal(t, tt.at15, v)
		})
	}
}

// The cap binds on level: 1 turn/level, max. 10 turns is 40 rounds at level 4
// and 100 at level 15 (ADR 0004 step 20.1).
func TestBuffs_Step20_CapByLevel(t *testing.T) {
	tests := []struct {
		raw   string
		level int
		want  int
	}{
		{"1 turn/level, max. 10 turns", 4, 40},
		{"1 turn/level, max. 10 turns", 10, 100},
		{"1 turn/level, max. 10 turns", 15, 100},
		{"1 hr./level, max. 12 hrs.", 4, 240},
		{"1 hr./level, max. 12 hrs.", 15, 720},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, ok := spellstats.ParseDuration(tt.raw).RoundsFormula()
			require.True(t, ok)
			f, err := formula.Parse(got)
			require.NoError(t, err)
			v, err := f.Eval(tt.level, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, v)
		})
	}
}

// ADR 0004 step 20.2: no row for a duration that is not a number of rounds.
func TestBuffs_Step20_NoRoundsFormula(t *testing.T) {
	for _, raw := range []string{
		"Instantaneous", "Permanent", "Concentration", "Special", "Until discharged",
		"Permanent until discharged", "Until used", "Time of chanting",
		"Special (up to three rounds)", "1 turn/recipient",
	} {
		t.Run(raw, func(t *testing.T) {
			_, ok := spellstats.ParseDuration(raw).RoundsFormula()
			assert.False(t, ok)
		})
	}
}

// A duration in segments rounds up to whole rounds (step 20.1).
func TestBuffs_Step20_SegmentsRoundUp(t *testing.T) {
	d := spellstats.Duration{Kind: spellstats.KindTimed, Time: spellstats.Quantity{Terms: []spellstats.Term{{Count: 3, Unit: "segment"}, {Count: 1, Unit: "round"}}}}
	got, ok := d.RoundsFormula()
	require.True(t, ok)
	assert.Equal(t, "(3+10+9)/10", got)
	f, err := formula.Parse(got)
	require.NoError(t, err)
	v, err := f.Eval(1, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, v)
}
