package spellsvc

import (
	"regexp"
	"strings"
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

// NameMatch is a spell named by the client: the search text matched the
// name the client knows for Key (a French name, say, which the server does
// not have) with Tier. Key is a catalogue key: it names every spell whose
// unique key is Key or Key plus a "_priest" / "_l<level>" suffix (BaseKey).
type NameMatch struct {
	Key  string
	Tier Tier
}

var keySuffix = regexp.MustCompile(`(_priest)?(_l\d+)?$`)

// BaseKey strips the "_priest" / "_l<level>" suffix Keys adds to a spell
// sharing its name with another ("hold_person_priest" -> "hold_person"), the
// key of the name in the translation catalogues.
func BaseKey(id string) string {
	if b := keySuffix.ReplaceAllString(id, ""); b != "" {
		return b
	}
	return id
}

// nameTier ranks name against the search text q (both compared lowercase,
// q trimmed). An empty q matches nothing.
func nameTier(name, q string) Tier {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return TierNone
	}
	n := strings.ToLower(name)
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

// tier is the best tier of a spell over the English-name match and the
// client's matches. With no search text and no client match, every spell
// matches with TierExact (no ranking).
func (f Filter) tier(sp Spell) Tier {
	if strings.TrimSpace(f.Query) == "" && len(f.Names) == 0 {
		return TierExact
	}
	best := nameTier(sp.Name, f.Query)
	base := BaseKey(sp.ID)
	for _, m := range f.Names {
		if m.Tier <= TierNone || m.Tier > TierContains {
			continue
		}
		if (m.Key == sp.ID || m.Key == base) && (best == TierNone || m.Tier < best) {
			best = m.Tier
		}
	}
	return best
}
