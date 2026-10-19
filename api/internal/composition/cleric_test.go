package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0030 steps 13 to 16 on clerics created in the app, through the wiring,
// the fixture catalogue (format 6) and the in-memory store.
//
// A 1st-level cleric of Wisdom 9 (Table 24: one 1st; Table 5: no bonus,
// 20% failure) prepares from the priest spells his spheres open at 1st
// level only: Thought and Animal are closed, Elemental (Water) is a minor
// sphere open at 1st, Produce Flame (2nd) has no slot. His cast rolls a
// d100: 20 fails, the slot spent. He turns a skeleton (Table 61 at 1st: 10)
// with a d20 of 10 and 2d6 affected, and is refused a ghast (a dash) with no
// die. One d100, one d20, two d6, exactly.
func TestCleric_Steps13to16_PrepareCastAndTurn(t *testing.T) {
	ctx := context.Background()
	dice := &queuedDice{t: t, values: []int{20, 10, 3, 5}, calls: map[int]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Ansel", Class: "cleric", Race: "human", Str: 12, Dex: 10, Con: 13, Int: 10, Wis: 9, Cha: 11, HPMax: 7,
	})
	require.NoError(t, err)

	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, map[int]int{1: 1}, c.Derived.SpellSlots, "Table 24 at 1st, no bonus at Wisdom 9")
	assert.Equal(t, 20, c.Derived.SpellFailure, "Table 5 at Wisdom 9")

	assert.Equal(t, []string{"bless", "command", "create_water", "cure_light_wounds", "detect_magic_priest"}, spellbookOf(t, bs, id))
	info, err := bs.Prep.Info(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []prepsvc.LevelLimit{{Level: 1, Max: 1}}, info.Limits)
	_, err = bs.Prep.Prepare(ctx, id, []string{"thought_capture"})
	require.ErrorIs(t, err, prepsvc.ErrNotInSpellbook, "Thought is closed")
	_, err = bs.Prep.Prepare(ctx, id, []string{"bless", "command"})
	require.ErrorIs(t, err, prepsvc.ErrOverLimit, "one 1st-level slot")
	_, err = bs.Prep.Prepare(ctx, id, []string{"bless"})
	require.NoError(t, err)

	res, err := bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.Equal(t, &castsvc.SpellFailure{Roll: 20, Chance: 20, Failed: true}, res.Failure)
	require.Len(t, res.Slots, 1)
	assert.True(t, res.Slots[0].Used, "a failed spell spends its slot")

	turn, err := bs.Turn.Turn(ctx, id, "skeleton")
	require.NoError(t, err)
	assert.Equal(t, ruletablesvc.TurningCell{Roll: 10}, turn.Cell)
	assert.True(t, turn.Success)
	assert.Equal(t, []int{3, 5}, turn.Affected)
	_, err = bs.Turn.Turn(ctx, id, "ghast")
	require.ErrorIs(t, err, ruletablesvc.ErrCannotTurn)
	assert.Equal(t, map[int]int{100: 1, 20: 1, 6: 2}, dice.calls)
	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindTurnUndead, entries[0].Kind)
}

// ADR 0030 steps 13 and 15.2: a 3rd-level cleric (3,000 XP, PHB Table 23) of
// Wisdom 15 has two 1st and one 2nd on top of Table 24's two 1st and one 2nd,
// rolls no failure die, and prepares 2nd-level spells too: Produce Flame
// (Elemental Fire, minor, 2nd) is open, Flame Walk (Elemental Fire, 4th)
// is not, nor Flame Strike (5th, no slot).
func TestCleric_Steps13_15_AThirdLevelClericOfWisdom15(t *testing.T) {
	ctx := context.Background()
	dice := &queuedDice{t: t, calls: map[int]int{}}
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
	info, err := bs.Prep.Info(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []prepsvc.LevelLimit{{Level: 1, Max: 4}, {Level: 2, Max: 2}}, info.Limits)
	got := spellbookOf(t, bs, id)
	assert.Contains(t, got, "produce_flame")
	assert.Contains(t, got, "chant")
	assert.NotContains(t, got, "flame_walk")
	assert.NotContains(t, got, "flame_strike")
	_, err = bs.Prep.Prepare(ctx, id, []string{"bless"})
	require.NoError(t, err)
	res, err := bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.Nil(t, res.Failure, "Wisdom 15: no die")
	assert.Empty(t, dice.calls)
}

// ADR 0030 step 14 with the log: undoing a failed cast gives the slot back
// and voids the entry; the next cast rolls a fresh d100.
func TestCleric_Step14_UndoAFailedCast(t *testing.T) {
	ctx := context.Background()
	dice := &queuedDice{t: t, values: []int{5, 90}, calls: map[int]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Ansel", Class: "cleric", Race: "human", Str: 12, Dex: 10, Con: 13, Int: 10, Wis: 9, Cha: 11, HPMax: 7,
	})
	require.NoError(t, err)
	_, err = bs.Prep.Prepare(ctx, id, []string{"bless"})
	require.NoError(t, err)

	failed, err := bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	require.NotNil(t, failed.Failure)
	assert.True(t, failed.Failure.Failed, "d100 5 at or under 20")
	_, err = bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	require.ErrorIs(t, err, spellsvc.ErrSlotAlreadyUsed, "the failed spell spent its slot")

	_, err = bs.Log.Undo(ctx, id, failed.EntryID)
	require.NoError(t, err)
	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	for _, e := range entries {
		if e.ID == failed.EntryID {
			assert.True(t, e.Voided(), "the failed cast is undone")
		}
	}

	again, err := bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	require.NoError(t, err, "the slot is unspent again")
	assert.Equal(t, &castsvc.SpellFailure{Roll: 90, Chance: 20}, again.Failure, "a fresh d100")
	assert.Equal(t, 2, dice.calls[100])
}
