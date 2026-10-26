package ruletablesadpt

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// classDTO holds the class and race tables of the rule_tables (ADR
// 0021 step 10). A nil slice is a table the artifact lacks.
type classDTO struct {
	// Experience is the formats-4-to-10 class-keyed XP table the artifact
	// still carries (ADR 0040 step 7); the thresholds are read from the class
	// definitions, so it is accepted and not read.
	Experience        json.RawMessage        `json:"experience"`
	HitDice           []hitDiceDTO           `json:"hit_dice"`
	Constitution      []constitutionDTO      `json:"constitution"`
	Intelligence      []intelligenceDTO      `json:"intelligence"`
	Wisdom            []wisdomDTO            `json:"wisdom"`
	ConSaveBonus      []conSaveBonusDTO      `json:"con_save_bonus"`
	RaceRequirements  []raceRequirementDTO   `json:"race_requirements"`
	RaceAdjustments   []raceAdjustmentDTO    `json:"race_adjustments"`
	THAC0             []thac0DTO             `json:"thac0"`
	SavingThrows      []savingThrowDTO       `json:"saving_throws"`
	WarriorAttacks    []attackRateDTO        `json:"warrior_attacks"`
	SpecialistAttacks []specialistAttackDTO  `json:"specialist_attacks"`
	RateOfFire        []rateOfFireDTO        `json:"rate_of_fire"`
	WizardSpells      []progressionDTO       `json:"wizard_spells"`
	PriestSpells      []progressionDTO       `json:"priest_spells"`
	PriestSpellWisdom []priestSpellWisdomDTO `json:"priest_spell_wisdom"`
	SkillRace         []skillRaceDTO         `json:"skill_race"`
	SkillDexterity    []skillDexterityDTO    `json:"skill_dexterity"`
	SkillArmor        []skillArmorDTO        `json:"skill_armor"`
	Backstab          []backstabDTO          `json:"backstab"`
	TurningUndead     []turningDTO           `json:"turning_undead"`
	// The psionicist tables (ADR 0021 step 20, format 13): Table 5 is read
	// (ADR 0043 step 4), Table 6 too (steps 6 and 7), Table 4 the powers known
	// (step 9).
	PsionicistPowers  []powerProgressionDTO  `json:"psionicist_powers"`
	InherentPotential []inherentPotentialDTO `json:"inherent_potential"`
	PSPRecovery       []pspRecoveryDTO       `json:"psp_recovery"`
	// The psionic combat tables (ADR 0021 step 25, format 18; ADR 0044 step 1):
	// read as data here, checked and looked up by the rule tables service.
	AttackVersusDefence []attackVersusDefenceDTO `json:"attack_versus_defence"`
	ContactCost         []contactCostDTO         `json:"contact_cost"`
	ContactModifiers    []contactModifierDTO     `json:"contact_modifiers"`
}

type attackVersusDefenceDTO struct {
	Attack   string `json:"attack"`
	Defence  string `json:"defence"`
	Modifier int    `json:"modifier"`
}

type contactCostDTO struct {
	MinLevel int `json:"min_level"`
	MaxLevel int `json:"max_level"`
	Cost     int `json:"cost"`
}

type contactModifierDTO struct {
	Kind     string `json:"kind"`
	Code     string `json:"code"`
	Modifier int    `json:"modifier"`
}

type powerProgressionDTO struct {
	Level        int `json:"level"`
	Disciplines  int `json:"disciplines"`
	Sciences     int `json:"sciences"`
	Devotions    int `json:"devotions"`
	DefenceModes int `json:"defence_modes"`
}

type pspRecoveryDTO struct {
	Activity string `json:"activity"`
	PerHour  int    `json:"per_hour"`
	// RequiresProficiency is format 20's gate (ADR 0047 item 6).
	RequiresProficiency string `json:"requires_proficiency"`
}

type inherentPotentialDTO struct {
	Score    int `json:"score"`
	Base     int `json:"base"`
	Modifier int `json:"modifier"`
}

type hitDiceDTO struct {
	Group     string `json:"group"`
	Die       int    `json:"die"`
	NameLevel int    `json:"name_level"`
	GainAfter int    `json:"gain_after"`
}

type constitutionDTO struct {
	ConMin    int `json:"con_min"`
	ConMax    int `json:"con_max"`
	HP        int `json:"hp"`
	HPWarrior int `json:"hp_warrior"`
	LowDieMin int `json:"low_die_min"`
}

type intelligenceDTO struct {
	IntMin        int  `json:"int_min"`
	IntMax        int  `json:"int_max"`
	MaxSpellLevel int  `json:"max_spell_level"`
	LearnChance   int  `json:"learn_chance"`
	MaxSpells     *int `json:"max_spells"`
}

type wisdomDTO struct {
	WisMin       int   `json:"wis_min"`
	WisMax       int   `json:"wis_max"`
	BonusSpells  []int `json:"bonus_spells"`
	SpellFailure int   `json:"spell_failure"`
	MagicDefence int   `json:"magic_defence"`
	// SpellImmunities is format 12's column (ADR 0021 step 19).
	SpellImmunities []string `json:"spell_immunities"`
}

type conSaveBonusDTO struct {
	ConMin int `json:"con_min"`
	ConMax int `json:"con_max"`
	Bonus  int `json:"bonus"`
}

type raceRequirementDTO struct {
	Race    string `json:"race"`
	Ability string `json:"ability"`
	Min     int    `json:"min"`
	Max     int    `json:"max"`
}

type raceAdjustmentDTO struct {
	Race       string `json:"race"`
	Ability    string `json:"ability"`
	Adjustment int    `json:"adjustment"`
}

type thac0DTO struct {
	Level   int `json:"level"`
	Warrior int `json:"warrior"`
	Priest  int `json:"priest"`
	Rogue   int `json:"rogue"`
	Wizard  int `json:"wizard"`
	// Psionicist is format 13's column; the group entries carry the figures the service reads (ADR 0043).
	Psionicist int `json:"psionicist"`
}

type savingThrowDTO struct {
	Group        string `json:"group"`
	LevelMin     int    `json:"level_min"`
	LevelMax     int    `json:"level_max"`
	ParaPoison   int    `json:"para_poison"`
	RodStaffWand int    `json:"rod_staff_wand"`
	PetriPoly    int    `json:"petri_poly"`
	Breath       int    `json:"breath"`
	Spell        int    `json:"spell"`
}

type attackRateDTO struct {
	LevelMin int `json:"level_min"`
	LevelMax int `json:"level_max"`
	Attacks  int `json:"attacks"`
	Rounds   int `json:"rounds"`
}

type specialistAttackDTO struct {
	Weapon   string `json:"weapon"`
	LevelMin int    `json:"level_min"`
	LevelMax int    `json:"level_max"`
	Attacks  int    `json:"attacks"`
	Rounds   int    `json:"rounds"`
}

type rateOfFireDTO struct {
	WeaponType string `json:"weapon_type"`
	Attacks    int    `json:"attacks"`
	Rounds     int    `json:"rounds"`
}

type progressionDTO struct {
	Level         int   `json:"level"`
	PerSpellLevel []int `json:"per_spell_level"`
}

type priestSpellWisdomDTO struct {
	SpellLevel int `json:"spell_level"`
	WisMin     int `json:"wis_min"`
}

type skillRaceDTO struct {
	Race       string `json:"race"`
	Skill      string `json:"skill"`
	Adjustment int    `json:"adjustment"`
}

type skillDexterityDTO struct {
	DexMin     int    `json:"dex_min"`
	DexMax     int    `json:"dex_max"`
	Skill      string `json:"skill"`
	Adjustment int    `json:"adjustment"`
}

type skillArmorDTO struct {
	Armor      string `json:"armor"`
	Skill      string `json:"skill"`
	Adjustment int    `json:"adjustment"`
}

type backstabDTO struct {
	LevelMin   int `json:"level_min"`
	LevelMax   int `json:"level_max"`
	Multiplier int `json:"multiplier"`
}

type turningDTO struct {
	Undead  string           `json:"undead"`
	HDMin   int              `json:"hd_min"`
	HDMax   int              `json:"hd_max"`
	ByLevel []turningCellDTO `json:"by_level"`
}

type turningCellDTO struct {
	Roll    int    `json:"roll"`
	Outcome string `json:"outcome"`
}

// missing names the tables (by JSON key) the artifact does not carry.
func (d classDTO) missing() []string {
	var out []string
	v := reflect.ValueOf(d)
	for i := range v.NumField() {
		if v.Field(i).IsNil() {
			out = append(out, strings.Split(v.Type().Field(i).Tag.Get("json"), ",")[0])
		}
	}
	return out
}

// classes maps the DTOs to validated class tables. Every table is required
// (a missing one names it); the rules are ruletablesvc.NewClassTables'.
func (d classDTO) classes(defs *classes.Registry, sets []catalogue.SkillSet) (*ruletablesvc.ClassTables, error) {
	if m := d.missing(); len(m) > 0 {
		return nil, fmt.Errorf("the artifact needs the class and race tables, missing: %s", strings.Join(m, ", "))
	}
	if err := d.checkSpecialization(defs); err != nil {
		return nil, err
	}
	turning := make([]ruletablesvc.TurningRow, len(d.TurningUndead))
	for i, r := range d.TurningUndead {
		if len(r.ByLevel) != ruletablesvc.MaxLevel {
			return nil, fmt.Errorf("turning_undead table: %s: %d levels, want %d", r.Undead, len(r.ByLevel), ruletablesvc.MaxLevel)
		}
		turning[i] = ruletablesvc.TurningRow{Undead: r.Undead, HDMin: r.HDMin, HDMax: r.HDMax}
		for l, c := range r.ByLevel {
			turning[i].ByLevel[l] = ruletablesvc.TurningCell(c)
		}
	}
	return ruletablesvc.NewClassTables(ruletablesvc.ClassTables{
		HitDice:      mapRows(d.HitDice, func(r hitDiceDTO) ruletablesvc.HitDiceRow { return ruletablesvc.HitDiceRow(r) }),
		Constitution: mapRows(d.Constitution, func(r constitutionDTO) ruletablesvc.ConstitutionRow { return ruletablesvc.ConstitutionRow(r) }),
		Intelligence: mapRows(d.Intelligence, func(r intelligenceDTO) ruletablesvc.IntelligenceRow {
			out := ruletablesvc.IntelligenceRow{
				IntMin: r.IntMin, IntMax: r.IntMax, MaxSpellLevel: r.MaxSpellLevel, LearnChance: r.LearnChance,
				NoMaximum: r.MaxSpells == nil,
			}
			if r.MaxSpells != nil {
				out.MaxSpells = *r.MaxSpells
			}
			return out
		}),
		Wisdom: mapRows(d.Wisdom, func(r wisdomDTO) ruletablesvc.WisdomRow {
			return ruletablesvc.WisdomRow{WisMin: r.WisMin, WisMax: r.WisMax, BonusSpells: append([]int{}, r.BonusSpells...), SpellFailure: r.SpellFailure, MagicDefence: r.MagicDefence, SpellImmunities: append([]string{}, r.SpellImmunities...)}
		}),
		ConSaveBonus:     mapRows(d.ConSaveBonus, func(r conSaveBonusDTO) ruletablesvc.ConSaveBonusRow { return ruletablesvc.ConSaveBonusRow(r) }),
		RaceRequirements: mapRows(d.RaceRequirements, func(r raceRequirementDTO) ruletablesvc.RaceRequirementRow { return ruletablesvc.RaceRequirementRow(r) }),
		RaceAdjustments:  mapRows(d.RaceAdjustments, func(r raceAdjustmentDTO) ruletablesvc.RaceAdjustmentRow { return ruletablesvc.RaceAdjustmentRow(r) }),
		THAC0: mapRows(d.THAC0, func(r thac0DTO) ruletablesvc.THAC0Row {
			return ruletablesvc.THAC0Row{Level: r.Level, Warrior: r.Warrior, Priest: r.Priest, Rogue: r.Rogue, Wizard: r.Wizard}
		}),
		SavingThrows:   mapRows(d.SavingThrows, func(r savingThrowDTO) ruletablesvc.SavingThrowRow { return ruletablesvc.SavingThrowRow(r) }),
		WarriorAttacks: mapRows(d.WarriorAttacks, func(r attackRateDTO) ruletablesvc.AttackRateRow { return ruletablesvc.AttackRateRow(r) }),
		SpecialistAttacks: mapRows(d.SpecialistAttacks, func(r specialistAttackDTO) ruletablesvc.SpecialistAttackRow {
			return ruletablesvc.SpecialistAttackRow(r)
		}),
		RateOfFire:   mapRows(d.RateOfFire, func(r rateOfFireDTO) ruletablesvc.RateOfFireRow { return ruletablesvc.RateOfFireRow(r) }),
		WizardSpells: mapRows(d.WizardSpells, progression),
		PriestSpells: mapRows(d.PriestSpells, progression),
		PriestSpellWisdom: mapRows(d.PriestSpellWisdom, func(r priestSpellWisdomDTO) ruletablesvc.PriestSpellWisdomRow {
			return ruletablesvc.PriestSpellWisdomRow(r)
		}),
		SkillSets:      sets,
		SkillRace:      mapRows(d.SkillRace, func(r skillRaceDTO) ruletablesvc.SkillRaceRow { return ruletablesvc.SkillRaceRow(r) }),
		SkillDexterity: mapRows(d.SkillDexterity, func(r skillDexterityDTO) ruletablesvc.SkillDexterityRow { return ruletablesvc.SkillDexterityRow(r) }),
		SkillArmor:     mapRows(d.SkillArmor, func(r skillArmorDTO) ruletablesvc.SkillArmorRow { return ruletablesvc.SkillArmorRow(r) }),
		Backstab:       mapRows(d.Backstab, func(r backstabDTO) ruletablesvc.BackstabRow { return ruletablesvc.BackstabRow(r) }),
		TurningUndead:  turning,
		InherentPotential: mapRows(d.InherentPotential, func(r inherentPotentialDTO) ruletablesvc.InherentPotentialRow {
			return ruletablesvc.InherentPotentialRow(r)
		}),
		PowerProgression: mapRows(d.PsionicistPowers, func(r powerProgressionDTO) ruletablesvc.PowerProgressionRow {
			return ruletablesvc.PowerProgressionRow(r)
		}),
		PSPRecovery: mapRows(d.PSPRecovery, func(r pspRecoveryDTO) ruletablesvc.PSPRecoveryRow { return ruletablesvc.PSPRecoveryRow(r) }),
		AttackVersusDefence: mapRows(d.AttackVersusDefence, func(r attackVersusDefenceDTO) ruletablesvc.AttackVersusDefenceRow {
			return ruletablesvc.AttackVersusDefenceRow(r)
		}),
		ContactCost:      mapRows(d.ContactCost, func(r contactCostDTO) ruletablesvc.ContactCostRow { return ruletablesvc.ContactCostRow(r) }),
		ContactModifiers: mapRows(d.ContactModifiers, func(r contactModifierDTO) ruletablesvc.ContactModifierRow { return ruletablesvc.ContactModifierRow(r) }),
	}, defs)
}

func progression(r progressionDTO) ruletablesvc.SpellProgressionRow {
	return ruletablesvc.SpellProgressionRow{Level: r.Level, PerSpellLevel: append([]int{}, r.PerSpellLevel...)}
}

func mapRows[D, R any](in []D, f func(D) R) []R {
	out := make([]R, len(in))
	for i, d := range in {
		out[i] = f(d)
	}
	return out
}

// checkSpecialization is ADR 0040 step 13's start-up check on every weapon_specialization the
// classes list: a slot count of at least 1, a column that is a Table 35 weapon, a weapon type in
// one row only, and a rate_of_fire type that has a Table 45 row.
func (d classDTO) checkSpecialization(defs *classes.Registry) error {
	weapons := map[string]bool{}
	for _, r := range d.SpecialistAttacks {
		weapons[r.Weapon] = true
	}
	rated := map[string]bool{}
	for _, r := range d.RateOfFire {
		rated[r.WeaponType] = true
	}
	for _, c := range defs.Classes() {
		for _, f := range c.ListsAll(classes.KindWeaponSpecialization) {
			if err := checkSpecializationParams(f.Specialization, weapons, rated); err != nil {
				return fmt.Errorf("class %q: weapon_specialization: %w", c.ID, err)
			}
		}
	}
	return nil
}

func checkSpecializationParams(w *classes.WeaponSpecialization, weapons, rated map[string]bool) error {
	if w == nil {
		return errors.New("the parameters are missing")
	}
	if w.DefaultSlots < 1 {
		return fmt.Errorf("default_slots %d under 1", w.DefaultSlots)
	}
	for _, column := range []string{w.MeleeColumn, w.MissileColumn} {
		if !weapons[column] {
			return fmt.Errorf("column %q is no specialist_attacks weapon", column)
		}
	}
	seen := map[string]bool{}
	for _, t := range w.Types {
		if t.Slots < 1 {
			return fmt.Errorf("types %v: slots %d under 1", t.WeaponTypes, t.Slots)
		}
		if t.Column != "" && !weapons[t.Column] {
			return fmt.Errorf("types %v: column %q is no specialist_attacks weapon", t.WeaponTypes, t.Column)
		}
		for _, wt := range t.WeaponTypes {
			if seen[wt] {
				return fmt.Errorf("weapon type %q listed twice", wt)
			}
			seen[wt] = true
			if t.RateOfFire && !rated[wt] {
				return fmt.Errorf("weapon type %q keeps its rate of fire but has no rate_of_fire row", wt)
			}
		}
	}
	return nil
}
