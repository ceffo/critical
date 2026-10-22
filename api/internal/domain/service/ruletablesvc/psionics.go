package ruletablesvc

import (
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// InherentPotentialRow is CPsiH Table 5 at an ability score (ADR 0043 step
// 4): the base PSP score at the score and the modifier the score adds.
type InherentPotentialRow struct {
	Score    int
	Base     int
	Modifier int
}

// checkInherentPotential: the rows' scores ascending and contiguous, and
// rows present when a class lists psionics (ADR 0040 step 8: a table a
// feature reads is there at start).
func checkInherentPotential(rows []InherentPotentialRow, defs *classes.Registry) error {
	for i := 1; i < len(rows); i++ {
		if rows[i].Score != rows[i-1].Score+1 {
			return fmt.Errorf("row %d: score %d after %d", i, rows[i].Score, rows[i-1].Score)
		}
	}
	if len(rows) > 0 {
		return nil
	}
	return readByPsionics(defs)
}

// readByPsionics is the error for a psionicist table with no rows when a
// class lists psionics, nil when none does.
func readByPsionics(defs *classes.Registry) error {
	for _, c := range defs.Classes() {
		if _, ok := c.Lists(classes.KindPsionics); ok {
			return fmt.Errorf("no rows, read by class %s", c.ID)
		}
	}
	return nil
}

// InherentPotentialFor is Table 5 at score (ADR 0043 step 4.3, Q1): a score
// under the table has modifier 0 and no base (the class minimum keeps a
// psionicist's base ability in the table); over it, each point adds 2 to the
// last row's base and 1 to its modifier.
func (t *Tables) InherentPotentialFor(score int) (InherentPotentialRow, error) {
	c, err := t.classTables()
	if err != nil {
		return InherentPotentialRow{}, err
	}
	rows := c.InherentPotential
	if len(rows) == 0 {
		return InherentPotentialRow{}, errors.New("inherent potential: no Table 5")
	}
	first, last := rows[0], rows[len(rows)-1]
	switch {
	case score < first.Score:
		return InherentPotentialRow{Score: score, Base: -1, Modifier: 0}, nil
	case score > last.Score:
		over := score - last.Score
		return InherentPotentialRow{Score: score, Base: last.Base + 2*over, Modifier: last.Modifier + over}, nil
	}
	return rows[score-first.Score], nil
}

// PSPMax is a psionics entry's PSP maximum with its terms (ADR 0043 step 4).
type PSPMax struct {
	// InherentPotential is step 4.1: the level-1 maximum.
	InherentPotential int
	// PerLevel is step 4.2: the gain of each level from 2, PerLevelModifier (Table 5's modifier at
	// the per-level ability) plus the rule's constant.
	PerLevel         int
	PerLevelModifier int
	// Max is InherentPotential plus PerLevel for each level above 1.
	Max int
}

// PSPMaxFor is ADR 0043 step 4 for a psionics entry at level, read through
// the feature's PSP rule (step 2.2); score is the sheet's score in an ability
// code. Step 4.1: Table 5's base at the base ability plus Table 5's modifier
// at each modifier ability. Step 4.2: each level from 2 adds Table 5's
// modifier at the per-level ability plus the rule's constant, no die. Step
// 4.4: a multi-class character's gain is not divided (Q2): the caller passes
// the psionics entry's own level.
func (t *Tables) PSPMaxFor(rule classes.PSPRule, score func(ability string) int, level int) (PSPMax, error) {
	if level < 1 {
		return PSPMax{}, fmt.Errorf("psp max: level %d under 1", level)
	}
	base, err := t.InherentPotentialFor(score(rule.BaseAbility))
	if err != nil {
		return PSPMax{}, err
	}
	if base.Base < 0 {
		return PSPMax{}, fmt.Errorf("psp max: %s %d under Table 5", rule.BaseAbility, base.Score)
	}
	inherent := base.Base
	for _, a := range rule.ModifierAbilities {
		r, err := t.InherentPotentialFor(score(a))
		if err != nil {
			return PSPMax{}, err
		}
		inherent += r.Modifier
	}
	per, err := t.InherentPotentialFor(score(rule.PerLevel.Ability))
	if err != nil {
		return PSPMax{}, err
	}
	gain := per.Modifier + rule.PerLevel.Plus
	return PSPMax{InherentPotential: inherent, PerLevel: gain, PerLevelModifier: per.Modifier, Max: inherent + (level-1)*gain}, nil
}

// PSPRecoveryRow is CPsiH Table 6 for an activity (ADR 0043 step 6): the PSPs
// an hour of it recovers.
type PSPRecoveryRow struct {
	Activity string
	PerHour  int
}

// PSPActivities are Table 6's activities (ADR 0043 step 6), each once.
var PSPActivities = []string{"exertion", "walking", "resting", "sleeping"}

// checkPSPRecovery: each activity once with a rate of 0 or more, and rows
// present when a class lists psionics (ADR 0040 step 8).
func checkPSPRecovery(rows []PSPRecoveryRow, defs *classes.Registry) error {
	if len(rows) == 0 {
		return readByPsionics(defs)
	}
	for _, r := range rows {
		if r.PerHour < 0 {
			return fmt.Errorf("%s: %d PSPs an hour", r.Activity, r.PerHour)
		}
	}
	return once(rows, func(r PSPRecoveryRow) string { return r.Activity }, PSPActivities)
}

// PSPRecoveryRate is Table 6's hourly rate for activity (ADR 0043 step 6).
func (t *Tables) PSPRecoveryRate(activity string) (int, error) {
	c, err := t.classTables()
	if err != nil {
		return 0, err
	}
	for _, r := range c.PSPRecovery {
		if r.Activity == activity {
			return r.PerHour, nil
		}
	}
	return 0, fmt.Errorf("psp recovery: no Table 6 row for %q", activity)
}
