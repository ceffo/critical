package ruletablesadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
)

func TestFromArtifactRefusesAnArtifactWithoutRuleTablesNamingTheSource(t *testing.T) {
	_, err := ruletablesadpt.FromArtifact(catalogueadpt.Artifact{}, "cat.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spell catalogue cat.json")
}
