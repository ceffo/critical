package skills_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slotsImport is the owner of the proficiency pools (ADR 0046 step 4).
const slotsImport = "github.com/ceffo/critical/api/internal/domain/slots"

// removedPoolNames are the pool names the service packages carried before ADR 0046 step 4.5
// removed them; a service that grew one again would be a second owner.
var removedPoolNames = []string{
	"SlotRow", "NonweaponPool", "WeaponPool", "NonweaponSlots", "WeaponSlots", "ExtraSlots", "SlotsGainedAt",
}

// ADR 0046 step 4.1: skills reads Table 34's row, the pools, the extra slots and what a level
// grants from domain/slots, never from a service package. The skills sources (test files left
// out) are parsed: no selector into an import under domain/service names one of slots' exported
// names (read from the slots sources, so a name added to the owner later is covered) or one of
// the names the step removed; and skills imports slots. The rest of ruletablesvc (languages,
// skill points, specialization) is still read from there (ADR 0046 step 4.1 as built).
func TestSlots_Step4_SkillsReadsNoPoolFromAService(t *testing.T) {
	pool := map[string]bool{}
	for _, n := range removedPoolNames {
		pool[n] = true
	}
	for _, f := range parseDir(t, filepath.Join("..", "slots")) {
		for _, name := range exportedNames(f) {
			pool[name] = true
		}
	}
	require.Contains(t, pool, "Nonweapon", "slots' exported names were read")

	importsSlots := false
	fset := token.NewFileSet()
	for _, f := range parseDirWith(t, fset, ".") {
		services := map[string]bool{} // local import name -> under domain/service
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			require.NoError(t, err)
			if p == slotsImport {
				importsSlots = true
			}
			if !strings.Contains(p, "/internal/domain/service/") {
				continue
			}
			name := filepath.Base(p)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			services[name] = true
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && services[x.Name] && pool[sel.Sel.Name] {
				t.Errorf("%s: skills reads %s.%s from a service; the pools' owner is domain/slots",
					fset.Position(sel.Pos()), x.Name, sel.Sel.Name)
			}
			return true
		})
	}
	assert.True(t, importsSlots, "skills reads the pools from domain/slots")
}

// exportedNames are f's exported top-level types, functions, constants and variables.
func exportedNames(f *ast.File) []string {
	var out []string
	add := func(id *ast.Ident) {
		if id.IsExported() {
			out = append(out, id.Name)
		}
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				add(d.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					add(s.Name)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						add(n)
					}
				}
			}
		}
	}
	return out
}

// parseDir parses the non-test Go sources of dir.
func parseDir(t *testing.T, dir string) map[string]*ast.File {
	t.Helper()
	return parseDirWith(t, token.NewFileSet(), dir)
}

// parseDirWith parses the non-test Go sources of dir into fset, by path; at least one.
func parseDirWith(t *testing.T, fset *token.FileSet, dir string) map[string]*ast.File {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	require.NoError(t, err)
	out := map[string]*ast.File{}
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		require.NoError(t, err)
		out[p] = f
	}
	require.NotEmpty(t, out, "no source parsed in %s", dir)
	return out
}
