package composition_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
)

// The XP ledger on the real stack (fixture Bard: level 15, DEX 17, CHA 16):
// it starts empty at the sheet's XP with the bonus eligible, entries add,
// edit and delete in place, survive a restart, and log nothing on the
// action stack.
func TestXPLedger_AddEditDeleteOnTheRealStack(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "critical.db")
	seeds := filepath.Join("..", "..", "testdata", "seeds")
	open := func() *composition.BackingServices {
		bs, err := composition.New(ctx, composition.Config{DSN: dsn, SeedsDir: seeds})
		require.NoError(t, err)
		return bs
	}

	bs := open()
	l, err := bs.XP.Get(ctx, bs.CharacterID)
	require.NoError(t, err)
	opening := l.Total
	assert.Equal(t, l.Opening, opening, "no seeded entries: the total is the sheet's XP")
	assert.Empty(t, l.Entries)
	assert.True(t, l.BonusEligible)
	require.NotNil(t, l.NextThreshold)
	assert.Equal(t, *l.NextThreshold-opening, l.ToNext)

	l, err = bs.XP.Add(ctx, bs.CharacterID, xpsvc.EntryInput{Base: 1205, Bonus: true, Reason: "troll"})
	require.NoError(t, err)
	assert.Equal(t, opening+1326, l.Total)
	require.Len(t, l.Entries, 1)
	id := l.Entries[0].ID
	_, err = bs.XP.Add(ctx, bs.CharacterID, xpsvc.EntryInput{Base: 100, Reason: "riddle"})
	require.NoError(t, err)

	_, hasTop, err := bs.Log.TopID(ctx, bs.CharacterID)
	require.NoError(t, err)
	assert.False(t, hasTop, "the XP ledger is outside the action stack")
	require.NoError(t, bs.Close(ctx))

	bs = open()
	t.Cleanup(func() { _ = bs.Close(ctx) })
	bonus := false
	l, err = bs.XP.Update(ctx, bs.CharacterID, id, xpsvc.EntryPatch{Bonus: &bonus})
	require.NoError(t, err)
	assert.Equal(t, opening+1205+100, l.Total)

	l, err = bs.XP.Delete(ctx, bs.CharacterID, id)
	require.NoError(t, err)
	assert.Equal(t, opening+100, l.Total)
	require.Len(t, l.Entries, 1)
	assert.Equal(t, "riddle", l.Entries[0].Reason)

	_, err = bs.XP.Delete(ctx, bs.CharacterID, id)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
