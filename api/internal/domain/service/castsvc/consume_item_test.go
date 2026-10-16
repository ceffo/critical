package castsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// ADR 0008 step 10: a spell with consume_item and attack_roll (Chromatic Orb
// in the seed: a gem) spends one item of that type, rolls a to-hit with no
// crit and shows hit or miss and the item used (no gem effect is modelled).
var (
	diamonds = itemsvc.Item{ID: "diamond", Name: localized.Of("Diamond"), Type: "gem", Behaviour: itemsvc.BehaviourStock, Count: 41}
	rubies   = itemsvc.Item{ID: "ruby", Name: localized.Of("Ruby"), Type: "gem", Behaviour: itemsvc.BehaviourStock, Count: 2}
	powerGem = itemsvc.Item{ID: "power_gem_dispel", Name: localized.Of("Power Gem: Dispel Magic"), Type: "gem", Behaviour: itemsvc.BehaviourDaily, Count: 1, MaxCount: 1}
)

func (f *fixture) gemSpellSetup(items ...itemsvc.Item) {
	f.prepared(slot("chromatic_orb", "Chromatic Orb", 1))
	f.items.EXPECT().List(mock.Anything).Return(items, nil)
}

func (f *fixture) spend(it itemsvc.Item) {
	left := it
	left.Count--
	f.items.EXPECT().Use(mock.Anything, it.ID).Return(left, nil).Once()
}

func TestCast_ConsumeItemAttackRoll_HitShowsTheItemSpent(t *testing.T) {
	f := newFixture(t) // no dice: the spell's level damage must not be rolled
	f.gemSpellSetup(powerGem, diamonds)
	f.spend(diamonds)
	f.attacks.EXPECT().RollSpellToHit(mock.Anything, charID, 4).Return(&combatsvc.ToHitRoll{
		D20Roll: 20, BaseThac0: 12, EffectiveThac0: 12, TargetAC: 4, TargetACNeeded: 8, Hit: true, HitMargin: 12,
	}, nil)

	ac := 4
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{TargetAC: &ac})
	require.NoError(t, err)
	require.NotNil(t, res.Consumed)
	require.NotNil(t, res.Attack)
	assert.Nil(t, res.Damage, "no damage is rolled")
	assert.Empty(t, res.TextOnly, "the cast did something")
	assert.Equal(t, castsvc.ConsumedItem{ItemID: "diamond", Name: "Diamond", Remaining: 40}, *res.Consumed)
	assert.True(t, res.Attack.Hit)
	require.Len(t, f.entries, 1)
	assert.Contains(t, keys(f.entries[0].Snapshots), "items:diamond", "voiding gives the gem back")
	assert.JSONEq(t, `{"itemId":"diamond","name":"Diamond","remaining":40}`, field(t, f.entries[0].Payload, "item"))
	assert.JSONEq(t, `{"d20":20,"targetAc":4,"hit":true}`, field(t, f.entries[0].Payload, "attack"))
}

func TestCast_ConsumeItemAttackRoll_MissSpendsTheItemNoEffect(t *testing.T) {
	f := newFixture(t)
	f.gemSpellSetup(diamonds)
	f.spend(diamonds)
	f.attacks.EXPECT().RollSpellToHit(mock.Anything, charID, 4).Return(&combatsvc.ToHitRoll{D20Roll: 3, TargetAC: 4, TargetACNeeded: 8}, nil)

	ac := 4
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{TargetAC: &ac})
	require.NoError(t, err)
	assert.False(t, res.Attack.Hit)
	assert.Equal(t, 40, res.Consumed.Remaining)
}

func TestCast_ConsumeItemAttackRoll_Refusals(t *testing.T) {
	ac := 4
	tests := []struct {
		name  string
		items []itemsvc.Item
		req   castsvc.CastRequest
	}{
		{"no gem left", []itemsvc.Item{powerGem, {ID: "diamond", Name: localized.Of("Diamond"), Type: "gem", Behaviour: itemsvc.BehaviourStock}}, castsvc.CastRequest{TargetAC: &ac}},
		{"several gems, none chosen", []itemsvc.Item{diamonds, rubies}, castsvc.CastRequest{TargetAC: &ac}},
		{"a gem that is not in the inventory", []itemsvc.Item{diamonds}, castsvc.CastRequest{TargetAC: &ac, ItemID: "emerald"}},
		{"no target AC", []itemsvc.Item{diamonds}, castsvc.CastRequest{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.prepared(slot("chromatic_orb", "Chromatic Orb", 1))
			f.items.EXPECT().List(mock.Anything).Return(tt.items, nil).Maybe()
			_, err := f.svc.Cast(context.Background(), charID, 0, tt.req)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.Empty(t, f.entries, "nothing is logged; the transaction rolls the slot back")
		})
	}
}

func TestCast_ConsumeItem_PicksTheChosenItem(t *testing.T) {
	f := newFixture(t)
	f.gemSpellSetup(diamonds, rubies)
	f.spend(rubies)
	f.attacks.EXPECT().RollSpellToHit(mock.Anything, charID, 4).Return(&combatsvc.ToHitRoll{D20Roll: 19, Hit: true}, nil)

	ac := 4
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{TargetAC: &ac, ItemID: "ruby"})
	require.NoError(t, err)
	assert.Equal(t, castsvc.ConsumedItem{ItemID: "ruby", Name: "Ruby", Remaining: 1}, *res.Consumed)
}

// A spell received from someone else: the item and the to-hit are the
// caster's, nothing is spent or rolled here.
func TestReceive_CastMechanicsAreTheCasters(t *testing.T) {
	f := newFixture(t) // no items, no attacks, no dice expected
	res, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "chromatic_orb", CasterLevel: 5})
	require.NoError(t, err)
	assert.Nil(t, res.Consumed)
	assert.Nil(t, res.Attack)
	assert.Equal(t, castsvc.TextOnlyUtility, res.TextOnly)
}

// field returns the raw JSON of a top-level key of payload.
func field(t *testing.T, payload []byte, key string) string {
	t.Helper()
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &m))
	require.Contains(t, m, key)
	return string(m[key])
}

// A blank extra slot cast as a spell with an attack roll (Chromatic Orb in a
// level 1 extra slot) applies its cast mechanics like a prepared one; without
// a target AC it is refused and nothing is spent or saved.
func TestCast_ExtraSlot_AttackRollSpell(t *testing.T) {
	extra := spellsvc.SpellSlot{Level: 1, Extra: true}
	ac := 6

	f := newFixture(t)
	f.prepared(extra)
	f.items.EXPECT().List(mock.Anything).Return([]itemsvc.Item{powerGem, diamonds}, nil)
	f.spend(diamonds)
	f.attacks.EXPECT().RollSpellToHit(mock.Anything, charID, 6).Return(&combatsvc.ToHitRoll{D20Roll: 14, TargetAC: 6, Hit: true}, nil)
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{SpellID: "chromatic_orb", TargetAC: &ac})
	require.NoError(t, err)
	assert.Equal(t, "chromatic_orb", res.SpellID)
	require.NotNil(t, res.Consumed)
	assert.Equal(t, "diamond", res.Consumed.ItemID, "the Power Gem is never spent")
	require.NotNil(t, res.Attack)
	assert.True(t, res.Attack.Hit)
	assert.Equal(t, spellsvc.SpellSlot{Level: 1, SpellID: "chromatic_orb", PreparedSpellName: "Chromatic Orb", Used: true, Extra: true}, f.saved[0][0])

	f = newFixture(t) // no item or attack call expected
	f.prepared(extra)
	_, err = f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{SpellID: "chromatic_orb"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Empty(t, f.entries)
}
