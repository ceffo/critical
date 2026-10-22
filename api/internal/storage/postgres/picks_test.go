package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// picksCharacter stores a character of class with the proficiencies rows ids, in that sheet
// order, as a database before migration 025 holds them, and returns its id.
func picksCharacter(t *testing.T, f *fakeStore, class string, ids ...string) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := f.InsertCharacter(ctx, storeq.InsertCharacterParams{Name: class, ProficiencyPicks: "{}"})
	require.NoError(t, err)
	f.t.Classes[id] = class
	for i, p := range ids {
		f.t.Profs[rowKey(id, p)] = storeq.UpsertProficiencyParams{CharacterID: id, ID: p, Position: int64(len(ids) - i), Ability: "dex"}
	}
	return id
}

// pick runs the data step after migration 025 on f with the fixture's catalogue.
func pick(t *testing.T, f *fakeStore) error {
	t.Helper()
	_, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
	return f.InTx(context.Background(), func(ctx context.Context) error {
		return after[picksMigration](ctx, f)
	})
}

// ADR 0037 step 16.1: each proficiencies row becomes a non-weapon pick of its id at its cost for
// the character's class (step 4: Table 37's slots, one more outside the class's Table 38 groups),
// in sheet order (the rows' position, not their id), so no pick has an extra slot and every check
// target stays what it was. The rows stay in their table. A character without rows keeps no
// pick ('{}').
func TestMigrate_Step16_ProficiencyRowsBecomePicks(t *testing.T) {
	f := newFakeStore()
	// Positions are written in reverse: tumbling is last in the sheet, tracking first.
	bard := picksCharacter(t, f, "bard", "tumbling", "tracking", "cooking")
	fighter := picksCharacter(t, f, "fighter", "tumbling", "tracking", "cooking")
	mage := picksCharacter(t, f, "mage", "healing", "gem_cutting")
	none := picksCharacter(t, f, "thief")
	rows := len(f.t.Profs)

	require.NoError(t, pick(t, f))
	// Bard (rogue, warrior, wizard, general): all in its groups.
	assert.JSONEq(t, `{"nonweapon":[{"id":"cooking","slots":1},{"id":"tracking","slots":2},{"id":"tumbling","slots":1}]}`, f.picks(bard))
	// Fighter (warrior, general): Tumbling (rogue) costs one more.
	assert.JSONEq(t, `{"nonweapon":[{"id":"cooking","slots":1},{"id":"tracking","slots":2},{"id":"tumbling","slots":2}]}`, f.picks(fighter))
	// Mage (wizard, general): Healing (priest, 2) costs 3, Gem Cutting (rogue and wizard, 2) costs 2.
	assert.JSONEq(t, `{"nonweapon":[{"id":"gem_cutting","slots":2},{"id":"healing","slots":3}]}`, f.picks(mage))
	assert.Equal(t, "{}", f.picks(none))
	assert.Len(t, f.t.Profs, rows, "the rows stay, read by nothing")
}

// ADR 0037 step 16 as decided for the non-weapon slice: a row whose id the catalogue does not
// hold is kept as a pick of 1 slot, never dropped (the derived warnings report it,
// proficiency_unknown); the catalogue ids beside it convert as usual.
func TestMigrate_Step16_UnknownIDKeptAsOneSlot(t *testing.T) {
	f := newFakeStore()
	id := picksCharacter(t, f, "bard", "underwater_basket_weaving", "singing")
	require.NoError(t, pick(t, f))
	assert.JSONEq(t, `{"nonweapon":[{"id":"singing","slots":1},{"id":"underwater_basket_weaving","slots":1}]}`, f.picks(id))
}

// The data step is part of the migration run: it runs once, when migration 025 is applied, on the
// seed bard of an upgraded database (eight rows, all in his groups), and a second run applies
// nothing.
func TestMigrate_Step16_RunsOnceWithTheMigration(t *testing.T) {
	f := newFakeStore()
	kelsen := picksCharacter(t, f, "bard", "musical_instrument", "tumbling", "ventriloquism", "riding_land",
		"appraising", "singing", "reading_writing", "local_history")
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	before, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
	run := &fakeMigrations{store: f}
	for _, m := range embedded {
		if m.name != picksMigration {
			run.recorded = append(run.recorded, m.name)
		}
	}
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Contains(t, run.steps, "apply "+picksMigration)
	assert.JSONEq(t, `{"nonweapon":[
		{"id":"local_history","slots":1},{"id":"reading_writing","slots":1},{"id":"singing","slots":1},
		{"id":"appraising","slots":1},{"id":"riding_land","slots":1},{"id":"ventriloquism","slots":1},
		{"id":"tumbling","slots":1},{"id":"musical_instrument","slots":1}]}`, f.picks(kelsen))

	f.t.Picks[kelsen] = `{"nonweapon":[{"id":"singing","slots":2}]}` // the player's edit since
	run.steps = nil
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Equal(t, []string{"lock", "ensure table", "applied"}, run.steps, "nothing pending")
	assert.JSONEq(t, `{"nonweapon":[{"id":"singing","slots":2}]}`, f.picks(kelsen), "never converted twice")
}

// weaponItem stores a weapon piece of weaponType ("" for none) in state on character id.
func weaponItem(t *testing.T, f *fakeStore, id int64, item, weaponType, state string) {
	t.Helper()
	require.NoError(t, f.InsertItem(context.Background(), storeq.InsertItemParams{
		CharacterID: id, ID: item, Position: int64(len(f.t.Items)), Category: "weapon", WeaponType: weaponType, State: state,
		Behaviour: "stock", Count: 1, MaxCount: 1, Powers: "[]", DamageTypes: []string{},
	}))
}

// grant runs the data step after migration 026 on f.
func grant(t *testing.T, f *fakeStore) error {
	t.Helper()
	_, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
	return f.InTx(context.Background(), func(ctx context.Context) error {
		return after[weaponPicksMigration](ctx, f)
	})
}

// ADR 0037 step 16.2: each character gets one weapon pick (1 slot) per distinct weapon type among
// its weapons, in any state, in the weapon types' display order; a weapon without a type, one
// whose stored type is no weapon type and a piece of another category give none; the non-weapon picks stay. Nothing is refused (step
// 16.4): a mage gets its long sword (outside its list, step 7) and goes over its 1 weapon slot.
func TestMigrate_Step16_OwnedWeaponTypesBecomePicks(t *testing.T) {
	f := newFakeStore()
	bard := picksCharacter(t, f, "bard")
	f.t.Picks[bard] = `{"nonweapon":[{"id":"singing","slots":1}]}`
	weaponItem(t, f, bard, "bastard_sword", "other_melee", "wielded")
	weaponItem(t, f, bard, "mace", "other_melee", "carried")
	weaponItem(t, f, bard, "longsword", "long_sword", "carried")
	weaponItem(t, f, bard, "shortbow", "short_bow", "worn")
	weaponItem(t, f, bard, "odd_stick", "", "carried")
	weaponItem(t, f, bard, "katana", "katana", "carried") // no weapon type: never granted
	require.NoError(t, f.InsertItem(context.Background(), storeq.InsertItemParams{
		CharacterID: bard, ID: "arrows", Category: "ammunition", Behaviour: "stock", Count: 1, MaxCount: 1, Powers: "[]", DamageTypes: []string{},
	}))
	mage := picksCharacter(t, f, "mage")
	weaponItem(t, f, mage, "dagger", "dagger", "wielded")
	weaponItem(t, f, mage, "sword", "long_sword", "carried")
	none := picksCharacter(t, f, "thief")

	require.NoError(t, grant(t, f))
	assert.JSONEq(t, `{"nonweapon":[{"id":"singing","slots":1}],
		"weapon":[{"type":"short_bow","slots":1},{"type":"long_sword","slots":1},{"type":"other_melee","slots":1}]}`, f.picks(bard))
	assert.JSONEq(t, `{"weapon":[{"type":"dagger","slots":1},{"type":"long_sword","slots":1}]}`, f.picks(mage))
	assert.Equal(t, "{}", f.picks(none), "no weapon, no pick")

	// Idempotent: a second run adds nothing, and a stored weapon pick is kept as it is.
	f.t.Picks[mage] = `{"weapon":[{"type":"long_sword","slots":1}]}`
	require.NoError(t, grant(t, f))
	assert.JSONEq(t, `{"weapon":[{"type":"long_sword","slots":1},{"type":"dagger","slots":1}]}`, f.picks(mage))
	assert.JSONEq(t, `{"nonweapon":[{"id":"singing","slots":1}],
		"weapon":[{"type":"short_bow","slots":1},{"type":"long_sword","slots":1},{"type":"other_melee","slots":1}]}`, f.picks(bard))
}

// The weapon data step is part of the migration run: on the seed bard of an upgraded database
// (his six seed weapons as items: bastard sword and mace other_melee, long sword, short bow,
// dagger, darts) it runs once, after the non-weapon conversion, giving him five weapon picks (5
// of his 5 slots at level 15); a second run applies nothing and leaves the player's edits.
func TestMigrate_Step16_WeaponPicksRunOnceWithTheMigration(t *testing.T) {
	f := newFakeStore()
	kelsen := picksCharacter(t, f, "bard", "singing")
	for _, w := range []struct{ id, weaponType string }{
		{"bastard_sword", "other_melee"},
		{"longsword", "long_sword"},
		{"shortbow", "short_bow"},
		{"dagger", "dagger"},
		{"mace", "other_melee"},
		{"darts", "dart"},
	} {
		weaponItem(t, f, kelsen, w.id, w.weaponType, "carried")
	}
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	before, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
	run := &fakeMigrations{store: f}
	for _, m := range embedded {
		if m.name != picksMigration && m.name != weaponPicksMigration {
			run.recorded = append(run.recorded, m.name)
		}
	}
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Contains(t, run.steps, "apply "+weaponPicksMigration)
	assert.JSONEq(t, `{"nonweapon":[{"id":"singing","slots":1}],"weapon":[
		{"type":"short_bow","slots":1},{"type":"dagger","slots":1},{"type":"dart","slots":1},
		{"type":"long_sword","slots":1},{"type":"other_melee","slots":1}]}`, f.picks(kelsen))

	f.t.Picks[kelsen] = `{"weapon":[{"type":"dagger","slots":1}]}` // the player's edit since
	run.steps = nil
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Equal(t, []string{"lock", "ensure table", "applied"}, run.steps, "nothing pending")
	assert.JSONEq(t, `{"weapon":[{"type":"dagger","slots":1}]}`, f.picks(kelsen), "never granted twice")
}

// specializedItem stores a weapon piece of weaponType flagged specialized (ADR 0030 step 12's
// per-weapon flag, as a database before migration 027 holds it).
func specializedItem(t *testing.T, f *fakeStore, id int64, item, weaponType string) {
	t.Helper()
	require.NoError(t, f.InsertItem(context.Background(), storeq.InsertItemParams{
		CharacterID: id, ID: item, Position: int64(len(f.t.Items)), Category: "weapon", WeaponType: weaponType, State: "wielded",
		Behaviour: "stock", Count: 1, MaxCount: 1, Powers: "[]", DamageTypes: []string{}, Specialized: 1,
	}))
}

// ADR 0037 step 16.3: a fighter with a specialized weapon gets that type's pick as the
// specialization (step 9's slots: 2 for a long sword, 3 for a bow), the plain pick of step 16.2
// raised, or added when missing; with several specialized types the first in list order, the
// others plain picks. Another class's flag is dropped, and so is a flag on a type that is no
// weapon type. The whole run applies it once, after the weapon picks of migration 026: a rerun
// applies nothing, and a character already holding a specialization is left as it is.
func TestMigrate_Step16_SpecializedFlagBecomesAPick(t *testing.T) {
	f := newFakeStore()
	swordsman := picksCharacter(t, f, "fighter")
	specializedItem(t, f, swordsman, "longsword", "long_sword")
	specializedItem(t, f, swordsman, "longbow", "long_bow")
	weaponItem(t, f, swordsman, "dagger", "dagger", "carried")
	archer := picksCharacter(t, f, "fighter")
	weaponItem(t, f, archer, "katana", "katana", "carried")
	specializedItem(t, f, archer, "katana_spec", "katana") // no weapon type: dropped
	specializedItem(t, f, archer, "shortbow", "short_bow")
	bard := picksCharacter(t, f, "bard")
	specializedItem(t, f, bard, "rapier", "short_sword")
	plain := picksCharacter(t, f, "fighter")
	weaponItem(t, f, plain, "mace", "other_melee", "wielded")
	late := picksCharacter(t, f, "fighter") // created after step 16.2: its flagged types hold no pick
	specializedItem(t, f, late, "longsword", "long_sword")
	specializedItem(t, f, late, "longbow", "long_bow")

	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	before, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
	run := &fakeMigrations{store: f}
	for _, m := range embedded {
		if m.name < weaponPicksMigration {
			run.recorded = append(run.recorded, m.name)
		}
	}
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Contains(t, run.steps, "apply "+specializationMigration)
	assert.JSONEq(t, `{"weapon":[{"type":"long_bow","slots":1},{"type":"dagger","slots":1},{"type":"long_sword","slots":2}]}`,
		f.picks(swordsman), "the first in list order is the specialization, the bow a plain pick")
	assert.JSONEq(t, `{"weapon":[{"type":"short_bow","slots":3}]}`, f.picks(archer), "a bow costs 3")
	assert.JSONEq(t, `{"weapon":[{"type":"short_sword","slots":1}]}`, f.picks(bard), "a bard's flag is dropped")
	assert.JSONEq(t, `{"weapon":[{"type":"other_melee","slots":1}]}`, f.picks(plain), "no flag, no specialization")

	t.Run("a missing pick is added, an existing specialization kept", func(t *testing.T) {
		f.t.Picks[late] = `{"weapon":[{"type":"dagger","slots":1}]}`
		f.t.Picks[archer] = `{"nonweapon":[{"id":"singing","slots":1}]}`
		f.t.Picks[swordsman] = `{"weapon":[{"type":"long_bow","slots":3}]}` // the player's own specialization
		require.NoError(t, f.InTx(context.Background(), func(ctx context.Context) error {
			return after[specializationMigration](ctx, f)
		}))
		assert.JSONEq(t, `{"nonweapon":[{"id":"singing","slots":1}],"weapon":[{"type":"short_bow","slots":3}]}`, f.picks(archer))
		assert.JSONEq(t, `{"weapon":[{"type":"long_bow","slots":3}]}`, f.picks(swordsman), "never a second specialization")
		assert.JSONEq(t, `{"weapon":[{"type":"dagger","slots":1},{"type":"long_sword","slots":2},{"type":"long_bow","slots":1}]}`,
			f.picks(late), "the other flagged type becomes a plain pick: its attacks keep their proficiency")
	})

	run.steps = nil
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Equal(t, []string{"lock", "ensure table", "applied"}, run.steps, "nothing pending")
}
