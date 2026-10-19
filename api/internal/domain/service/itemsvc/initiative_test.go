package itemsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// table56 is PHB Table 56's magic item rows (the fixture catalogue's).
func table56(t *testing.T) *ruletablesvc.InitiativeItems {
	t.Helper()
	tbl, err := ruletablesvc.NewInitiativeItems([]ruletablesvc.InitiativeItemRow{
		{Kind: "potion", Modifier: 4, HasModifier: true},
		{Kind: "scroll"},
		{Kind: "ring", Modifier: 3, HasModifier: true},
		{Kind: "rod", Modifier: 1, HasModifier: true},
		{Kind: "staff", Modifier: 2, HasModifier: true},
		{Kind: "wand", Modifier: 3, HasModifier: true},
		{Kind: "misc", Modifier: 3, HasModifier: true},
	})
	require.NoError(t, err)
	return tbl
}

// ADR 0035 item 6.2: using a magic item corrects the round's initiative by
// its magic kind's Table 56 modifier (a scroll's row has none: 0), after the
// use's entry, the transaction's last. Any other piece corrects nothing, nor
// does a use before catalogue format 7 (no Table 56).
func TestUseItem_ADR0035Item6_2_AMagicItemUsedCorrectsTheInitiative(t *testing.T) {
	tests := []struct {
		name  string
		item  itemsvc.Item
		table bool
		want  []buffsvc.InitiativeModifier
	}{
		{
			"a wand: +3",
			itemsvc.Item{ID: "w", Name: localized.Of("Wand of frost"), Category: itemsvc.CategoryMagic, MagicKind: "wand", Behaviour: itemsvc.BehaviourCharges, Count: 4},
			true,
			[]buffsvc.InitiativeModifier{{Source: buffsvc.InitiativeItem, Value: 3, ItemID: "w", Name: "Wand of frost", MagicKind: "wand"}},
		},
		{
			"a potion: +4",
			itemsvc.Item{ID: "p", Name: localized.Of("Potion"), Category: itemsvc.CategoryMagic, MagicKind: "potion", Behaviour: itemsvc.BehaviourStock, Count: 1},
			true,
			[]buffsvc.InitiativeModifier{{Source: buffsvc.InitiativeItem, Value: 4, ItemID: "p", Name: "Potion", MagicKind: "potion"}},
		},
		{
			"a scroll: 0",
			itemsvc.Item{ID: "s", Name: localized.Of("Scroll"), Category: itemsvc.CategoryMagic, MagicKind: "scroll", Behaviour: itemsvc.BehaviourStock, Count: 1},
			true,
			[]buffsvc.InitiativeModifier{{Source: buffsvc.InitiativeItem, Value: 0, ItemID: "s", Name: "Scroll", MagicKind: "scroll"}},
		},
		{
			"gear: nothing",
			itemsvc.Item{ID: "f", Name: localized.Of("Flask"), Category: itemsvc.CategoryGear, Behaviour: itemsvc.BehaviourStock, Count: 2},
			true, nil,
		},
		{
			"no Table 56: nothing",
			itemsvc.Item{ID: "w", Name: localized.Of("Wand"), Category: itemsvc.CategoryMagic, MagicKind: "wand", Behaviour: itemsvc.BehaviourCharges, Count: 4},
			false, nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t)
			f := deps(t)
			if tt.table {
				f.InitiativeItems = table56(t)
			}
			svc, err := itemsvc.New(dal, f.Deps)
			require.NoError(t, err)
			dal.EXPECT().Decrement(mock.Anything, int64(1), tt.item.ID).Return(tt.item, nil)
			recorded := 0
			f.log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
				func(context.Context, actionlog.NewEntry) (int64, error) { recorded++; return 9, nil })
			var got []buffsvc.InitiativeModifier
			f.initiative.EXPECT().CorrectInitiative(mock.Anything, int64(1), mock.Anything).RunAndReturn(
				func(_ context.Context, _ int64, cause buffsvc.InitiativeModifier) (*buffsvc.Initiative, error) {
					assert.Equal(t, 1, recorded, "after the use's entry")
					got = append(got, cause)
					return nil, nil
				}).Maybe()

			_, err = svc.UseItem(context.Background(), 1, tt.item.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
