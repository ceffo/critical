// Package ruletablesadpt reads the PHB rule tables into a
// ruletablesvc.Tables from the catalogue artifact's rule_tables (ADR 0021
// step 9, through Parse), with the class and race tables (step 10), the
// racial level limits (step 11), the sphere access (step 12), the equipment
// (step 13) and the languages (step 14). It only holds the private JSON DTOs and the
// mapping; the entities and their validation live in ruletablesvc.
package ruletablesadpt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

type fileDTO struct {
	Strength  []strengthDTO `json:"strength"`
	Dexterity []dexDTO      `json:"dexterity"`
	Bard      []bardDTO     `json:"bard_spells"`
	XP        []xpDTO       `json:"bard_experience"`
	classDTO
	// LevelLimits is DMG Table 7 (ADR 0021 step 11).
	LevelLimits []levelLimitDTO `json:"race_level_limits"`
	// SphereAccess is the artifact's class-keyed sphere rows (ADR 0021 step
	// 12). Not read: a priest's spheres are its sphere_access feature (ADR
	// 0040 step 3); the field only lets the strict decoder accept them.
	SphereAccess []sphereAccessDTO `json:"sphere_access"`
	// WeaponVsArmor is PHB Table 52 and InitiativeItem Table 56 (ADR 0021
	// step 13).
	WeaponVsArmor  []weaponVsArmorDTO  `json:"weapon_vs_armor"`
	InitiativeItem []initiativeItemDTO `json:"initiative_item"`
	// RaceLanguages is each race's language list (ADR 0021 step 14).
	RaceLanguages []raceLanguageDTO `json:"race_languages"`
}

type sphereAccessDTO struct {
	Class  string `json:"class"`
	Sphere string `json:"sphere"`
	Access string `json:"access"`
}

// levelLimitDTO is a race_level_limits row: MaxLevel absent is the book's U
// (no limit), 0 its dash (the race cannot belong to the class).
type levelLimitDTO struct {
	Race     string `json:"race"`
	Class    string `json:"class"`
	MaxLevel *int   `json:"max_level"`
}

type xpDTO struct {
	Level int `json:"level"`
	XP    int `json:"xp"`
}

type strengthDTO struct {
	StrMin int `json:"str_min"`
	StrMax int `json:"str_max"`
	ExcMin int `json:"exc_min"`
	ExcMax int `json:"exc_max"`
	Hit    int `json:"hit"`
	Dmg    int `json:"dmg"`
}

type dexDTO struct {
	DexMin    int `json:"dex_min"`
	DexMax    int `json:"dex_max"`
	Reaction  int `json:"reaction"`
	Missile   int `json:"missile"`
	Defensive int `json:"defensive"`
}

type bardDTO struct {
	Level         int   `json:"level"`
	PerSpellLevel []int `json:"per_spell_level"`
}

// Parse maps the rule_tables JSON of a current-format artifact (ADR 0021
// step 16.5: the only format the API reads, which catalogueadpt enforces
// before this runs) to validated Tables. A violation names the table. Every
// class and race table, the racial level limits, the sphere access and the
// two equipment tables (Table 52, Table 56) are required, as are the artifact's
// top-level equipment object and languages list (both as written) with the
// race_languages table.
func Parse(raw, equipment, languages, defs []byte) (*ruletablesvc.Tables, error) {
	reg, err := definitions(defs)
	if err != nil {
		return nil, err
	}
	if err := ruletablesvc.CheckDefinitions(reg); err != nil {
		return nil, fmt.Errorf("definitions: %w", err)
	}
	var f fileDTO
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	str := make([]ruletablesvc.StrengthRow, len(f.Strength))
	for i, d := range f.Strength {
		str[i] = ruletablesvc.StrengthRow{
			StrMin: d.StrMin, StrMax: d.StrMax, ExcMin: d.ExcMin, ExcMax: d.ExcMax, Hit: d.Hit, Dmg: d.Dmg,
		}
	}
	dex := make([]ruletablesvc.DexRow, len(f.Dexterity))
	for i, d := range f.Dexterity {
		dex[i] = ruletablesvc.DexRow{
			DexMin: d.DexMin, DexMax: d.DexMax, Reaction: d.Reaction, Missile: d.Missile, Defensive: d.Defensive,
		}
	}
	bard := make([]ruletablesvc.BardRow, len(f.Bard))
	for i, d := range f.Bard {
		if len(d.PerSpellLevel) != ruletablesvc.MaxSpellLevel {
			return nil, fmt.Errorf("bard level %d: per_spell_level needs %d entries, got %d",
				d.Level, ruletablesvc.MaxSpellLevel, len(d.PerSpellLevel))
		}
		bard[i].Level = d.Level
		copy(bard[i].PerSpellLevel[:], d.PerSpellLevel)
	}
	xp := make([]ruletablesvc.XPRow, len(f.XP))
	for i, d := range f.XP {
		xp[i] = ruletablesvc.XPRow{Level: d.Level, XP: d.XP}
	}
	tables, err := ruletablesvc.New(str, dex, bard, xp)
	if err != nil {
		return nil, err
	}
	tables = tables.WithDefinitions(reg)
	classTables, err := f.classes(reg)
	if err != nil {
		return nil, err
	}
	limits, err := levelLimits(f.LevelLimits, reg)
	if err != nil {
		return nil, err
	}
	tables = tables.WithClasses(classTables).WithLevelLimits(limits)
	tables, err = withEquipment(tables, f, equipment)
	if err != nil {
		return nil, err
	}
	return withLanguages(tables, f, languages, reg)
}

// withEquipment adds the equipment data and its two tables (the rules
// are ruletablesvc's); all three are required.
func withEquipment(t *ruletablesvc.Tables, f fileDTO, equipment []byte) (*ruletablesvc.Tables, error) {
	e, err := parseEquipment(equipment)
	if err != nil {
		return nil, err
	}
	w, err := weaponVsArmor(f.WeaponVsArmor)
	if err != nil {
		return nil, err
	}
	i, err := initiativeItems(f.InitiativeItem)
	if err != nil {
		return nil, err
	}
	return t.WithEquipment(e, w, i), nil
}

// levelLimits maps race_level_limits to validated level limits (the rules
// are ruletablesvc.NewLevelLimits'); the table is required.
func levelLimits(rows []levelLimitDTO, defs *classes.Registry) (*ruletablesvc.LevelLimits, error) {
	if rows == nil {
		return nil, errors.New("the artifact needs the race_level_limits table")
	}
	out := make([]ruletablesvc.LevelLimitRow, len(rows))
	for i, r := range rows {
		out[i] = ruletablesvc.LevelLimitRow{Race: r.Race, Class: r.Class, Unlimited: r.MaxLevel == nil}
		if r.MaxLevel != nil {
			out[i].MaxLevel = *r.MaxLevel
		}
	}
	limits, err := ruletablesvc.NewLevelLimits(out, defs)
	if err != nil {
		return nil, fmt.Errorf("race_level_limits table: %w", err)
	}
	return limits, nil
}
