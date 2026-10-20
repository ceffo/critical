package composition_test

// The level-up roll end to end over the in-memory storage (ADR 0030 step 7):
// the real services, the fixture's rule tables (the books' numbers), the
// action log's undo and redo.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// queuedDice answers RollDie from a queue of distinct values and counts the
// calls by sides; the other dice are never expected here.
type queuedDice struct {
	t      *testing.T
	values []int
	calls  map[int]int
}

func (d *queuedDice) RollDie(sides int) int {
	d.t.Helper()
	require.NotEmpty(d.t, d.values, "an unexpected d%d", sides)
	d.calls[sides]++
	v := d.values[0]
	d.values = d.values[1:]
	return v
}

func (d *queuedDice) RollD20() int  { d.t.Fatal("unexpected d20"); return 0 }
func (d *queuedDice) RollD100() int { d.t.Fatal("unexpected d100"); return 0 }

func levelUpOf(t *testing.T, bs *composition.BackingServices, id int64) (charactersvc.LevelUp, int, int) {
	t.Helper()
	ctx := context.Background()
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	hp, err := bs.Log.GetHPState(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, c.HPMax, hp.MaxHp)
	return c.Derived.LevelUp, hp.MaxHp, hp.CurrentHp
}

func undoTop(t *testing.T, bs *composition.BackingServices, id int64) {
	t.Helper()
	ctx := context.Background()
	top, ok, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	_, err = bs.Log.Undo(ctx, id, top)
	require.NoError(t, err)
}

// ADR 0030 steps 7.2 and 7.3 on the fixture bard (the seed's sheet: level
// 15, 85 max HP set for 15): an award that reaches 1,540,000 XP (Table 25's
// level 17) owes levels 16 and 17, each +2 above the rogues' name level (no
// die). Rolling 16 raises the max HP to 87 and keeps the damage taken (75 of
// 85 becomes 77 of 87); undo gives the max HP and the pending level back,
// redo the roll again. ADR 0037 step 15: the roll answers what 16 grants (a
// weapon slot, a non-weapon slot, a bard's 15 points), what is left to spend
// grows by it, and the undo takes it back.
func TestLevelUp_TheRollRaisesTheMaxHPAndUndoMakesItPendingAgain(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id := rootCharacter(t, bs)

	_, err = bs.Log.AppendHPEvent(ctx, id, -10, "an ogre")
	require.NoError(t, err)
	_, err = bs.XP.Add(ctx, id, xpsvc.EntryInput{Base: 1_540_000 - 1_293_418, Reason: "the dragon"})
	require.NoError(t, err)
	grants16 := charactersvc.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1, SkillPoints: 15}
	owed := []charactersvc.PendingLevel{
		{Level: 16, FixedGain: 2, Grants: grants16},
		{Level: 17, FixedGain: 2, Grants: charactersvc.LevelGrants{SkillPoints: 15}},
	}
	at15 := charactersvc.LevelGrants{NonweaponSlots: 2, SkillPoints: 30}
	at16 := charactersvc.LevelGrants{WeaponSlots: 1, NonweaponSlots: 3, SkillPoints: 45}
	lu, maxHP, current := levelUpOf(t, bs, id)
	assert.Equal(t, charactersvc.LevelUp{Pending: owed, ToSpend: at15}, lu)
	assert.Equal(t, 85, maxHP)
	assert.Equal(t, 75, current)

	got, err := bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Level: 16})
	require.NoError(t, err)
	assert.Equal(t, 2, got.Gain)
	assert.True(t, got.Fixed)
	assert.Equal(t, grants16, got.Grants)
	lu, maxHP, current = levelUpOf(t, bs, id)
	assert.Equal(t, charactersvc.LevelUp{Pending: owed[1:], ToSpend: at16}, lu)
	assert.Equal(t, 87, maxHP)
	assert.Equal(t, 77, current, "the damage taken is kept")
	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindLevelUp, entries[0].Kind)

	undoTop(t, bs, id)
	lu, maxHP, current = levelUpOf(t, bs, id)
	assert.Equal(t, charactersvc.LevelUp{Pending: owed, ToSpend: at15}, lu, "the level-up is pending again, its grants gone")
	assert.Equal(t, 85, maxHP)
	assert.Equal(t, 75, current)

	redo, err := bs.Log.RedoTarget(ctx, id)
	require.NoError(t, err)
	_, err = bs.Log.Redo(ctx, id, redo.ID)
	require.NoError(t, err)
	lu, maxHP, _ = levelUpOf(t, bs, id)
	assert.Equal(t, charactersvc.LevelUp{Pending: owed[1:], ToSpend: at16}, lu)
	assert.Equal(t, 87, maxHP)

	// Undoing the roll, then the award: the level and the max HP are back
	// where they were, nothing owed (step 7.4 never fires).
	undoTop(t, bs, id)
	undoTop(t, bs, id)
	lu, maxHP, _ = levelUpOf(t, bs, id)
	assert.Equal(t, charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{}, ToSpend: at15}, lu)
	assert.Equal(t, 85, maxHP)
}

// ADR 0030 steps 7.1 and 7.3 with the dice: a level 1 bard created in the
// app (Constitution 12: Table 3's 0) whose award reaches 2,500 XP (Table 25's
// level 3) owes two d6, rolled one per request, lowest first, through the
// dice port: two dice, distinct values, exactly two calls. Its max HP was set
// for level 1 at creation (step 7.5).
func TestLevelUp_TwoLevelsAtOnceAreTwoDiceThroughTheDicePort(t *testing.T) {
	ctx := context.Background()
	dice := &queuedDice{t: t, values: []int{4, 2}, calls: map[int]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Lyra", Class: "bard", Race: "half_elf", Str: 10, Dex: 15, Con: 12, Int: 14, Wis: 9, Cha: 16, HPMax: 6,
	})
	require.NoError(t, err)
	lu, _, _ := levelUpOf(t, bs, id)
	assert.Empty(t, lu.Pending)

	_, err = bs.XP.Add(ctx, id, xpsvc.EntryInput{Base: 2_500})
	require.NoError(t, err)
	lu, _, _ = levelUpOf(t, bs, id)
	points := charactersvc.LevelGrants{SkillPoints: 15}
	assert.Equal(t, []charactersvc.PendingLevel{{Level: 2, Die: 6, Grants: points}, {Level: 3, Die: 6, Grants: points}}, lu.Pending)

	first, err := bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Level: 2})
	require.NoError(t, err)
	second, err := bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Level: 3})
	require.NoError(t, err)
	assert.Equal(t, []int{4, 2}, []int{*first.Rolled, *second.Rolled})
	assert.Equal(t, map[int]int{6: 2}, dice.calls, "two d6, no other die")
	assert.Equal(t, 0, first.ConAdjustment)
	_, maxHP, _ := levelUpOf(t, bs, id)
	assert.Equal(t, 6+4+2, maxHP)

	_, err = bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Level: 4})
	require.ErrorIs(t, err, levelupsvc.ErrNotPending)
}
