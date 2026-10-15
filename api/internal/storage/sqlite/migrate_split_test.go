package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want []string
	}{
		{"two statements", "A; B;", []string{"A", "B"}},
		{"semicolon in line comment", "A -- x; y\n; B;", []string{"A -- x; y", "B"}},
		{"semicolon in block comment", "A /* x; y */ ; B;", []string{"A /* x; y */", "B"}},
		{"multi-line block comment", "/* a;\nb; */ A; B", []string{"/* a;\nb; */ A", "B"}},
		{"semicolon in literal", "INSERT 'a;b'; B;", []string{"INSERT 'a;b'", "B"}},
		{"escaped quote in literal", "INSERT 'it''s; ok'; B;", []string{"INSERT 'it''s; ok'", "B"}},
		{"comment marker in literal", "INSERT '--; /*'; B;", []string{"INSERT '--; /*'", "B"}},
		{"quote in comment", "A -- don't\n; B;", []string{"A -- don't", "B"}},
		{"trailing statement without semicolon", "A; B", []string{"A", "B"}},
		{"empty statements skipped", ";; A;\n ;  ; B;;", []string{"A", "B"}},
		{"empty input", " \n ", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, splitStatements(tc.sql))
		})
	}
}
