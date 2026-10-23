package castsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// clericCaster is a caster with a chance of spell failure (Wisdom 9, PHB
// Table 5: 20%).
func clericCaster() charactersvc.View {
	v := viewtest.Single("cleric", bardLevel, viewtest.WithIdentity(charID, "Test", ""), viewtest.WithScores(0, 0, 0, 0, 9, 0))
	v.Entries[0].Group = "priest"
	v.SpellFailure = 20
	return v
}

// ADR 0030 step 14: a caster with a chance of spell failure rolls one d100
// at each cast. At or under the chance the spell fails: its slot is spent,
// no effect is rolled or applied, and the cast entry records the d100, the
// chance and the failure. Above it the spell resolves as usual, the roll on
// the result. A caster without a chance rolls no d100.
func TestCast_Step14_SpellFailure(t *testing.T) {
	t.Run("at the chance: fails, slot spent, nothing rolled", func(t *testing.T) {
		f := newFixtureOf(t, catalog(t), clericCaster(), 20)
		f.prepared(slot("alarm", "Alarm", 1), slot("fireball", "Fireball", 3))
		res, err := f.svc.Cast(context.Background(), charID, 1, castsvc.CastRequest{})
		require.NoError(t, err)
		assert.Equal(t, &castsvc.SpellFailure{Roll: 20, Chance: 20, Failed: true}, res.Failure)
		assert.Nil(t, res.Damage, "no effect")
		assert.Equal(t, []int{100}, f.dice.sides, "one d100, no damage die")
		require.Len(t, f.saved, 1)
		assert.Equal(t, []bool{false, true}, usedFlags(f.saved[0]), "the slot is spent")
		require.Len(t, f.entries, 1)
		assert.Equal(t, []string{"spell_slots"}, keys(f.entries[0].Snapshots), "only the slot is restored by an undo")
		var payload struct {
			SpellID string `json:"spellId"`
			Failure struct {
				D100, Chance int
				Failed       bool
			} `json:"failure"`
		}
		require.NoError(t, json.Unmarshal(f.entries[0].Payload, &payload))
		assert.Equal(t, "fireball", payload.SpellID)
		assert.Equal(t, 20, payload.Failure.D100)
		assert.Equal(t, 20, payload.Failure.Chance)
		assert.True(t, payload.Failure.Failed)
	})
	t.Run("above the chance: the spell resolves", func(t *testing.T) {
		f := newFixtureOf(t, catalog(t), clericCaster(), 21, 1, 2, 3, 4, 5, 6, 1, 2, 3, 4)
		f.prepared(slot("fireball", "Fireball", 3))
		res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
		require.NoError(t, err)
		assert.Equal(t, &castsvc.SpellFailure{Roll: 21, Chance: 20}, res.Failure)
		require.NotNil(t, res.Damage)
		assert.Equal(t, 31, res.Damage.Total)
		assert.Equal(t, 100, f.dice.sides[0], "the d100 first")
		assert.Empty(t, f.dice.vals, "every die rolled once")
	})
	t.Run("no chance: no d100", func(t *testing.T) {
		f := newFixture(t, 1, 2, 3, 4, 5, 6, 1, 2, 3, 4)
		f.prepared(slot("fireball", "Fireball", 3))
		res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
		require.NoError(t, err)
		assert.Nil(t, res.Failure)
		assert.NotContains(t, f.dice.sides, 100)
	})
}

// ADR 0030 step 14: the request's refusals come before the spell failure
// d100. An invalid request from a caster who would fail (the d100 at the
// chance) is a 400 with nothing written and no d100 rolled, whatever the die
// would show: an attack spell with no target AC, a material item with none
// left or none chosen, a wrong split of granted slots.
func TestCast_Step14_RefusalsBeforeTheD100(t *testing.T) {
	ac := 4
	tests := []struct {
		name  string
		slot  spellsvc.SpellSlot
		items []itemsvc.Item
		req   castsvc.CastRequest
	}{
		{"an attack spell with no target AC", slot("chromatic_orb", "Chromatic Orb", 1), []itemsvc.Item{diamonds}, castsvc.CastRequest{}},
		{"a material item with none left", slot("chromatic_orb", "Chromatic Orb", 1), nil, castsvc.CastRequest{TargetAC: &ac}},
		{"several items, none chosen", slot("chromatic_orb", "Chromatic Orb", 1), []itemsvc.Item{diamonds, rubies}, castsvc.CastRequest{TargetAC: &ac}},
		{"a wrong split of granted slots", slot(raryID, "Rary's Mnemonic Enhancer", 4), nil, castsvc.CastRequest{ExtraSlots: []int{4}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixtureOf(t, catalog(t), clericCaster(), 1)
			f.prepared(tt.slot)
			f.items.EXPECT().List(mock.Anything, charID).Return(tt.items, nil).Maybe()
			_, err := f.svc.Cast(context.Background(), charID, 0, tt.req)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.Empty(t, f.dice.sides, "no d100")
			assert.Empty(t, f.saved, "no slot spent")
			assert.Empty(t, f.entries, "nothing logged")
		})
	}
}
