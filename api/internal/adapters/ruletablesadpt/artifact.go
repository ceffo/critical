package ruletablesadpt

import (
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// FromArtifact parses the PHB rule tables the artifact carries (ADR 0021
// step 9.4), and a format-4 artifact's class and race tables (step 10, which
// the server needs: a format-3 artifact refuses the start), with
// the table rules of steps 9.3 and 10.3 (a missing or broken table refuses
// the start, naming it), at every start, whatever catalogue version the
// database recorded (step 9.5). source names the artifact in the errors.
// The seeds hold no rule tables (step 9.6.3).
func FromArtifact(artifact catalogueadpt.Artifact, source string) (*ruletablesvc.Tables, error) {
	t, err := Parse(artifact.RuleTables, artifact.Equipment, artifact.Languages, artifact.Definitions)
	if err != nil {
		return nil, fmt.Errorf("spell catalogue %s: rule_tables: %w", source, err)
	}
	// The class, group and race definitions (ADR 0040 step 8): every class is
	// read from them, so an artifact without them cannot serve a character.
	if _, err := t.Definitions(); err != nil {
		return nil, fmt.Errorf("spell catalogue %s: %w", source, err)
	}
	// Every character's level, THAC0 and saves come from the class tables
	// (ADR 0030 steps 3 to 6): an artifact without them cannot serve one.
	if _, ok := t.Classes(); !ok {
		return nil, fmt.Errorf("spell catalogue %s: %w", source, ruletablesvc.ErrNoClassTables)
	}
	// ADR 0044 step 1.1: Table 14 names attack and defence modes only, and
	// every mode has its cells; a devotion where a mode belongs refuses the start.
	if err := t.CheckPowerKinds(artifact.Powers); err != nil {
		return nil, fmt.Errorf("spell catalogue %s: rule_tables: %w", source, err)
	}
	return t, nil
}
