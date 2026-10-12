package weaponsadpt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/weaponsadpt"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeSeeds(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, weaponsadpt.FileName), []byte(content), 0o600))
	return dir
}

func TestNew_RepoSeedKeepsOriginalWeapons(t *testing.T) {
	c, err := weaponsadpt.New(filepath.Join("..", "..", "..", "..", "seeds"))
	require.NoError(t, err)
	w, ok := c.Get("longsword")
	require.True(t, ok)
	assert.Equal(t, combatsvc.Weapon{ID: "longsword", Name: "Longsword", BaseDamage: "1d8"}, w)
	w, ok = c.Get("shortbow")
	require.True(t, ok)
	assert.Equal(t, combatsvc.Weapon{ID: "shortbow", Name: "Shortbow", BaseDamage: "1d6"}, w)
}

func TestNew_ThirdWeaponNeedsNoCodeChange(t *testing.T) {
	dir := writeSeeds(t, `[{"id":"dagger","name":"Dagger","base_damage":"1d4"}]`)
	c, err := weaponsadpt.New(dir)
	require.NoError(t, err)
	w, ok := c.Get("dagger")
	require.True(t, ok)
	assert.Equal(t, "1d4", w.BaseDamage)
	_, ok = c.Get("nope")
	assert.False(t, ok)
}

func TestNew_Errors(t *testing.T) {
	tests := map[string]string{
		"invalid json":    `{not json`,
		"missing field":   `[{"id":"x","name":"X"}]`,
		"duplicate id":    `[{"id":"x","name":"X","base_damage":"1d4"},{"id":"x","name":"Y","base_damage":"1d6"}]`,
		"wrong top-level": `{"id":"x"}`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := weaponsadpt.New(writeSeeds(t, content))
			require.Error(t, err)
		})
	}
}

func TestNew_MissingFile(t *testing.T) {
	_, err := weaponsadpt.New(t.TempDir())
	require.ErrorContains(t, err, "weapons.json")
}
