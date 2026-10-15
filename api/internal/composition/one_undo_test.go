package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
)

// journalUndo is what the journal's Undo button does: the global undo, with
// the top the panel last read as expectedTopId (ADR 0018 step 3). It returns
// the HP state the panel then shows.
func journalUndo(t *testing.T, bs *composition.BackingServices) *hpsvc.HPState {
	t.Helper()
	ctx := context.Background()
	top, ok, err := bs.Log.TopID(ctx, bs.CharacterID)
	require.NoError(t, err)
	require.True(t, ok, "something to undo")
	_, err = bs.Log.Undo(ctx, bs.CharacterID, top)
	require.NoError(t, err)
	st, err := bs.HP.GetState(ctx, bs.CharacterID)
	require.NoError(t, err)
	return st
}

// ADR 0018 step 3: the journal's Undo pops whatever is on top, not only an
// HP change.
func TestOneUndo_JournalPopsWhateverIsOnTop(t *testing.T) {
	t.Run("an exposure on top: HP comes back", func(t *testing.T) {
		bs := bootMaxDice(t)
		ctx, id := context.Background(), bs.CharacterID
		st, err := bs.HP.GetState(ctx, id)
		require.NoError(t, err)
		start := st.CurrentHp

		res, err := bs.Cast.Expose(ctx, id, castsvc.ExposureRequest{SpellID: "death_fog", CasterLevel: 12, Round: 3, OnSelf: true})
		require.NoError(t, err)
		require.Equal(t, 4, res.Damage.Total)
		st, err = bs.HP.GetState(ctx, id)
		require.NoError(t, err)
		require.Equal(t, start-4, st.CurrentHp)

		assert.Equal(t, start, journalUndo(t, bs).CurrentHp)
	})
	t.Run("a cast on top: the slot comes back", func(t *testing.T) {
		bs := bootMaxDice(t)
		ctx, id := context.Background(), bs.CharacterID
		_, err := bs.Prep.Prepare(ctx, id, []string{"stoneskin", "fireball"})
		require.NoError(t, err)
		_, err = bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
		require.NoError(t, err)
		require.Equal(t, 1, usedSlots(t, bs))

		journalUndo(t, bs)
		assert.Equal(t, 0, usedSlots(t, bs))
	})
}
