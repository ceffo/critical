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
			D20Roll: 11, D100CritRoll: 50, D100MultiplierRoll: 63, CritMultiplier: 1, BaseThac0: 13, EffectiveThac0: 11,
			TargetACNeeded: 0, HitsAC: 0, CritsFromAC: 50, Hit: true, HitMargin: 7, Modifiers: []combatsvc.Modifier{},
			BaseDamage: 5, DamageDice: "1d8", SpellsBuff: 1, WeaponBuff: 2, Multipliers: 1,
			DamageTerms: []combatsvc.DamageTerm{}, TotalDamage: 7, UnroundedDamage: 7,
			WeaponID: "longsword", WeaponName: "Long Sword +2", TargetAC: 7, ACKnown: true, TargetSize: combatsvc.SizeSmallMedium,
			RolledTargetAC: &prevAC, RolledHit: &prevHit, RolledCrit: &prevCrit, LogEntryID: 7,
		}, nil)
	rec := do(h, "POST", "/api/roll/attack/7/adjust", `{"targetAc":7}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"d20Roll":11,"d100CritRoll":50,"d100MultiplierRoll":63,"isCrit":false,"critMultiplier":1,"critPlaceholder":false,"critFrom":0,"critTo":0,
		"baseThac0":13,"modifiers":[],"effectiveThac0":11,"targetACNeeded":0,"hitsAc":0,"critsFromAc":50,"hit":true,"hitMargin":7,
		"baseDamage":5,"damageDice":"1d8","spellsBuff":1,"weaponBuff":2,"multipliers":1,"diceMultiplier":0,"strengthBonus":0,"bonusDamage":0,
		"damageTerms":[],"totalDamage":7,"unroundedDamage":7,
		"weaponId":"longsword","weaponName":"Long Sword +2","targetAc":7,"acKnown":true,"targetSize":"sm",
		"rolledTargetAc":5,"rolledHit":false,"rolledCrit":false,"logEntryId":7}`, rec.Body.String())
}

// POST /api/roll/attack/{id}/verdict passes the GM's hit or miss to the
// service; an attack against an unknown AC has a null targetAc and its
// verdict (ADR 0012 step 4).
func TestAttackVerdict_WireShape(t *testing.T) {
	h, m := newRouter(t)
	m.cmb.EXPECT().SetAttackVerdict(mock.Anything, int64(1), int64(7), true).
		Return(&combatsvc.RollResult{
			D20Roll: 11, D100CritRoll: 50, D100MultiplierRoll: 63, CritMultiplier: 1, BaseThac0: 13, EffectiveThac0: 11,
			TargetACNeeded: 0, HitsAC: 0, CritsFromAC: 50, Hit: true, Modifiers: []combatsvc.Modifier{},
			BaseDamage: 5, DamageDice: "1d8", SpellsBuff: 1, WeaponBuff: 2, Multipliers: 1,
			DamageTerms: []combatsvc.DamageTerm{}, TotalDamage: 7, UnroundedDamage: 7,
			WeaponID: "longsword", WeaponName: "Long Sword +2", TargetSize: combatsvc.SizeSmallMedium,
			RolledACUnknown: true, Verdict: combatsvc.VerdictHit, LogEntryID: 7,
		}, nil)
	rec := do(h, "POST", "/api/roll/attack/7/verdict", `{"hit":true}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"d20Roll":11,"d100CritRoll":50,"d100MultiplierRoll":63,"isCrit":false,"critMultiplier":1,"critPlaceholder":false,"critFrom":0,"critTo":0,
		"baseThac0":13,"modifiers":[],"effectiveThac0":11,"targetACNeeded":0,"hitsAc":0,"critsFromAc":50,"hit":true,"hitMargin":0,
		"baseDamage":5,"damageDice":"1d8","spellsBuff":1,"weaponBuff":2,"multipliers":1,"diceMultiplier":0,"strengthBonus":0,"bonusDamage":0,
		"damageTerms":[],"totalDamage":7,"unroundedDamage":7,
		"weaponId":"longsword","weaponName":"Long Sword +2","targetAc":null,"acKnown":false,"rolledAcUnknown":true,"verdict":"hit",
		"targetSize":"sm","logEntryId":7}`, rec.Body.String())
}

func TestAttackVerdict_BadRequests(t *testing.T) {
	tests := []struct {
		name, path, body, want string
	}{
		{"missing hit", "/api/roll/attack/7/verdict", `{}`, "hit is required"},
		{"bad id", "/api/roll/attack/0/verdict", `{"hit":true}`, "invalid log entry id"},
		{"bad body", "/api/roll/attack/7/verdict", `{"hit":"yes"}`, "invalid request body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newRouter(t)
			rec := do(h, "POST", tt.path, tt.body)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"error":"`+tt.want+`","code":"bad_request"}`, rec.Body.String())
		})
	}
}

// POST /api/roll/attack without target_ac (or with null) is an attack
// against an unknown AC (ADR 0012 step 1).
func TestResolveAttack_TargetACOptional(t *testing.T) {
	for _, body := range []string{`{"weapon_id":"longsword"}`, `{"weapon_id":"longsword","target_ac":null}`} {
		h, m := newRouter(t)
		m.cmb.EXPECT().ResolveAttack(mock.Anything, int64(1), combatsvc.AttackRequest{WeaponID: "longsword"}).
			Return(&combatsvc.RollResult{WeaponID: "longsword", Modifiers: []combatsvc.Modifier{}, DamageTerms: []combatsvc.DamageTerm{}}, nil).Once()
		rec := do(h, "POST", "/api/roll/attack", body)
		require.Equal(t, http.StatusOK, rec.Code, body)
	}
	h, m := newRouter(t)
	m.cmb.EXPECT().ResolveAttack(mock.Anything, int64(1), combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(-3)}).
		Return(&combatsvc.RollResult{WeaponID: "longsword", TargetAC: -3, ACKnown: true}, nil).Once()
	rec := do(h, "POST", "/api/roll/attack", `{"weapon_id":"longsword","target_ac":-3}`)
	require.Equal(t, http.StatusOK, rec.Code)
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
			assert.JSONEq(t, `{"error":"`+tt.want+`","code":"bad_request"}`, rec.Body.String())
		})
	}
}
