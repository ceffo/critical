package itemsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

func legacy(cat itemsvc.Category, kind string, b itemsvc.Behaviour, count, maxCount int, dice string) itemsvc.Item {
	return itemsvc.Item{
		ID: "x", Name: localized.Of("X"), State: itemsvc.StateCarried, Category: cat, MagicKind: kind, Behaviour: b, Count: count, MaxCount: maxCount,
		EffectDice: dice, EffectSave: map[bool]string{true: "rod_staff_wand"}[dice != ""],
	}
}

func use(l itemsvc.Limit, dice string) itemsvc.Ability {
	a := itemsvc.Ability{ID: "use", Limit: l, Effects: nil}
	if dice != "" {
		a.Effects = []itemsvc.Effect{{Kind: itemsvc.EffectDice, Dice: dice, Save: "rod_staff_wand"}}
	}
	return a
}

// ADR 0049 step 19: one test row per row of the table, quantity, abilities and review reasons.
func TestConvertLegacy_ADR0049Step19_Table(t *testing.T) {
	const (
		stock   = itemsvc.BehaviourStock
		charges = itemsvc.BehaviourCharges
		daily   = itemsvc.BehaviourDaily
	)
	tests := []struct {
		name      string
		in        itemsvc.Item
		quantity  int
		abilities []itemsvc.Ability
		reasons   []string
	}{
		{"stock, no dice (arrows, rope, torches)", legacy(itemsvc.CategoryGear, "", stock, 12, 12, ""), 12, []itemsvc.Ability{}, nil},
		{
			"stock with dice (fire flasks 5d6, 5)", legacy(itemsvc.CategoryGear, "", stock, 5, 5, "5d6"), 5,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitWithItem}, "5d6")},
			nil,
		},
		{
			"stock potion, no dice", legacy(itemsvc.CategoryMagic, itemsvc.MagicPotion, stock, 3, 3, ""), 3,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitWithItem}, "")},
			nil,
		},
		{
			"stock scroll, no dice", legacy(itemsvc.CategoryMagic, itemsvc.MagicScroll, stock, 2, 2, ""), 2,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitWithItem}, "")},
			nil,
		},
		{
			"charges with dice (wand of lightning, 4)", legacy(itemsvc.CategoryMagic, itemsvc.MagicWand, charges, 4, 4, "6d6"), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 4, Left: 4, Cost: 1}, "6d6")},
			[]string{itemsvc.ReasonChargesTotal},
		},
		{
			"charges, no dice, a potion of one dose", legacy(itemsvc.CategoryMagic, itemsvc.MagicPotion, charges, 1, 1, ""), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitWithItem}, "")},
			nil,
		},
		{
			"charges, no dice, a scroll of 2", legacy(itemsvc.CategoryMagic, itemsvc.MagicScroll, charges, 2, 2, ""), 2,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitWithItem}, "")},
			nil,
		},
		{
			"charges, no dice, a wand without a rule", legacy(itemsvc.CategoryMagic, itemsvc.MagicWand, charges, 7, 7, ""), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 7, Left: 7, Cost: 1}, "")},
			[]string{itemsvc.ReasonChargesTotal},
		},
		{
			"daily (power gem's dispel, used today)", legacy(itemsvc.CategoryMagic, itemsvc.MagicMisc, daily, 0, 1, ""), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 0}, "")},
			nil,
		},
		{
			"daily 3 a day with dice, 2 left", legacy(itemsvc.CategoryMagic, itemsvc.MagicRod, daily, 2, 3, "1d8"), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 3, Left: 2}, "1d8")},
			nil,
		},
		{"weapon, plain stock", legacy(itemsvc.CategoryWeapon, "", stock, 1, 1, ""), 1, []itemsvc.Ability{}, nil},
		{
			"weapon that was charges", legacy(itemsvc.CategoryWeapon, "", charges, 3, 3, ""), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 3, Left: 3, Cost: 1}, "")},
			[]string{itemsvc.ReasonChargesTotal, itemsvc.ReasonWeaponWithUse},
		},
		{
			"armour that was daily", legacy(itemsvc.CategoryArmour, "", daily, 1, 1, ""), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 1}, "")},
			[]string{itemsvc.ReasonWeaponWithUse},
		},
		{
			"shield that was stock with dice", legacy(itemsvc.CategoryShield, "", stock, 1, 1, "1d6"), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitUnlimited}, "1d6")},
			[]string{itemsvc.ReasonWeaponWithUse},
		},
		{"weapon stored with a count of 2: kept, shown", legacy(itemsvc.CategoryWeapon, "", stock, 2, 2, ""), 2, []itemsvc.Ability{}, nil},
		{
			"a ring stored with 3: kept, reviewed", legacy(itemsvc.CategoryMagic, itemsvc.MagicRing, stock, 3, 3, ""), 3,
			[]itemsvc.Ability{},
			[]string{itemsvc.ReasonQuantityAboveOne},
		},
		{
			"charges above the ceiling: clamped, reviewed", legacy(itemsvc.CategoryMagic, itemsvc.MagicWand, charges, 150, 150, ""), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: itemsvc.MaxCharges, Left: itemsvc.MaxCharges, Cost: 1}, "")},
			[]string{itemsvc.ReasonChargesTotal, itemsvc.ReasonLimitClamped},
		},
		{
			"charges, none left", legacy(itemsvc.CategoryMagic, itemsvc.MagicWand, charges, 0, 0, ""), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 1, Left: 0, Cost: 1}, "")},
			[]string{itemsvc.ReasonChargesTotal},
		},
		{
			"daily above the ceiling: clamped, reviewed", legacy(itemsvc.CategoryMagic, itemsvc.MagicMisc, daily, 120, 150, ""), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: itemsvc.MaxPerDay, Left: itemsvc.MaxPerDay}, "")},
			[]string{itemsvc.ReasonLimitClamped},
		},
		{
			"daily with a count above its max: reviewed", legacy(itemsvc.CategoryMagic, itemsvc.MagicMisc, daily, 5, 2, ""), 1,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 2, Left: 2}, "")},
			[]string{itemsvc.ReasonLimitClamped},
		},
		{
			"potion with charges above the ceiling: doses are the quantity", legacy(itemsvc.CategoryMagic, itemsvc.MagicPotion, charges, 150, 150, ""), 150,
			[]itemsvc.Ability{use(itemsvc.Limit{Kind: itemsvc.LimitWithItem}, "")},
			nil,
		},
		{
			"ammunition that was daily: as stock, reviewed", legacy(itemsvc.CategoryAmmunition, "", daily, 20, 20, ""), 20,
			[]itemsvc.Ability{},
			[]string{itemsvc.ReasonAmmoNotStock},
		},
		{
			"ammunition that was charges: as stock, reviewed", legacy(itemsvc.CategoryAmmunition, "", charges, 5, 5, ""), 5,
			[]itemsvc.Ability{},
			[]string{itemsvc.ReasonAmmoNotStock},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := itemsvc.ConvertLegacy(tt.in)
			assert.Equal(t, tt.quantity, got.Quantity, "quantity")
			assert.Equal(t, tt.abilities, got.Abilities, "abilities")
			assert.Equal(t, tt.reasons, got.Reasons, "reasons")
			// What the conversion writes must be what the editor accepts, for every row.
			assert.NoError(t, got.Check(tt.in), "Check")
			renamed := tt.in
			renamed.Abilities = got.Abilities
			renamed.Count = tt.in.Count // the stored count keeps its old meaning
			renamed.Name = localized.Of("Renamed")
			if (tt.in.Category == itemsvc.CategoryGear || tt.in.Category == itemsvc.CategoryMagic) && (len(got.Abilities) == 0 || !got.Abilities[0].Limit.PaysPerObject()) {
				assert.NoError(t, renamed.Validate(), "renamed item validates")
			}
			if !tt.in.Category.OneObject() && tt.in.Category != itemsvc.CategoryAmmunition {
				piece := tt.in
				piece.Abilities, piece.Count = got.Abilities, got.Quantity
				assert.NoError(t, piece.Validate())
			}
		})
	}
}

// ADR 0049 step 19 (slice 2): the seed in the new shape stores the old columns the previous release
// still reads; MirrorLegacy is the inverse of ConvertLegacy for the seed's items.
func TestMirrorLegacy_ADR0049Step23_InvertsTheConversion(t *testing.T) {
	for _, in := range []itemsvc.Item{
		legacy(itemsvc.CategoryGear, "", itemsvc.BehaviourStock, 5, 5, "5d6"),
		legacy(itemsvc.CategoryMagic, itemsvc.MagicWand, itemsvc.BehaviourCharges, 4, 4, "6d6"),
		legacy(itemsvc.CategoryMagic, itemsvc.MagicMisc, itemsvc.BehaviourDaily, 1, 1, ""),
		legacy(itemsvc.CategoryAmmunition, "", itemsvc.BehaviourStock, 18, 18, ""),
	} {
		c := itemsvc.ConvertLegacy(in)
		got := itemsvc.MirrorLegacy(c.Quantity, c.Abilities)
		want := itemsvc.LegacyFields{Behaviour: in.Behaviour, Count: in.Count, MaxCount: in.MaxCount, EffectDice: in.EffectDice, EffectSave: in.EffectSave}
		assert.Equal(t, want, got, in.MagicKind+string(in.Behaviour))
	}
}

// ADR 0049 step 19: the Power Gem's name comes from data (the abilities of the seed row), never from an id.
func TestNamedFrom_ADR0049Step19_TakesNamesOfTheSameAbilityId(t *testing.T) {
	got := itemsvc.NamedFrom(
		[]itemsvc.Ability{{ID: "use"}, {ID: "other"}, {ID: "kept", Name: localized.Of("Mine")}},
		[]itemsvc.Ability{{ID: "use", Name: localized.Text{"en": "Dispel Magic"}}, {ID: "kept", Name: localized.Of("Seed")}})
	assert.Equal(t, localized.Text{"en": "Dispel Magic"}, got[0].Name)
	assert.Empty(t, got[1].Name)
	assert.Equal(t, localized.Of("Mine"), got[2].Name)
}

const oldGemUsed = `{"count":0,"replace":true,"item":{"position":5,"name":{"en":"Power Gem"},"type":"gem","behaviour":"daily","magicalBonus":0,"ammoKind":"","maxCount":1,"effectDice":"","effectSave":"","notes":"n","powers":[],"piece":{"category":"magic","magicKind":"misc","state":"carried"}}}`

// ADR 0049 step 20: popping an entry recorded before abilities runs the conversion on its old
// fields: the gem used today comes back as a per_day ability at left 0, named as the stored item's
// ability of the same id (the boot conversion's name survives the pop).
func TestRestore_ADR0049Step20_OldSnapshotIsConvertedAndKeepsTheStoredName(t *testing.T) {
	cur := itemsvc.Item{ID: "gem", Abilities: []itemsvc.Ability{{ID: "use", Name: localized.Text{"en": "Dispel Magic"}}}}
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "gem").Return(cur, nil)
	var got itemsvc.Item
	dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { got = it }).Return(nil).Once()
	require.NoError(t, svc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, snaptest.EnvID("gem", oldGemUsed)))
	require.Len(t, got.Abilities, 1)
	assert.Equal(t, itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 0}, got.Abilities[0].Limit)
	assert.Equal(t, localized.Text{"en": "Dispel Magic"}, got.Abilities[0].Name)
	assert.Equal(t, 0, got.Count, "the stored count keeps its old meaning in this slice")
}

// ADR 0049 step 20: popping the removal of a charges item brings it back as a charges ability.
func TestRestore_ADR0049Step20_RemovedChargesItemComesBackAsCharges(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	snap, err := itemsvc.UseSnapshot(wand) // the old use spent the last charge; "wand" has no abilities key written
	require.NoError(t, err)
	dal.EXPECT().SetCount(mock.Anything, int64(1), "wand_lightning", 1).Return(itemsvc.ErrItemNotFound).Once()
	dal.EXPECT().Get(mock.Anything, int64(1), "wand_lightning").Return(itemsvc.Item{}, itemsvc.ErrItemNotFound)
	var got itemsvc.Item
	dal.EXPECT().Insert(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { got = it }).Return(nil).Once()
	require.NoError(t, svc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, snap.Before))
	require.Len(t, got.Abilities, 1)
	assert.Equal(t, itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 1, Left: 1, Cost: 1}, got.Abilities[0].Limit)
}

// ADR 0049 step 20: a snapshot that has the abilities key is written back as it was (no second conversion).
func TestRestore_ADR0049Step20_SnapshotWithAbilitiesIsNotConvertedAgain(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	state := `{"count":0,"replace":true,"item":{"position":5,"name":{"en":"Gem"},"type":"gem","behaviour":"daily","magicalBonus":0,"ammoKind":"","maxCount":1,"effectDice":"","effectSave":"","notes":"","powers":[],"abilities":[],"piece":{"category":"magic","magicKind":"misc","state":"carried"}}}`
	var got itemsvc.Item
	dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { got = it }).Return(nil).Once()
	require.NoError(t, svc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, snaptest.EnvID("gem", state)))
	assert.Equal(t, []itemsvc.Ability{}, got.Abilities)
}
