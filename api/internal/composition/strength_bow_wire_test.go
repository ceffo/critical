package composition_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// A bow's Strength rating on the wire (ADR 0033 step 3) and in the attack (ADR 0002 step 4):
// PATCH /api/items/{id} sets it with an object and clears it with null, GET /api/items serves it
// (null for none), the attack form's preview takes the strength bow's adjustments, a rating on any
// other weapon is refused on the field, and a bow built for more exceptional Strength than the
// archer has is warned bow_too_strong.
func TestStrengthBow_Step3_TheRatingOnTheWireAndInTheAttack(t *testing.T) {
	tb, m := newTable(t)
	bob, bobID := tb.login("bob@example.com")
	ulf, err := m.AddCharacterOf(context.Background(), bobID, charactersvc.Character{
		Name: "Ulf", Class: "fighter", Str: 18, StrExceptional: 50, Dex: 12, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 30,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, tb.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(ulf)+`}`, nil))

	type rating struct {
		Score       int `json:"score"`
		Exceptional int `json:"exceptional"`
	}
	type piece struct {
		ID             string   `json:"id"`
		CatalogueCode  string   `json:"catalogueCode"`
		StrengthRating *rating  `json:"strengthRating"`
		Warnings       []string `json:"warnings"`
	}
	var created struct {
		Item piece `json:"item"`
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/items", `{"catalogueCode":"long_bow","equip":true}`, &created))
	bow := created.Item.ID
	assert.Nil(t, created.Item.StrengthRating, "a new bow is an ordinary one")
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/items", `{"catalogueCode":"flight_arrow","count":10}`, nil))
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/items", `{"catalogueCode":"long_sword"}`, &created))
	sword := created.Item.ID

	// strength is the attack form preview's strength modifier, nil for none.
	type modifier struct {
		Kind           string `json:"kind"`
		Detail         string `json:"detail"`
		Value          int    `json:"value"`
		StrExceptional int    `json:"strExceptional"`
	}
	strength := func() *modifier {
		t.Helper()
		var form struct {
			Preview *struct {
				Modifiers []modifier `json:"modifiers"`
			} `json:"preview"`
		}
		require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/attack/form", "", &form))
		require.NotNil(t, form.Preview)
		for _, mod := range form.Preview.Modifiers {
			if mod.Kind == "strength" {
				return &mod
			}
		}
		return nil
	}
	stored := func() piece {
		t.Helper()
		var list []piece
		require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/items", "", &list))
		for _, p := range list {
			if p.ID == bow {
				return p
			}
		}
		t.Fatalf("no bow %s", bow)
		return piece{}
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "PATCH", "/api/attack/form", `{"weaponId":"`+bow+`"}`, nil))
	assert.Nil(t, strength(), "an ordinary bow at 18/50: no Strength")

	var edited struct {
		Item piece `json:"item"`
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "PATCH", "/api/items/"+bow, `{"strengthRating":{"score":17,"exceptional":0}}`, &edited))
	assert.Equal(t, &rating{Score: 17}, edited.Item.StrengthRating)
	assert.Equal(t, &rating{Score: 17}, stored().StrengthRating)
	assert.Equal(t, &modifier{Kind: "strength", Detail: "17", Value: 1}, strength(), "rated 17: the 17 adjustments")

	require.Equal(t, http.StatusOK, tb.call(bob, "PATCH", "/api/items/"+bow, `{"strengthRating":{"score":18,"exceptional":50}}`, nil))
	assert.Equal(t, &modifier{Kind: "strength", Detail: "18", StrExceptional: 50, Value: 1}, strength(), "rated 18/50: Table 1's 18/50")
	assert.Empty(t, stored().Warnings)

	require.Equal(t, http.StatusOK, tb.call(bob, "PATCH", "/api/items/"+bow, `{"strengthRating":{"score":18,"exceptional":90}}`, nil))
	assert.Equal(t, []string{"bow_too_strong"}, stored().Warnings, "built for 18/90, drawn at 18/50")
	assert.Equal(t, &modifier{Kind: "strength", Detail: "18", StrExceptional: 50, Value: 1}, strength(), "the archer's own 18/50")

	require.Equal(t, http.StatusOK, tb.call(bob, "PATCH", "/api/items/"+bow, `{"notes":"yew"}`, nil))
	assert.Equal(t, &rating{Score: 18, Exceptional: 90}, stored().StrengthRating, "absent: kept")

	require.Equal(t, http.StatusOK, tb.call(bob, "PATCH", "/api/items/"+bow, `{"strengthRating":null}`, &edited))
	assert.Nil(t, edited.Item.StrengthRating)
	assert.Nil(t, stored().StrengthRating, "null: cleared")
	assert.Nil(t, strength())

	var refusal struct {
		Field string `json:"field"`
		Error string `json:"error"`
	}
	assert.Equal(t, http.StatusBadRequest, tb.call(bob, "PATCH", "/api/items/"+sword, `{"strengthRating":{"score":17,"exceptional":0}}`, &refusal))
	assert.Equal(t, "strengthRating", refusal.Field)
	assert.Equal(t, "not_allowed", refusal.Error)
}
