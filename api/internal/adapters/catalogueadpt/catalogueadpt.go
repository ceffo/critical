// Package catalogueadpt reads the spell catalogue artifact (ADR 0021) from
// the file at CRITICAL_CATALOGUE and validates it (steps 5.1 and 5.2): the
// file must exist, carry the one format this API knows (11, step 18.3; the
// formats 1 to 10 are retired and refused like any unknown format), match the
// committed JSON Schema and hold no duplicate id. It maps the spells to catalogue.Catalogue
// and hands the artifact's rule tables (the PHB rule tables, from format 4
// the class and race tables, from format 5 the racial level limits, in format 6 the sphere access) on as the
// schema-checked JSON (Artifact.RuleTables): their table rules (steps 9.3,
// 10.3 and 11.3) are ruletablesadpt's, the one reader of
// rule-table JSON. The import into
// the database is the storage layer's.
package catalogueadpt

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// schemaJSON is a copy of schemas/catalogue.schema.json (a test keeps the
// two identical): the binary validates without the repo at hand.
//
//go:embed catalogue.schema.json
var schemaJSON []byte

// maxOffences caps the schema violations listed in a refusal.
const maxOffences = 5

var (
	schemaOnce sync.Once
	schema     *jsonschema.Schema
	errSchema  error
)

func compiled() (*jsonschema.Schema, error) {
	schemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
		if err != nil {
			errSchema = fmt.Errorf("catalogue schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource("catalogue.schema.json", doc); err != nil {
			errSchema = fmt.Errorf("catalogue schema: %w", err)
			return
		}
		schema, errSchema = c.Compile("catalogue.schema.json")
	})
	return schema, errSchema
}

// Artifact is one parsed artifact: its spells and its rule_tables object as
// written (shape checked by the schema, step 9.2). The caller parses the
// tables with ruletablesadpt.Parse, which applies the rules of step 9.3.
// Equipment is a format-7 artifact's top-level equipment object as written
// (ADR 0021 step 13), nil before format 7; ruletablesadpt reads it too.
type Artifact struct {
	Catalogue  catalogue.Catalogue
	RuleTables json.RawMessage
	Equipment  json.RawMessage
	// Languages is a format-8 artifact's top-level languages list as written
	// (ADR 0021 step 14), nil before format 8; ruletablesadpt reads it too.
	Languages json.RawMessage
}

// Load reads and validates the artifact at path and returns its spells.
func Load(path string) (catalogue.Catalogue, error) {
	a, err := LoadArtifact(path)
	return a.Catalogue, err
}

// LoadArtifact reads and validates the artifact at path. A missing file is
// an error naming the path and `just catalogue` (step 5.1).
func LoadArtifact(path string) (Artifact, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return Artifact{}, fmt.Errorf(
			"spell catalogue: no file at %s: run `just catalogue` to fetch it, or set CRITICAL_CATALOGUE (the fixture api/testdata/catalogue.json works without access to the rulebooks repo)", path)
	}
	if err != nil {
		return Artifact{}, fmt.Errorf("spell catalogue: read %s: %w", path, err)
	}
	a, err := ParseArtifact(raw)
	if err != nil {
		return Artifact{}, fmt.Errorf("spell catalogue %s: %w", path, err)
	}
	return a, nil
}

type spellDTO struct {
	ID           string         `json:"id"`
	Class        string         `json:"class"`
	Level        int            `json:"level"`
	School       string         `json:"school"`
	Sphere       string         `json:"sphere"`
	Spheres      []string       `json:"spheres"`
	CastingTime  string         `json:"casting_time"`
	Range        string         `json:"range"`
	Duration     string         `json:"duration"`
	AreaOfEffect string         `json:"area_of_effect"`
	Components   string         `json:"components"`
	SavingThrow  string         `json:"saving_throw"`
	Name         localized.Text `json:"name"`
	Description  localized.Text `json:"description"`
	Source       string         `json:"source"`
}

type artifactDTO struct {
	Format  int        `json:"format"`
	Version string     `json:"version"`
	Spells  []spellDTO `json:"spells"`
	// RuleTables (step 9.2) has the keys and rows ruletablesadpt.Parse reads.
	RuleTables json.RawMessage `json:"rule_tables"`
	// Equipment (format 7, step 13) has the keys and rows ruletablesadpt reads.
	Equipment json.RawMessage `json:"equipment"`
	// Languages (format 8, step 14) is the language catalogue ruletablesadpt reads.
	Languages json.RawMessage `json:"languages"`
	// ProficiencyDescriptions (format 9, step 16) are the non-weapon proficiencies' texts.
	ProficiencyDescriptions []proficiencyDescriptionDTO `json:"proficiency_descriptions"`
}

type proficiencyDescriptionDTO struct {
	ID          string         `json:"id"`
	Description localized.Text `json:"description"`
}

// Parse validates artifact bytes and returns the spells (see ParseArtifact).
func Parse(raw []byte) (catalogue.Catalogue, error) {
	a, err := ParseArtifact(raw)
	return a.Catalogue, err
}

// ParseArtifact validates artifact bytes (step 5.2): an unknown format (a
// retired format 1 included, step 9.6.3) is refused naming it and `just
// catalogue`, then the schema is checked (the
// first offences are listed, the artifact's tables included), then
// ids must be unique.
func ParseArtifact(raw []byte) (Artifact, error) {
	var head struct {
		Format json.RawMessage `json:"format"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return Artifact{}, fmt.Errorf("not a JSON object: %w", err)
	}
	format, known := knownFormat(strings.TrimSpace(string(head.Format)))
	if !known {
		return Artifact{}, fmt.Errorf(
			"unknown format %s: this API reads %s; run `just catalogue` to fetch a current artifact", format, formatList())
	}

	sch, err := compiled()
	if err != nil {
		return Artifact{}, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return Artifact{}, fmt.Errorf("parse: %w", err)
	}
	if err := sch.Validate(doc); err != nil {
		return Artifact{}, schemaError(format, err)
	}

	var a artifactDTO
	if err := json.Unmarshal(raw, &a); err != nil {
		return Artifact{}, fmt.Errorf("parse: %w", err)
	}
	out := catalogue.Catalogue{Format: a.Format, Version: a.Version, Spells: make([]catalogue.Spell, 0, len(a.Spells))}
	seen := make(map[string]bool, len(a.Spells))
	var dups []string
	for _, s := range a.Spells {
		if seen[s.ID] {
			dups = append(dups, s.ID)
			continue
		}
		seen[s.ID] = true
		out.Spells = append(out.Spells, catalogue.Spell{
			// The artifact's plain texts are the English books' (step 1):
			// texts in the fallback language. Its name and description are
			// already language maps (ADR 0023).
			ID: s.ID, Class: s.Class, Level: s.Level, School: localized.Of(s.School), Sphere: localized.Of(s.Sphere),
			Spheres:     append([]string(nil), s.Spheres...),
			CastingTime: localized.Of(s.CastingTime), Range: localized.Of(s.Range), Duration: localized.Of(s.Duration),
			AreaOfEffect: localized.Of(s.AreaOfEffect), Components: localized.Of(s.Components),
			SavingThrow: localized.Of(s.SavingThrow),
			Name:        s.Name.Clean(), Description: s.Description.Clean(), Source: s.Source,
		})
	}
	if len(dups) > 0 {
		return Artifact{}, fmt.Errorf("duplicate spell ids: %s", strings.Join(first(dups), ", "))
	}
	seenProf := make(map[string]bool, len(a.ProficiencyDescriptions))
	out.ProficiencyDescriptions = make([]catalogue.ProficiencyDescription, 0, len(a.ProficiencyDescriptions))
	for _, p := range a.ProficiencyDescriptions {
		if seenProf[p.ID] {
			dups = append(dups, p.ID)
			continue
		}
		seenProf[p.ID] = true
		out.ProficiencyDescriptions = append(out.ProficiencyDescriptions,
			catalogue.ProficiencyDescription{ID: p.ID, Description: p.Description.Clean()})
	}
	if len(dups) > 0 {
		return Artifact{}, fmt.Errorf("duplicate proficiency description ids: %s", strings.Join(first(dups), ", "))
	}
	return Artifact{Catalogue: out, RuleTables: a.RuleTables, Equipment: a.Equipment, Languages: a.Languages}, nil
}

// knownFormat reports whether the raw format value is one this API reads,
// and the value as named in a refusal ("none" when absent).
func knownFormat(raw string) (string, bool) {
	if raw == "" {
		return "none", false
	}
	for _, f := range catalogue.Formats {
		if raw == strconv.Itoa(f) {
			return raw, true
		}
	}
	return raw, false
}

// formatList names the formats this API reads: "format 3", or "formats 1
// and 3" when it reads several.
func formatList() string {
	names := make([]string, len(catalogue.Formats))
	for i, f := range catalogue.Formats {
		names[i] = strconv.Itoa(f)
	}
	if len(names) == 1 {
		return "format " + names[0]
	}
	return "formats " + strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// schemaError lists the first leaf violations of a schema validation error.
func schemaError(format string, err error) error {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return fmt.Errorf("schema: %w", err)
	}
	var leaves []string
	var walk func(*jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			leaves = append(leaves, e.Error())
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return fmt.Errorf("does not match the format %s schema: %s", format, strings.Join(first(leaves), "; "))
}

func first(list []string) []string {
	if len(list) <= maxOffences {
		return list
	}
	return append(list[:maxOffences:maxOffences], fmt.Sprintf("and %d more", len(list)-maxOffences))
}
