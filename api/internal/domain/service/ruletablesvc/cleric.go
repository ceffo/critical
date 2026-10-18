package ruletablesvc

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// The cleric's rules (ADR 0030 steps 13 to 16): the Wisdom bonus spells, the
// low-Wisdom spell failure, the spheres of access and turning undead. The
// numbers are the class tables' (PHB Tables 5, 24 and 61) and the sphere
// access the catalogue's (format 6); this file says which row, column and
// cell a cleric reads.

// wisdomFailureClasses are the classes whose low Wisdom may make a spell
// fail and whose high Wisdom adds bonus spells (ADR 0030 steps 13 and 14,
// PHB Table 5: priests).
var wisdomFailureClasses = []string{"cleric"}

// turningClasses are the classes that turn undead (ADR 0030 step 16).
var turningClasses = []string{"cleric"}

// MinorSphereMaxLevel is the highest spell level a minor sphere opens (ADR
// 0030 step 15.2, PHB chapter 3: minor access, spells of 1st to 3rd level).
const MinorSphereMaxLevel = 3

// Sphere access kinds (ADR 0030 step 15.1).
const (
	AccessMajor = "major"
	AccessMinor = "minor"
)

// SphereAccesses are the access kinds a sphere_access row may carry.
var SphereAccesses = []string{AccessMajor, AccessMinor}

// ErrCannotTurn means the Table 61 cell is a dash; refused before any die (ADR
// 0030 step 16.3).
var ErrCannotTurn = errors.New("this cleric cannot turn that undead")

// ErrNoSphereAccess means the artifact carries no sphere access (a format-5
// catalogue, ADR 0021 step 12.4).
var ErrNoSphereAccess = errors.New("the spell catalogue carries no sphere access: run `just catalogue` to fetch a format-6 artifact")

// UsesWisdomSpells reports whether class gets PHB Table 5's bonus spells and
// spell failure (ADR 0030 steps 13 and 14).
func UsesWisdomSpells(class string) bool { return slices.Contains(wisdomFailureClasses, class) }

// TurnsUndead reports whether class turns undead (ADR 0030 step 16).
func TurnsUndead(class string) bool { return slices.Contains(turningClasses, class) }

// WisdomFor is PHB Table 5's row of a Wisdom score (bonus spells and chance
// of spell failure). A score outside the table is an error.
func (t *Tables) WisdomFor(score int) (WisdomRow, error) {
	c, err := t.classTables()
	if err != nil {
		return WisdomRow{}, err
	}
	for _, r := range c.Wisdom {
		if score >= r.WisMin && score <= r.WisMax {
			return r, nil
		}
	}
	return WisdomRow{}, fmt.Errorf("wisdom: score %d outside the table", score)
}

// PriestSpellsPerDay is ADR 0030 step 13 (with step 8): PHB Table 24 at
// level, a level needing more Wisdom than wis (Table 24's notes: 6th needs
// 17, 7th 18) at 0, plus Table 5's bonus spells of every Wisdom row from the
// lowest up to wis, each only at a spell level Table 24 gives at least one
// slot of. Levels with no slot are absent.
func (t *Tables) PriestSpellsPerDay(level, wis int) (map[int]int, error) {
	c, err := t.classTables()
	if err != nil {
		return nil, err
	}
	if level < 1 || level > len(c.PriestSpells) {
		return nil, fmt.Errorf("priest spells: level %d out of range 1..%d", level, len(c.PriestSpells))
	}
	if _, err := t.WisdomFor(wis); err != nil {
		return nil, err
	}
	out := map[int]int{}
	for i, n := range c.PriestSpells[level-1].PerSpellLevel {
		spellLevel := i + 1
		if n > 0 && priestLevelAllowed(c, spellLevel, wis) {
			out[spellLevel] = n
		}
	}
	for _, r := range c.Wisdom {
		if r.WisMin > wis {
			break
		}
		for _, l := range r.BonusSpells {
			if out[l] > 0 {
				out[l]++
			}
		}
	}
	return out, nil
}

func priestLevelAllowed(c *ClassTables, spellLevel, wis int) bool {
	for _, r := range c.PriestSpellWisdom {
		if r.SpellLevel == spellLevel && wis < r.WisMin {
			return false
		}
	}
	return true
}

// SpellFailureFor is ADR 0030 step 14: the chance (percent) that a spell of
// a character of class with Wisdom wis fails; 0 for a class without it or a
// Wisdom of 13 and up (no die is rolled).
func (t *Tables) SpellFailureFor(class string, wis int) (int, error) {
	if !UsesWisdomSpells(class) {
		return 0, nil
	}
	r, err := t.WisdomFor(wis)
	if err != nil {
		return 0, err
	}
	return r.SpellFailure, nil
}

// TurningTarget is one Table 61 row as a cleric of a level reads it (ADR
// 0030 step 16): the undead type, its Hit Dice and the cell at the level.
type TurningTarget struct {
	Undead       string
	HDMin, HDMax int
	Cell         TurningCell
}

// TurningAt is Table 61 at a cleric's level, the book's rows in order:
// column level for levels 1 to 9, the shared columns above (the table is
// expanded per level). An error for a level out of 1..20.
func (t *Tables) TurningAt(level int) ([]TurningTarget, error) {
	c, err := t.classTables()
	if err != nil {
		return nil, err
	}
	if level < 1 || level > MaxLevel {
		return nil, fmt.Errorf("turning undead: level %d out of range 1..%d", level, MaxLevel)
	}
	out := make([]TurningTarget, len(c.TurningUndead))
	for i, r := range c.TurningUndead {
		out[i] = TurningTarget{Undead: r.Undead, HDMin: r.HDMin, HDMax: r.HDMax, Cell: r.ByLevel[level-1]}
	}
	return out, nil
}

// SphereAccessRow is one sphere a class has access to (catalogue format 6,
// ADR 0030 step 15.1): the class code, the sphere code and the access kind.
type SphereAccessRow struct {
	Class, Sphere, Access string
}

// SphereAccess is the validated sphere access of the classes (format 6).
// Built by NewSphereAccess only.
type SphereAccess struct {
	byClass map[string]map[string]string
}

// NewSphereAccess validates rows: known class and access codes, each class
// and sphere once, and returns them.
func NewSphereAccess(rows []SphereAccessRow) (*SphereAccess, error) {
	out := &SphereAccess{byClass: map[string]map[string]string{}}
	for _, r := range rows {
		if !slices.Contains(Classes, r.Class) {
			return nil, fmt.Errorf("unknown class %q", r.Class)
		}
		if !slices.Contains(SphereAccesses, r.Access) {
			return nil, fmt.Errorf("%s %s: unknown access %q", r.Class, r.Sphere, r.Access)
		}
		if r.Sphere == "" {
			return nil, fmt.Errorf("%s: empty sphere code", r.Class)
		}
		m := out.byClass[r.Class]
		if m == nil {
			m = map[string]string{}
			out.byClass[r.Class] = m
		}
		if _, dup := m[r.Sphere]; dup {
			return nil, fmt.Errorf("%s %s twice", r.Class, r.Sphere)
		}
		m[r.Sphere] = r.Access
	}
	return out, nil
}

// CheckSpheres refuses a sphere access row whose sphere code is on no spell
// of known (the catalogue's spells' sphere codes): a typo would silently
// close the sphere.
func (a *SphereAccess) CheckSpheres(known []string) error {
	if a == nil {
		return nil
	}
	for _, class := range slices.Sorted(maps.Keys(a.byClass)) {
		for _, sphere := range slices.Sorted(maps.Keys(a.byClass[class])) {
			if !slices.Contains(known, sphere) {
				return fmt.Errorf("sphere_access: %s sphere %q is on no spell of the catalogue", class, sphere)
			}
		}
	}
	return nil
}

// HasSpheres reports whether class prepares from spheres (it has a sphere
// access row) rather than from a spellbook (ADR 0030 step 15).
func (a *SphereAccess) HasSpheres(class string) bool {
	return a != nil && len(a.byClass[class]) > 0
}

// Opens is ADR 0030 step 15.2: a spell of spellLevel with the sphere codes
// spheres is open to class when one of its spheres is a major sphere of the
// class (any level) or a minor one at levels 1 to MinorSphereMaxLevel.
func (a *SphereAccess) Opens(class string, spellLevel int, spheres []string) bool {
	if a == nil {
		return false
	}
	access := a.byClass[class]
	for _, s := range spheres {
		switch access[s] {
		case AccessMajor:
			return true
		case AccessMinor:
			if spellLevel <= MinorSphereMaxLevel {
				return true
			}
		}
	}
	return false
}
