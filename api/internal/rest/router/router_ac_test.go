package router_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
)

// ADR 0033 steps 10 and 13 on the wire: the derived AC, its rows as codes
// and sources (the web words them, ADR 0023), the override or null.
func TestAC_GetWireShape(t *testing.T) {
	h, m := newRouter(t)
	missile := -2
	m.ac.EXPECT().Get(mock.Anything, int64(1)).Return(&acsvc.AC{
		Value: 4, Missile: new(3),
		Rows: []acsvc.Row{
			{Term: acsvc.TermBase, ItemID: "chain_mail", Value: 5},
			{Term: acsvc.TermBase, Buff: "Armor", SpellID: "armor", Value: 6, Off: acsvc.OffArmourWorn},
			{Term: acsvc.TermArmourMagic},
			{Term: acsvc.TermShield, ItemID: "body_shield", Value: -1, MissileValue: &missile},
			{Term: acsvc.TermDexterity, Value: 0, Dexterity: 12},
		},
	}, nil).Once()
	rec := do(h, "GET", "/api/ac", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"ac":4,"missile":3,"floored":false,"override":null,"rows":[
		{"term":"base","itemId":"chain_mail","value":5},
		{"term":"base","buff":"Armor","spellId":"armor","value":6,"off":"armour_worn"},
		{"term":"armour_magic","value":0},
		{"term":"shield","itemId":"body_shield","value":-1,"missileValue":-2},
		{"term":"dexterity","value":0,"dexterity":12}]}`, rec.Body.String())
}

// ADR 0033 step 13: PUT sets the override and DELETE clears it, each
// answering the AC; a body without a value is a 400, a value out of range
// a 400 naming the field.
func TestAC_OverrideRoutes(t *testing.T) {
	h, m := newRouter(t)
	m.ac.EXPECT().SetOverride(mock.Anything, int64(1), acsvc.Override{Value: 4, Label: "Shield spell"}).Return(&acsvc.AC{
		Value: 2, Rows: []acsvc.Row{}, Override: &acsvc.Override{Value: 4, Label: "Shield spell"},
	}, nil).Once()
	rec := do(h, "PUT", "/api/ac/override", `{"value":4,"label":"Shield spell"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"ac":2,"floored":false,"rows":[],"override":{"value":4,"label":"Shield spell"}}`, rec.Body.String())

	m.ac.EXPECT().ClearOverride(mock.Anything, int64(1)).Return(&acsvc.AC{Value: 2, Rows: []acsvc.Row{}}, nil).Once()
	rec = do(h, "DELETE", "/api/ac/override", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"ac":2,"floored":false,"rows":[],"override":null}`, rec.Body.String())

	for _, body := range []string{`{}`, `{"label":"x"}`, `{"value":4,"extra":1}`} {
		rec = do(h, "PUT", "/api/ac/override", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
	m.ac.EXPECT().SetOverride(mock.Anything, int64(1), acsvc.Override{Value: 11}).
		Return(nil, &domain.FieldError{Field: "value", Message: acsvc.CodeOutOfRange}).Once()
	rec = do(h, "PUT", "/api/ac/override", `{"value":11}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"out_of_range","code":"invalid_input","field":"value"}`, rec.Body.String())
}
