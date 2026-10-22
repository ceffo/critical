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

// ADR 0037 step 7: the weapon types each class may pick (ruletablesvc's committed lists) are
// what ADR 0033 step 20's class table allows: a type the class's PHB list names (mage p.30,
// thief p.39), the type of a weapon it names (the knife, the quarterstaff, the club, the broad
// sword, the lasso), or the type of a catalogue weapon that, wielded alone, draws no
// weapon_not_allowed warning (the cleric's blunt weapons, p.33; a launcher by its ammunition).
// A class with no rule (the fighter, p.26; the bard, p.41) may pick every type.
func TestClassWeaponTypes_Step7_FromTheStep20TableAndTheCatalogue(t *testing.T) {
	weapons := fixtureWeapons(t)
	for _, class := range cataloguetest.Definitions(t).ClassIDs() {
		t.Run(class, func(t *testing.T) {
			rule, ruled := classRules[class]
			allowed := map[string]bool{}
			for _, typ := range rule.weaponTypes {
				allowed[typ] = true
			}
			for _, w := range rule.weapons {
				allowed[w.weaponType] = true
			}
			for _, w := range weapons {
				piece := Item{
					ID: w.Code, Category: CategoryWeapon, State: StateWielded, CatalogueCode: w.Code,
					WeaponType: w.WeaponType, WeaponKind: w.Kind, DamageTypes: w.DamageTypes,
				}
				if len(w.Fires) > 0 {
					piece.Fires = w.Fires[0]
				}
				if !slices.ContainsFunc(ClassWarnings([]string{class}, []Item{piece}), func(x Warning) bool { return x.Code == WarnWeaponNotAllowed }) {
					allowed[w.WeaponType] = true
				}
			}
			var derived []string
			for _, typ := range WeaponTypes {
				if allowed[typ] || !ruled {
					derived = append(derived, typ)
				}
			}
			want := ruletablesvc.ClassWeaponTypes(class)
			if want == nil {
				want = WeaponTypes
			}
			assert.Equal(t, want, derived)
		})
	}
}
