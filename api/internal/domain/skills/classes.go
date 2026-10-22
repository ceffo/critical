package skills

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ClassLevel is one active class entry as the proficiency pools read it: its class and its
// realized level (ADR 0042 step 15).
type ClassLevel struct {
	Class string
	Level int
}

// Classes are a character's active class entries, in order: one for a single-class character.
type Classes []ClassLevel

// One is the single class entry of class at level.
func One(class string, level int) Classes { return Classes{{Class: class, Level: level}} }

// first is the first class, "" for none.
func (cls Classes) first() string {
	if len(cls) == 0 {
		return ""
	}
	return cls[0].Class
}

// groups are the Table 38 groups of every class, each once (ADR 0042 step 15, Q13: a
// non-weapon proficiency is in-group when it is in any active class's groups).
func (cls Classes) groups(book Book) []string {
	var out []string
	for _, c := range cls {
		for _, g := range book.GroupsOf(c.Class) {
			if !slices.Contains(out, g) {
				out = append(out, g)
			}
		}
	}
	return out
}

// basis is what the pools are read on: one Table 34 row and the level each pool counts.
type basis struct {
	row                         ruletablesvc.SlotRow
	weaponLevel, nonweaponLevel int
	groups                      []string
}

// basis is ADR 0042 step 15 (amending ADR 0037 step 3), each column of Table 34 on its own: the
// largest initial count, the fastest rate (the fewest levels per slot), counted on the level of
// the entry whose rate is used (Q11; the highest when several share it), and the smallest
// non-proficiency penalty (Q12: the one closest to 0). One class is its own row at its level.
// ok is false when no class has a Table 34 row.
func (cls Classes) basis(book Book) (basis, bool) {
	var b basis
	found := false
	for _, c := range cls {
		row, ok := book.SlotRowOf(c.Class)
		if !ok {
			continue
		}
		if !found {
			b = basis{row: row, weaponLevel: c.Level, nonweaponLevel: c.Level}
			found = true
			continue
		}
		b.row.WeaponInitial = max(b.row.WeaponInitial, row.WeaponInitial)
		b.row.NonweaponInitial = max(b.row.NonweaponInitial, row.NonweaponInitial)
		b.row.Penalty = max(b.row.Penalty, row.Penalty)
		b.row.WeaponLevels, b.weaponLevel = fastest(b.row.WeaponLevels, b.weaponLevel, row.WeaponLevels, c.Level)
		b.row.NonweaponLevels, b.nonweaponLevel = fastest(b.row.NonweaponLevels, b.nonweaponLevel, row.NonweaponLevels, c.Level)
	}
	if !found {
		return basis{}, false
	}
	b.groups = cls.groups(book)
	return b, true
}

// fastest is the faster of two rates (fewer levels per slot) with the level it is counted on,
// the higher level when both rates are the same.
func fastest(levels, level, otherLevels, otherLevel int) (int, int) {
	switch {
	case otherLevels < levels:
		return otherLevels, otherLevel
	case otherLevels == levels:
		return levels, max(level, otherLevel)
	}
	return levels, level
}

// with is cls with the level of the entry of class set to level.
func (cls Classes) with(class string, level int) Classes {
	out := slices.Clone(cls)
	for i := range out {
		if out[i].Class == class {
			out[i].Level = level
		}
	}
	return out
}
