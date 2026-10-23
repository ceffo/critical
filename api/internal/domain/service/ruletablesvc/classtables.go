package ruletablesvc

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// The class and race tables of ADR 0030 step 23, carried by catalogue format
// 4 (ADR 0021 step 10). They are read and validated at every start; the
// derivations that use them (level, THAC0, saves, slots, skills...) are
// ADR 0030's steps, not this file's. Every code is language-neutral.

// MaxLevel is the last character level the tables cover (ADR 0030 step 3).
const MaxLevel = 20

// MaxScore is the highest ability score of the PHB tables.
const MaxScore = 25

// Abilities are the six ability score codes.
var Abilities = classes.Abilities

// ThiefSkills are the eight thieving skills of PHB Table 26.
var ThiefSkills = []string{
	"pick_pockets", "open_locks", "find_remove_traps", "move_silently",
	"hide_in_shadows", "detect_noise", "climb_walls", "read_languages",
}

// ArmourFreeSkills are the two thieving skills a multi-class thief in body
// armour its class may not wear still uses, at the no-armour percentage (ADR
// 0042 step 16.3, Q16; PHB multi/dual, Thief: the gauntlets off for the first,
// the helmet for the second). Every other skill of the set is blocked.
var ArmourFreeSkills = []string{"open_locks", "detect_noise"}

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

// The Table 61 cells that need no die: T, D, D* and a dash.
const (
	TurningTurn        = "turn"
	TurningDestroy     = "destroy"
	TurningDestroyMore = "destroy_more"
	TurningCannot      = "cannot"
)

// TurningOutcomes are the Table 61 cells that need no die.
var TurningOutcomes = []string{TurningTurn, TurningDestroy, TurningDestroyMore, TurningCannot}

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
// cumulative), the chance of spell failure and the magical defence
// adjustment (ADR 0006 step 9.1: a signed bonus on saves against magic that
// attacks the mind).
type WisdomRow struct {
	WisMin, WisMax int
	BonusSpells    []int
	SpellFailure   int
	MagicDefence   int
	// SpellImmunities are the catalogue ids of the spells this row adds to
	// the immunities of the rows below it (ADR 0006 step 10.1).
	SpellImmunities []string
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

// SkillRaceRow is PHB Table 27.
type SkillRaceRow struct {
	Race, Skill string
	Adjustment  int
}

// SkillDexterityRow is a row of PHB Table 28 keyed by skill (ADR 0040 step
// 12): the adjustment of Skill at the scores DexMin..DexMax (scores outside
// the skill's rows: 0).
type SkillDexterityRow struct {
	DexMin, DexMax int
	Skill          string
	Adjustment     int
}

// SkillArmorRow is PHB Table 29.
type SkillArmorRow struct {
	Armor, Skill string
	Adjustment   int
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
	HitDice           []HitDiceRow
	Constitution      []ConstitutionRow
	Intelligence      []IntelligenceRow
	Wisdom            []WisdomRow
	ConSaveBonus      []ConSaveBonusRow
	RaceRequirements  []RaceRequirementRow
	RaceAdjustments   []RaceAdjustmentRow
	THAC0             []THAC0Row
	SavingThrows      []SavingThrowRow
	WarriorAttacks    []AttackRateRow
	SpecialistAttacks []SpecialistAttackRow
	RateOfFire        []RateOfFireRow
	WizardSpells      []SpellProgressionRow
	PriestSpells      []SpellProgressionRow
	PriestSpellWisdom []PriestSpellWisdomRow
	// SkillSets are the artifact's skill sets (format 19, ADR 0040 step 12): the
	// skills of each with their base percentages (PHB Tables 26 and 33).
	SkillSets      []catalogue.SkillSet
	SkillRace      []SkillRaceRow
	SkillDexterity []SkillDexterityRow
	SkillArmor     []SkillArmorRow
	Backstab       []BackstabRow
	TurningUndead  []TurningRow
	// InherentPotential is CPsiH Table 5 (ADR 0043 step 4), empty in a
	// catalogue without the psionicist.
	InherentPotential []InherentPotentialRow
	// PSPRecovery is CPsiH Table 6 (ADR 0043 steps 6 and 7), empty in a
	// catalogue without the psionicist.
	PSPRecovery []PSPRecoveryRow
	// PowerProgression is CPsiH Table 4 (ADR 0043 step 9), empty in a
	// catalogue without the psionicist.
	PowerProgression []PowerProgressionRow
	// The psionic combat tables (ADR 0044 step 1), empty in a catalogue
	// without the psionicist.
	AttackVersusDefence []AttackVersusDefenceRow
	ContactCost         []ContactCostRow
	ContactModifiers    []ContactModifierRow
}

// MaxWizardSpellLevel and MaxPriestSpellLevel are the widths of PHB Tables
// 21 and 24.
const (
	MaxWizardSpellLevel = 9
	MaxPriestSpellLevel = 7
)

// NewClassTables validates c against the definitions (the rules the
// catalogue schema cannot express) and returns it. The groups and races are
// the definitions' (ADR 0040 step 8: a row keyed by a group or race names a
// definition); a race with no row of a race table (a human) has no
// requirement, adjustment or skill adjustment. A violation names the table:
// levels 1 to 20 once and in order, ability rows
// ascending and contiguous (1 to 25 for Constitution, Intelligence and
// Wisdom), level bands covering 1 to 20, and every code combination exactly
// once.
func NewClassTables(c ClassTables, defs *classes.Registry) (*ClassTables, error) {
	if defs == nil {
		return nil, errors.New("class tables: no class definitions")
	}
	groups, races := defs.Groups(), defs.Races()
	checks := []struct {
		table string
		err   error
	}{
		{"hit_dice", once(c.HitDice, func(r HitDiceRow) string { return r.Group }, groups)},
		{"constitution", checkScores(c.Constitution, func(r ConstitutionRow) (int, int) { return r.ConMin, r.ConMax }, true)},
		{"intelligence", checkScores(c.Intelligence, func(r IntelligenceRow) (int, int) { return r.IntMin, r.IntMax }, true)},
		{"wisdom", checkWisdom(c.Wisdom)},
		{"con_save_bonus", checkScores(c.ConSaveBonus, func(r ConSaveBonusRow) (int, int) { return r.ConMin, r.ConMax }, false)},
		{"race_requirements", checkRaceRequirements(c.RaceRequirements, races)},
		{"race_adjustments", checkRaceAdjustments(c.RaceAdjustments, defs.RaceIDs())},
		{"thac0", levelsInOrder(c.THAC0, func(r THAC0Row) int { return r.Level })},
		{"saving_throws", checkSaves(c.SavingThrows, groups)},
		{"warrior_attacks", checkBands(c.WarriorAttacks, func(r AttackRateRow) (int, int) { return r.LevelMin, r.LevelMax })},
		{"specialist_attacks", checkSpecialists(c.SpecialistAttacks)},
		{"rate_of_fire", once(c.RateOfFire, func(r RateOfFireRow) string { return r.WeaponType }, MissileWeaponTypes)},
		{"wizard_spells", checkProgression(c.WizardSpells, MaxWizardSpellLevel)},
		{"priest_spells", checkProgression(c.PriestSpells, MaxPriestSpellLevel)},
		{"priest_spell_wisdom", checkPriestWisdom(c.PriestSpellWisdom)},
		{"skill_sets", checkSkillSets(c.SkillSets)},
		{"skill_race", perRace(c.SkillRace, func(r SkillRaceRow) (string, string) { return r.Race, r.Skill }, races, ThiefSkills)},
		{"skill_dexterity", checkSkillDexterity(c.SkillDexterity)},
		{"skill_armor", once(c.SkillArmor, func(r SkillArmorRow) string { return r.Armor + " " + r.Skill }, pairs(SkillArmors, ThiefSkills))},
		{"backstab", checkBands(c.Backstab, func(r BackstabRow) (int, int) { return r.LevelMin, r.LevelMax })},
		{"turning_undead", checkTurning(c.TurningUndead)},
		{"inherent_potential", checkInherentPotential(c.InherentPotential, defs)},
		{"psp_recovery", checkPSPRecovery(c.PSPRecovery, defs)},
		{"psionicist_powers", checkPowerProgression(c.PowerProgression, defs)},
		{"attack_versus_defence", checkAttackVersusDefence(c.AttackVersusDefence, defs)},
		{"contact_cost", checkContactCost(c.ContactCost, defs)},
		{"contact_modifiers", checkContactModifiers(c.ContactModifiers, defs)},
	}
	for _, ch := range checks {
		if ch.err != nil {
			return nil, fmt.Errorf("%s table: %w", ch.table, ch.err)
		}
	}
	return &c, nil
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

// checkSkillSets: the sets the Go skill lists still stand for (SkillSetThief
// and SkillSetBard) exist and list exactly those skills, each with a base
// (ADR 0040 step 12 replaces the lists by the sets themselves); set ids and
// skills within a set are unique (catalogue.CheckSkillSets, at the import).
func checkSkillSets(sets []catalogue.SkillSet) error {
	if err := catalogue.CheckSkillSets(sets); err != nil {
		return err
	}
	for set, want := range map[string][]string{SkillSetThief: ThiefSkills, SkillSetBard: BardSkills} {
		i := slices.IndexFunc(sets, func(s catalogue.SkillSet) bool { return s.ID == set })
		if i < 0 {
			return fmt.Errorf("the skill set %q is missing", set)
		}
		got := make([]string, len(sets[i].Skills))
		for j, k := range sets[i].Skills {
			got[j] = k.ID
		}
		if !slices.Equal(got, want) {
			return fmt.Errorf("the skill set %q lists %v, want %v", set, got, want)
		}
	}
	return nil
}

// checkSkillDexterity: Table 28's rows name a thieving skill, and each
// skill's rows ascend without overlap inside 1..MaxScore (a skill with no row
// at a score adjusts by 0).
func checkSkillDexterity(rows []SkillDexterityRow) error {
	if len(rows) == 0 {
		return errors.New("no rows")
	}
	bySkill := map[string][]SkillDexterityRow{}
	for _, r := range rows {
		if !slices.Contains(ThiefSkills, r.Skill) {
			return fmt.Errorf("unknown skill %q", r.Skill)
		}
		bySkill[r.Skill] = append(bySkill[r.Skill], r)
	}
	for skill, rs := range bySkill {
		if err := checkScores(rs, func(r SkillDexterityRow) (int, int) { return r.DexMin, r.DexMax }, false); err != nil {
			return fmt.Errorf("skill %s: %w", skill, err)
		}
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

// perRace: every row's race is one of races and its key one of keys, each
// pair once; a restricted race (classes.Race.Restricted) has a row for every
// key, an unrestricted one (the human) one for every key or none.
func perRace[R any](rows []R, key func(R) (race, k string), races []classes.Race, keys []string) error {
	ids := make([]string, len(races))
	for i, r := range races {
		ids[i] = r.ID
	}
	seen := map[string]map[string]bool{}
	for _, r := range rows {
		race, k := key(r)
		switch {
		case !slices.Contains(ids, race):
			return fmt.Errorf("unknown race %q", race)
		case !slices.Contains(keys, k):
			return fmt.Errorf("unknown %q", k)
		case seen[race][k]:
			return fmt.Errorf("%q twice", race+" "+k)
		}
		if seen[race] == nil {
			seen[race] = map[string]bool{}
		}
		seen[race][k] = true
	}
	for _, race := range races {
		if seen[race.ID] == nil && !race.Restricted() {
			continue
		}
		for _, k := range keys {
			if !seen[race.ID][k] {
				return fmt.Errorf("missing %q", race.ID+" "+k)
			}
		}
	}
	return nil
}

func checkRaceRequirements(rows []RaceRequirementRow, races []classes.Race) error {
	if err := perRace(rows, func(r RaceRequirementRow) (string, string) { return r.Race, r.Ability }, races, Abilities); err != nil {
		return err
	}
	for _, r := range rows {
		if r.Min > r.Max {
			return fmt.Errorf("%s %s: min above max", r.Race, r.Ability)
		}
	}
	return nil
}

func checkRaceAdjustments(rows []RaceAdjustmentRow, races []string) error {
	seen := map[string]bool{}
	for _, r := range rows {
		if !slices.Contains(races, r.Race) || !slices.Contains(Abilities, r.Ability) {
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

func checkSaves(rows []SavingThrowRow, groups []string) error {
	for _, r := range rows {
		if !slices.Contains(groups, r.Group) {
			return fmt.Errorf("unknown group %q", r.Group)
		}
	}
	for _, g := range groups {
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
