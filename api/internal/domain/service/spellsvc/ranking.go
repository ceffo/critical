package spellsvc

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Tier is how well a spell's name matches the search text; lower ranks first.
// TierNone means no match.
type Tier int

// Match tiers, best first.
const (
	TierNone Tier = iota
	TierExact
	TierPrefix
	TierContains
)

// ligatures are folded to their letters ("Œil" -> "oeil").
var ligatures = strings.NewReplacer("œ", "oe", "æ", "ae")

// Fold is s lowercased without accents or ligatures, for matching typed text
// against names in any language ("Éclair" and "eclair" fold alike).
func Fold(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, strings.ToLower(s))
	if err != nil {
		out = strings.ToLower(s)
	}
	return ligatures.Replace(out)
}

// nameTier ranks name against the folded search text q. An empty q matches
// nothing.
func nameTier(name, q string) Tier {
	if q == "" || name == "" {
		return TierNone
	}
	n := Fold(name)
	switch {
	case n == q:
		return TierExact
	case strings.HasPrefix(n, q):
		return TierPrefix
	case strings.Contains(n, q):
		return TierContains
	}
	return TierNone
}

// tier is the best tier of a spell over its English and French names (case,
// accents and ligatures ignored). With no search text every spell matches
// with TierExact (no ranking).
func (f Filter) tier(sp Spell) Tier {
	q := Fold(strings.TrimSpace(f.Query))
	if q == "" {
		return TierExact
	}
	best := nameTier(sp.Name, q)
	if fr := nameTier(sp.NameFR, q); fr != TierNone && (best == TierNone || fr < best) {
		best = fr
	}
	return best
}
