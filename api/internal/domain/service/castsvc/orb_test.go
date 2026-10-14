package castsvc_test

import (
	"context"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// critical-qki.42: Chromatic Orb spends a gem, rolls an attack with no crit
// and shows hit or miss and the gem used (no gem effect is modelled).
var (
	diamonds = itemsvc.Item{ID: "diamond", Name: "Diamond", Type: "gem", Behaviour: itemsvc.BehaviourStock, Count: 41}
	rubies   = itemsvc.Item{ID: "ruby", Name: "Ruby", Type: "gem", Behaviour: itemsvc.BehaviourStock, Count: 2}
	powerGem = itemsvc.Item{ID: "power_gem_dispel", Name: "Power Gem: Dispel Magic", Type: "gem", Behaviour: itemsvc.BehaviourDaily, Count: 1, MaxCount: 1}
)

func (f *fixture) orbSetup(items ...itemsvc.Item) {
	f.prepared(slot("chromatic_orb", "Chromatic Orb", 1))
	f.items.EXPECT().List(mock.Anything).Return(items, nil)
}

func (f *fixture) spend(it itemsvc.Item) {
	left := it
	left.Count--
	f.items.EXPECT().Use(mock.Anything, it.ID).Return(left, nil).Once()
}

func TestCast_ChromaticOrb_HitShowsTheGemEffect(t *testing.T) {
	f := newFixture(t) // no dice: the spell's level damage must not be rolled
	f.orbSetup(powerGem, diamonds)
	f.spend(diamonds)
	f.attacks.EXPECT().RollSpellToHit(mock.Anything, charID, 4).Return(&combatsvc.ToHitRoll{
		D20Roll: 20, BaseThac0: 12, EffectiveThac0: 12, TargetAC: 4, TargetACNeeded: 8, Hit: true, HitMargin: 12,
	}, nil)

	ac := 4
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{TargetAC: &ac})
	require.NoError(t, err)
	require.NotNil(t, res.Orb)
	assert.Nil(t, res.Damage, "the gem decides the effect, not the level damage table")
	assert.Equal(t, "Diamond", res.Orb.Gem)
	assert.Equal(t, 40, res.Orb.Remaining)
	assert.True(t, res.Orb.Attack.Hit)
	require.Len(t, f.entries, 1)
	assert.Contains(t, keys(f.entries[0].Snapshots), "items:diamond", "voiding gives the gem back")
	assert.Contains(t, string(f.entries[0].Payload), `"orb"`)
}

func TestCast_ChromaticOrb_MissSpendsTheGemNoEffect(t *testing.T) {
	f := newFixture(t)
	f.orbSetup(diamonds)
	f.spend(diamonds)
	f.attacks.EXPECT().RollSpellToHit(mock.Anything, charID, 4).Return(&combatsvc.ToHitRoll{D20Roll: 3, TargetAC: 4, TargetACNeeded: 8}, nil)

	ac := 4
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{TargetAC: &ac})
	require.NoError(t, err)
	assert.False(t, res.Orb.Attack.Hit)
	assert.Equal(t, 40, res.Orb.Remaining)
}

func TestCast_ChromaticOrb_Refusals(t *testing.T) {
	ac := 4
	tests := []struct {
		name  string
		items []itemsvc.Item
		req   castsvc.CastRequest
	}{
		{"no gem left", []itemsvc.Item{powerGem, {ID: "diamond", Name: "Diamond", Type: "gem", Behaviour: itemsvc.BehaviourStock}}, castsvc.CastRequest{TargetAC: &ac}},
		{"several gems, none chosen", []itemsvc.Item{diamonds, rubies}, castsvc.CastRequest{TargetAC: &ac}},
		{"a gem that is not in the inventory", []itemsvc.Item{diamonds}, castsvc.CastRequest{TargetAC: &ac, GemItemID: "emerald"}},
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

func TestCast_ChromaticOrb_PicksTheChosenGem(t *testing.T) {
	f := newFixture(t)
	f.orbSetup(diamonds, rubies)
	f.spend(rubies)
	f.attacks.EXPECT().RollSpellToHit(mock.Anything, charID, 4).Return(&combatsvc.ToHitRoll{D20Roll: 19, Hit: true}, nil)

	ac := 4
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{TargetAC: &ac, GemItemID: "ruby"})
	require.NoError(t, err)
	assert.Equal(t, "Ruby", res.Orb.Gem)
	assert.Equal(t, 1, res.Orb.Remaining)
}
