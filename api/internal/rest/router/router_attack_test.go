package router_test

import (
	"net/http"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// POST /api/roll/attack/{id}/adjust passes the entry id and the corrected
// target AC to the service and returns the recomputed attack with its
// previous target AC and outcome.
func TestAdjustAttack_WireShape(t *testing.T) {
	h, m := newRouter(t)
	prevAC, prevHit, prevCrit := 5, false, false
	m.cmb.EXPECT().AdjustAttackAC(mock.Anything, int64(1), int64(7), 7).
		Return(&combatsvc.RollResult{
			D20Roll: 11, D100CritRoll: 50, CritMultiplier: 1, BaseThac0: 13, EffectiveThac0: 11,
			TargetACNeeded: 0, Hit: true, HitMargin: 7, Modifiers: []combatsvc.Modifier{},
			BaseDamage: 5, DamageDice: "1d8", SpellsBuff: 1, WeaponBuff: 2, Multipliers: 1,
			DamageTerms: []combatsvc.DamageTerm{}, TotalDamage: 7, DamageBreakdown: "((5 × 1.0) + 2) × 1.0 + 0 = 7",
			WeaponID: "longsword", WeaponName: "Long Sword +2", TargetAC: 7, TargetSize: combatsvc.SizeSmallMedium,
			PreviousTargetAC: &prevAC, PreviousHit: &prevHit, PreviousCrit: &prevCrit, LogEntryID: 7,
		}, nil)
	rec := do(h, "POST", "/api/roll/attack/7/adjust", `{"targetAc":7}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"d20Roll":11,"d100CritRoll":50,"isCrit":false,"critMultiplier":1,"critPlaceholder":false,"critFrom":0,"critTo":0,
		"baseThac0":13,"modifiers":[],"effectiveThac0":11,"targetACNeeded":0,"hit":true,"hitMargin":7,
		"baseDamage":5,"damageDice":"1d8","spellsBuff":1,"weaponBuff":2,"multipliers":1,"strengthBonus":0,"bonusDamage":0,
		"damageTerms":[],"totalDamage":7,"damageBreakdown":"((5 × 1.0) + 2) × 1.0 + 0 = 7",
		"weaponId":"longsword","weaponName":"Long Sword +2","targetAc":7,"targetSize":"sm",
		"previousTargetAc":5,"previousHit":false,"previousCrit":false,"logEntryId":7}`, rec.Body.String())
}

func TestAdjustAttack_BadRequests(t *testing.T) {
	tests := []struct {
		name, path, body, want string
	}{
		{"missing target AC", "/api/roll/attack/7/adjust", `{}`, "targetAc is required"},
		{"bad id", "/api/roll/attack/x/adjust", `{"targetAc":7}`, "invalid log entry id"},
		{"bad body", "/api/roll/attack/7/adjust", `{`, "invalid request body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newRouter(t)
			rec := do(h, "POST", tt.path, tt.body)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"error":"`+tt.want+`"}`, rec.Body.String())
		})
	}
}
