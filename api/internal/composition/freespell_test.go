package composition_test

// ADR 0030 step 20.8, the level-up free spell, end to end over the in-memory
// storage: the real services, the fixture's rule tables (PHB Table 21 for a
// mage's slots, Table 4 for the Intelligence), the action log's undo and redo.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// mageLevel20XP is PHB Table 20's level 20 for a mage.
const mageLevel20XP = 3_750_000

// freeSpellTable builds the services over the fixture with dice, and creates
// a character of class at the starting XP with Intelligence intel.
func freeSpellTable(t *testing.T, dice composition.Dice, class string, intel, xp int) (*composition.BackingServices, int64) {
	t.Helper()
	ctx := context.Background()
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	n := charactersvc.NewCharacter{
		Details: identity(), Name: "Elminster", Class: class, Race: "human",
		Str: 10, Dex: 12, Con: 11, Int: intel, Wis: 12, Cha: 10, HPMax: 3, XP: xp,
	}
	if class == "bard" {
		n.Dex, n.Cha, n.Int = 15, 15, max(intel, 13)
	}
	id, err := bs.Character.Create(ctx, s.User.ID, n)
	require.NoError(t, err)
	return bs, id
}

// pendingFreeSpells are the free spells of each pending level, by level
// (empty for none).
func pendingFreeSpells(t *testing.T, bs *composition.BackingServices, id int64) map[int][]spellbook.FreeSpell {
	t.Helper()
	lu, _, _ := levelUpOf(t, bs, id)
	out := map[int][]spellbook.FreeSpell{}
	for _, p := range lu.Pending {
		out[p.Level] = p.FreeSpells
	}
	return out
}

// owed is what the character's "to spend" reminder counts of the free spells.
func owed(t *testing.T, bs *composition.BackingServices, id int64) []spellbook.FreeSpell {
	t.Helper()
	lu, _, _ := levelUpOf(t, bs, id)
	return lu.FreeSpellsOwed
}

// table21Opens are the levels at which PHB Table 21 (p.30) first gives a
// mage a slot of a spell level, and that spell level.
var table21Opens = map[int]int{3: 2, 5: 3, 7: 4, 9: 5, 12: 6, 14: 7, 16: 8, 18: 9}

// ADR 0030 step 20.8: a level 1 mage (Intelligence 18, Table 4: 9th) whose XP
// reaches level 20 owes levels 2 to 20; each level where Table 21 opens a
// spell level grants one free spell of it, every other level none.
func TestLevelUp_Step20_8_AMageOpeningASpellLevelOwesOneFreeSpell(t *testing.T) {
	bs, id := freeSpellTable(t, &queuedDice{t: t, calls: map[int]int{}}, "mage", 18, 0)
	_, err := bs.XP.Add(context.Background(), id, xpsvc.EntryInput{Base: mageLevel20XP})
	require.NoError(t, err)

	got := pendingFreeSpells(t, bs, id)
	require.Len(t, got, 19, "levels 2 to 20 are pending")
	for level := 2; level <= 20; level++ {
		var want []spellbook.FreeSpell
		if spellLevel, ok := table21Opens[level]; ok {
			want = []spellbook.FreeSpell{{Level: level, SpellLevel: spellLevel}}
		}
		assert.Equal(t, want, got[level], "level %d", level)
	}
	assert.Empty(t, owed(t, bs, id), "nothing is owed before a roll")
}

// ADR 0030 step 20.8: "A bard gets none": the same levels, taken, grant none.
func TestLevelUp_Step20_8_ABardOwesNone(t *testing.T) {
	dice := &queuedDice{t: t, values: []int{3, 4}, calls: map[int]int{}}
	bs, id := freeSpellTable(t, dice, "bard", 18, 0)
	_, err := bs.XP.Add(context.Background(), id, xpsvc.EntryInput{Base: 2_500_000})
	require.NoError(t, err)
	for level, spells := range pendingFreeSpells(t, bs, id) {
		assert.Empty(t, spells, "level %d", level)
	}
	for _, level := range []int{2, 3} {
		_, err := bs.LevelUp.Roll(context.Background(), id, levelupsvc.RollInput{Level: level})
		require.NoError(t, err)
	}
	assert.Empty(t, owed(t, bs, id))
}

// ADR 0030 step 20.8: "No grant when the Intelligence's maximum spell level
// is below the new spell level": at Intelligence 9 (Table 4: 4th), levels 3,
// 5 and 7 grant theirs, 9 and above none.
func TestLevelUp_Step20_8_NoneAboveTheIntelligencesMaxLevel(t *testing.T) {
	bs, id := freeSpellTable(t, &queuedDice{t: t, calls: map[int]int{}}, "mage", 9, 0)
	_, err := bs.XP.Add(context.Background(), id, xpsvc.EntryInput{Base: mageLevel20XP})
	require.NoError(t, err)
	got := pendingFreeSpells(t, bs, id)
	for level := 2; level <= 20; level++ {
		var want []spellbook.FreeSpell
		if spellLevel, ok := table21Opens[level]; ok && spellLevel <= 4 {
			want = []spellbook.FreeSpell{{Level: level, SpellLevel: spellLevel}}
		}
		assert.Equal(t, want, got[level], "level %d", level)
	}
}

// ADR 0030 step 20.8: "a created character's levels ... owe none": a mage
// created at level 7 owes nothing for levels 1 to 7; level 9, taken in the
// app, owes its 5th-level spell.
func TestLevelUp_Step20_8_ACharacterCreatedAtLevel7OwesNothing(t *testing.T) {
	dice := &queuedDice{t: t, values: []int{2, 3}, calls: map[int]int{}}
	bs, id := freeSpellTable(t, dice, "mage", 15, 60_000)
	c, err := bs.Character.GetCharacter(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, 7, c.Derived.RealizedLevel)
	assert.Empty(t, c.Derived.LevelUp.FreeSpellsOwed)

	_, err = bs.XP.Add(context.Background(), id, xpsvc.EntryInput{Base: 135_000 - 60_000})
	require.NoError(t, err)
	for _, level := range []int{8, 9} {
		_, err := bs.LevelUp.Roll(context.Background(), id, levelupsvc.RollInput{Level: level})
		require.NoError(t, err)
	}
	assert.Equal(t, []spellbook.FreeSpell{{Level: 9, SpellLevel: 5}}, owed(t, bs, id))
}

// ADR 0030 step 20.8 with the roll, the claim and the journal: a level 1 mage
// (Intelligence 15) rolls levels 2, 3 and 4; level 3's free spell (2nd
// level) is owed from its roll on and never blocks the roll of 4. Claiming
// it with Mirror Image rolls no die and stores the origin level_grant with
// level 3; the reminder drops to nothing. Undoing the claim owes it again, a
// redo pays it again; undoing the claim and the rolls of 4 and 3 takes the
// grant back: level 3 is pending again with its free spell, nothing owed.
// Three d4 and no other die.
func TestLevelUp_Step20_8_UndoTakesTheGrantBack(t *testing.T) {
	ctx := context.Background()
	dice := &queuedDice{t: t, values: []int{1, 2, 3}, calls: map[int]int{}}
	bs, id := freeSpellTable(t, dice, "mage", 15, 0)
	_, err := bs.XP.Add(ctx, id, xpsvc.EntryInput{Base: 10_000})
	require.NoError(t, err)
	third := spellbook.FreeSpell{Level: 3, SpellLevel: 2}

	_, err = bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Level: 2})
	require.NoError(t, err)
	assert.Empty(t, owed(t, bs, id), "level 2 opens no spell level")
	_, err = bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Level: 3})
	require.NoError(t, err)
	assert.Equal(t, []spellbook.FreeSpell{third}, owed(t, bs, id))
	_, err = bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Level: 4})
	require.NoError(t, err, "an owed free spell never blocks the level-up")
	assert.Equal(t, []spellbook.FreeSpell{third}, owed(t, bs, id))

	_, err = bs.Learn.AddFreeSpell(ctx, id, "sleep", 3)
	require.ErrorIs(t, err, learnsvc.ErrFreeSpellWrongLevel, "level 3 owes a 2nd-level spell")
	got, err := bs.Learn.AddFreeSpell(ctx, id, "mirror_image", 3)
	require.NoError(t, err)
	assert.Equal(t, learnsvc.Added{SpellID: "mirror_image", SpellLevel: 2, Origin: spellbook.OriginLevelGrant, GrantLevel: 3}, *got)
	assert.Empty(t, owed(t, bs, id), "the claim pays it")
	assert.Equal(t, []spellbook.Entry{{Key: "mirror_image", Origin: spellbook.OriginLevelGrant, GrantLevel: 3}}, entriesOf(t, bs, id))
	_, err = bs.Learn.AddFreeSpell(ctx, id, "strength", 3)
	require.ErrorIs(t, err, learnsvc.ErrNoFreeSpellOwed, "one free spell per spell level opened")

	undoTop(t, bs, id)
	assert.Equal(t, []spellbook.FreeSpell{third}, owed(t, bs, id), "the claim's undo owes it again")
	assert.Empty(t, entriesOf(t, bs, id))
	redo, err := bs.Log.RedoTarget(ctx, id)
	require.NoError(t, err)
	_, err = bs.Log.Redo(ctx, id, redo.ID)
	require.NoError(t, err)
	assert.Empty(t, owed(t, bs, id))
	assert.Equal(t, []spellbook.Entry{{Key: "mirror_image", Origin: spellbook.OriginLevelGrant, GrantLevel: 3}}, entriesOf(t, bs, id))

	undoTop(t, bs, id) // the claim
	undoTop(t, bs, id) // the roll of 4
	assert.Equal(t, []spellbook.FreeSpell{third}, owed(t, bs, id))
	undoTop(t, bs, id) // the roll of 3
	assert.Empty(t, owed(t, bs, id), "the undone level-up takes its grant back")
	assert.Equal(t, []spellbook.FreeSpell{third}, pendingFreeSpells(t, bs, id)[3], "pending again, with its free spell")
	assert.Empty(t, entriesOf(t, bs, id))
	assert.Equal(t, map[int]int{4: 3}, dice.calls, "three d4, no other die (no d100 for the claim)")
}

// entriesOf is the character's spellbook entries with their origins.
func entriesOf(t *testing.T, bs *composition.BackingServices, id int64) []spellbook.Entry {
	t.Helper()
	info, err := bs.Prep.Info(context.Background(), id)
	require.NoError(t, err)
	out := []spellbook.Entry{}
	for _, e := range info.Spellbook {
		out = append(out, spellbook.Entry{Key: e.ID, Origin: e.Origin, GrantLevel: e.GrantLevel})
	}
	return out
}
