package ruletablesvc

import (
	"errors"
	"fmt"
)

// The lookups the character's derived values read (ADR 0030 steps 2, 3, 5, 6
// and 22.1). The combination (which group, which race gets which bonus) is
// charactersvc's; the numbers are the class tables' (catalogue format 4 and
// up).

// ErrNoClassTables is a lookup on a catalogue that carries no class tables
// (format 3): the server refuses to start on one (the composition root).
var ErrNoClassTables = errors.New("the spell catalogue carries no class tables (format 3): run `just catalogue`")

// classGroups is ADR 0030 step 2: fighter is a warrior, mage a wizard, cleric
// a priest, thief and bard rogues.
var classGroups = map[string]string{
	"fighter": "warrior",
	"mage":    "wizard",
	"cleric":  "priest",
	"thief":   "rogue",
	"bard":    "rogue",
}

// ClassGroup is the class group of class (ADR 0030 step 2), an error for a
// code that is not one of Classes.
func ClassGroup(class string) (string, error) {
	g, ok := classGroups[class]
	if !ok {
		return "", fmt.Errorf("unknown class %q", class)
	}
	return g, nil
}

func (t *Tables) classTables() (*ClassTables, error) {
	if t.classes == nil {
		return nil, ErrNoClassTables
	}
	return t.classes, nil
}

// XPThreshold is the XP that reaches level in class's experience table (PHB
// Tables 14, 20, 23 and 25), an error outside 1..MaxLevel.
func (t *Tables) XPThreshold(class string, level int) (int, error) {
	c, err := t.classTables()
	if err != nil {
		return 0, err
	}
	col, ok := experienceColumns[class]
	if !ok {
		return 0, fmt.Errorf("unknown class %q", class)
	}
	if level < 1 || level > len(c.Experience) {
		return 0, fmt.Errorf("%s level %d out of range 1..%d", class, level, len(c.Experience))
	}
	return col(c.Experience[level-1]), nil
}

// LevelFor is ADR 0030 step 3: the highest level whose threshold in class's
// experience table is at or under xp, from 1 to MaxLevel (the table stops at
// 20, so does the level). Negative XP is level 1, as 0 is.
func (t *Tables) LevelFor(class string, xp int) (int, error) {
	c, err := t.classTables()
	if err != nil {
		return 0, err
	}
	col, ok := experienceColumns[class]
	if !ok {
		return 0, fmt.Errorf("unknown class %q", class)
	}
	level := 1
	for _, r := range c.Experience {
		if col(r) <= xp {
			level = r.Level
		}
	}
	return level, nil
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
