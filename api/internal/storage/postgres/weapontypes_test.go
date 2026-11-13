package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// typedCatalogue is a catalogue whose staff sling has its own type (format 22) beside the blowgun,
// still an other missile weapon.
var typedCatalogue = ruletablesvc.Equipment{Weapons: append([]ruletablesvc.Weapon{
	{Code: "staff_sling", WeaponType: "staff_sling", Kind: "missile", Size: "m", Speed: 11, Fires: []string{"bullet"}},
	{Code: "blowgun", WeaponType: "other_missile", Kind: "missile", Size: "l", Speed: 5, Fires: []string{"blowgun_dart", "needle"}},
}, catalogueWeapons...), Armour: catalogueArmour}

// weaponPiece is a stored weapon piece of the character.
func weaponPiece(characterID int64, id, code, weaponType string) storeq.InsertItemParams {
	return storeq.InsertItemParams{CharacterID: characterID, ID: id, Category: "weapon", CatalogueCode: code, WeaponType: weaponType}
}

// ADR 0030 step 23 (catalogue format 22): the data step after migration 041 gives the stored
// pieces of a catalogue row still typed other_missile the row's own type, so a staff sling picked
// before format 22 is a staff_sling; every other piece keeps its type, and a second run changes
// nothing.
func TestMigrate_Step23_StoredCatalogueWeaponsTakeTheirRowsType(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name  string
		piece storeq.InsertItemParams
		want  string
	}{
		{"a staff sling picked before format 22", weaponPiece(1, "staff_sling", "staff_sling", "other_missile"), "staff_sling"},
		{"another character's staff sling", weaponPiece(2, "sling_staff", "staff_sling", "other_missile"), "staff_sling"},
		{"a staff sling the player typed a sling", weaponPiece(1, "staff_sling", "staff_sling", "sling"), "sling"},
		{"a blowgun (its row is still other_missile)", weaponPiece(1, "blowgun", "blowgun", "other_missile"), "other_missile"},
		{"a homebrew other missile weapon", weaponPiece(1, "atlatl", "", "other_missile"), "other_missile"},
		{"a dagger the player typed other_missile", weaponPiece(1, "dagger", "dagger", "other_missile"), "other_missile"},
		{"a piece of another category", storeq.InsertItemParams{CharacterID: 1, ID: "bag", Category: "gear", CatalogueCode: "staff_sling", WeaponType: "other_missile"}, "other_missile"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeStore()
			f.t.Items[rowKey(tc.piece.CharacterID, tc.piece.ID)] = tc.piece
			for range 2 {
				require.NoError(t, retypeCatalogueWeapons(ctx, f, typedCatalogue.Weapons))
				assert.Equal(t, tc.want, f.t.Items[rowKey(tc.piece.CharacterID, tc.piece.ID)].WeaponType)
			}
		})
	}
}

// No catalogue equipment changes nothing.
func TestMigrate_Step23_NoCatalogueRetypesNothing(t *testing.T) {
	f := newFakeStore()
	p := weaponPiece(1, "staff_sling", "staff_sling", "other_missile")
	f.t.Items[rowKey(1, p.ID)] = p
	require.NoError(t, retypeCatalogueWeapons(context.Background(), f, nil))
	assert.Equal(t, "other_missile", f.t.Items[rowKey(1, p.ID)].WeaponType)
}

// The data step is reached through runMigrations from the embedded file: a typo in the step's key
// would leave it silently never run.
func TestMigrate_Step23_TheRetypeStepIsWiredToTheEmbeddedFile(t *testing.T) {
	f := newFakeStore()
	p := weaponPiece(1, "staff_sling", "staff_sling", "other_missile")
	f.t.Items[rowKey(1, p.ID)] = p
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	before, after := migrationDataSteps(repoSeeds(), typedCatalogue, catalogueDefs)
	run := &fakeMigrations{store: f}
	for _, m := range embedded {
		if m.name != catalogueWeaponTypesMigration {
			run.recorded = append(run.recorded, m.name)
		}
	}
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Contains(t, run.steps, "apply "+catalogueWeaponTypesMigration)
	assert.Equal(t, "staff_sling", f.t.Items[rowKey(1, p.ID)].WeaponType)
}
