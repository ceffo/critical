package ruletablesadpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ProficienciesFile is the seed file of the non-weapon catalogue and PHB
// Table 38's crossovers and Table 34's slots (ADR 0037 steps 1 and 2), in the seeds directory.
const ProficienciesFile = "proficiencies.json"

// proficiencyDTO is one catalogue row: ability and modifier are absent for a
// proficiency with no check.
type proficiencyDTO struct {
	ID       string   `json:"id"`
	Ability  string   `json:"ability"`
	Modifier int      `json:"modifier"`
	Slots    int      `json:"slots"`
	Groups   []string `json:"groups"`
}

type crossoverDTO struct {
	Class  string   `json:"class"`
	Groups []string `json:"groups"`
}

// slotRowDTO is one row of PHB Table 34 (ADR 0037 step 1).
type slotRowDTO struct {
	Group            string `json:"group"`
	WeaponInitial    int    `json:"weaponInitial"`
	WeaponLevels     int    `json:"weaponLevels"`
	Penalty          int    `json:"penalty"`
	NonweaponInitial int    `json:"nonweaponInitial"`
	NonweaponLevels  int    `json:"nonweaponLevels"`
}

type proficienciesFileDTO struct {
	Slots      []slotRowDTO     `json:"slots"`
	Crossovers []crossoverDTO   `json:"crossovers"`
	Nonweapon  []proficiencyDTO `json:"nonweapon"`
}

// ReadProficiencies reads and validates seeds/proficiencies.json (the rules
// are ruletablesvc.NewProficiencies'); the file is required.
func ReadProficiencies(seedsDir string, defs *classes.Registry) (*ruletablesvc.Proficiencies, error) {
	raw, err := os.ReadFile(filepath.Join(filepath.Clean(seedsDir), ProficienciesFile))
	if err != nil {
		return nil, fmt.Errorf("proficiencies: %w", err)
	}
	return ParseProficiencies(raw, defs)
}

// ParseProficiencies maps the JSON of ProficienciesFile to the catalogue.
func ParseProficiencies(raw []byte, defs *classes.Registry) (*ruletablesvc.Proficiencies, error) {
	var f proficienciesFileDTO
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("proficiencies: parse: %w", err)
	}
	list := make([]ruletablesvc.Proficiency, len(f.Nonweapon))
	for i, p := range f.Nonweapon {
		list[i] = ruletablesvc.Proficiency(p)
	}
	cross := make([]ruletablesvc.Crossover, len(f.Crossovers))
	for i, c := range f.Crossovers {
		cross[i] = ruletablesvc.Crossover(c)
	}
	slots := make([]ruletablesvc.SlotRow, len(f.Slots))
	for i, r := range f.Slots {
		slots[i] = ruletablesvc.SlotRow(r)
	}
	return ruletablesvc.NewProficiencies(list, cross, slots, defs)
}
