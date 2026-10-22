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
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// thiefPsionicist stores the party's human thief 15/psionicist 14 with 181
// max PSPs (the sheet's: 25 + 13 x 12) and returns its id.
func thiefPsionicist(t *testing.T) (*composition.BackingServices, int64) {
	t.Helper()
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id, err := m.AddCharacter(ctx, charactersvc.Character{
		Name: "Thief/psionicist", Race: "human", Class: "thief", RealizedLevel: 15,
		Str: 12, Dex: 17, Con: 16, Int: 15, Wis: 17, Cha: 10, HPMax: 50, PSPMax: 181,
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
