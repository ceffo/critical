package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// derivedOf is what the character's readers see: its derived block, the XP
// ledger's level and the preparation limits.
func derivedOf(t *testing.T, bs *composition.BackingServices, id int64) (charactersvc.Derived, int, []prepsvc.LevelLimit) {
	t.Helper()
	ctx := context.Background()
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	l, err := bs.XP.Get(ctx, id)
	require.NoError(t, err)
	info, err := bs.Prep.Info(ctx, id)
	require.NoError(t, err)
	return c.Derived, l.Level, info.Limits
}

// ADR 0030 steps 3, 5, 6 and 8 over the fixture's tables (the books'
// numbers): the fixture bard (the seed's sheet, 1,293,418 XP) is level 15,
// THAC0 13, saves 10/8/9/13/9, slots 3/3/3/3/2, as his sheet says. An XP award
// that reaches 1,540,000 (Table 25's level 17) changes the level, the THAC0
// (Table 53: 12), the saves (Table 60, rogues 17-20) and the slots (Table 32)
// at once, for every reader; undoing it brings them all back. The sheet
// comparison (step 24) stays empty: it reads the sheet's own XP.
func TestDerived_AnXPAwardCrossingALevelChangesEverythingAtOnce(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id := rootCharacter(t, bs)

	next := 1_320_000
	at15 := charactersvc.Derived{
		XPTotal: 1_293_418, Level: 15, NextLevelXP: &next, THAC0: 13,
		Saves:      charactersvc.Saves{ParaPoison: 10, RodStaffWand: 8, PetriPoly: 9, Breath: 13, Spell: 9},
		SpellSlots: map[int]int{1: 3, 2: 3, 3: 3, 4: 3, 5: 2}, SheetDifferences: []charactersvc.SheetDifference{},
		LevelUp: charactersvc.LevelUp{HPLevel: 15, Pending: []charactersvc.PendingLevel{}},
	}
	limits15 := []prepsvc.LevelLimit{{Level: 1, Max: 3}, {Level: 2, Max: 3}, {Level: 3, Max: 3}, {Level: 4, Max: 3}, {Level: 5, Max: 2}}
	d, ledgerLevel, limits := derivedOf(t, bs, id)
	assert.Equal(t, at15, d)
	assert.Equal(t, 15, ledgerLevel)
	assert.Equal(t, limits15, limits)

	_, err = bs.XP.Add(ctx, id, xpsvc.EntryInput{Base: 1_540_000 - 1_293_418, Reason: "the dragon"})
	require.NoError(t, err)
	next17 := 1_760_000
	d, ledgerLevel, limits = derivedOf(t, bs, id)
	assert.Equal(t, charactersvc.Derived{
		XPTotal: 1_540_000, Level: 17, NextLevelXP: &next17, THAC0: 12,
		Saves:      charactersvc.Saves{ParaPoison: 9, RodStaffWand: 6, PetriPoly: 8, Breath: 12, Spell: 7},
		SpellSlots: map[int]int{1: 4, 2: 4, 3: 3, 4: 3, 5: 3, 6: 1}, SheetDifferences: []charactersvc.SheetDifference{},
		// The hit points wait for the rolls (ADR 0030 step 7): above the
		// rogues' name level, +2 each.
		LevelUp: charactersvc.LevelUp{HPLevel: 15, Pending: []charactersvc.PendingLevel{{Level: 16, FixedGain: 2}, {Level: 17, FixedGain: 2}}},
	}, d)
	assert.Equal(t, 17, ledgerLevel)
	assert.Equal(t, []prepsvc.LevelLimit{
		{Level: 1, Max: 4}, {Level: 2, Max: 4}, {Level: 3, Max: 3}, {Level: 4, Max: 3}, {Level: 5, Max: 3}, {Level: 6, Max: 1},
	}, limits)

	top, ok, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	_, err = bs.Log.Undo(ctx, id, top)
	require.NoError(t, err)
	d, ledgerLevel, limits = derivedOf(t, bs, id)
	assert.Equal(t, at15, d)
	assert.Equal(t, 15, ledgerLevel)
	assert.Equal(t, limits15, limits)
}
