package spellbook

import "sort"

// FreeSpell is one free spell a level grants (ADR 0030 step 20.8): the
// realized level that grants it and the spell level it must be of.
type FreeSpell struct {
	Level      int
	SpellLevel int
}

// FreeSpellsAt is step 20.8 for a class whose levels grant free spells (a
// mage; the caller asks the class): realized level grants one free spell of
// each spell level it opens, the spell levels with slots at level (at) and
// none at the level below (below; nil at level 1), up to intMax, the
// Intelligence's maximum spell level (PHB Table 4: a spell level above it
// grants nothing, and nothing is deferred). Lowest spell level first; nil
// when none.
func FreeSpellsAt(level int, below, at map[int]int, intMax int) []FreeSpell {
	var out []FreeSpell
	for spellLevel, n := range at {
		if n > 0 && below[spellLevel] == 0 && spellLevel <= intMax {
			out = append(out, FreeSpell{Level: level, SpellLevel: spellLevel})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SpellLevel < out[j].SpellLevel })
	return out
}

// Owed is step 20.8's "free spells owed": the free spells the levels taken
// in the app granted (granted), minus the ones claimed (the OriginLevelGrant
// entries, each as its grant level and its spell's level). Each claim pays
// one grant of the same level and spell level. In granted's order; nil when
// none is owed.
func Owed(granted, claimed []FreeSpell) []FreeSpell {
	paid := make(map[FreeSpell]int, len(claimed))
	for _, c := range claimed {
		paid[c]++
	}
	var out []FreeSpell
	for _, g := range granted {
		if paid[g] > 0 {
			paid[g]--
			continue
		}
		out = append(out, g)
	}
	return out
}
