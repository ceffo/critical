package spellbookadpt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// ADR 0030 step 20.7: the pre-ticked spells are data, checked at start-up. A class without a
// spellbook, a spell the catalogue lacks (rather than a pre-tick silently dropped), a priest spell
// or a spell listed twice is an error.
func TestParseStartingSeed_Step20_7_CheckedAgainstTheCatalogue(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	for _, tc := range []struct {
		name, seed, wantErr string
	}{
		{"ok", `{"mage":["fireball","mirror_image"],"bard":[]}`, ""},
		{"a class without a spellbook", `{"fighter":["fireball"]}`, "keeps no spellbook"},
		{"not in the catalogue", `{"mage":["read_magic"]}`, "not a Wizard spell"},
		{"a priest spell", `{"mage":["cure_light_wounds"]}`, "not a Wizard spell"},
		{"twice", `{"mage":["fireball","fireball"]}`, "duplicate"},
		{"not an object", `["fireball"]`, "parse"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := spellbookadpt.ParseStartingSeed([]byte(tc.seed), cat, defs)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// ADR 0050 steps 6 and 9: a specialist's pre-ticked spells are data like the mage's, and one of a
// school in his opposition (PHB Table 22: the illusionist's are necromancy, invocation/evocation
// and abjuration) is a start-up error, since the creation would refuse it; a spell of his school
// (Mirror Image, an illusion) or of a school he does not oppose (Fireball for a transmuter, who
// opposes abjuration and necromancy) is accepted.
func TestParseStartingSeed_Step9_AnOpposedPreTickStopsTheStart(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	seed, err := spellbookadpt.ParseStartingSeed([]byte(`{"illusionist":["mirror_image"],"transmuter":["fireball"]}`), cat, defs)
	require.NoError(t, err)
	assert.Equal(t, spellbookadpt.StartingSeed{"illusionist": {"mirror_image"}, "transmuter": {"fireball"}}, seed)

	_, err = spellbookadpt.ParseStartingSeed([]byte(`{"illusionist":["mirror_image","fireball"]}`), cat, defs)
	require.ErrorContains(t, err, `illusionist: "fireball" is of a school in the class's opposition`)
}

func TestReadStartingSeed_MissingFile(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	_, err := spellbookadpt.ReadStartingSeed(t.TempDir(), cat, defs)
	require.Error(t, err)
}

// ADR 0030 step 20.7: the creation reads each spell through the spellbook's one catalogue mapping,
// offers the catalogue's wizard spells by level then id, and the seed's pre-ticks in its order.
func TestCreation_Step20_7_ReadsTheCatalogueAndTheSeed(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, spellbookadpt.StartingFileName),
		[]byte(`{"mage":["fireball","mirror_image"]}`), 0o600))
	seed, err := spellbookadpt.ReadStartingSeed(dir, cat, defs)
	require.NoError(t, err)
	c := spellbookadpt.NewCreation(cat, seed)

	sp, ok := c.Spell("fireball")
	assert.True(t, ok)
	assert.Equal(t, spellbook.Spell{Level: 3, Wizard: true, Schools: []string{"invocation_evocation"}}, sp)
	sp, ok = c.Spell("cure_light_wounds")
	assert.True(t, ok)
	assert.False(t, sp.Wizard)
	_, ok = c.Spell("nope")
	assert.False(t, ok)

	assert.Equal(t, []charactersvc.SpellOption{{ID: "mirror_image", Level: 2}, {ID: "fireball", Level: 3}}, c.WizardSpells())
	assert.Equal(t, []string{"fireball", "mirror_image"}, c.StartingSpells("mage"))
	assert.Empty(t, c.StartingSpells("bard"))
}
