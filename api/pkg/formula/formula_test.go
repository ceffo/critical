package formula_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/pkg/formula"
	"github.com/ceffo/critical/api/pkg/must"
)

// maxRoll makes every die show its highest face; minRoll its lowest.
func maxRoll(s int) int { return s }
func minRoll(int) int   { return 1 }

func TestEval(t *testing.T) {
	tests := []struct {
		src      string
		level    int
		wantMin  int
		wantMax  int
		wantRoll int // with a die that always shows 2 (or 1 on a d1)
	}{
		{"3+level", 5, 8, 8, 8},
		{"2*level", 7, 14, 14, 14},
		{"2d4", 1, 2, 8, 4},
		{"1d6", 1, 1, 6, 2},
		{"leveld6", 4, 4, 24, 8},
		{"(min(level,10))d6", 12, 10, 60, 20},
		{"(min(level,10))d6", 3, 3, 18, 6},
		{"1d3+2*min(level,10)", 15, 21, 23, 22},
		{"1+(level-1)/2", 1, 1, 1, 1},
		{"1+(level-1)/2", 3, 2, 2, 2},
		{"min(5,1+(level-1)/2)", 9, 5, 5, 5},
		{"5*level", 3, 15, 15, 15},
		{"-2", 9, -2, -2, -2},
		{"1d4+level/2", 9, 5, 8, 6},
		{"max(1,level/3)", 1, 1, 1, 1},
		{"(10+level)*10/10", 6, 16, 16, 16},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			f, err := formula.Parse(tt.src)
			require.NoError(t, err)
			assert.Equal(t, tt.src, f.String())
			lo, err := f.Eval(tt.level, minRoll)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMin, lo)
			hi, err := f.Eval(tt.level, maxRoll)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMax, hi)
			two, err := f.Eval(tt.level, func(s int) int { return min(2, s) })
			require.NoError(t, err)
			assert.Equal(t, tt.wantRoll, two)
		})
	}
}

func TestMinMaxHelpers(t *testing.T) {
	f := must.Do(formula.Parse("(min(level,10))d6"))
	lo, err := f.Min(20)
	require.NoError(t, err)
	hi, err := f.Max(20)
	require.NoError(t, err)
	assert.Equal(t, 10, lo)
	assert.Equal(t, 60, hi)
}

func TestParse_Errors(t *testing.T) {
	tests := map[string]string{
		"empty":            "",
		"blank":            "   ",
		"unknown word":     "2 rds./level",
		"trailing op":      "1+",
		"unbalanced paren": "(1+2",
		"extra token":      "1 2",
		"zero sides":       "2d0",
		"missing sides":    "2d",
		"division by zero": "level/0",
		"div by zero lvl2": "5/(level-2)",
		"negative dice":    "(level-5)d6",
		"too many dice":    "(level*100)d6",
		"bad min arity":    "min(1)",
		"bad char":         "1 % 2",
		"unknown func":     "avg(1,2)",
		"huge number":      "99999999999",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := formula.Parse(src)
			require.Error(t, err)
		})
	}
}

func TestParse_ErrorNamesFormula(t *testing.T) {
	_, err := formula.Parse("3 rounds")
	require.ErrorContains(t, err, `"3 rounds"`)
}

func TestExpand(t *testing.T) {
	d := func(count, sides int) formula.Dice { return formula.Dice{Count: count, Sides: sides} }
	tests := []struct {
		src   string
		level int
		want  formula.Sum
	}{
		{"(min(level,10))d6", 15, formula.Sum{Dice: []formula.Dice{d(10, 6)}}},
		{"(min(level,10))d6", 4, formula.Sum{Dice: []formula.Dice{d(4, 6)}}},
		{"1d3+2*min(level,10)", 15, formula.Sum{Dice: []formula.Dice{d(1, 3)}, Constant: 20}},
		{"1d8+level", 15, formula.Sum{Dice: []formula.Dice{d(1, 8)}, Constant: 15}},
		{"leveld4+level", 15, formula.Sum{Dice: []formula.Dice{d(15, 4)}, Constant: 15}},
		{"1d6+4d6", 15, formula.Sum{Dice: []formula.Dice{d(1, 6), d(4, 6)}}},
		{"5d4+2*min(level,15)", 20, formula.Sum{Dice: []formula.Dice{d(5, 4)}, Constant: 30}},
		{"(min(level/2,6))d6", 1, formula.Sum{}}, // zero dice are dropped
		{"min(level,10)", 15, formula.Sum{Constant: 10}},
		{"2d4-1", 1, formula.Sum{Dice: []formula.Dice{d(2, 4)}, Constant: -1}},
		{"3+1d4", 1, formula.Sum{Dice: []formula.Dice{d(1, 4)}, Constant: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			got, err := must.Do(formula.Parse(tt.src)).Expand(tt.level)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExpandMatchesEval: the expansion and Eval agree on the lowest and the
// highest roll, so the served dice are the ones a cast rolls.
func TestExpandMatchesEval(t *testing.T) {
	for _, src := range []string{"1d3+2*min(level,10)", "leveld4+level", "1d6+4d6", "(min(level,12))d4", "2d4+level"} {
		f := must.Do(formula.Parse(src))
		for level := 1; level <= 20; level++ {
			s, err := f.Expand(level)
			require.NoError(t, err)
			lo, hi := s.Constant, s.Constant
			for _, g := range s.Dice {
				lo += g.Count
				hi += g.Count * g.Sides
			}
			assert.Equal(t, must.Do(f.Min(level)), lo, "%s min at %d", src, level)
			assert.Equal(t, must.Do(f.Max(level)), hi, "%s max at %d", src, level)
		}
	}
}

func TestExpandRejectsDiceNotAdded(t *testing.T) {
	for _, src := range []string{"2*1d4", "1d6/2", "-1d4", "10-1d4", "min(1d6,3)", "(1d4)d6"} {
		t.Run(src, func(t *testing.T) {
			_, err := must.Do(formula.Parse(src)).Expand(5)
			assert.ErrorIs(t, err, formula.ErrNotDiceSum)
		})
	}
}

// ADR 0008 step 15: the step variable is the 1-based number of a dice step,
// accepted only by ParseStep, which probes every step up to MaxSteps (a dice
// count going negative at a later step is refused, so the data caps it).
func TestParseStep(t *testing.T) {
	_, err := formula.Parse("(step)d6")
	require.ErrorContains(t, err, "step is only allowed in a dice step")

	f := must.Do(formula.ParseStep("(max(min(level,12)-2*(step-1),0))d4"))
	d4 := func(n int) formula.Sum { return formula.Sum{Dice: []formula.Dice{{Count: n, Sides: 4}}} }
	for _, tt := range []struct {
		level, step int
		want        formula.Sum
	}{{12, 1, d4(12)}, {12, 2, d4(10)}, {12, 6, d4(2)}, {12, 7, formula.Sum{}}, {20, 1, d4(12)}, {5, 3, d4(1)}} {
		got, err := f.ExpandStep(tt.level, tt.step)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got, "level %d step %d", tt.level, tt.step)
		v, err := f.EvalStep(tt.level, tt.step, maxRoll)
		require.NoError(t, err)
		if len(tt.want.Dice) > 0 {
			assert.Equal(t, 4*tt.want.Dice[0].Count, v)
		}
	}

	_, err = formula.ParseStep("(min(level,12)-2*(step-1))d4")
	require.ErrorContains(t, err, "negative number of dice", "step 7 at level 12 would roll -2d4")
	_, err = formula.ParseStep("(step*20)d6")
	require.ErrorContains(t, err, "too many dice", "2000 dice at step 100")
	_, err = formula.ParseStep("(stepx)d6")
	require.Error(t, err)
}

func TestUsesLevel(t *testing.T) {
	tests := map[string]bool{
		"1d12": false, "2d4+1": false, "3": false, "max(1d6,2)": false,
		"level": true, "leveld6": true, "1d6+level": true, "(min(level,10))d6": true, "-level": true,
	}
	for src, want := range tests {
		t.Run(src, func(t *testing.T) {
			assert.Equal(t, want, must.Do(formula.Parse(src)).UsesLevel())
		})
	}
}
