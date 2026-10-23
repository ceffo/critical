package psionics

// The psychic contest (ADR 0044 step 6): an attack mode's power check meets a
// defence mode's, the higher successful roll wins and the defender wins the
// ties. Pure: the checks come in.

// Sides of a contest (step 6).
const (
	SideAttacker = "attacker"
	SideDefender = "defender"
)

// ContestSide is one side's check as the contest compares it: the roll it
// counts with and whether it counts as a success.
type ContestSide struct {
	Roll    int  `json:"roll"`
	Success bool `json:"success"`
}

// ContestCheck is a check rolled by the app for a contest (step 6): the d20
// against target, plus 1 when the power is kept from an earlier round. A
// kept power's failed roll counts as a success of 1 (the book's Maintained
// Powers); the step 10 outcomes hold (a natural 20 fails, a natural 1
// succeeds as a roll of 1, a roll equal to the target succeeds).
type ContestCheck struct {
	// Target is the score the die is rolled against, with the kept +1.
	Target  int    `json:"target"`
	Roll    int    `json:"d20"`
	Outcome string `json:"outcome"`
	// Kept: the power is kept from an earlier round (+1, a failure is 1).
	Kept bool `json:"kept,omitempty"`
}

// CheckContest is the app's check of a side: target is the power's score
// with its modifiers, before the kept +1.
func CheckContest(target, roll int, kept bool) ContestCheck {
	if kept {
		target++
	}
	return ContestCheck{Target: target, Roll: roll, Outcome: Check(target, roll), Kept: kept}
}

// Side is the check as the contest counts it.
func (c ContestCheck) Side() ContestSide {
	switch {
	case Succeeded(c.Outcome):
		return ContestSide{Roll: c.Roll, Success: true}
	case c.Kept:
		return ContestSide{Roll: 1, Success: true}
	}
	return ContestSide{Roll: c.Roll}
}

// Contest is step 6's winner: the higher successful roll; the defender when
// neither succeeds or both rolls are equal.
func Contest(attack, defence ContestSide) string {
	switch {
	case !attack.Success:
		return SideDefender
	case !defence.Success:
		return SideAttacker
	case attack.Roll > defence.Roll:
		return SideAttacker
	}
	return SideDefender
}
