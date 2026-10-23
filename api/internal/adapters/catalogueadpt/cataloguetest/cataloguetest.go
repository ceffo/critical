// Package cataloguetest loads the fixture catalogue (api/testdata/catalogue.json)
// for tests: its rule tables and its class, group and race definitions (ADR
// 0040 step 5), parsed as the composition root parses the real one.
package cataloguetest

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// Path is the fixture catalogue's path.
func Path() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "testdata", "catalogue.json")
}

// Artifact is the fixture catalogue.
func Artifact(t testing.TB) catalogueadpt.Artifact {
	t.Helper()
	a, err := catalogueadpt.LoadArtifact(Path())
	if err != nil {
		t.Fatalf("fixture catalogue: %v", err)
	}
	return a
}

// Tables are the fixture's rule tables, with its definitions.
func Tables(t testing.TB) *ruletablesvc.Tables {
	t.Helper()
	a := Artifact(t)
	tables, err := ruletablesadpt.Parse(a.RuleTables, a.Equipment, a.Languages, a.Definitions)
	if err != nil {
		t.Fatalf("fixture rule tables: %v", err)
	}
	if err := tables.CheckPowerKinds(a.Powers); err != nil {
		t.Fatalf("fixture rule tables: %v", err)
	}
	return tables
}

// Definitions are the fixture's class, group and race definitions.
func Definitions(t testing.TB) *classes.Registry {
	t.Helper()
	d, err := Tables(t).Definitions()
	if err != nil {
		t.Fatalf("fixture definitions: %v", err)
	}
	return d
}
