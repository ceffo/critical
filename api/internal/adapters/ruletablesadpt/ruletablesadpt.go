// Package ruletablesadpt reads the PHB rule tables into a
// ruletablesvc.Tables from the catalogue artifact's rule_tables (ADR 0021
// step 9, through Parse). It only holds the private JSON DTOs and the
// mapping; the entities and their validation live in ruletablesvc.
package ruletablesadpt

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

type fileDTO struct {
	Strength  []strengthDTO `json:"strength"`
	Dexterity []dexDTO      `json:"dexterity"`
	Bard      []bardDTO     `json:"bard_spells"`
	XP        []xpDTO       `json:"bard_experience"`
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

// Parse maps the artifact's rule_tables JSON to validated Tables. A
// violation names the table.
func Parse(raw []byte) (*ruletablesvc.Tables, error) {
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
	return ruletablesvc.New(str, dex, bard, xp)
}
