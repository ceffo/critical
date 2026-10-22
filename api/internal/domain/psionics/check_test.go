package psionics_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/psionics"
)

// ADR 0043 step 10: the four outcomes, in the step's order.
func TestCheck_Step10_OutcomesInTheStepsOrder(t *testing.T) {
	cases := []struct {
		name         string
		target, roll int
		want         string
	}{
		{"a natural 20 fails at a target of 25", 25, 20, psionics.OutcomeNatural20},
		{"a natural 20 fails at a target of 20", 20, 20, psionics.OutcomeNatural20},
		{"a natural 1 succeeds at a target of -2", -2, 1, psionics.OutcomeSuccess},
		{"a natural 1 succeeds at a target of 0", 0, 1, psionics.OutcomeSuccess},
		{"a natural 1 at a target of 1 is a plain success (10.2 before 10.3)", 1, 1, psionics.OutcomeSuccess},
		{"a roll equal to 14 is the power score", 14, 14, psionics.OutcomePowerScore},
		{"15 against 14 fails", 14, 15, psionics.OutcomeFailure},
		{"13 against 14 succeeds", 14, 13, psionics.OutcomeSuccess},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, psionics.Check(c.target, c.roll))
		})
	}
	assert.True(t, psionics.Succeeded(psionics.OutcomeSuccess))
	assert.True(t, psionics.Succeeded(psionics.OutcomePowerScore))
	assert.False(t, psionics.Succeeded(psionics.OutcomeFailure))
	assert.False(t, psionics.Succeeded(psionics.OutcomeNatural20))
}

// ADR 0043 step 11: a success spends the initial cost, a failure half of it,
// rounded up; a natural 20 is a failure.
func TestSpent_Step11_HalfRoundedUpOnAFailure(t *testing.T) {
	assert.Equal(t, 7, psionics.Spent(psionics.OutcomeSuccess, 7))
	assert.Equal(t, 7, psionics.Spent(psionics.OutcomePowerScore, 7))
	assert.Equal(t, 4, psionics.Spent(psionics.OutcomeFailure, 7))
	assert.Equal(t, 4, psionics.Spent(psionics.OutcomeNatural20, 7))
	assert.Equal(t, 5, psionics.Spent(psionics.OutcomeFailure, 10))
	assert.Equal(t, 1, psionics.Spent(psionics.OutcomeFailure, 1))
}

// ADR 0043 steps 9 and 10: a power is known by its power pick; each relearn
// is +1 to its score.
func TestPowerCheck_Step10_KnownAndRelearns(t *testing.T) {
	list := append(disc("telepathy"), append(pow("s1", "d1"), picks(psionics.PickRelearn, "d1", "d1")...)...)
	assert.True(t, psionics.Knows(list, "d1"))
	assert.False(t, psionics.Knows(list, "d2"))
	assert.False(t, psionics.Knows(list, "telepathy"), "a discipline is not a power")
	assert.Equal(t, 2, psionics.Relearns(list, "d1"))
	assert.Equal(t, 0, psionics.Relearns(list, "s1"))
}
