package classes_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// thresholdNames are the names a second owner of the experience tables answered by before ADR
// 0046 step 5 (the rule tables' and charactersvc's XPThreshold, xpsvc's Thresholds port).
var thresholdNames = map[string]bool{"XPThreshold": true, "Thresholds": true}

// ADR 0046 step 5: the class definition is the one owner of a level's threshold
// (classes.Class.XPThreshold). Every non-test Go source under internal outside this package is
// parsed (generated mocks included, so a mocked port is caught too): none declares a function, a
// method, an interface method or a type named XPThreshold or Thresholds. A threshold is read by
// calling the definition's method (def.XPThreshold), never re-served by a service or a port.
// BardXPThreshold, the rulebook-check helper ADR 0046 step 5.2 keeps, is another name.
func TestLevels_Step5_NoThresholdOutsideClasses(t *testing.T) {
	root := filepath.Join("..", "..")
	self, err := filepath.Abs(".")
	require.NoError(t, err)
	fset := token.NewFileSet()
	parsed := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			if abs == self || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		parsed++
		ast.Inspect(f, func(n ast.Node) bool {
			var name *ast.Ident
			switch n := n.(type) {
			case *ast.FuncDecl:
				name = n.Name
			case *ast.TypeSpec:
				name = n.Name
			case *ast.InterfaceType:
				for _, m := range n.Methods.List {
					for _, id := range m.Names {
						if thresholdNames[id.Name] {
							t.Errorf("%s: an interface asks for %s; a threshold is the class definition's",
								fset.Position(id.Pos()), id.Name)
						}
					}
				}
			}
			if name != nil && thresholdNames[name.Name] {
				t.Errorf("%s: %s is declared outside domain/classes; a threshold is the class definition's",
					fset.Position(name.Pos()), name.Name)
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)
	assert.Greater(t, parsed, 100, "the sources under internal were parsed")
}
