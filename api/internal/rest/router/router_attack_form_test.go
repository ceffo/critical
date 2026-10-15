package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var bowForm = &combatsvc.AttackFormView{
	Form: combatsvc.AttackForm{
		WeaponID: "shortbow", PreviousWeaponID: "longsword", AmmoItemID: "a3", TargetAC: 4,
		TargetSize: combatsvc.SizeLarge, Situational: combatsvc.Situational{Value: 2, Label: "rear"},
	},
	AmmoFromDefault: true,
	AmmoChoices:     []combatsvc.AmmoChoice{{ItemID: "a3", Name: "Arrows +3", Magic: 3, Count: 2}},
	Preview: &combatsvc.AttackPreview{
		BaseThac0: 13, Modifiers: []combatsvc.Modifier{{Kind: combatsvc.ModDexterity, Detail: "17", Value: 2}},
		EffectiveThac0: 11, ToHit: 2, RollNeeded: 7, DamageBonus: 5,
	},
}

const bowFormJSON = `{"form":{"weaponId":"shortbow","previousWeaponId":"longsword","ammoItemId":"a3","targetAc":4,
	"targetSize":"l","situational":{"value":2,"label":"rear"}},"ammoFromDefault":true,
	"ammoChoices":[{"itemId":"a3","name":"Arrows +3","magic":3,"count":2}],
	"preview":{"baseThac0":13,"modifiers":[{"kind":"dexterity","detail":"17","value":2}],"effectiveThac0":11,
	"toHit":2,"rollNeeded":7,"damageBonus":5}}`

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
	assert.JSONEq(t, `{"form":{"weaponId":"","previousWeaponId":"","ammoItemId":"","targetAc":5,"targetSize":"sm",
		"situational":{"value":0,"label":""}},"ammoFromDefault":false,"ammoChoices":[],"preview":null}`, rec.Body.String())
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

func TestAttackForm_PatchErrors(t *testing.T) {
	h, _ := newRouter(t)
	assert.Equal(t, http.StatusBadRequest, do(h, "PATCH", "/api/attack/form", `nope`).Code)

	h, m := newRouter(t)
	m.cmb.EXPECT().UpdateAttackForm(mock.Anything, int64(1), mock.Anything).
		Return(nil, fmt.Errorf("wrapped: %w", domain.FieldErrorf("targetAc", "target AC must be between -20 and 20")))
	rec := do(h, "PATCH", "/api/attack/form", `{"targetAc":30}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"target AC must be between -20 and 20","field":"targetAc"}`, rec.Body.String())
}
