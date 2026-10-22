package composition_test

// ADR 0043's PSP ledger end to end over the in-memory storage: the real
// services, the fixture's Table 6 and the action log's undo and redo.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// thiefPsionicist stores the party's human thief 15/psionicist 14 with 181
// max PSPs (the sheet's: 25 + 13 x 12) and returns its id.
func thiefPsionicist(t *testing.T) (*composition.BackingServices, int64) {
	t.Helper()
	return thiefPsionicistKnowing(t, nil)
}

// thiefPsionicistKnowing is thiefPsionicist with the given known powers.
func thiefPsionicistKnowing(t *testing.T, picks []psionics.Pick) (*composition.BackingServices, int64) {
	t.Helper()
	return thiefPsionicistRolling(t, picks, nil)
}

// thiefPsionicistRolling is thiefPsionicistKnowing with known dice (nil: the
// crypto dice).
func thiefPsionicistRolling(t *testing.T, picks []psionics.Pick, dice composition.Dice) (*composition.BackingServices, int64) {
	t.Helper()
	ctx := context.Background()
	m := memory.New()
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id, err := m.AddCharacter(ctx, charactersvc.Character{
		Name: "Thief/psionicist", Race: "human", Class: "thief", RealizedLevel: 15,
		Str: 12, Dex: 17, Con: 16, Int: 15, Wis: 17, Cha: 10, HPMax: 50, PSPMax: 181, PsionicPicks: picks,
		Classes: []classes.Entry{classes.SingleEntry("thief", 15, 0), classes.SingleEntry("psionicist", 14, 0)},
	})
	require.NoError(t, err)
	return bs, id
}

// currentPSP is the derived character's current PSPs.
func currentPSP(t *testing.T, bs *composition.BackingServices, id int64) int {
	t.Helper()
	c, err := bs.Character.GetCharacter(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, c.Derived.PSP)
	return c.Derived.PSP.Current
}

// spendPSP logs an entry taking n PSPs off the pool (power use is a later
// step; any entry may carry a PSP delta, step 5).
func spendPSP(t *testing.T, bs *composition.BackingServices, id int64, n int) {
	t.Helper()
	_, err := bs.Log.Record(context.Background(), actionlog.NewEntry{
		CharacterID: id, Kind: actionlog.Kind("psp_spent"), Summary: "spent", PSPDelta: -n,
	})
	require.NoError(t, err)
}

// redoTop replays the newest popped transaction.
func redoTop(t *testing.T, bs *composition.BackingServices, id int64) {
	t.Helper()
	ctx := context.Background()
	target, err := bs.Log.RedoTarget(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, target)
	_, err = bs.Log.Redo(ctx, id, target.ID)
	require.NoError(t, err)
}

// Step 7 with step 5: the long rest recovers a night's sleep (96, Table 6's
// sleeping rate for 8 hours) capped at the PSPs spent; undoing it takes back
// exactly what was applied, not what was asked, and redoing it applies the
// same again (ADR 0019: nothing recomputed).
func TestLongRest_Step7_UndoRestoresPSP(t *testing.T) {
	ctx := context.Background()
	t.Run("a night within what was spent", func(t *testing.T) {
		bs, id := thiefPsionicist(t)
		spendPSP(t, bs, id, 150)
		require.Equal(t, 31, currentPSP(t, bs, id))
		_, err := bs.Rest.LongRest(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 127, currentPSP(t, bs, id), "31 + 96")
		undoTop(t, bs, id)
		assert.Equal(t, 31, currentPSP(t, bs, id))
		redoTop(t, bs, id)
		assert.Equal(t, 127, currentPSP(t, bs, id))
	})
	t.Run("a night capped at the maximum", func(t *testing.T) {
		bs, id := thiefPsionicist(t)
		spendPSP(t, bs, id, 40)
		_, err := bs.Rest.LongRest(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 181, currentPSP(t, bs, id), "96 asked, 40 applied")
		entries, err := bs.Log.ActiveEntries(ctx, id, actionlog.KindLongRest, 1)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, 40, entries[0].PSPDelta)
		assert.JSONEq(t, `{"slotsReset":0,"itemsRefilled":[],"pspRequested":96,"pspApplied":40}`, string(entries[0].Payload))
		undoTop(t, bs, id)
		assert.Equal(t, 141, currentPSP(t, bs, id), "the 40 applied are taken back, not the 96 asked")
		redoTop(t, bs, id)
		assert.Equal(t, 181, currentPSP(t, bs, id))
	})
	t.Run("a full pool recovers nothing", func(t *testing.T) {
		bs, id := thiefPsionicist(t)
		_, err := bs.Rest.LongRest(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 181, currentPSP(t, bs, id))
		undoTop(t, bs, id)
		assert.Equal(t, 181, currentPSP(t, bs, id))
	})
}

// Step 5's hand edit: a psp_change goes through the recording path, so a loss
// stops at 0 and a gain at the maximum; undoing it takes back exactly what was
// applied and redoing it applies the same again.
func TestPSPChange_Step5_UndoRedoExact(t *testing.T) {
	ctx := context.Background()
	t.Run("a loss, then a gain capped at the maximum", func(t *testing.T) {
		bs, id := thiefPsionicist(t)
		state, err := bs.Log.AppendPSPChange(ctx, id, -30, "body equilibrium")
		require.NoError(t, err)
		assert.Equal(t, actionlog.PSPState{Current: 151, Max: 181}, state)
		state, err = bs.Log.AppendPSPChange(ctx, id, 50, "")
		require.NoError(t, err)
		assert.Equal(t, actionlog.PSPState{Current: 181, Max: 181}, state, "50 asked, 30 applied")
		entries, err := bs.Log.ActiveEntries(ctx, id, actionlog.KindPSPChange, 1)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, 30, entries[0].PSPDelta)
		assert.JSONEq(t, `{"pspRequested":50,"pspApplied":30}`, string(entries[0].Payload))
		undoTop(t, bs, id)
		assert.Equal(t, 151, currentPSP(t, bs, id), "the 30 applied are taken back, not the 50 asked")
		redoTop(t, bs, id)
		assert.Equal(t, 181, currentPSP(t, bs, id))
		undoTop(t, bs, id)
		undoTop(t, bs, id)
		assert.Equal(t, 181, currentPSP(t, bs, id), "both popped: the pool is as before")
	})
	t.Run("a loss past 0 is floored there", func(t *testing.T) {
		bs, id := thiefPsionicist(t)
		state, err := bs.Log.AppendPSPChange(ctx, id, -200, "")
		require.NoError(t, err)
		assert.Equal(t, 0, state.Current)
		undoTop(t, bs, id)
		assert.Equal(t, 181, currentPSP(t, bs, id), "the 181 applied come back")
	})
	t.Run("refused without a pool or a delta, nothing logged", func(t *testing.T) {
		m := memory.New()
		bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
		fighter, err := m.AddCharacter(ctx, charactersvc.Character{
			Name: "Fighter", Race: "human", Class: "fighter", RealizedLevel: 3,
			Str: 16, Dex: 12, Con: 14, Int: 10, Wis: 10, Cha: 10, HPMax: 24,
			Classes: []classes.Entry{classes.SingleEntry("fighter", 3, 0)},
		})
		require.NoError(t, err)
		_, err = bs.Log.AppendPSPChange(ctx, fighter, 5, "")
		require.ErrorIs(t, err, actionlogsvc.ErrNoPSPPool)
		_, found, err := bs.Log.TopID(ctx, fighter)
		require.NoError(t, err)
		assert.False(t, found)
		psi, psionicist := thiefPsionicist(t)
		_, err = psi.Log.AppendPSPChange(ctx, psionicist, 0, "")
		require.ErrorIs(t, err, domain.ErrInvalidInput)
		_, found, err = psi.Log.TopID(ctx, psionicist)
		require.NoError(t, err)
		assert.False(t, found)
	})
}

// Step 6 through the real ledger: a recovery's gain is capped at the PSPs
// spent (step 5), and undoing it takes back what was applied.
func TestRecover_Step6_CappedAndUndone(t *testing.T) {
	ctx := context.Background()
	bs, id := thiefPsionicist(t)
	_, err := bs.Log.AppendPSPChange(ctx, id, -30, "")
	require.NoError(t, err)
	state, err := bs.Psionics.Recover(ctx, id, 3, "resting")
	require.NoError(t, err)
	assert.Equal(t, actionlog.PSPState{Current: 169, Max: 181}, state, "3 h resting: 18")
	state, err = bs.Psionics.Recover(ctx, id, 2, "sleeping")
	require.NoError(t, err)
	assert.Equal(t, actionlog.PSPState{Current: 181, Max: 181}, state, "24 asked, 12 applied")
	entries, err := bs.Log.ActiveEntries(ctx, id, actionlog.KindPSPRecovery, 1)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, 12, entries[0].PSPDelta)
	assert.JSONEq(t, `{"hours":2,"activity":"sleeping","perHour":12,"pspRequested":24,"pspApplied":12}`, string(entries[0].Payload))
	undoTop(t, bs, id)
	assert.Equal(t, 169, currentPSP(t, bs, id), "the 12 applied are taken back")
	undoTop(t, bs, id)
	assert.Equal(t, 151, currentPSP(t, bs, id))
}

// Step 9: a psionicist's read carries Table 4's allowance at the psionics
// entry's own realized level (14, not the thief's 15), what the stored picks
// take (a relearn takes the place of a devotion, a defence mode counts apart)
// and the primary discipline; a character with no psionics entry has none.
func TestKnownPowers_Step9_DerivedBesideThePSPPool(t *testing.T) {
	ctx := context.Background()
	bs, id := thiefPsionicist(t)
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, c.Derived.Powers)
	assert.Equal(t, psionics.Allowance{Disciplines: 5, Sciences: 7, Devotions: 19, DefenceModes: 5}, c.Derived.Powers.Allowance)
	assert.Equal(t, psionics.Used{}, c.Derived.Powers.Used)
	assert.Empty(t, c.Derived.Powers.Primary)
	assert.NotNil(t, c.Derived.Powers.Picks)

	picks := []psionics.Pick{
		{Kind: psionics.PickDiscipline, ID: "telepathy"},
		{Kind: psionics.PickPower, ID: "contact"},
		{Kind: psionics.PickPower, ID: "mindlink"},
		{Kind: psionics.PickRelearn, ID: "contact"},
		{Kind: psionics.PickPower, ID: "mind_blank"},
	}
	bs, id = thiefPsionicistKnowing(t, picks)
	c, err = bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, psionics.Used{Disciplines: 1, Sciences: 1, Devotions: 2, DefenceModes: 1}, c.Derived.Powers.Used)
	assert.Equal(t, "telepathy", c.Derived.Powers.Primary)
	assert.Equal(t, picks, c.Derived.Powers.Picks)
}

// Step 9: the player saves the whole list; it is one psionic_pick entry,
// undone and redone exactly; a list over Table 4 is refused and writes
// nothing; a character with no psionics entry is refused.
func TestKnownPowers_Step9_SaveUndoRedoAndRefusal(t *testing.T) {
	ctx := context.Background()
	bs, id := thiefPsionicist(t)
	picks := []psionics.Pick{
		{Kind: psionics.PickDiscipline, ID: "telepathy"},
		{Kind: psionics.PickPower, ID: "contact"},
	}
	k, err := bs.Psionics.SetKnownPowers(ctx, id, picks)
	require.NoError(t, err)
	assert.Equal(t, "telepathy", k.Primary)
	assert.Equal(t, psionics.Used{Disciplines: 1, Devotions: 1}, k.Used)
	stored := func() []psionics.Pick {
		c, err := bs.Character.GetCharacter(ctx, id)
		require.NoError(t, err)
		return c.Derived.Powers.Picks
	}
	assert.Equal(t, picks, stored())

	log, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, log, 1)
	assert.Equal(t, actionlog.KindPsionicPick, log[0].Kind)

	// Over the allowance: nothing written, nothing logged.
	over := make([]psionics.Pick, 0, 6)
	for _, d := range []string{"telepathy", "clairsentience", "psychokinesis", "psychometabolism", "psychoportation", "metapsionics"} {
		over = append(over, psionics.Pick{Kind: psionics.PickDiscipline, ID: d})
	}
	_, err = bs.Psionics.SetKnownPowers(ctx, id, over)
	require.Error(t, err)
	assert.Equal(t, picks, stored())

	_, err = bs.Log.Undo(ctx, id, log[0].ID)
	require.NoError(t, err)
	assert.Empty(t, stored())
	redoTop(t, bs, id)
	assert.Equal(t, picks, stored())

	m := memory.New()
	other, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, other.Close(ctx)) })
	fid, err := m.AddCharacter(ctx, charactersvc.Character{
		Name: "Fighter", Race: "human", Class: "fighter", RealizedLevel: 3, Str: 12, Dex: 12, Con: 12, Int: 12, Wis: 12, Cha: 12, HPMax: 20,
		Classes: []classes.Entry{classes.SingleEntry("fighter", 3, 0)},
	})
	require.NoError(t, err)
	_, err = other.Psionics.SetKnownPowers(ctx, fid, nil)
	assert.ErrorIs(t, err, psionicsvc.ErrNotPsionicist)
}

// Step 9: undoing a save takes the picks it made away, and nothing else.
func TestPicks_Step9_UndoRemovesThePick(t *testing.T) {
	ctx := context.Background()
	first := []psionics.Pick{{Kind: psionics.PickDiscipline, ID: "telepathy"}}
	bs, id := thiefPsionicistKnowing(t, first)
	_, err := bs.Psionics.SetKnownPowers(ctx, id, append(first, psionics.Pick{Kind: psionics.PickPower, ID: "contact"}))
	require.NoError(t, err)

	log, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, log, 1)
	_, err = bs.Log.Undo(ctx, id, log[0].ID)
	require.NoError(t, err)
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, first, c.Derived.Powers.Picks)
}

// Step 11: a power's use spends its PSPs in its power_check entry, so one
// undo gives them back and a redo spends them again. Psionic blast costs 10
// against Wisdom 17 - 5 = 12: a 5 succeeds and spends 10, a 15 fails and
// spends half, 5. The entry records the power's catalogue name.
func TestActivate_Step11_UndoGivesThePSPBack(t *testing.T) {
	ctx := context.Background()
	dice := &attackDice{t: t, dies: []int{5, 15}, calls: map[string]int{}}
	bs, id := thiefPsionicistRolling(t, []psionics.Pick{
		{Kind: psionics.PickDiscipline, ID: "telepathy"}, {Kind: psionics.PickPower, ID: "psionic_blast"},
	}, dice)

	res, err := bs.Psionics.Use(ctx, id, "psionic_blast", psionicsvc.UseRequest{})
	require.NoError(t, err)
	assert.Equal(t, 12, res.Check.Target)
	assert.Equal(t, "Souffle psionique", res.Check.Name["fr"], "the name is recorded in every language")
	assert.Equal(t, psionics.OutcomeSuccess, res.Check.Outcome)
	assert.Equal(t, actionlog.PSPState{Current: 171, Max: 181}, res.PSP)
	assert.Equal(t, 171, currentPSP(t, bs, id), "the answer is the pool as read")
	top, ok, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, res.LogEntryID, top)

	undoTop(t, bs, id)
	assert.Equal(t, 181, currentPSP(t, bs, id), "the undo gives the 10 back")
	redoTop(t, bs, id)
	assert.Equal(t, 171, currentPSP(t, bs, id), "the redo spends them again, no die rolled")

	res, err = bs.Psionics.Use(ctx, id, "psionic_blast", psionicsvc.UseRequest{})
	require.NoError(t, err)
	assert.Equal(t, psionics.OutcomeFailure, res.Check.Outcome)
	assert.Equal(t, 166, currentPSP(t, bs, id), "a failure spends half")
	undoTop(t, bs, id)
	assert.Equal(t, 171, currentPSP(t, bs, id))
	assert.Equal(t, 2, dice.calls["die"], "one d20 per use, none on undo or redo")
}

// ADR 0043 steps 13 and 14 end to end: Aura Sight (9 PSPs, 9 a round, Wis 17
// - 5 = 12) succeeds at round 0 and puts its row on; the Next Round of its
// start round charges nothing, the next one charges 9; undoing that Next
// Round gives the PSPs back and leaves the row on, undoing the check takes
// the row off.
func TestAdvanceRound_Step14_UndoRestoresPSPAndRows(t *testing.T) {
	ctx := context.Background()
	dice := &attackDice{t: t, dies: []int{5, 3, 4}, calls: map[string]int{}}
	bs, id := thiefPsionicistRolling(t, []psionics.Pick{
		{Kind: psionics.PickDiscipline, ID: "clairsentience"}, {Kind: psionics.PickPower, ID: "aura_sight"},
	}, dice)
	rows := func() int {
		b, err := bs.Buffs.ListActive(ctx, id)
		require.NoError(t, err)
		return len(b)
	}

	res, err := bs.Psionics.Use(ctx, id, "aura_sight", psionicsvc.UseRequest{})
	require.NoError(t, err)
	require.True(t, res.Check.Success)
	require.NotEmpty(t, res.BuffID)
	assert.Equal(t, 172, currentPSP(t, bs, id))
	assert.Equal(t, 1, rows())

	_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 172, currentPSP(t, bs, id), "the start round's Next Round charges nothing")
	_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 163, currentPSP(t, bs, id), "9 to keep it from round 1")

	undoTop(t, bs, id)
	assert.Equal(t, 172, currentPSP(t, bs, id), "the undo gives the 9 back")
	assert.Equal(t, 1, rows())
	undoTop(t, bs, id)
	undoTop(t, bs, id)
	assert.Equal(t, 181, currentPSP(t, bs, id))
	assert.Equal(t, 0, rows(), "the check's undo takes the row off")
}
