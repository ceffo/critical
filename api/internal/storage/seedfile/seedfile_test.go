package seedfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
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
		{"bard skill duplicate", `{"bard_skills":[{"id":"a","percent":5},{"id":"a","percent":6}]}`, "duplicate id", readSkills},
		{"character pick twice", `{"name":"A","class":"bard","race":"human","proficiencies":[{"id":"a","slots":1},{"id":"a","slots":2}]}`, `proficiency "a" twice`, readCharacter},
		{"character pick no slot", `{"name":"A","class":"bard","race":"human","proficiencies":[{"id":"a","slots":0}]}`, "at least 1 slot", readCharacter},
		{"character pick no id", `{"name":"A","class":"bard","race":"human","proficiencies":[{"slots":1}]}`, "id is required", readCharacter},
		{"bard skill above 100", `{"bard_skills":[{"id":"a","percent":101}]}`, "outside 0-100", readSkills},
		{"character without a class", `{"name":"A","race":"human"}`, "class is required", readCharacter},
		{"character without a race", `{"name":"A","class":"bard"}`, "race is required", readCharacter},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.ErrorContains(t, c.read(write(t, c.body)), c.want)
		})
	}
}

// ADR 0040 step 9: the seed's class and race must be ones the catalogue
// defines.
func TestCheckDefinitions_Refusals(t *testing.T) {
	defs := cataloguetest.Definitions(t)
	cases := []struct{ name, class, race, want string }{
		{"race not a catalogue race code", "bard", "Half-Elf", `race "Half-Elf" is not one of`},
		{"class not a catalogue class", "ranger", "human", `class "ranger" is not one of`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ch := seedfile.Character{Name: "A", Class: c.class, Race: c.race}
			assert.ErrorContains(t, ch.CheckDefinitions(defs), c.want)
		})
	}
	assert.NoError(t, seedfile.Character{Name: "A", Class: "bard", Race: "half_elf"}.CheckDefinitions(defs))
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

// ADR 0049 steps 2 and 23: an item in the new shape (quantity, abilities) is read with the old columns the
// web still reads derived from them, its count the quantity (a counter is the ability's left); the old keys cannot go with it; a bad ability is refused.
func TestReadItems_ADR0049Step23_NewShape(t *testing.T) {
	items, err := seedfile.ReadItems(write(t, `[
	  {"id":"wand","name":{"en":"Wand"},"type":"wand","quantity":1,
	   "abilities":[{"id":"use","limit":{"kind":"charges","max":4,"left":3,"cost":1},"effects":[{"kind":"dice","dice":"6d6","save":"rod_staff_wand"}]}]},
	  {"id":"gem","name":{"en":"Gem"},"type":"gem","abilities":[{"id":"use","name":{"en":"Dispel Magic"},"limit":{"kind":"per_day","max":2,"left":1},"effects":[]}]},
	  {"id":"arrows","name":{"en":"Arrows"},"type":"arrow","ammo_kind":"arrow","quantity":18,"abilities":[]}
	]`))
	require.NoError(t, err)
	wand, gem, arrows := items[0], items[1], items[2]
	assert.Equal(t, []any{"charges", 1, 3, "6d6", "rod_staff_wand"}, []any{wand.Behaviour, wand.Count, wand.Max(), wand.EffectDice, wand.EffectSave})
	assert.Equal(t, []any{"daily", 1, 2}, []any{gem.Behaviour, gem.Count, gem.Max()}, "quantity defaults to 1; daily left and max")
	assert.Equal(t, "Dispel Magic", gem.AbilityList[0].Name["en"])
	assert.Equal(t, []any{"stock", 18, 18}, []any{arrows.Behaviour, arrows.Count, arrows.Max()})
	assert.NotNil(t, arrows.AbilityList, "an empty list is converted with none")
}

func TestReadItems_ADR0049Step23_RefusesAMixedOrBadNewShape(t *testing.T) {
	for name, body := range map[string]string{
		"the old keys with abilities": `[{"id":"a","name":{"en":"A"},"behaviour":"stock","abilities":[]}]`,
		"an unknown ability key":      `[{"id":"a","name":{"en":"A"},"abilities":[{"id":"use","limit":{"kind":"unlimited"},"effects":[],"cooldown":1}]}]`,
		"a negative quantity":         `[{"id":"a","name":{"en":"A"},"quantity":-1,"abilities":[]}]`,
	} {
		_, err := seedfile.ReadItems(write(t, body))
		assert.Error(t, err, name)
	}
}

// ADR 0049 steps 19 and 23: an item still in the old shape is converted by the same function as the
// boot step, so the seed and an upgraded database hold the same abilities and review reasons.
func TestEquipment_ADR0049Step19_OldShapeSeedIsConverted(t *testing.T) {
	items, err := seedfile.ReadItems(write(t, `[
	  {"id":"wand","name":{"en":"Wand"},"type":"wand","behaviour":"charges","count":4,"effect_dice":"6d6"},
	  {"id":"rope","name":{"en":"Rope"},"type":"rope","behaviour":"stock","count":2}
	]`))
	require.NoError(t, err)
	conv, err := seedfile.Equipment(items, nil, nil)
	require.NoError(t, err)
	require.Len(t, conv.Pieces, 2)
	require.Len(t, conv.Pieces[0].Abilities, 1)
	assert.Equal(t, "charges", string(conv.Pieces[0].Abilities[0].Limit.Kind))
	assert.Equal(t, 4, conv.Pieces[0].Abilities[0].Limit.Max)
	assert.NotNil(t, conv.Pieces[1].Abilities)
	assert.Contains(t, conv.Reviews, itemsvc.Review{ItemID: "wand", Reason: itemsvc.ReasonChargesTotal})
}
