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
	_, after := migrationDataSteps(repoSeeds(), catalogueEquipment)
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
	before, after := migrationDataSteps(repoSeeds(), catalogueEquipment)
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
