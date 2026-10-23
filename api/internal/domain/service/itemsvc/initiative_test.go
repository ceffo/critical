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
	buffmocks "github.com/ceffo/critical/api/internal/domain/service/buffsvc/mocks"
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
// its magic kind's Table 56 modifier, after the
// use's entry, the transaction's last. Any other piece corrects nothing, nor
// does a use before catalogue format 7 (no Table 56). A scroll's row is the
// casting time of its spell: a scroll that names none is not counted
// (timing spell_casting_time), never a computed +0.
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
			"a scroll: the casting time of its spell, not counted",
			itemsvc.Item{ID: "s", Name: localized.Of("Scroll"), Category: itemsvc.CategoryMagic, MagicKind: "scroll", Behaviour: itemsvc.BehaviourStock, Count: 1},
			true,
			[]buffsvc.InitiativeModifier{{Source: buffsvc.InitiativeItem, Value: 0, ItemID: "s", Name: "Scroll", MagicKind: "scroll", Timing: buffsvc.TimingSpellCastingTime}},
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

// The items service is built with a real corrector over the initiative store
// and the log only: no buffs service exists in this test (the other side of
// the former items/buffs cycle). Using a wand corrects the round's initiative
// inside the use, as its last entry (ADR 0035 item 6.2).
func TestUseItem_ADR0035Item6_2_CorrectsThroughTheCorrectorWithoutTheBuffs(t *testing.T) {
	ctx := context.Background()
	store := buffmocks.NewMockDAL(t)
	f := deps(t)
	corrector, err := buffsvc.NewInitiativeCorrector(store, f.log)
	require.NoError(t, err)
	f.Initiative = corrector
	f.InitiativeItems = table56(t)
	dal := mocks.NewMockDAL(t)
	svc, err := itemsvc.New(dal, f.Deps)
	require.NoError(t, err)
	wand := itemsvc.Item{ID: "w", Name: localized.Of("Wand of frost"), Category: itemsvc.CategoryMagic, MagicKind: "wand", Behaviour: itemsvc.BehaviourCharges, Count: 4}
	dal.EXPECT().Decrement(mock.Anything, int64(1), "w").Return(wand, nil)

	store.EXPECT().GetRoundNumber(mock.Anything, int64(1)).Return(3, nil).Once()
	store.EXPECT().GetInitiative(mock.Anything, int64(1)).Return(&buffsvc.Initiative{D10: 5, LogEntryID: 10}, nil).Once()
	var kinds []actionlog.Kind
	f.log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, e actionlog.NewEntry) (int64, error) {
			kinds = append(kinds, e.Kind)
			return int64(10 + len(kinds)), nil
		})
	cause := buffsvc.ItemInitiative("w", "Wand of frost", "wand", 3, true)
	store.EXPECT().SaveInitiative(mock.Anything, int64(1), &buffsvc.Initiative{D10: 5, LogEntryID: 12, Modifiers: []buffsvc.InitiativeModifier{cause}}).Return(nil).Once()

	_, err = svc.UseItem(ctx, 1, "w")
	require.NoError(t, err)
	require.Len(t, kinds, 2)
	assert.Equal(t, actionlog.KindInitiativeCorrect, kinds[1], "the correction is the use's last entry")
}
