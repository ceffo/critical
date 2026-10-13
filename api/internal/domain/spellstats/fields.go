package spellstats

import (
	"regexp"
	"strings"
)

// Kind values shared by the timed fields.
const (
	KindSpecial = "special"
	KindTimed   = "timed"
)

// CastingTime kinds.
// (KindTimed: Time holds the quantity, bare numbers are segments.)

// CastingTime is "3" (segments), "1 rd.", "1 turn", "2d12 hours".
type CastingTime struct {
	Raw  string   `json:"raw"`
	Kind string   `json:"kind"` // timed | special
	Time Quantity `json:"time"`
}

// ParseCastingTime reads a PHB casting time. Bare numbers are segments.
func ParseCastingTime(raw string) CastingTime {
	c := CastingTime{Raw: raw, Kind: KindSpecial}
	if q, err := parseQuantity(raw, "segment"); err == nil {
		c.Kind, c.Time = KindTimed, q
	}
	return c
}

// At computes the casting time (zero Value for special).
func (c CastingTime) At(casterLevel int) Value {
	if c.Kind != KindTimed {
		return Value{}
	}
	return c.Time.At(casterLevel)
}

// Range kinds.
const (
	RangeSelf      = "self"
	RangeTouch     = "touch"
	RangeDistance  = "distance"
	RangeUnlimited = "unlimited"
)

// Range is "Touch", "0" (self), "60 yds. + 10 yds./level", "Unlimited".
type Range struct {
	Raw      string   `json:"raw"`
	Kind     string   `json:"kind"` // self | touch | distance | unlimited | special
	Distance Quantity `json:"distance"`
	// Radius is set for "10-ft. radius" style ranges.
	Radius bool `json:"radius,omitempty"`
}

var radiusRe = regexp.MustCompile(`(?i)[\s-]*radius$`)

// ParseRange reads a PHB range. "0" is self.
func ParseRange(raw string) Range {
	r := Range{Raw: raw, Kind: KindSpecial}
	s := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(raw), ". "))
	switch strings.ToLower(s) {
	case "0", "self", "the caster":
		r.Kind = RangeSelf
		return r
	case "touch":
		r.Kind = RangeTouch
		return r
	case "unlimited":
		r.Kind = RangeUnlimited
		return r
	}
	if radiusRe.MatchString(s) {
		r.Radius = true
		s = radiusRe.ReplaceAllString(s, "")
	}
	if q, err := parseQuantity(s, ""); err == nil && q.Terms[0].Unit != "" {
		r.Kind, r.Distance = RangeDistance, q
	} else {
		r.Radius = false
	}
	return r
}

// At computes the distance (zero Value unless Kind is distance).
func (r Range) At(casterLevel int) Value {
	if r.Kind != RangeDistance {
		return Value{}
	}
	return r.Distance.At(casterLevel)
}

// Duration kinds.
const (
	DurationInstantaneous = "instantaneous"
	DurationPermanent     = "permanent"
	DurationConcentration = "concentration"
	DurationUntil         = "until"
)

// Duration is "3 rds. + 1 rd./level", "Instantaneous", "Until dispelled".
type Duration struct {
	Raw  string `json:"raw"`
	Kind string `json:"kind"` // instantaneous | permanent | concentration | until | timed | special
	// Time is set for timed.
	Time Quantity `json:"time"`
	// Condition is the text after "Until" / "Permanent until".
	Condition string `json:"condition,omitempty"`
}

var untilRe = regexp.MustCompile(`(?i)^(?:permanent\s+)?until\s+(.+)$`)

// ParseDuration reads a PHB duration.
func ParseDuration(raw string) Duration {
	d := Duration{Raw: raw, Kind: KindSpecial}
	s := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(raw), ". "))
	switch strings.ToLower(s) {
	case "instantaneous":
		d.Kind = DurationInstantaneous
		return d
	case "permanent":
		d.Kind = DurationPermanent
		return d
	case "concentration":
		d.Kind = DurationConcentration
		return d
	case "one touch":
		d.Kind, d.Condition = DurationUntil, "one touch"
		return d
	}
	if m := untilRe.FindStringSubmatch(s); m != nil {
		d.Kind, d.Condition = DurationUntil, strings.ToLower(m[1])
		if strings.HasPrefix(strings.ToLower(s), "permanent") {
			d.Kind = DurationPermanent
		}
		return d
	}
	if q, err := parseQuantity(s, ""); err == nil && q.Terms[0].Unit != "" {
		d.Kind, d.Time = KindTimed, q
	}
	return d
}

// At computes the duration (zero Value unless Kind is timed).
func (d Duration) At(casterLevel int) Value {
	if d.Kind != KindTimed {
		return Value{}
	}
	return d.Time.At(casterLevel)
}

// Save kinds.
const (
	SaveNone    = "none"
	SaveNegates = "negates"
	SaveHalf    = "half"
	SaveSpecial = "special"
)

// Save is the saving throw: "None", "Neg.", "1/2", "Special", "Neg. or 1/2".
// Kinds lists the possible outcomes in the order the PHB gives them.
type Save struct {
	Raw   string   `json:"raw"`
	Kinds []string `json:"kinds"`
}

var saveWordRe = regexp.MustCompile(`(?i)[a-z0-9/.]+`)

// ParseSave reads a PHB saving throw. Anything unreadable is [special].
func ParseSave(raw string) Save {
	sv := Save{Raw: raw}
	s := strings.ToLower(strings.TrimSpace(raw))
	for _, w := range saveWordRe.FindAllString(s, -1) {
		switch strings.TrimRight(w, ".") {
		case "none":
			sv.Kinds = append(sv.Kinds, SaveNone)
		case "neg":
			sv.Kinds = append(sv.Kinds, SaveNegates)
		case "1/2":
			sv.Kinds = append(sv.Kinds, SaveHalf)
		case "special":
			sv.Kinds = append(sv.Kinds, SaveSpecial)
		case "or":
		default:
			return Save{Raw: raw, Kinds: []string{SaveSpecial}}
		}
	}
	if len(sv.Kinds) == 0 {
		sv.Kinds = []string{SaveSpecial}
	}
	return sv
}

// Components is "V, S, M", "V, S (M optional)", "V, S, special".
type Components struct {
	Raw       string `json:"raw"`
	V         bool   `json:"v"`
	S         bool   `json:"s"`
	M         bool   `json:"m"`
	MOptional bool   `json:"m_optional,omitempty"`
	// Special is set when the text has a component beyond V, S, M.
	Special bool `json:"special,omitempty"`
}

var mOptionalRe = regexp.MustCompile(`(?i)\(\s*M\s+optional\s*\)`)

// ParseComponents reads a PHB components list.
func ParseComponents(raw string) Components {
	c := Components{Raw: raw}
	s := raw
	if mOptionalRe.MatchString(s) {
		c.M, c.MOptional = true, true
		s = mOptionalRe.ReplaceAllString(s, "")
	}
	for _, tok := range strings.Split(s, ",") {
		switch strings.ToUpper(strings.TrimSpace(tok)) {
		case "V":
			c.V = true
		case "S":
			c.S = true
		case "M":
			c.M = true
		case "":
		default:
			c.Special = true
		}
	}
	return c
}
