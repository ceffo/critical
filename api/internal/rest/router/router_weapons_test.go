package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const bowJSON = `{"id":"long_bow","name":"Long Bow","damage_sm":"1d8","damage_l":"1d8","magic_to_hit":1,"magic_damage":2,
	"kind":"missile","two_handed":true,"ammo_kind":"arrow"}`

var longBow = weaponsvc.Weapon{
	ID: "long_bow", Name: "Long Bow", DamageSM: "1d8", DamageL: "1d8", MagicToHit: 1, MagicDamage: 2,
	Kind: "missile", TwoHanded: true, AmmoKind: "arrow",
}

func TestWeapons_Create(t *testing.T) {
	h, m := newRouter(t)
	m.weapons.EXPECT().Create(mock.Anything, int64(1), weaponsvc.WeaponInput{
		Name: "Long Bow", DamageSM: "1d8", DamageL: "1d8", MagicToHit: 1, MagicDamage: 2, Kind: "missile", TwoHanded: true, AmmoKind: "arrow",
	}).Return(&weaponsvc.ChangeResult{Weapon: longBow, LogEntryID: 12}, nil)
	rec := do(h, "POST", "/api/weapons", `{"name":"Long Bow","damage_sm":"1d8","damage_l":"1d8","magic_to_hit":1,"magic_damage":2,"kind":"missile","two_handed":true,"ammo_kind":"arrow"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"weapon":`+bowJSON+`,"logEntryId":12}`, rec.Body.String())
}

// An invalid field comes back as 400 with the message and the field name.
func TestWeapons_CreateInvalidNamesTheField(t *testing.T) {
	h, m := newRouter(t)
	m.weapons.EXPECT().Create(mock.Anything, int64(1), mock.Anything).
		Return(nil, fmt.Errorf("wrapped: %w", &domain.FieldError{Field: "damage_sm", Message: "Damage must look like 1d8 or 2d4"}))
	rec := do(h, "POST", "/api/weapons", `{"name":"X","damage_sm":"2x6","kind":"melee"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"Damage must look like 1d8 or 2d4","field":"damage_sm"}`, rec.Body.String())
}

func TestWeapons_UpdatePatchesOnlyTheFieldsSent(t *testing.T) {
	h, m := newRouter(t)
	three := 3
	m.weapons.EXPECT().Update(mock.Anything, int64(1), "long_bow", weaponsvc.WeaponPatch{MagicToHit: &three, MagicDamage: &three}).
		Return(&weaponsvc.ChangeResult{Weapon: longBow, LogEntryID: 13}, nil)
	rec := do(h, "PATCH", "/api/weapons/long_bow", `{"magic_to_hit":3,"magic_damage":3}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"weapon":`+bowJSON+`,"logEntryId":13}`, rec.Body.String())
}

func TestWeapons_Delete(t *testing.T) {
	h, m := newRouter(t)
	m.weapons.EXPECT().Delete(mock.Anything, int64(1), "long_bow").Return(&weaponsvc.ChangeResult{Weapon: longBow, LogEntryID: 14}, nil)
	rec := do(h, "DELETE", "/api/weapons/long_bow", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"weapon":`+bowJSON+`,"logEntryId":14}`, rec.Body.String())
}

func TestWeapons_EditErrors(t *testing.T) {
	for _, body := range []string{`nope`, `{"bogus":1}`, `{"magic_to_hit":"x"}`} {
		h, _ := newRouter(t)
		assert.Equal(t, http.StatusBadRequest, do(h, "PATCH", "/api/weapons/w", body).Code, body)
		assert.Equal(t, http.StatusBadRequest, do(h, "POST", "/api/weapons", body).Code, body)
	}
	h, m := newRouter(t)
	m.weapons.EXPECT().Update(mock.Anything, int64(1), "x", mock.Anything).Return(nil, fmt.Errorf("%w: x", weaponsvc.ErrWeaponNotFound))
	m.weapons.EXPECT().Delete(mock.Anything, int64(1), "x").Return(nil, fmt.Errorf("%w: x", weaponsvc.ErrWeaponNotFound))
	rec := do(h, "PATCH", "/api/weapons/x", `{"name":"y"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"weapon not found"}`, rec.Body.String())
	assert.Equal(t, http.StatusNotFound, do(h, "DELETE", "/api/weapons/x", "").Code)
}
