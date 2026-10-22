package charactersvc

import (
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// Rules is the driven port over the class tables the derivation reads
// (ruletablesvc.Tables implements it). Every method errs on a value outside
// its table.
type Rules interface {
	// Definitions are the catalogue's class, group and race definitions
	// (ADR 0040 step 5): the creatable classes (step 8) and the races.
	Definitions() (*classes.Registry, error)
	// Class is a class's definition (ADR 0040 step 1), an error for a class
	// the catalogue does not define.
	Class(class string) (classes.Class, error)
	// ClassGroup is the class's group (ADR 0040 step 1.2).
	ClassGroup(class string) (string, error)
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
	// SpellsPerDay is step 8: the slot table the class's spell_progression
	// names at level (ADR 0040 step 1.9), with the Wisdom bonus spells for a
	// class with wisdom_bonus_spells (step 13).
	SpellsPerDay(def classes.Class, level, wis int) (map[int]int, error)
	// HitDiceFor is the group's hit die, name level and fixed gain after it
	// (step 7).
	HitDiceFor(group string) (ruletablesvc.HitDiceRow, error)
	// RaceAdjustmentsFor is PHB Table 8 for a race (the creation form's
	// race cards, ADR 0034 step 3.2).
	RaceAdjustmentsFor(race string) (map[string]int, error)
	// RaceAllows is DMG Table 7: false for a race the class does not admit
	// (step 21, checked when a character is created).
	RaceAllows(race, class string) (bool, error)
	// LevelLimit is DMG Table 7's cell for the race in the class, false for
	// none (an unrestricted race, or no limits in the catalogue): the
	// level_limit warning (ADR 0042 step 7).
	LevelLimit(race, class string) (ruletablesvc.LevelLimitRow, bool)
	// IntelligenceFor is PHB Table 4 (step 20).
	IntelligenceFor(score int) (ruletablesvc.IntelligenceRow, error)
	// SpellFailureFor is PHB Table 5's chance of spell failure at a score
	// (step 14).
	SpellFailureFor(score int) (int, error)
	// MagicDefenceFor is PHB Table 5's magical defence adjustment (ADR 0006
	// step 9.1), for every class.
	MagicDefenceFor(wis int) (int, error)
	// SpellImmunitiesFor is PHB Table 5's spell immunities, cumulative
	// (ADR 0006 step 10.1), for every class.
	SpellImmunitiesFor(wis int) ([]string, error)
	// TurningAt is PHB Table 61 at a priest level (step 16).
	TurningAt(level int) ([]ruletablesvc.TurningTarget, error)
	// RogueSkills is PHB Tables 26 to 29 and 33 on the entered points,
	// through a class's rogue_skills params (steps 17 and 18).
	RogueSkills(rs classes.RogueSkills, race string, dex int, armor string, points map[string]int) ([]ruletablesvc.SkillPercent, error)
	// BackstabFor is PHB Table 30 at a level (step 19).
	BackstabFor(level int) (int, error)
}

// Derive computes the derived block of c (ADR 0030 steps 3 to 8, 17, 19,
// 22.1, and the pending level-ups of step 7) through its class entries (ADR
// 0042 steps 5, 7 to 9, 12 and 14). awards are the XP ledger's awards, each
// its amount (base plus bonus, ADR 0015 step 1). armour is the Table 29 column
// of the body armour c wears, "" for none (ADR 0033 step 14); only a thief's
// or a bard's skills read it. Two levels meet in each entry (step 7): the
// target level is step 3, the class's experience table at the entry's XP (its
// starting XP plus its share of the awards, ADR 0042 step 5), never stored;
// the realized level is the stored one, which only the level-up step moves.
// Every benefit of a level (THAC0, saves, spell slots) follows the realized
// level; the target only drives the pending level-ups and the XP progress.
// Nothing derived is read from c.Sheet. A single-class character is the
// one-entry case and derives as before.
func Derive(c *Character, awards []int, armour string, rules Rules) (Derived, error) {
	entries, err := deriveEntries(c, awards, rules)
	if err != nil {
		return Derived{}, err
	}
	d := Derived{Classes: make([]EntryDerived, len(entries))}
	for i := range entries {
		d.Classes[i] = entries[i].out
	}
	// The figures that are one per entry are the first active entry's here
	// (the mirror of Character.Class): the XP progress, the group of a
	// by-group effect.
	primary := firstActive(entries)
	d.XPTotal, d.Group = primary.out.XP, primary.out.Group
	d.TargetLevel, d.RealizedLevel, d.NextLevelXP = primary.out.TargetLevel, primary.out.RealizedLevel, primary.out.NextLevelXP
	// ADR 0042 steps 8 and 9: the best (lowest) THAC0 and the best save per
	// category among the active entries, each at its realized level.
	if d.THAC0, d.Saves, err = bestCore(entries, rules); err != nil {
		return Derived{}, err
	}
	if d.RaceSaveBonus, err = raceSaveBonus(c, rules); err != nil {
		return Derived{}, err
	}
	// ADR 0042 step 14: each spellcasting entry has its own slots and caster
	// level (EntryDerived); the character's slots are the first active
	// spellcasting entry's (Derived.CasterLevel() is its level), the first
	// active entry's when none casts.
	d.SpellSlots = primary.out.SpellSlots
	for _, e := range entries {
		if e.entry.State == classes.EntryActive && e.out.Casts {
			d.SpellSlots = e.out.SpellSlots
			break
		}
	}
	// ADR 0006 step 9: Table 5's magical defence adjustment at the sheet's
	// Wisdom, an ability rule every class and race gets (step 9.2).
	if d.MagicDefence, err = rules.MagicDefenceFor(c.Wis); err != nil {
		return Derived{}, fmt.Errorf("derive magic defence: %w", err)
	}
	// ADR 0006 step 10.1: the spells Table 5 makes the sheet's Wisdom immune
	// to, for every class and race like the adjustment.
	if d.SpellImmunities, err = rules.SpellImmunitiesFor(c.Wis); err != nil {
		return Derived{}, fmt.Errorf("derive spell immunities: %w", err)
	}
	if err := d.setFeatures(c, entries, armour, rules); err != nil {
		return Derived{}, err
	}
	// Step 7, per entry (ADR 0042 step 7): every active entry's pending
	// levels, each lowest first, in entry order.
	d.LevelUp = LevelUp{Pending: []PendingLevel{}}
	for _, e := range entries {
		d.LevelUp.Pending = append(d.LevelUp.Pending, e.out.LevelUp.Pending...)
		d.LevelUp.RealizedAboveTarget = d.LevelUp.RealizedAboveTarget || e.out.LevelUp.RealizedAboveTarget
	}
	return d, nil
}

// setFeatures is ADR 0040 steps 3 and 5 through the entries (ADR 0042 step
// 12): each mechanic below is a feature kind an entry's class grants at the
// entry's level, read with its params and at that level. A figure that is one
// per character takes the first entry granting it.
func (d *Derived) setFeatures(c *Character, entries []entryCalc, armour string, rules Rules) error {
	// Each entry at the level it was derived at (step 7.5: a character
	// being created has no realized level yet, its target's).
	es := make([]classes.Entry, len(entries))
	for i, e := range entries {
		es[i] = e.entry
		if e.entry.State == classes.EntryActive {
			es[i].RealizedLevel = e.out.RealizedLevel
		}
	}
	def := func(id string) (classes.Class, error) {
		for _, e := range entries {
			if e.def.ID == id {
				return e.def, nil
			}
		}
		return rules.Class(id)
	}
	for _, kind := range classes.FeatureKinds {
		gs, err := classes.Grants(es, def, kind)
		if err != nil {
			return fmt.Errorf("derive %s: %w", kind, err)
		}
		for _, g := range gs {
			d.Features = append(d.Features, FeatureGrant{Kind: kind, Class: g.Class.ID, Level: g.Level})
		}
	}
	grant := func(kind string) (classes.Grant, bool) {
		gs, _ := classes.Grants(es, def, kind) // resolved above
		if len(gs) == 0 {
			return classes.Grant{}, false
		}
		return gs[0], true
	}
	var err error
	_, d.ExceptionalStrength = grant(classes.KindExceptionalStrength)
	if err := d.setSpellFailure(c, es, def, rules); err != nil {
		return err
	}
	if g, ok := grant(classes.KindTurnUndead); ok {
		if d.Turning, err = rules.TurningAt(g.Level); err != nil {
			return fmt.Errorf("derive %s turning: %w", g.Class.ID, err)
		}
	}
	if g, ok := grant(classes.KindRogueSkills); ok {
		// Steps 17 and 18: the sheet's Dexterity, the entered points and
		// the worn body armour's Table 29 column (ADR 0033 step 14: none
		// worn is the none column) for each skill set the class grants (ADR
		// 0040 step 3.4); the budget at the entry's level. The figures are
		// the skills module's (ADR 0037 step 17).
		d.SkillColumn = armour
		if d.SkillColumn == "" {
			d.SkillColumn = ruletablesvc.ColumnNone
		}
		if err := d.SetSkillFigures(rules, g.Class, c, c.SkillPoints.Resolve(g.Class)); err != nil {
			return err
		}
	}
	if g, ok := grant(classes.KindBackstab); ok {
		if d.Backstab, err = rules.BackstabFor(g.Level); err != nil {
			return fmt.Errorf("derive %s backstab: %w", g.Class.ID, err)
		}
	}
	_, d.Specializes = grant(classes.KindWeaponSpecialization)
	if g, ok := grant(classes.KindSpellbook); ok {
		if d.Spellbook, err = spellbookRules(c, g, rules); err != nil {
			return err
		}
	}
	return nil
}

// entryCalc is one class entry while it is derived: the entry, its class
// definition and its derived block.
type entryCalc struct {
	entry classes.Entry
	def   classes.Class
	out   EntryDerived
}

// firstActive is the first active entry (classes.ValidateEntries refused a
// character with none; Entries gives a character built from the mirror one).
func firstActive(entries []entryCalc) entryCalc {
	for _, e := range entries {
		if e.entry.State == classes.EntryActive {
			return e
		}
	}
	return entries[0]
}

// deriveEntries is each entry's block: its XP (ADR 0042 step 5), target and
// realized level (ADR 0030 step 3, ADR 0042 step 7), next threshold, slots
// (ADR 0042 step 14) and pending levels (step 7).
func deriveEntries(c *Character, awards []int, rules Rules) ([]entryCalc, error) {
	entries := c.Entries()
	shares := len(classes.Active(entries))
	out := make([]entryCalc, 0, len(entries))
	for _, e := range entries {
		def, err := rules.Class(e.Class)
		if err != nil {
			return nil, fmt.Errorf("derive: %w", err)
		}
		ed := EntryDerived{
			Class: e.Class, Group: def.Group, State: e.State, XP: EntryXP(e, awards, shares),
			Casts: def.SpellProgression != "", NoArmour: def.Equipment.NoArmour(),
		}
		if e.State == classes.EntryFrozen {
			// A dual-class's old class: no XP, no target, nothing pending
			// (ADR 0042 steps 5.2 and 7).
			ed.RealizedLevel = e.Level()
			ed.LevelUp = LevelUp{Pending: []PendingLevel{}}
		} else if err := ed.setLevels(e, shares, rules); err != nil {
			return nil, err
		}
		// Step 8 per entry: the slot table the class's spell_progression
		// names (ADR 0040 step 1.9), never a class id.
		if ed.SpellSlots, err = rules.SpellsPerDay(def, ed.RealizedLevel, c.Wis); err != nil {
			return nil, fmt.Errorf("derive %s spell slots: %w", e.Class, err)
		}
		out = append(out, entryCalc{entry: e, def: def, out: ed})
	}
	return out, nil
}

// setLevels sets an active entry's target and realized levels, its next
// threshold and its pending levels; shares is the number of active entries
// (the hit point divisor, ADR 0042 step 10.2).
func (ed *EntryDerived) setLevels(e classes.Entry, shares int, rules Rules) error {
	target, err := rules.LevelFor(e.Class, ed.XP)
	if err != nil {
		return fmt.Errorf("derive %s level: %w", e.Class, err)
	}
	// Step 7.5: no realized level yet (a character being created) is the
	// target level, the one the entered max HP is for.
	realized := e.RealizedLevel
	if realized == 0 {
		realized = target
	}
	ed.TargetLevel, ed.RealizedLevel = target, realized
	if target < ruletablesvc.MaxLevel {
		next, err := rules.XPThreshold(e.Class, target+1)
		if err != nil {
			return fmt.Errorf("derive %s: %w", e.Class, err)
		}
		ed.NextLevelXP = &next
	}
	ed.LevelUp, err = levelUp(e.Class, ed.Group, realized, target, shares, rules)
	return err
}

// EntryXP is ADR 0042 step 5: an entry's XP is its starting XP plus its share
// of each award. A multi-class character's active entries each take
// floor(amount / shares) of every award, the remainder lost (Q3), even an
// entry over its race's level limit (step 5.1); shares is the number of
// active entries. A frozen entry's XP never changes (step 5.2). One active
// entry takes every award whole (ADR 0015 step 1).
func EntryXP(e classes.Entry, awards []int, shares int) int {
	xp := e.StartingXP
	if e.State != classes.EntryActive || shares < 1 {
		return xp
	}
	for _, a := range awards {
		xp += a / shares
	}
	return xp
}

// bestCore is ADR 0042 steps 8 and 9 (amending ADR 0030 steps 5 and 6): the
// lowest Table 53 THAC0 and, category by category, the lowest Table 60
// target among the active entries, each at its realized level (Q6: two
// classes' rows may mix).
func bestCore(entries []entryCalc, rules Rules) (int, Saves, error) {
	var best *core
	for _, e := range entries {
		if e.entry.State != classes.EntryActive {
			continue
		}
		k, err := coreAt(e.entry.Class, e.out.Group, e.out.RealizedLevel, rules)
		if err != nil {
			return 0, Saves{}, err
		}
		if best == nil {
			best = &k
			continue
		}
		best.thac0 = min(best.thac0, k.thac0)
		best.saves = Saves{
			ParaPoison: min(best.saves.ParaPoison, k.saves.ParaPoison), RodStaffWand: min(best.saves.RodStaffWand, k.saves.RodStaffWand),
			PetriPoly: min(best.saves.PetriPoly, k.saves.PetriPoly), Breath: min(best.saves.Breath, k.saves.Breath),
			Spell: min(best.saves.Spell, k.saves.Spell),
		}
	}
	if best == nil {
		return 0, Saves{}, fmt.Errorf("derive: no active class")
	}
	return best.thac0, best.saves, nil
}

// spellbookRules is step 20.1 for the grant of the spellbook feature (ADR
// 0040 step 3): its max_spell_level and PHB Table 4 at the sheet's
// Intelligence.
func spellbookRules(c *Character, g classes.Grant, rules Rules) (*SpellbookRules, error) {
	classMax := g.Feature.Spellbook.MaxSpellLevel
	row, err := rules.IntelligenceFor(c.Int)
	if err != nil {
		return nil, fmt.Errorf("derive %s spellbook: %w", g.Class.ID, err)
	}
	return &SpellbookRules{
		ClassMaxSpellLevel: classMax, IntMaxSpellLevel: row.MaxSpellLevel, LearnChance: row.LearnChance,
		MaxSpellsPerLevel: row.MaxSpells, NoMaximum: row.NoMaximum,
	}, nil
}

// levelUp is step 7 for one entry of class (ADR 0042 step 7): a level-up is
// pending for each level above the realized one up to the target, rolled
// lowest first; each is the group's hit die up to its name level (step 7.1),
// the fixed gain above it (step 7.2), divided by shares, the character's
// number of classes, when it is rolled (ADR 0042 step 10). A target under the
// realized level (an XP correction, step 7.4) removes nothing: it is flagged.
func levelUp(class, group string, realized, target, shares int, rules Rules) (LevelUp, error) {
	l := LevelUp{Pending: []PendingLevel{}, RealizedAboveTarget: realized > target}
	if realized >= target {
		return l, nil
	}
	hd, err := rules.HitDiceFor(group)
	if err != nil {
		return LevelUp{}, fmt.Errorf("derive %s level-up: %w", class, err)
	}
	// Step 7.5: level 1's hit points are entered, never rolled.
	for n := max(realized+1, 2); n <= target; n++ {
		p := PendingLevel{Class: class, Level: n, Classes: shares}
		if n <= hd.NameLevel {
			p.Die = hd.Die
		} else {
			p.FixedGain = hd.GainAfter
		}
		l.Pending = append(l.Pending, p)
	}
	return l, nil
}

// core are the values steps 5 and 6 derive from the class at a level.
type core struct {
	thac0 int
	saves Saves
}

func coreAt(class, group string, level int, rules Rules) (core, error) {
	// Step 5: Table 53 for the group at that level.
	thac0, err := rules.THAC0For(group, level)
	if err != nil {
		return core{}, fmt.Errorf("derive %s THAC0: %w", class, err)
	}
	// Step 6: the Table 60 row of the group's level band.
	row, err := rules.SavesFor(group, level)
	if err != nil {
		return core{}, fmt.Errorf("derive %s saves: %w", class, err)
	}
	return core{thac0: thac0, saves: Saves{
		ParaPoison: row.ParaPoison, RodStaffWand: row.RodStaffWand, PetriPoly: row.PetriPoly,
		Breath: row.Breath, Spell: row.Spell,
	}}, nil
}

// raceSaveBonus is step 22.1 through the race's con_save_bonus trait (ADR
// 0040 step 4): the race adds PHB Table 9's bonus (from the sheet's
// Constitution) to the saves of the trait's categories. A race without the
// trait (or a race the catalogue does not define) adds nothing. The trait's
// poison flag waits for the save request's "poison" flag (step 22.1,
// proposed): the Paralyze/Poison category also covers paralysis and death
// magic, so the data does not list it among the categories.
func raceSaveBonus(c *Character, rules Rules) (Saves, error) {
	defs, err := rules.Definitions()
	if err != nil {
		return Saves{}, fmt.Errorf("derive %s save bonus: %w", c.Race, err)
	}
	race, ok := defs.Race(c.Race)
	if !ok {
		return Saves{}, nil
	}
	trait, ok := race.Trait(classes.TraitConSaveBonus)
	if !ok {
		return Saves{}, nil
	}
	b, err := rules.ConSaveBonusFor(c.Con)
	if err != nil {
		return Saves{}, fmt.Errorf("derive %s save bonus: %w", c.Race, err)
	}
	var out Saves
	for _, cat := range trait.Categories {
		if p := saveField(&out, cat); p != nil {
			*p = b
		}
	}
	return out, nil
}

// saveField is the field of s for a save category code (classes.SaveCategories),
// nil for a code that is none.
func saveField(s *Saves, cat string) *int {
	switch cat {
	case classes.SaveCategories[0]:
		return &s.ParaPoison
	case classes.SaveCategories[1]:
		return &s.RodStaffWand
	case classes.SaveCategories[2]:
		return &s.PetriPoly
	case classes.SaveCategories[3]:
		return &s.Breath
	case classes.SaveCategories[4]:
		return &s.Spell
	}
	return nil
}

// SetSkillFigures sets d's skill sets (percentages, armour, budget and points of each) from
// points entered on c, at the level of the entry granting the skills (ADR 0042 step 12; d's
// Features must be set, else d's realized level) and worn body armour's Table 29 column
// (SkillColumn must be set): the one place they are filled, for the derivation and for a draft of
// the points (ADR 0030 steps 17 and 18; the figures are the skills module's, ADR 0037 step 17).
func (d *Derived) SetSkillFigures(t skills.SkillTables, def classes.Class, c *Character, points skills.SetPoints) error {
	sets, err := skills.SkillSetsOf(t, skills.PointsSheet{
		Class: def, Race: c.Race, Dex: c.Dex, Level: d.FeatureLevel(classes.KindRogueSkills), ArmorColumn: d.SkillColumn, Points: points,
		MultiClass: len(c.Entries()) > 1, // ADR 0042 step 16.3: a dual-class's frozen entry counts
	})
	if err != nil {
		return fmt.Errorf("skill figures: %w", err)
	}
	d.SkillSets = sets
	return nil
}

// setSpellFailure is ADR 0030 step 14 through the entries (ADR 0042 steps 12
// and 14): each entry whose class grants spell_failure has the chance of the
// feature's ability score (EntryDerived.SpellFailure), which only the spells
// of that entry's slots roll; d.SpellFailure, one per character on the sheet,
// is the first granting entry's.
func (d *Derived) setSpellFailure(c *Character, es []classes.Entry, def func(string) (classes.Class, error), rules Rules) error {
	gs, err := classes.Grants(es, def, classes.KindSpellFailure)
	if err != nil {
		return fmt.Errorf("derive spell failure: %w", err)
	}
	for i, g := range gs {
		score, _ := c.Score(Ability(g.Feature.Ability)) // classes.New checked the ability code
		chance, err := rules.SpellFailureFor(score)
		if err != nil {
			return fmt.Errorf("derive %s spell failure: %w", g.Class.ID, err)
		}
		if i == 0 {
			d.SpellFailure = chance
		}
		for j := range d.Classes {
			if d.Classes[j].Class == g.Class.ID {
				d.Classes[j].SpellFailure = chance
			}
		}
	}
	return nil
}
