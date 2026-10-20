package postgres

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// repoSeeds is the frozen test fixture, not the campaign seeds/.
func repoSeeds() string { return filepath.Join("..", "..", "..", "testdata", "seeds") }

// campaignSeeds is the campaign's seeds/ (the real Power Gem), not the test fixture.
func campaignSeeds() string { return filepath.Join("..", "..", "..", "..", "seeds") }

// seedOn runs Seed's logic on the fake, which runs the transaction and the queries, with
// seedBook as the spellbook seed.
func seedOn(f *fakeStore, dir string) error { return seedBookOn(f, dir, seedBook) }

// seedWithoutArmourOn is seedOn with a catalogue that has no body armour: the seed character
// wears none (ADR 0033 step 14's carry-over has nothing to give), for the tests about the other
// seeded rows.
func seedWithoutArmourOn(f *fakeStore, dir string) error {
	return seed(context.Background(), f, f, dir, seedBook, ruletablesvc.Equipment{Weapons: catalogueWeapons})
}

// seedBookOn is seedOn with the spellbook seed book (catalogue keys, in order).
func seedBookOn(f *fakeStore, dir string, book []string) error {
	return seed(context.Background(), f, f, dir, book, catalogueEquipment)
}

// seedBook is a spellbook seed: the seed writes the keys the caller checked, in order.
var seedBook = []string{"sleep", "alarm"}

// copySeeds copies the fixture seeds to a temp dir and replaces the named files.
func copySeeds(t *testing.T, replace map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(repoSeeds())
	require.NoError(t, err)
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(repoSeeds(), e.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, e.Name()), data, 0o600)) //nolint:gosec // test temp dir, fixture file names
	}
	for name, content := range replace {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)) //nolint:gosec // test temp dir, fixed names
	}
	return dir
}

// names are the names of the items or weapons by id, read through their keys.
func (f *fakeStore) itemNames() map[string]localized.Text {
	out := map[string]localized.Text{}
	for _, it := range f.t.Items {
		if it.Category != "weapon" {
			out[it.ID] = f.t.Texts[it.Name]
		}
	}
	return out
}

// weaponNames are the weapon pieces' names (ADR 0033 step 1: weapons are pieces).
func (f *fakeStore) weaponNames() map[string]localized.Text {
	out := map[string]localized.Text{}
	for _, w := range f.t.Items {
		if w.Category == "weapon" {
			out[w.ID] = f.t.Texts[w.Name]
		}
	}
	return out
}

// sheet is every field of the items and weapons but the name and the owner, by id: what the
// seed reconcile must keep (the owner a row without one gets is the ownership tests' subject).
func (f *fakeStore) sheet() map[string]any {
	out := map[string]any{}
	for _, it := range f.t.Items {
		id := it.ID
		it.Name, it.CharacterID = 0, 0
		out["item:"+id] = it
	}
	return out
}

// The character comes first, then the rows that refer to it (the foreign keys PostgreSQL
// enforces): its slots, its buff list and its round state, once.
func TestSeed_TheCharacterAndItsStateRows(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	require.Len(t, f.t.Characters, 1)
	id := f.t.Characters[0]
	assert.Contains(t, f.t.Slots, id)
	assert.Equal(t, "[]", f.t.Buffs[id])
	assert.True(t, f.t.Rounds[id])
	assert.Equal(t, "half_elf", f.t.Races[id], "the seed's race code (ADR 0030 step 1)")

	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Len(t, f.t.Characters, 1, "a restart adds no character")
}

func TestSeed_WeaponsAndItemsMatchSheetAndAreIdempotent(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))

	assert.Empty(t, f.t.Weapons, "weapons are pieces (ADR 0033 step 1)")
	// The twelve seeded pieces and the Leather armour the seed bard wears, none entered being
	// leather (ADR 0033 step 14's carry-over, as an upgraded database's).
	assert.Len(t, f.t.Items, 13)
	leather := f.item("leather")
	assert.Equal(t, []any{"armour", "worn", "leather", "leather"}, []any{leather.Category, leather.State, leather.CatalogueCode, leather.SkillColumn})
	assert.Equal(t, int64(18), f.item("arrows_plus1").Count)
	assert.Equal(t, int64(16), f.item("arrows_plus2").Count)
	assert.Equal(t, int64(2), f.item("arrows_plus3").Count)
	assert.Equal(t, int64(5), f.item("fire_flask").Count)
	assert.Equal(t, int64(4), f.item("wand_lightning").MaxCount, "max_count defaults to count")
	gem, wand := f.item("power_gem_dispel"), f.item("wand_lightning")
	assert.Equal(t, []string{"daily", "gem"}, []string{gem.Behaviour, gem.Type})
	assert.Equal(t, []string{"charges", "wand"}, []string{wand.Behaviour, wand.Type})
	stock := 0
	for _, it := range f.t.Items {
		if it.Behaviour == "stock" {
			stock++
		}
	}
	assert.Equal(t, 11, stock, "arrows, flasks, the six weapons and the armour")

	// A table change must survive a re-seed (a restart).
	arrows := f.item("arrows_plus1")
	arrows.Count = 3
	f.t.Items[rowKey(arrows.CharacterID, arrows.ID)] = arrows
	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Len(t, f.t.Items, 13, "no second armour")
	assert.Equal(t, int64(3), f.item("arrows_plus1").Count)
}

func TestSeed_InvalidCatalogsFailWithoutPartialRows(t *testing.T) {
	tests := map[string]struct{ file, content string }{
		"weapon missing damage": {"weapons.json", `[{"id":"a","name":{"en":"A"},"damage_sm":"1d4","damage_l":"1d4","kind":"melee"},{"id":"b","name":{"en":"B"}}]`},
		"weapon bad kind":       {"weapons.json", `[{"id":"a","name":{"en":"A"},"damage_sm":"1d4","damage_l":"1d4","kind":"magic"}]`},
		"item bad behaviour":    {"items.json", `[{"id":"a","name":{"en":"A"},"behaviour":"relic","count":1}]`},
		"item old kind":         {"items.json", `[{"id":"a","name":{"en":"A"},"kind":"ammo","count":1}]`},
		"item negative count":   {"items.json", `[{"id":"a","name":{"en":"A"},"behaviour":"stock","count":-1}]`},
		"weapon duplicate id":   {"weapons.json", `[{"id":"a","name":{"en":"A"},"damage_sm":"1d4","damage_l":"1d4","kind":"melee"},{"id":"a","name":{"en":"B"},"damage_sm":"1d4","damage_l":"1d4","kind":"melee"}]`},
		"item duplicate id":     {"items.json", `[{"id":"a","name":{"en":"A"},"behaviour":"stock","count":1},{"id":"a","name":{"en":"B"},"behaviour":"stock","count":1}]`},
		"item count over max":   {"items.json", `[{"id":"a","name":{"en":"A"},"behaviour":"daily","count":5,"max_count":3}]`},
		"invalid json":          {"items.json", `{nope`},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newFakeStore()
			require.Error(t, seedOn(f, copySeeds(t, map[string]string{tt.file: tt.content})))
			assert.Empty(t, f.t.Weapons, "the seed is one transaction: nothing stays")
			assert.Empty(t, f.t.Items)
			assert.Empty(t, f.t.Texts)
			assert.Empty(t, f.t.Characters)
		})
	}
}

func TestSeed_MissingCatalogFileIsAStartupError(t *testing.T) {
	for _, missing := range []string{"weapons.json", "items.json"} {
		t.Run(missing, func(t *testing.T) {
			dir := copySeeds(t, nil)
			require.NoError(t, os.Remove(filepath.Join(dir, missing)))
			err := seedOn(newFakeStore(), dir)
			require.ErrorContains(t, err, missing[:len(missing)-5])
			require.ErrorContains(t, err, "seed file is unreadable")
		})
	}
}

func TestSeed_MissingCatalogFileIsFineOnceTableIsFilled(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	require.NoError(t, seedOn(f, t.TempDir()))
}

// A fresh database: the seed character gets the character seed's picks (ADR 0037 step 16), each
// a catalogue id at its slots; its bard skills load from skills.json; a restart with the same seed
// changes nothing. No proficiencies row is written any more.
func TestSeed_PicksAndSkillsLoadAndAreIdempotent(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	id := f.t.Characters[0]

	assert.Empty(t, f.t.Profs, "the proficiencies are picks, not rows")
	assert.JSONEq(t, `{"nonweapon":[
		{"id":"musical_instrument","slots":1},{"id":"tumbling","slots":1},{"id":"ventriloquism","slots":1},
		{"id":"riding_land","slots":1},{"id":"appraising","slots":1},{"id":"singing","slots":1},
		{"id":"reading_writing","slots":1},{"id":"local_history","slots":1}]}`, f.picks(id))
	assert.Len(t, f.t.Bard, 4)
	for id, want := range map[string]int64{"climb_walls": 85, "detect_noise": 90, "pick_pockets": 50, "read_languages": 75} {
		assert.Equal(t, want, f.bardSkill(id).Percent, id)
	}

	again := copyTables(f.t)
	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Equal(t, again, f.t, "a second start writes nothing")
	require.NoError(t, seedOn(f, t.TempDir()), "a missing file is fine once the tables are filled")
}

// Once stored, the picks are the player's: an edited character seed never reaches them (the
// character is inserted only into an empty database).
func TestSeed_PicksAreNotReconciled(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	id := f.t.Characters[0]
	f.t.Picks[id] = `{"nonweapon":[{"id":"tumbling","slots":3}]}` // the player's edit
	require.NoError(t, seedOn(f, repoSeeds()))
	assert.JSONEq(t, `{"nonweapon":[{"id":"tumbling","slots":3}]}`, f.picks(id))
}

// An edited skills seed reaches an existing database: changed percentages and order are applied by
// id, nothing is duplicated, and a row the seed no longer lists is kept.
func TestSeed_SkillsReconcileByID(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, copySeeds(t, map[string]string{"skills.json": `{
		"bard_skills":[{"id":"climb_walls","percent":85},{"id":"pick_pockets","percent":50}]}`})))

	edited := copySeeds(t, map[string]string{"skills.json": `{
		"bard_skills":[{"id":"climb_walls","percent":90}]}`})
	for range 2 {
		require.NoError(t, seedOn(f, edited))
		assert.Equal(t, int64(90), f.bardSkill("climb_walls").Percent, "raised percentage")
		assert.Len(t, f.t.Bard, 2, "a row the seed dropped is kept")
	}
}

func TestSeed_InvalidSkillsFailWithoutPartialRows(t *testing.T) {
	tests := map[string]string{
		"empty id":          `{"bard_skills":[{"id":"","percent":5}]}`,
		"duplicate id":      `{"bard_skills":[{"id":"a","percent":5},{"id":"a","percent":6}]}`,
		"percent above 100": `{"bard_skills":[{"id":"a","percent":101}]}`,
		"negative percent":  `{"bard_skills":[{"id":"a","percent":-1}]}`,
		"invalid json":      `{nope`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			f := newFakeStore()
			require.Error(t, seedOn(f, copySeeds(t, map[string]string{"skills.json": content})))
			assert.Empty(t, f.t.Profs)
			assert.Empty(t, f.t.Bard)
		})
	}
}

func TestSeed_MissingSkillsFileIsAStartupError(t *testing.T) {
	dir := copySeeds(t, nil)
	require.NoError(t, os.Remove(filepath.Join(dir, "skills.json")))
	err := seedOn(newFakeStore(), dir)
	require.ErrorContains(t, err, "seed skills")
	require.ErrorContains(t, err, "seed file is unreadable")
}

// thunderHammer is the gem's one listed power (ADR 0016 step 16).
var thunderHammer = []itemsvc.Power{{
	ID: "thunder_hammer", MinStr: new(18),
	Effects: buffsvc.BuffEffects{WeaponBuff: new(5), Multiplier: new(2.0), WeaponKind: buffsvc.WeaponMelee},
}}

func decodedPowers(t *testing.T, raw string) []itemsvc.Power {
	t.Helper()
	ps, err := itemsvc.DecodePowers(raw)
	require.NoError(t, err)
	return ps
}

// ADR 0016 step 16: a new database seeded from seeds/items.json (and the test fixture alike) has
// the one "Power Gem": daily, 1 a day, with the thunder hammer; the other items have no powers.
func TestSeed_Step16_PowerGemHasTheHammer(t *testing.T) {
	for name, dir := range map[string]string{"campaign seeds": campaignSeeds(), "test fixture": repoSeeds()} {
		t.Run(name, func(t *testing.T) {
			f := newFakeStore()
			require.NoError(t, seedOn(f, dir))
			gem := f.item("power_gem_dispel")
			assert.Equal(t, "Power Gem", f.t.Texts[gem.Name]["en"])
			assert.Equal(t, int64(1), gem.Count)
			assert.Equal(t, int64(1), gem.MaxCount)
			assert.Equal(t, "daily", gem.Behaviour)
			assert.Equal(t, thunderHammer, decodedPowers(t, gem.Powers))
			for _, it := range f.t.Items {
				if it.ID != "power_gem_dispel" {
					assert.Equal(t, "[]", it.Powers, "only the gem has powers: %s", it.ID)
				}
			}
		})
	}
}

// ADR 0016 step 9: a seed item whose power has a Strength effect (or any invalid power) refuses
// the seed, and no item is stored.
func TestSeed_Step9_RejectsAPowerWithAStrengthEffect(t *testing.T) {
	tests := map[string]string{
		"str_bonus":      `{"id":"belt","effects":{"str_bonus":4}}`,
		"str_max":        `{"id":"belt","effects":{"str_max":19}}`,
		"unknown effect": `{"id":"belt","effects":{"weapon_bluff":5}}`,
		"bad weapon":     `{"id":"belt","effects":{"weapon_kind":"thrown"}}`,
	}
	for name, power := range tests {
		t.Run(name, func(t *testing.T) {
			f := newFakeStore()
			err := seedOn(f, copySeeds(t, map[string]string{"items.json": `[{"id":"a","name":{"en":"A"},"behaviour":"stock","count":1},
				{"id":"belt","name":{"en":"Belt"},"behaviour":"stock","count":1,"powers":[` + power + `]}]`}))
			require.Error(t, err)
			assert.Contains(t, err.Error(), `"belt"`)
			assert.Empty(t, f.t.Items)
		})
	}
}

// ADR 0023 step 10: a fresh database gets every language of every seed name.
func TestSeed_Step10_FreshDatabaseGetsEveryLanguage(t *testing.T) {
	for name, dir := range map[string]string{"campaign seeds": campaignSeeds(), "test fixture": repoSeeds()} {
		t.Run(name, func(t *testing.T) {
			f := newFakeStore()
			require.NoError(t, seedOn(f, dir))
			items, weapons := f.itemNames(), f.weaponNames()
			assert.Equal(t, localized.Text{"en": "Power Gem", "fr": "Gemme de pouvoir"}, items["power_gem_dispel"])
			assert.Equal(t, localized.Text{"en": "Long Sword +2", "fr": "Épée longue +2"}, weapons["longsword"])
			for id, n := range items {
				assert.Equal(t, []string{"en", "fr"}, n.Languages(), "item %s", id)
			}
			for id, n := range weapons {
				assert.Equal(t, []string{"en", "fr"}, n.Languages(), "weapon %s", id)
			}
			assertTextIntegrity(t, f)
		})
	}
}

// firstCharacter is the id the fake gives the first character it stores: on a fresh fake, the
// seed character's.
const firstCharacter = 1

// plantItem and plantWeapon store a played row whose name is an English text under a new key,
// with no seed offer recorded, owned by the seed character unless the row names another: a
// database the seed has not reconciled yet.
func (f *fakeStore) plantItem(t *testing.T, it storeq.InsertItemParams, name localized.Text) {
	t.Helper()
	key, err := addSeedText(context.Background(), f, name)
	require.NoError(t, err)
	it.Name = key
	if it.CharacterID == 0 {
		it.CharacterID = firstCharacter
	}
	if it.Powers == "" {
		it.Powers = "[]"
	}
	f.t.Items[rowKey(it.CharacterID, it.ID)] = it
}

// plantWeapon plants a weapon piece (as the conversion of ADR 0033 step 26 stores one).
func (f *fakeStore) plantWeapon(t *testing.T, w weaponRow, name localized.Text) {
	t.Helper()
	f.plantItem(t, storeq.InsertItemParams{
		CharacterID: w.CharacterID, ID: w.ID, Position: 10 + w.Position, Category: "weapon", State: "wielded",
		Behaviour: "stock", Count: 1, MaxCount: 1, DamageSm: w.DamageSm, DamageL: w.DamageL, WeaponKind: w.Kind,
		DamageTypes: []string{},
	}, name)
}

// ADR 0023 step 10: on a played database, the seed gives a name it still holds unchanged the
// languages it lacks (the gem becomes "Gemme de pouvoir" in French); a name the player changed
// is kept, an item the player removed is not added back, no count, position or other field
// moves, and a second start writes nothing.
func TestSeed_Step10_ReconcileAddsLanguagesKeepingPlayerNamesAndRows(t *testing.T) {
	f := newFakeStore()
	f.plantItem(t, storeq.InsertItemParams{ID: "arrows_plus1", Type: "arrow", Behaviour: "stock", MagicBonus: 1, AmmoKind: "arrow", Count: 3, MaxCount: 18},
		// A French name the player typed: the seed's English one still matches, the French one
		// does not, so the name is the player's.
		localized.Text{"en": "Arrows +1", "fr": "Mes flèches"})
	f.plantItem(t, storeq.InsertItemParams{ID: "wand_lightning", Position: 4, Type: "wand", Behaviour: "charges", Count: 2, MaxCount: 4}, localized.Of("My wand"))
	f.plantItem(t, storeq.InsertItemParams{ID: "power_gem_dispel", Position: 5, Type: "gem", Behaviour: "daily", MaxCount: 1, Notes: "mine"}, localized.Of("Power Gem"))
	f.plantItem(t, storeq.InsertItemParams{ID: "rope", Position: 6, Type: "gear", Behaviour: "stock", Count: 1, MaxCount: 1}, localized.Of("Rope"))
	f.plantWeapon(t, weaponRow{ID: "dagger", DamageSm: "1d4", DamageL: "1d3", Kind: "melee"}, localized.Of("Dagger +3"))
	f.plantWeapon(t, weaponRow{ID: "mace", Position: 1, DamageSm: "1d6", DamageL: "1d6", Kind: "melee"}, localized.Of("Big mace"))
	before := f.sheet()

	require.NoError(t, seedOn(f, campaignSeeds()))

	assert.Equal(t, map[string]localized.Text{
		"arrows_plus1":     {"en": "Arrows +1", "fr": "Mes flèches"},
		"wand_lightning":   {"en": "My wand"},
		"power_gem_dispel": {"en": "Power Gem", "fr": "Gemme de pouvoir"},
		"rope":             {"en": "Rope"},
	}, f.itemNames(), "removed seed items (fire flasks) stay removed")
	assert.Equal(t, map[string]localized.Text{
		"dagger": {"en": "Dagger +3", "fr": "Dague +3"},
		"mace":   {"en": "Big mace"},
	}, f.weaponNames())
	assert.Equal(t, before, f.sheet(), "no count, position or other field moves")
	assertTextIntegrity(t, f)

	texts := copyTables(f.t).Texts
	require.NoError(t, seedOn(f, campaignSeeds()))
	assert.Equal(t, texts, f.t.Texts, "a second start writes nothing")
}

// A seed name must hold a language: an empty map or blank bodies refuse the seed and store
// nothing.
func TestSeed_Step10_RejectsANameWithoutLanguage(t *testing.T) {
	for name, content := range map[string]string{
		"empty map":  `[{"id":"a","name":{},"behaviour":"stock","count":1}]`,
		"blank body": `[{"id":"a","name":{"en":"  "},"behaviour":"stock","count":1}]`,
		"plain text": `[{"id":"a","name":"A","behaviour":"stock","count":1}]`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeStore()
			require.Error(t, seedOn(f, copySeeds(t, map[string]string{"items.json": content})))
			assert.Empty(t, f.t.Items)
		})
	}
}

// The seed gives each language to a name once: a language added to the seed later reaches a
// filled table on the next start (a name the player changed gets it once it reads as the seed's
// again), but a language the player removed from a name (a blank body, step 11) is not put back,
// even once the row was removed and put back under a new key (an undo).
func TestSeed_Step10_ALanguageThePlayerRemovedStaysRemoved(t *testing.T) {
	seeds := func(item, weapon string) string {
		return copySeeds(t, map[string]string{
			"items.json":   `[{"id":"gem","name":` + item + `,"behaviour":"daily","count":1}]`,
			"weapons.json": `[{"id":"axe","name":` + weapon + `,"damage_sm":"1d6","damage_l":"1d4","kind":"melee"}]`,
		})
	}
	f := newFakeStore()
	require.NoError(t, seedWithoutArmourOn(f, seeds(`{"en":"Gem"}`, `{"en":"Axe"}`)))
	axe := func() int64 { return f.weapon("axe").Name }

	// The player renamed the axe: the French seed name is not given...
	f.t.Texts[axe()]["en"] = "Big axe"
	both := seeds(`{"en":"Gem","fr":"Gemme"}`, `{"en":"Axe","fr":"Hache"}`)
	require.NoError(t, seedWithoutArmourOn(f, both))
	require.Equal(t, map[string]localized.Text{"axe": {"en": "Big axe"}}, f.weaponNames())
	// ...until the name reads as the seed's again (an undo).
	f.t.Texts[axe()]["en"] = "Axe"
	require.NoError(t, seedWithoutArmourOn(f, both))
	require.Equal(t, map[string]localized.Text{"gem": {"en": "Gem", "fr": "Gemme"}}, f.itemNames(),
		"a language added to the seed later is offered")
	require.Equal(t, map[string]localized.Text{"axe": {"en": "Axe", "fr": "Hache"}}, f.weaponNames())

	// The player blanks the French names (the DAL deletes the language's row).
	delete(f.t.Texts[f.item("gem").Name], "fr")
	delete(f.t.Texts[axe()], "fr")
	require.NoError(t, seedWithoutArmourOn(f, both))
	assert.Equal(t, map[string]localized.Text{"gem": {"en": "Gem"}}, f.itemNames(), "a removed language stays removed")
	assert.Equal(t, map[string]localized.Text{"axe": {"en": "Axe"}}, f.weaponNames())

	// The gem is removed, then put back under a new key (an undo).
	gem := f.item("gem")
	delete(f.t.Texts, gem.Name)
	key, err := f.NextTextKey(context.Background())
	require.NoError(t, err)
	f.t.Texts[key] = localized.Text{"en": "Gem"}
	gem.Name = key
	f.t.Items[rowKey(gem.CharacterID, gem.ID)] = gem
	texts := copyTables(f.t).Texts
	require.NoError(t, seedWithoutArmourOn(f, both))
	assert.Equal(t, texts, f.t.Texts, "the record follows the id, not the key")
	assertTextIntegrity(t, f)
}
