package itemsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// quantityRow is one row of ADR 0049 step 2's table: a category (and magic kind), whether the
// player moves its quantity, and what a spend to 0 does.
type quantityRow struct {
	name      string
	item      itemsvc.Item
	stock     bool // restock and spend one are allowed
	goneAtOne bool // spending the last one removes the item
}

func quantityRows() []quantityRow {
	mk := func(c itemsvc.Category, k string) itemsvc.Item {
		return itemsvc.Item{ID: "x", Name: localized.Of("X"), Type: "t", Category: c, MagicKind: k, Count: 1}
	}
	return []quantityRow{
		{"weapon", mk(itemsvc.CategoryWeapon, ""), false, false},
		{"armour", mk(itemsvc.CategoryArmour, ""), false, false},
		{"shield", mk(itemsvc.CategoryShield, ""), false, false},
		{"ammunition", mk(itemsvc.CategoryAmmunition, ""), true, false},
		{"gear", mk(itemsvc.CategoryGear, ""), true, false},
		{"magic potion", mk(itemsvc.CategoryMagic, itemsvc.MagicPotion), true, true},
		{"magic scroll", mk(itemsvc.CategoryMagic, itemsvc.MagicScroll), true, true},
		{"magic ring", mk(itemsvc.CategoryMagic, itemsvc.MagicRing), false, false},
		{"magic rod", mk(itemsvc.CategoryMagic, itemsvc.MagicRod), false, false},
		{"magic staff", mk(itemsvc.CategoryMagic, itemsvc.MagicStaff), false, false},
		{"magic wand", mk(itemsvc.CategoryMagic, itemsvc.MagicWand), false, false},
		{"magic misc", mk(itemsvc.CategoryMagic, itemsvc.MagicMisc), false, false},
	}
}

func quantitySvc(t *testing.T) (itemsvc.SVC, *mocks.MockDAL, fixture) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	f.chars.EXPECT().WeaponProficiency(mock.Anything, mock.Anything).Return(skills.WeaponProficiency{}, nil).Maybe()
	svc, err := itemsvc.New(dal, f.Deps)
	require.NoError(t, err)
	return svc, dal, f
}

// ADR 0049 step 2: spend one, row by row. A stock row spends one in one entry; a one-object row
// is refused with ErrNotUsable, nothing moves or is logged (the strict mocks fail on any other call).
func TestUseItem_ADR0049Step2_EachCategoryRow(t *testing.T) {
	for _, r := range quantityRows() {
		t.Run(r.name, func(t *testing.T) {
			svc, dal, f := quantitySvc(t)
			twoHeld := r.item
			twoHeld.Count = 2
			dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(twoHeld, nil)
			if !r.stock {
				_, err := svc.UseItem(context.Background(), 1, "x")
				assert.ErrorIs(t, err, itemsvc.ErrNotUsable)
				return
			}
			after := r.item // one left
			dal.EXPECT().Decrement(mock.Anything, int64(1), "x").Return(after, nil)
			f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
			res, err := svc.UseItem(context.Background(), 1, "x")
			require.NoError(t, err)
			assert.Equal(t, 1, res.Remaining)
			assert.False(t, res.Removed, "one is left")
		})
	}
}

// ADR 0049 step 2, "At 0": a potion or a scroll spent to 0 is removed in the same entry (one
// snapshot, absent after, the whole item before so undo puts it back); ammunition and gear stay listed.
func TestUseItem_ADR0049Step2_AtZeroByRow(t *testing.T) {
	for _, r := range quantityRows() {
		if !r.stock {
			continue
		}
		t.Run(r.name, func(t *testing.T) {
			svc, dal, f := quantitySvc(t)
			dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(r.item, nil)
			spent := r.item
			spent.Count = 0
			dal.EXPECT().Decrement(mock.Anything, int64(1), "x").Return(spent, nil)
			if r.goneAtOne {
				dal.EXPECT().Delete(mock.Anything, int64(1), "x").Return(nil).Once()
			}
			var got actionlog.NewEntry
			f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(1), nil).Once()
			res, err := svc.UseItem(context.Background(), 1, "x")
			require.NoError(t, err)
			assert.Equal(t, r.goneAtOne, res.Removed)
			require.Len(t, got.Snapshots, 1, "one entry, one snapshot")
		})
	}
}

// ADR 0049 step 2: restock, row by row; a one-object row is refused with ErrNotRestockable.
func TestRestock_ADR0049Step2_EachCategoryRow(t *testing.T) {
	for _, r := range quantityRows() {
		t.Run(r.name, func(t *testing.T) {
			svc, dal, f := quantitySvc(t)
			dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(r.item, nil)
			if !r.stock {
				_, err := svc.Restock(context.Background(), 1, "x", 1)
				assert.ErrorIs(t, err, itemsvc.ErrNotRestockable)
				return
			}
			more := r.item
			more.Count = 3
			dal.EXPECT().Increment(mock.Anything, int64(1), "x", 2).Return(more, nil)
			f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
			res, err := svc.Restock(context.Background(), 1, "x", 2)
			require.NoError(t, err)
			assert.Equal(t, 3, res.Count)
		})
	}
}

// ADR 0049 step 2: a quantity is 0 to 999. The editor refuses 1000 and takes 999; a restock that
// would build more than 999 is refused before any write (the strict DAL fails on an Increment).
func TestRestock_ADR0049Step2_StopsAtTheQuantityBound(t *testing.T) {
	arrows := itemsvc.Item{ID: "x", Name: localized.Of("Arrows"), Type: "arrow", Category: itemsvc.CategoryAmmunition, Count: 990}
	svc, dal, f := quantitySvc(t)
	dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(arrows, nil)
	_, err := svc.Restock(context.Background(), 1, "x", 10)
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	full := arrows
	full.Count = itemsvc.MaxQuantity
	dal.EXPECT().Increment(mock.Anything, int64(1), "x", 9).Return(full, nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
	res, err := svc.Restock(context.Background(), 1, "x", 9)
	require.NoError(t, err)
	assert.Equal(t, itemsvc.MaxQuantity, res.Count)
}

func TestValidate_ADR0049Step2_QuantityIs0To999(t *testing.T) {
	for count, ok := range map[int]bool{-1: false, 0: true, 999: true, 1000: false} {
		it := itemsvc.Item{Name: localized.Of("Arrows"), Category: itemsvc.CategoryAmmunition, AmmoKind: "arrow", Behaviour: itemsvc.BehaviourStock, State: itemsvc.StateCarried, Count: count}
		if ok {
			assert.NoError(t, it.Validate(), count)
		} else {
			assert.Error(t, it.Validate(), count)
		}
	}
}
