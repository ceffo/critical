// Package catalogueadpt reads the spell catalogue artifact (ADR 0021) from
// the file at CRITICAL_CATALOGUE and validates it (steps 5.1 and 5.2): the
// file must exist, carry a format this API knows, match the committed JSON
// Schema and hold no duplicate id. It only maps the JSON to
// catalogue.Catalogue; the import into the database is the storage layer's.
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

// Load reads and validates the artifact at path. A missing file is an error
// naming the path and `just catalogue` (step 5.1).
func Load(path string) (catalogue.Catalogue, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return catalogue.Catalogue{}, fmt.Errorf(
			"spell catalogue: no file at %s: run `just catalogue` to fetch it, or set CRITICAL_CATALOGUE (the fixture api/testdata/catalogue.json works without access to the rulebooks repo)", path)
	}
	if err != nil {
		return catalogue.Catalogue{}, fmt.Errorf("spell catalogue: read %s: %w", path, err)
	}
	c, err := Parse(raw)
	if err != nil {
		return catalogue.Catalogue{}, fmt.Errorf("spell catalogue %s: %w", path, err)
	}
	return c, nil
}

type spellDTO struct {
	ID           string         `json:"id"`
	Class        string         `json:"class"`
	Level        int            `json:"level"`
	School       string         `json:"school"`
	Sphere       string         `json:"sphere"`
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
}

// Parse validates artifact bytes (step 5.2): an unknown format is refused
// naming it, then the schema is checked (the first offences are listed),
// then ids must be unique.
func Parse(raw []byte) (catalogue.Catalogue, error) {
	var head struct {
		Format json.RawMessage `json:"format"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return catalogue.Catalogue{}, fmt.Errorf("not a JSON object: %w", err)
	}
	if f := strings.TrimSpace(string(head.Format)); f != strconv.Itoa(catalogue.Format) {
		if f == "" {
			f = "none"
		}
		return catalogue.Catalogue{}, fmt.Errorf("unknown format %s: this API reads format %d", f, catalogue.Format)
	}

	sch, err := compiled()
	if err != nil {
		return catalogue.Catalogue{}, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return catalogue.Catalogue{}, fmt.Errorf("parse: %w", err)
	}
	if err := sch.Validate(doc); err != nil {
		return catalogue.Catalogue{}, schemaError(err)
	}

	var a artifactDTO
	if err := json.Unmarshal(raw, &a); err != nil {
		return catalogue.Catalogue{}, fmt.Errorf("parse: %w", err)
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
			// The artifact's plain texts are the English books' (format 1):
			// texts in the fallback language. Its name and description are
			// already language maps (ADR 0023).
			ID: s.ID, Class: s.Class, Level: s.Level, School: localized.Of(s.School), Sphere: localized.Of(s.Sphere),
			CastingTime: localized.Of(s.CastingTime), Range: localized.Of(s.Range), Duration: localized.Of(s.Duration),
			AreaOfEffect: localized.Of(s.AreaOfEffect), Components: localized.Of(s.Components),
			SavingThrow: localized.Of(s.SavingThrow),
			Name:        s.Name.Clean(), Description: s.Description.Clean(), Source: s.Source,
		})
	}
	if len(dups) > 0 {
		return catalogue.Catalogue{}, fmt.Errorf("duplicate spell ids: %s", strings.Join(first(dups), ", "))
	}
	return out, nil
}

// schemaError lists the first leaf violations of a schema validation error.
func schemaError(err error) error {
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
	return fmt.Errorf("does not match the format %d schema: %s", catalogue.Format, strings.Join(first(leaves), "; "))
}

func first(list []string) []string {
	if len(list) <= maxOffences {
		return list
	}
	return append(list[:maxOffences:maxOffences], fmt.Sprintf("and %d more", len(list)-maxOffences))
}
