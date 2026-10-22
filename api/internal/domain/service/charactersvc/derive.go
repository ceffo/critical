package charactersvc

import (
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
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
	// RaceAdjustmentsFor is PHB Table 8 for a race (the creation form's
	// race cards, ADR 0034 step 3.2).
	RaceAdjustmentsFor(race string) (map[string]int, error)
	// RaceAllows is DMG Table 7: false for a race the class does not admit
	// (step 21, checked when a character is created).
	RaceAllows(race, class string) (bool, error)
	// IntelligenceFor is PHB Table 4 (step 20).
	IntelligenceFor(score int) (ruletablesvc.IntelligenceRow, error)
	// PriestSpellsPerDay is PHB Table 24 with the Wisdom bonus spells
	// (step 13).
	PriestSpellsPerDay(level, wis int) (map[int]int, error)
	// SpellFailureFor is PHB Table 5's chance of spell failure (step 14).
	SpellFailureFor(class string, wis int) (int, error)
	// MagicDefenceFor is PHB Table 5's magical defence adjustment (ADR 0006
	// step 9.1), for every class.
	MagicDefenceFor(wis int) (int, error)
	// TurningAt is PHB Table 61 at a priest level (step 16).
	TurningAt(level int) ([]ruletablesvc.TurningTarget, error)
	// ThiefSkills is PHB Tables 26 to 29 on the entered points (step 17).
	ThiefSkills(race string, dex int, armor string, points map[string]int) ([]ruletablesvc.SkillPercent, error)
	// BardSkills is PHB Tables 27 to 29 and 33 on the entered points
	// (step 18).
	BardSkills(race string, dex int, armor string, points map[string]int) ([]ruletablesvc.SkillPercent, error)
	// BackstabFor is PHB Table 30 at a thief's level (step 19).
	BackstabFor(level int) (int, error)
}

// conSaveRaces are the races that add their PHB Table 9 bonus to the saves
// against Rod/Staff/Wand and Spells (ADR 0030 step 22.1). Their bonus against
// poison (dwarves and halflings) waits for the save request's "poison" flag
// (step 22.1, proposed): the Paralyze/Poison category also covers paralysis
// and death magic.
var conSaveRaces = []string{"dwarf", "gnome", "halfling"}

// Derive computes the derived block of c (ADR 0030 steps 3 to 8, 17, 19,
// 22.1, and the pending level-ups of step 7). armour is the Table 29 column of
// the body armour c wears, "" for none (ADR 0033 step 14); only a thief's or a
// bard's skills read it. Two levels meet here (step 7):
// the target level is step 3, the class's experience table at the XP total
// (the starting XP, c.XPTotal, plus ledgerXP, what the XP ledger's entries
// add; ADR 0015 step 1), never stored; the realized level is the stored one
// (c.RealizedLevel), which only the level-up step moves. Every benefit of a
// level (THAC0, saves, spell slots) follows the realized level; the target
// only drives the pending level-ups and the XP progress. Nothing derived is
// read from c.Sheet.
func Derive(c *Character, ledgerXP int, armour string, rules Rules) (Derived, error) {
	d := Derived{XPTotal: c.XPTotal + ledgerXP}
	if _, err := ruletablesvc.ClassGroup(c.Class); err != nil {
		return Derived{}, fmt.Errorf("derive: %w", err)
	}
	target, err := levelAt(c, d.XPTotal, rules)
	if err != nil {
		return Derived{}, err
	}
	// Step 7.5: no realized level yet (a character being created) is the
	// target level, the one the entered max HP is for.
	realized := c.RealizedLevel
	if realized == 0 {
		realized = target
	}
	d.TargetLevel, d.RealizedLevel = target, realized
	core, err := coreAt(c, realized, rules)
	if err != nil {
		return Derived{}, err
	}
	d.THAC0, d.Saves = core.thac0, core.saves
	if target < ruletablesvc.MaxLevel {
		next, err := rules.XPThreshold(c.Class, target+1)
		if err != nil {
			return Derived{}, fmt.Errorf("derive %s: %w", c.Class, err)
		}
		d.NextLevelXP = &next
	}
	if d.RaceSaveBonus, err = raceSaveBonus(c, rules); err != nil {
		return Derived{}, err
	}
	if d.SpellSlots, err = spellSlots(c, realized, rules); err != nil {
		return Derived{}, err
	}
	// ADR 0006 step 9: Table 5's magical defence adjustment at the sheet's
	// Wisdom, an ability rule every class and race gets (step 9.2).
	if d.MagicDefence, err = rules.MagicDefenceFor(c.Wis); err != nil {
		return Derived{}, fmt.Errorf("derive magic defence: %w", err)
	}
	if ruletablesvc.UsesWisdomSpells(c.Class) {
		if d.SpellFailure, err = rules.SpellFailureFor(c.Class, c.Wis); err != nil {
			return Derived{}, fmt.Errorf("derive %s spell failure: %w", c.Class, err)
		}
	}
	if ruletablesvc.TurnsUndead(c.Class) {
		if d.Turning, err = rules.TurningAt(realized); err != nil {
			return Derived{}, fmt.Errorf("derive %s turning: %w", c.Class, err)
		}
	}
	if ruletablesvc.HasSkillPoints(c.Class) {
		// Steps 17 and 18: the sheet's Dexterity, the entered points and
		// the worn body armour's Table 29 column (ADR 0033 step 14: none
		// worn is the none column); the budget at the realized level. The
		// figures are the skills module's (ADR 0037 step 17).
		d.SkillArmor = ruletablesvc.SkillArmorFor(c.Class, armour)
		if err := d.SetSkillFigures(rules, c, c.SkillPoints); err != nil {
			return Derived{}, err
		}
	}
	if ruletablesvc.Backstabs(c.Class) {
		if d.Backstab, err = rules.BackstabFor(realized); err != nil {
			return Derived{}, fmt.Errorf("derive %s backstab: %w", c.Class, err)
		}
	}
	if d.LevelUp, err = levelUp(c, realized, target, rules); err != nil {
		return Derived{}, err
	}
	d.Specializes = ruletablesvc.Specializes(c.Class)
	if d.Spellbook, err = spellbookRules(c, rules); err != nil {
		return Derived{}, err
	}
	return d, nil
}

// spellbookRules is step 20.1 for a class with a spellbook (nil for the
// others): the class's highest spell level and PHB Table 4 at the sheet's
// Intelligence.
func spellbookRules(c *Character, rules Rules) (*SpellbookRules, error) {
	classMax, ok := ruletablesvc.SpellbookMaxLevel(c.Class)
	if !ok {
		return nil, nil
	}
	row, err := rules.IntelligenceFor(c.Int)
	if err != nil {
		return nil, fmt.Errorf("derive %s spellbook: %w", c.Class, err)
	}
	return &SpellbookRules{
		ClassMaxSpellLevel: classMax, IntMaxSpellLevel: row.MaxSpellLevel, LearnChance: row.LearnChance,
		MaxSpellsPerLevel: row.MaxSpells, NoMaximum: row.NoMaximum,
	}, nil
}

// levelUp is step 7: a level-up is pending for each level above the
// realized one up to the target, rolled lowest first; each is the group's
// hit die up to its name level (step 7.1), the fixed gain above it (step
// 7.2). A target under the realized level (an XP correction, step 7.4)
// removes nothing: it is flagged.
func levelUp(c *Character, realized, target int, rules Rules) (LevelUp, error) {
	l := LevelUp{Pending: []PendingLevel{}, RealizedAboveTarget: realized > target}
	if realized >= target {
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
	for n := max(realized+1, 2); n <= target; n++ {
		if n <= hd.NameLevel {
			l.Pending = append(l.Pending, PendingLevel{Level: n, Die: hd.Die})
		} else {
			l.Pending = append(l.Pending, PendingLevel{Level: n, FixedGain: hd.GainAfter})
		}
	}
	return l, nil
}

// core are the values steps 5 and 6 derive from the class at a level.
type core struct {
	thac0 int
	saves Saves
}

// levelAt is step 3: the class's experience table at xp.
func levelAt(c *Character, xp int, rules Rules) (int, error) {
	level, err := rules.LevelFor(c.Class, xp)
	if err != nil {
		return 0, fmt.Errorf("derive %s level: %w", c.Class, err)
	}
	return level, nil
}

func coreAt(c *Character, level int, rules Rules) (core, error) {
	group, err := ruletablesvc.ClassGroup(c.Class)
	if err != nil {
		return core{}, fmt.Errorf("derive: %w", err)
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
	return core{thac0: thac0, saves: Saves{
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
// level, a mage's Table 21, a cleric's Table 24 plus the Wisdom bonus spells
// at the sheet's Wisdom (step 13); fighters and thieves have none.
func spellSlots(c *Character, level int, rules Rules) (map[int]int, error) {
	class := c.Class
	var table func(int) (map[int]int, error)
	switch class {
	case "bard":
		table = rules.BardSpellsPerDay
	case "mage":
		table = rules.WizardSpellsPerDay
	case "cleric":
		table = func(level int) (map[int]int, error) { return rules.PriestSpellsPerDay(level, c.Wis) }
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

// SetSkillFigures sets d's skill percentages, budget and points from points
// entered on c, at d's realized level and worn skill armour (SkillArmor must
// be set): the one place those four fields are filled, for the derivation and
// for a draft of the points (ADR 0030 steps 17 and 18; the figures are the
// skills module's, ADR 0037 step 17).
func (d *Derived) SetSkillFigures(t skills.SkillTables, c *Character, points map[string]int) error {
	f, err := skills.PointsFiguresOf(t, skills.PointsSheet{
		Class: c.Class, Race: c.Race, Dex: c.Dex, Level: d.RealizedLevel, Armor: d.SkillArmor, Points: points,
	})
	if err != nil {
		return fmt.Errorf("skill figures: %w", err)
	}
	d.ThiefSkills, d.BardSkills, d.SkillBudget, d.SkillPoints = f.Thief, f.Bard, f.Budget, f.Points
	return nil
}
