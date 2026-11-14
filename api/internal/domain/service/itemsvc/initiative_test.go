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

// withUse is the item holding one ability "use" under limit, with the counter that limit asks for.
func withUse(it itemsvc.Item, l itemsvc.Limit) itemsvc.Item {
	it.Abilities = []itemsvc.Ability{{ID: "use", Limit: l}}
	return it
}

// expectUse is the reads and writes UseAbility makes on the DAL for the item as it is before the
// use: the read, then the write of the limit it pays (the whole item for a counter, one atomic
// decrement for a quantity, the delete of a potion or a scroll at 0).
func expectUse(dal *mocks.MockDAL, before itemsvc.Item) {
	dal.EXPECT().Get(mock.Anything, int64(1), before.ID).Return(before, nil)
	switch before.Abilities[0].Limit.Kind {
	case itemsvc.LimitPerDay, itemsvc.LimitCharges:
		dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil).Once()
	case itemsvc.LimitWithItem:
		after := before
		after.Count--
		dal.EXPECT().Decrement(mock.Anything, int64(1), before.ID).Return(after, nil).Once()
		if after.Removed() {
			dal.EXPECT().Delete(mock.Anything, int64(1), before.ID).Return(nil).Once()
		}
	}
}

// ADR 0049 step 15 (ADR 0035 item 6.2): using an ability in the round corrects the initiative by the
// item's magic kind (Table 56), a scroll by the casting time of its spell, and an item of any other
// category (a sword's "turn zombies", a rope) by the miscellaneous row, +3; after the use's entry,
// the transaction's last. Nothing is corrected before catalogue format 7 (no Table 56).
func TestUseAbility_ADR0049Step15_InitiativeByKindScrollAndMisc(t *testing.T) {
	charges := itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 4, Left: 4, Cost: 1}
	tests := []struct {
		name  string
		item  itemsvc.Item
		limit itemsvc.Limit
		table bool
		want  []buffsvc.InitiativeModifier
	}{
		{
			"a wand: +3",
			itemsvc.Item{ID: "w", Name: localized.Of("Wand of frost"), Category: itemsvc.CategoryMagic, MagicKind: "wand", Count: 1},
			charges, true,
			[]buffsvc.InitiativeModifier{{Source: buffsvc.InitiativeItem, Value: 3, ItemID: "w", Name: "Wand of frost", MagicKind: "wand"}},
		},
		{
			"a rod: +1",
			itemsvc.Item{ID: "r", Name: localized.Of("Rod"), Category: itemsvc.CategoryMagic, MagicKind: "rod", Count: 1},
			itemsvc.Limit{Kind: itemsvc.LimitUnlimited},
			true,
			[]buffsvc.InitiativeModifier{{Source: buffsvc.InitiativeItem, Value: 1, ItemID: "r", Name: "Rod", MagicKind: "rod"}},
		},
		{
			"a potion: +4",
			itemsvc.Item{ID: "p", Name: localized.Of("Potion"), Category: itemsvc.CategoryMagic, MagicKind: "potion", Count: 1},
			itemsvc.Limit{Kind: itemsvc.LimitWithItem},
			true,
			[]buffsvc.InitiativeModifier{{Source: buffsvc.InitiativeItem, Value: 4, ItemID: "p", Name: "Potion", MagicKind: "potion"}},
		},
		{
			"a scroll: the casting time of its spell, not counted",
			itemsvc.Item{ID: "s", Name: localized.Of("Scroll"), Category: itemsvc.CategoryMagic, MagicKind: "scroll", Count: 1},
			itemsvc.Limit{Kind: itemsvc.LimitWithItem},
			true,
			[]buffsvc.InitiativeModifier{{Source: buffsvc.InitiativeItem, Value: 0, ItemID: "s", Name: "Scroll", MagicKind: "scroll", Timing: buffsvc.TimingSpellCastingTime}},
		},
		{
			"a sword with an ability: the misc row, +3",
			itemsvc.Item{ID: "sw", Name: localized.Of("Sword"), Category: itemsvc.CategoryWeapon, Count: 1},
			itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 2, Left: 2},
			true,
			[]buffsvc.InitiativeModifier{{Source: buffsvc.InitiativeItem, Value: 3, ItemID: "sw", Name: "Sword", MagicKind: "misc"}},
		},
		{
			"a rope: the misc row, +3",
			itemsvc.Item{ID: "ro", Name: localized.Of("Rope"), Category: itemsvc.CategoryGear, Count: 1},
			itemsvc.Limit{Kind: itemsvc.LimitUnlimited},
			true,
			[]buffsvc.InitiativeModifier{{Source: buffsvc.InitiativeItem, Value: 3, ItemID: "ro", Name: "Rope", MagicKind: "misc"}},
		},
		{
			"no Table 56: nothing",
			itemsvc.Item{ID: "w", Name: localized.Of("Wand"), Category: itemsvc.CategoryMagic, MagicKind: "wand", Count: 1},
			charges, false, nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t)
			f := readsCharacter(t) // a weapon read derives its Specialized
			if tt.table {
				f.InitiativeItems = table56(t)
			}
			svc, err := itemsvc.New(dal, f.Deps)
			require.NoError(t, err)
			expectUse(dal, withUse(tt.item, tt.limit))
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

			_, err = svc.UseAbility(context.Background(), 1, tt.item.ID, "use", itemsvc.UseAsks{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// The items service is built with a real corrector over the initiative store and the log only: no
// buffs service exists in this test (the other side of the former items/buffs cycle). Using a
// wand's ability corrects the round's initiative inside the use, as its last entry (ADR 0049 step 15).
func TestUseAbility_ADR0049Step15_CorrectsThroughTheCorrectorWithoutTheBuffs(t *testing.T) {
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
	wand := withUse(itemsvc.Item{ID: "w", Name: localized.Of("Wand of frost"), Category: itemsvc.CategoryMagic, MagicKind: "wand", Count: 1},
		itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 4, Left: 4, Cost: 1})
	expectUse(dal, wand)

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

	_, err = svc.UseAbility(ctx, 1, "w", "use", itemsvc.UseAsks{})
	require.NoError(t, err)
	require.Len(t, kinds, 2)
	assert.Equal(t, actionlog.KindInitiativeCorrect, kinds[1], "the correction is the use's last entry")
}
