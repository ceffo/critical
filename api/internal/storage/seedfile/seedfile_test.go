package seedfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seed.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// The fixture seeds pass every check, as the server reads them at boot.
func TestRead_FixtureSeeds(t *testing.T) {
	dir := "../../../testdata/seeds"
	c, ok, err := seedfile.ReadCharacter(seedfile.Path(dir, seedfile.CharacterFile))
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "Test Bard", c.Name)
	assert.Equal(t, "half_elf", c.Race)
	_, err = seedfile.ReadWeapons(seedfile.Path(dir, seedfile.WeaponsFile))
	require.NoError(t, err)
	items, err := seedfile.ReadItems(seedfile.Path(dir, seedfile.ItemsFile))
	require.NoError(t, err)
	require.NotEmpty(t, items)
	_, err = seedfile.ReadSkills(seedfile.Path(dir, seedfile.SkillsFile))
	require.NoError(t, err)
}

// A missing file is ErrUnreadable for the files a catalog needs, and nothing
// to seed for the character.
func TestRead_MissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none.json")
	_, ok, err := seedfile.ReadCharacter(missing)
	require.NoError(t, err)
	assert.False(t, ok)
	_, err = seedfile.ReadWeapons(missing)
	require.ErrorIs(t, err, seedfile.ErrUnreadable)
	_, err = seedfile.ReadItems(missing)
	require.ErrorIs(t, err, seedfile.ErrUnreadable)
	_, err = seedfile.ReadSkills(missing)
	require.ErrorIs(t, err, seedfile.ErrUnreadable)
}

func TestRead_Refusals(t *testing.T) {
	cases := []struct {
		name, body, want string
		read             func(string) error
	}{
		{"weapon duplicate id", `[{"id":"a","name":{"en":"A"},"damage_sm":"1d6","damage_l":"1d8"},{"id":"a","name":{"en":"B"},"damage_sm":"1d6","damage_l":"1d8"}]`, "duplicate id", readWeapons},
		{"weapon without damage", `[{"id":"a","name":{"en":"A"}}]`, "are required", readWeapons},
		{"item bad behaviour", `[{"id":"a","name":{"en":"A"},"behaviour":"x"}]`, "behaviour must be", readItems},
		{"item negative count", `[{"id":"a","name":{"en":"A"},"behaviour":"stock","count":-1}]`, "negative count", readItems},
		{"item above max", `[{"id":"a","name":{"en":"A"},"behaviour":"daily","count":3,"max_count":2}]`, "exceeds max_count", readItems},
		{"skill unknown ability", `{"proficiencies":[{"id":"a","ability":"luck"}]}`, "unknown ability", readSkills},
		{"skill duplicate across lists", `{"proficiencies":[{"id":"a","ability":"str"}],"bard_skills":[{"id":"a","percent":5}]}`, "duplicate id", readSkills},
		{"bard skill above 100", `{"bard_skills":[{"id":"a","percent":101}]}`, "outside 0-100", readSkills},
		{"character race not a PHB race code", `{"name":"A","race":"Half-Elf"}`, `race "Half-Elf" is not one of`, readCharacter},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.ErrorContains(t, c.read(write(t, c.body)), c.want)
		})
	}
}

func readWeapons(p string) error   { _, err := seedfile.ReadWeapons(p); return err }
func readCharacter(p string) error { _, _, err := seedfile.ReadCharacter(p); return err }
func readItems(p string) error     { _, err := seedfile.ReadItems(p); return err }
func readSkills(p string) error    { _, err := seedfile.ReadSkills(p); return err }

// max_count defaults to the count; powers come back in their stored
// encoding, "[]" for none.
func TestReadItems_Defaults(t *testing.T) {
	items, err := seedfile.ReadItems(write(t, `[{"id":"a","name":{"en":"A"},"behaviour":"daily","count":2}]`))
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, 2, items[0].Max())
	assert.Equal(t, "[]", items[0].PowersJSON)
}
