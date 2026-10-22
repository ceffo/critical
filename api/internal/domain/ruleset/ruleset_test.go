package ruleset_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
)

func rows(r ...[3]int) []ruleset.CritRow {
	out := make([]ruleset.CritRow, len(r))
	for i, x := range r {
		out[i] = ruleset.CritRow{From: x[0], To: x[1], Multiplier: float64(x[2])}
	}
	return out
}

func TestValidateCritRows(t *testing.T) {
	tests := []struct {
		name    string
		in      []ruleset.CritRow
		want    []ruleset.CritRow // sorted rows when valid
		wantErr string            // substring naming the bad row
	}{
		{name: "one row covers everything", in: rows([3]int{1, 100, 2}), want: rows([3]int{1, 100, 2})},
		{
			name: "three ranges", in: rows([3]int{1, 50, 2}, [3]int{51, 90, 3}, [3]int{91, 100, 4}),
			want: rows([3]int{1, 50, 2}, [3]int{51, 90, 3}, [3]int{91, 100, 4}),
		},
		{
			name: "rows in any order are sorted", in: rows([3]int{91, 100, 4}, [3]int{1, 90, 2}),
			want: rows([3]int{1, 90, 2}, [3]int{91, 100, 4}),
		},
		{name: "single-value ranges", in: rows([3]int{1, 1, 5}, [3]int{2, 100, 2}), want: rows([3]int{1, 1, 5}, [3]int{2, 100, 2})},
		{name: "no rows", in: nil, wantErr: "at least one row"},
		{name: "gap in the middle", in: rows([3]int{1, 49, 2}, [3]int{51, 100, 3}), wantErr: "gap before row 2 (51-100): 50-50"},
		{name: "gap at the start", in: rows([3]int{2, 100, 2}), wantErr: "gap before row 1 (2-100): 1-1"},
		{name: "gap at the end", in: rows([3]int{1, 99, 2}), wantErr: "gap after row 1 (1-99): 100-100"},
		{name: "overlap by one", in: rows([3]int{1, 50, 2}, [3]int{50, 100, 3}), wantErr: "row 2 (50-100) overlaps"},
		{name: "duplicate start", in: rows([3]int{1, 100, 2}, [3]int{1, 10, 3}), wantErr: "row 2 (1-10) overlaps"},
		{name: "from below 1", in: rows([3]int{0, 100, 2}), wantErr: "row 1 (0-100) is outside 1-100"},
		{name: "to above 100", in: rows([3]int{1, 50, 2}, [3]int{51, 101, 2}), wantErr: "row 2 (51-101) is outside 1-100"},
		{name: "backwards range", in: rows([3]int{1, 100, 2}, [3]int{60, 50, 2}), wantErr: "row 2 (60-50) ends before it starts"},
		{name: "multiplier 0", in: rows([3]int{1, 50, 2}, [3]int{51, 100, 0}), wantErr: "row 2 (51-100) has multiplier 0"},
		{name: "negative multiplier", in: rows([3]int{1, 100, -3}), wantErr: "row 1 (1-100) has multiplier -3"},
		{name: "multiplier 1 is allowed", in: rows([3]int{1, 100, 1}), want: rows([3]int{1, 100, 1})},
		{name: "decimal multiplier 1.5", in: []ruleset.CritRow{{From: 1, To: 100, Multiplier: 1.5}}, want: []ruleset.CritRow{{From: 1, To: 100, Multiplier: 1.5}}},
		{name: "0.99 is below 1", in: []ruleset.CritRow{{From: 1, To: 100, Multiplier: 0.99}}, wantErr: "row 1 (1-100) has multiplier 0.99"},
		{name: "NaN multiplier", in: []ruleset.CritRow{{From: 1, To: 100, Multiplier: math.NaN()}}, wantErr: "row 1 (1-100) has multiplier NaN"},
		{name: "infinite multiplier", in: []ruleset.CritRow{{From: 1, To: 100, Multiplier: math.Inf(1)}}, wantErr: "row 1 (1-100) has multiplier +Inf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ruleset.ValidateCritRows(tt.in)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorIs(t, err, domain.ErrInvalidInput)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ADR 0041 step 2 (tests needed, one per rule): the default ruleset's value of each rule.
func TestDefault_Step2_Values(t *testing.T) {
	d := ruleset.Default()
	assert.Equal(t, "default", d.Name)
	assert.Equal(t, ruleset.CritTable{Rows: rows([3]int{1, 100, 2}), Placeholder: true}, d.Crit, "2.1: the unedited table")
	assert.Equal(t, ruleset.RacialLift{MulticlassAnyRace: true, DualclassAnyRace: true}, d.RacialLift, "2.2: both lifts on")
	assert.Equal(t, ruleset.LevelLimitsWarn, d.LevelLimits, "2.3")
	assert.Equal(t, []ruleset.SuccessEffect{{ProficiencyID: "tumbling", AC: -4, Rounds: 1}}, d.SuccessEffects, "2.4: Tumbling")
}

func TestSuccessEffectFor(t *testing.T) {
	d := ruleset.Default()
	e, ok := d.SuccessEffectFor("tumbling")
	require.True(t, ok)
	assert.Equal(t, ruleset.SuccessEffect{ProficiencyID: "tumbling", AC: -4, Rounds: 1}, e)
	_, ok = d.SuccessEffectFor("tracking")
	assert.False(t, ok)
}

// ADR 0041 step 2: the domain refuses a success effect for an unknown proficiency id, one of zero
// rounds, and one proficiency twice; the valid ones come back sorted by proficiency.
func TestValidateSuccessEffects_Step2(t *testing.T) {
	known := func(id string) bool { return id == "tumbling" || id == "tracking" }
	t.Run("valid, sorted", func(t *testing.T) {
		got, err := ruleset.ValidateSuccessEffects([]ruleset.SuccessEffect{
			{ProficiencyID: "tumbling", AC: -4, Rounds: 1}, {ProficiencyID: "tracking", AC: 1, Rounds: 2},
		}, known)
		require.NoError(t, err)
		assert.Equal(t, []ruleset.SuccessEffect{
			{ProficiencyID: "tracking", AC: 1, Rounds: 2}, {ProficiencyID: "tumbling", AC: -4, Rounds: 1},
		}, got)
	})
	for name, tc := range map[string]struct {
		in   []ruleset.SuccessEffect
		want string
	}{
		"unknown proficiency": {[]ruleset.SuccessEffect{{ProficiencyID: "nope", AC: -1, Rounds: 1}}, `"nope" is not a known proficiency`},
		"zero rounds":         {[]ruleset.SuccessEffect{{ProficiencyID: "tumbling", AC: -1, Rounds: 0}}, "at least one round"},
		"negative rounds":     {[]ruleset.SuccessEffect{{ProficiencyID: "tumbling", AC: -1, Rounds: -2}}, "at least one round"},
		"twice": {[]ruleset.SuccessEffect{
			{ProficiencyID: "tumbling", AC: -1, Rounds: 1}, {ProficiencyID: "tumbling", AC: -2, Rounds: 1},
		}, "appears twice"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ruleset.ValidateSuccessEffects(tc.in, known)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func TestCritTable_Lookup(t *testing.T) {
	table := ruleset.CritTable{Rows: rows([3]int{1, 50, 2}, [3]int{51, 100, 3})}
	r, err := table.Lookup(51)
	require.NoError(t, err)
	assert.Equal(t, 3.0, r.Multiplier)
	_, err = ruleset.CritTable{}.Lookup(10)
	assert.Error(t, err)
}
