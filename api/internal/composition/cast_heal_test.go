package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0008 step 17.5 through the real services, recorder and storage, on the
// fixture's Cure Light Wounds (1d8): a cleric of 20 max HP, wounded to 15,
// casts it on himself. The heal lands on the cast entry's own HP delta, so
// one undo gives back both the slot and the HP. A second cast rolls past the
// gap and the ledger caps it at max HP.
func TestCast_Step17_5_HealThroughTheLedgerAndUndo(t *testing.T) {
	ctx := context.Background()
	dice := &queuedDice{t: t, values: []int{3, 8}, calls: map[int]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Brenna", Class: "cleric", Race: "dwarf", Str: 12, Dex: 10, Con: 14, Int: 10, Wis: 15, Cha: 9, HPMax: 20, XP: 3000,
	})
	require.NoError(t, err)
	_, err = bs.Prep.Prepare(ctx, id, []string{"cure_light_wounds"})
	require.NoError(t, err)
	_, err = bs.Log.AppendHPEvent(ctx, id, -5, "goblin")
	require.NoError(t, err)
	hp := func() int {
		st, err := bs.Log.GetHPState(ctx, id)
		require.NoError(t, err)
		return st.CurrentHp
	}
	undo := func() {
		t.Helper()
		top, ok, err := bs.Log.TopID(ctx, id)
		require.NoError(t, err)
		require.True(t, ok)
		_, err = bs.Log.Undo(ctx, id, top)
		require.NoError(t, err)
	}
	require.Equal(t, 15, hp())

	res, err := bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	require.NotNil(t, res.Heal)
	assert.Equal(t, 3, res.Heal.Total)
	assert.Equal(t, 18, hp(), "the d8 of 3")
	require.NotEmpty(t, res.Slots)
	assert.True(t, res.Slots[0].Used)

	undo()
	assert.Equal(t, 15, hp(), "one undo gives the HP back")
	again, err := bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	require.NoError(t, err, "and the slot: the same prepared spell casts again")
	assert.Equal(t, 20, hp(), "15 + 8 is capped at max HP 20")
	assert.Equal(t, 8, again.Heal.Total, "the roll is shown in full")
	undo()
	assert.Equal(t, 15, hp())
	assert.Equal(t, map[int]int{8: 2}, dice.calls, "one d8 per cast, no failure die at Wisdom 15")
}
