package router_test

import (
	"net/http"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// POST /api/roll/save/{id}/adjust passes the entry id, adjustment and reason
// to the service and returns the recomputed save with its log entry id.
func TestAdjustSave_WireShape(t *testing.T) {
	h, m := newRouter(t)
	m.cmb.EXPECT().AdjustSave(mock.Anything, int64(1), int64(7), combatsvc.SaveAdjustment{Value: 4, Reason: "Gaia fire"}).
		Return(&combatsvc.RollResult{
			D20Roll: 12, BaseThac0: 14, EffectiveThac0: 14, Hit: true, HitMargin: 2,
			Modifiers:       []combatsvc.Modifier{{Kind: combatsvc.ModAdjustment, Source: "Adjustment", Detail: "Gaia fire", Value: 4}},
			DamageBreakdown: "Roll: 12 + Mod: 0 + Adj: +4 = 16 vs Target: 14 (Spells)",
			SaveCategory:    combatsvc.SaveSpell, Adjustment: 4, AdjustmentReason: "Gaia fire", LogEntryID: 7,
		}, nil)
	rec := do(h, "POST", "/api/roll/save/7/adjust", `{"adjustment":4,"reason":"Gaia fire"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"d20Roll":12,"d100CritRoll":0,"isCrit":false,"critMultiplier":0,"critPlaceholder":false,"critFrom":0,"critTo":0,
		"baseThac0":14,"modifiers":[{"kind":"adjustment","source":"Adjustment","detail":"Gaia fire","value":4}],
		"effectiveThac0":14,"targetACNeeded":0,"hit":true,"hitMargin":2,
		"baseDamage":0,"damageDice":"","spellsBuff":0,"weaponBuff":0,"multipliers":0,"strengthBonus":0,"bonusDamage":0,
		"damageTerms":[],"totalDamage":0,"damageBreakdown":"Roll: 12 + Mod: 0 + Adj: +4 = 16 vs Target: 14 (Spells)",
		"targetAc":0,"saveCategory":"spell","adjustment":4,"adjustmentReason":"Gaia fire","logEntryId":7}`, rec.Body.String())
}

// A save always carries its adjustment, 0 included, so the card can show it.
func TestRollSave_SendsZeroAdjustment(t *testing.T) {
	h, m := newRouter(t)
	m.cmb.EXPECT().ResolveSavingThrow(mock.Anything, int64(1), combatsvc.SaveSpell).
		Return(&combatsvc.RollResult{D20Roll: 12, SaveCategory: combatsvc.SaveSpell, LogEntryID: 7}, nil)
	rec := do(h, "POST", "/api/roll/save", `{"category":"spell"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"adjustment":0`)
	assert.Contains(t, rec.Body.String(), `"logEntryId":7`)
}
