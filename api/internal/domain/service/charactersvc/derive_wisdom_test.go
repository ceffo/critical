package charactersvc_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// ADR 0006 steps 9.1 and 9.2: the derived magical defence adjustment is PHB
// Table 5 at the sheet's Wisdom (the testdata catalogue's real numbers), for a
// character of any class.
func TestDerive_Step9_MagicDefenceForEveryClass(t *testing.T) {
	artifact, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	tables, err := ruletablesadpt.Parse(artifact.RuleTables, artifact.Equipment, artifact.Languages, artifact.Definitions)
	require.NoError(t, err)
	for _, class := range []string{"fighter", "mage", "cleric", "thief", "bard"} {
		for wis, want := range map[int]int{3: -3, 12: 0, 17: 3, 18: 4} {
			c := &charactersvc.Character{
				Class: class, Race: "human", Str: 12, Dex: 12, Con: 12, Int: 12, Wis: wis, Cha: 12, RealizedLevel: 1,
			}
			d, err := charactersvc.Derive(c, 0, "", tables)
			require.NoError(t, err, "%s Wisdom %d", class, wis)
			assert.Equal(t, want, d.MagicDefence, "%s Wisdom %d", class, wis)
		}
	}
}

// ADR 0006 step 10.1: the derived spell immunities are PHB Table 5's at the
// sheet's Wisdom, cumulative (the testdata catalogue's real column), for a
// character of any class: none at 18, charm person and not hold person at 19,
// hold person at 20.
func TestDerive_Step10_SpellImmunitiesForEveryClass(t *testing.T) {
	artifact, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	tables, err := ruletablesadpt.Parse(artifact.RuleTables, artifact.Equipment, artifact.Languages, artifact.Definitions)
	require.NoError(t, err)
	for _, class := range []string{"fighter", "mage", "cleric", "thief", "bard"} {
		derive := func(wis int) []string {
			c := &charactersvc.Character{
				Class: class, Race: "human", Str: 12, Dex: 12, Con: 12, Int: 12, Wis: wis, Cha: 12, RealizedLevel: 1,
			}
			d, err := charactersvc.Derive(c, 0, "", tables)
			require.NoError(t, err, "%s Wisdom %d", class, wis)
			return d.SpellImmunities
		}
		assert.Empty(t, derive(18), "%s Wisdom 18", class)
		assert.Contains(t, derive(19), "charm_person", "%s Wisdom 19", class)
		assert.NotContains(t, derive(19), "hold_person", "%s Wisdom 19", class)
		assert.Contains(t, derive(20), "hold_person", "%s Wisdom 20", class)
	}
}
