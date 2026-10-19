package spellstats

import (
	"math"
	"strconv"
	"strings"
)

// roundSegments is how many segments a round is (the time base of the unit
// table is the segment).
const roundSegments = 10

// RoundsFormula is the duration as a formula in rounds at the caster level
// ("level", "10+level", "60*4d12"), for a spell whose duration only the
// catalogue gives (ADR 0004 step 20.1: the Priest spells). It is read from
// the parsed stat line, never from text in a language. Every term is
// converted to rounds by the unit table (1 turn = 10 rounds, 1 hour = 60,
// 1 day = 1,440, 1 week = 10,080); a term per N levels counts once every N
// levels; dice are rolled in their own unit, then converted; a range ("1-4
// days") rolls evenly between its ends; "up to" takes the top; a cap caps
// the total; a part of a round rounds up to a whole round.
//
// ok is false for a duration that is not timed (instantaneous, permanent,
// concentration, "until", special) and for one it cannot convert: a unit
// that is not a time, a divisor that is not the level ("per recipient"), a
// range with dice.
func (d Duration) RoundsFormula() (string, bool) {
	if d.Kind != KindTimed || len(d.Time.Terms) == 0 {
		return "", false
	}
	// Whole rounds unless a term is in segments: then the sum is in
	// segments and rounded up to rounds.
	base := float64(roundSegments)
	for _, t := range d.Time.Terms {
		f, fam := unitFactor(t.Unit)
		if fam != "time" {
			return "", false
		}
		if f < base {
			base = f
		}
	}
	parts := make([]string, 0, len(d.Time.Terms))
	for _, t := range d.Time.Terms {
		p, ok := termFormula(t, base)
		if !ok {
			return "", false
		}
		parts = append(parts, p)
	}
	sum := strings.Join(parts, "+")
	if d.Time.Max > 0 {
		f, fam := unitFactor(d.Time.MaxUnit)
		if fam != "time" {
			return "", false
		}
		limit := int(math.Ceil(d.Time.Max * f / base))
		sum = "min(" + sum + "," + strconv.Itoa(limit) + ")"
	}
	if base < roundSegments {
		per := int(roundSegments / base)
		return "(" + sum + "+" + strconv.Itoa(per-1) + ")/" + strconv.Itoa(per), true
	}
	return sum, true
}

// termFormula is one term in the unit base (rounds, or segments): its count,
// its dice or its range, times the unit's size, per level when it scales.
func termFormula(t Term, base float64) (string, bool) {
	if t.Per != "" {
		return "", false
	}
	f, _ := unitFactor(t.Unit)
	scale, ok := whole(f / base)
	if !ok {
		return "", false
	}
	levels := ""
	if t.PerLevel {
		levels = "level"
		if t.Step > 1 {
			levels = "(level/" + strconv.Itoa(t.Step) + ")"
		}
	}
	var amount string
	switch {
	case t.Upper != 0:
		lo, ok1 := whole(t.Count)
		hi, ok2 := whole(t.Upper)
		if t.Dice != "" || !ok1 || !ok2 || hi < lo {
			return "", false
		}
		amount = "1d" + strconv.Itoa(hi-lo+1)
		if lo != 1 {
			amount = "(" + strconv.Itoa(lo-1) + "+" + amount + ")"
		}
	case t.Dice != "":
		dice := t.Dice
		if levels != "" {
			m := diceRe.FindStringSubmatch(t.Dice)
			if m == nil {
				return "", false
			}
			dice = "(" + m[1] + "*" + levels + ")d" + m[2]
		}
		amount = dice
		if t.Count != 0 {
			n, ok := whole(t.Count)
			if !ok {
				return "", false
			}
			amount = "(" + dice + "+" + scaled(n, levels) + ")"
		}
		return times(scale, amount), true
	default:
		// A fractional count ("1 1/2 hrs.") is whole once in the base.
		n, ok := whole(t.Count * float64(scale))
		if !ok {
			return "", false
		}
		return scaled(n, levels), true
	}
	return times(scale, scaled1(amount, levels)), true
}

// scaled is n, times the levels when the term scales ("3*level", "level").
func scaled(n int, levels string) string {
	switch {
	case levels == "":
		return strconv.Itoa(n)
	case n == 1:
		return levels
	default:
		return strconv.Itoa(n) + "*" + levels
	}
}

// scaled1 is an amount times the levels when the term scales.
func scaled1(amount, levels string) string {
	if levels == "" {
		return amount
	}
	return amount + "*" + levels
}

// times is the amount times the unit's size in the base.
func times(scale int, amount string) string {
	if scale == 1 {
		return amount
	}
	return strconv.Itoa(scale) + "*" + amount
}

// whole is f as an int when it is one.
func whole(f float64) (int, bool) {
	r := math.Round(f)
	if math.Abs(f-r) > 1e-9 {
		return 0, false
	}
	return int(r), true
}
