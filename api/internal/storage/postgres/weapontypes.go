package postgres

import (
	"context"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// catalogueWeaponTypesMigration is the migration whose data step gives the stored catalogue
// weapons the type their catalogue row now names (retypeCatalogueWeapons), once, right after it.
const catalogueWeaponTypesMigration = "041_catalogue_weapon_types.sql"

// retypedFrom is the catch-all type a catalogue row named before it got a type of its own (ADR
// 0030 step 23: the staff sling, other_missile until catalogue format 22).
const retypedFrom = "other_missile"

// typesOfFormat22 are the weapon types catalogue format 22 added (ADR 0021 step 30): the rows of
// these types are the ones that were other_missile before.
var typesOfFormat22 = []string{"staff_sling"}

// retypeCatalogueWeapons is the data step after migration 041 (ADR 0030 step 23): every weapon
// piece of the code of a catalogue row of a type format 22 added, still typed other_missile, takes
// the row's type, as a pick of the row would type it now (ADR 0033 step 23). It reads the
// catalogue's rows, never a weapon code: a piece the player typed otherwise, a homebrew piece, the
// blowgun (still other_missile) and a piece of an older type the player set to other_missile are
// left as they are, and a second run changes nothing. No catalogue equipment (before format 7)
// changes nothing.
func retypeCatalogueWeapons(ctx context.Context, q storeq.Querier, catalogue []ruletablesvc.Weapon) error {
	for _, w := range catalogue {
		if !slices.Contains(typesOfFormat22, w.WeaponType) {
			continue
		}
		if _, err := q.RetypeCatalogueWeapons(ctx, storeq.RetypeCatalogueWeaponsParams{
			ToType: w.WeaponType, CatalogueCode: w.Code, FromType: retypedFrom,
		}); err != nil {
			return fmt.Errorf("retype the %s pieces: %w", w.Code, err)
		}
	}
	return nil
}
