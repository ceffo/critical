package composition_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// ADR 0037 step 8 on the wire: GET /api/weapons and GET /api/items carry each weapon's
// `nonproficientPenalty` (Table 34's for the class group when its type has no pick, 0 once
// picked), so the attack form and the weapon card show it before a roll; a weapon pick saved
// through PUT /api/proficiencies clears it on the next read.
func TestNonproficientPenalty_Step8_OnTheWeaponsAndTheItems(t *testing.T) {
	tb, m := newTable(t)
	bob, bobID := tb.login("bob@example.com")
	bram, err := m.AddCharacterOf(context.Background(), bobID, charactersvc.Character{
		Name: "Bram", Class: "fighter", Str: 16, Dex: 12, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 30,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, tb.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(bram)+`}`, nil))
	for _, code := range []string{"long_sword", "dagger"} {
		require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/items", `{"catalogueCode":"`+code+`"}`, nil), code)
	}

	penalties := func() (weapons, items map[string]int) {
		var ws []struct {
			Type    string `json:"weapon_type"`
			Penalty int    `json:"nonproficientPenalty"`
		}
		require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/weapons", "", &ws))
		var list []struct {
			Type    string `json:"weaponType"`
			Penalty int    `json:"nonproficientPenalty"`
		}
		require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/items", "", &list))
		weapons, items = map[string]int{}, map[string]int{}
		for _, w := range ws {
			weapons[w.Type] = w.Penalty
		}
		for _, it := range list {
			items[it.Type] = it.Penalty
		}
		return weapons, items
	}
	weapons, items := penalties()
	assert.Equal(t, map[string]int{"long_sword": -2, "dagger": -2}, weapons, "a warrior without picks: -2 on each")
	assert.Equal(t, -2, items["long_sword"])

	require.Equal(t, http.StatusOK, tb.call(bob, "PUT", "/api/proficiencies", `{"weapon":[{"type":"long_sword","slots":1}]}`, nil))
	weapons, items = penalties()
	assert.Equal(t, map[string]int{"long_sword": 0, "dagger": -2}, weapons)
	assert.Equal(t, 0, items["long_sword"])
	assert.Equal(t, -2, items["dagger"])
}
