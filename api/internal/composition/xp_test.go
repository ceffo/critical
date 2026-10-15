package composition_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
)

// The XP ledger on the real stack (fixture Bard: level 15, DEX 17, CHA 16):
// it starts empty at the sheet's XP with the bonus eligible, entries add,
// edit and delete, survive a restart, and each write is one transaction on
// the action stack (ADR 0015 step 4).
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

	entries, err := bs.Log.List(ctx, bs.CharacterID, 10)
	require.NoError(t, err)
	var kinds []actionlog.Kind
	for _, e := range entries {
		kinds = append(kinds, e.Kind)
		assert.Equal(t, e.ID, e.TransactionID, "each XP write is its own transaction")
	}
	assert.Equal(t, []actionlog.Kind{
		actionlog.KindXPRemove, actionlog.KindXPEdit, actionlog.KindXPAdd, actionlog.KindXPAdd,
	}, kinds, "a refused delete records nothing")
}

// ADR 0015 step 4, ADR 0018 scope: the journal's Undo pops one XP add, edit
// or delete and the ledger comes back whole, its total and the progress
// derived from it (to next level, percent, level cells) with it. An edit pops
// back to the entry's values before it; a delete puts the entry back under its
// id, so the undo of the add below it still finds it.
func TestXPUndo_EachVerbGivesTheLedgerBack(t *testing.T) {
	ctx := context.Background()
	verbs := map[string]func(bs *composition.BackingServices, id int64) error{
		"add": func(bs *composition.BackingServices, _ int64) error {
			_, err := bs.XP.Add(ctx, bs.CharacterID, xpsvc.EntryInput{Base: 400_000, Reason: "dragon"})
			return err
		},
		"edit": func(bs *composition.BackingServices, id int64) error {
			_, err := bs.XP.Update(ctx, bs.CharacterID, id, xpsvc.EntryPatch{Base: ptr(300_000), Bonus: ptr(false), Session: ptr("S42")})
			return err
		},
		"delete": func(bs *composition.BackingServices, id int64) error {
			_, err := bs.XP.Delete(ctx, bs.CharacterID, id)
			return err
		},
	}
	for name, verb := range verbs {
		t.Run(name, func(t *testing.T) {
			bs := bootMaxDice(t)
			l, err := bs.XP.Add(ctx, bs.CharacterID, xpsvc.EntryInput{Base: 1205, Bonus: true, Reason: "troll"})
			require.NoError(t, err)
			id := l.Entries[0].ID
			before, err := bs.XP.Get(ctx, bs.CharacterID)
			require.NoError(t, err)
			addTop := topID(t, bs)

			require.NoError(t, verb(bs, id))
			after, err := bs.XP.Get(ctx, bs.CharacterID)
			require.NoError(t, err)
			require.NotEqual(t, before.Total, after.Total, "the verb changed the total")
			require.NotEqual(t, before.ToNext, after.ToNext, "and the progress")

			_, err = bs.Log.Undo(ctx, bs.CharacterID, topID(t, bs))
			require.NoError(t, err)
			got, err := bs.XP.Get(ctx, bs.CharacterID)
			require.NoError(t, err)
			assert.Equal(t, before, got, "the ledger, its total and progress are back")
			assert.Equal(t, addTop, topID(t, bs), "the add below is on top again")

			_, err = bs.Log.Undo(ctx, bs.CharacterID, addTop)
			require.NoError(t, err)
			got, err = bs.XP.Get(ctx, bs.CharacterID)
			require.NoError(t, err)
			assert.Empty(t, got.Entries)
			assert.Equal(t, got.Opening, got.Total)
		})
	}
}
