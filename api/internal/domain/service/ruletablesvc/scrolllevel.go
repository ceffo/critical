package ruletablesvc

import "strings"

// MinScrollLevel is the lowest level a scroll spell is written at (DMG p.145,
// "Spell Level of Scroll Spells": never below 6th level of experience).
const MinScrollLevel = 6

// ScrollCasterLevel is the level a scroll's spell is written at (DMG p.145,
// "Spell Level of Scroll Spells": one level above the minimum level required
// to cast the spell, never below 6th; a 6th-level wizard spell is written at
// 13th level, a 7th-level one at 15th). The minimum level is read from the
// slot table of the spell's list (ADR 0030 step 8, ADR 0040 step 1.9: the
// table a class's spell_progression names, `<list>_spells`): the lowest level
// at which it holds a slot of spellLevel. ok is false when no table serves the
// list or none ever reaches the spell level (a scroll then keeps no level to
// give), or when the class tables are not loaded.
func (t *Tables) ScrollCasterLevel(list string, spellLevel int) (level int, ok bool) {
	if spellLevel < 1 {
		return 0, false
	}
	rows, ok := t.progressionRows(strings.ToLower(strings.TrimSpace(list)) + "_spells")
	if !ok {
		return 0, false
	}
	for _, r := range rows {
		if spellLevel <= len(r.PerSpellLevel) && r.PerSpellLevel[spellLevel-1] > 0 {
			return min(max(r.Level+1, MinScrollLevel), MaxScrollCasterLevel), true
		}
	}
	return 0, false
}

// MaxScrollCasterLevel is the highest caster level an item names (ADR 0049
// step 10).
const MaxScrollCasterLevel = 30

// progressionRows is the slot table named table, as its rows by level (Table
// 21 or 24), the table without the Wisdom bonus spells of Table 5.
func (t *Tables) progressionRows(table string) ([]SpellProgressionRow, bool) {
	switch table {
	case TableWizardSpells, TablePriestSpells:
		c, err := t.classTables()
		if err != nil {
			return nil, false
		}
		if table == TableWizardSpells {
			return c.WizardSpells, true
		}
		return c.PriestSpells, true
	}
	return nil, false
}
