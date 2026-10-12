package composition_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
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

func arrowCount(t *testing.T, bs *composition.BackingServices, id string) int {
	t.Helper()
	items, err := bs.Items.List(context.Background())
	require.NoError(t, err)
	for _, it := range items {
		if it.ID == id {
			return it.Count
		}
	}
	t.Fatalf("no item %s", id)
	return 0
}

// A bow attack spends the arrow and logs the attack with an items snapshot
// in one transaction on the real stack; voiding the entry gives the arrow
// back (the items Restorer is registered), and a later arrow blocks voiding
// an older one.
func TestMissileAttackSpendsAmmoAndVoidRestoresIt(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID
	before := arrowCount(t, bs, "arrows_plus3")
	require.GreaterOrEqual(t, before, 2, "seeds/items.json has at least two +3 arrows")

	shot := combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus3", TargetAC: 5}
	first, err := bs.Combat.ResolveAttack(ctx, id, shot)
	require.NoError(t, err)
	require.NotNil(t, first.Ammo)
	assert.Equal(t, before-1, first.Ammo.Remaining)
	assert.Equal(t, before-1, arrowCount(t, bs, "arrows_plus3"))
	second, err := bs.Combat.ResolveAttack(ctx, id, shot)
	require.NoError(t, err)
	assert.Equal(t, before-2, arrowCount(t, bs, "arrows_plus3"))

	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, actionlog.KindAttack, entries[0].Kind)
	assert.Equal(t, second.LogEntryID, entries[0].ID)

	_, err = bs.Log.Void(ctx, id, first.LogEntryID)
	assert.ErrorIs(t, err, actionlogsvc.ErrVoidBlocked, "the later arrow changed the count since")

	_, err = bs.Log.Void(ctx, id, second.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, before-1, arrowCount(t, bs, "arrows_plus3"))
	_, err = bs.Log.Void(ctx, id, first.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, before, arrowCount(t, bs, "arrows_plus3"))
}

// The ammo count does not change when the attack fails (no arrows of that
// kind: the Use is rolled back with the transaction).
func TestFailedMissileAttackLeavesAmmo(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	before := arrowCount(t, bs, "fire_flask")

	_, err := bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{
		WeaponID: "shortbow", AmmoItemID: "fire_flask", TargetAC: 5,
	})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Equal(t, before, arrowCount(t, bs, "fire_flask"))
	entries, err := bs.Log.List(ctx, bs.CharacterID, 10)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// Effective STR from buffs drives the attack on the real stack: the sheet's
// STR 12 gives +0, a Strength buff to 18 gives PHB Table 1's +1 to hit.
func TestAttackUsesEffectiveStr(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID
	sword := combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5}

	res, err := bs.Combat.ResolveAttack(ctx, id, sword)
	require.NoError(t, err)
	assert.Equal(t, combatsvc.Modifier{Kind: combatsvc.ModStrength, Source: "Strength 12", Detail: "12", Value: 0}, res.Modifiers[0])

	_, err = bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{
		Name: "Strength", Duration: 2, Effects: buffsvc.BuffEffects{StrBonus: intPtr(6), StrMax: intPtr(18)},
	})
	require.NoError(t, err)
	res, err = bs.Combat.ResolveAttack(ctx, id, sword)
	require.NoError(t, err)
	assert.Equal(t, combatsvc.Modifier{Kind: combatsvc.ModStrength, Source: "Strength 18", Detail: "18", Value: 1}, res.Modifiers[0])
}
