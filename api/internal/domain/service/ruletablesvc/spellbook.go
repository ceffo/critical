package ruletablesvc

import "fmt"

// The spellbook's rules (ADR 0030 step 20): which classes keep a
// spellbook and the highest spell level each may write in it. Only mages and
// bards have one (user, 2026-10-17); the levels are each class's highest
// spell level in its slot table (PHB Table 21: 9th, Table 32: 6th).
var spellbookMaxLevels = map[string]int{"mage": 9, "bard": 6}

// SpellbookMaxLevel is the highest spell level class may write in its
// spellbook (ADR 0030 step 20.1); ok is false for a class without a
// spellbook.
func SpellbookMaxLevel(class string) (level int, ok bool) {
	level, ok = spellbookMaxLevels[class]
	return level, ok
}

// startingSpells are the spells a class's starting spellbook is offered with,
// ticked and removable (ADR 0030 step 20.7): the DM's starting books hold Read
// Magic and Detect Magic for a mage (DMG p. 40, "Player Choice"); a bard starts
// with none ticked. A rules table like the spellbook's highest levels above,
// served in the creation options, so no service branches on a spell.
var startingSpells = map[string][]string{"mage": {"read_magic", "detect_magic"}}

// StartingSpells are the catalogue ids of the spells a new character of class
// is offered pre-ticked in its starting spellbook (ADR 0030 step 20.7), a
// copy; empty for a class with none.
func StartingSpells(class string) []string {
	return append([]string{}, startingSpells[class]...)
}

// IntelligenceFor is PHB Table 4's row of an Intelligence score (ADR 0030
// step 20): the highest spell level, the chance to learn a spell and the
// maximum number of spells per level. A score outside the table is an
// error.
func (t *Tables) IntelligenceFor(score int) (IntelligenceRow, error) {
	c, err := t.classTables()
	if err != nil {
		return IntelligenceRow{}, err
	}
	for _, r := range c.Intelligence {
		if score >= r.IntMin && score <= r.IntMax {
			return r, nil
		}
	}
	return IntelligenceRow{}, fmt.Errorf("intelligence: score %d outside the table", score)
}
