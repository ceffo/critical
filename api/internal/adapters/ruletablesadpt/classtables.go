package ruletablesadpt

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// classDTO holds the class and race tables of a format-4 rule_tables (ADR
// 0021 step 10). A nil slice is a table the artifact does not carry.
type classDTO struct {
	Experience          []experienceDTO        `json:"experience"`
	HitDice             []hitDiceDTO           `json:"hit_dice"`
	Constitution        []constitutionDTO      `json:"constitution"`
	Intelligence        []intelligenceDTO      `json:"intelligence"`
	Wisdom              []wisdomDTO            `json:"wisdom"`
	ConSaveBonus        []conSaveBonusDTO      `json:"con_save_bonus"`
	RaceRequirements    []raceRequirementDTO   `json:"race_requirements"`
	RaceAdjustments     []raceAdjustmentDTO    `json:"race_adjustments"`
	THAC0               []thac0DTO             `json:"thac0"`
	SavingThrows        []savingThrowDTO       `json:"saving_throws"`
	WarriorAttacks      []attackRateDTO        `json:"warrior_attacks"`
	SpecialistAttacks   []specialistAttackDTO  `json:"specialist_attacks"`
	RateOfFire          []rateOfFireDTO        `json:"rate_of_fire"`
	WizardSpells        []progressionDTO       `json:"wizard_spells"`
	PriestSpells        []progressionDTO       `json:"priest_spells"`
	PriestSpellWisdom   []priestSpellWisdomDTO `json:"priest_spell_wisdom"`
	ThiefSkillBase      []skillBaseDTO         `json:"thief_skill_base"`
	ThiefSkillRace      []skillRaceDTO         `json:"thief_skill_race"`
	ThiefSkillDexterity []skillDexterityDTO    `json:"thief_skill_dexterity"`
	ThiefSkillArmor     []skillArmorDTO        `json:"thief_skill_armor"`
	BardArmorPenalty    []bardArmorPenaltyDTO  `json:"bard_armor_penalty"`
	BardSkillBase       []skillBaseDTO         `json:"bard_skill_base"`
	Backstab            []backstabDTO          `json:"backstab"`
	TurningUndead       []turningDTO           `json:"turning_undead"`
}

type experienceDTO struct {
	Level   int `json:"level"`
	Fighter int `json:"fighter"`
	Mage    int `json:"mage"`
	Cleric  int `json:"cleric"`
	Thief   int `json:"thief"`
	Bard    int `json:"bard"`
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

type skillBaseDTO struct {
	Skill string `json:"skill"`
	Base  int    `json:"base"`
}

type skillRaceDTO struct {
	Race       string `json:"race"`
	Skill      string `json:"skill"`
	Adjustment int    `json:"adjustment"`
}

type skillDexterityDTO struct {
	DexMin          int `json:"dex_min"`
	DexMax          int `json:"dex_max"`
	PickPockets     int `json:"pick_pockets"`
	OpenLocks       int `json:"open_locks"`
	FindRemoveTraps int `json:"find_remove_traps"`
	MoveSilently    int `json:"move_silently"`
	HideInShadows   int `json:"hide_in_shadows"`
}

type skillArmorDTO struct {
	Armor      string `json:"armor"`
	Skill      string `json:"skill"`
	Adjustment int    `json:"adjustment"`
}

type bardArmorPenaltyDTO struct {
	Armor      string `json:"armor"`
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

// present reports whether the artifact carries any class or race table.
func (d classDTO) present() bool {
	return len(d.missing()) < reflect.TypeOf(d).NumField()
}

// classes maps the DTOs to validated class tables. Every table is required
// (a missing one names it); the rules are ruletablesvc.NewClassTables'.
func (d classDTO) classes() (*ruletablesvc.ClassTables, error) {
	if m := d.missing(); len(m) > 0 {
		return nil, fmt.Errorf("format 4 needs the class and race tables, missing: %s", strings.Join(m, ", "))
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
		Experience:   mapRows(d.Experience, func(r experienceDTO) ruletablesvc.ExperienceRow { return ruletablesvc.ExperienceRow(r) }),
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
			return ruletablesvc.WisdomRow{WisMin: r.WisMin, WisMax: r.WisMax, BonusSpells: append([]int{}, r.BonusSpells...), SpellFailure: r.SpellFailure}
		}),
		ConSaveBonus:     mapRows(d.ConSaveBonus, func(r conSaveBonusDTO) ruletablesvc.ConSaveBonusRow { return ruletablesvc.ConSaveBonusRow(r) }),
		RaceRequirements: mapRows(d.RaceRequirements, func(r raceRequirementDTO) ruletablesvc.RaceRequirementRow { return ruletablesvc.RaceRequirementRow(r) }),
		RaceAdjustments:  mapRows(d.RaceAdjustments, func(r raceAdjustmentDTO) ruletablesvc.RaceAdjustmentRow { return ruletablesvc.RaceAdjustmentRow(r) }),
		THAC0:            mapRows(d.THAC0, func(r thac0DTO) ruletablesvc.THAC0Row { return ruletablesvc.THAC0Row(r) }),
		SavingThrows:     mapRows(d.SavingThrows, func(r savingThrowDTO) ruletablesvc.SavingThrowRow { return ruletablesvc.SavingThrowRow(r) }),
		WarriorAttacks:   mapRows(d.WarriorAttacks, func(r attackRateDTO) ruletablesvc.AttackRateRow { return ruletablesvc.AttackRateRow(r) }),
		SpecialistAttacks: mapRows(d.SpecialistAttacks, func(r specialistAttackDTO) ruletablesvc.SpecialistAttackRow {
			return ruletablesvc.SpecialistAttackRow(r)
		}),
		RateOfFire:   mapRows(d.RateOfFire, func(r rateOfFireDTO) ruletablesvc.RateOfFireRow { return ruletablesvc.RateOfFireRow(r) }),
		WizardSpells: mapRows(d.WizardSpells, progression),
		PriestSpells: mapRows(d.PriestSpells, progression),
		PriestSpellWisdom: mapRows(d.PriestSpellWisdom, func(r priestSpellWisdomDTO) ruletablesvc.PriestSpellWisdomRow {
			return ruletablesvc.PriestSpellWisdomRow(r)
		}),
		ThiefSkillBase: mapRows(d.ThiefSkillBase, func(r skillBaseDTO) ruletablesvc.SkillBaseRow { return ruletablesvc.SkillBaseRow(r) }),
		ThiefSkillRace: mapRows(d.ThiefSkillRace, func(r skillRaceDTO) ruletablesvc.SkillRaceRow { return ruletablesvc.SkillRaceRow(r) }),
		ThiefSkillDexterity: mapRows(d.ThiefSkillDexterity, func(r skillDexterityDTO) ruletablesvc.SkillDexterityRow {
			return ruletablesvc.SkillDexterityRow(r)
		}),
		ThiefSkillArmor: mapRows(d.ThiefSkillArmor, func(r skillArmorDTO) ruletablesvc.SkillArmorRow { return ruletablesvc.SkillArmorRow(r) }),
		BardArmorPenalty: mapRows(d.BardArmorPenalty, func(r bardArmorPenaltyDTO) ruletablesvc.BardArmorPenaltyRow {
			return ruletablesvc.BardArmorPenaltyRow(r)
		}),
		BardSkillBase: mapRows(d.BardSkillBase, func(r skillBaseDTO) ruletablesvc.SkillBaseRow { return ruletablesvc.SkillBaseRow(r) }),
		Backstab:      mapRows(d.Backstab, func(r backstabDTO) ruletablesvc.BackstabRow { return ruletablesvc.BackstabRow(r) }),
		TurningUndead: turning,
	})
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
