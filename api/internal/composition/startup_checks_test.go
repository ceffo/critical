package composition_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
)

// A spell effect whose id the catalogue does not hold refuses the startup,
// naming every such id, before anything is imported.
func TestStartup_RefusesSpellEffectsTheCatalogueDoesNotHold(t *testing.T) {
	seeds := t.TempDir()
	require.NoError(t, os.CopyFS(seeds, os.DirFS(filepath.Join("..", "..", "testdata", "seeds"))))
	path := filepath.Join(seeds, "spell_effects.json")
	raw, err := os.ReadFile(path) //nolint:gosec // test temp dir
	require.NoError(t, err)
	var entries []json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &entries))
	for id, name := range map[string]string{"no_such_spell": "No Such Spell", "magic_rocket": "Magic Rocket"} {
		entries = append(entries, json.RawMessage(`{"id":"`+id+`","name":"`+name+`","level":1,"kind":"utility",`+
			`"confidence":"high","target":"none","duration":{"kind":"instant"}}`))
	}
	raw, err = json.Marshal(entries)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	_, err = composition.New(context.Background(), composition.Config{
		DSN:       filepath.Join(t.TempDir(), "critical.db"),
		SeedsDir:  seeds,
		Catalogue: filepath.Join("..", "..", "testdata", "catalogue.json"),
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "spell effects: ids not in the spell catalogue: ")
	assert.ErrorContains(t, err, "no_such_spell")
	assert.ErrorContains(t, err, "magic_rocket")
}
