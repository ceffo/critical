package ruletablesvc

import (
	"errors"
	"fmt"
)

// The cleric's rules (ADR 0030 steps 13 to 16): the Wisdom bonus spells, the
// low-Wisdom spell failure and turning undead. The numbers are the class
// tables' (PHB Tables 5, 24 and 61); this file says which row, column and
// cell a cleric reads. The spheres of access are the class's sphere_access
// feature (classes.SphereAccess).

// ErrCannotTurn means the Table 61 cell is a dash; refused before any die (ADR
// 0030 step 16.3).
var ErrCannotTurn = errors.New("this cleric cannot turn that undead")

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
	return t.priestSpells(level, wis, true)
}

// priestSpells is PriestSpellsPerDay, with Table 5's bonus spells only when
// bonus (the wisdom_bonus_spells kind, ADR 0040 step 3).
func (t *Tables) priestSpells(level, wis int, bonus bool) (map[int]int, error) {
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
		if !bonus || r.WisMin > wis {
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

// MagicDefenceFor is ADR 0006 step 9.1: PHB Table 5's magical defence
// adjustment at Wisdom wis, for a character of any class (step 9.2). A score
// outside the table is an error.
func (t *Tables) MagicDefenceFor(wis int) (int, error) {
	r, err := t.WisdomFor(wis)
	if err != nil {
		return 0, err
	}
	return r.MagicDefence, nil
}

// SpellImmunitiesFor is ADR 0006 step 10.1: PHB Table 5's spell immunities
// at Wisdom wis, cumulative (every row from the lowest up to wis), as
// catalogue spell ids in table order; none below 19. An error for a score
// outside the table.
func (t *Tables) SpellImmunitiesFor(wis int) ([]string, error) {
	if _, err := t.WisdomFor(wis); err != nil {
		return nil, err
	}
	c, err := t.classTables()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, r := range c.Wisdom {
		if r.WisMin <= wis {
			out = append(out, r.SpellImmunities...)
		}
	}
	return out, nil
}

// SpellFailureFor is ADR 0030 step 14: the chance (percent) that a spell of
// a character with Wisdom wis fails, for a class with spell_failure (ADR 0040
// step 3; the caller reads the feature); 0 for a Wisdom of 13 and up (no die
// is rolled).
func (t *Tables) SpellFailureFor(wis int) (int, error) {
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
