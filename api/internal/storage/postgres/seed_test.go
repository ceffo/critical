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
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// repoSeeds is the frozen test fixture, not the campaign seeds/.
func repoSeeds() string { return filepath.Join("..", "..", "..", "testdata", "seeds") }

// campaignSeeds is the campaign's seeds/ (the real Power Gem), not the test fixture.
func campaignSeeds() string { return filepath.Join("..", "..", "..", "..", "seeds") }

// seedOn runs Seed's logic on the fake, which runs the transaction and the queries.
func seedOn(f *fakeStore, dir string) error { return seed(context.Background(), f, f, dir) }

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
	for id, it := range f.t.Items {
		out[id] = f.t.Texts[it.Name]
	}
	return out
}

func (f *fakeStore) weaponNames() map[string]localized.Text {
	out := map[string]localized.Text{}
	for id, w := range f.t.Weapons {
		out[id] = f.t.Texts[w.Name]
	}
	return out
}

// sheet is every field of the items and weapons but the name, by id: what the seed reconcile
// must keep.
func (f *fakeStore) sheet() map[string]any {
	out := map[string]any{}
	for id, it := range f.t.Items {
		it.Name = 0
		out["item:"+id] = it
	}
	for id, w := range f.t.Weapons {
		w.Name = 0
		out["weapon:"+id] = w
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

	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Len(t, f.t.Characters, 1, "a restart adds no character")
}

func TestSeed_WeaponsAndItemsMatchSheetAndAreIdempotent(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))

	assert.Len(t, f.t.Weapons, 6)
	assert.Len(t, f.t.Items, 6)
	assert.Equal(t, int64(18), f.t.Items["arrows_plus1"].Count)
	assert.Equal(t, int64(16), f.t.Items["arrows_plus2"].Count)
	assert.Equal(t, int64(2), f.t.Items["arrows_plus3"].Count)
	assert.Equal(t, int64(5), f.t.Items["fire_flask"].Count)
	assert.Equal(t, int64(4), f.t.Items["wand_lightning"].MaxCount, "max_count defaults to count")
	gem, wand := f.t.Items["power_gem_dispel"], f.t.Items["wand_lightning"]
	assert.Equal(t, []string{"daily", "gem"}, []string{gem.Behaviour, gem.Type})
	assert.Equal(t, []string{"charges", "wand"}, []string{wand.Behaviour, wand.Type})
	stock := 0
	for _, it := range f.t.Items {
		if it.Behaviour == "stock" {
			stock++
		}
	}
	assert.Equal(t, 4, stock, "arrows and flasks")

	// A table change must survive a re-seed (a restart).
	arrows := f.t.Items["arrows_plus1"]
	arrows.Count = 3
	f.t.Items["arrows_plus1"] = arrows
	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Len(t, f.t.Items, 6)
	assert.Equal(t, int64(3), f.t.Items["arrows_plus1"].Count)
	assert.Len(t, f.t.Weapons, 6)
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

// The sheet's proficiencies (PHB Table 37 abilities and modifiers) and bard skills load, and a
// restart with the same seed changes nothing.
func TestSeed_SkillsMatchSheetAndAreIdempotent(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))

	assert.Len(t, f.t.Profs, 8)
	assert.Len(t, f.t.Bard, 4)
	for id, want := range map[string]struct {
		ability  string
		modifier int64
	}{
		"musical_instrument": {"dex", -1}, "tumbling": {"dex", 0}, "ventriloquism": {"int", -2},
		"riding_land": {"wis", 3}, "appraising": {"int", 0}, "singing": {"cha", 0},
		"reading_writing": {"int", 1}, "local_history": {"cha", 0},
	} {
		assert.Equal(t, want.ability, f.t.Profs[id].Ability, id)
		assert.Equal(t, want.modifier, f.t.Profs[id].Modifier, id)
	}
	for id, want := range map[string]int64{"climb_walls": 85, "detect_noise": 90, "pick_pockets": 50, "read_languages": 75} {
		assert.Equal(t, want, f.t.Bard[id].Percent, id)
	}
	assert.Zero(t, f.t.Profs["musical_instrument"].Position, "seed order kept")

	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Len(t, f.t.Profs, 8)
	assert.Len(t, f.t.Bard, 4)
	assert.Equal(t, int64(50), f.t.Bard["pick_pockets"].Percent)
	assert.Equal(t, int64(7), f.t.Profs["local_history"].Position, "order kept")
	require.NoError(t, seedOn(f, t.TempDir()), "a missing file is fine once the tables are filled")
}

// An edited seed reaches an existing database: changed values and order are applied by id, a
// new row is added, nothing is duplicated, and a row the seed no longer lists is kept.
func TestSeed_SkillsReconcileByID(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, copySeeds(t, map[string]string{"skills.json": `{
		"proficiencies":[{"id":"tumbling","ability":"dex","modifier":0},{"id":"singing","ability":"cha","modifier":0}],
		"bard_skills":[{"id":"climb_walls","percent":85},{"id":"pick_pockets","percent":50}]}`})))

	edited := copySeeds(t, map[string]string{"skills.json": `{
		"proficiencies":[{"id":"singing","ability":"cha","modifier":1},{"id":"tumbling","ability":"dex","modifier":0},
			{"id":"ventriloquism","ability":"int","modifier":-2}],
		"bard_skills":[{"id":"climb_walls","percent":90}]}`})
	for range 2 {
		require.NoError(t, seedOn(f, edited))
		assert.Len(t, f.t.Profs, 3, "new row added once")
		assert.Equal(t, int64(1), f.t.Profs["singing"].Modifier, "corrected modifier")
		assert.Zero(t, f.t.Profs["singing"].Position, "new order")
		assert.Equal(t, int64(2), f.t.Profs["ventriloquism"].Position)
		assert.Equal(t, int64(-2), f.t.Profs["ventriloquism"].Modifier)
		assert.Equal(t, int64(90), f.t.Bard["climb_walls"].Percent, "raised percentage")
		assert.Len(t, f.t.Bard, 2, "a row the seed dropped is kept")
	}
}

func TestSeed_InvalidSkillsFailWithoutPartialRows(t *testing.T) {
	tests := map[string]string{
		"unknown ability":        `{"proficiencies":[{"id":"a","ability":"dex"},{"id":"b","ability":"luck"}]}`,
		"empty id":               `{"proficiencies":[{"id":"","ability":"dex"}]}`,
		"duplicate across lists": `{"proficiencies":[{"id":"a","ability":"dex"}],"bard_skills":[{"id":"a","percent":5}]}`,
		"percent above 100":      `{"bard_skills":[{"id":"a","percent":101}]}`,
		"negative percent":       `{"bard_skills":[{"id":"a","percent":-1}]}`,
		"invalid json":           `{nope`,
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
			gem := f.t.Items["power_gem_dispel"]
			assert.Equal(t, "Power Gem", f.t.Texts[gem.Name]["en"])
			assert.Equal(t, int64(1), gem.Count)
			assert.Equal(t, int64(1), gem.MaxCount)
			assert.Equal(t, "daily", gem.Behaviour)
			assert.Equal(t, thunderHammer, decodedPowers(t, gem.Powers))
			for id, it := range f.t.Items {
				if id != "power_gem_dispel" {
					assert.Equal(t, "[]", it.Powers, "only the gem has powers: %s", id)
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

// plantItem and plantWeapon store a played row whose name is an English text under a new key,
// with no seed offer recorded: a database the seed has not reconciled yet.
func (f *fakeStore) plantItem(t *testing.T, it storeq.InsertItemParams, name localized.Text) {
	t.Helper()
	key, err := addSeedText(context.Background(), f, name)
	require.NoError(t, err)
	it.Name = key
	if it.Powers == "" {
		it.Powers = "[]"
	}
	f.t.Items[it.ID] = it
}

func (f *fakeStore) plantWeapon(t *testing.T, w storeq.InsertWeaponParams, name localized.Text) {
	t.Helper()
	key, err := addSeedText(context.Background(), f, name)
	require.NoError(t, err)
	w.Name = key
	f.t.Weapons[w.ID] = w
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
	f.plantWeapon(t, storeq.InsertWeaponParams{ID: "dagger", DamageSm: "1d4", DamageL: "1d3", Kind: "melee"}, localized.Of("Dagger +3"))
	f.plantWeapon(t, storeq.InsertWeaponParams{ID: "mace", Position: 1, DamageSm: "1d6", DamageL: "1d6", Kind: "melee"}, localized.Of("Big mace"))
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
	require.NoError(t, seedOn(f, seeds(`{"en":"Gem"}`, `{"en":"Axe"}`)))
	axe := func() int64 { return f.t.Weapons["axe"].Name }

	// The player renamed the axe: the French seed name is not given...
	f.t.Texts[axe()]["en"] = "Big axe"
	both := seeds(`{"en":"Gem","fr":"Gemme"}`, `{"en":"Axe","fr":"Hache"}`)
	require.NoError(t, seedOn(f, both))
	require.Equal(t, map[string]localized.Text{"axe": {"en": "Big axe"}}, f.weaponNames())
	// ...until the name reads as the seed's again (an undo).
	f.t.Texts[axe()]["en"] = "Axe"
	require.NoError(t, seedOn(f, both))
	require.Equal(t, map[string]localized.Text{"gem": {"en": "Gem", "fr": "Gemme"}}, f.itemNames(),
		"a language added to the seed later is offered")
	require.Equal(t, map[string]localized.Text{"axe": {"en": "Axe", "fr": "Hache"}}, f.weaponNames())

	// The player blanks the French names (the DAL deletes the language's row).
	delete(f.t.Texts[f.t.Items["gem"].Name], "fr")
	delete(f.t.Texts[axe()], "fr")
	require.NoError(t, seedOn(f, both))
	assert.Equal(t, map[string]localized.Text{"gem": {"en": "Gem"}}, f.itemNames(), "a removed language stays removed")
	assert.Equal(t, map[string]localized.Text{"axe": {"en": "Axe"}}, f.weaponNames())

	// The gem is removed, then put back under a new key (an undo).
	gem := f.t.Items["gem"]
	delete(f.t.Texts, gem.Name)
	key, err := f.NextTextKey(context.Background())
	require.NoError(t, err)
	f.t.Texts[key] = localized.Text{"en": "Gem"}
	gem.Name = key
	f.t.Items["gem"] = gem
	texts := copyTables(f.t).Texts
	require.NoError(t, seedOn(f, both))
	assert.Equal(t, texts, f.t.Texts, "the record follows the id, not the key")
	assertTextIntegrity(t, f)
}
