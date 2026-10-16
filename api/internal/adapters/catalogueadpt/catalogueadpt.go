// Package catalogueadpt reads the spell catalogue artifact (ADR 0021) from
// the file at CRITICAL_CATALOGUE and validates it (steps 5.1 and 5.2): the
// file must exist, carry a format this API knows, match the committed JSON
// Schema and hold no duplicate id. It only maps the JSON to
// catalogue.Catalogue; the import into the database is the storage layer's.
//
// The schema is format 2's. A format 1 artifact is still read: it is the
// same but for its six stat texts, plain English strings, which are read as
// format 2 stats with no French text ({"en": text}) before validation.
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

type localized struct {
	EN string `json:"en"`
	FR string `json:"fr"`
}

type spellDTO struct {
	ID           string    `json:"id"`
	Class        string    `json:"class"`
	Level        int       `json:"level"`
	School       string    `json:"school"`
	Sphere       string    `json:"sphere"`
	CastingTime  localized `json:"casting_time"`
	Range        localized `json:"range"`
	Duration     localized `json:"duration"`
	AreaOfEffect localized `json:"area_of_effect"`
	Components   localized `json:"components"`
	SavingThrow  localized `json:"saving_throw"`
	Name         localized `json:"name"`
	Description  localized `json:"description"`
	Source       string    `json:"source"`
}

// statFields are the six stat texts: plain strings in format 1, localized
// objects since format 2.
var statFields = []string{"casting_time", "range", "duration", "area_of_effect", "components", "saving_throw"}

type artifactDTO struct {
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
	format := catalogue.Format
	switch f := strings.TrimSpace(string(head.Format)); f {
	case strconv.Itoa(catalogue.Format):
	case strconv.Itoa(catalogue.Format1):
		format = catalogue.Format1
		up, err := upgradeFormat1(raw)
		if err != nil {
			return catalogue.Catalogue{}, err
		}
		raw = up
	default:
		if f == "" {
			f = "none"
		}
		return catalogue.Catalogue{}, fmt.Errorf("unknown format %s: this API reads formats %d and %d", f,
			catalogue.Format1, catalogue.Format)
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
	out := catalogue.Catalogue{Format: format, Version: a.Version, Spells: make([]catalogue.Spell, 0, len(a.Spells))}
	seen := make(map[string]bool, len(a.Spells))
	var dups []string
	for _, s := range a.Spells {
		if seen[s.ID] {
			dups = append(dups, s.ID)
			continue
		}
		seen[s.ID] = true
		out.Spells = append(out.Spells, catalogue.Spell{
			ID: s.ID, Class: s.Class, Level: s.Level, School: s.School, Sphere: s.Sphere,
			CastingTime: s.CastingTime.EN, Range: s.Range.EN, Duration: s.Duration.EN,
			AreaOfEffect: s.AreaOfEffect.EN, Components: s.Components.EN, SavingThrow: s.SavingThrow.EN,
			StatsFR: catalogue.StatsFR{
				CastingTime: s.CastingTime.FR, Range: s.Range.FR, Duration: s.Duration.FR,
				AreaOfEffect: s.AreaOfEffect.FR, Components: s.Components.FR, SavingThrow: s.SavingThrow.FR,
			},
			Name: s.Name.EN, NameFR: s.Name.FR, Description: s.Description.EN, DescriptionFR: s.Description.FR,
			Source: s.Source,
		})
	}
	if len(dups) > 0 {
		return catalogue.Catalogue{}, fmt.Errorf("duplicate spell ids: %s", strings.Join(first(dups), ", "))
	}
	return out, nil
}

// upgradeFormat1 rewrites a format 1 artifact as format 2: every stat value
// v of every spell becomes {"en": v}, and the format 2. Anything else is
// left as it is, for the schema to judge; a value that was not a string
// fails it there.
func upgradeFormat1(raw []byte) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("not a JSON object: %w", err)
	}
	var spells []map[string]json.RawMessage
	if json.Unmarshal(doc["spells"], &spells) != nil {
		return raw, nil // not a list of objects: the schema names the offence
	}
	for _, sp := range spells {
		for _, k := range statFields {
			if v, ok := sp[k]; ok {
				sp[k] = json.RawMessage(`{"en":` + string(v) + `}`)
			}
		}
	}
	list, err := json.Marshal(spells)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	doc["spells"] = list
	doc["format"] = json.RawMessage(strconv.Itoa(catalogue.Format))
	return json.Marshal(doc)
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
