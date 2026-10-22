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
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// seedsWithCharacter copies the fixture seeds to a temp dir, with the seed
// character's JSON changed by edit.
func seedsWithCharacter(t *testing.T, edit func(map[string]any)) string {
	t.Helper()
	seeds := t.TempDir()
	require.NoError(t, os.CopyFS(seeds, os.DirFS(fixtureSeeds)))
	path := filepath.Join(seeds, "character.json")
	raw, err := os.ReadFile(path) //nolint:gosec // test temp dir
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	edit(doc)
	raw, err = json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	return seeds
}

// ADR 0040 step 9: the seed character's class is the seed's, not a constant:
// a seed of class fighter starts a fighter (of the warrior group); a seed
// without a class refuses the start.
func TestSeed_Step9_TheSeedNamesItsClass(t *testing.T) {
	ctx := context.Background()
	seeds := seedsWithCharacter(t, func(d map[string]any) {
		d["class"] = "fighter"
		delete(d, "skill_points") // a bard's; a fighter has none to spend
	})
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), composition.Config{SeedsDir: seeds, Catalogue: fixtureCatalogue})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	c, err := bs.Character.GetCharacter(ctx, rootCharacter(t, bs))
	require.NoError(t, err)
	assert.Equal(t, "fighter", c.Class)
	assert.Equal(t, "warrior", c.Derived.Group)

	seeds = seedsWithCharacter(t, func(d map[string]any) { delete(d, "class") })
	_, err = composition.Build(ctx, composition.MemoryStorage(memory.New()), composition.Config{SeedsDir: seeds, Catalogue: fixtureCatalogue})
	require.ErrorContains(t, err, "class is required")
}

// ADR 0040 step 6: the XP bonus reads the class's own prime requisites. A
// fighter of Str 17 with Dex and Cha 10 is eligible (the old test read the
// bard's Dex and Cha for every class and refused him); a fighter of Str 15
// with Dex and Cha 16 is not.
func TestGet_Step6_TheClassPicksThePrimeRequisites(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	rootCharacter(t, bs)

	for _, tc := range []struct {
		name          string
		str, dex, cha int
		want          bool
	}{
		{"Str 17, Dex and Cha 10", 17, 10, 10, true},
		{"Str 15, Dex and Cha 16", 15, 16, 16, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, err := m.AddCharacter(ctx, charactersvc.Character{
				Name: "Brute", Class: "fighter", Race: "human", Str: tc.str, Dex: tc.dex, Con: 10, Int: 10, Wis: 10, Cha: tc.cha,
				HPMax: 8, RealizedLevel: 1,
			})
			require.NoError(t, err)
			l, err := bs.XP.Get(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, tc.want, l.BonusEligible)
		})
	}
}
