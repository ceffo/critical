package composition_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
)

// ADR 0037 steps 1 and 2: GET /api/characters/options carries the non-weapon
// catalogue and Table 38's crossovers. The wire shape is pinned key by key: the
// web reads exactly these.
func TestProficiencies_Step2_OptionsWireShape(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	var raw map[string]json.RawMessage
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters/options", "", &raw))

	var list []map[string]any
	require.NoError(t, json.Unmarshal(raw["proficiencies"], &list))
	require.Len(t, list, 66)
	byID := map[string]map[string]any{}
	for _, p := range list {
		byID[p["id"].(string)] = p
	}
	assert.Equal(t, map[string]any{
		"id": "navigation", "ability": "int", "modifier": float64(-2), "slots": float64(1),
		"groups": []any{"priest", "warrior", "wizard"},
	}, byID["navigation"], "a proficiency of several groups is one row; the keys are id, ability, modifier, slots, groups")
	assert.Equal(t, map[string]any{
		"id": "blind_fighting", "modifier": float64(0), "slots": float64(2), "groups": []any{"rogue", "warrior"},
	}, byID["blind_fighting"], "a proficiency with no check has no ability")

	var groups map[string][]string
	require.NoError(t, json.Unmarshal(raw["proficiencyGroups"], &groups))
	assert.Equal(t, map[string][]string{
		"fighter": {"warrior", "general"}, "mage": {"wizard", "general"}, "cleric": {"priest", "general"},
		"thief": {"rogue", "general"}, "bard": {"rogue", "warrior", "wizard", "general"},
		"psionicist": {"psionicist", "general"},
	}, groups)

	// Step 4: each proficiency's cost by class, the crossover's +1 included.
	var costs map[string]map[string]int
	require.NoError(t, json.Unmarshal(raw["proficiencyCosts"], &costs))
	assert.Equal(t, 2, costs["fighter"]["tumbling"], "tumbling is a rogue proficiency: a fighter pays one more")
	assert.Equal(t, 1, costs["bard"]["tumbling"], "a bard learns the rogue group at the table's cost")
	assert.Equal(t, 2, costs["thief"]["blind_fighting"], "rogue and warrior: the thief's rogue group, Table 37's 2")
	assert.Equal(t, 1, costs["psionicist"]["rejuvenation"], "ADR 0047 item 6: Rejuvenation is a psionicist proficiency, one slot")
	assert.Equal(t, 2, costs["fighter"]["rejuvenation"], "outside its group a pick costs one more")
	for class, byID := range costs {
		assert.Len(t, byID, 65, "every pickable catalogue proficiency has a cost for %s", class)
		assert.NotContains(t, byID, "languages_modern", "step 6: a language is no pick of its own, so %s gets no cost for it", class)
	}
	assert.Len(t, costs, 6)
}

// ADR 0037 steps 21.2, 21.5 and 21.6: each class option of GET /api/characters/options carries
// the catalogue weapons the class may pick (weapons, in place of weaponTypes), in offer order, with
// the specialization's slots for a class that specializes (its type's: a bow 3, the composite long
// bow as the long bow, 2 otherwise) and 0 for the others, so the weapon editor sends the server's
// cost: a fighter every row, the mage its five codes, the cleric its blunt rows and slings.
func TestProficiencies_Step21_6_OptionsWeapons(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	type option struct {
		Code                string `json:"code"`
		SpecializationSlots int    `json:"specializationSlots"`
	}
	var body struct {
		Classes []struct {
			Class   string   `json:"class"`
			Weapons []option `json:"weapons"`
		} `json:"classes"`
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters/options", "", &body))
	byClass := map[string][]option{}
	for _, c := range body.Classes {
		byClass[c.Class] = c.Weapons
	}
	eq, ok := cataloguetest.Tables(t).Equipment()
	require.True(t, ok)
	fighter := byClass["fighter"]
	require.Len(t, fighter, len(eq.Weapons), "a fighter may pick every catalogue weapon")
	assert.Equal(t, option{Code: "composite_long_bow", SpecializationSlots: 3}, fighter[0], "a bow's specialization spends 3")
	assert.Contains(t, fighter, option{Code: "long_sword", SpecializationSlots: 2})
	assert.Contains(t, fighter, option{Code: "heavy_horse_lance", SpecializationSlots: 2})
	assert.Equal(t, []option{{Code: "dagger"}, {Code: "dart"}, {Code: "sling"}, {Code: "knife"}, {Code: "quarterstaff"}},
		byClass["mage"], "a mage picks only its five weapons and never specializes")
	var cleric []string
	for _, o := range byClass["cleric"] {
		cleric = append(cleric, o.Code)
	}
	assert.ElementsMatch(t, []string{
		"club", "footmans_flail", "horsemans_flail", "footmans_mace", "horsemans_mace", "morning_star",
		"quarterstaff", "warhammer", "bec_de_corbin", "lucern_hammer", "sling", "staff_sling",
	}, cleric, "the cleric's blunt weapons and its slings by their bullets")
	require.Len(t, byClass["bard"], len(eq.Weapons))
	assert.Zero(t, byClass["bard"][0].SpecializationSlots, "a bard picks any weapon but does not specialize")
}

// The fixture's copy is the real seed (api/testdata/README: same format; this
// one is the data of a rules table, so it must not drift).
func TestProficiencies_TheFixtureIsTheRealSeed(t *testing.T) {
	real, err := os.ReadFile(filepath.Join("..", "..", "..", "seeds", "proficiencies.json"))
	require.NoError(t, err)
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "seeds", "proficiencies.json"))
	require.NoError(t, err)
	assert.Equal(t, string(real), string(fixture))
}
