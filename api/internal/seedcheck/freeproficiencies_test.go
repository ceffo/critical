package seedcheck_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// ADR 0037 step 22.1: the real artifact's bard lists free_proficiencies at level 1 with exactly
// reading_writing, local_history, singing and musical_instrument (PHB p.41-42), no other class lists
// the kind, and the seed's non-weapon catalogue holds the four (the start would refuse it else).
func TestFreeProficiencies_Step22_1_TheBardListsItsFour(t *testing.T) {
	realCatalogue(t)
	real, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	tables, err := ruletablesadpt.Parse(real.RuleTables, real.Equipment, real.Languages, real.Definitions)
	require.NoError(t, err)
	defs, err := tables.Definitions()
	require.NoError(t, err)
	for _, c := range defs.Classes() {
		listed := c.ListsAll(classes.KindFreeProficiencies)
		if c.ID != "bard" {
			assert.Empty(t, listed, "%s lists no free proficiency", c.ID)
			continue
		}
		require.Len(t, listed, 1)
		assert.Equal(t, 1, listed[0].FromLevel)
		assert.Equal(t, []string{"reading_writing", "local_history", "singing", "musical_instrument"}, listed[0].FreeProficiencies)
	}
	_, err = ruletablesadpt.ReadProficiencies(seedsDir, defs)
	require.NoError(t, err, "the seed holds the four")
}
