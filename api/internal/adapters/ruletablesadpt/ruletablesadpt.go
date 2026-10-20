// Package ruletablesadpt reads the PHB rule tables into a
// ruletablesvc.Tables from the catalogue artifact's rule_tables (ADR 0021
// step 9, through Parse), with the class and race tables of a format-4 or
// format-5 artifact (step 10) and the racial level limits of a format-5 one
// (step 11). It only holds the private JSON DTOs and the
// mapping; the entities and their validation live in ruletablesvc.
package ruletablesadpt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

type fileDTO struct {
	Strength  []strengthDTO `json:"strength"`
	Dexterity []dexDTO      `json:"dexterity"`
	Bard      []bardDTO     `json:"bard_spells"`
	XP        []xpDTO       `json:"bard_experience"`
	classDTO
	// LevelLimits is DMG Table 7 (format 5, ADR 0021 step 11); nil when the
	// artifact does not carry it.
	LevelLimits []levelLimitDTO `json:"race_level_limits"`
	// SphereAccess is the classes' sphere access (format 6, ADR 0021 step
	// 12); nil when the artifact does not carry it.
	SphereAccess []sphereAccessDTO `json:"sphere_access"`
	// WeaponVsArmor is PHB Table 52 and InitiativeItem Table 56 (format 7,
	// ADR 0021 step 13); nil when the artifact does not carry them.
	WeaponVsArmor  []weaponVsArmorDTO  `json:"weapon_vs_armor"`
	InitiativeItem []initiativeItemDTO `json:"initiative_item"`
	// RaceLanguages is each race's language list (format 8, ADR 0021 step
	// 14); nil when the artifact does not carry it.
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

// Parse maps the rule_tables JSON of an artifact of the given format to
// validated Tables. A violation names the table. Format 4 carries every
// class and race table (a missing one is named) but the racial level limits,
// format 5 those too, format 6 also the sphere access; format 3 carries none.
// equipment is the artifact's top-level equipment object as written, nil when
// absent (format 7, ADR 0021 step 13): formats 7 and 8 need it and the two
// tables Table 52 and Table 56; formats 3 to 6 carry none of them. languages
// is the artifact's top-level languages list as written, nil when absent
// (format 8, step 14): format 8 needs it and the race_languages table;
// formats 3 to 7 carry neither.
func Parse(format int, raw, equipment, languages []byte) (*ruletablesvc.Tables, error) {
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
	hasEquipment := format >= catalogue.FormatEquipment
	if !hasEquipment && (f.WeaponVsArmor != nil || f.InitiativeItem != nil || equipment != nil) {
		return nil, fmt.Errorf("format %d carries no equipment, weapon_vs_armor or initiative_item", format)
	}
	if format < catalogue.FormatLanguages && (f.RaceLanguages != nil || languages != nil) {
		return nil, fmt.Errorf("format %d carries no languages or race_languages", format)
	}
	switch format {
	case catalogue.FormatRuleTables:
		if f.present() || f.LevelLimits != nil || f.SphereAccess != nil {
			return nil, errors.New("format 3 carries no class or race table")
		}
		return tables, nil
	case catalogue.FormatClassTables:
		if f.LevelLimits != nil {
			return nil, errors.New("format 4 carries no race_level_limits table")
		}
		if f.SphereAccess != nil {
			return nil, errors.New("format 4 carries no sphere_access table")
		}
		classes, err := f.classes()
		if err != nil {
			return nil, err
		}
		return tables.WithClasses(classes), nil
	case catalogue.FormatLevelLimits, catalogue.FormatSpheres, catalogue.FormatEquipment, catalogue.FormatLanguages,
		catalogue.FormatProficiencyDescriptions:
		classes, err := f.classes()
		if err != nil {
			return nil, err
		}
		limits, err := levelLimits(f.LevelLimits)
		if err != nil {
			return nil, err
		}
		tables = tables.WithClasses(classes).WithLevelLimits(limits)
		if format == catalogue.FormatLevelLimits {
			if f.SphereAccess != nil {
				return nil, errors.New("format 5 carries no sphere_access table")
			}
			return tables, nil
		}
		access, err := sphereAccess(f.SphereAccess)
		if err != nil {
			return nil, err
		}
		tables = tables.WithSphereAccess(access)
		if format == catalogue.FormatSpheres {
			return tables, nil
		}
		tables, err = withEquipment(tables, f, equipment)
		if err != nil || format == catalogue.FormatEquipment {
			return tables, err
		}
		return withLanguages(tables, f, languages)
	default:
		return nil, fmt.Errorf("unknown format %d", format)
	}
}

// withEquipment adds the format-7 equipment data and its two tables (the
// rules are ruletablesvc's); format 7 requires all three.
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

// sphereAccess maps sphere_access to the validated sphere access (the rules
// are ruletablesvc.NewSphereAccess'); format 6 requires the table.
func sphereAccess(rows []sphereAccessDTO) (*ruletablesvc.SphereAccess, error) {
	if len(rows) == 0 {
		return nil, errors.New("format 6 needs the sphere_access table")
	}
	out := make([]ruletablesvc.SphereAccessRow, len(rows))
	for i, r := range rows {
		out[i] = ruletablesvc.SphereAccessRow(r)
	}
	a, err := ruletablesvc.NewSphereAccess(out)
	if err != nil {
		return nil, fmt.Errorf("sphere_access table: %w", err)
	}
	return a, nil
}

// levelLimits maps race_level_limits to validated level limits (the rules
// are ruletablesvc.NewLevelLimits'); the table is required.
func levelLimits(rows []levelLimitDTO) (*ruletablesvc.LevelLimits, error) {
	if rows == nil {
		return nil, errors.New("format 5 needs the race_level_limits table")
	}
	out := make([]ruletablesvc.LevelLimitRow, len(rows))
	for i, r := range rows {
		out[i] = ruletablesvc.LevelLimitRow{Race: r.Race, Class: r.Class, Unlimited: r.MaxLevel == nil}
		if r.MaxLevel != nil {
			out[i].MaxLevel = *r.MaxLevel
		}
	}
	limits, err := ruletablesvc.NewLevelLimits(out)
	if err != nil {
		return nil, fmt.Errorf("race_level_limits table: %w", err)
	}
	return limits, nil
}
