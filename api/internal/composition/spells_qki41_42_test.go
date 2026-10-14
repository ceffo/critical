package composition_test

import (
	"context"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// putSlots writes the slot list (the test spellbook has no Rary's to prepare).
func putSlots(t *testing.T, bs *composition.BackingServices, json string) {
	t.Helper()
	_, err := bs.DB.ExecContext(context.Background(),
		`INSERT INTO spell_slots (character_id, slots) VALUES (?, ?)
		 ON CONFLICT(character_id) DO UPDATE SET slots = excluded.slots`, bs.CharacterID, json)
	require.NoError(t, err)
}

func slotList(t *testing.T, bs *composition.BackingServices) []spellsvc.SpellSlot {
	t.Helper()
	s, err := bs.Spells.GetSlots(context.Background(), bs.CharacterID)
	require.NoError(t, err)
	return s
}

// qki.41: voiding a Rary's cast drops the extra slots and frees the base slot.
func TestRary_VoidRemovesExtrasAndRestoresTheBaseSlot(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	putSlots(t, bs, `[{"level":4,"prepared_spell_id":"rarys_mnemonic_enhancer","prepared_spell_name":"Rary's Mnemonic Enhancer","used":false}]`)

	res, err := bs.Cast.Cast(ctx, bs.CharacterID, 0, castsvc.CastRequest{ExtraSlots: []int{1, 2}})
	require.NoError(t, err)
	got := slotList(t, bs)
	require.Len(t, got, 3)
	assert.True(t, got[0].Used)
	assert.Equal(t, spellsvc.SpellSlot{Level: 1, Extra: true}, got[1])
	assert.Equal(t, spellsvc.SpellSlot{Level: 2, Extra: true}, got[2])

	_, err = bs.Log.Void(ctx, bs.CharacterID, res.EntryID)
	require.NoError(t, err)
	got = slotList(t, bs)
	require.Len(t, got, 1)
	assert.False(t, got[0].Used)
	assert.False(t, got[0].Extra)
}

// qki.41: a long rest removes the extra slots, a new preparation keeps them
// at the same indices.
func TestRary_PrepareKeepsExtrasLongRestRemovesThem(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	putSlots(t, bs, `[{"level":1,"prepared_spell_id":"alarm","prepared_spell_name":"Alarm","used":false},{"level":2,"extra":true,"used":false}]`)

	_, err := bs.Prep.Prepare(ctx, bs.CharacterID, []string{"alarm"})
	require.NoError(t, err)
	got := slotList(t, bs)
	require.Len(t, got, 2)
	assert.True(t, got[1].Extra, "the extra slot keeps its index")

	after, err := bs.Spells.ResetSlots(ctx, bs.CharacterID)
	require.NoError(t, err)
	assert.Len(t, after, 1)
	assert.Len(t, slotList(t, bs), 1)
}

// qki.42: voiding an orb cast gives the gem back.
func TestOrb_VoidRestoresTheGem(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	item, err := bs.Items.Create(ctx, bs.CharacterID, itemsvc.ItemInput{Name: "Diamond", Type: "gem", Behaviour: itemsvc.BehaviourStock, Count: 3})
	require.NoError(t, err)
	_, err = bs.Prep.Prepare(ctx, bs.CharacterID, []string{"chromatic_orb"})
	require.NoError(t, err)
	count := func() int {
		items, err := bs.Items.List(ctx)
		require.NoError(t, err)
		for _, i := range items {
			if i.ID == item.Item.ID {
				return i.Count
			}
		}
		return -1
	}

	ac := 5
	res, err := bs.Cast.Cast(ctx, bs.CharacterID, 0, castsvc.CastRequest{TargetAC: &ac})
	require.NoError(t, err)
	require.NotNil(t, res.Orb)
	assert.True(t, res.Orb.Attack.Hit, "d20 20 vs AC 5")
	assert.Equal(t, 2, count())

	_, err = bs.Log.Void(ctx, bs.CharacterID, res.EntryID)
	require.NoError(t, err)
	assert.Equal(t, 3, count())
	assert.Equal(t, 0, usedSlots(t, bs))
}
