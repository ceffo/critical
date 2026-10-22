package composition_test

// ADR 0043's PSP ledger end to end over the in-memory storage: the real
// services, the fixture's Table 6 and the action log's undo and redo.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
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
