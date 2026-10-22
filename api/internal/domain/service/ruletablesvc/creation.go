package ruletablesvc

import (
	"fmt"
	"slices"
)

// The rule data of the creation rolls (ADR 0032 steps 6 and 7): the six
// ability score methods of DMG chapter 1, PHB Table 8 read by race, and the
// hit point arithmetic of one hit die (ADR 0030 step 7.1) that the level-up
// roll and the creation's max HP roll share.

// How a method's totals become scores (ADR 0032 step 6, "The player" column).
const (
	// AssignInOrder: the totals are the scores, in ability order (method I).
	AssignInOrder = "in_order"
	// AssignKeepPerAbility: each ability gets its own totals, in ability
	// order, and the player keeps one of them (method II).
	AssignKeepPerAbility = "keep_per_ability"
	// AssignArrange: the player picks Keep of the totals and assigns them to
	// the abilities (methods III, IV and V).
	AssignArrange = "arrange"
	// AssignAddToBase: every ability starts at Base and the player adds each
	// total (one die), whole, to one ability, none above Max (method VI).
	AssignAddToBase = "add_to_base"
)

// ScoreMethod is one of DMG chapter 1's ability score methods: Rolls totals
// of Dice d6 each, the lowest die of each dropped when DropLowest, assigned
// as Assign says.
type ScoreMethod struct {
	Code string
	// Rolls is the number of totals; Dice the d6 each total adds.
	Rolls, Dice int
	// DropLowest drops the lowest die of each total (one of them on a tie).
	DropLowest bool
	Assign     string
	// Keep is how many totals the player keeps (6 for every method but VI,
	// whose dice all go to the abilities).
	Keep int
	// Base and Max are method VI's: the score every ability starts at and
	// the highest one the dice may raise it to. 0 for the other methods.
	Base, Max int
}

// ScoreMethods are ADR 0032 step 6's table, in the book's order.
var ScoreMethods = []ScoreMethod{
	{Code: "I", Rolls: 6, Dice: 3, Assign: AssignInOrder, Keep: 6},
	{Code: "II", Rolls: 12, Dice: 3, Assign: AssignKeepPerAbility, Keep: 6},
	{Code: "III", Rolls: 6, Dice: 3, Assign: AssignArrange, Keep: 6},
	{Code: "IV", Rolls: 12, Dice: 3, Assign: AssignArrange, Keep: 6},
	{Code: "V", Rolls: 6, Dice: 4, DropLowest: true, Assign: AssignArrange, Keep: 6},
	{Code: "VI", Rolls: 7, Dice: 1, Assign: AssignAddToBase, Keep: 7, Base: 8, Max: 18},
}

// ScoreMethodFor is the method of code; false for none.
func ScoreMethodFor(code string) (ScoreMethod, bool) {
	i := slices.IndexFunc(ScoreMethods, func(m ScoreMethod) bool { return m.Code == code })
	if i < 0 {
		return ScoreMethod{}, false
	}
	return ScoreMethods[i], true
}

// RaceAdjustmentsFor is PHB Table 8 for race: the adjustment of each of the
// six abilities, 0 where the table has none (every ability of a human). An
// error for a race the catalogue does not define.
func (t *Tables) RaceAdjustmentsFor(race string) (map[string]int, error) {
	c, err := t.classTables()
	if err != nil {
		return nil, err
	}
	d, err := t.Definitions()
	if err != nil {
		return nil, err
	}
	if !slices.Contains(d.RaceIDs(), race) {
		return nil, fmt.Errorf("race adjustments: unknown race %q", race)
	}
	out := make(map[string]int, len(Abilities))
	for _, a := range Abilities {
		out[a] = 0
	}
	for _, r := range c.RaceAdjustments {
		if r.Race == race {
			out[r.Ability] = r.Adjustment
		}
	}
	return out, nil
}

// HitDieGain is ADR 0030 step 7.1's arithmetic for one hit die rolled by a
// character with the Constitution row con: the die counted (Table
// 3's footnotes from Constitution 20 raise a low die to con.LowDieMin), the
// Constitution adjustment (the bracketed value for a class whose definition
// says so, the plain one for every other) and the gain, at least 1.
func HitDieGain(bracketed bool, rolled int, con ConstitutionRow) (counted, conAdjustment, gain int) {
	counted = max(rolled, con.LowDieMin)
	conAdjustment = con.HP
	if bracketed {
		conAdjustment = con.HPWarrior
	}
	return counted, conAdjustment, max(1, counted+conAdjustment)
}

// SharedGain is ADR 0042 step 10.2 and 10.3 (amending ADR 0030 steps 7.1 and
// 7.2) on a level-up of one entry of a character of classes classes: the
// die counted (or a fixed gain, conAdjustment 0) divided by the number of
// classes, rounded down, at least 1 (a hit die never yields less than 1); the
// Constitution adjustment's share, the adjustment divided by the number of
// classes, rounded down (Q7: towards minus infinity for a negative one); and
// the gain, at least 1 as step 7.1's. One class is ADR 0030 step 7.1 and 7.2
// unchanged: the die (or fixed gain) and the whole adjustment.
func SharedGain(die, conAdjustment, classes int) (dieShare, conShare, gain int) {
	if classes <= 1 {
		return die, conAdjustment, max(1, die+conAdjustment)
	}
	dieShare = max(1, die/classes)
	conShare = floorDiv(conAdjustment, classes)
	return dieShare, conShare, max(1, dieShare+conShare)
}

// floorDiv is a / b rounded towards minus infinity (b > 0).
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}
