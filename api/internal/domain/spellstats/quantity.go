// Package spellstats parses the PHB stat-block text of a spell (casting
// time, range, duration, area of effect, components, saving throw) into typed
// values and computes them for a caster level. It is pure: the parsers run
// once, when the spells are seeded, and the typed result is stored. Every
// type keeps the raw PHB text, and text that is "Special" or that the parser
// cannot read becomes an explicit special kind, never an error.
package spellstats

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Term is one summand of a Quantity: "3 rds.", "1 rd./level", "1 rd./2
// levels", "1d4 turns", "1d4+1 hrs.".
type Term struct {
	Count float64 `json:"count"`
	// Upper is the top of a range term ("1 to 4 days", "3-4"); 0 when none.
	Upper float64 `json:"upper,omitempty"`
	// Dice is the dice part ("1d4"); Count holds the flat modifier.
	Dice string `json:"dice,omitempty"`
	Unit string `json:"unit,omitempty"`
	// PerLevel makes Count a coefficient applied once every Step levels.
	PerLevel bool `json:"per_level,omitempty"`
	Step     int  `json:"step,omitempty"`
	// Per is a non-level divisor ("recipient", "seed"); it is kept, not applied.
	Per string `json:"per,omitempty"`
}

// Quantity is a sum of Terms, with an optional cap.
type Quantity struct {
	Terms []Term `json:"terms"`
	// UpTo marks "up to N" phrasing.
	UpTo bool `json:"up_to,omitempty"`
	// OrMore marks "1 or more creatures": the count is a floor, not a value.
	OrMore  bool    `json:"or_more,omitempty"`
	Max     float64 `json:"max,omitempty"`
	MaxUnit string  `json:"max_unit,omitempty"`
}

// Value is a Quantity computed for a caster level, expressed in Unit.
type Value struct {
	Amount float64 `json:"amount"`
	// Upper is the top of a range ("3-4"); 0 when the value is not a range.
	Upper float64 `json:"upper,omitempty"`
	// Dice is the dice still to roll on top of Amount ("1d4"); empty when none.
	Dice string `json:"dice,omitempty"`
	Unit string `json:"unit,omitempty"`
}

// String renders "210 yds", "3 rounds + 1d4", "1d4 turns".
func (v Value) String() string {
	var parts []string
	if v.Amount != 0 || v.Dice == "" {
		parts = append(parts, strconv.FormatFloat(round2(v.Amount), 'f', -1, 64))
		if v.Upper != 0 {
			parts[0] += "-" + strconv.FormatFloat(round2(v.Upper), 'f', -1, 64)
		}
	}
	if v.Dice != "" {
		parts = append(parts, v.Dice)
	}
	s := strings.Join(parts, " + ")
	if v.Unit != "" {
		s += " " + v.Unit
	}
	return s
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// unit tables: how many base units one unit is. Time base is the segment
// (a round is 10 segments); length base is the inch.
var (
	timeUnits   = map[string]float64{"segment": 1, "round": 10, "turn": 100, "hour": 600, "day": 14400, "week": 100800}
	lengthUnits = map[string]float64{"inch": 1, "foot": 12, "yard": 36, "mile": 63360}
)

var unitAliases = map[string]string{
	"seg": "segment", "segs": "segment", "segment": "segment", "segments": "segment",
	"rd": "round", "rds": "round", "round": "round", "rounds": "round",
	"turn": "turn", "turns": "turn",
	"hr": "hour", "hrs": "hour", "hour": "hour", "hours": "hour",
	"day": "day", "days": "day", "week": "week", "weeks": "week",
	"yd": "yard", "yds": "yard", "yard": "yard", "yards": "yard",
	"ft": "foot", "foot": "foot", "feet": "foot",
	"mi": "mile", "mile": "mile", "miles": "mile",
	"in": "inch", "inch": "inch", "inches": "inch",
	// Not units: the per-level divisor ("1 rd./level", "1 hour/lvl").
	"level": "level", "levels": "level", "lvl": "level", "lvls": "level",
}

// normUnit canonicalizes a unit word; unknown words are kept lowercased.
func normUnit(s string) string {
	s = strings.ToLower(strings.TrimRight(strings.TrimSpace(s), "."))
	if u, ok := unitAliases[s]; ok {
		return u
	}
	return s
}

// unitFactor returns the base-unit factor and the table family of u.
func unitFactor(u string) (float64, string) {
	if f, ok := timeUnits[u]; ok {
		return f, "time"
	}
	if f, ok := lengthUnits[u]; ok {
		return f, "length"
	}
	return 0, ""
}

var (
	termRe       = regexp.MustCompile(`^(\d+ \d+/\d+|\d+/\d+|\d+(?:d\d+)?(?:\+\d+(?:d\d+)?)*)(?:\s*(?:-|to)\s*(\d+))?\s*([A-Za-z]+\.?)?(?:\s*/\s*(?:(\d+)\s*)?([A-Za-z]+\.?))?$`)
	termSplit    = regexp.MustCompile(`\s+\+\s*|\s*\+\s+`)
	maxRe        = regexp.MustCompile(`(?i)\s*[,(]?\s*(?:max\.?\s*(\d+)\s*([a-z]+)\.?|(\d+)\s*([a-z]+)\.?\s*max(?:imum)?\.?)\)?\s*$`)
	diceRe       = regexp.MustCompile(`^(\d+)d(\d+)$`)
	hyphenUnitRe = regexp.MustCompile(`(\d)-([A-Za-z])`)
	upToRe       = regexp.MustCompile(`(?i)^up to\s+`)
)

var errUnparsed = errors.New("unparseable")

var (
	dashRe    = regexp.MustCompile(`[\x{2012}-\x{2015}\x{2212}]`)
	feetMark  = regexp.MustCompile(`(\d)'`)
	inchMark  = regexp.MustCompile(`(\d+(?:\.\d+)?)"`)
	trailMax  = regexp.MustCompile(`(?i)\s+max(?:imum|\.)?$`)
	leadMax   = regexp.MustCompile(`(?i)^max(?:imum|\.)?\s+`)
	numWordRe = regexp.MustCompile(`(?i)\b(one|two|three|four|five|six|seven|eight|nine|ten)\b`)
	specialRe = regexp.MustCompile(`(?i)^special\s*(?:\((.+)\)|[;:,]\s*(.+))$`)
)

var numWords = map[string]string{
	"one": "1", "two": "2", "three": "3", "four": "4", "five": "5",
	"six": "6", "seven": "7", "eight": "8", "nine": "9", "ten": "10",
}

// yardsPerInch is the PHB scale for spell text in inches: 1" is 10 yards.
const yardsPerInch = 10

// normText folds the typographic variants of PHB stat text into the plain
// forms the parsers read: dashes become "-", 30' becomes "30 ft." and the
// inch mark 3" becomes yards at the PHB scale (1" = 10 yds.).
func normText(s string) string {
	s = dashRe.ReplaceAllString(s, "-")
	s = feetMark.ReplaceAllString(s, "$1 ft.")
	return inchMark.ReplaceAllStringFunc(s, func(m string) string {
		n, _ := strconv.ParseFloat(strings.TrimSuffix(m, `"`), 64)
		return strconv.FormatFloat(n*yardsPerInch, 'f', -1, 64) + " yds."
	})
}

// specialDetail reads the number a "Special (8 turns)" style text keeps:
// "Special (up to three rounds)", "Special (2 rounds/level maximum)",
// "Special; max. 10 rounds". The field stays special; the quantity only
// preserves what the book says. ok is false for text with no such number.
func specialDetail(raw, defUnit string) (Quantity, bool) {
	m := specialRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return Quantity{}, false
	}
	inner := normText(strings.TrimSpace(m[1] + m[2]))
	inner = numWordRe.ReplaceAllStringFunc(inner, func(w string) string { return numWords[strings.ToLower(w)] })
	upTo := false
	if trailMax.MatchString(inner) {
		inner, upTo = trailMax.ReplaceAllString(inner, ""), true
	}
	if leadMax.MatchString(inner) {
		inner, upTo = leadMax.ReplaceAllString(inner, ""), true
	}
	q, err := parseQuantity(inner, defUnit)
	if err != nil || q.Terms[0].Unit == "" {
		return Quantity{}, false
	}
	q.UpTo = q.UpTo || upTo
	return q, true
}

func parseNumber(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, " "); i > 0 { // mixed fraction "1 1/2"
		whole, err := strconv.ParseFloat(s[:i], 64)
		if err != nil {
			return 0, err
		}
		frac, err := parseNumber(s[i+1:])
		return whole + frac, err
	}
	if n, d, ok := strings.Cut(s, "/"); ok {
		a, err1 := strconv.ParseFloat(n, 64)
		b, err2 := strconv.ParseFloat(d, 64)
		if err1 != nil || err2 != nil || b == 0 {
			return 0, errUnparsed
		}
		return a / b, nil
	}
	return strconv.ParseFloat(s, 64)
}

// parseExpr reads "3", "1d4", "1d4+1", "1+1d4", "1 1/2", "1/2".
func parseExpr(s string) (count float64, dice string, err error) {
	for _, piece := range strings.Split(s, "+") {
		if diceRe.MatchString(piece) {
			if dice != "" {
				return 0, "", errUnparsed
			}
			dice = piece
			continue
		}
		n, err := parseNumber(piece)
		if err != nil {
			return 0, "", errUnparsed
		}
		count += n
	}
	return count, dice, nil
}

// parseQuantity reads "60 yds. + 10 yds./level", "1d4 rds. + 1/level",
// "1 rd./2 levels", "10 yds./level, max. 100 yds.", "up to 4 hrs.". defUnit
// is the unit given to a bare number (casting time: segments); "" leaves it
// empty. A term without a unit takes the unit of the previous term.
func parseQuantity(s, defUnit string) (Quantity, error) {
	var q Quantity
	s = strings.TrimSpace(s)
	if upToRe.MatchString(s) {
		q.UpTo = true
		s = upToRe.ReplaceAllString(s, "")
	}
	if m := maxRe.FindStringSubmatch(s); m != nil {
		num, unit := m[1], m[2]
		if num == "" {
			num, unit = m[3], m[4]
		}
		q.Max, _ = strconv.ParseFloat(num, 64)
		q.MaxUnit = normUnit(unit)
		s = maxRe.ReplaceAllString(s, "")
	}
	s = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), ". "))
	s = hyphenUnitRe.ReplaceAllString(s, "$1 $2") // "10-ft." -> "10 ft."
	if s == "" {
		return q, errUnparsed
	}
	for _, raw := range termSplit.Split(s, -1) {
		raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "."))
		m := termRe.FindStringSubmatch(raw)
		if m == nil {
			return q, fmt.Errorf("%w term %q", errUnparsed, raw)
		}
		count, dice, err := parseExpr(m[1])
		if err != nil {
			return q, err
		}
		t := Term{Count: count, Dice: dice, Unit: normUnit(m[3])}
		if m[2] != "" {
			t.Upper, _ = strconv.ParseFloat(m[2], 64)
		}
		if m[5] != "" {
			per := normUnit(m[5])
			if per == "level" {
				t.PerLevel, t.Step = true, 1
				if m[4] != "" {
					t.Step, _ = strconv.Atoi(m[4])
				}
				if t.Step < 1 {
					return q, errUnparsed
				}
			} else {
				t.Per = per
			}
		}
		if t.Unit == "" {
			switch {
			case len(q.Terms) > 0:
				t.Unit = q.Terms[len(q.Terms)-1].Unit
			default:
				t.Unit = defUnit
			}
		}
		q.Terms = append(q.Terms, t)
	}
	if _, err := q.unit(); err != nil {
		return q, err
	}
	return q, nil
}

// unit returns the unit At expresses the quantity in: the shared unit, or
// the smallest one when the terms mix units of the same family.
func (q Quantity) unit() (string, error) {
	best, bestF, family := "", 0.0, ""
	for i, t := range q.Terms {
		f, fam := unitFactor(t.Unit)
		if i == 0 {
			best, bestF, family = t.Unit, f, fam
			continue
		}
		if t.Unit == best {
			continue
		}
		if fam == "" || fam != family {
			return "", fmt.Errorf("%w: mixed units %q and %q", errUnparsed, best, t.Unit)
		}
		if f < bestF {
			best, bestF = t.Unit, f
		}
	}
	// Dice cannot be rescaled into another unit.
	for _, t := range q.Terms {
		if t.Dice != "" && t.Unit != best {
			return "", fmt.Errorf("%w: dice in %q, result in %q", errUnparsed, t.Unit, best)
		}
	}
	return best, nil
}

// scaledDice is the dice of t at casterLevel: "1d6" per level at level 5 is
// "5d6". Dice that do not scale are returned as is.
func (t Term) scaledDice(casterLevel int) string {
	if !t.PerLevel {
		return t.Dice
	}
	m := diceRe.FindStringSubmatch(t.Dice)
	if m == nil {
		return t.Dice
	}
	n, _ := strconv.Atoi(m[1])
	return strconv.Itoa(n*(casterLevel/t.Step)) + "d" + m[2]
}

// At computes the quantity for casterLevel. Max caps the amount only (not
// Upper or the dice); no PHB form in the seed needs more.
func (q Quantity) At(casterLevel int) Value {
	unit, _ := q.unit()
	outF, _ := unitFactor(unit)
	var v Value
	v.Unit = unit
	var dice []string
	for _, t := range q.Terms {
		scale := 1.0
		if f, _ := unitFactor(t.Unit); f != 0 && outF != 0 {
			scale = f / outF
		}
		n := t.Count
		if t.PerLevel {
			n *= float64(casterLevel / t.Step)
		}
		v.Amount += n * scale
		if t.Upper != 0 {
			v.Upper += t.Upper * scale
		}
		if t.Dice != "" {
			dice = append(dice, t.scaledDice(casterLevel))
		}
	}
	if v.Upper != 0 && v.Upper < v.Amount {
		v.Upper = 0
	}
	v.Dice = strings.Join(dice, " + ")
	if q.Max > 0 {
		maxF, _ := unitFactor(q.MaxUnit)
		if maxF == 0 || outF == 0 {
			maxF, outF = 1, 1
		}
		if limit := q.Max * maxF / outF; v.Amount > limit {
			v.Amount = limit
		}
	}
	return v
}
