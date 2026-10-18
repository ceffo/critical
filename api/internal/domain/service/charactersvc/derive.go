package charactersvc

import (
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// Rules is the driven port over the class tables the derivation reads
// (ruletablesvc.Tables implements it). Every method errs on a value outside
// its table.
type Rules interface {
	// LevelFor is the class's level at xp (ADR 0030 step 3).
	LevelFor(class string, xp int) (int, error)
	// XPThreshold is the XP that reaches level in the class's table.
	XPThreshold(class string, level int) (int, error)
	// THAC0For is PHB Table 53 for the group at level (step 5).
	THAC0For(group string, level int) (int, error)
	// SavesFor is the PHB Table 60 row of the group at level (step 6).
	SavesFor(group string, level int) (ruletablesvc.SavingThrowRow, error)
	// ConSaveBonusFor is PHB Table 9 (step 22.1).
	ConSaveBonusFor(con int) (int, error)
	// BardSpellsPerDay is PHB Table 32 by spell level (step 8).
	BardSpellsPerDay(level int) (map[int]int, error)
	// WizardSpellsPerDay is PHB Table 21 by spell level (step 8).
	WizardSpellsPerDay(level int) (map[int]int, error)
	// HitDiceFor is the group's hit die, name level and fixed gain after it
	// (step 7).
	HitDiceFor(group string) (ruletablesvc.HitDiceRow, error)
	// RaceAllows is DMG Table 7: false for a race the class does not admit
	// (step 21, checked when a character is created).
	RaceAllows(race, class string) (bool, error)
}

// conSaveRaces are the races that add their PHB Table 9 bonus to the saves
// against Rod/Staff/Wand and Spells (ADR 0030 step 22.1). Their bonus against
// poison (dwarves and halflings) waits for the save request's "poison" flag
// (step 22.1, proposed): the Paralyze/Poison category also covers paralysis
// and death magic.
var conSaveRaces = []string{"dwarf", "gnome", "halfling"}

// Derive computes the derived block of c (ADR 0030 steps 3 to 8, 22.1 and
// 24, and the pending level-ups of step 7) from its class, race, Constitution and XP total: the starting XP
// (c.XPTotal) plus ledgerXP, what the XP ledger's entries add (ADR 0015 step
// 1). Nothing derived is read from c.Sheet but the comparison of step 24.
func Derive(c *Character, ledgerXP int, rules Rules) (Derived, error) {
	d := Derived{XPTotal: c.XPTotal + ledgerXP}
	core, err := deriveCore(c, d.XPTotal, rules)
	if err != nil {
		return Derived{}, err
	}
	d.Level, d.THAC0, d.Saves = core.level, core.thac0, core.saves
	if d.Level < ruletablesvc.MaxLevel {
		next, err := rules.XPThreshold(c.Class, d.Level+1)
		if err != nil {
			return Derived{}, fmt.Errorf("derive %s: %w", c.Class, err)
		}
		d.NextLevelXP = &next
	}
	if d.RaceSaveBonus, err = raceSaveBonus(c, rules); err != nil {
		return Derived{}, err
	}
	if d.SpellSlots, err = spellSlots(c.Class, d.Level, rules); err != nil {
		return Derived{}, err
	}
	// Step 24: the sheet's figures against the rules at the sheet's own XP
	// (the starting XP, without the ledger): the awards since the import
	// raise the level, they are no difference with the sheet.
	atImport := core
	if ledgerXP != 0 {
		if atImport, err = deriveCore(c, c.XPTotal, rules); err != nil {
			return Derived{}, err
		}
	}
	d.SheetDifferences = sheetDifferences(c.Sheet, atImport)
	if d.LevelUp, err = levelUp(c, d.Level, rules); err != nil {
		return Derived{}, err
	}
	return d, nil
}

// levelUp is step 7: a level-up is pending for each level above the one the
// max HP was last set for, up to the level of step 3, rolled lowest first;
// each is the group's hit die up to its name level (step 7.1), the fixed
// gain above it (step 7.2). A level under the max HP's (an XP correction,
// step 7.4) removes nothing: it is flagged.
func levelUp(c *Character, level int, rules Rules) (LevelUp, error) {
	// Step 7.5: no level set for the max HP (a character being created) is
	// the level of step 3, the one the entered max HP is for. A stored
	// character never gets here with 0: the DAL reads a NULL hp_level as its
	// sheet level (the step 7.5 amendment in ADR 0030).
	hpLevel := c.HPLevel
	if hpLevel == 0 {
		hpLevel = level
	}
	l := LevelUp{HPLevel: hpLevel, Pending: []PendingLevel{}, HPAboveLevel: hpLevel > level}
	if hpLevel >= level {
		return l, nil
	}
	group, err := ruletablesvc.ClassGroup(c.Class)
	if err != nil {
		return LevelUp{}, fmt.Errorf("derive level-up: %w", err)
	}
	hd, err := rules.HitDiceFor(group)
	if err != nil {
		return LevelUp{}, fmt.Errorf("derive %s level-up: %w", c.Class, err)
	}
	// Step 7.5: level 1's hit points are entered, never rolled.
	for n := max(hpLevel+1, 2); n <= level; n++ {
		if n <= hd.NameLevel {
			l.Pending = append(l.Pending, PendingLevel{Level: n, Die: hd.Die})
		} else {
			l.Pending = append(l.Pending, PendingLevel{Level: n, FixedGain: hd.GainAfter})
		}
	}
	return l, nil
}

// core are the values steps 3, 5 and 6 derive from the class and the XP.
type core struct {
	level, thac0 int
	saves        Saves
}

func deriveCore(c *Character, xp int, rules Rules) (core, error) {
	group, err := ruletablesvc.ClassGroup(c.Class)
	if err != nil {
		return core{}, fmt.Errorf("derive: %w", err)
	}
	// Step 3: the level from the class's experience table.
	level, err := rules.LevelFor(c.Class, xp)
	if err != nil {
		return core{}, fmt.Errorf("derive %s level: %w", c.Class, err)
	}
	// Step 5: Table 53 for the group at that level.
	thac0, err := rules.THAC0For(group, level)
	if err != nil {
		return core{}, fmt.Errorf("derive %s THAC0: %w", c.Class, err)
	}
	// Step 6: the Table 60 row of the group's level band.
	row, err := rules.SavesFor(group, level)
	if err != nil {
		return core{}, fmt.Errorf("derive %s saves: %w", c.Class, err)
	}
	return core{level: level, thac0: thac0, saves: Saves{
		ParaPoison: row.ParaPoison, RodStaffWand: row.RodStaffWand, PetriPoly: row.PetriPoly,
		Breath: row.Breath, Spell: row.Spell,
	}}, nil
}

// raceSaveBonus is step 22.1: dwarves, gnomes and halflings add their PHB
// Table 9 bonus (from the sheet's Constitution) to the saves against
// Rod/Staff/Wand and Spells. Every other race (and none) adds nothing.
func raceSaveBonus(c *Character, rules Rules) (Saves, error) {
	if !slices.Contains(conSaveRaces, c.Race) {
		return Saves{}, nil
	}
	b, err := rules.ConSaveBonusFor(c.Con)
	if err != nil {
		return Saves{}, fmt.Errorf("derive %s save bonus: %w", c.Race, err)
	}
	return Saves{RodStaffWand: b, Spell: b}, nil
}

// spellSlots is step 8: a bard's slots per day are PHB Table 32 at the
// level, a mage's Table 21; fighters and thieves have none. The cleric's
// (Table 24 plus the Wisdom bonus spells of step 13) comes with the cleric.
func spellSlots(class string, level int, rules Rules) (map[int]int, error) {
	var table func(int) (map[int]int, error)
	switch class {
	case "bard":
		table = rules.BardSpellsPerDay
	case "mage":
		table = rules.WizardSpellsPerDay
	case "fighter", "thief":
		return map[int]int{}, nil
	default:
		return nil, fmt.Errorf("derive spell slots: class %q is not implemented yet", class)
	}
	slots, err := table(level)
	if err != nil {
		return nil, fmt.Errorf("derive %s spell slots: %w", class, err)
	}
	return slots, nil
}

// sheetDifferences lists each figure of the sheet that differs from the
// rules' value at the sheet's XP, in the sheet's order (step 24).
func sheetDifferences(sheet Sheet, rules core) []SheetDifference {
	pairs := []struct {
		value           SheetValue
		sheet, computed int
	}{
		{SheetLevel, sheet.Level, rules.level},
		{SheetTHAC0, sheet.THAC0, rules.thac0},
		{SheetSaveParaPoison, sheet.Saves.ParaPoison, rules.saves.ParaPoison},
		{SheetSaveRodStaffWand, sheet.Saves.RodStaffWand, rules.saves.RodStaffWand},
		{SheetSavePetriPoly, sheet.Saves.PetriPoly, rules.saves.PetriPoly},
		{SheetSaveBreath, sheet.Saves.Breath, rules.saves.Breath},
		{SheetSaveSpell, sheet.Saves.Spell, rules.saves.Spell},
	}
	out := []SheetDifference{}
	for _, p := range pairs {
		if p.sheet != p.computed {
			out = append(out, SheetDifference{Value: p.value, Sheet: p.sheet, Computed: p.computed})
		}
	}
	return out
}
