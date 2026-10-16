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
	// AreaAlternatives is a choice between readings ("1 creature or object",
	// "10-ft. cube or one item"): Alternatives holds each, every one an area
	// or targets reading the text covers whole.
	AreaAlternatives = "alternatives"
	// AreaTogether is a sum of readings ("One 40-ft. cube + one 10-ft.
	// cube/level", "Caster + 1 creature/level"): Parts holds each, every one a
	// whole reading.
	AreaTogether = "together"
)

// AreaOfEffect is "20-ft. radius", "10 x 60 ft.", "1 creature/level", "40-ft.
// cube, 1 creature/level", "Creature touched".
type AreaOfEffect struct {
	Raw  string `json:"raw"`
	Kind string `json:"kind"` // area | targets | self | touch | alternatives | together | special
	// Shape is radius, cube, square, cone, sphere, ... ("" when only extents).
	Shape string `json:"shape,omitempty"`
	// Figure is the solid or figure a radius or diameter measures
	// ("10-ft. radius sphere": Shape radius, Figure sphere); "" when the text
	// names none.
	Figure string `json:"figure,omitempty"`
	// Dims are the extents: one for a radius/cube, several for "10 x 60 ft.".
	Dims []Quantity `json:"dims,omitempty"`
	// Count is the number of such areas when it is more than one ("Two 10-ft.
	// cubes/level": Count 2, one 10-ft./level cube); 0 for one.
	Count int `json:"count,omitempty"`
	// Targets is the creature/object count; Noun is what is counted.
	Targets *Quantity `json:"targets,omitempty"`
	Noun    string    `json:"noun,omitempty"`
	// Members is the size of a group Noun names ("One unit of up to 200
	// individuals": Noun unit, Members 200 up to, MembersNoun individuals).
	Members     *Quantity `json:"members,omitempty"`
	MembersNoun string    `json:"members_noun,omitempty"`
	// Within is set when the extents bound where the targets are ("1 animal
	// within 30 ft."), rather than describe an area of their own.
	Within bool `json:"within,omitempty"`
	// AroundTouched is set when the area is centred on the creature or object
	// touched ("10-ft. radius around creature touched").
	AroundTouched bool `json:"around_touched,omitempty"`
	// Partial is set on an area or targets kind whose text says more than
	// the typed form holds: an alternative ("1 creature or item"), a number
	// it dropped ("One creature/two levels") or a word it does not read
	// ("Line of sight, max. 60 ft."). A reader that words the stat shows the
	// raw text instead, and At computes nothing from it (ADR 0021 step 8).
	Partial bool `json:"partial,omitempty"`
	// Alternatives are the readings of an alternatives kind, in the text's
	// order; each keeps its own part of the text as Raw.
	Alternatives []AreaOfEffect `json:"alternatives,omitempty"`
	// Parts are the readings of a together kind, in the text's order.
	Parts []AreaOfEffect `json:"parts,omitempty"`
	// Special says the book adds that the area is special: "plus" ("5 ft.
	// radius + special"), "or" ("15 sq. ft./level or special") or "note" ("5-ft.
	// radius (special)"). The typed form is the rest of the text.
	Special string `json:"special,omitempty"`
	// Usually is who the text says it usually is ("One individual (usually
	// caster)": caster).
	Usually string `json:"usually,omitempty"`
}

// AreaValue is an AreaOfEffect computed for a caster level.
type AreaValue struct {
	Dims    []Value `json:"dims,omitempty"`
	Targets *Value  `json:"targets,omitempty"`
}

var (
	areaSplit = regexp.MustCompile(`(?i),\s*| in (?:an? )?| within | around |\s+or\s+| with `)
	dimTok    = regexp.MustCompile(`(?i)(\d+d\d+|\d[\d,]*)(?:\s*(?:-|to)\s*(\d+))?(?:\s*-?\s*((?:sq\.?|cu\.?|cubic|square)\s*)?(ft|feet|foot|yds?|yards?|mi|miles?|in)\b\.?)?`)
	levelXRe  = regexp.MustCompile(`(?i)(?:\(|caster'?s )level\s*x\s*(\d+)\)?[\s-]*(sq\.?|cu\.?)?\s*(ft|yds?)?`)
	shapeRe   = regexp.MustCompile(`(?i)(?:\b(radius|diameter|cone|cube|square|sphere|globe|cloud|column|spray|hemisphere|volume|wall|circle|path|line of sight|line)\b|(\bsq\b))`)
	targetRe  = regexp.MustCompile(`(?i)^(?:up to\s+|one\s+|two\s+)?(\d+(?:d\d+)?)(?:\s*(?:-|to)\s*(\d+)|(\+))?\s+(.*?)\s*$`)
	// orMoreRe is "1 or more", "two or more": an open-ended count, read as
	// "1+" so that areaSplit does not take the "or" for a choice.
	orMoreRe = regexp.MustCompile(`(?i)\b(\d+(?:d\d+)?|one|two)\s+or more\b`)
	// perWordRe is a non-level divisor after an extent: "square/priest".
	perWordRe = regexp.MustCompile(`(?i)^[a-z.\s-]*?/\s*([a-z]+)\b`)
	// perLevelWordRe is "per level" spelled out: "1 creature per level".
	perLevelWordRe = regexp.MustCompile(`(?i)\s+per\s+(?:(\d+|two|three|four|five|six|seven|eight|nine|ten)\s+)?levels?\b`)
	// diameterOfRe is "sphere with diameter of 1 ft./level", read as "1
	// ft./level diameter sphere".
	diameterOfRe = regexp.MustCompile(`(?i)\b(sphere|globe|hemisphere|circle|cylinder)\s+with\s+(?:a\s+)?diameter\s+of\s+(.+)$`)
	// aroundTouchedRe is an area centred on what is touched.
	aroundTouchedRe = regexp.MustCompile(`(?i)^(.+?)\s+around\s+(?:a\s+|the\s+)?(?:creature|person|object)s?\s+touched$`)
	// membersRe is the size of a group: "unit of up to 200 individuals".
	membersRe = regexp.MustCompile(`(?i)^(.+?)\s+(?:of\s+)?up to\s+(\d+)\s+([a-z]+)$`)
	// axisRe is the word that says which way an extent runs.
	axisRe    = regexp.MustCompile(`(?i)\b(wide|long|high|tall|thick|deep)\b`)
	createsRe = regexp.MustCompile(`(?i)^creates?\s+`)
	bareCount = regexp.MustCompile(`^\d+(?:d\d+)?(?:\+\d+(?:d\d+)?)*$`)
	lengthRe  = regexp.MustCompile(`(?i)\b(ft|feet|foot|yds?|yards?|mi|miles?|sq|cu)\b|-ft\b`)
	// thousandsRe is a thousands separator ("1,000 square feet"), which
	// areaSplit would otherwise take for a list comma.
	thousandsRe = regexp.MustCompile(`(\d),(\d{3})\b`)
	// perLevelRe is a per-level divisor after an extent: "/level", "/lvl.",
	// "/2 levels", "/two levels".
	perLevelRe = regexp.MustCompile(`(?i)^[^+]*?/\s*(?:(\d+|one|two|three|four|five|six|seven|eight|nine|ten)\s*)?(?:levels?|lvls?)\b`)
	// orRe separates alternatives; ", or" closes a list whose commas
	// separate alternatives too ("One doorway, portal, or small object").
	orRe      = regexp.MustCompile(`(?i)(,?)\s+or\s+`)
	bareNoun  = regexp.MustCompile(`(?i)^[a-z][a-z -]*$`)
	bareNum   = regexp.MustCompile(`^\d[\d,]*$`)
	leadNumRe = regexp.MustCompile(`^\d[\d,]*`)
	// specialRes are the ways the book adds "special" to an area.
	specialRes = []struct {
		re   *regexp.Regexp
		kind string
	}{
		{regexp.MustCompile(`(?i)\s*\+\s*special$`), "plus"},
		{regexp.MustCompile(`(?i)\s+or\s+special$`), "or"},
		{regexp.MustCompile(`(?i)\s*\(special\)$`), "note"},
	}
	usuallyRe = regexp.MustCompile(`(?i)\s*\(usually\s+(caster)\)$`)
	// plusRe separates the readings of a sum.
	plusRe = regexp.MustCompile(`\s*\+\s*`)
	// glueRe is a count glued to its noun ("1creature").
	glueRe = regexp.MustCompile(`^(\d+)([a-z]{3,})`)
	// manyAreasRe is "Two 10-ft. cubes/level": a number word, then an extent
	// and a plural shape.
	manyAreasRe = regexp.MustCompile(`(?i)^(two|three|four|five|six|seven|eight|nine|ten)\s+(\d.*?\b(?:cube|square|sphere|cone|circle|globe|hemisphere|wall|cloud|column))s\b(.*)$`)
	// maxLeadRe is a leading "max." that bounds the extent after it.
	maxLeadRe = regexp.MustCompile(`(?i)^max(?:imum|\.)?\s*`)
	// siteRe is where on a figure an extent is measured ("wide at base").
	siteRe = regexp.MustCompile(`(?i)\bat (?:the )?(base|end)\b`)
)

var wordCounts = map[string]string{"one": "1", "two": "2"}

// ParseArea reads a PHB area of effect. Descriptive text that holds neither
// an extent nor a count ("Line of sight") is special. A text the single
// reading holds only in part is read as alternatives when it is a choice
// ("1 creature or object") whose every reading is whole.
func ParseArea(raw string) AreaOfEffect {
	if a, ok := parseAnnotated(raw); ok {
		return a
	}
	a := parseAreaReading(raw)
	if !a.Partial && (a.Kind != AreaSelf || !strings.Contains(raw, "+")) {
		return a
	}
	if parts, ok := parseParts(raw); ok {
		return AreaOfEffect{Raw: raw, Kind: AreaTogether, Parts: parts}
	}
	if alts, ok := parseAlternatives(raw); ok {
		return AreaOfEffect{Raw: raw, Kind: AreaAlternatives, Alternatives: alts}
	}
	return a
}

// parseAnnotated reads an area that ends in a note the typed form keeps
// beside it: "+ special", "or special", "(special)", "(usually caster)". ok is
// false unless the text before the note is a whole area or targets reading.
func parseAnnotated(raw string) (AreaOfEffect, bool) {
	s := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(raw), ". "))
	var special, usually string
	rest := ""
	for _, sp := range specialRes {
		if loc := sp.re.FindStringIndex(s); loc != nil {
			special, rest = sp.kind, s[:loc[0]]
		}
	}
	if m := usuallyRe.FindStringSubmatchIndex(s); m != nil {
		usually, rest = strings.ToLower(s[m[2]:m[3]]), s[:m[0]]
	}
	if rest == "" {
		return AreaOfEffect{}, false
	}
	a := parseAreaReading(rest)
	if (a.Kind != AreaArea && a.Kind != AreaTargets) || a.Partial {
		return AreaOfEffect{}, false
	}
	a.Raw, a.Special, a.Usually = raw, special, usually
	return a, true
}

// parseParts reads "A + B" as one reading per part. ok is false unless there
// are several parts and every one is an area, targets or self reading the
// text covers whole (ADR 0021 step 8, areas 15).
func parseParts(raw string) ([]AreaOfEffect, bool) {
	s := normText(strings.TrimSpace(strings.TrimRight(strings.TrimSpace(raw), ". ")))
	texts := plusRe.Split(s, -1)
	if len(texts) < 2 {
		return nil, false
	}
	parts := make([]AreaOfEffect, 0, len(texts))
	for _, p := range texts {
		p = glueRe.ReplaceAllString(strings.TrimSpace(p), "$1 $2")
		a := parseAreaReading(p)
		if (a.Kind != AreaArea && a.Kind != AreaTargets && a.Kind != AreaSelf) || a.Partial {
			return nil, false
		}
		// A part with an extent and a count ("1 stick/level in a 10-ft. cube")
		// may bound the other parts too: not read.
		if len(a.Dims) > 0 && a.Targets != nil {
			return nil, false
		}
		parts = append(parts, a)
	}
	return parts, true
}

// parseAlternatives reads "A or B" and "A, B, or C" as one reading per
// alternative. A bare noun counts as many as the alternative before it ("1
// creature or object": one object), unless that one's noun is qualified
// ("1 dead human or demihuman" is not read), and a bare number takes the unit and
// shape of the alternative after it ("20 or 40 ft. radius": a 20-ft.
// radius). ok is false unless there are several alternatives and every one
// is an area or targets reading the text covers whole.
func parseAlternatives(raw string) ([]AreaOfEffect, bool) {
	s := normText(strings.TrimSpace(strings.TrimRight(strings.TrimSpace(raw), ". ")))
	var parts []string
	list := false
	start := 0
	for _, m := range orRe.FindAllStringSubmatchIndex(s, -1) {
		if strings.HasPrefix(strings.ToLower(s[m[1]:]), "more") { // "1 or more creatures": not a choice
			continue
		}
		parts = append(parts, s[start:m[0]])
		list = list || m[3] > m[2]
		start = m[1]
	}
	if len(parts) == 0 {
		return nil, false
	}
	parts = append(parts, s[start:])
	if list {
		var split []string
		for _, p := range parts {
			split = append(split, strings.Split(p, ",")...)
		}
		parts = split
	}
	alts := make([]AreaOfEffect, 0, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if bareNum.MatchString(p) && i+1 < len(parts) {
			next := strings.TrimSpace(parts[i+1])
			if rest := leadNumRe.ReplaceAllString(next, ""); rest != next {
				p += rest
			}
		}
		if bareNoun.MatchString(p) && len(alts) > 0 && wordCounts[strings.ToLower(strings.Fields(p)[0])] == "" {
			prev := alts[len(alts)-1]
			// After a qualified noun ("1 dead human or demihuman") the
			// qualifier may be the bare noun's too: not read.
			if prev.Kind != AreaTargets || strings.Contains(prev.Noun, " ") {
				return nil, false
			}
			alts = append(alts, AreaOfEffect{Raw: p, Kind: AreaTargets, Targets: prev.Targets, Noun: p})
			continue
		}
		a := parseAreaReading(p)
		if (a.Kind != AreaArea && a.Kind != AreaTargets) || a.Partial {
			return nil, false
		}
		alts = append(alts, a)
	}
	return alts, true
}

// parseAreaReading reads raw as one area or targets reading (or a self,
// touch or special kind).
func parseAreaReading(raw string) AreaOfEffect {
	a := AreaOfEffect{Raw: raw, Kind: KindSpecial}
	s := normText(strings.TrimSpace(strings.TrimRight(strings.TrimSpace(raw), ". ")))
	s = thousandsRe.ReplaceAllString(s, "$1$2")
	created := createsRe.MatchString(s)
	s = createsRe.ReplaceAllString(s, "")
	count := 0
	if m := manyAreasRe.FindStringSubmatch(s); m != nil { // "Two 10-ft. cubes/level"
		count, _ = strconv.Atoi(numWords[strings.ToLower(m[1])])
		s = m[2] + m[3]
	}
	s = orMoreRe.ReplaceAllStringFunc(s, func(m string) string {
		n := strings.ToLower(orMoreRe.FindStringSubmatch(m)[1])
		if d := wordCounts[n]; d != "" {
			n = d
		}
		return n + "+"
	})
	s = perLevelWordRe.ReplaceAllStringFunc(s, func(m string) string {
		if n := strings.ToLower(perLevelWordRe.FindStringSubmatch(m)[1]); n != "" {
			return "/" + n + " levels"
		}
		return "/level"
	})
	s = diameterOfRe.ReplaceAllString(s, "$2 diameter $1")
	if m := aroundTouchedRe.FindStringSubmatch(s); m != nil {
		s, a.AroundTouched = m[1], true
	}
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
		upTo := upToRe.MatchString(part) // "Up to 20-ft. cube", "Up to 6 creatures"
		part = upToRe.ReplaceAllString(part, "")
		// A cap: "5 yds./level (90 yds. maximum)", or its own part after a
		// comma ("30 ft. radius, max. 60 ft."), which caps the extent before it.
		var capM []string
		if m := maxRe.FindStringSubmatch(part); m != nil && lengthRe.MatchString(part) {
			if rest := strings.TrimSpace(maxRe.ReplaceAllString(part, "")); rest != "" {
				part, capM = rest, m
			} else if len(a.Dims) > 0 && prevSep == "," {
				setCap(&a.Dims[len(a.Dims)-1], m)
				continue
			}
		}
		// "Line of sight, max. 60 ft.": a leading "max." bounds the extent.
		if len(a.Dims) == 0 && maxLeadRe.MatchString(part) && lengthRe.MatchString(part) {
			part, upTo = maxLeadRe.ReplaceAllString(part, ""), true
		}
		if part == "" {
			continue
		}
		if lengthRe.MatchString(part) {
			first := len(a.Dims)
			switch {
			case a.Dims == nil:
				a.Dims = parseDims(part)
				a.Within = prevSep == "within" && a.Targets != nil
				if a.Shape == "" {
					a.Shape = shapeOf(part)
				}
			case prevSep == ",": // "10 ft.-wide path, 60 feet long": more extents
				a.Dims = append(a.Dims, parseDims(part)...)
			}
			if upTo && first < len(a.Dims) {
				a.Dims[first].UpTo = true
			}
			if capM != nil && len(a.Dims)-first == 1 {
				setCap(&a.Dims[first], capM)
			}
		} else if a.Targets == nil {
			if q, noun, ok := parseTargets(part); ok {
				q.UpTo = upTo
				a.Targets, a.Noun = &q, noun
				if m := membersRe.FindStringSubmatch(noun); m != nil {
					n, _ := strconv.ParseFloat(m[2], 64)
					a.Noun, a.MembersNoun = m[1], m[3]
					a.Members = &Quantity{Terms: []Term{{Count: n}}, UpTo: true}
				}
			}
		}
	}
	switch {
	case len(a.Dims) > 0:
		a.Kind = AreaArea
	case a.Targets != nil:
		a.Kind = AreaTargets
	}
	if a.Kind == AreaArea && a.Shape == "" { // "wall, 10 x 5 ft./level": the shape is a part of its own
		a.Shape = shapeOf(s)
	}
	if a.Kind == AreaArea && (a.Shape == "radius" || a.Shape == "diameter") {
		a.Figure = figureOf(s, a.Shape)
	}
	if a.Kind != KindSpecial {
		a.Partial = !a.covers(s)
	}
	// "One creature + 1creature/2 levels": a sum swallowed into the noun.
	if a.Targets != nil && strings.Contains(a.Noun, "+") {
		a.Partial = true
	}
	if a.Kind != AreaArea && a.Kind != AreaTargets {
		a.AroundTouched = false
	}
	if a.Kind == AreaArea && len(a.Dims) == 1 {
		a.Count = count
	}
	if a.Dims == nil || a.Targets == nil {
		a.Within = false
	}
	return a
}

// setCap sets the cap of q from a maxRe match.
func setCap(q *Quantity, m []string) {
	num, unit := m[1], m[2]
	if num == "" {
		num, unit = m[3], m[4]
	}
	q.Max, _ = strconv.ParseFloat(num, 64)
	q.MaxUnit = normUnit(unit)
}

// figureOf is the first shape word of s other than shape and the square
// ("sq.", "square" are also units): what a radius or diameter measures.
func figureOf(s, shape string) string {
	for _, m := range shapeRe.FindAllStringSubmatch(s, -1) {
		if w := strings.ToLower(m[1]); w != "" && w != shape && w != "square" {
			return w
		}
	}
	return ""
}

var (
	coverTokRe   = regexp.MustCompile(`\d+d\d+|\d[\d,]*(?:\.\d+)?|[a-z]+`)
	numRangeRe   = regexp.MustCompile(`(\d)\s+to\s+(\d)`)
	leadCountRe  = regexp.MustCompile(`\b(one|two)\b`)
	articleOneRe = regexp.MustCompile(`^one\s+(\d)`)
	stepWordRe   = regexp.MustCompile(`/\s*(one|two|three|four|five|six|seven|eight|nine|ten)\s+level`)
	// coverWords are the words any area's typed form accounts for: the
	// length units, the "sq"/"cu" prefixes (and "square", a unit as much as
	// a shape) and the connectors. Another shape word is covered only by
	// the reading's Shape or Figure.
	coverWords = func() map[string]bool {
		out := map[string]bool{}
		for w, u := range unitAliases {
			if _, fam := unitFactor(u); fam == "length" {
				out[w] = true
			}
		}
		for _, w := range []string{
			"sq", "square", "cu", "cubic", "x", "in", "a", "an", "level", "levels", "lvl", "lvls", "area",
		} {
			out[w] = true
		}
		return out
	}()
)

// covers reports whether the typed form accounts for all of s (the
// normalized text): every number and die in s is one of the form's, and
// every word is a length unit, a shape, a word of the noun or a connector
// ("x", "in a", "level"). "One"/"two" count as numbers (and so do the number
// words of a level step), "1 to 4" as a range, and "up to" is read when a
// quantity holds it.
func (a AreaOfEffect) covers(s string) bool {
	nums, dice := map[float64]bool{}, map[string]bool{}
	pers := map[string]bool{} // non-level divisors, as normalized
	words := map[string]bool{a.Shape: a.Shape != "", a.Figure: a.Figure != ""}
	upTo := false
	addQ := func(q Quantity) {
		upTo = upTo || q.UpTo
		for _, t := range q.Terms {
			nums[t.Count], nums[t.Upper] = true, true
			if t.Per != "" {
				pers[t.Per] = true
			}
			if t.Step > 1 {
				nums[float64(t.Step)] = true
			}
			if t.Dice != "" {
				dice[t.Dice] = true
			}
		}
		if q.Max != 0 {
			nums[q.Max] = true
			words["max"], words["maximum"] = true, true
		}
	}
	for _, w := range strings.Fields(a.Shape) { // "line of sight"
		words[w] = true
	}
	for _, d := range a.Dims {
		addQ(d)
		for _, w := range axisWords[d.Axis] {
			words[w] = true
		}
		if d.Site != "" { // "wide at base"
			words["at"], words["the"], words[d.Site] = true, true, true
		}
		if len(a.Dims) > 1 {
			words["and"] = true
		}
		if d.UpTo {
			words["max"], words["maximum"] = true, true
		}
	}
	if a.Within {
		words["within"] = true
	}
	if a.Members != nil { // "unit of up to 200 individuals"
		addQ(*a.Members)
		for _, w := range []string{"of", "up", "to", a.MembersNoun} {
			words[strings.ToLower(w)] = true
		}
	}
	if a.Targets != nil {
		addQ(*a.Targets)
		for _, w := range coverTokRe.FindAllString(strings.ToLower(a.Noun), -1) {
			words[w] = true
		}
	}
	low := strings.ToLower(thousandsRe.ReplaceAllString(s, "$1$2"))
	if upTo {
		low = upToRe.ReplaceAllString(low, "")
	}
	low = articleOneRe.ReplaceAllString(low, "$1") // "One 10-ft. cube": one area, not a count
	low = leadCountRe.ReplaceAllStringFunc(low, func(w string) string { return wordCounts[w] })
	low = stepWordRe.ReplaceAllStringFunc(low, func(w string) string {
		n := stepWordRe.FindStringSubmatch(w)[1]
		return "/" + numWords[n] + " level"
	})
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
		case !coverWords[tok] && !words[tok] && !pers[normUnit(tok)]:
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
	chunks := dimSplit.Split(part, -1)
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

// dimSplit separates the extents of one part: "10 x 60 ft.", "60 yards long
// and 30 yards wide".
var dimSplit = regexp.MustCompile(`\s+x\s+|\s+and\s+`)

// axisWords are the words that name each axis of an extent.
var axisWords = map[string][]string{
	"wide": {"wide"}, "long": {"long"}, "high": {"high", "tall"}, "thick": {"thick"}, "deep": {"deep"},
}

// axisOf is the axis a chunk of text names ("2 ft. wide"), "" when none or
// when it names several.
func axisOf(ch string) string {
	ms := axisRe.FindAllString(ch, -1)
	if len(ms) != 1 {
		return ""
	}
	w := strings.ToLower(ms[0])
	if w == "tall" {
		return "high"
	}
	return w
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
		t := Term{Unit: areaUnit(m[3], m[4])}
		if diceRe.MatchString(m[1]) {
			t.Dice = m[1]
		} else {
			t.Count, _ = strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
		}
		if m[2] != "" {
			t.Upper, _ = strconv.ParseFloat(m[2], 64)
		}
		if t.PerLevel, t.Step = perLevelStep(ch[loc[1]:]); !t.PerLevel {
			t.Step = 0
			if pm := perWordRe.FindStringSubmatch(ch[loc[1]:]); pm != nil {
				t.Per = normUnit(pm[1])
			}
		}
		q.Terms = append(q.Terms, t)
	}
	if len(q.Terms) > 0 {
		q.Axis = axisOf(ch)
		if m := siteRe.FindStringSubmatch(ch); m != nil {
			q.Site = strings.ToLower(m[1])
		}
	}
	return q
}

// perLevelStep reads the per-level divisor that follows an extent, up to
// the next "+" term: "/level" and "/lvl." are every level, "/2 levels" and
// "/two levels" every second.
func perLevelStep(tail string) (bool, int) {
	m := perLevelRe.FindStringSubmatch(tail)
	if m == nil {
		return false, 0
	}
	n := strings.ToLower(m[1])
	if w, ok := numWords[n]; ok {
		n = w
	}
	if step, err := strconv.Atoi(n); err == nil && step > 0 {
		return true, step
	}
	return true, 1
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
	noun := m[4]
	if i := strings.Index(noun, "/"); i >= 0 { // "creature/level", "person/3 levels", "target/meteor"
		rest := strings.TrimSpace(noun[i+1:])
		noun = strings.TrimSpace(noun[:i])
		if ok, step := perLevelStep("/" + rest); ok {
			t.PerLevel, t.Step = true, step
		} else if bareNoun.MatchString(strings.TrimRight(rest, ".")) && !strings.Contains(rest, " ") {
			t.Per = normUnit(rest)
		}
	}
	if t.PerLevel && t.Dice != "" && strings.Contains(noun, "+") {
		return Quantity{}, "", false // "1d4 sticks + 1 stick/level": the level term is not the dice's
	}
	return Quantity{Terms: []Term{t}, OrMore: m[3] != ""}, noun, true
}

// At computes the area for casterLevel. A partial reading computes nothing:
// a value from what the parser kept would be wrong ("One creature/two
// levels" is not one creature per level), so there is no value to show or
// roll (ADR 0021 step 8).
//
// Alternatives compute nothing either: the value would be one reading's, and
// the book does not say which one the caster picks.
func (a AreaOfEffect) At(casterLevel int) AreaValue {
	var v AreaValue
	if a.Partial || a.Kind == AreaAlternatives {
		return v
	}
	for _, d := range a.Dims {
		v.Dims = append(v.Dims, d.At(casterLevel))
	}
	if a.Targets != nil && !a.Targets.OrMore { // "1 or more": no count to compute
		t := a.Targets.At(casterLevel)
		v.Targets = &t
	}
	return v
}
