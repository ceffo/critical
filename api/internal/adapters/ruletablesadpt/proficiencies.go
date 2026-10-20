package ruletablesadpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ProficienciesFile is the seed file of the non-weapon catalogue and PHB
// Table 38's crossovers (ADR 0037 steps 1 and 2), in the seeds directory.
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

type proficienciesFileDTO struct {
	Crossovers []crossoverDTO   `json:"crossovers"`
	Nonweapon  []proficiencyDTO `json:"nonweapon"`
}

// ReadProficiencies reads and validates seeds/proficiencies.json (the rules
// are ruletablesvc.NewProficiencies'); the file is required.
func ReadProficiencies(seedsDir string) (*ruletablesvc.Proficiencies, error) {
	raw, err := os.ReadFile(filepath.Join(filepath.Clean(seedsDir), ProficienciesFile))
	if err != nil {
		return nil, fmt.Errorf("proficiencies: %w", err)
	}
	return ParseProficiencies(raw)
}

// ParseProficiencies maps the JSON of ProficienciesFile to the catalogue.
func ParseProficiencies(raw []byte) (*ruletablesvc.Proficiencies, error) {
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
	return ruletablesvc.NewProficiencies(list, cross)
}
