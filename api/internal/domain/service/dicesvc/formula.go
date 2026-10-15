package dicesvc

import (
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
)

// Limits of a formula. They keep a roll readable on the table and a request
// cheap; the composer never gets near them by clicking.
const (
	MaxFormulaLen  = 200
	MaxDicePerTerm = 100
	MaxDiceTotal   = 100
	MaxSides       = 1000
	MaxConstant    = 10000
	// PercentSides is what d% rolls.
	PercentSides = 100
)

// Group is every die of one kind (sides and sign) of a formula: Count dice
// of Sides, added (Negative false) or subtracted.
type Group struct {
	Sides    int
	Count    int
	Negative bool
}

// Formula is a parsed formula: dice groups in order of first appearance and
// the sum of its integer constants.
type Formula struct {
	Groups   []Group
	Modifier int
}

// Dice is how many dice the formula rolls.
func (f Formula) Dice() int {
	n := 0
	for _, g := range f.Groups {
		n += g.Count
	}
	return n
}

// String is the normal form: groups in order of first appearance, then the
// constant. A single die has no count, 100 sides read d% and the first term
// has no leading plus. Parsing the normal form gives the same formula back.
func (f Formula) String() string {
	var b strings.Builder
	for _, g := range f.Groups {
		switch {
		case g.Negative:
			b.WriteByte('-')
		case b.Len() > 0:
			b.WriteByte('+')
		}
		if g.Count > 1 {
			b.WriteString(strconv.Itoa(g.Count))
		}
		b.WriteByte('d')
		if g.Sides == PercentSides {
			b.WriteByte('%')
		} else {
			b.WriteString(strconv.Itoa(g.Sides))
		}
	}
	switch {
	case f.Modifier > 0:
		b.WriteByte('+')
		b.WriteString(strconv.Itoa(f.Modifier))
	case f.Modifier < 0:
		b.WriteString(strconv.Itoa(f.Modifier))
	}
	return b.String()
}

func invalid(format string, args ...any) error {
	return domain.FieldErrorf("formula", format, args...)
}

// Parse reads a formula: a sum of terms joined by + or -, each term NdM, dM,
// d% (a d100) or an integer constant. Spaces are ignored and the letter d is
// case-insensitive. At least one die is required, none may have fewer than 2
// sides, and the limits above apply. Every failure is a domain.FieldError on
// "formula".
func Parse(s string) (Formula, error) {
	if len(s) > MaxFormulaLen {
		return Formula{}, invalid("formula is longer than %d characters", MaxFormulaLen)
	}
	s = strings.ToLower(strings.Join(strings.Fields(s), ""))
	if s == "" {
		return Formula{}, invalid("formula is empty")
	}
	var f Formula
	for i := 0; i < len(s); {
		negative := false
		switch s[i] {
		case '+':
			i++
		case '-':
			negative = true
			i++
		default:
			if i > 0 {
				return Formula{}, invalid("expected + or - at position %d", i+1)
			}
		}
		end := i
		for end < len(s) && s[end] != '+' && s[end] != '-' {
			end++
		}
		if end == i {
			return Formula{}, invalid("a number or a die is missing at position %d", i+1)
		}
		if err := f.add(s[i:end], negative); err != nil {
			return Formula{}, err
		}
		i = end
	}
	if f.Dice() == 0 {
		return Formula{}, invalid("formula needs at least one die")
	}
	if f.Dice() > MaxDiceTotal {
		return Formula{}, invalid("formula rolls more than %d dice", MaxDiceTotal)
	}
	if f.Modifier > MaxConstant || f.Modifier < -MaxConstant {
		return Formula{}, invalid("constants add up to more than %d", MaxConstant)
	}
	return f, nil
}

// add folds one term (no sign, lower case) into the formula.
func (f *Formula) add(term string, negative bool) error {
	count, sides, isDie := strings.Cut(term, "d")
	if !isDie {
		n, err := atoi(term)
		if err != nil {
			return invalid("%q is not a number or a die", term)
		}
		if n > MaxConstant {
			return invalid("constant %d is larger than %d", n, MaxConstant)
		}
		if negative {
			n = -n
		}
		f.Modifier += n
		return nil
	}
	n := 1
	if count != "" {
		var err error
		if n, err = atoi(count); err != nil {
			return invalid("%q is not a number of dice", count)
		}
	}
	if n < 1 || n > MaxDicePerTerm {
		return invalid("a term rolls between 1 and %d dice", MaxDicePerTerm)
	}
	var s int
	if sides == "%" {
		s = PercentSides
	} else {
		var err error
		if s, err = atoi(sides); err != nil {
			return invalid("%q is not a number of sides", sides)
		}
	}
	if s < 2 || s > MaxSides {
		return invalid("a die has between 2 and %d sides", MaxSides)
	}
	for i := range f.Groups {
		if g := &f.Groups[i]; g.Sides == s && g.Negative == negative {
			g.Count += n
			return nil
		}
	}
	f.Groups = append(f.Groups, Group{Sides: s, Count: n, Negative: negative})
	return nil
}

// atoi reads digits only (no sign, no space, no overflow past a few digits).
func atoi(s string) (int, error) {
	if s == "" || len(s) > 6 {
		return 0, strconv.ErrSyntax
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, strconv.ErrSyntax
		}
	}
	return strconv.Atoi(s)
}
