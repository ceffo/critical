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
		assert.Len(t, byID, 65, "every catalogue proficiency has a cost for %s", class)
	}
	assert.Len(t, costs, 5)
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
