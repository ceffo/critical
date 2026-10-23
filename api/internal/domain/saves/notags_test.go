package saves_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ADR 0046 step 6.1 and 6.4: an owner's figure is a domain value, so no
// struct of an owner package carries a json tag; the wire shape is the
// transport side's mapping. The owners' sources are parsed (test files
// left out) rather than reflected, so every struct is covered, a type added
// later and an anonymous one included, with no list of types to keep up.
// Each owner package is one directory, relative to this one.
func TestFigures_Step6_OwnersCarryNoTransportTags(t *testing.T) {
	owners := []string{"."} // saves
	for _, dir := range owners {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		require.NoError(t, err)
		fset := token.NewFileSet()
		parsed := 0
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			src, err := os.ReadFile(path)
			require.NoError(t, err)
			file, err := parser.ParseFile(fset, path, src, 0)
			require.NoError(t, err)
			parsed++
			ast.Inspect(file, func(n ast.Node) bool {
				st, ok := n.(*ast.StructType)
				if !ok {
					return true
				}
				for _, field := range st.Fields.List {
					if field.Tag == nil {
						continue
					}
					tag, err := strconv.Unquote(field.Tag.Value)
					require.NoError(t, err)
					if _, ok := reflect.StructTag(tag).Lookup("json"); ok {
						t.Errorf("%s: a json tag on an owner's struct (%s)", fset.Position(field.Pos()), tag)
					}
				}
				return true
			})
		}
		require.Positive(t, parsed, "no source parsed in %s", dir)
	}
}
