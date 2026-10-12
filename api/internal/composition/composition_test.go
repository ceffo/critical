package composition_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

func newBacking(t *testing.T) *composition.BackingServices {
	t.Helper()
	ctx := context.Background()
	bs, err := composition.New(ctx, composition.Config{
		DSN:      filepath.Join(t.TempDir(), "critical.db"),
		SeedsDir: filepath.Join("..", "..", "..", "seeds"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bs.Close(ctx) })
	// Pin the sheet's STR so the Power Gem needs a Strength buff to reach 18.
	_, err = bs.DB.ExecContext(ctx, `UPDATE characters SET str = 12 WHERE id = ?`, bs.CharacterID)
	require.NoError(t, err)
	return bs
}

func names(buffs []buffsvc.BuffEffect) []string {
	out := []string{}
	for _, b := range buffs {
		out = append(out, b.Name)
	}
	return out
}

// Every buff change is logged on the real stack and every one can be voided
// newest first, which needs a Restorer registered for each state key the
// buff service records (buffs, round). The writes and the log entries share
// one transaction (a pool write inside it would fail with "database is
// locked").
func TestBuffChangesAreLoggedAndVoidable(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID

	_, err := bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{
		Name: "Strength", Duration: 2,
		Effects: buffsvc.BuffEffects{StrBonus: intPtr(6), StrMax: intPtr(18)},
	})
	require.NoError(t, err)
	gem, err := bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{
		Name:    "Power Gem",
		Effects: buffsvc.BuffEffects{MinStr: intPtr(18), WeaponBuff: intPtr(5), Multiplier: floatPtr(2)},
	})
	require.NoError(t, err)
	stone, err := bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Stoneskin", Charges: 2})
	require.NoError(t, err)
	strID, stoneID := gem[0].ID, stone[2].ID

	mods, err := bs.Buffs.Aggregate(ctx, id, 12)
	require.NoError(t, err)
	assert.Equal(t, 18, mods.EffectiveStr)
	assert.Equal(t, 5, mods.WeaponBuff, "Power Gem applies at effective STR 18")

	_, err = bs.Buffs.Trigger(ctx, id, stoneID)
	require.NoError(t, err)
	after, err := bs.Buffs.Trigger(ctx, id, stoneID)
	require.NoError(t, err)
	assert.Equal(t, []string{"Strength", "Power Gem"}, names(after), "Stoneskin ends at 0 layers")

	_, err = bs.Buffs.Edit(ctx, id, strID, buffsvc.BuffEdit{Duration: intPtr(1)})
	require.NoError(t, err)

	active, expired, round, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 1, round)
	assert.Equal(t, []string{"Power Gem"}, names(active))
	assert.Equal(t, []string{"Strength"}, names(expired))
	str, err := bs.Buffs.GetEffectiveStr(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, buffsvc.Strength{Base: 12, Effective: 12}, str, "STR drops when Strength expires")
	mods, err = bs.Buffs.Aggregate(ctx, id, 12)
	require.NoError(t, err)
	assert.Zero(t, mods.WeaponBuff, "Power Gem no longer applies")

	entries, err := bs.Log.List(ctx, id, 50)
	require.NoError(t, err)
	require.Len(t, entries, 7)
	assert.Equal(t, actionlog.KindRoundAdvance, entries[0].Kind)
	assert.Equal(t, 0, entries[0].Round, "Next Round is logged in the round it ends")

	// Void Next Round: the round, the durations and the expired buff come back.
	_, err = bs.Log.Void(ctx, id, entries[0].ID)
	require.NoError(t, err)
	r, err := bs.Buffs.GetRound(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 0, r)
	buffs, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"Strength", "Power Gem"}, names(buffs))
	assert.Equal(t, 1, buffs[0].Duration)
	str, err = bs.Buffs.GetEffectiveStr(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 18, str.Effective)

	// Void the rest newest first (edit, both absorbs, three activations).
	for _, e := range entries[1:] {
		_, err := bs.Log.Void(ctx, id, e.ID)
		require.NoError(t, err, "void #%d %s", e.ID, e.Summary)
		if e.Summary == "Edited Strength: 2 rd → 1 rd" {
			buffs, err := bs.Buffs.ListActive(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, 2, buffs[0].Duration, "voiding the edit restores the rounds")
		}
		if e.Summary == "Stoneskin absorbed a hit, ended" {
			buffs, err := bs.Buffs.ListActive(ctx, id)
			require.NoError(t, err)
			require.Len(t, buffs, 3)
			assert.Equal(t, 1, buffs[2].Charges, "voiding the last absorb brings Stoneskin back")
		}
	}
	buffs, err = bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, buffs)
}

// A void is refused while a later entry changed the same state.
func TestVoidingAnOlderBuffChangeIsBlocked(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID

	_, err := bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Haste", Duration: 3})
	require.NoError(t, err)
	_, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)

	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	_, err = bs.Log.Void(ctx, id, entries[1].ID)
	assert.Error(t, err)
}
