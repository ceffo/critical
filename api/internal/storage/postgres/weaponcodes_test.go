package postgres

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// fixtureEquipment is the fixture catalogue's equipment (every PHB Table 44 weapon row with its
// type), where catalogueEquipment holds a few rows only.
var fixtureEquipment = func() ruletablesvc.Equipment {
	a, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "..", "testdata", "catalogue.json"))
	if err != nil {
		panic(err)
	}
	t, err := ruletablesadpt.Parse(a.RuleTables, a.Equipment, a.Languages, a.Definitions)
	if err != nil {
		panic(err)
	}
	eq, ok := t.Equipment()
	if !ok {
		panic("the fixture catalogue has no equipment")
	}
	return *eq
}()

// codedItem stores a weapon piece of the catalogue row code (its type the row's, as a pick of the
// row copies it, ADR 0033 step 23) in state, after the character's other pieces in list order.
func codedItem(t *testing.T, f *fakeStore, id int64, item, code, state string) {
	t.Helper()
	weaponType := ""
	for _, w := range fixtureEquipment.Weapons {
		if w.Code == code {
			weaponType = w.WeaponType
		}
	}
	require.NotEmpty(t, weaponType, "no catalogue row %s", code)
	require.NoError(t, f.InsertItem(context.Background(), storeq.InsertItemParams{
		CharacterID: id, ID: item, Position: int64(len(f.t.Items)), Category: "weapon", WeaponType: weaponType,
		CatalogueCode: code, State: state, Behaviour: "stock", Count: 1, MaxCount: 1, Powers: "[]", DamageTypes: []string{},
	}))
}

// pickCodes runs the data step after migration 042 on f with the fixture's catalogue.
func pickCodes(t *testing.T, f *fakeStore) error {
	t.Helper()
	_, after := migrationDataSteps(repoSeeds(), fixtureEquipment, catalogueDefs)
	return f.InTx(context.Background(), func(ctx context.Context) error {
		return after[weaponCodesMigration](ctx, f)
	})
}

// ADR 0037 step 21.8.1: a plain type pick becomes one plain pick per distinct catalogue code
// among the character's weapons of the type, in any state, in list order: an other_melee pick
// with a quarterstaff and a heavy horse lance gives two picks, a dagger pick with two daggers
// one; a weapon of a type with no pick gives none; the non-weapon picks stay. A staff_sling pick
// (migration 041's carried pick) becomes the staff sling's.
func TestMigrate_Step21_8_1_ATypePickBecomesItsOwnedWeapons(t *testing.T) {
	f := newFakeStore()
	fighter := picksCharacter(t, f, "fighter")
	f.t.Picks[fighter] = `{"nonweapon":[{"id":"riding_land","slots":1}],` +
		`"weapon":[{"type":"other_melee","slots":1},{"type":"dagger","slots":1},{"type":"staff_sling","slots":1}]}`
	codedItem(t, f, fighter, "lance", "heavy_horse_lance", "carried")
	codedItem(t, f, fighter, "dagger_1", "dagger", "wielded")
	codedItem(t, f, fighter, "staff", "quarterstaff", "wielded")
	codedItem(t, f, fighter, "dagger_2", "dagger", "carried")
	codedItem(t, f, fighter, "bow", "long_bow", "carried") // no long_bow pick: not proficient, and stays so
	codedItem(t, f, fighter, "staff_sling", "staff_sling", "carried")

	require.NoError(t, pickCodes(t, f))
	assert.JSONEq(t, `{"nonweapon":[{"id":"riding_land","slots":1}],"weapon":[
		{"code":"heavy_horse_lance","slots":1},{"code":"quarterstaff","slots":1},
		{"code":"dagger","slots":1},{"code":"staff_sling","slots":1}]}`, f.picks(fighter))
}

// ADR 0037 step 21.8.2: a specialization type pick (the type's specialization slots) makes the
// first owned weapon of the type in list order the specialization, at the same slots, the other
// codes plain picks.
func TestMigrate_Step21_8_2_ASpecializationGoesToTheFirstOwnedWeapon(t *testing.T) {
	f := newFakeStore()
	fighter := picksCharacter(t, f, "fighter")
	f.t.Picks[fighter] = `{"weapon":[{"type":"long_bow","slots":3},{"type":"other_melee","slots":2}]}`
	codedItem(t, f, fighter, "composite", "composite_long_bow", "wielded")
	codedItem(t, f, fighter, "bow", "long_bow", "carried")
	codedItem(t, f, fighter, "broad", "broad_sword", "carried")
	codedItem(t, f, fighter, "axe", "battle_axe", "carried")

	require.NoError(t, pickCodes(t, f))
	assert.JSONEq(t, `{"weapon":[{"code":"composite_long_bow","slots":3},{"code":"long_bow","slots":1},
		{"code":"broad_sword","slots":2},{"code":"battle_axe","slots":1}]}`, f.picks(fighter))
	picks, err := characterdal.DecodePicks(f.picks(fighter))
	require.NoError(t, err)
	book := migrationBook(t)
	assert.Equal(t, "composite_long_bow", skills.SpecializedCode(book, "fighter", picks),
		"the first bow is the specialization, at a bow's 3 slots")
}

// ADR 0037 step 21.8.3: a type pick with no owned weapon of the type (none, or only a piece
// without a catalogue code) gives no pick: the type and its slots are listed under weapon_redo,
// read back as the weapon_pick_to_redo warning, its slots back to spend.
func TestMigrate_Step21_8_3_ATypeWithNoOwnedWeaponIsListedToRedo(t *testing.T) {
	f := newFakeStore()
	bard := picksCharacter(t, f, "bard")
	f.t.Picks[bard] = `{"weapon":[{"type":"short_bow","slots":1},{"type":"other_melee","slots":1},{"type":"sling","slots":1}]}`
	codedItem(t, f, bard, "bow", "short_bow", "carried")
	weaponItem(t, f, bard, "mace", "other_melee", "wielded") // a homebrew mace: no catalogue code

	require.NoError(t, pickCodes(t, f))
	assert.JSONEq(t, `{"weapon":[{"code":"short_bow","slots":1}],
		"weapon_redo":[{"type":"other_melee","slots":1},{"type":"sling","slots":1}]}`, f.picks(bard))

	picks, err := characterdal.DecodePicks(f.picks(bard))
	require.NoError(t, err)
	got, ws := skills.Read(migrationBook(t), skills.Sheet{Class: "bard", Level: 1, Intelligence: 9, Picks: picks})
	require.NotNil(t, got)
	assert.Equal(t, 1, got.Weapon.Spent, "the types to redo spend nothing")
	one := 1
	assert.Contains(t, ws, skills.Warning{Code: "weapon_pick_to_redo", Field: "weapon.other_melee", Type: "other_melee", Slots: &one})
	assert.Contains(t, ws, skills.Warning{Code: "weapon_pick_to_redo", Field: "weapon.sling", Type: "sling", Slots: &one})
}

// ADR 0037 step 21.8.4: a stored type pick whose type is no weapon type (older data step 7 kept)
// is listed to redo the same way, even when a weapon of the character carries that stored type.
func TestMigrate_Step21_8_4_AnUnknownTypeIsListedToRedo(t *testing.T) {
	f := newFakeStore()
	fighter := picksCharacter(t, f, "fighter")
	f.t.Picks[fighter] = `{"weapon":[{"type":"katana","slots":1},{"type":"dagger","slots":1}]}`
	codedItem(t, f, fighter, "dagger", "dagger", "carried")
	weaponItem(t, f, fighter, "katana", "katana", "wielded")

	require.NoError(t, pickCodes(t, f))
	assert.JSONEq(t, `{"weapon":[{"code":"dagger","slots":1}],"weapon_redo":[{"type":"katana","slots":1}]}`, f.picks(fighter))
}

// ADR 0037 step 21.8.5: nothing is refused and neither the class lists nor the budget are checked:
// a level 1 mage whose one other_melee pick covered a quarterstaff, a club and a long sword it
// owns (the club and the long sword outside its list) keeps three picks, the two it may not make
// read allowed false, and is left over its one weapon slot, warned slots_over_budget.
func TestMigrate_Step21_8_5_OverBudgetIsKeptAndWarned(t *testing.T) {
	f := newFakeStore()
	mage := picksCharacter(t, f, "mage")
	f.t.Picks[mage] = `{"weapon":[{"type":"other_melee","slots":1}]}`
	codedItem(t, f, mage, "staff", "quarterstaff", "wielded")
	codedItem(t, f, mage, "club", "club", "carried")
	codedItem(t, f, mage, "spear", "spear", "carried")

	require.NoError(t, pickCodes(t, f))
	picks, err := characterdal.DecodePicks(f.picks(mage))
	require.NoError(t, err)
	assert.Equal(t, []skills.WeaponPick{{Code: "quarterstaff", Slots: 1}, {Code: "club", Slots: 1}, {Code: "spear", Slots: 1}}, picks.Weapon)
	got, ws := skills.Read(migrationBook(t), skills.Sheet{Class: "mage", Level: 1, Intelligence: 9, Picks: picks})
	require.NotNil(t, got)
	assert.Equal(t, []skills.WeaponPickFigures{
		{Code: "quarterstaff", Slots: 1, Allowed: true}, {Code: "club", Slots: 1}, {Code: "spear", Slots: 1},
	}, got.WeaponPicks)
	three, one := 3, 1
	assert.Contains(t, ws, skills.Warning{Code: "slots_over_budget", Field: "weapon", Pool: "weapon", Spent: &three, Available: &one})
}

// ADR 0037 step 21.8: the data step is part of the migration run: on an upgraded database it runs
// once, after migration 041 (a staff sling's other_missile pick carried to staff_sling first, then
// turned into the staff sling's code); a character whose weapon picks are already codes is left as
// it is, and a second run applies nothing and leaves the player's edits.
func TestMigrate_Step21_8_RunsOnceWithTheMigration(t *testing.T) {
	f := newFakeStore()
	slinger := picksCharacter(t, f, "cleric")
	f.t.Picks[slinger] = `{"weapon":[{"type":"other_missile","slots":1}]}`
	piece := storeq.InsertItemParams{
		CharacterID: slinger, ID: "staff_sling", Position: 0, Category: "weapon", WeaponType: "other_missile",
		CatalogueCode: "staff_sling", State: "wielded", Behaviour: "stock", Count: 1, MaxCount: 1, Powers: "[]", DamageTypes: []string{},
	}
	require.NoError(t, f.InsertItem(context.Background(), piece))
	coded := picksCharacter(t, f, "fighter")
	f.t.Picks[coded] = `{"weapon":[{"code":"quarterstaff","slots":1}]}`
	codedItem(t, f, coded, "lance", "heavy_horse_lance", "carried")

	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	before, after := migrationDataSteps(repoSeeds(), fixtureEquipment, catalogueDefs)
	run := &fakeMigrations{store: f}
	for _, m := range embedded {
		if m.name < catalogueWeaponTypesMigration {
			run.recorded = append(run.recorded, m.name)
		}
	}
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Contains(t, run.steps, "apply "+weaponCodesMigration)
	assert.JSONEq(t, `{"weapon":[{"code":"staff_sling","slots":1}]}`, f.picks(slinger))
	assert.JSONEq(t, `{"weapon":[{"code":"quarterstaff","slots":1}]}`, f.picks(coded), "already codes: left as it is")

	f.t.Picks[slinger] = `{"weapon":[{"code":"sling","slots":1}]}` // the player's edit since
	run.steps = nil
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Equal(t, []string{"lock", "ensure table", "applied"}, run.steps, "nothing pending")
	assert.JSONEq(t, `{"weapon":[{"code":"sling","slots":1}]}`, f.picks(slinger))
	require.NoError(t, pickCodes(t, f), "a rerun of the step alone")
	assert.JSONEq(t, `{"weapon":[{"code":"sling","slots":1}]}`, f.picks(slinger), "a rerun changes nothing")
	assert.JSONEq(t, `{"weapon":[{"code":"quarterstaff","slots":1}]}`, f.picks(coded))
}

// migrationBook is the fixture's proficiency book with its catalogue weapons, read as the app
// reads the column after the migration.
func migrationBook(t *testing.T) skills.Book {
	t.Helper()
	book, err := ruletablesadpt.ReadProficiencies(repoSeeds(), catalogueDefs)
	require.NoError(t, err)
	book, err = book.WithWeapons(fixtureEquipment.PickWeapons())
	require.NoError(t, err)
	return book
}
