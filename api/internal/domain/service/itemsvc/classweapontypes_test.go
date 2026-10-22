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
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
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

// ADR 0037 step 7: the weapon types each class may pick (derived at import, ADR 0040 step 1.6) are
// what ADR 0033 step 20 allows: a type the class's list names, the type of a weapon it names, or
// the type of a catalogue weapon that, wielded alone, draws no weapon_not_allowed warning (the
// cleric's blunt weapons, p.33; a launcher by its ammunition). A class with no weapon limit (the
// fighter, p.26; the bard, p.41) may pick every type.
func TestClassWeaponTypes_Step7_AgreesWithTheStep20Warnings(t *testing.T) {
	weapons := fixtureWeapons(t)
	for _, def := range cataloguetest.Definitions(t).Classes() {
		t.Run(def.ID, func(t *testing.T) {
			eq := def.Equipment
			allowed := map[string]bool{}
			for _, typ := range eq.WeaponTypes {
				allowed[typ] = true
			}
			for _, w := range eq.Weapons {
				allowed[w.WeaponType] = true
			}
			for _, w := range weapons {
				piece := Item{
					ID: w.Code, Category: CategoryWeapon, State: StateWielded, CatalogueCode: w.Code,
					WeaponType: w.WeaponType, WeaponKind: w.Kind, DamageTypes: w.DamageTypes,
				}
				if len(w.Fires) > 0 {
					piece.Fires = w.Fires[0]
				}
				if !slices.ContainsFunc(ClassWarnings(eq, true, []Item{piece}), func(x Warning) bool { return x.Code == WarnWeaponNotAllowed }) {
					allowed[w.WeaponType] = true
				}
			}
			var derived []string
			for _, typ := range WeaponTypes {
				if allowed[typ] || !eq.Limited() {
					derived = append(derived, typ)
				}
			}
			want := eq.PickTypes
			if want == nil {
				want = ruletablesvc.WeaponTypes
			}
			assert.Equal(t, want, derived)
		})
	}
}
