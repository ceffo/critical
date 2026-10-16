package postgres

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dataStatement matches a data statement's keyword; lineComment is a "--" comment to the end of
// its line (the migrations use no block comment and no string holding "--").
var (
	dataStatement = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE)\b`)
	lineComment   = regexp.MustCompile(`--[^\n]*`)
)

// ADR 0025 step 2.1: no migration holds data, only schema.
func TestMigrations_Step2_1_SchemaOnly(t *testing.T) {
	entries, err := migrationFiles.ReadDir("migrations")
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	for _, e := range entries {
		body, err := migrationFiles.ReadFile("migrations/" + e.Name())
		require.NoError(t, err)
		code := lineComment.ReplaceAllString(string(body), "")
		assert.Empty(t, dataStatement.FindAllString(code, -1), "%s holds a data statement", e.Name())
		assert.True(t, strings.HasSuffix(e.Name(), ".sql"), "%s is not a .sql migration", e.Name())
	}
}

// The scan above finds a data statement outside a comment, and only there.
func TestMigrations_DataStatementScanFindsOne(t *testing.T) {
	code := lineComment.ReplaceAllString("-- no INSERT here\nCREATE TABLE t (id BIGINT);\ninsert into t values (1);", "")
	assert.Equal(t, []string{"insert"}, dataStatement.FindAllString(code, -1))
}
