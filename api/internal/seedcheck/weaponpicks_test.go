package seedcheck_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// ADR 0037 steps 7 and 16.2 on the seed: Kelsen's weapon picks are the distinct weapon types of
// his seed weapons (bastard sword and mace other_melee, long sword, short bow, dagger, darts), in
// the weapon types' order, as the migration grants an upgraded database; a bard may pick each,
// and they spend his 5 weapon slots at level 15 (2 + 3) exactly, so no weapon warning.
func TestKelsen_Step16_WeaponPicksAreHisWeaponTypes(t *testing.T) {
	for name, dir := range map[string]string{"campaign seeds": seedsDir, "test fixture": fixtureSeedsDir} {
		t.Run(name, func(t *testing.T) {
			book, err := ruletablesadpt.ReadProficiencies(dir)
			require.NoError(t, err)
			c, ok, err := seedfile.ReadCharacter(filepath.Join(dir, seedfile.CharacterFile))
			require.NoError(t, err)
			require.True(t, ok)
			var weapons []struct {
				Type string `json:"weapon_type"`
			}
			readJSONFrom(t, dir, "weapons.json", &weapons)
			owned := map[string]bool{}
			for _, w := range weapons {
				owned[w.Type] = true
			}
			var want []charactersvc.WeaponPick
			for _, typ := range ruletablesvc.WeaponTypes {
				if owned[typ] {
					want = append(want, charactersvc.WeaponPick{Type: typ, Slots: 1})
				}
			}
			picks := c.Picks()
			assert.Equal(t, want, picks.Weapon)
			_, err = charactersvc.ValidPicks(book, seedfile.DefaultClass, picks, charactersvc.Picks{})
			require.NoError(t, err, "the editor accepts the seed's picks")
			f, ok := charactersvc.ProficiencyFigures(book, seedfile.DefaultClass, c.Level, c.Int, picks)
			require.True(t, ok)
			assert.Equal(t, ruletablesvc.WeaponPool{Initial: 2, PerLevels: 4, LevelGained: 3, Available: 5, Spent: 5, Left: 0}, f.Weapon)
			for _, w := range charactersvc.SlotWarnings(f) {
				assert.NotEqual(t, charactersvc.PoolWeapon, w.Pool, "no weapon warning: %+v", w)
			}
		})
	}
}

// readJSONFrom reads the seed file name of dir into into.
func readJSONFrom(t *testing.T, dir, name string, into any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // fixed test path
	require.NoError(t, err, name)
	require.NoError(t, json.Unmarshal(raw, into), name)
}
