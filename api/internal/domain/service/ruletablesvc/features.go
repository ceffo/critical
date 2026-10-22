package ruletablesvc

import (
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// The rule tables the class definitions name (ADR 0040 steps 1.9 and 3): a
// spell_progression and a feature's `table` param are table names, read here,
// never a class id.
const (
	TableBardSpells     = "bard_spells"
	TableWizardSpells   = "wizard_spells"
	TablePriestSpells   = "priest_spells"
	TableWarriorAttacks = "warrior_attacks"
	TableTurningUndead  = "turning_undead"
	TableBackstab       = "backstab"
)

// SpellProgressions are the slot tables a class's spell_progression may name
// (ADR 0040 step 1.9: PHB Tables 32, 21 and 24).
var SpellProgressions = []string{TableBardSpells, TableWizardSpells, TablePriestSpells}

// featureTables is the one table each kind with a `table` param reads.
var featureTables = map[string]string{
	classes.KindMeleeAttacksPerRound: TableWarriorAttacks,
	classes.KindTurnUndead:           TableTurningUndead,
	classes.KindBackstab:             TableBackstab,
}

// CheckDefinitions is ADR 0040 step 8 on what the rule tables read of the
// class definitions: a spell_progression naming a slot table, a feature's
// table param naming the table its kind reads, a known rogue_skills set with
// an armour entry for each Table 29 column (SkillColumns, none_allowed
// included: SkillArmorFor has no fallback).
func CheckDefinitions(defs *classes.Registry) error {
	for _, c := range defs.Classes() {
		if c.SpellProgression != "" && !slices.Contains(SpellProgressions, c.SpellProgression) {
			return fmt.Errorf("class %q: spell_progression %q names no slot table", c.ID, c.SpellProgression)
		}
		for _, f := range c.Features {
			if want, ok := featureTables[f.Kind]; ok && f.Table != want {
				return fmt.Errorf("class %q: feature %s: table %q, want %q", c.ID, f.Kind, f.Table, want)
			}
			if f.RogueSkills != nil {
				if err := CheckSkillSet(f.RogueSkills.Set); err != nil {
					return fmt.Errorf("class %q: %w", c.ID, err)
				}
				for _, column := range SkillColumns {
					if _, ok := f.RogueSkills.SkillArmourFor(column); !ok {
						return fmt.Errorf("class %q: rogue_skills: no armour entry for the %s column", c.ID, column)
					}
				}
			}
		}
	}
	return nil
}

// SpellsPerDay is ADR 0030 step 8 read through the class definition (ADR
// 0040 step 1.9): the slot table its spell_progression names at level (Table
// 32, 21 or 24), Table 24 with the Wisdom bonus spells at wis when the class
// has wisdom_bonus_spells at level (step 13). A class without a progression
// has no slots (an empty map).
func (t *Tables) SpellsPerDay(def classes.Class, level, wis int) (map[int]int, error) {
	switch def.SpellProgression {
	case "":
		return map[int]int{}, nil
	case TableBardSpells:
		return t.BardSpellsPerDay(level)
	case TableWizardSpells:
		return t.WizardSpellsPerDay(level)
	case TablePriestSpells:
		return t.priestSpells(level, wis, def.Has(classes.KindWisdomBonusSpells, level))
	}
	return nil, fmt.Errorf("spell slots: %s: unknown spell progression %q", def.ID, def.SpellProgression)
}
