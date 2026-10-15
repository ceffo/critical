package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// Passing time on the real stack: timed buffs count down, the counter goes
// to 0, and voiding the one entry brings back the buffs and the round.
func TestPassTimeIsLoggedAndVoidRestoresBuffsAndRound(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID

	for _, b := range []buffsvc.BuffEffect{
		{Name: "Long", Duration: 100},
		{Name: "Short", Duration: 60},
		{Name: "Stoneskin", Charges: 4},
	} {
		_, err := bs.Buffs.Activate(ctx, id, b)
		require.NoError(t, err)
	}
	_, _, err := bs.Buffs.SetRound(ctx, id, 3)
	require.NoError(t, err)

	active, expired, rounds, round, err := bs.Buffs.PassTime(ctx, id, 1.5)
	require.NoError(t, err)
	assert.Equal(t, 90, rounds)
	assert.Equal(t, 0, round)
	assert.Equal(t, []string{"Long", "Stoneskin"}, names(active))
	assert.Equal(t, []string{"Short"}, names(expired))
	r, err := bs.Buffs.GetRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 0, r)
	buffs, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	require.Len(t, buffs, 2)
	assert.Equal(t, 10, buffs[0].Duration)
	assert.Equal(t, 4, buffs[1].Charges, "charge-only buff untouched")

	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindTimePassed, entries[0].Kind)
	assert.Equal(t, 3, entries[0].Round, "logged in the round it ends")

	_, err = bs.Log.Undo(ctx, id, entries[0].ID)
	require.NoError(t, err)
	r, err = bs.Buffs.GetRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 3, r, "void restores the previous round")
	buffs, err = bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"Long", "Short", "Stoneskin"}, names(buffs))
	assert.Equal(t, 100, buffs[0].Duration)
	assert.Equal(t, 60, buffs[1].Duration)
}

// Time passing touches buffs and the round only: a spent daily item and a
// used spell slot stay as they are (only a long rest refills them).
func TestPassTimeDoesNotRefillDailyItemsOrSlots(t *testing.T) {
	bs := boot(t)
	spend(t, bs)
	require.Zero(t, gemCount(t, bs))
	require.Equal(t, 1, usedSlots(t, bs))

	_, _, _, _, err := bs.Buffs.PassTime(context.Background(), bs.CharacterID, 8)
	require.NoError(t, err)

	assert.Zero(t, gemCount(t, bs), "the daily gem stays spent")
	assert.Equal(t, 1, usedSlots(t, bs), "the used slot stays used")
}
