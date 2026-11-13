package itemsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

// rope is a piece of gear in a stack of 3: the base the ability tests put abilities on.
var rope = itemsvc.Item{
	ID: "rope", Position: 1, Name: localized.Of("Rope"), Category: itemsvc.CategoryGear, State: itemsvc.StateCarried,
	Behaviour: itemsvc.BehaviourStock, Count: 3, MaxCount: 3,
}

// withAbility is p, a piece of the test, carrying one ability.
func withAbility(p itemsvc.Item, a itemsvc.Ability) itemsvc.Item {
	p.Abilities = []itemsvc.Ability{a}
	return p
}

func unlimited(id string) itemsvc.Ability {
	return itemsvc.Ability{ID: id, Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}}
}

// ADR 0049 steps 4, 5 and 10: each rule of an ability's validation refuses with the field
// "abilities", before anything is stored; the valid shapes pass.
func TestItem_ADR0049Step4_5_10_AbilityValidation(t *testing.T) {
	wand := itemsvc.Item{
		ID: "wand", Name: localized.Of("Wand"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicWand,
		State: itemsvc.StateCarried, Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1,
	}
	potion := itemsvc.Item{
		ID: "potion", Name: localized.Of("Potion"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicPotion,
		State: itemsvc.StateCarried, Behaviour: itemsvc.BehaviourStock, Count: 2, MaxCount: 2,
	}
	perDay := func(max, left int) itemsvc.Ability {
		return itemsvc.Ability{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: max, Left: left}}
	}
	charges := func(max, left, cost int) itemsvc.Ability {
		return itemsvc.Ability{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: max, Left: left, Cost: cost}}
	}
	dice := func(e itemsvc.Effect) itemsvc.Ability {
		a := unlimited("a")
		a.Effects = []itemsvc.Effect{e}
		return a
	}
	two := func(a, b string) itemsvc.Item {
		p := rope
		p.Abilities = []itemsvc.Ability{unlimited(a), unlimited(b)}
		return p
	}

	tooMany := func() itemsvc.Item {
		p := rope
		for n := 0; n <= itemsvc.MaxAbilities; n++ {
			p.Abilities = append(p.Abilities, unlimited("a"+strings.Repeat("b", n)))
		}
		return p
	}
	manyEffects := func() itemsvc.Ability {
		a := unlimited("a")
		for n := 0; n <= itemsvc.MaxAbilityEffects; n++ {
			a.Effects = append(a.Effects, itemsvc.Effect{Kind: itemsvc.EffectDice, Dice: "1d6"})
		}
		return a
	}
	spent := wand
	spent.Count = 0

	refused := map[string]itemsvc.Item{
		// Step 4.
		"an id that is no slug":             withAbility(rope, unlimited("Turn Zombies")),
		"an empty id":                       withAbility(rope, unlimited("")),
		"a duplicate id":                    two("climb", "climb"),
		"a name in an unknown language":     withAbility(rope, itemsvc.Ability{ID: "a", Name: localized.Text{"xx": "Climb"}, Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}}),
		"a note too long":                   withAbility(rope, itemsvc.Ability{ID: "a", Note: strings.Repeat("x", itemsvc.MaxAbilityNote+1), Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}}),
		"too many abilities":                tooMany(),
		"too many effects":                  withAbility(rope, manyEffects()),
		"a name too long":                   withAbility(rope, itemsvc.Ability{ID: "a", Name: localized.Text{"en": strings.Repeat("x", itemsvc.MaxAbilityName+1)}, Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}}),
		"a label too long":                  withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectDice, Dice: "1d6", Label: strings.Repeat("x", itemsvc.MaxEffectLabel+1)})),
		"with_item on a wand (no quantity)": withAbility(wand, itemsvc.Ability{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}}),
		// Step 5.
		"an unknown limit":        withAbility(rope, itemsvc.Ability{ID: "a", Limit: itemsvc.Limit{Kind: "hourly"}}),
		"no limit":                withAbility(rope, itemsvc.Ability{ID: "a"}),
		"an unlimited with a max": withAbility(rope, itemsvc.Ability{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited, Max: 1}}),
		"a with_item with a left": withAbility(potion, itemsvc.Ability{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem, Left: 1}}),
		"per_day max 0":           withAbility(wand, perDay(0, 0)),
		"per_day max 100":         withAbility(wand, perDay(100, 1)),
		"per_day left above max":  withAbility(wand, perDay(2, 3)),
		"per_day left below 0":    withAbility(wand, perDay(2, -1)),
		"per_day with a cost":     withAbility(wand, itemsvc.Ability{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 2, Left: 2, Cost: 1}}),
		"charges max 0":           withAbility(wand, charges(0, 0, 1)),
		"charges max 101":         withAbility(wand, charges(101, 5, 1)),
		"charges left above max":  withAbility(wand, charges(5, 6, 1)),
		"charges cost 0":          withAbility(wand, charges(5, 5, 0)),
		"charges cost above max":  withAbility(wand, charges(5, 5, 6)),
		// Step 10.
		"an unknown effect kind":          withAbility(rope, dice(itemsvc.Effect{Kind: "buff", Dice: "1d4"})),
		"a dice effect that is full":      withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectDice, Dice: "1d4", Full: true})),
		"a heal with dice and full":       withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectHeal, Dice: "1d4", Full: true})),
		"a heal with neither":             withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectHeal})),
		"a heal with bad dice":            withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectHeal, Dice: "lots"})),
		"a heal with a save":              withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectHeal, Dice: "1d4", Save: "rod_staff_wand"})),
		"a dice effect without dice":      withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectDice})),
		"dice that are no dice":           withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectDice, Dice: "lots"})),
		"dice with 0 sides":               withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectDice, Dice: "2d0"})),
		"a save category that is unknown": withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectDice, Dice: "2d6", Save: "dodge"})),
	}
	for name, in := range refused {
		t.Run("refuses "+name, func(t *testing.T) {
			err := in.Validate()
			var fe *domain.FieldError
			require.True(t, errors.As(err, &fe), "%v", err)
			assert.Equal(t, "abilities", fe.Field)
		})
	}

	accepted := map[string]itemsvc.Item{
		"none (not converted)":     rope,
		"an empty list":            {ID: "r", Name: localized.Of("Rope"), Category: itemsvc.CategoryGear, State: itemsvc.StateCarried, Behaviour: itemsvc.BehaviourStock, Count: 3, MaxCount: 3, Abilities: []itemsvc.Ability{}},
		"an unlimited, no effect":  withAbility(rope, unlimited("climb")),
		"per_day on a one object":  withAbility(wand, perDay(1, 0)),
		"per_day on a spent piece": withAbility(spent, perDay(1, 0)),
		"charges on a spent piece": withAbility(spent, charges(5, 0, 1)),
		"the most abilities": func() itemsvc.Item {
			p := rope
			for n := 0; n < itemsvc.MaxAbilities; n++ {
				p.Abilities = append(p.Abilities, unlimited("a"+strings.Repeat("b", n)))
			}
			return p
		}(),
		"per_day 99":                withAbility(wand, perDay(99, 99)),
		"charges at their bounds":   withAbility(wand, charges(100, 0, 100)),
		"with_item on a potion":     withAbility(potion, itemsvc.Ability{ID: "drink", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}}),
		"with_item on a stack":      withAbility(rope, itemsvc.Ability{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}}),
		"a dice effect with a save": withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectDice, Dice: "6d6", Save: "rod_staff_wand", Label: "fire"})),
		"dice with a modifier":      withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectDice, Dice: "1d8+2"})),
		"a heal of dice":            withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectHeal, Dice: "2d4+2", Label: "drink"})),
		"a full heal":               withAbility(rope, dice(itemsvc.Effect{Kind: itemsvc.EffectHeal, Full: true})),
	}
	for name, in := range accepted {
		t.Run("accepts "+name, func(t *testing.T) {
			require.NoError(t, in.Validate())
		})
	}
}

// ADR 0049 steps 4 and 5: an ability sent without an id gets the first free ability_<n>; a
// charges limit sent without a cost costs 1 (the default); the typed text is trimmed.
func TestCreate_ADR0049Step4_5_AbilityDefaults(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	dal.EXPECT().Insert(mock.Anything, int64(1), mock.Anything).Return(nil)
	dal.EXPECT().DeleteReviews(mock.Anything, int64(1), mock.Anything).Return(nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)

	res, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{
		Name: localized.Of("Wand of sparks"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicWand,
		Behaviour: itemsvc.BehaviourStock, Count: 1,
		Abilities: []itemsvc.Ability{
			{Note: "  zap  ", Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 10, Left: 10}},
			{ID: "ability_1", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}},
			{Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 1}},
		},
	})
	require.NoError(t, err)
	ids := []string{}
	for _, a := range res.Item.Abilities {
		ids = append(ids, a.ID)
	}
	assert.Equal(t, []string{"ability_2", "ability_1", "ability_3"}, ids)
	assert.Equal(t, 1, res.Item.Abilities[0].Limit.Cost)
	assert.Equal(t, "zap", res.Item.Abilities[0].Note)
}

// ADR 0049 step 4: an edit that carries a bad ability is refused before anything is read back
// or written (field abilities).
func TestUpdate_ADR0049Step4_RefusesABadAbilityBeforeWriting(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "rope").Return(rope, nil)
	bad := []itemsvc.Ability{{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 1}}}
	_, err := svc.Update(context.Background(), 1, "rope", itemsvc.ItemPatch{Abilities: &bad})
	var fe *domain.FieldError
	require.True(t, errors.As(err, &fe), "%v", err)
	assert.Equal(t, "abilities", fe.Field)
}

// ADR 0049 step 4: an item_edit entry carries the abilities, as a change (the stored JSON
// before and after) and in the payload, and its snapshot holds them, so popping it restores
// them; an item not converted yet (NULL) is restored as not converted.
func TestUpdate_ADR0049Step4_EditLogsAndSnapshotsAbilitiesAndUndoRestoresThem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "rope").Return(rope, nil).Once()
	climb := itemsvc.Ability{
		ID: "climb", Name: localized.Of("Climb"), Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited},
		Effects: []itemsvc.Effect{{Kind: itemsvc.EffectDice, Dice: "1d6", Save: "rod_staff_wand"}},
	}
	want := rope
	want.Abilities = []itemsvc.Ability{climb}
	dal.EXPECT().Update(mock.Anything, int64(1), want).Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(7), nil).Once()

	patch := []itemsvc.Ability{climb}
	res, err := svc.Update(context.Background(), 1, "rope", itemsvc.ItemPatch{Abilities: &patch})
	require.NoError(t, err)
	assert.Equal(t, want.Abilities, res.Item.Abilities)
	assert.Equal(t, actionlog.KindItemEdit, got.Kind)

	const stored = `[{"id":"climb","name":{"en":"Climb"},"limit":{"kind":"unlimited"},"effects":[{"kind":"dice","dice":"1d6","save":"rod_staff_wand"}]}]`
	var rec struct {
		Changes   []itemsvc.FieldChange `json:"changes"`
		Abilities json.RawMessage       `json:"abilities"`
	}
	require.NoError(t, json.Unmarshal(got.Payload, &rec))
	assert.Equal(t, []itemsvc.FieldChange{{Field: "abilities", Before: "", After: stored}}, rec.Changes)
	assert.JSONEq(t, stored, string(rec.Abilities))

	// Popping the edit writes the item back as it was: not converted.
	restore := mocks.NewMockDAL(t)
	rsvc, _ := itemsvc.New(restore, deps(t).Deps)
	restore.EXPECT().Get(mock.Anything, int64(1), "rope").Return(want, nil).Maybe()
	restore.EXPECT().Update(mock.Anything, int64(1), rope).Return(nil).Once()
	require.NoError(t, rsvc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, got.Snapshots[0].Before))

	// Popping it forward (the "after" state) gives the abilities back.
	redo := mocks.NewMockDAL(t)
	vsvc, _ := itemsvc.New(redo, deps(t).Deps)
	redo.EXPECT().Get(mock.Anything, int64(1), "rope").Return(rope, nil).Maybe()
	redo.EXPECT().Update(mock.Anything, int64(1), want).Return(nil).Once()
	require.NoError(t, vsvc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, got.Snapshots[0].After))
}

// ADR 0049 step 4: a snapshot taken before abilities existed has no abilities key; popping it
// writes the item back not converted (NULL), as the unconverted item it recorded.
func TestRestore_ADR0049Step4_SnapshotWithoutAbilitiesRestoresNotConverted(t *testing.T) {
	cur := withAbility(rope, unlimited("climb"))
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "rope").Return(cur, nil).Maybe()
	dal.EXPECT().Update(mock.Anything, int64(1), rope).Return(nil).Once()
	old := snaptest.EnvID("rope", `{"count":3,"replace":true,"item":{"position":1,"name":{"en":"Rope"},"type":"","behaviour":"stock","magicalBonus":0,"ammoKind":"","maxCount":3,"effectDice":"","effectSave":"","notes":"","powers":[],"piece":{"category":"gear","state":"carried"}}}`)
	require.NoError(t, svc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, old))
}

// ADR 0049 step 4: an edit of a one-object piece spent to 0 that carries a per_day ability is
// accepted (the stock "spend one" can take it there); the same piece at 2 is refused.
func TestUpdate_ADR0049Step4_SpentPieceWithPerDayStaysEditable(t *testing.T) {
	ring := itemsvc.Item{
		ID: "ring", Name: localized.Of("Ring"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicRing,
		State: itemsvc.StateCarried, Behaviour: itemsvc.BehaviourStock, Count: 0, MaxCount: 1,
		Abilities: []itemsvc.Ability{{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 0}}},
	}
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "ring").Return(ring, nil)
	dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil).Once()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil).Once()
	notes := "cursed"
	_, err := svc.Update(context.Background(), 1, "ring", itemsvc.ItemPatch{Notes: &notes})
	require.NoError(t, err)
}

// ADR 0049 step 18: the stored encoding tells "not converted" (NULL, nil) from "converted with
// none" ("[]", empty) and reads every field back; an unknown key is refused.
func TestAbilities_ADR0049Step18_StoredEncodingRoundTrips(t *testing.T) {
	text, ok, err := itemsvc.EncodeAbilities(nil)
	require.NoError(t, err)
	assert.False(t, ok, "nil is NULL")
	assert.Empty(t, text)
	got, err := itemsvc.DecodeAbilities("", false)
	require.NoError(t, err)
	assert.Nil(t, got)

	text, ok, err = itemsvc.EncodeAbilities([]itemsvc.Ability{})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "[]", text)
	got, err = itemsvc.DecodeAbilities(text, true)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)

	full := []itemsvc.Ability{{
		ID: "dispel", Name: localized.Text{"en": "Dispel", "fr": "Dissipation"}, Note: "once a day",
		Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 10, Left: 4, Cost: 2},
		Effects: []itemsvc.Effect{
			{Kind: itemsvc.EffectDice, Dice: "6d6", Save: "rod_staff_wand", Label: "fire"},
			{Kind: itemsvc.EffectHeal, Dice: "2d4+2"},
			{Kind: itemsvc.EffectHeal, Full: true},
		},
	}}
	text, _, err = itemsvc.EncodeAbilities(full)
	require.NoError(t, err)
	got, err = itemsvc.DecodeAbilities(text, true)
	require.NoError(t, err)
	assert.Equal(t, full, got)

	// A key a later slice wrote is ignored on read (a rollback must still read the row).
	got, err = itemsvc.DecodeAbilities(`[{"id":"a","limit":{"kind":"unlimited"},"effects":[],"heal":1}]`, true)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "a", got[0].ID)
	_, err = itemsvc.DecodeAbilities(`not json`, true)
	require.Error(t, err)
}

// ADR 0049 step 18: an item not converted (NULL) patched with [] becomes converted-with-none, and
// popping that edit restores NULL (nil), not an empty list; a patch without abilities (null on the
// wire) leaves the column as it is.
func TestUpdate_ADR0049Step18_NullToEmptyAndUndoBackToNull(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "rope").Return(rope, nil).Twice()
	empty := rope
	empty.Abilities = []itemsvc.Ability{}
	dal.EXPECT().Update(mock.Anything, int64(1), empty).Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(7), nil).Once()
	none := []itemsvc.Ability{}
	res, err := svc.Update(context.Background(), 1, "rope", itemsvc.ItemPatch{Abilities: &none})
	require.NoError(t, err)
	assert.NotNil(t, res.Item.Abilities)
	assert.Empty(t, res.Item.Abilities)

	restore := mocks.NewMockDAL(t)
	rsvc, _ := itemsvc.New(restore, deps(t).Deps)
	restore.EXPECT().Get(mock.Anything, int64(1), "rope").Return(empty, nil).Maybe()
	restore.EXPECT().Update(mock.Anything, int64(1), rope).Return(nil).Once()
	require.NoError(t, rsvc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, got.Snapshots[0].Before))

	// No abilities in the patch: the stored value (here NULL) is untouched.
	two := rope
	two.Count = 2
	dal.EXPECT().Update(mock.Anything, int64(1), two).Return(nil).Once()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(8), nil).Once()
	cnt := 2
	res, err = svc.Update(context.Background(), 1, "rope", itemsvc.ItemPatch{Count: &cnt})
	require.NoError(t, err)
	assert.Nil(t, res.Item.Abilities)
}

// ADR 0049 step 4 (slice 2 ruling): a converted wand keeps its old count of 4 (the boot
// conversion writes the abilities only), so an edit that carries no ability change is not
// refused for it; one that adds or changes an ability on a stack is.
func TestUpdate_ADR0049Step4_StoredCountAboveOneDoesNotBlockEditsButBlocksAbilityChanges(t *testing.T) {
	converted := withAbility(rope, itemsvc.Ability{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 4, Left: 4, Cost: 1}})
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "rope").Return(converted, nil)
	dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil).Once()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil).Once()
	notes := "frayed"
	_, err := svc.Update(context.Background(), 1, "rope", itemsvc.ItemPatch{Notes: &notes})
	require.NoError(t, err)

	// A count edit is a change of quantity: per-object abilities refuse more than one object.
	more := converted.Count + 46
	var fe *domain.FieldError
	_, err = svc.Update(context.Background(), 1, "rope", itemsvc.ItemPatch{Count: &more})
	require.True(t, errors.As(err, &fe), "a count edit on a converted per-object item is refused: %v", err)
	assert.Equal(t, "abilities", fe.Field)

	changed := []itemsvc.Ability{{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 1}}}
	_, err = svc.Update(context.Background(), 1, "rope", itemsvc.ItemPatch{Abilities: &changed})
	require.True(t, errors.As(err, &fe), "%v", err)
	assert.Equal(t, "abilities", fe.Field)

	_, err = svc.Create(context.Background(), 1, itemsvc.ItemInput{
		Name: localized.Of("Coil"), Category: itemsvc.CategoryGear, Behaviour: itemsvc.BehaviourStock, Count: 3,
		Abilities: []itemsvc.Ability{{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 5, Left: 5, Cost: 1}}},
	})
	require.True(t, errors.As(err, &fe), "%v", err)
}
