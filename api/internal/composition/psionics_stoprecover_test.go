package composition_test

// ADR 0047 item 4 end to end: the paying rows end and the PSPs recover in one
// entry, and one undo gives both back.

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
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

var payingAndFree = []psionics.Pick{
	{Kind: psionics.PickDiscipline, ID: "clairsentience"},
	{Kind: psionics.PickPower, ID: "aura_sight"},
	{Kind: psionics.PickDiscipline, ID: "telepathy"},
	{Kind: psionics.PickPower, ID: "mind_blank"},
}

// Aura Sight (paying, two rounds kept), a tangent row on a mind (paying) and
// a standing mind blank (free), with 40 PSPs spent after them.
func stopFixture(t *testing.T) (*composition.BackingServices, int64, []buffsvc.BuffEffect) {
	t.Helper()
	ctx := context.Background()
	bs, id := thiefPsionicistRolling(t, payingAndFree, fives(t))
	_, err := bs.Psionics.Use(ctx, id, "aura_sight", psionicsvc.UseRequest{})
	require.NoError(t, err)
	_, err = bs.Psionics.RaiseDefence(ctx, id, "mind_blank")
	require.NoError(t, err)
	for range 2 {
		_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
		require.NoError(t, err)
	}
	_, _, err = bs.Buffs.AddTangents(ctx, id, "Grey Ogre", 2)
	require.NoError(t, err)
	spendPSP(t, bs, id, 40)
	rows, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	return bs, id, rows
}

// Item 4: the paying rows end by their maintenance data, the free one stays,
// the recovery is hours x rate, and the single entry is the only thing to
// undo: it puts the rows back as they were (rounds kept, tangents) and takes
// the PSPs off. Recover alone still refuses while a paying row is up.
func TestStopAndRecover_ADR0047Item4_UndoRestoresBoth(t *testing.T) {
	ctx := context.Background()
	bs, id, before := stopFixture(t)
	pspBefore := currentPSP(t, bs, id)
	_, err := bs.Psionics.Recover(ctx, id, 2, "resting")
	require.ErrorIs(t, err, psionicsvc.ErrRecoveryWhileMaintaining)

	res, err := bs.Psionics.StopAndRecover(ctx, id, 2, "resting")
	require.NoError(t, err)
	var endedPower, endedTarget string
	for _, b := range res.Ended {
		endedPower += b.PowerID
		endedTarget += b.Target
	}
	assert.Len(t, res.Ended, 2)
	assert.Equal(t, "aura_sight", endedPower)
	assert.Equal(t, "Grey Ogre", endedTarget)
	assert.Greater(t, res.PSP.Current, pspBefore, "recovered")
	assert.Equal(t, currentPSP(t, bs, id), res.PSP.Current)
	left, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	require.Len(t, left, 1, "the free standing defence stays up")
	assert.Equal(t, buffsvc.DefenceStanding, left[0].Defence)

	entries, err := bs.Log.ActiveEntries(ctx, id, actionlog.KindPSPRecovery, 5)
	require.NoError(t, err)
	require.Len(t, entries, 1, "one entry")
	assert.Contains(t, string(entries[0].Payload), `"powersEnded":["aura_sight"]`)
	assert.Contains(t, string(entries[0].Payload), `"targetsEnded":["Grey Ogre"]`)

	undoTop(t, bs, id)
	assert.Equal(t, pspBefore, currentPSP(t, bs, id), "the PSPs are taken off")
	back, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.ElementsMatch(t, before, back, "rows back with their rounds kept, tangents and all")
	_, err = bs.Psionics.Recover(ctx, id, 1, "resting")
	require.ErrorIs(t, err, psionicsvc.ErrRecoveryWhileMaintaining, "kept up again")

	redoTop(t, bs, id)
	assert.Equal(t, res.PSP.Current, currentPSP(t, bs, id))
	left, err = bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Len(t, left, 1, "the redo ends them again")
}

// Item 4: nothing paying is up (only a free row): the recovery is not refused
// and no row is touched.
func TestStopAndRecover_ADR0047Item4_FreeRowsDoNotBlock(t *testing.T) {
	ctx := context.Background()
	bs, id := thiefPsionicistRolling(t, payingAndFree, fives(t))
	_, err := bs.Psionics.RaiseDefence(ctx, id, "mind_blank")
	require.NoError(t, err)
	spendPSP(t, bs, id, 20)
	res, err := bs.Psionics.StopAndRecover(ctx, id, 1, "resting")
	require.NoError(t, err)
	assert.Empty(t, res.Ended)
	assert.Len(t, defenceRows(t, bs.Buffs, id), 1)
}

// Item 4 with item 6: the gate is Recover's, shared. A gated activity is
// refused without the proficiency, and nothing is ended.
func TestStopAndRecover_ADR0047Item4_GatedActivityAndNoPool(t *testing.T) {
	ctx := context.Background()
	t.Run("gated, without the pick: refused, rows kept", func(t *testing.T) {
		bs, id, before := stopFixture(t)
		_, err := bs.Psionics.StopAndRecover(ctx, id, 2, "meditating")
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "activity", fe.Field)
		rows, err := bs.Buffs.ListActive(ctx, id)
		require.NoError(t, err)
		assert.ElementsMatch(t, before, rows)
	})
	t.Run("gated, with the pick: recovers 12 an hour", func(t *testing.T) {
		held := skills.Picks{Nonweapon: []skills.Pick{{ID: "rejuvenation", Slots: 1}}}
		bs, id := thiefPsionicistHolding(t, nil, nil, fixtureCatalogue, held)
		spendPSP(t, bs, id, 100)
		res, err := bs.Psionics.StopAndRecover(ctx, id, 2, "meditating")
		require.NoError(t, err)
		assert.Equal(t, 81+24, res.PSP.Current)
	})
}

// Item 4: a character with no PSP pool is refused as Recover refuses, and
// nothing is logged.
func TestStopAndRecover_ADR0047Item4_NoPoolRefused(t *testing.T) {
	ctx := context.Background()
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
	_, err = bs.Psionics.StopAndRecover(ctx, fighter, 1, "resting")
	require.ErrorIs(t, err, actionlogsvc.ErrNoPSPPool)
	_, found, err := bs.Log.TopID(ctx, fighter)
	require.NoError(t, err)
	assert.False(t, found)
}
