package ruletablesvc

import (
	"errors"
	"fmt"
	"slices"
)

// The class and race tables of ADR 0030 step 23, carried by catalogue format
// 4 (ADR 0021 step 10). They are read and validated at every start; the
// derivations that use them (level, THAC0, saves, slots, skills...) are
// ADR 0030's steps, not this file's. Every code is language-neutral.

// MaxLevel is the last character level the tables cover (ADR 0030 step 3).
const MaxLevel = 20

// MaxScore is the highest ability score of the PHB tables.
const MaxScore = 25

// Classes are the class codes (ADR 0030 step 1).
var Classes = []string{"fighter", "mage", "cleric", "thief", "bard"}

// Groups are the class groups (ADR 0030 step 2).
var Groups = []string{"warrior", "priest", "rogue", "wizard"}

// Races are the non-human PHB races the race tables list (ADR 0030 step 21);
// humans have no requirement, adjustment or skill row.
var Races = []string{"dwarf", "elf", "gnome", "half_elf", "halfling"}

// Abilities are the six ability score codes.
var Abilities = []string{"str", "dex", "con", "int", "wis", "cha"}

// ThiefSkills are the eight thieving skills of PHB Table 26.
var ThiefSkills = []string{
	"pick_pockets", "open_locks", "find_remove_traps", "move_silently",
	"hide_in_shadows", "detect_noise", "climb_walls", "read_languages",
}

// BardSkills are the four bard skills of PHB Table 33.
var BardSkills = []string{"climb_walls", "detect_noise", "pick_pockets", "read_languages"}

// SkillArmors are the armour columns of PHB Table 29 (leather, the
// baseline, has none).
var SkillArmors = []string{"none", "elven_chain", "padded_studded_leather"}

// SpecialistWeapons are the columns of PHB Table 35.
var SpecialistWeapons = []string{
	"melee", "light_crossbow", "heavy_crossbow", "thrown_dagger", "thrown_dart", "other_missile",
}

// MissileWeaponTypes are the weapon types with a rate of fire (PHB Table 45).
var MissileWeaponTypes = []string{
	"long_bow", "short_bow", "hand_crossbow", "light_crossbow", "heavy_crossbow", "dagger", "dart", "sling",
}

// Undead are the rows of PHB Table 61.
var Undead = []string{
	"skeleton", "zombie", "ghoul", "shadow", "wight", "ghast", "wraith",
	"mummy", "spectre", "vampire", "ghost", "lich", "special",
}

// TurningOutcomes are the Table 61 cells that need no die.
var TurningOutcomes = []string{"turn", "destroy", "destroy_more", "cannot"}

// ExperienceRow is the XP that reaches Level, per class (PHB Tables 14, 20,
// 23 and 25).
type ExperienceRow struct {
	Level                              int
	Fighter, Mage, Cleric, Thief, Bard int
}

// HitDiceRow is a group's hit die, the last level that rolls it and the
// fixed gain per level after it.
type HitDiceRow struct {
	Group                     string
	Die, NameLevel, GainAfter int
}

// ConstitutionRow is PHB Table 3: hit points per die (HPWarrior for
// warriors) and the value a low die counts as (0 = none).
type ConstitutionRow struct {
	ConMin, ConMax           int
	HP, HPWarrior, LowDieMin int
}

// IntelligenceRow is PHB Table 4. NoMaximum is the book's "All" (MaxSpells
// is then 0 and means nothing).
type IntelligenceRow struct {
	IntMin, IntMax             int
	MaxSpellLevel, LearnChance int
	MaxSpells                  int
	NoMaximum                  bool
}

// WisdomRow is PHB Table 5: the bonus spell levels this score adds (not
// cumulative) and the chance of spell failure.
type WisdomRow struct {
	WisMin, WisMax int
	BonusSpells    []int
	SpellFailure   int
}

// ConSaveBonusRow is PHB Table 9.
type ConSaveBonusRow struct {
	ConMin, ConMax, Bonus int
}

// RaceRequirementRow is PHB Table 7: the range a race allows for an ability.
type RaceRequirementRow struct {
	Race, Ability string
	Min, Max      int
}

// RaceAdjustmentRow is PHB Table 8.
type RaceAdjustmentRow struct {
	Race, Ability string
	Adjustment    int
}

// THAC0Row is PHB Table 53 at Level.
type THAC0Row struct {
	Level                          int
	Warrior, Priest, Rogue, Wizard int
}

// SavingThrowRow is PHB Table 60: a group's five targets over a level band.
type SavingThrowRow struct {
	Group                                              string
	LevelMin, LevelMax                                 int
	ParaPoison, RodStaffWand, PetriPoly, Breath, Spell int
}

// AttackRateRow is PHB Table 15 (warrior melee attacks) over a level band,
// as Attacks per Rounds.
type AttackRateRow struct {
	LevelMin, LevelMax, Attacks, Rounds int
}

// SpecialistAttackRow is PHB Table 35 for one weapon over a level band.
type SpecialistAttackRow struct {
	Weapon                              string
	LevelMin, LevelMax, Attacks, Rounds int
}

// RateOfFireRow is PHB Table 45's ROF for a weapon type.
type RateOfFireRow struct {
	WeaponType      string
	Attacks, Rounds int
}

// SpellProgressionRow is PHB Table 21 or 24 at Level, PerSpellLevel[i]
// being the spells of spell level i+1.
type SpellProgressionRow struct {
	Level         int
	PerSpellLevel []int
}

// PriestSpellWisdomRow is a Table 24 note: the Wisdom a priest needs to use
// spells of SpellLevel.
type PriestSpellWisdomRow struct {
	SpellLevel, WisMin int
}

// SkillBaseRow is PHB Table 26 or 33.
type SkillBaseRow struct {
	Skill string
	Base  int
}

// SkillRaceRow is PHB Table 27.
type SkillRaceRow struct {
	Race, Skill string
	Adjustment  int
}

// SkillDexterityRow is PHB Table 28 (scores outside the rows: 0).
type SkillDexterityRow struct {
	DexMin, DexMax                                                       int
	PickPockets, OpenLocks, FindRemoveTraps, MoveSilently, HideInShadows int
}

// SkillArmorRow is PHB Table 29.
type SkillArmorRow struct {
	Armor, Skill string
	Adjustment   int
}

// BardArmorPenaltyRow is Table 29's note: the bard's extra adjustment in
// metal armour, added to the elven chain column (ADR 0030 step 1).
type BardArmorPenaltyRow struct {
	Armor      string
	Adjustment int
}

// BackstabRow is PHB Table 30 over a level band.
type BackstabRow struct {
	LevelMin, LevelMax, Multiplier int
}

// TurningCell is one Table 61 cell: Roll (success on a d20 at or above it)
// or Outcome (one of TurningOutcomes), never both.
type TurningCell struct {
	Roll    int
	Outcome string
}

// TurningRow is a Table 61 row: the undead type, its Hit Dice (HDMin 0 =
// by type only, HDMax 0 = and more) and the cell for each priest level.
type TurningRow struct {
	Undead       string
	HDMin, HDMax int
	ByLevel      [MaxLevel]TurningCell
}

// ClassTables holds the validated class and race tables. Built by
// NewClassTables only; its slices are not to be modified.
type ClassTables struct {
	Experience          []ExperienceRow
	HitDice             []HitDiceRow
	Constitution        []ConstitutionRow
	Intelligence        []IntelligenceRow
	Wisdom              []WisdomRow
	ConSaveBonus        []ConSaveBonusRow
	RaceRequirements    []RaceRequirementRow
	RaceAdjustments     []RaceAdjustmentRow
	THAC0               []THAC0Row
	SavingThrows        []SavingThrowRow
	WarriorAttacks      []AttackRateRow
	SpecialistAttacks   []SpecialistAttackRow
	RateOfFire          []RateOfFireRow
	WizardSpells        []SpellProgressionRow
	PriestSpells        []SpellProgressionRow
	PriestSpellWisdom   []PriestSpellWisdomRow
	ThiefSkillBase      []SkillBaseRow
	ThiefSkillRace      []SkillRaceRow
	ThiefSkillDexterity []SkillDexterityRow
	ThiefSkillArmor     []SkillArmorRow
	BardArmorPenalty    []BardArmorPenaltyRow
	BardSkillBase       []SkillBaseRow
	Backstab            []BackstabRow
	TurningUndead       []TurningRow
}

// MaxWizardSpellLevel and MaxPriestSpellLevel are the widths of PHB Tables
// 21 and 24.
const (
	MaxWizardSpellLevel = 9
	MaxPriestSpellLevel = 7
)

// NewClassTables validates c (the rules the catalogue schema cannot
// express) and returns it. A violation names the table: levels 1 to 20 once
// and in order, XP from 0 and strictly increasing per class, ability rows
// ascending and contiguous (1 to 25 for Constitution, Intelligence and
// Wisdom), level bands covering 1 to 20, and every code combination exactly
// once.
func NewClassTables(c ClassTables) (*ClassTables, error) {
	checks := []struct {
		table string
		err   error
	}{
		{"experience", checkExperience(c.Experience)},
		{"hit_dice", once(c.HitDice, func(r HitDiceRow) string { return r.Group }, Groups)},
		{"constitution", checkScores(c.Constitution, func(r ConstitutionRow) (int, int) { return r.ConMin, r.ConMax }, true)},
		{"intelligence", checkScores(c.Intelligence, func(r IntelligenceRow) (int, int) { return r.IntMin, r.IntMax }, true)},
		{"wisdom", checkWisdom(c.Wisdom)},
		{"con_save_bonus", checkScores(c.ConSaveBonus, func(r ConSaveBonusRow) (int, int) { return r.ConMin, r.ConMax }, false)},
		{"race_requirements", checkRaceRequirements(c.RaceRequirements)},
		{"race_adjustments", checkRaceAdjustments(c.RaceAdjustments)},
		{"thac0", levelsInOrder(c.THAC0, func(r THAC0Row) int { return r.Level })},
		{"saving_throws", checkSaves(c.SavingThrows)},
		{"warrior_attacks", checkBands(c.WarriorAttacks, func(r AttackRateRow) (int, int) { return r.LevelMin, r.LevelMax })},
		{"specialist_attacks", checkSpecialists(c.SpecialistAttacks)},
		{"rate_of_fire", once(c.RateOfFire, func(r RateOfFireRow) string { return r.WeaponType }, MissileWeaponTypes)},
		{"wizard_spells", checkProgression(c.WizardSpells, MaxWizardSpellLevel)},
		{"priest_spells", checkProgression(c.PriestSpells, MaxPriestSpellLevel)},
		{"priest_spell_wisdom", checkPriestWisdom(c.PriestSpellWisdom)},
		{"thief_skill_base", once(c.ThiefSkillBase, func(r SkillBaseRow) string { return r.Skill }, ThiefSkills)},
		{"thief_skill_race", once(c.ThiefSkillRace, func(r SkillRaceRow) string { return r.Race + " " + r.Skill }, pairs(Races, ThiefSkills))},
		{"thief_skill_dexterity", checkScores(c.ThiefSkillDexterity, func(r SkillDexterityRow) (int, int) { return r.DexMin, r.DexMax }, false)},
		{"thief_skill_armor", once(c.ThiefSkillArmor, func(r SkillArmorRow) string { return r.Armor + " " + r.Skill }, pairs(SkillArmors, ThiefSkills))},
		{"bard_armor_penalty", once(c.BardArmorPenalty, func(r BardArmorPenaltyRow) string { return r.Armor }, []string{"metal"})},
		{"bard_skill_base", once(c.BardSkillBase, func(r SkillBaseRow) string { return r.Skill }, BardSkills)},
		{"backstab", checkBands(c.Backstab, func(r BackstabRow) (int, int) { return r.LevelMin, r.LevelMax })},
		{"turning_undead", checkTurning(c.TurningUndead)},
	}
	for _, ch := range checks {
		if ch.err != nil {
			return nil, fmt.Errorf("%s table: %w", ch.table, ch.err)
		}
	}
	return &c, nil
}

// experienceColumns reads a class's column of the experience table.
var experienceColumns = map[string]func(ExperienceRow) int{
	"fighter": func(r ExperienceRow) int { return r.Fighter },
	"mage":    func(r ExperienceRow) int { return r.Mage },
	"cleric":  func(r ExperienceRow) int { return r.Cleric },
	"thief":   func(r ExperienceRow) int { return r.Thief },
	"bard":    func(r ExperienceRow) int { return r.Bard },
}

func checkExperience(rows []ExperienceRow) error {
	if err := levelsInOrder(rows, func(r ExperienceRow) int { return r.Level }); err != nil {
		return err
	}
	for _, class := range Classes {
		xp := experienceColumns[class]
		if xp(rows[0]) != 0 {
			return fmt.Errorf("%s: level 1 must start at 0 XP", class)
		}
		for i := 1; i < len(rows); i++ {
			if xp(rows[i]) <= xp(rows[i-1]) {
				return fmt.Errorf("%s level %d: %d XP is not above level %d", class, rows[i].Level, xp(rows[i]), i)
			}
		}
	}
	return nil
}

func levelsInOrder[R any](rows []R, level func(R) int) error {
	if len(rows) != MaxLevel {
		return fmt.Errorf("needs levels 1..%d, got %d rows", MaxLevel, len(rows))
	}
	for i, r := range rows {
		if level(r) != i+1 {
			return fmt.Errorf("row %d: level %d expected, got %d", i, i+1, level(r))
		}
	}
	return nil
}

// checkScores: rows ascending without overlap inside 1..MaxScore; full
// demands the rows cover 1..MaxScore contiguously.
func checkScores[R any](rows []R, rng func(R) (int, int), full bool) error {
	if len(rows) == 0 {
		return errors.New("no rows")
	}
	next := 1
	for i, r := range rows {
		lo, hi := rng(r)
		switch {
		case lo > hi:
			return fmt.Errorf("row %d: min above max", i)
		case lo < 1 || hi > MaxScore:
			return fmt.Errorf("row %d: scores %d..%d outside 1..%d", i, lo, hi, MaxScore)
		case full && lo != next:
			return fmt.Errorf("row %d: score %d expected, got %d", i, next, lo)
		case !full && lo < next:
			return fmt.Errorf("row %d: score %d overlaps the row before", i, lo)
		}
		next = hi + 1
	}
	if full && next != MaxScore+1 {
		return fmt.Errorf("must cover scores 1..%d", MaxScore)
	}
	return nil
}

// checkBands: level bands covering 1..MaxLevel contiguously, in order.
func checkBands[R any](rows []R, rng func(R) (int, int)) error {
	next := 1
	for i, r := range rows {
		lo, hi := rng(r)
		if lo > hi {
			return fmt.Errorf("row %d: min above max", i)
		}
		if lo != next {
			return fmt.Errorf("row %d: level %d expected, got %d", i, next, lo)
		}
		next = hi + 1
	}
	if next != MaxLevel+1 {
		return fmt.Errorf("must cover levels 1..%d", MaxLevel)
	}
	return nil
}

// once: every key of want exactly once, and no other key.
func once[R any](rows []R, key func(R) string, want []string) error {
	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		k := key(r)
		if !slices.Contains(want, k) {
			return fmt.Errorf("unknown %q", k)
		}
		if seen[k] {
			return fmt.Errorf("%q twice", k)
		}
		seen[k] = true
	}
	for _, k := range want {
		if !seen[k] {
			return fmt.Errorf("missing %q", k)
		}
	}
	return nil
}

func pairs(a, b []string) []string {
	out := make([]string, 0, len(a)*len(b))
	for _, x := range a {
		for _, y := range b {
			out = append(out, x+" "+y)
		}
	}
	return out
}

func checkWisdom(rows []WisdomRow) error {
	if err := checkScores(rows, func(r WisdomRow) (int, int) { return r.WisMin, r.WisMax }, true); err != nil {
		return err
	}
	for _, r := range rows {
		for _, l := range r.BonusSpells {
			if l < 1 || l > MaxPriestSpellLevel {
				return fmt.Errorf("wisdom %d: bonus spell level %d outside 1..%d", r.WisMin, l, MaxPriestSpellLevel)
			}
		}
	}
	return nil
}

func checkRaceRequirements(rows []RaceRequirementRow) error {
	if err := once(rows, func(r RaceRequirementRow) string { return r.Race + " " + r.Ability }, pairs(Races, Abilities)); err != nil {
		return err
	}
	for _, r := range rows {
		if r.Min > r.Max {
			return fmt.Errorf("%s %s: min above max", r.Race, r.Ability)
		}
	}
	return nil
}

func checkRaceAdjustments(rows []RaceAdjustmentRow) error {
	seen := map[string]bool{}
	for _, r := range rows {
		if !slices.Contains(Races, r.Race) || !slices.Contains(Abilities, r.Ability) {
			return fmt.Errorf("unknown %q %q", r.Race, r.Ability)
		}
		k := r.Race + " " + r.Ability
		if seen[k] {
			return fmt.Errorf("%q twice", k)
		}
		seen[k] = true
	}
	return nil
}

func checkSaves(rows []SavingThrowRow) error {
	for _, r := range rows {
		if !slices.Contains(Groups, r.Group) {
			return fmt.Errorf("unknown group %q", r.Group)
		}
	}
	for _, g := range Groups {
		var band []SavingThrowRow
		for _, r := range rows {
			if r.Group == g {
				band = append(band, r)
			}
		}
		if err := checkBands(band, func(r SavingThrowRow) (int, int) { return r.LevelMin, r.LevelMax }); err != nil {
			return fmt.Errorf("%s: %w", g, err)
		}
	}
	return nil
}

func checkSpecialists(rows []SpecialistAttackRow) error {
	for _, r := range rows {
		if !slices.Contains(SpecialistWeapons, r.Weapon) {
			return fmt.Errorf("unknown weapon %q", r.Weapon)
		}
	}
	for _, w := range SpecialistWeapons {
		var band []SpecialistAttackRow
		for _, r := range rows {
			if r.Weapon == w {
				band = append(band, r)
			}
		}
		if err := checkBands(band, func(r SpecialistAttackRow) (int, int) { return r.LevelMin, r.LevelMax }); err != nil {
			return fmt.Errorf("%s: %w", w, err)
		}
	}
	return nil
}

func checkProgression(rows []SpellProgressionRow, width int) error {
	if err := levelsInOrder(rows, func(r SpellProgressionRow) int { return r.Level }); err != nil {
		return err
	}
	for _, r := range rows {
		if len(r.PerSpellLevel) != width {
			return fmt.Errorf("level %d: %d spell levels, want %d", r.Level, len(r.PerSpellLevel), width)
		}
		for i, n := range r.PerSpellLevel {
			if n < 0 {
				return fmt.Errorf("level %d spell level %d: negative count", r.Level, i+1)
			}
		}
	}
	return nil
}

func checkPriestWisdom(rows []PriestSpellWisdomRow) error {
	seen := map[int]bool{}
	for _, r := range rows {
		if r.SpellLevel < 1 || r.SpellLevel > MaxPriestSpellLevel {
			return fmt.Errorf("spell level %d outside 1..%d", r.SpellLevel, MaxPriestSpellLevel)
		}
		if seen[r.SpellLevel] {
			return fmt.Errorf("spell level %d twice", r.SpellLevel)
		}
		seen[r.SpellLevel] = true
	}
	return nil
}

func checkTurning(rows []TurningRow) error {
	if err := once(rows, func(r TurningRow) string { return r.Undead }, Undead); err != nil {
		return err
	}
	for _, r := range rows {
		if r.HDMax != 0 && r.HDMax < r.HDMin {
			return fmt.Errorf("%s: Hit Dice %d..%d", r.Undead, r.HDMin, r.HDMax)
		}
		for i, c := range r.ByLevel {
			roll := c.Roll >= 1 && c.Roll <= 20
			outcome := slices.Contains(TurningOutcomes, c.Outcome)
			if roll == outcome || (c.Roll != 0 && !roll) || (c.Outcome != "" && !outcome) {
				return fmt.Errorf("%s level %d: want a roll 1..20 or one outcome, got %+v", r.Undead, i+1, c)
			}
		}
	}
	return nil
}
