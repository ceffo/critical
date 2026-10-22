package psionics

// The power check and its cost (ADR 0043 steps 10 and 11): what a d20 against
// a power's target gives, and the PSPs it spends. Pure: the roll and the
// target come in.

// CheckDie is the power check's die (step 10).
const CheckDie = 20

// MaxModifier is the largest labelled modifier on a power check, either sign
// (step 10: -20 to +20, as ADR 0014 step 5's adjustment).
const MaxModifier = 20

// Outcomes of a power check (step 10).
const (
	// OutcomeSuccess: the roll is at or under the target, or a natural 1.
	OutcomeSuccess = "success"
	// OutcomeFailure: the roll is above the target.
	OutcomeFailure = "failure"
	// OutcomePowerScore: the roll equals the target, a success with the
	// power's special result (the DM's to apply).
	OutcomePowerScore = "power_score"
	// OutcomeNatural20: a natural 20, a failure whatever the target.
	OutcomeNatural20 = "natural_20"
)

// Check is the outcome of a d20 roll against target (step 10), in the step's
// order: a natural 20 fails (10.1), a natural 1 succeeds even at a target of
// 0 or under (10.2), a roll equal to the target is the power score's special
// result (10.3), otherwise at or under the target succeeds (10.4).
func Check(target, roll int) string {
	switch {
	case roll == CheckDie:
		return OutcomeNatural20
	case roll == 1:
		return OutcomeSuccess
	case roll == target:
		return OutcomePowerScore
	case roll <= target:
		return OutcomeSuccess
	}
	return OutcomeFailure
}

// Succeeded reports whether outcome is a success: a plain one or the power
// score's.
func Succeeded(outcome string) bool {
	return outcome == OutcomeSuccess || outcome == OutcomePowerScore
}

// Spent is the PSPs a check of outcome spends of a power whose initial cost
// is cost (step 11): all of it on a success, half on a failure, rounded up
// (the primer's rounding).
func Spent(outcome string, cost int) int {
	if Succeeded(outcome) {
		return cost
	}
	return (cost + 1) / 2
}

// Knows reports whether picks hold power id as a known power (step 9).
func Knows(picks []Pick, id string) bool {
	for _, p := range picks {
		if p.Kind == PickPower && p.ID == id {
			return true
		}
	}
	return false
}

// Relearns counts the relearns of power id in picks: +1 each to its score
// (step 9, Raising Psionic Power Scores; step 10's target).
func Relearns(picks []Pick, id string) int {
	n := 0
	for _, p := range picks {
		if p.Kind == PickRelearn && p.ID == id {
			n++
		}
	}
	return n
}
