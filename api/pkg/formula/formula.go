// Package formula parses and evaluates small integer expressions that may
// contain dice and the caster level, e.g. "3+level", "2d4", "(min(level,10))d6"
// or "1+(level-1)/2".
//
// Grammar:
//
//	expr    = term { ("+" | "-") term }
//	term    = unary { ("*" | "/") unary }
//	unary   = "-" unary | primary
//	primary = ( INT | "level" | "(" expr ")" ) [ "d" INT ]
//	        | ("min" | "max") "(" expr "," expr ")"
//
// A trailing "d" INT turns the preceding primary into a dice roll: the primary
// is the number of dice, the INT the number of sides ("1d6", "leveld6",
// "(min(level,10))d6"). Division is integer division truncating toward zero.
// There is no dice-per-level shorthand: write "leveld6"; "1d6/level" is
// legal but means (1d6) divided by the level.
package formula

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const (
	// maxDice bounds the number of dice one term may roll.
	maxDice = 1000
	// checkMaxLevel is the highest caster level a formula is probed with at parse time.
	checkMaxLevel = 40
)

// Roller rolls one die with the given number of sides and returns 1..sides.
type Roller func(sides int) int

// Formula is a parsed, validated expression.
type Formula struct {
	src  string
	root node
}

// String returns the source text the formula was parsed from.
func (f *Formula) String() string { return f.src }

// Parse parses src and probes it for runtime errors (division by zero, a
// negative number of dice, too many dice) at caster levels 1..40, so a formula
// that parses is safe to evaluate at any realistic level.
func Parse(src string) (*Formula, error) {
	if strings.TrimSpace(src) == "" {
		return nil, errors.New("empty formula")
	}
	p := &parser{toks: lex(src)}
	root, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("formula %q: %w", src, err)
	}
	if t := p.peek(); t.kind != tokEOF {
		return nil, fmt.Errorf("formula %q: unexpected %s", src, t)
	}
	f := &Formula{src: src, root: root}
	for level := 1; level <= checkMaxLevel; level++ {
		for _, roll := range []Roller{func(int) int { return 1 }, func(s int) int { return s }} {
			if _, err := f.Eval(level, roll); err != nil {
				return nil, fmt.Errorf("formula %q at level %d: %w", src, level, err)
			}
		}
	}
	return f, nil
}

// Eval evaluates the formula for a caster level, rolling dice with roll.
func (f *Formula) Eval(level int, roll Roller) (int, error) {
	return f.root.eval(level, roll)
}

// Min returns the smallest value the formula can take at the level (every die
// shows 1) and Max the largest (every die shows its maximum).
func (f *Formula) Min(level int) (int, error) {
	return f.Eval(level, func(int) int { return 1 })
}

// Max returns the largest value the formula can take at the level.
func (f *Formula) Max(level int) (int, error) {
	return f.Eval(level, func(s int) int { return s })
}

// ErrNotDiceSum is returned by Expand when a formula's dice are not simply
// added: a die that is multiplied, divided, negated, subtracted, capped by
// min/max, or whose count is itself rolled.
var ErrNotDiceSum = errors.New("not a sum of dice plus a constant")

// Dice is Count dice of Sides sides.
type Dice struct {
	Count int
	Sides int
}

// Sum is a formula evaluated at a caster level down to what is left to roll:
// dice groups, in the order the formula writes them, plus a constant.
type Sum struct {
	Dice     []Dice
	Constant int
}

// Expand evaluates every part of the formula that does not roll at the level
// and returns the dice still to roll plus the constant: "(min(level,10))d6"
// at level 12 is 10d6, "1d3+2*min(level,10)" is 1d3 plus 20, "1d6+4d6" keeps
// its two groups. Groups of zero dice are dropped. A formula whose dice are
// not simply added (see ErrNotDiceSum) returns that error.
func (f *Formula) Expand(level int) (Sum, error) {
	s, err := expand(f.root, level)
	if err != nil {
		return Sum{}, fmt.Errorf("expand %s at level %d: %w", f, level, err)
	}
	return s, nil
}

// noRoll is the roller of a subtree without dice: it is never called.
func noRoll(int) int { return 0 }

func expand(n node, level int) (Sum, error) {
	if !hasDice(n) {
		v, err := n.eval(level, noRoll)
		return Sum{Constant: v}, err
	}
	switch n := n.(type) {
	case diceNode:
		if hasDice(n.count) {
			return Sum{}, ErrNotDiceSum
		}
		c, err := n.count.eval(level, noRoll)
		if err != nil {
			return Sum{}, err
		}
		if c < 0 {
			return Sum{}, fmt.Errorf("negative number of dice (%d)", c)
		}
		if c > maxDice {
			return Sum{}, fmt.Errorf("too many dice (%d)", c)
		}
		if c == 0 {
			return Sum{}, nil
		}
		return Sum{Dice: []Dice{{Count: c, Sides: n.sides}}}, nil
	case binNode:
		if n.op != '+' && n.op != '-' || n.op == '-' && hasDice(n.r) {
			return Sum{}, ErrNotDiceSum
		}
		a, err := expand(n.l, level)
		if err != nil {
			return Sum{}, err
		}
		b, err := expand(n.r, level)
		if err != nil {
			return Sum{}, err
		}
		if n.op == '-' {
			return Sum{Dice: a.Dice, Constant: a.Constant - b.Constant}, nil
		}
		return Sum{Dice: slices.Concat(a.Dice, b.Dice), Constant: a.Constant + b.Constant}, nil
	default: // negNode, minMaxNode
		return Sum{}, ErrNotDiceSum
	}
}

// hasDice reports whether a subtree rolls any die.
func hasDice(n node) bool {
	switch n := n.(type) {
	case diceNode:
		return true
	case negNode:
		return hasDice(n.x)
	case binNode:
		return hasDice(n.l) || hasDice(n.r)
	case minMaxNode:
		return hasDice(n.l) || hasDice(n.r)
	default: // numNode, levelNode
		return false
	}
}

// ---- AST ----

type node interface {
	eval(level int, roll Roller) (int, error)
}

type numNode int

func (n numNode) eval(int, Roller) (int, error) { return int(n), nil }

type levelNode struct{}

func (levelNode) eval(level int, _ Roller) (int, error) { return level, nil }

type negNode struct{ x node }

func (n negNode) eval(level int, r Roller) (int, error) {
	v, err := n.x.eval(level, r)
	return -v, err
}

type binNode struct {
	op   byte
	l, r node
}

func (n binNode) eval(level int, r Roller) (int, error) {
	a, err := n.l.eval(level, r)
	if err != nil {
		return 0, err
	}
	b, err := n.r.eval(level, r)
	if err != nil {
		return 0, err
	}
	switch n.op {
	case '+':
		return a + b, nil
	case '-':
		return a - b, nil
	case '*':
		return a * b, nil
	default: // '/'
		if b == 0 {
			return 0, errors.New("division by zero")
		}
		return a / b, nil
	}
}

type minMaxNode struct {
	isMax bool
	l, r  node
}

func (n minMaxNode) eval(level int, r Roller) (int, error) {
	a, err := n.l.eval(level, r)
	if err != nil {
		return 0, err
	}
	b, err := n.r.eval(level, r)
	if err != nil {
		return 0, err
	}
	if (a > b) == n.isMax {
		return a, nil
	}
	return b, nil
}

type diceNode struct {
	count node
	sides int
}

func (n diceNode) eval(level int, r Roller) (int, error) {
	c, err := n.count.eval(level, r)
	if err != nil {
		return 0, err
	}
	if c < 0 {
		return 0, fmt.Errorf("negative number of dice (%d)", c)
	}
	if c > maxDice {
		return 0, fmt.Errorf("too many dice (%d)", c)
	}
	sum := 0
	for range c {
		sum += r(n.sides)
	}
	return sum, nil
}

// ---- lexer ----

type tokKind int

const (
	tokEOF tokKind = iota
	tokInt
	tokLevel
	tokD
	tokMin
	tokMax
	tokOp // + - * / ( ) ,
	tokBad
)

type token struct {
	kind tokKind
	text string
	num  int
}

func (t token) String() string {
	if t.kind == tokEOF {
		return "end of formula"
	}
	return strconv.Quote(t.text)
}

func lex(src string) []token {
	var toks []token
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c >= '0' && c <= '9':
			j := i
			for j < len(src) && src[j] >= '0' && src[j] <= '9' {
				j++
			}
			n, err := strconv.Atoi(src[i:j])
			if err != nil || n > 1_000_000 {
				toks = append(toks, token{kind: tokBad, text: src[i:j]})
			} else {
				toks = append(toks, token{kind: tokInt, text: src[i:j], num: n})
			}
			i = j
		case strings.ContainsRune("+-*/(),", rune(c)):
			toks = append(toks, token{kind: tokOp, text: string(c)})
			i++
		case c >= 'a' && c <= 'z':
			j := i
			for j < len(src) && src[j] >= 'a' && src[j] <= 'z' {
				j++
			}
			word := src[i:j]
			// "leveld6" is "level" "d" "6": split off the dice marker.
			if word == "leveld" && j < len(src) && src[j] >= '0' && src[j] <= '9' {
				toks = append(toks, token{kind: tokLevel, text: "level"}, token{kind: tokD, text: "d"})
				i = j
				continue
			}
			switch word {
			case "level":
				toks = append(toks, token{kind: tokLevel, text: word})
			case "d":
				toks = append(toks, token{kind: tokD, text: word})
			case "min":
				toks = append(toks, token{kind: tokMin, text: word})
			case "max":
				toks = append(toks, token{kind: tokMax, text: word})
			default:
				toks = append(toks, token{kind: tokBad, text: word})
			}
			i = j
		default:
			toks = append(toks, token{kind: tokBad, text: string(c)})
			i++
		}
	}
	return append(toks, token{kind: tokEOF})
}

// ---- parser ----

type parser struct {
	toks []token
	pos  int
}

func (p *parser) peek() token { return p.toks[p.pos] }

func (p *parser) next() token {
	t := p.toks[p.pos]
	if t.kind != tokEOF {
		p.pos++
	}
	return t
}

func (p *parser) isOp(s string) bool {
	t := p.peek()
	return t.kind == tokOp && t.text == s
}

func (p *parser) expectOp(s string) error {
	if !p.isOp(s) {
		return fmt.Errorf("expected %q, got %s", s, p.peek())
	}
	p.next()
	return nil
}

func (p *parser) parseExpr() (node, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for p.isOp("+") || p.isOp("-") {
		op := p.next().text[0]
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		left = binNode{op: op, l: left, r: right}
	}
	return left, nil
}

func (p *parser) parseTerm() (node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.isOp("*") || p.isOp("/") {
		op := p.next().text[0]
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = binNode{op: op, l: left, r: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (node, error) {
	if p.isOp("-") {
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return negNode{x}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (node, error) {
	t := p.next()
	var n node
	switch {
	case t.kind == tokInt:
		n = numNode(t.num)
	case t.kind == tokLevel:
		n = levelNode{}
	case t.kind == tokOp && t.text == "(":
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expectOp(")"); err != nil {
			return nil, err
		}
		n = inner
	case t.kind == tokMin || t.kind == tokMax:
		if err := p.expectOp("("); err != nil {
			return nil, err
		}
		l, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expectOp(","); err != nil {
			return nil, err
		}
		r, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expectOp(")"); err != nil {
			return nil, err
		}
		n = minMaxNode{isMax: t.kind == tokMax, l: l, r: r}
	default:
		return nil, fmt.Errorf("unexpected %s", t)
	}
	if p.peek().kind == tokD {
		p.next()
		s := p.next()
		if s.kind != tokInt || s.num < 1 {
			return nil, fmt.Errorf("dice need a positive number of sides, got %s", s)
		}
		n = diceNode{count: n, sides: s.num}
	}
	return n, nil
}
