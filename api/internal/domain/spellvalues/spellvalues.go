// Package spellvalues finds the gameplay values in a spell text: dice, times, levels,
// distances and areas, percentages and modifiers (ADR 0021 step 15). It is pure recognition:
// a text in, the same text out as spans, the values marked with their kind. The API serves the
// spans so that the client highlights values without recognising anything.
//
// The unit words below ("round", "feet", "tour", "pieds"...) are matching data, the input
// vocabulary of the books' own words, like the stat parser's (spellstats). They are never
// emitted or displayed (ADR 0023). A language without a vocabulary here gets only the
// language-neutral kinds (dice, percentages, modifiers).
package spellvalues

import (
	"regexp"
	"sort"
	"unicode"
	"unicode/utf8"
)

// Kind is the kind of a gameplay value; the zero Kind marks plain text.
type Kind string

// The six kinds of gameplay value.
const (
	Plain    Kind = ""
	Dice     Kind = "dice"
	Time     Kind = "time"
	Level    Kind = "level"
	Distance Kind = "distance"
	Percent  Kind = "percent"
	Modifier Kind = "modifier"
)

// Span is a piece of a text; the spans of a text concatenate to exactly that text.
type Span struct {
	Text string `json:"text"`
	Kind Kind   `json:"kind,omitempty"`
}

// Fields are the spell texts the API marks, by their wire names.
var Fields = []string{"description", "casting_time", "range", "duration", "area_of_effect", "components", "saving_throw"}

// rule finds one kind of value: group 1 of re is the value, and the rule keeps the match only
// when the character before it is acceptable (see Mark).
type rule struct {
	kind     Kind
	re       *regexp.Regexp
	modifier bool // a signed number: needs a space, a parenthesis or the start before it
}

// end closes every pattern: the value is not followed by a letter or a digit (RE2 has no
// lookahead, so the character is consumed and group 1 is the value).
const end = `(?:[^\p{L}\p{N}]|$)`

const (
	num   = `\d+(?:[.,]\d+)?`
	rng   = num + `(?:\s?[-–]\s?` + num + `)?`
	dice  = `(?:\d+)?[dD]\d+(?:\s?[+\-−]\s?\d+)?`
	size  = `(?:(?:sq|cu)\.?\s?|(?:square|cubic|carrés?|cubes?)\s)?`
	quant = `(?:` + dice + `|` + rng + `)(?:\s|-)?`
)

func compile(kind Kind, pat string, modifier bool) rule {
	return rule{kind: kind, re: regexp.MustCompile(`(` + pat + `)` + end), modifier: modifier}
}

// perUnit follows a measure: "/level", "per 2 levels", "par niveau", "per round".
func perUnit(words string) string {
	return `(?:\s?/\s?|\s+(?:per|par|each|chaque)\s+)(?:\d+\s+)?(?:` + words + `)`
}

var neutral = []rule{
	compile(Percent, num+`\s?%`, false),
	compile(Dice, dice, false),
	compile(Modifier, `[+\-−–]\d+`, true),
}

// vocabulary is the matching data of one language.
type vocabulary struct {
	time, distance, per string // alternations
	ordinal, levelWord  string
}

var vocabularies = map[string]vocabulary{
	"en": {
		time:      `melee rounds?|rounds?|rds?\.?|turns?|segments?|seconds?|minutes?|hours?|hrs?\.?|days?|weeks?|months?|years?|yrs?\.?`,
		distance:  `feet|foot|ft\.?|yards?|yds?\.?|inch(?:es)?|in\.|miles?|mi\.?|paces?|meters?|metres?|km|cm|m[²³]?`,
		per:       `levels?|rounds?|turns?|hours?|days?`,
		ordinal:   `\d+(?:st|nd|rd|th)(?:[- ]levels?)?`,
		levelWord: `(?:per|each|every)\s+(?:\d+\s+)?levels?|/\s?(?:\d+\s+)?levels?|[Ll]evel\s+\d+`,
	},
	"fr": {
		time:      `rounds?|rds?\.?|tours?|segments?|secondes?|minutes?|heures?|jours?|semaines?|mois|ans?|années?`,
		distance:  `pieds?|pds?\.?|yards?|yds?\.?|pouces?|miles?|lieues?|mètres?|metres?|km[²]?|cm|m[²³]?`,
		per:       `niveaux?|rounds?|tours?|heures?|jours?`,
		ordinal:   `\d+(?:er|re|ère|e|è|ème)\s+niveau`,
		levelWord: `(?:par|chaque|tous les)\s+(?:\d+\s+)?niveaux?|/\s?(?:\d+\s+)?niveaux?|[Nn]iveau\s+\d+`,
	},
}

var rules = func() map[string][]rule {
	out := map[string][]rule{}
	for lang, v := range vocabularies {
		per := `(?:` + perUnit(v.per) + `)?`
		rs := []rule{
			compile(Time, quant+`(?:`+v.time+`)`+per, false),
			compile(Distance, quant+size+`(?:`+v.distance+`)`+per, false),
			compile(Level, v.ordinal, false),
			compile(Level, v.levelWord, false),
		}
		out[lang] = append(rs, neutral...)
	}
	return out
}()

type found struct {
	from, to int
	kind     Kind
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// startOK says whether a value may begin at i: not inside a word or a number ("x10", "1,000"),
// and a signed number only after a space or a parenthesis.
func startOK(s string, i int, modifier bool) bool {
	if i == 0 {
		return true
	}
	prev, size := utf8.DecodeLastRuneInString(s[:i])
	if modifier {
		return unicode.IsSpace(prev) || prev == '('
	}
	if isWordRune(prev) {
		return false
	}
	if prev == '.' || prev == ',' {
		if before, _ := utf8.DecodeLastRuneInString(s[:i-size]); unicode.IsDigit(before) {
			return false
		}
	}
	return true
}

func (r rule) find(s string) []found {
	var out []found
	pos := 0
	for pos <= len(s) {
		m := r.re.FindStringSubmatchIndex(s[pos:])
		if m == nil {
			break
		}
		from, to := pos+m[2], pos+m[3]
		if startOK(s, from, r.modifier) {
			out = append(out, found{from, to, r.kind})
			pos = to
			continue
		}
		_, w := utf8.DecodeRuneInString(s[from:])
		pos = from + w
	}
	return out
}

// Mark is text as spans: its gameplay values, in language lang (a code such as "en" or "fr"; a
// region suffix is ignored), marked with their kind. At overlapping finds the leftmost, then the
// longest, wins. The spans concatenate to exactly text.
func Mark(lang, text string) []Span {
	rs, ok := rules[baseLang(lang)]
	if !ok {
		rs = neutral
	}
	var all []found
	for _, r := range rs {
		all = append(all, r.find(text)...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].from != all[j].from {
			return all[i].from < all[j].from
		}
		return all[i].to > all[j].to
	})
	var spans []Span
	at := 0
	for _, f := range all {
		if f.from < at {
			continue
		}
		if f.from > at {
			spans = append(spans, Span{Text: text[at:f.from]})
		}
		spans = append(spans, Span{Text: text[f.from:f.to], Kind: f.kind})
		at = f.to
	}
	if at < len(text) {
		spans = append(spans, Span{Text: text[at:]})
	}
	return spans
}

// HasValue says whether any span is a value.
func HasValue(spans []Span) bool {
	for _, s := range spans {
		if s.Kind != Plain {
			return true
		}
	}
	return false
}

func baseLang(lang string) string {
	for i, r := range lang {
		if r == '-' || r == '_' {
			return lang[:i]
		}
	}
	return lang
}
