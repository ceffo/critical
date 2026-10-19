package router_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// ADR 0004 step 17: a buff from a spell is served with its spellId (omitted
// otherwise); a spellId in an activation body is ignored, only a spell cast
// stamps it.
func TestBuffs_SpellIDOnTheWire(t *testing.T) {
	t.Run("served when the buff comes from a spell", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().ListStatus(mock.Anything, int64(1)).Return([]buffsvc.BuffStatus{
			{BuffEffect: buffsvc.BuffEffect{ID: "h", Name: "Haste", Duration: 4, SpellID: "haste"}, Applies: true},
			{BuffEffect: buffsvc.BuffEffect{ID: "g", Name: "Power Gem"}, Applies: true},
		}, nil)
		m.buffs.EXPECT().ListPowers(mock.Anything, int64(1)).Return(nil, nil)
		rec := do(h, "GET", "/api/buffs", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"buffs":[
			{"id":"h","name":"Haste","charges":0,"duration":4,"effects":{},"note":"","startRound":0,"chargesTotal":0,"spellId":"haste","applies":true},
			{"id":"g","name":"Power Gem","charges":0,"duration":0,"effects":{},"note":"","startRound":0,"chargesTotal":0,"applies":true}],
			"powers":[]}`,
			rec.Body.String())
	})

	t.Run("ignored in an activation body", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().Activate(mock.Anything, int64(1), buffsvc.BuffEffect{Name: "Haste", Duration: 4}).
			Return([]buffsvc.BuffEffect{}, nil).Once()
		m.buffs.EXPECT().ListStatus(mock.Anything, int64(1)).Return([]buffsvc.BuffStatus{}, nil).Maybe()
		m.buffs.EXPECT().ListPowers(mock.Anything, int64(1)).Return(nil, nil).Maybe()
		rec := do(h, "POST", "/api/buffs", `{"name":"Haste","duration":4,"spellId":"haste"}`)
		assert.Less(t, rec.Code, 300, rec.Body.String())
	})
}

// ADR 0004 step 20.5: a row for a spell cast on someone else or on an area
// is served with onOther (omitted for a buff on the character) and as not
// applying; an onOther in an activation body is ignored.
func TestBuffs_Step20_OnOtherOnTheWire(t *testing.T) {
	t.Run("served on a row on someone else", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().ListStatus(mock.Anything, int64(1)).Return([]buffsvc.BuffStatus{
			{BuffEffect: buffsvc.BuffEffect{ID: "w", Name: "Web", Duration: 30, SpellID: "web", OnOther: true}},
			{BuffEffect: buffsvc.BuffEffect{ID: "p", Name: "Polymorph Self", Duration: 300, SpellID: "polymorph_self"}, Applies: true},
		}, nil)
		m.buffs.EXPECT().ListPowers(mock.Anything, int64(1)).Return(nil, nil)
		rec := do(h, "GET", "/api/buffs", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"buffs":[
			{"id":"w","name":"Web","charges":0,"duration":30,"effects":{},"note":"","startRound":0,"chargesTotal":0,"spellId":"web","onOther":true,"applies":false},
			{"id":"p","name":"Polymorph Self","charges":0,"duration":300,"effects":{},"note":"","startRound":0,"chargesTotal":0,"spellId":"polymorph_self","applies":true}],
			"powers":[]}`,
			rec.Body.String())
	})

	t.Run("ignored in an activation body", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().Activate(mock.Anything, int64(1), buffsvc.BuffEffect{Name: "Web", Duration: 4}).
			Return([]buffsvc.BuffEffect{}, nil).Once()
		m.buffs.EXPECT().ListStatus(mock.Anything, int64(1)).Return([]buffsvc.BuffStatus{}, nil).Maybe()
		m.buffs.EXPECT().ListPowers(mock.Anything, int64(1)).Return(nil, nil).Maybe()
		rec := do(h, "POST", "/api/buffs", `{"name":"Web","duration":4,"onOther":true}`)
		assert.Less(t, rec.Code, 300, rec.Body.String())
	})
}

// ADR 0016 step 13: the effects list serves the item powers that are on in
// a list of their own, next to the buffs and never mixed into them: the
// item's id, the power's id and its effects (weapon-kind condition
// included), no name, no counters. Buff changes answer the same list.
func TestItemPowers_Step13_PowersOnTheWire(t *testing.T) {
	hammer := buffsvc.ItemPower{
		ItemID: "power_gem_dispel", ItemName: "Power Gem", PowerID: "thunder_hammer", MinStr: new(18),
		Effects: buffsvc.BuffEffects{WeaponBuff: new(5), Multiplier: new(2.0), WeaponKind: buffsvc.WeaponMelee},
	}
	const want = `{"buffs":[
		{"id":"s","name":"Strength","charges":0,"duration":5,"effects":{"str_bonus":6},"note":"","startRound":0,"chargesTotal":0,"applies":true}],
		"powers":[{"itemId":"power_gem_dispel","powerId":"thunder_hammer",
			"effects":{"weapon_buff":5,"multiplier":2,"weapon_kind":"melee"}}]}`
	status := []buffsvc.BuffStatus{{BuffEffect: buffsvc.BuffEffect{
		ID: "s", Name: "Strength", Duration: 5,
		Effects: buffsvc.BuffEffects{StrBonus: new(6)},
	}, Applies: true}}

	t.Run("GET", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().ListStatus(mock.Anything, int64(1)).Return(status, nil)
		m.buffs.EXPECT().ListPowers(mock.Anything, int64(1)).Return([]buffsvc.ItemPower{hammer}, nil)
		rec := do(h, "GET", "/api/buffs", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, want, rec.Body.String())
	})
	t.Run("a buff change answers the same list", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().Activate(mock.Anything, int64(1), mock.Anything).Return(nil, nil).Once()
		m.buffs.EXPECT().ListStatus(mock.Anything, int64(1)).Return(status, nil)
		m.buffs.EXPECT().ListPowers(mock.Anything, int64(1)).Return([]buffsvc.ItemPower{hammer}, nil)
		rec := do(h, "POST", "/api/buffs", `{"name":"Strength","duration":5,"effects":{"str_bonus":6}}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.JSONEq(t, want, rec.Body.String())
	})
	t.Run("no power on: an empty list", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().ListStatus(mock.Anything, int64(1)).Return(nil, nil)
		m.buffs.EXPECT().ListPowers(mock.Anything, int64(1)).Return(nil, nil)
		rec := do(h, "GET", "/api/buffs", "")
		assert.JSONEq(t, `{"buffs":[],"powers":[]}`, rec.Body.String())
	})
}
