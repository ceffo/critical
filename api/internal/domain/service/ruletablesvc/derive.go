package ruletablesvc

import (
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// The lookups the character's derived values read (ADR 0030 steps 2, 3, 5, 6
// and 22.1). The combination (which group, which race gets which bonus) is
// charactersvc's; the numbers are the class tables' (catalogue format 4 and
// up).

// ErrNoClassTables is a lookup on a catalogue that carries no class tables
// (format 3): the server refuses to start on one (the composition root).
var ErrNoClassTables = errors.New("the spell catalogue carries no class tables (format 3): run `just catalogue`")

// ErrNoDefinitions is a lookup on tables built without the class
// definitions (format 11, ADR 0040 step 11.2): the server refuses to start on
// one (the composition root).
var ErrNoDefinitions = errors.New("the catalogue carries no class definitions (format 11): run `just catalogue`")

// Definitions are the class, group and race definitions (ADR 0040 step 5).
func (t *Tables) Definitions() (*classes.Registry, error) {
	if t.defs == nil {
		return nil, ErrNoDefinitions
	}
	return t.defs, nil
}

// Class is the definition of class (ADR 0040 step 1), an error for a class
// the catalogue does not define.
func (t *Tables) Class(class string) (classes.Class, error) {
	d, err := t.Definitions()
	if err != nil {
		return classes.Class{}, err
	}
	return d.Get(class)
}

// ClassGroup is the class group of class (ADR 0040 step 1.2, read from its
// definition), an error for a class the catalogue does not define.
func (t *Tables) ClassGroup(class string) (string, error) {
	c, err := t.Class(class)
	return c.Group, err
}

// PrimeRequisites are class's prime requisites (ADR 0040 step 6).
func (t *Tables) PrimeRequisites(class string) ([]string, error) {
	c, err := t.Class(class)
	return c.PrimeRequisites, err
}

func (t *Tables) classTables() (*ClassTables, error) {
	if t.classes == nil {
		return nil, ErrNoClassTables
	}
	return t.classes, nil
}

// XPThreshold is the XP that reaches level in class's experience table (PHB
// Tables 14, 20, 23 and 25, the class definition's rows, ADR 0040 step 1.7), an
// error outside 1..MaxLevel.
func (t *Tables) XPThreshold(class string, level int) (int, error) {
	c, err := t.Class(class)
	if err != nil {
		return 0, err
	}
	return c.XPThreshold(level)
}

// LevelFor is ADR 0030 step 3: the highest level whose threshold in class's
// experience table is at or under xp, from 1 to MaxLevel (the table stops at
// 20, so does the level). Negative XP is level 1, as 0 is.
func (t *Tables) LevelFor(class string, xp int) (int, error) {
	c, err := t.Class(class)
	if err != nil {
		return 0, err
	}
	return c.LevelFor(xp), nil
}

// THAC0For is ADR 0030 step 5: PHB Table 53's THAC0 for group at level.
func (t *Tables) THAC0For(group string, level int) (int, error) {
	c, err := t.classTables()
	if err != nil {
		return 0, err
	}
	if level < 1 || level > len(c.THAC0) {
		return 0, fmt.Errorf("thac0: level %d out of range 1..%d", level, len(c.THAC0))
	}
	r := c.THAC0[level-1]
	switch group {
	case "warrior":
		return r.Warrior, nil
	case "priest":
		return r.Priest, nil
	case "rogue":
		return r.Rogue, nil
	case "wizard":
		return r.Wizard, nil
	}
	return 0, fmt.Errorf("thac0: unknown class group %q", group)
}

// SavesFor is ADR 0030 step 6: PHB Table 60's row of group whose level band
// holds level.
func (t *Tables) SavesFor(group string, level int) (SavingThrowRow, error) {
	c, err := t.classTables()
	if err != nil {
		return SavingThrowRow{}, err
	}
	for _, r := range c.SavingThrows {
		if r.Group == group && level >= r.LevelMin && level <= r.LevelMax {
			return r, nil
		}
	}
	return SavingThrowRow{}, fmt.Errorf("saving throws: no %s row for level %d", group, level)
}

// ConSaveBonusFor is PHB Table 9: the saving throw bonus of a Constitution
// score; 0 below the table's first row (the table starts at 4), the last
// row's bonus above it (the table stops at 19, the racial maximum).
func (t *Tables) ConSaveBonusFor(con int) (int, error) {
	c, err := t.classTables()
	if err != nil {
		return 0, err
	}
	rows := c.ConSaveBonus
	if len(rows) > 0 && con > rows[len(rows)-1].ConMax {
		return rows[len(rows)-1].Bonus, nil
	}
	for _, r := range rows {
		if con >= r.ConMin && con <= r.ConMax {
			return r.Bonus, nil
		}
	}
	return 0, nil
}

// WizardSpellsPerDay is PHB Table 21 at level (ADR 0030 step 8): a mage's
// spells per day keyed by spell level, levels with none absent.
func (t *Tables) WizardSpellsPerDay(level int) (map[int]int, error) {
	c, err := t.classTables()
	if err != nil {
		return nil, err
	}
	if level < 1 || level > len(c.WizardSpells) {
		return nil, fmt.Errorf("wizard spells: level %d out of range 1..%d", level, len(c.WizardSpells))
	}
	out := map[int]int{}
	for i, n := range c.WizardSpells[level-1].PerSpellLevel {
		if n > 0 {
			out[i+1] = n
		}
	}
	return out, nil
}

// HitDiceFor is the hit dice row of group (ADR 0030 step 7): the die, the
// name level (the last level that rolls it) and the fixed gain per level
// after it (PHB Tables 14, 20, 23 and 25, Hit Dice column, with chapter 3's
// hit points paragraphs).
func (t *Tables) HitDiceFor(group string) (HitDiceRow, error) {
	c, err := t.classTables()
	if err != nil {
		return HitDiceRow{}, err
	}
	for _, r := range c.HitDice {
		if r.Group == group {
			return r, nil
		}
	}
	return HitDiceRow{}, fmt.Errorf("hit dice: unknown class group %q", group)
}

// ConstitutionFor is PHB Table 3's row of a Constitution score (ADR 0030 step
// 7): the hit point adjustment per die (HPWarrior for warriors) and the value
// a low die counts as (0: none). The table covers 1 to MaxScore; a score
// outside it is an error.
func (t *Tables) ConstitutionFor(con int) (ConstitutionRow, error) {
	c, err := t.classTables()
	if err != nil {
		return ConstitutionRow{}, err
	}
	for _, r := range c.Constitution {
		if con >= r.ConMin && con <= r.ConMax {
			return r, nil
		}
	}
	return ConstitutionRow{}, fmt.Errorf("constitution: score %d outside the table", con)
}
