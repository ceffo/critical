package itemsvc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
)

// catalogueWeapon is a weapon row of the fixture catalogue (PHB Table 44, the real artifact's
// numbers and codes: seedcheck's TestRealCatalogue_Step13_5_TheFixtureCarriesTheRealArtifactsEquipment).
type catalogueWeapon struct {
	Code        string   `json:"code"`
	WeaponType  string   `json:"weapon_type"`
	Kind        string   `json:"kind"`
	DamageTypes []string `json:"damage_types"`
	Fires       []string `json:"fires"`
}

func fixtureWeapons(t *testing.T) []catalogueWeapon {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	var doc struct {
		Equipment struct {
			Weapons []catalogueWeapon `json:"weapons"`
		} `json:"equipment"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	require.NotEmpty(t, doc.Equipment.Weapons)
	return doc.Equipment.Weapons
}

// ADR 0037 step 21.2: one rule judges the pick and the wielding warning. For every catalogue row
// and every fixture class, the class may pick the row's code (Equipment.PickCodes, derived at
// import) exactly when that weapon, wielded alone, draws no weapon_not_allowed warning (ADR 0033
// step 20: the class's list by type or code, the cleric's blunt weapons by their damage types, a
// launcher by its ammunition). A class with no weapon limit (the fighter, p.26; the bard, p.41)
// may pick every row.
func TestWeaponPickCodes_Step21_2_AgreeWithTheWieldingWarnings(t *testing.T) {
	weapons := fixtureWeapons(t)
	for _, def := range cataloguetest.Definitions(t).Classes() {
		t.Run(def.ID, func(t *testing.T) {
			eq := def.Equipment
			for _, w := range weapons {
				piece := Item{
					ID: w.Code, Category: CategoryWeapon, State: StateWielded, CatalogueCode: w.Code,
					WeaponType: w.WeaponType, WeaponKind: w.Kind, DamageTypes: w.DamageTypes,
				}
				if len(w.Fires) > 0 {
					piece.Fires = w.Fires[0]
				}
				warned := slices.ContainsFunc(ClassWarnings([]classes.Equipment{eq}, ruleset.WeaponLift{}, OldClasses{}, []Item{piece}),
					func(x Warning) bool { return x.Code == WarnWeaponNotAllowed })
				assert.Equal(t, !warned, eq.WeaponPickAllowed(w.Code), "%s %s", def.ID, w.Code)
			}
			if !eq.Limited() {
				assert.Nil(t, eq.PickCodes, "no limit: any weapon")
			}
		})
	}
}
