package itemsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// ADR 0049 step 2: the rules the editor reads equal Item.HasStock for every
// category and magic kind pair, and list them all in display order.
func TestQuantityRules_Step2_EqualHasStockForEveryCategoryAndKind(t *testing.T) {
	rules := itemsvc.QuantityRules()
	require.Len(t, rules, len(itemsvc.Categories))
	for n, c := range itemsvc.Categories {
		r := rules[n]
		assert.Equal(t, c, r.Category)
		if c != itemsvc.CategoryMagic {
			assert.Empty(t, r.Kinds, c)
			assert.Equal(t, itemsvc.Item{Category: c}.HasStock(), r.HasQuantity, c)
			continue
		}
		require.Len(t, r.Kinds, len(itemsvc.MagicKinds))
		for k, kind := range itemsvc.MagicKinds {
			assert.Equal(t, kind, r.Kinds[k].Kind)
			assert.Equal(t, itemsvc.Item{Category: c, MagicKind: kind}.HasStock(), r.Kinds[k].HasQuantity, kind)
		}
	}
}
