package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

var bowForm = &combatsvc.AttackFormView{
	Form: combatsvc.AttackForm{
		WeaponID: "shortbow", PreviousWeaponID: "longsword", AmmoItemID: "a3", TargetAC: new(4),
		TargetSize: combatsvc.SizeLarge, Situational: combatsvc.Situational{Value: 2, Label: "rear"},
	},
	AmmoFromDefault: true,
	AmmoChoices:     []combatsvc.AmmoChoice{{ItemID: "a3", Name: "Arrows +3", Magic: 3, Count: 2}},
	Preview: &combatsvc.AttackPreview{
		BaseThac0: 13, Modifiers: []combatsvc.Modifier{{Kind: combatsvc.ModDexterity, Detail: "17", Value: 2}},
		EffectiveThac0: 11, ToHit: 2, RollNeeded: new(7), DamageBonus: 5,
	},
}

const bowFormJSON = `{"form":{"weaponId":"shortbow","previousWeaponId":"longsword","ammoItemId":"a3","targetAc":4,
	"targetSize":"l","situational":{"value":2,"label":"rear"}},"ammoFromDefault":true,
	"ammoChoices":[{"itemId":"a3","name":"Arrows +3","magic":3,"count":2}],
	"preview":{"baseThac0":13,"modifiers":[{"kind":"dexterity","detail":"17","value":2}],"effectiveThac0":11,
	"toHit":2,"rollNeeded":7,"damageBonus":5},"maxRealAc":10}`

func TestAttackForm_Get(t *testing.T) {
	h, m := newRouter(t)
	m.cmb.EXPECT().GetAttackForm(mock.Anything, int64(1)).Return(bowForm, nil)
	rec := do(h, "GET", "/api/attack/form", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, bowFormJSON, rec.Body.String())
}

func TestAttackForm_GetWithoutWeaponHasNullPreview(t *testing.T) {
	h, m := newRouter(t)
	m.cmb.EXPECT().GetAttackForm(mock.Anything, int64(1)).Return(&combatsvc.AttackFormView{
		Form: combatsvc.DefaultAttackForm(), AmmoChoices: []combatsvc.AmmoChoice{},
	}, nil)
	rec := do(h, "GET", "/api/attack/form", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"form":{"weaponId":"","previousWeaponId":"","ammoItemId":"","targetAc":null,"targetSize":"sm",
		"situational":{"value":0,"label":""}},"ammoFromDefault":false,"ammoChoices":[],"preview":null,"maxRealAc":10}`, rec.Body.String())
}

// PATCH sends only the fields in the body.
func TestAttackForm_PatchPassesOnlyTheFieldsSent(t *testing.T) {
	h, m := newRouter(t)
	weapon, ac := "shortbow", 4
	size := combatsvc.SizeLarge
	m.cmb.EXPECT().UpdateAttackForm(mock.Anything, int64(1), combatsvc.AttackFormPatch{
		WeaponID: &weapon, TargetAC: &ac, TargetSize: &size, Situational: &combatsvc.Situational{Value: 2, Label: "rear"},
	}).Return(bowForm, nil)
	rec := do(h, "PATCH", "/api/attack/form", `{"weaponId":"shortbow","targetAc":4,"targetSize":"l","situational":{"value":2,"label":"rear"}}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, bowFormJSON, rec.Body.String())
}

// "targetAc": null makes the target AC unknown; a non-number is refused.
func TestAttackForm_PatchUnknownTargetAC(t *testing.T) {
	h, m := newRouter(t)
	m.cmb.EXPECT().UpdateAttackForm(mock.Anything, int64(1), combatsvc.AttackFormPatch{UnknownTargetAC: true}).
		Return(&combatsvc.AttackFormView{
			Form: combatsvc.AttackForm{WeaponID: "shortbow", TargetSize: combatsvc.SizeSmallMedium}, AmmoChoices: []combatsvc.AmmoChoice{},
			Preview: &combatsvc.AttackPreview{BaseThac0: 13, Modifiers: []combatsvc.Modifier{}, EffectiveThac0: 13},
		}, nil)
	rec := do(h, "PATCH", "/api/attack/form", `{"targetAc":null}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"form":{"weaponId":"shortbow","previousWeaponId":"","ammoItemId":"","targetAc":null,"targetSize":"sm",
		"situational":{"value":0,"label":""}},"ammoFromDefault":false,"ammoChoices":[],
		"preview":{"baseThac0":13,"modifiers":[],"effectiveThac0":13,"toHit":0,"rollNeeded":null,"damageBonus":0},"maxRealAc":10}`, rec.Body.String())

	h, _ = newRouter(t)
	rec = do(h, "PATCH", "/api/attack/form", `{"targetAc":"five"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"targetAc must be a whole number or null","code":"bad_request"}`, rec.Body.String())
}

func TestAttackForm_PatchErrors(t *testing.T) {
	h, _ := newRouter(t)
	assert.Equal(t, http.StatusBadRequest, do(h, "PATCH", "/api/attack/form", `nope`).Code)

	h, m := newRouter(t)
	m.cmb.EXPECT().UpdateAttackForm(mock.Anything, int64(1), mock.Anything).
		Return(nil, fmt.Errorf("wrapped: %w", domain.FieldErrorf("targetAc", "target AC must be between -20 and 20")))
	rec := do(h, "PATCH", "/api/attack/form", `{"targetAc":30}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"target AC must be between -20 and 20","field":"targetAc","code":"invalid_input"}`, rec.Body.String())
}
