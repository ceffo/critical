package composition_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.Len(t, list, 65)
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
	}, groups)

	// Step 4: each proficiency's cost by class, the crossover's +1 included.
	var costs map[string]map[string]int
	require.NoError(t, json.Unmarshal(raw["proficiencyCosts"], &costs))
	assert.Equal(t, 2, costs["fighter"]["tumbling"], "tumbling is a rogue proficiency: a fighter pays one more")
	assert.Equal(t, 1, costs["bard"]["tumbling"], "a bard learns the rogue group at the table's cost")
	assert.Equal(t, 2, costs["thief"]["blind_fighting"], "rogue and warrior: the thief's rogue group, Table 37's 2")
	for class, byID := range costs {
		assert.Len(t, byID, 64, "every pickable catalogue proficiency has a cost for %s", class)
		assert.NotContains(t, byID, "languages_modern", "step 6: a language is no pick of its own, so %s gets no cost for it", class)
	}
	assert.Len(t, costs, 5)
}

// ADR 0037 steps 7 and 9: each class option carries the weapon types the class
// may pick, in display order, with the specialization's slots for a class that
// specializes (2, a bow 3) and 0 for the others, so the weapon editor sends the
// server's cost.
func TestProficiencies_Step7_OptionsWeaponTypes(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	var body struct {
		Classes []struct {
			Class       string           `json:"class"`
			WeaponTypes []map[string]any `json:"weaponTypes"`
		} `json:"classes"`
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters/options", "", &body))
	byClass := map[string][]map[string]any{}
	for _, c := range body.Classes {
		byClass[c.Class] = c.WeaponTypes
	}
	fighter := byClass["fighter"]
	require.Len(t, fighter, 13, "a fighter may pick every weapon type")
	assert.Equal(t, map[string]any{"type": "long_bow", "specializationSlots": float64(3)}, fighter[0], "a bow's specialization spends 3")
	assert.Equal(t, map[string]any{"type": "long_sword", "specializationSlots": float64(2)}, fighter[9])
	assert.Equal(t, []map[string]any{
		{"type": "dagger", "specializationSlots": float64(0)},
		{"type": "dart", "specializationSlots": float64(0)},
		{"type": "sling", "specializationSlots": float64(0)},
		{"type": "other_melee", "specializationSlots": float64(0)},
	}, byClass["mage"], "a mage picks only its four types and never specializes")
	require.Len(t, byClass["bard"], 13)
	assert.Equal(t, float64(0), byClass["bard"][0]["specializationSlots"], "a bard picks any type but does not specialize")
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
