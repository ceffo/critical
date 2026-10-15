package spellstats

import (
	"regexp"
	"strconv"
	"strings"
)

// Area kinds.
const (
	AreaArea    = "area"    // Dims holds the extents
	AreaTargets = "targets" // only a creature/object count
	AreaSelf    = "self"    // "The caster"
	AreaTouch   = "touch"   // "Creature touched"
)

// AreaOfEffect is "20-ft. radius", "10 x 60 ft.", "1 creature/level", "40-ft.
// cube, 1 creature/level", "Creature touched".
type AreaOfEffect struct {
	Raw  string `json:"raw"`
	Kind string `json:"kind"` // area | targets | self | touch | special
	// Shape is radius, cube, square, cone, sphere, ... ("" when only extents).
	Shape string `json:"shape,omitempty"`
	// Dims are the extents: one for a radius/cube, several for "10 x 60 ft.".
	Dims []Quantity `json:"dims,omitempty"`
	// Targets is the creature/object count; Noun is what is counted.
	Targets *Quantity `json:"targets,omitempty"`
	Noun    string    `json:"noun,omitempty"`
	// Partial is set on an area or targets kind whose text says more than
	// the typed form holds: an alternative ("1 creature or item"), a number
	// it dropped ("One creature/two levels") or a word it does not read
	// ("Line of sight, max. 60 ft."). The typed form still computes what it
	// read; a reader that words the stat shows the raw text instead.
	Partial bool `json:"partial,omitempty"`
}

// AreaValue is an AreaOfEffect computed for a caster level.
type AreaValue struct {
	Dims    []Value `json:"dims,omitempty"`
	Targets *Value  `json:"targets,omitempty"`
}

var (
	areaSplit = regexp.MustCompile(`(?i),\s*| in (?:an? )?| within | around |\s+or\s+| with `)
	dimTok    = regexp.MustCompile(`(?i)(\d[\d,]*)(?:\s*(?:-|to)\s*(\d+))?(?:\s*-?\s*((?:sq\.?|cu\.?|cubic|square)\s*)?(ft|feet|foot|yds?|yards?|mi|miles?|in)\b\.?)?`)
	levelXRe  = regexp.MustCompile(`(?i)(?:\(|caster'?s )level\s*x\s*(\d+)\)?[\s-]*(sq\.?|cu\.?)?\s*(ft|yds?)?`)
	shapeRe   = regexp.MustCompile(`(?i)(?:\b(radius|diameter|cone|cube|square|sphere|globe|cloud|column|spray|hemisphere|volume|wall)\b|(\bsq\b))`)
	targetRe  = regexp.MustCompile(`(?i)^(?:up to\s+|one\s+|two\s+)?(\d+(?:d\d+)?)(?:\s*(?:-|to)\s*(\d+))?(?:\s+or more)?\s+(.*?)\s*$`)
	createsRe = regexp.MustCompile(`(?i)^creates?\s+`)
	bareCount = regexp.MustCompile(`^\d+(?:d\d+)?(?:\+\d+(?:d\d+)?)*$`)
	lengthRe  = regexp.MustCompile(`(?i)\b(ft|feet|foot|yds?|yards?|mi|miles?|sq|cu)\b|-ft\b`)
)

var wordCounts = map[string]string{"one": "1", "two": "2"}

// ParseArea reads a PHB area of effect. Descriptive text that holds neither
// an extent nor a count ("Line of sight") is special.
func ParseArea(raw string) AreaOfEffect {
	a := AreaOfEffect{Raw: raw, Kind: KindSpecial}
	s := normText(strings.TrimSpace(strings.TrimRight(strings.TrimSpace(raw), ". ")))
	created := createsRe.MatchString(s)
	s = createsRe.ReplaceAllString(s, "")
	low := strings.ToLower(s)
	switch {
	case low == "special" || low == "":
		return a
	case strings.HasPrefix(low, "the caster") || strings.HasPrefix(low, "caster"):
		a.Kind = AreaSelf
		return a
	case strings.Contains(low, "touched"):
		a.Kind = AreaTouch
		return a
	}
	if created && bareCount.MatchString(s) { // "Creates 1d4+1": a count with no noun
		if count, dice, err := parseExpr(s); err == nil {
			a.Kind, a.Targets = AreaTargets, &Quantity{Terms: []Term{{Count: count, Dice: dice}}}
			return a
		}
	}
	if m := levelXRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.ParseFloat(m[1], 64)
		unit := normUnit(m[3])
		if unit == "" {
			unit = "foot"
		}
		a.Kind = AreaArea
		a.Dims = []Quantity{{Terms: []Term{{Count: n, Unit: unit, PerLevel: true, Step: 1}}}}
		a.Shape = shapeOf(s)
		return a
	}
	cuts := areaSplit.FindAllStringIndex(s, -1)
	start := 0
	for i := 0; i <= len(cuts); i++ {
		end, next := len(s), len(s)
		if i < len(cuts) {
			end, next = cuts[i][0], cuts[i][1]
		}
		part := strings.TrimSpace(s[start:end])
		prevSep := ""
		if i > 0 {
			prevSep = strings.TrimSpace(s[cuts[i-1][0]:cuts[i-1][1]])
		}
		start = next
		if part == "" {
			continue
		}
		if lengthRe.MatchString(part) {
			switch {
			case a.Dims == nil:
				a.Dims = parseDims(part)
				if a.Shape == "" {
					a.Shape = shapeOf(part)
				}
			case prevSep == ",": // "10 ft.-wide path, 60 feet long": more extents
				a.Dims = append(a.Dims, parseDims(part)...)
			}
		} else if a.Targets == nil {
			if q, noun, ok := parseTargets(part); ok {
				a.Targets, a.Noun = &q, noun
			}
		}
	}
	switch {
	case len(a.Dims) > 0:
		a.Kind = AreaArea
	case a.Targets != nil:
		a.Kind = AreaTargets
	}
	if a.Kind != KindSpecial {
		a.Partial = !a.covers(s)
	}
	return a
}

var (
	coverTokRe  = regexp.MustCompile(`\d+d\d+|\d[\d,]*(?:\.\d+)?|[a-z]+`)
	numRangeRe  = regexp.MustCompile(`(\d)\s+to\s+(\d)`)
	leadCountRe = regexp.MustCompile(`\b(one|two)\b`)
	// coverWords are the words an area's typed form accounts for: the
	// length units, the "sq"/"cu" prefixes, the shapes and the connectors.
	coverWords = func() map[string]bool {
		out := map[string]bool{}
		for w, u := range unitAliases {
			if _, fam := unitFactor(u); fam == "length" {
				out[w] = true
			}
		}
		for _, w := range []string{
			"sq", "square", "cu", "cubic", "radius", "diameter", "cone", "cube", "sphere",
			"globe", "cloud", "column", "spray", "hemisphere", "volume", "wall", "x", "in", "a", "an", "level",
			"levels", "area",
		} {
			out[w] = true
		}
		return out
	}()
)

// covers reports whether the typed form accounts for all of s (the
// normalized text): every number and die in s is one of the form's, and
// every word is a length unit, a shape, a word of the noun or a connector
// ("x", "in a", "level"). "One"/"two" count as numbers, "1 to 4" as a range.
func (a AreaOfEffect) covers(s string) bool {
	nums, dice := map[float64]bool{}, map[string]bool{}
	addQ := func(q Quantity) {
		for _, t := range q.Terms {
			nums[t.Count], nums[t.Upper] = true, true
			if t.Step > 1 {
				nums[float64(t.Step)] = true
			}
			if t.Dice != "" {
				dice[t.Dice] = true
			}
		}
		if q.Max != 0 {
			nums[q.Max] = true
		}
	}
	for _, d := range a.Dims {
		addQ(d)
	}
	words := map[string]bool{}
	if a.Targets != nil {
		addQ(*a.Targets)
		for _, w := range coverTokRe.FindAllString(strings.ToLower(a.Noun), -1) {
			words[w] = true
		}
	}
	low := strings.ToLower(s)
	low = leadCountRe.ReplaceAllStringFunc(low, func(w string) string { return wordCounts[w] })
	low = numRangeRe.ReplaceAllString(low, "$1-$2")
	for _, tok := range coverTokRe.FindAllString(low, -1) {
		switch {
		case diceRe.MatchString(tok):
			if !dice[tok] {
				return false
			}
		case tok[0] >= '0' && tok[0] <= '9':
			n, err := strconv.ParseFloat(strings.ReplaceAll(tok, ",", ""), 64)
			if err != nil || !nums[n] {
				return false
			}
		case !coverWords[tok] && !words[tok]:
			return false
		}
	}
	return true
}

func shapeOf(s string) string {
	if m := shapeRe.FindStringSubmatch(s); m != nil {
		if m[2] != "" {
			return "square"
		}
		return strings.ToLower(m[1])
	}
	return ""
}

// parseDims reads one part holding extents: "20-ft. radius", "10 x 60 ft.",
// "10 ft. + 5 ft./level". Chunks split on " x "; a chunk without a unit
// takes the unit of the next chunk that has one.
func parseDims(part string) []Quantity {
	chunks := regexp.MustCompile(`\s+x\s+`).Split(part, -1)
	var dims []Quantity
	for _, ch := range chunks {
		q := parseDimChunk(ch)
		if len(q.Terms) == 0 {
			continue
		}
		dims = append(dims, q)
	}
	for i := len(dims) - 2; i >= 0; i-- {
		for j := range dims[i].Terms {
			if dims[i].Terms[j].Unit == "" {
				dims[i].Terms[j].Unit = dims[i+1].Terms[len(dims[i+1].Terms)-1].Unit
			}
		}
	}
	return dims
}

func parseDimChunk(ch string) Quantity {
	locs := dimTok.FindAllStringSubmatchIndex(ch, -1)
	var q Quantity
	for i, loc := range locs {
		m := dimTok.FindStringSubmatch(ch[loc[0]:loc[1]])
		// Only tokens joined by "+" sum; anything else is an alternative.
		if i > 0 {
			between := ch[locs[i-1][1]:loc[0]]
			if !strings.Contains(between, "+") {
				break
			}
		}
		count, _ := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
		t := Term{Count: count, Unit: areaUnit(m[3], m[4])}
		if m[2] != "" {
			t.Upper, _ = strconv.ParseFloat(m[2], 64)
		}
		end := len(ch)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		tail := strings.ToLower(ch[loc[1]:end])
		if strings.Contains(tail, "/level") || strings.Contains(tail, "/ level") {
			t.PerLevel, t.Step = true, 1
		}
		q.Terms = append(q.Terms, t)
	}
	return q
}

func areaUnit(prefix, unit string) string {
	u := normUnit(unit)
	p := strings.ToLower(strings.TrimRight(strings.TrimSpace(prefix), ". "))
	switch {
	case strings.HasPrefix(p, "sq"), p == "square":
		return "sq " + u
	case strings.HasPrefix(p, "cu"), p == "cubic":
		return "cu " + u
	}
	return u
}

func parseTargets(part string) (Quantity, string, bool) {
	lower := strings.ToLower(part)
	for w, d := range wordCounts {
		if strings.HasPrefix(lower, w+" ") {
			part = d + " " + part[len(w)+1:]
			break
		}
	}
	m := targetRe.FindStringSubmatch(part)
	if m == nil {
		return Quantity{}, "", false
	}
	count, dice, err := parseExpr(m[1])
	if err != nil {
		return Quantity{}, "", false
	}
	t := Term{Count: count, Dice: dice}
	if m[2] != "" {
		t.Upper, _ = strconv.ParseFloat(m[2], 64)
	}
	noun := m[3]
	if i := strings.Index(noun, "/"); i >= 0 { // "creature/level", "person/3 levels"
		rest := strings.TrimSpace(noun[i+1:])
		noun = strings.TrimSpace(noun[:i])
		if strings.Contains(rest, "level") {
			t.PerLevel, t.Step = true, 1
			if n, err := strconv.Atoi(strings.Fields(rest)[0]); err == nil {
				t.Step = n
			}
		}
	}
	if t.PerLevel && t.Dice != "" && strings.Contains(noun, "+") {
		return Quantity{}, "", false // "1d4 sticks + 1 stick/level": the level term is not the dice's
	}
	return Quantity{Terms: []Term{t}}, noun, true
}

// At computes the area for casterLevel.
func (a AreaOfEffect) At(casterLevel int) AreaValue {
	var v AreaValue
	for _, d := range a.Dims {
		v.Dims = append(v.Dims, d.At(casterLevel))
	}
	if a.Targets != nil {
		t := a.Targets.At(casterLevel)
		v.Targets = &t
	}
	return v
}
