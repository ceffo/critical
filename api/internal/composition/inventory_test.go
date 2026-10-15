package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// Inventory behaviours on the real stack: stock, charges and
// daily items, each count change one log entry that a pop reverses.

func listed(t *testing.T, bs *composition.BackingServices) map[string]itemsvc.Item {
	t.Helper()
	items, err := bs.Items.List(context.Background())
	require.NoError(t, err)
	out := map[string]itemsvc.Item{}
	for _, i := range items {
		out[i.ID] = i
	}
	return out
}

func logCount(t *testing.T, bs *composition.BackingServices) int {
	t.Helper()
	var n int
	require.NoError(t, bs.DB.QueryRow(`SELECT COUNT(*) FROM action_log`).Scan(&n))
	return n
}

// The seed maps to the three behaviours with their type labels.
func TestInventory_SeededBehaviours(t *testing.T) {
	bs := newBacking(t)
	items := listed(t, bs)
	require.Len(t, items, 6)
	for id, want := range map[string][2]string{
		"arrows_plus1": {"stock", "arrow"}, "arrows_plus3": {"stock", "arrow"}, "fire_flask": {"stock", "flask"},
		"wand_lightning": {"charges", "wand"}, "power_gem_dispel": {"daily", "gem"},
	} {
		assert.Equal(t, want, [2]string{string(items[id].Behaviour), items[id].Type}, id)
	}
	assert.Equal(t, 1, items["arrows_plus1"].MagicBonus)
	assert.Equal(t, "arrow", items["arrows_plus1"].AmmoKind)
}

// The inventory's − on arrows spends one and logs it.
func TestInventory_UseSpendsStockAndLogs(t *testing.T) {
	bs := newBacking(t)
	res, err := bs.Items.UseItem(context.Background(), bs.CharacterID, "arrows_plus1")
	require.NoError(t, err)
	assert.Equal(t, 17, res.Remaining)
	assert.False(t, res.Removed)
	assert.Equal(t, 17, itemCount(t, bs, "arrows_plus1"))

	_, err = bs.Log.Void(context.Background(), bs.CharacterID, res.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, 18, itemCount(t, bs, "arrows_plus1"))
}

// A wand used down to 0 disappears; popping that use brings it back with 1
// charge, in its place in the list.
func TestInventory_WandAtZeroIsRemovedAndPopBringsItBack(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	orig := listed(t, bs)["wand_lightning"]

	var last *itemsvc.UseResult
	for range 4 {
		r, err := bs.Items.UseItem(ctx, bs.CharacterID, "wand_lightning")
		require.NoError(t, err)
		last = r
	}
	assert.True(t, last.Removed)
	assert.Zero(t, last.Remaining)
	_, still := listed(t, bs)["wand_lightning"]
	assert.False(t, still, "the spent wand is gone")
	_, err := bs.Items.UseItem(ctx, bs.CharacterID, "wand_lightning")
	assert.ErrorIs(t, err, itemsvc.ErrItemNotFound)

	_, err = bs.Log.Void(ctx, bs.CharacterID, last.LogEntryID)
	require.NoError(t, err)
	back, ok := listed(t, bs)["wand_lightning"]
	require.True(t, ok, "popping the use restores the wand")
	want := orig
	want.Count = 1
	assert.Equal(t, want, back)
	items, err := bs.Items.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, "wand_lightning", items[4].ID, "back in its sheet position")
}

// Arrows at 0 stay listed, can be restocked (+ adds one, logged), and the
// restock is popped like any entry.
func TestInventory_ArrowsAtZeroStayAndRestock(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	for range 2 {
		_, err := bs.Items.UseItem(ctx, bs.CharacterID, "arrows_plus3")
		require.NoError(t, err)
	}
	at0, ok := listed(t, bs)["arrows_plus3"]
	require.True(t, ok, "stock stays listed at 0")
	assert.Zero(t, at0.Count)

	res, err := bs.Items.Restock(ctx, bs.CharacterID, "arrows_plus3", 1)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Count)
	assert.Equal(t, 1, itemCount(t, bs, "arrows_plus3"))
	entries, err := bs.Log.List(ctx, bs.CharacterID, 1)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, actionlog.KindItemRestock, entries[0].Kind)
	assert.Equal(t, res.LogEntryID, entries[0].ID)

	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.NoError(t, err)
	assert.Zero(t, itemCount(t, bs, "arrows_plus3"))
}

// Charges and daily items cannot be restocked: refused, count unchanged,
// nothing logged.
func TestInventory_RestockRefusedForChargesAndDaily(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	before := logCount(t, bs)
	for id, count := range map[string]int{"wand_lightning": 4, "power_gem_dispel": 1} {
		_, err := bs.Items.Restock(ctx, bs.CharacterID, id, 1)
		assert.ErrorIs(t, err, itemsvc.ErrNotRestockable, id)
		assert.Equal(t, count, itemCount(t, bs, id), id)
	}
	assert.Equal(t, before, logCount(t, bs))
}

// Short Bow +2 with Arrows +1: both bonuses to hit and damage, one arrow
// spent per attack (unchanged by the one item model).
func TestInventory_BowWithArrowsStillAddsBothBonusesAndSpendsOne(t *testing.T) {
	bs := newBacking(t)
	res, err := bs.Combat.ResolveAttack(context.Background(), bs.CharacterID, combatsvc.AttackRequest{
		WeaponID: "shortbow", AmmoItemID: "arrows_plus1", TargetAC: new(1000), // always hits, so damage is filled
	})
	require.NoError(t, err)
	assert.Equal(t, 17, itemCount(t, bs, "arrows_plus1"))
	assert.Equal(t, 3, res.WeaponBuff, "bow +2 and arrow +1 to damage")
	var toHit int
	for _, m := range res.Modifiers {
		if m.Kind == combatsvc.ModWeaponMagic || m.Kind == combatsvc.ModAmmoMagic {
			toHit += m.Value
		}
	}
	assert.Equal(t, 3, toHit, "bow +2 and arrow +1 to hit")
}

// Add / edit / remove by hand: each is one log entry, and
// popping it (it is the newest) undoes it.

func newestEntry(t *testing.T, bs *composition.BackingServices) actionlog.Entry {
	t.Helper()
	entries, err := bs.Log.List(context.Background(), bs.CharacterID, 1)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	return entries[0]
}

func TestInventory_AddItemAppearsAndPopRemovesIt(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	res, err := bs.Items.Create(ctx, bs.CharacterID, itemsvc.ItemInput{
		Name: "Arrows +3", Type: "arrow", Behaviour: itemsvc.BehaviourStock, Count: 6, MagicBonus: 3, AmmoKind: "arrow",
	})
	require.NoError(t, err)
	assert.Equal(t, "arrows_3", res.Item.ID, "a second Arrows +3: the seeded id is arrows_plus3")
	got := listed(t, bs)[res.Item.ID]
	assert.Equal(t, 6, got.Count)
	assert.Equal(t, 3, got.MagicBonus)
	assert.Equal(t, 6, got.Position, "after the six seeded items")
	assert.Equal(t, actionlog.KindItemAdd, newestEntry(t, bs).Kind)

	// It feeds a Short Bow like any ammo.
	_, err = bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: res.Item.ID, TargetAC: new(1000)})
	require.NoError(t, err)
	assert.Equal(t, 5, itemCount(t, bs, res.Item.ID))
	// The attack is newer: the add cannot be popped before it is.
	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.Error(t, err)
}

func TestInventory_AddThenPopDeletesTheItem(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	res, err := bs.Items.Create(ctx, bs.CharacterID, itemsvc.ItemInput{Name: "Rope", Behaviour: itemsvc.BehaviourStock, Count: 1})
	require.NoError(t, err)
	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.NoError(t, err)
	_, still := listed(t, bs)[res.Item.ID]
	assert.False(t, still)
}

func TestInventory_EditWandChargesFourToThreeAndPopRestoresFour(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	res, err := bs.Items.Update(ctx, bs.CharacterID, "wand_lightning", itemsvc.ItemPatch{Count: ptr(3)})
	require.NoError(t, err)
	assert.Equal(t, 3, itemCount(t, bs, "wand_lightning"))
	assert.Equal(t, actionlog.KindItemEdit, newestEntry(t, bs).Kind)

	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, 4, itemCount(t, bs, "wand_lightning"))
}

// Every field of an edit comes back on a pop, not only the count.
func TestInventory_EditAllFieldsAndPopRestoresThem(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	orig := listed(t, bs)["wand_lightning"]
	res, err := bs.Items.Update(ctx, bs.CharacterID, "wand_lightning", itemsvc.ItemPatch{
		Name: ptr("Staff"), Type: ptr("staff"), Behaviour: ptr(itemsvc.BehaviourDaily), MaxCount: ptr(2), Count: ptr(2),
		MagicBonus: ptr(1), AmmoKind: ptr("bolt"), EffectDice: ptr("1d4+1"), EffectSave: ptr("spell"), Notes: ptr("hum"),
	})
	require.NoError(t, err)
	assert.Equal(t, "Staff", listed(t, bs)["wand_lightning"].Name)
	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, orig, listed(t, bs)["wand_lightning"])
}

func TestInventory_RemoveItemAndPopBringsItBackWithItsCount(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	orig := listed(t, bs)["arrows_plus2"]
	res, err := bs.Items.Delete(ctx, bs.CharacterID, "arrows_plus2")
	require.NoError(t, err)
	_, gone := listed(t, bs)["arrows_plus2"]
	assert.False(t, gone)
	assert.Equal(t, actionlog.KindItemRemove, newestEntry(t, bs).Kind)

	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, orig, listed(t, bs)["arrows_plus2"], "same id, position, fields and count")
	items, err := bs.Items.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, "arrows_plus2", items[1].ID)
}

func TestInventory_InvalidEditsLogNothing(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	before := logCount(t, bs)
	_, err := bs.Items.Create(ctx, bs.CharacterID, itemsvc.ItemInput{Name: "", Behaviour: itemsvc.BehaviourStock})
	require.Error(t, err)
	_, err = bs.Items.Update(ctx, bs.CharacterID, "wand_lightning", itemsvc.ItemPatch{Count: ptr(-2)})
	require.Error(t, err)
	_, err = bs.Items.Delete(ctx, bs.CharacterID, "nope")
	require.ErrorIs(t, err, itemsvc.ErrItemNotFound)
	assert.Equal(t, before, logCount(t, bs))
	assert.Equal(t, 4, itemCount(t, bs, "wand_lightning"))
}

func itemIDs(t *testing.T, bs *composition.BackingServices) []string {
	t.Helper()
	items, err := bs.Items.List(context.Background())
	require.NoError(t, err)
	var ids []string
	for _, i := range items {
		ids = append(ids, i.ID)
	}
	return ids
}

// Review checklist 1: an add whose later use was popped can itself be popped.
func TestInventory_PopAddAfterItsUseWasPopped(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	add, err := bs.Items.Create(ctx, bs.CharacterID, itemsvc.ItemInput{
		Name: "Arrows +4", Type: "arrow", Behaviour: itemsvc.BehaviourStock, Count: 3, MagicBonus: 4, AmmoKind: "arrow",
	})
	require.NoError(t, err)
	use, err := bs.Items.UseItem(ctx, bs.CharacterID, add.Item.ID)
	require.NoError(t, err)
	_, err = bs.Log.Void(ctx, bs.CharacterID, use.LogEntryID)
	require.NoError(t, err)
	_, err = bs.Log.Void(ctx, bs.CharacterID, add.LogEntryID)
	require.NoError(t, err)
	assert.NotContains(t, itemIDs(t, bs), add.Item.ID)
	_, err = bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: add.Item.ID, TargetAC: new(10)})
	require.Error(t, err, "the popped item is no longer ammo")
}

// Checklist 2: removing then popping a charges item changes nothing, order included.
func TestInventory_RemoveChargesItemAndPopKeepsOrder(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	order, orig := itemIDs(t, bs), listed(t, bs)["wand_lightning"]
	res, err := bs.Items.Delete(ctx, bs.CharacterID, "wand_lightning")
	require.NoError(t, err)
	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, orig, listed(t, bs)["wand_lightning"])
	assert.Equal(t, order, itemIDs(t, bs))
}

// Checklist 3: stock -> daily (needs a daily number) -> charges, each popped back field by field.
func TestInventory_EditBehavioursAndPopEach(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	orig := listed(t, bs)["fire_flask"]

	_, err := bs.Items.Update(ctx, bs.CharacterID, "fire_flask", itemsvc.ItemPatch{Behaviour: ptr(itemsvc.BehaviourDaily)})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "maxCount", fe.Field)
	assert.Equal(t, orig, listed(t, bs)["fire_flask"])

	toDaily, err := bs.Items.Update(ctx, bs.CharacterID, "fire_flask", itemsvc.ItemPatch{Behaviour: ptr(itemsvc.BehaviourDaily), MaxCount: ptr(7)})
	require.NoError(t, err)
	daily := listed(t, bs)["fire_flask"]
	assert.Equal(t, 7, daily.MaxCount)
	toCharges, err := bs.Items.Update(ctx, bs.CharacterID, "fire_flask", itemsvc.ItemPatch{Behaviour: ptr(itemsvc.BehaviourCharges)})
	require.NoError(t, err)
	assert.Equal(t, itemsvc.BehaviourCharges, listed(t, bs)["fire_flask"].Behaviour)

	_, err = bs.Log.Void(ctx, bs.CharacterID, toCharges.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, daily, listed(t, bs)["fire_flask"])
	_, err = bs.Log.Void(ctx, bs.CharacterID, toDaily.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, orig, listed(t, bs)["fire_flask"], "maxCount included")
}

// Checklist 4 (review fix): a charges item at 0 is refused on add and edit.
func TestInventory_ChargesAtZeroRefused(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	before := logCount(t, bs)
	_, err := bs.Items.Create(ctx, bs.CharacterID, itemsvc.ItemInput{Name: "Rod", Behaviour: itemsvc.BehaviourCharges, Count: 0})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "count", fe.Field)
	_, err = bs.Items.Update(ctx, bs.CharacterID, "wand_lightning", itemsvc.ItemPatch{Count: ptr(0)})
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "count", fe.Field)
	assert.Equal(t, 4, itemCount(t, bs, "wand_lightning"))
	assert.Equal(t, before, logCount(t, bs))
}

// Checklist 5: remove X, add a new X (same id), pop the add, pop the remove: the original is back.
func TestInventory_RemoveAddSameNamePopBoth(t *testing.T) {
	bs := newBacking(t)
	ctx := context.Background()
	orig := listed(t, bs)["fire_flask"]
	order := itemIDs(t, bs)
	rm, err := bs.Items.Delete(ctx, bs.CharacterID, "fire_flask")
	require.NoError(t, err)
	add, err := bs.Items.Create(ctx, bs.CharacterID, itemsvc.ItemInput{Name: "Fire flasks", Type: "other", Behaviour: itemsvc.BehaviourStock, Count: 1})
	require.NoError(t, err)
	assert.Equal(t, "fire_flasks", add.Item.ID)
	_, err = bs.Log.Void(ctx, bs.CharacterID, add.LogEntryID)
	require.NoError(t, err)
	_, err = bs.Log.Void(ctx, bs.CharacterID, rm.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, orig, listed(t, bs)["fire_flask"])
	assert.Equal(t, order, itemIDs(t, bs))
}
