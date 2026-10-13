package spellsvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// Filter narrows a catalog search. Zero fields do not filter. The stored
// columns (Query, Class, Level, School) are applied by the Reader; the typed
// stats (Components, Save, CastingTime) by the service.
type Filter struct {
	// Query is a name substring.
	Query string
	// Class is "Wizard" or "Priest" (any case); empty means both.
	Class string
	// Level is the spell level; nil means any.
	Level *int
	// School is a substring of the school text ("Evocation").
	School string
	// Components lists the letters (V, S, M) a spell must all need.
	Components []string
	// Save is one spellstats Save kind the spell's saving throw may give.
	Save string
	// CastingTime is the unit of the casting time ("segment", "round", ...) or
	// "special".
	CastingTime string
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
