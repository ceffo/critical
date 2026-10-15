package castsvc_test

import (
	"context"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const raryID = "rarys_mnemonic_enhancer"

// ADR 0008 step 9: a spell with grant_slots (Rary's Mnemonic Enhancer, 3
// levels in the seed) adds extra slots split by the caster: 3 x L1, or L1 +
// L2, or one L3.
func TestCast_GrantSlots_AddsTheChosenSlots(t *testing.T) {
	tests := []struct {
		name   string
		split  []int
		extras []int
	}{
		{"default is three level 1 slots", nil, []int{1, 1, 1}},
		{"L1 + L2", []int{2, 1}, []int{1, 2}},
		{"one L3", []int{3}, []int{3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.prepared(slot(raryID, "Rary's Mnemonic Enhancer", 4))
			res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{ExtraSlots: tt.split})
			require.NoError(t, err)
			assert.Equal(t, tt.extras, res.ExtraSlots)
			require.Len(t, f.saved, 1)
			got := f.saved[0]
			require.Len(t, got, 1+len(tt.extras))
			assert.True(t, got[0].Used, "the granting spell's own slot is spent")
			assert.Empty(t, res.TextOnly, "the cast did something")
			for i, l := range tt.extras {
				assert.Equal(t, spellsvc.SpellSlot{Level: l, Extra: true}, got[1+i])
			}
			assert.Equal(t, got, res.Slots)
		})
	}
}

func TestCast_GrantSlots_RejectsAWrongSplit(t *testing.T) {
	for _, split := range [][]int{{1, 1}, {2, 2}, {4}, {0, 3}, {1, 1, 1, 1}} {
		f := newFixture(t)
		f.prepared(slot(raryID, "Rary's Mnemonic Enhancer", 4))
		_, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{ExtraSlots: split})
		assert.ErrorIs(t, err, domain.ErrInvalidInput, "%v", split)
		assert.Empty(t, f.saved)
	}
}

// ADR 0008 step 9: a blank extra slot is cast with a spell of its level
// picked at cast time.
func TestCast_ExtraSlot_PicksTheSpell(t *testing.T) {
	extra := spellsvc.SpellSlot{Level: 1, Extra: true}

	f := newFixture(t)
	f.prepared(extra)
	res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{SpellID: "alarm"})
	require.NoError(t, err)
	assert.Equal(t, "alarm", res.SpellID)
	require.Len(t, f.saved, 1)
	assert.Equal(t, spellsvc.SpellSlot{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm", Used: true, Extra: true}, f.saved[0][0])

	f = newFixture(t)
	f.prepared(extra)
	_, err = f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
	assert.ErrorIs(t, err, domain.ErrInvalidInput, "no spell chosen")

	f = newFixture(t)
	f.prepared(extra)
	_, err = f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{SpellID: "fireball"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput, "a level 3 spell in a level 1 slot")
	assert.Empty(t, f.saved)
}
