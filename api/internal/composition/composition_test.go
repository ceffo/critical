package composition_test

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
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
		SeedsDir: filepath.Join("..", "..", "testdata", "seeds"),
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
		require.NoError(t, err, "void #%d %s", e.ID, e.Payload)
		p := string(e.Payload)
		if strings.Contains(p, `"action":"edit"`) && strings.Contains(p, `"buff":"Strength"`) {
			buffs, err := bs.Buffs.ListActive(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, 2, buffs[0].Duration, "voiding the edit restores the rounds")
		}
		if strings.Contains(p, `"action":"absorb"`) && strings.Contains(p, `"ended":true`) {
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
// back (the items Restorer is registered for the "items" family of
// per-item keys), and a later arrow of the same kind blocks voiding an older
// one.
func TestMissileAttackSpendsAmmoAndVoidRestoresIt(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID
	before := arrowCount(t, bs, "arrows_plus3")
	require.GreaterOrEqual(t, before, 2, "fixture items.json has at least two +3 arrows")

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
	assert.ErrorIs(t, err, actionlogsvc.ErrNotLatest, "a later arrow of the same kind changed the count since")

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
	assert.Equal(t, combatsvc.Modifier{Kind: combatsvc.ModStrength, Detail: "12", Value: 0}, res.Modifiers[0])

	_, err = bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{
		Name: "Strength", Duration: 2, Effects: buffsvc.BuffEffects{StrBonus: intPtr(6), StrMax: intPtr(18)},
	})
	require.NoError(t, err)
	res, err = bs.Combat.ResolveAttack(ctx, id, sword)
	require.NoError(t, err)
	assert.Equal(t, combatsvc.Modifier{Kind: combatsvc.ModStrength, Detail: "18", Value: 1}, res.Modifiers[0])
}

// Pop-only (critical-qki.38): a later +1 shot blocks voiding an earlier +3
// shot; popping the +1 then the +3 restores each item's own count.
func TestVoidingAnOlderShotIsRefused(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID
	plus3, plus1 := arrowCount(t, bs, "arrows_plus3"), arrowCount(t, bs, "arrows_plus1")

	shot3, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus3", TargetAC: 5})
	require.NoError(t, err)
	shot1, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus1", TargetAC: 5})
	require.NoError(t, err)

	_, err = bs.Log.Void(ctx, id, shot3.LogEntryID)
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)
	assert.Equal(t, plus3-1, arrowCount(t, bs, "arrows_plus3"))

	_, err = bs.Log.Void(ctx, id, shot1.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, plus1, arrowCount(t, bs, "arrows_plus1"))
	assert.Equal(t, plus3-1, arrowCount(t, bs, "arrows_plus3"), "only the +1 came back")
	_, err = bs.Log.Void(ctx, id, shot3.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, plus3, arrowCount(t, bs, "arrows_plus3"))
}

// Voiding a melee attack (no snapshot) changes no item count.
func TestVoidingAMeleeAttackLeavesItems(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	before, err := bs.Items.List(ctx)
	require.NoError(t, err)
	res, err := bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 5})
	require.NoError(t, err)
	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.NoError(t, err)
	after, err := bs.Items.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func itemCount(t *testing.T, bs *composition.BackingServices, id string) int {
	t.Helper()
	var n int
	require.NoError(t, bs.DB.QueryRow(`SELECT count FROM items WHERE id = ?`, id).Scan(&n))
	return n
}

// The wand: 4 -> 3 charges with a 6d6 roll, the entry carries the snapshot,
// and voiding it gives the charge back.
func TestUsingTheWandRollsLogsAndVoidRestoresTheCharge(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	require.Equal(t, 4, itemCount(t, bs, "wand_lightning"))

	res, err := bs.Items.UseItem(ctx, bs.CharacterID, "wand_lightning")
	require.NoError(t, err)
	assert.Equal(t, 3, res.Remaining)
	assert.Equal(t, 3, itemCount(t, bs, "wand_lightning"))
	assert.Equal(t, "6d6", res.Dice)
	assert.Len(t, res.Rolls, 6)
	assert.GreaterOrEqual(t, res.Total, 6)
	assert.LessOrEqual(t, res.Total, 36)
	assert.Equal(t, "rod_staff_wand", res.Save)

	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, 4, itemCount(t, bs, "wand_lightning"))
}

func TestUsingAnEmptyItemFailsAndLogsNothing(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ { // fire flasks: 5 in the seed
		_, err := bs.Items.UseItem(ctx, bs.CharacterID, "fire_flask")
		require.NoError(t, err)
	}
	entries := func() int {
		var n int
		require.NoError(t, bs.DB.QueryRow(`SELECT COUNT(*) FROM action_log`).Scan(&n))
		return n
	}
	before := entries()
	_, err := bs.Items.UseItem(ctx, bs.CharacterID, "fire_flask")
	assert.ErrorIs(t, err, itemsvc.ErrItemEmpty)
	assert.Equal(t, before, entries())
	assert.Zero(t, itemCount(t, bs, "fire_flask"))
}

// The Power Gem's Dispel is once a day: it rolls nothing, a second use is
// refused, and a long rest refills it.
func TestGemDispelOncePerDayUntilLongRest(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	res, err := bs.Items.UseItem(ctx, bs.CharacterID, "power_gem_dispel")
	require.NoError(t, err)
	assert.Empty(t, res.Rolls)
	assert.Zero(t, res.Remaining)

	_, err = bs.Items.UseItem(ctx, bs.CharacterID, "power_gem_dispel")
	assert.ErrorIs(t, err, itemsvc.ErrItemEmpty)

	_, err = bs.Items.ResetDaily(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, itemCount(t, bs, "power_gem_dispel"))
}

// The crit table is stored: the next crit uses it, an invalid table is refused
// and leaves the stored one, and editing never rewrites a past attack entry.
func TestCritTable_EditedTableIsUsedAndPastEntriesKeepTheirMultiplier(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	id := bs.CharacterID
	// AC 1000 makes the roll needed so low that every attack is a crit.
	shot := combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 1000}

	def, err := bs.Combat.GetCritTable(ctx)
	require.NoError(t, err)
	assert.True(t, def.Placeholder)

	before, err := bs.Combat.ResolveAttack(ctx, id, shot)
	require.NoError(t, err)
	require.True(t, before.IsCrit)
	assert.InDelta(t, 2, before.CritMultiplier, 0)
	assert.True(t, before.CritPlaceholder)

	saved, err := bs.Combat.SetCritTable(ctx, []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 5}})
	require.NoError(t, err)
	assert.False(t, saved.Placeholder)

	_, err = bs.Combat.SetCritTable(ctx, []combatsvc.CritRow{{From: 1, To: 60, Multiplier: 7}})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	got, err := bs.Combat.GetCritTable(ctx)
	require.NoError(t, err)
	assert.Equal(t, saved, got, "the refused table did not replace the stored one")

	after, err := bs.Combat.ResolveAttack(ctx, id, shot)
	require.NoError(t, err)
	assert.InDelta(t, 5, after.CritMultiplier, 0)
	assert.Equal(t, [2]int{1, 100}, [2]int{after.CritFrom, after.CritTo})
	assert.False(t, after.CritPlaceholder)

	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	byID := map[int64]string{}
	for _, e := range entries {
		byID[e.ID] = string(e.Payload)
	}
	assert.Contains(t, byID[before.LogEntryID], `"critMultiplier":2`)
	assert.Contains(t, byID[before.LogEntryID], `"critPlaceholder":true`)
	assert.Contains(t, byID[after.LogEntryID], `"critMultiplier":5`)
}

// Saving the default table explicitly (1-100 x2) is an edit: it is no longer
// reported as the placeholder, on the real stack and after the attack too.
func TestCritTable_SavingTheDefaultExplicitlyClearsPlaceholder(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()

	saved, err := bs.Combat.SetCritTable(ctx, []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 2}})
	require.NoError(t, err)
	assert.False(t, saved.Placeholder)
	got, err := bs.Combat.GetCritTable(ctx)
	require.NoError(t, err)
	assert.False(t, got.Placeholder)

	res, err := bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 1000})
	require.NoError(t, err)
	require.True(t, res.IsCrit)
	assert.InDelta(t, 2, res.CritMultiplier, 0)
	assert.False(t, res.CritPlaceholder)
}

// A decimal multiplier survives the real store and the crit uses it: the
// damage is floored once, and the log entry keeps the decimal as entered.
func TestCritTable_DecimalMultiplierIsStoredAndFloored(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	_, err := bs.Combat.SetCritTable(ctx, []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 1.5}})
	require.NoError(t, err)
	got, err := bs.Combat.GetCritTable(ctx)
	require.NoError(t, err)
	assert.InDelta(t, 1.5, got.Rows[0].Multiplier, 0)

	res, err := bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: 1000})
	require.NoError(t, err)
	require.True(t, res.IsCrit)
	assert.InDelta(t, 1.5, res.CritMultiplier, 0)
	assert.Equal(t, int(math.Floor(res.UnroundedDamage)), res.TotalDamage)
	entries, err := bs.Log.List(ctx, bs.CharacterID, 10)
	require.NoError(t, err)
	assert.Contains(t, string(entries[0].Payload), `"critMultiplier":1.5`)
}
