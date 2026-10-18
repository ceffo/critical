package spellsvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// Filter narrows a catalog search. Zero fields do not filter. The Reader
// applies the stored columns Class and Levels; the service the rest (names,
// spellbook, school group, sphere, typed stats) and the ranking.
type Filter struct {
	// Query is matched against the English and the French name, ignoring
	// case and accents: exact, prefix or substring (Tier), the best of the
	// two. Empty means every spell.
	Query string
	// Class is "Wizard" or "Priest" (any case); empty means both.
	Class string
	// Mine keeps only the spells of the character's spellbook (set with Class
	// "wizard": the spellbook holds Wizard spells).
	Mine bool
	// Levels lists the spell levels to keep; empty means any.
	Levels []int
	// School is a school group key (SchoolGroups): a spell matches when any
	// school it lists maps to it.
	School string
	// Sphere is a Priest sphere (Spheres): a spell matches when it lists it.
	Sphere string
	// Components lists the letters (V, S, M) a spell must all need.
	Components []string
	// Save is one spellstats Save kind the spell's saving throw may give.
	Save string
	// CastingTime is the unit of the casting time ("segment", "round", ...) or
	// "special".
	CastingTime string
}

// matchGroups applies School and Sphere, read from the books' (fallback) text.
func (f Filter) matchGroups(sp Spell) bool {
	if f.School != "" && !slices.Contains(SpellSchoolGroups(sp.School.Default()), f.School) {
		return false
	}
	if f.Sphere != "" && !slices.Contains(SpellSpheres(sp.Sphere.Default()), f.Sphere) {
		return false
	}
	return true
}

func (f Filter) matchStats(st spellstats.Stats) bool {
	for _, c := range f.Components {
		switch c {
		case "V":
			if !st.Components.V {
				return false
			}
		case "S":
			if !st.Components.S {
				return false
			}
		case "M":
			if !st.Components.M {
				return false
			}
		}
	}
	if f.Save != "" && !slices.Contains(st.Save.Kinds, f.Save) {
		return false
	}
	if f.CastingTime != "" {
		unit := "special"
		if st.CastingTime.Kind == spellstats.KindTimed {
			unit = st.CastingTime.At(1).Unit
		}
		if unit != f.CastingTime {
			return false
		}
	}
	return true
}
