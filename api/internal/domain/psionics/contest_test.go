package psionics_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/psionics"
)

// ADR 0044 step 6: the higher successful roll wins, the defender takes the
// ties and the double failures. The book's example: an attack at 15 against
// a defence at 12.
func TestContest_Step6_HigherSuccessWins(t *testing.T) {
	cases := []struct {
		name          string
		attack, def   int
		want          string
		attackOutcome string
	}{
		{"14 against 9: the higher success, the attacker", 14, 9, psionics.SideAttacker, psionics.OutcomeSuccess},
		{"9 against 11: the higher success, the defender", 9, 11, psionics.SideDefender, psionics.OutcomeSuccess},
		{"8 against 8: equal rolls, the defender", 8, 8, psionics.SideDefender, psionics.OutcomeSuccess},
		{"16 against 13: both fail, the defender", 16, 13, psionics.SideDefender, psionics.OutcomeFailure},
		{"10 against 13: the defence fails, the attacker", 10, 13, psionics.SideAttacker, psionics.OutcomeSuccess},
		{"17 against 3: the attack fails, the defender", 17, 3, psionics.SideDefender, psionics.OutcomeFailure},
		{"15 against 12: both on the score, the higher", 15, 12, psionics.SideAttacker, psionics.OutcomePowerScore},
		{"a natural 20 fails at any score", 20, 13, psionics.SideDefender, psionics.OutcomeNatural20},
		{"a natural 1 against a defence that fails: the attacker", 1, 13, psionics.SideAttacker, psionics.OutcomeSuccess},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := psionics.CheckContest(15, c.attack, false)
			d := psionics.CheckContest(12, c.def, false)
			assert.Equal(t, c.attackOutcome, a.Outcome)
			assert.Equal(t, c.want, psionics.Contest(a.Side(), d.Side()))
		})
	}
}

// ADR 0044 step 6: a power kept from an earlier round checks at +1 and its
// failed roll counts as a success of 1.
func TestContest_Step6_AKeptPowerPlusOneAndFailedIsOne(t *testing.T) {
	// Kept at 12: the target is 13, so 13 is the power score, not a failure.
	d := psionics.CheckContest(12, 13, true)
	assert.Equal(t, 13, d.Target)
	assert.Equal(t, psionics.OutcomePowerScore, d.Outcome)
	assert.Equal(t, psionics.ContestSide{Roll: 13, Success: true}, d.Side())
	fresh := psionics.CheckContest(12, 13, false)
	assert.Equal(t, psionics.OutcomeFailure, fresh.Outcome)
	assert.Equal(t, psionics.ContestSide{Roll: 13}, fresh.Side())

	// A kept failure is a success of 1: it beats an attack that fails...
	failed := psionics.CheckContest(12, 18, true)
	assert.Equal(t, psionics.OutcomeFailure, failed.Outcome)
	assert.Equal(t, psionics.ContestSide{Roll: 1, Success: true}, failed.Side())
	attack := psionics.CheckContest(15, 17, false)
	assert.Equal(t, psionics.SideDefender, psionics.Contest(attack.Side(), failed.Side()))
	// ...and loses to any success above 1.
	assert.Equal(t, psionics.SideAttacker, psionics.Contest(psionics.CheckContest(15, 2, false).Side(), failed.Side()))
	// A kept attack's failure, against a defence that fails, wins.
	keptAttack := psionics.CheckContest(15, 19, true)
	assert.Equal(t, psionics.SideAttacker, psionics.Contest(keptAttack.Side(), psionics.CheckContest(12, 14, false).Side()))
	// A natural 20 kept is still a failure, counted as 1.
	assert.Equal(t, psionics.ContestSide{Roll: 1, Success: true}, psionics.CheckContest(12, 20, true).Side())
}
