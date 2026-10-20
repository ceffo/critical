// Package spellbook is a character's spellbook as the rules see it: the
// wizard spells it holds, each with the origin of its entry (ADR 0030 step
// 20.5). The origin never changes a rule of the book: a spell is the same
// once in.
package spellbook

// Origin is how an entry came into the spellbook (ADR 0030 step 20.5).
type Origin string

// The origins of step 20.5.
const (
	// OriginLearned is a Learn roll that succeeded (step 20.2).
	OriginLearned Origin = "learned"
	// OriginGiven is a roll-free add, Ajouter (step 20.6).
	OriginGiven Origin = "given"
	// OriginStarting is the book a character starts with (step 20.7), and
	// every entry stored before the origins were (Kelsen's imported book).
	OriginStarting Origin = "starting"
	// OriginLevelGrant is the free spell a level grants (step 20.8); the
	// entry carries the realized level that granted it.
	OriginLevelGrant Origin = "level_grant"
)

// Valid reports whether o is one of the origins of step 20.5.
func (o Origin) Valid() bool {
	switch o {
	case OriginLearned, OriginGiven, OriginStarting, OriginLevelGrant:
		return true
	}
	return false
}

// Entry is one spell of a character's spellbook: its catalogue key, its
// origin and, for OriginLevelGrant, the realized level that granted it (0
// otherwise).
type Entry struct {
	Key        string
	Origin     Origin
	GrantLevel int
}
