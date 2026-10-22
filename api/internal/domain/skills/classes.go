package skills

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ClassLevel is one class entry as the proficiency pools read it: its class and its realized
// level (ADR 0042 step 15). Frozen is a dual-class's old class, at its left-at level: it keeps the
// slots it had (Classes.kept) and takes no part in the active entries' combination.
type ClassLevel struct {
	Class  string
	Level  int
	Frozen bool
}

// Classes are a character's class entries, in order (a dual-class's first class first): one for
// a single-class character.
type Classes []ClassLevel

// active are the entries that are not Frozen: what the pools' rate, penalty and groups, the weapon
// picks and the specialization read (ADR 0042 step 15).
func (cls Classes) active() Classes {
	var out Classes
	for _, c := range cls {
		if !c.Frozen {
			out = append(out, c)
		}
	}
	return out
}

// One is the single class entry of class at level.
func One(class string, level int) Classes { return Classes{{Class: class, Level: level}} }

// specialist is the class whose weapon_specialization a specialization reads: the single active
// class, "" for a multi-class character (ADR 0042 as built, PHB p.52: specialization is a
// single-class fighter's only).
func (cls Classes) specialist() string {
	act := cls.active()
	if len(act) != 1 {
		return ""
	}
	return act[0].Class
}

// groups are the Table 38 groups of every class, each once (ADR 0042 step 15, Q13: a
// non-weapon proficiency is in-group when it is in any active class's groups).
func (cls Classes) groups(book Book) []string {
	var out []string
	for _, c := range cls.active() {
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
	for _, c := range cls.active() {
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

// keptSlots are the slots a dual-class's frozen entries had when left (ADR 0042 step 15, Q14):
// the first class's initial counts, when it is frozen, and each frozen class's slots gained up
// to its left-at level. The active entry adds its own gained slots, no second initial count.
type keptSlots struct {
	weaponInitial, weaponGained, nonweaponInitial, nonweaponGained int
}

// kept is the frozen entries' keptSlots; ok is false when cls has no frozen entry with a Table
// 34 row (not a dual-class character).
func (cls Classes) kept(book Book) (keptSlots, bool) {
	var k keptSlots
	found := false
	for i, c := range cls {
		if !c.Frozen {
			continue
		}
		row, ok := book.SlotRowOf(c.Class)
		if !ok {
			continue
		}
		found = true
		w := ruletablesvc.WeaponSlots(row, c.Level, 0)
		n := ruletablesvc.NonweaponSlots(row, c.Level, 0, 0)
		k.weaponGained += w.LevelGained
		k.nonweaponGained += n.LevelGained
		if i == 0 {
			k.weaponInitial, k.nonweaponInitial = w.Initial, n.Initial
		}
	}
	return k, found
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
		if out[i].Class == class && !out[i].Frozen {
			out[i].Level = level
		}
	}
	return out
}
