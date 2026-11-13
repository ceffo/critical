package seedcheck_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/slots"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// ADR 0037 step 21.8.7 on the seed: Kelsen's weapon picks are the catalogue codes of his seed
// weapons as the equipment conversion gives them (ADR 0033 step 26.4) on the real artifact: the
// two-handed bastard sword (his other_melee pick's weapon), the long sword, the short bow, the
// dagger and the darts, 5 picks of his 5 weapon slots at level 15 (2 + 3), so no weapon pool
// warning; a bard may pick each. His Mace +1 matches no catalogue row (a footman's or a
// horseman's mace, no "mace"), so it has no code: non-proficient (the rogue's -3) and warned
// weapon_code_missing until he sets its row and picks it. The fixture catalogue's placeholder
// names match no bastard sword row by name, so only the real artifact's conversion is checked
// against the codes (skipped without it).
func TestKelsen_Step21_8_WeaponPicksAreHisWeapons(t *testing.T) {
	for name, dir := range map[string]string{"campaign seeds": seedsDir, "test fixture": fixtureSeedsDir} {
		t.Run(name, func(t *testing.T) {
			defs := cataloguetest.Definitions(t)
			eq, ok := cataloguetest.Tables(t).Equipment()
			require.True(t, ok)
			book := weaponBook(t, dir)
			c, ok, err := seedfile.ReadCharacter(filepath.Join(dir, seedfile.CharacterFile))
			require.NoError(t, err)
			require.True(t, ok)
			weapons, err := seedfile.ReadWeapons(seedfile.Path(dir, seedfile.WeaponsFile))
			require.NoError(t, err)
			conv, err := seedfile.Equipment(nil, weapons, eq.Weapons)
			require.NoError(t, err)
			codes := map[string]string{} // seed weapon id: its catalogue code
			for _, p := range conv.Pieces {
				codes[p.ID] = p.CatalogueCode
			}
			assert.Empty(t, codes["mace"], "the mace matches no row")
			picks := c.Picks()
			var picked []string
			for _, p := range picks.Weapon {
				picked = append(picked, p.Code)
				assert.Equal(t, skills.WeaponPickSlots, p.Slots, p.Code)
			}
			assert.ElementsMatch(t, []string{"bastard_sword_two_handed", "long_sword", "short_bow", "dagger", "dart"}, picked)

			_, err = skills.ValidPicks(book, c.Class, picks, skills.Picks{})
			require.NoError(t, err, "the editor accepts the seed's picks")
			f, ok := skills.ProficiencyFigures(book, c.Class, c.Level, c.Int, skills.Spending{Picks: picks})
			require.True(t, ok)
			assert.Equal(t, slots.WeaponPool{Initial: 2, PerLevels: 4, LevelGained: 3, Available: 5, Spent: 5, Left: 0}, f.Weapon)
			for _, w := range skills.SlotWarnings(f) {
				assert.NotEqual(t, skills.PoolWeapon, w.Pool, "no weapon warning: %+v", w)
			}
			for _, code := range picked {
				assert.Zero(t, f.NonproficientPenalty(code), code)
			}
			assert.Equal(t, -3, f.NonproficientPenalty(codes["mace"]), "the mace has no code: non-proficient")

			bard, ok := defs.Class(c.Class)
			require.True(t, ok)
			var mace itemsvc.Item
			for _, p := range conv.Pieces {
				if p.ID == "mace" {
					mace = p
				}
			}
			mace.State = itemsvc.StateWielded
			assert.Contains(t, itemsvc.ClassWarnings([]classes.Equipment{bard.Equipment}, ruleset.WeaponLift{}, itemsvc.OldClasses{}, []itemsvc.Item{mace}),
				itemsvc.Warning{ItemID: "mace", Code: itemsvc.WarnWeaponCodeMissing})
		})
	}
	t.Run("the real artifact converts his weapons to those codes", func(t *testing.T) {
		if _, err := os.Stat(catalogueFile); err != nil {
			t.Skipf("no real catalogue at %s (run `just catalogue`): %v", catalogueFile, err)
		}
		eq, ok := parseTables(t, catalogueFile).Equipment()
		require.True(t, ok)
		weapons, err := seedfile.ReadWeapons(seedfile.Path(seedsDir, seedfile.WeaponsFile))
		require.NoError(t, err)
		conv, err := seedfile.Equipment(nil, weapons, eq.Weapons)
		require.NoError(t, err)
		codes := map[string]string{}
		for _, p := range conv.Pieces {
			codes[p.ID] = p.CatalogueCode
		}
		assert.Equal(t, map[string]string{
			"bastard_sword": "bastard_sword_two_handed", "longsword": "long_sword", "shortbow": "short_bow",
			"dagger": "dagger", "mace": "", "darts": "dart",
		}, codes)
		c, ok, err := seedfile.ReadCharacter(filepath.Join(seedsDir, seedfile.CharacterFile))
		require.NoError(t, err)
		require.True(t, ok)
		for _, p := range c.Picks().Weapon {
			assert.NotEmpty(t, keyOf(codes, p.Code), "the pick %s is one of his weapons", p.Code)
		}
	})
}

// weaponBook is the proficiency book of the seeds in dir with the fixture catalogue's weapons, the
// codes a weapon pick names (ADR 0037 step 21.1).
func weaponBook(t *testing.T, dir string) *ruletablesvc.Proficiencies {
	t.Helper()
	book, err := ruletablesadpt.ReadProficiencies(dir, cataloguetest.Definitions(t))
	require.NoError(t, err)
	eq, ok := cataloguetest.Tables(t).Equipment()
	require.True(t, ok)
	book, err = book.WithWeapons(eq.PickWeapons())
	require.NoError(t, err)
	return book
}

// keyOf is the key of m whose value is v, "" for none.
func keyOf(m map[string]string, v string) string {
	for k, x := range m {
		if x == v {
			return k
		}
	}
	return ""
}
