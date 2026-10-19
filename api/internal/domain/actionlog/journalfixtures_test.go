package actionlog_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/journalfixture"
)

// declaredKinds are the values of this package's Kind constants, read from the
// source so that a kind added without a fixture is found without a list to
// keep up to date.
func declaredKinds(t *testing.T) []actionlog.Kind {
	t.Helper()
	names, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	var kinds []actionlog.Kind
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, s := range gd.Specs {
				vs := s.(*ast.ValueSpec)
				if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "Kind" {
					continue
				}
				for _, v := range vs.Values {
					lit, ok := v.(*ast.BasicLit)
					require.True(t, ok, "a Kind constant must be a string literal")
					val, err := strconv.Unquote(lit.Value)
					require.NoError(t, err)
					kinds = append(kinds, actionlog.Kind(val))
				}
			}
		}
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	return kinds
}

// Every log kind has a web journal fixture file and every fixture file is a
// log kind, so the journal's per-kind tests cover what the server can log.
func TestEveryLogKindHasAJournalFixture(t *testing.T) {
	declared := declaredKinds(t)
	require.NotEmpty(t, declared)
	require.Equal(t, declared, journalfixture.Kinds(t),
		"web/src/journal/fixtures/<kind>.json and the actionlog Kind constants differ")
}

// A fixture marks a roll as the server does (GET /api/log: actionlog.IsRoll).
func TestJournalFixtureRollFlagIsTheServers(t *testing.T) {
	journalfixture.RollFlagsMatch(t)
}
