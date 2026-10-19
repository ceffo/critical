package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// catalogueWeapons are the catalogue rows (format 7, PHB Table 44's numbers) the seeds' and the
// conversion's weapons are matched to in these tests.
var catalogueWeapons = []ruletablesvc.Weapon{
	{Code: "long_sword", WeaponType: "long_sword", Kind: "melee", Size: "m", Types: []string{"s"}, Speed: 5, DamageSM: "1d8", DamageL: "1d12"},
	{Code: "short_bow", WeaponType: "short_bow", Kind: "missile", Size: "m", Speed: 7, Fires: []string{"arrow"}},
	{Code: "dagger", WeaponType: "dagger", Kind: "melee", Size: "s", Types: []string{"p"}, Speed: 2, DamageSM: "1d4", DamageL: "1d3"},
	{Code: "dart", WeaponType: "dart", Kind: "missile", Size: "s", Types: []string{"p"}, Speed: 2, DamageSM: "1d3", DamageL: "1d2"},
}

// legacyCharacter is a database as a release before migration 016 left it: a character with
// items and weapons in their own tables, an item and a weapon of the same id.
func legacyCharacter(t *testing.T) (*fakeStore, int64) {
	t.Helper()
	f := newFakeStore()
	ctx := context.Background()
	id, err := f.InsertCharacter(ctx, storeq.InsertCharacterParams{Name: "Kelsen"})
	require.NoError(t, err)
	name := func(n string) int64 {
		key, err := addSeedText(ctx, f, localized.Of(n))
		require.NoError(t, err)
		return key
	}
	for _, it := range []storeq.InsertItemParams{
		{ID: "arrows_plus1", Position: 0, Name: name("Arrows +1"), Type: "arrow", Behaviour: "stock", MagicBonus: 1, AmmoKind: "arrow", Count: 18, MaxCount: 18, Powers: "[]"},
		{
			ID: "power_gem_dispel", Position: 3, Name: name("Power Gem"), Type: "gem", Behaviour: "daily", Count: 1, MaxCount: 1,
			Powers: `[{"id":"thunder_hammer","trigger":{"min_str":18},"effects":{"weapon_buff":5,"multiplier":2,"weapon_kind":"melee"}}]`,
		},
		{ID: "dagger", Position: 4, Name: name("Spare dagger"), Type: "sack", Behaviour: "stock", Count: 1, MaxCount: 1, Notes: "kept", Powers: "[]"},
	} {
		it.CharacterID = id
		it.DamageTypes = []string{}
		require.NoError(t, f.InsertItem(ctx, it))
	}
	for _, w := range []weaponRow{
		{ID: "shortbow", Position: 0, Name: name("Short Bow +2"), DamageSm: "1d6", DamageL: "1d6", MagicToHit: 2, MagicDamage: 2, Kind: "missile", TwoHanded: 1, AmmoKind: "arrow", WeaponType: "short_bow"},
		{ID: "dagger", Position: 1, Name: name("Dagger +3"), DamageSm: "1d4", DamageL: "1d3", MagicToHit: 3, MagicDamage: 3, Kind: "melee", WeaponType: "dagger"},
		{ID: "mace", Position: 2, Name: name("Mace +1"), DamageSm: "1d6", DamageL: "1d6", MagicToHit: 1, MagicDamage: 1, Kind: "melee", WeaponType: "other_melee"},
	} {
		w.CharacterID = id
		require.NoError(t, f.addWeapon(w))
	}
	return f, id
}

// ADR 0033 step 26: the data step after migration 016 converts every character's items and
// weapons into pieces with no manual step: ids, names (the same text keys), counts and stats
// kept, the weapons after the items, the weapons matched by type and wielded, the gem worn, the
// item whose id a weapon has renamed (step 26.3) and the review list filled (step 27).
func TestConvertEquipment_Step26_ExistingDatabase(t *testing.T) {
	f, owner := legacyCharacter(t)
	names := map[string]localized.Text{}
	for _, it := range f.t.Items {
		names["item:"+it.ID] = f.t.Texts[it.Name]
	}
	for _, w := range f.t.Weapons {
		names["weapon:"+w.ID] = f.t.Texts[w.Name]
	}
	_, after := migrationDataSteps(repoSeeds(), catalogueWeapons)
	require.NoError(t, f.InTx(context.Background(), func(ctx context.Context) error {
		return after[equipmentMigration](ctx, f)
	}))

	piece := func(id string) storeq.InsertItemParams {
		it, ok := f.t.Items[rowKey(owner, id)]
		require.True(t, ok, id)
		return it
	}
	require.Len(t, f.t.Items, 6)
	arrows := piece("arrows_plus1")
	assert.Equal(t, []any{"ammunition", "arrow", "carried", int64(18), int64(1), "1d6", "1d6"},
		[]any{arrows.Category, arrows.AmmoKind, arrows.State, arrows.Count, arrows.MagicBonus, arrows.DamageSm, arrows.DamageL},
		"the bow's dice, so the attack rolls the same")
	gem := piece("power_gem_dispel")
	assert.Equal(t, []any{"magic", "misc", "worn", "gem"}, []any{gem.Category, gem.MagicKind, gem.State, gem.Component},
		"step 1.1: the gem label is copied into the component code")
	assert.Contains(t, gem.Powers, "thunder_hammer")

	spare := piece("dagger_2")
	assert.Equal(t, []any{"gear", "carried", "kept"}, []any{spare.Category, spare.State, spare.Notes})
	assert.Equal(t, names["item:dagger"], f.t.Texts[spare.Name], "the renamed item keeps its name")
	assert.Equal(t, map[string]string{rowKey(owner, "dagger"): "dagger_2"}, f.t.Renames)

	dagger := piece("dagger")
	assert.Equal(t, names["weapon:dagger"], f.t.Texts[dagger.Name])
	assert.Equal(t, []any{"weapon", "wielded", "dagger", "s", int64(2), []string{"p"}, int64(3), "1d4"},
		[]any{dagger.Category, dagger.State, dagger.CatalogueCode, dagger.Size, dagger.Speed, dagger.DamageTypes, dagger.MagicToHit, dagger.DamageSm})
	bow := piece("shortbow")
	assert.Equal(t, []any{"short_bow", "arrow", int64(1), int64(5)}, []any{bow.CatalogueCode, bow.Fires, bow.TwoHanded, bow.Position},
		"the weapons follow the items (last item at 4)")
	assert.Equal(t, int64(6), dagger.Position)
	mace := piece("mace")
	assert.Equal(t, []any{"", int64(7)}, []any{mace.CatalogueCode, mace.Position})

	assert.Equal(t, map[string]bool{
		"1|dagger_2|uncategorised": true,
		"1|mace|weapon_unmatched":  true,
	}, f.t.Reviews)
	assert.Len(t, f.t.Weapons, 3, "the weapons table is left as it was, unread")
	assertTextIntegrity(t, f)
}

// A fresh database seeds the converted equipment: the same pieces as an upgraded one, with their
// review rows (the bastard sword and the mace are other_melee).
func TestSeed_Step26_SeedsTheConvertedEquipment(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	owner := f.t.Characters[0]
	sword := f.item("longsword")
	assert.Equal(t, []any{"weapon", "wielded", "long_sword", int64(5)}, []any{sword.Category, sword.State, sword.CatalogueCode, sword.Speed})
	assert.Equal(t, "worn", f.item("power_gem_dispel").State)
	assert.Equal(t, "1d6", f.item("arrows_plus1").DamageSm)
	assert.Equal(t, map[string]bool{
		rowKey(owner, "bastard_sword") + "|weapon_unmatched": true,
		rowKey(owner, "mace") + "|weapon_unmatched":          true,
	}, f.t.Reviews)
}
