package postgres

import (
	"context"
	"fmt"
	"slices"
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

// ADR 0033 step 26.4 (amended): a weapon no type matches is matched by its stored
// name, read from its text key: "Bastard Sword +2" finds "Bastard sword, two-handed" by the flag,
// while "Mace +1" has no one row and stays on the review list. The weapon keeps its own name.
func TestConvertEquipment_Step26_4_MatchedByTheStoredName(t *testing.T) {
	f, owner := legacyCharacter(t)
	ctx := context.Background()
	key, err := addSeedText(ctx, f, localized.Text{"en": "Bastard Sword +2", "fr": "Épée bâtarde +2"})
	require.NoError(t, err)
	require.NoError(t, f.addWeapon(weaponRow{
		CharacterID: owner, ID: "bastard_sword", Position: 3, Name: key, DamageSm: "2d4", DamageL: "2d8",
		MagicToHit: 2, MagicDamage: 2, Kind: "melee", TwoHanded: 1, WeaponType: "other_melee",
	}))
	rows := append(slices.Clone(catalogueWeapons),
		ruletablesvc.Weapon{
			Code: "bastard_sword_one_handed", WeaponType: "other_melee", Kind: "melee", Size: "m", Types: []string{"s"}, Speed: 6,
			Name: localized.Text{"en": "Bastard sword, one-handed"},
		},
		ruletablesvc.Weapon{
			Code: "bastard_sword_two_handed", WeaponType: "other_melee", Kind: "melee", Size: "m", Types: []string{"s"}, Speed: 8,
			TwoHanded: true, Name: localized.Text{"en": "Bastard sword, two-handed"},
		},
		ruletablesvc.Weapon{Code: "footmans_mace", WeaponType: "other_melee", Kind: "melee", Speed: 7, Name: localized.Text{"en": "Footman's mace"}})
	_, after := migrationDataSteps(repoSeeds(), rows)
	require.NoError(t, f.InTx(ctx, func(ctx context.Context) error { return after[equipmentMigration](ctx, f) }))

	sword := f.t.Items[rowKey(owner, "bastard_sword")]
	assert.Equal(t, []any{"bastard_sword_two_handed", int64(8), "m"}, []any{sword.CatalogueCode, sword.Speed, sword.Size})
	assert.Equal(t, "Bastard Sword +2", f.t.Texts[sword.Name]["en"], "its own name stays")
	assert.Equal(t, int64(2), sword.MagicToHit, "its own bonus stays")
	assert.Empty(t, f.t.Items[rowKey(owner, "mace")].CatalogueCode, "the mace is no row's name")
	assert.True(t, f.t.Reviews[fmt.Sprintf("%d|mace|weapon_unmatched", owner)], "the mace stays on the review list")
	assert.False(t, f.t.Reviews[fmt.Sprintf("%d|bastard_sword|weapon_unmatched", owner)], "the sword is matched")
}

// ADR 0033 step 26.4: the conversion runs once, so a catalogue without the equipment section (format
// 6 and before) refuses it when there is something to convert, naming `just catalogue`; the
// run rolls back, so the database is as it was and the next start retries. A database with nothing
// to convert does not need the equipment.
func TestConvertEquipment_Step26_4_RefusesACatalogueWithoutEquipment(t *testing.T) {
	f, owner := legacyCharacter(t)
	_, after := migrationDataSteps(repoSeeds(), nil)
	err := f.InTx(context.Background(), func(ctx context.Context) error { return after[equipmentMigration](ctx, f) })
	require.ErrorIs(t, err, ErrCatalogueWithoutEquipment)
	assert.Contains(t, err.Error(), "just catalogue")
	assert.Empty(t, f.t.Reviews, "nothing was written")
	assert.NotContains(t, f.t.Items, rowKey(owner, "dagger_2"))

	empty := newFakeStore()
	require.NoError(t, empty.InTx(context.Background(), func(ctx context.Context) error {
		return after[equipmentMigration](ctx, empty)
	}), "nothing to convert, nothing refused")
}

// ADR 0033 step 26.3: the attack form's chosen ammunition is an item id, so it follows the item
// the conversion renamed; left alone the form would offer the first matching ammunition and the
// next attack would save and spend that one.
func TestConvertEquipment_Step26_3_TheChosenAmmunitionFollowsTheRename(t *testing.T) {
	f, owner := legacyCharacter(t)
	f.t.AttackAmmo[owner] = "dagger" // the item that shares its id with the weapon
	_, after := migrationDataSteps(repoSeeds(), catalogueWeapons)
	require.NoError(t, f.InTx(context.Background(), func(ctx context.Context) error {
		return after[equipmentMigration](ctx, f)
	}))
	assert.Equal(t, "dagger_2", f.t.AttackAmmo[owner])
}

// ADR 0033 step 26.3: an item and a weapon of one id that clash only through the action log (one of
// them removed, its removal still undoable) are kept apart too: the log's ids are read from its
// snapshot keys. Here the item is stored and the weapon only in the log, and the reverse.
func TestConvertEquipment_Step26_3_IdsOnlyInTheLogAreKeptApart(t *testing.T) {
	f, owner := legacyCharacter(t)
	ctx := context.Background()
	for _, it := range []storeq.InsertItemParams{
		{ID: "torch", Position: 9, Name: 1, Behaviour: "stock", Count: 1, MaxCount: 1, Powers: "[]", DamageTypes: []string{}},
	} {
		it.CharacterID = owner
		require.NoError(t, f.InsertItem(ctx, it))
	}
	// The weapon "torch" was removed (its snapshot is in the log) while the item "torch" is stored; the
	// item "mace" was removed while the weapon "mace" is stored.
	f.t.SnapshotKeys[owner] = []string{"weapons:torch", "items:mace", "items:never_clashes"}
	_, after := migrationDataSteps(repoSeeds(), catalogueWeapons)
	require.NoError(t, f.InTx(ctx, func(ctx context.Context) error { return after[equipmentMigration](ctx, f) }))

	assert.Equal(t, "torch_2", f.t.Renames[rowKey(owner, "torch")], "the stored item steps aside for the removed weapon")
	assert.Contains(t, f.t.Items, rowKey(owner, "torch_2"))
	assert.NotContains(t, f.t.Items, rowKey(owner, "torch"))
	assert.Equal(t, "mace_2", f.t.Renames[rowKey(owner, "mace")], "the removed item's id is mapped for its undo")
	assert.NotContains(t, f.t.Items, rowKey(owner, "mace_2"), "no piece is made for it")
	assert.NotContains(t, f.t.Renames, rowKey(owner, "never_clashes"))
}

// A seeded item the conversion renamed (a player's weapon has its id) still gains the seed's
// translations: the reconcile reads it under its new id, its offers stay under the seed's id, and the
// weapon that took the id is not touched.
func TestSeed_Step26_3_ARenamedSeededItemStillGainsTranslations(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	owner := f.t.Characters[0]
	ctx := context.Background()

	ammo := f.item("arrows_plus1")
	delete(f.t.Items, rowKey(owner, "arrows_plus1"))
	renamed := ammo
	renamed.ID = "arrows_plus1_2"
	f.t.Items[rowKey(owner, renamed.ID)] = renamed
	f.t.Renames[rowKey(owner, "arrows_plus1")] = renamed.ID
	// The player's weapon took the id, under a name of its own.
	key, err := addSeedText(ctx, f, localized.Of("My bow"))
	require.NoError(t, err)
	weapon := ammo
	weapon.Name, weapon.Category = key, "weapon"
	f.t.Items[rowKey(owner, "arrows_plus1")] = weapon
	// The seed's French name is new to this database.
	delete(f.t.Texts[ammo.Name], "fr")
	delete(f.t.Offers, offer("item", "arrows_plus1", "fr"))

	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Equal(t, "Flèches +1", f.t.Texts[ammo.Name]["fr"])
	assert.Equal(t, localized.Of("My bow"), f.t.Texts[key], "the weapon's name is untouched")
	assert.True(t, f.t.Offers[offer("item", "arrows_plus1", "fr")], "recorded under the seed's id")
}
