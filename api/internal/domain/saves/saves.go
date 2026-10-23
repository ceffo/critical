// Package saves owns the saving-throw figure (ADR 0046 step 3): the five
// targets, the best of several class entries' rows, the race's Constitution
// bonus and the class save_bonus grants. It is pure (ADR 0046 step 1.2): the
// table rows are values handed in by the caller that reads the catalogue
// (charactersvc); no port, no dice, no store. No type here carries a
// transport tag (ADR 0046 step 6.1).
package saves

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// Categories are the saving throw category codes, in the order of the
// character sheet: the one list (the catalogue's, classes.SaveCategories),
// so no package re-types it.
var Categories = classes.SaveCategories

// Saves are the five targets of a character, or one per-category value of
// any other save figure (a race's bonus): PHB Table 60's columns.
type Saves struct {
	ParaPoison   int
	RodStaffWand int
	PetriPoly    int
	Breath       int
	Spell        int
}

// Of is the value of category cat, false for a code that is none of
// Categories.
func (s Saves) Of(cat string) (int, bool) {
	if p := s.field(cat); p != nil {
		return *p, true
	}
	return 0, false
}

// field is the field of s for a category code, nil for a code that is none:
// the one switch on a category.
func (s *Saves) field(cat string) *int {
	switch cat {
	case Categories[0]:
		return &s.ParaPoison
	case Categories[1]:
		return &s.RodStaffWand
	case Categories[2]:
		return &s.PetriPoly
	case Categories[3]:
		return &s.Breath
	case Categories[4]:
		return &s.Spell
	}
	return nil
}

// Best is ADR 0042 step 9 (amending ADR 0030 step 6): category by category,
// the lowest target among the rows of the entries that count (the caller
// says which, ADR 0042 step 13). Two classes' rows may mix (ADR 0042 Q6). No
// row is the zero Saves.
func Best(rows ...Saves) Saves {
	if len(rows) == 0 {
		return Saves{}
	}
	best := rows[0]
	for _, r := range rows[1:] {
		best = Saves{
			ParaPoison: min(best.ParaPoison, r.ParaPoison), RodStaffWand: min(best.RodStaffWand, r.RodStaffWand),
			PetriPoly: min(best.PetriPoly, r.PetriPoly), Breath: min(best.Breath, r.Breath), Spell: min(best.Spell, r.Spell),
		}
	}
	return best
}

// RaceBonus is ADR 0030 step 22.1 through the race's con_save_bonus trait
// (ADR 0040 step 4): bonus, PHB Table 9's value at the sheet's Constitution,
// on each of the trait's categories; a code that is no category adds
// nothing.
func RaceBonus(categories []string, bonus int) Saves {
	var out Saves
	for _, cat := range categories {
		if p := out.field(cat); p != nil {
			*p = bonus
		}
	}
	return out
}

// ClassBonus is one class's save_bonus grant (ADR 0043 step 3): Bonus on the
// saves whose request carries the tag Against.
type ClassBonus struct {
	Class   string
	Against string
	Bonus   int
}

// ClassBonuses is ADR 0043 step 3 over the grants of the entries that count
// (the caller drops the dormant ones, ADR 0042 step 9): per tag the best
// bonus, the first grant's on a tie (two classes' bonuses against one tag do
// not add up, as two classes' saves do not). Nil for no grant.
func ClassBonuses(grants []ClassBonus) []ClassBonus {
	var out []ClassBonus
	for _, b := range grants {
		i := slices.IndexFunc(out, func(o ClassBonus) bool { return o.Against == b.Against })
		switch {
		case i < 0:
			out = append(out, b)
		case b.Bonus > out[i].Bonus:
			out[i] = b
		}
	}
	return out
}
