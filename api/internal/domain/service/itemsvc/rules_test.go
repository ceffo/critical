package itemsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// ADR 0049 step 2's table, written out: ammunition, gear, a potion and a
// scroll have a quantity; every other piece is one object.
var wantQuantity = map[itemsvc.Category]bool{
	itemsvc.CategoryWeapon: false, itemsvc.CategoryAmmunition: true, itemsvc.CategoryArmour: false,
	itemsvc.CategoryShield: false, itemsvc.CategoryMagic: false, itemsvc.CategoryGear: true,
}

var wantMagicQuantity = map[string]bool{
	itemsvc.MagicPotion: true, itemsvc.MagicScroll: true, itemsvc.MagicRing: false,
	itemsvc.MagicRod: false, itemsvc.MagicStaff: false, itemsvc.MagicWand: false, itemsvc.MagicMisc: false,
}

// ADR 0049 step 2: the rules the editor reads and Item.HasStock both equal the
// table above for every category and magic kind, in display order. The table
// is independent of HasStock, so changing the rule for a kind fails here.
func TestQuantityRules_Step2_EqualTheTableAndHasStockForEveryCategoryAndKind(t *testing.T) {
	rules := itemsvc.QuantityRules()
	require.Len(t, rules, len(itemsvc.Categories))
	for n, c := range itemsvc.Categories {
		r := rules[n]
		assert.Equal(t, c, r.Category)
		if c != itemsvc.CategoryMagic {
			assert.Empty(t, r.Kinds, c)
			assert.Equal(t, wantQuantity[c], r.HasQuantity, c)
			assert.Equal(t, wantQuantity[c], itemsvc.Item{Category: c}.HasStock(), c)
			continue
		}
		require.Len(t, r.Kinds, len(itemsvc.MagicKinds))
		for k, kind := range itemsvc.MagicKinds {
			assert.Equal(t, kind, r.Kinds[k].Kind)
			assert.Equal(t, wantMagicQuantity[kind], r.Kinds[k].HasQuantity, kind)
			assert.Equal(t, wantMagicQuantity[kind], itemsvc.Item{Category: c, MagicKind: kind}.HasStock(), kind)
		}
	}
}
