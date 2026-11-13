package itemsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

func TestNew_NilDAL(t *testing.T) {
	_, err := itemsvc.New(nil, itemsvc.Deps{})
	require.Error(t, err)
}

func TestUse_ReturnsUpdatedItem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, err := itemsvc.NewWithConfig(itemsvc.DefaultConfig(), dal, deps(t).Deps)
	require.NoError(t, err)
	dal.EXPECT().Decrement(mock.Anything, int64(1), "wand").Return(itemsvc.Item{ID: "wand", Count: 3}, nil)

	got, err := svc.Use(context.Background(), 1, "wand")
	require.NoError(t, err)
	assert.Equal(t, 3, got.Count)
}

func TestUse_PropagatesSentinels(t *testing.T) {
	for _, target := range []error{itemsvc.ErrItemEmpty, itemsvc.ErrItemNotFound} {
		dal := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		dal.EXPECT().Decrement(mock.Anything, int64(1), "x").Return(itemsvc.Item{}, fmt.Errorf("%w: x", target))
		_, err := svc.Use(context.Background(), 1, "x")
		assert.True(t, errors.Is(err, target))
	}
}

func TestBehaviour_Rules(t *testing.T) {
	tests := []struct {
		b           itemsvc.Behaviour
		valid, gone bool
	}{
		{itemsvc.BehaviourStock, true, false},
		{itemsvc.BehaviourCharges, true, true},
		{itemsvc.BehaviourDaily, true, false},
		{"charged", false, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.b), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.b.Valid())
			assert.Equal(t, tt.gone, tt.b.RemovedAtZero())
		})
	}
}

// ADR 0049 step 2: a use at 0 removes a potion or a scroll, whatever the old behaviour said; every
// other piece stays listed.
func TestUse_ADR0049Step2_RemovesAPotionOrScrollAtZeroOnly(t *testing.T) {
	tests := []struct {
		name    string
		item    itemsvc.Item
		removed bool
	}{
		{"a potion spent to 0", itemsvc.Item{ID: "x", Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicPotion, Count: 0}, true},
		{"a scroll spent to 0", itemsvc.Item{ID: "x", Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicScroll, Count: 0}, true},
		{"a potion with doses left", itemsvc.Item{ID: "x", Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicPotion, Count: 1}, false},
		{"a charged behaviour no longer removes", itemsvc.Item{ID: "x", Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicWand, Behaviour: itemsvc.BehaviourCharges, Count: 0}, false},
		{"arrows at 0 stay", itemsvc.Item{ID: "x", Category: itemsvc.CategoryAmmunition, Count: 0}, false},
		{"gear at 0 stays", itemsvc.Item{ID: "x", Category: itemsvc.CategoryGear, Count: 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t) // Delete fails the test unless expected
			svc, _ := itemsvc.New(dal, deps(t).Deps)
			dal.EXPECT().Decrement(mock.Anything, int64(1), "x").Return(tt.item, nil)
			if tt.removed {
				dal.EXPECT().Delete(mock.Anything, int64(1), "x").Return(nil).Once()
			}
			got, err := svc.Use(context.Background(), 1, "x")
			require.NoError(t, err)
			assert.Equal(t, tt.removed, got.Removed())
			assert.Equal(t, tt.item.Count, got.Count)
		})
	}
}

func TestUse_RemoveErrorFails(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Decrement(mock.Anything, int64(1), "x").Return(potion, nil)
	dal.EXPECT().Delete(mock.Anything, int64(1), "potion_healing").Return(errors.New("boom"))
	_, err := svc.Use(context.Background(), 1, "x")
	require.Error(t, err)
}

// converted is the item as the boot conversion leaves it (ADR 0049 step 19): the abilities of its
// old fields. Popping a snapshot of before abilities gives the item back so (step 20).
func converted(it itemsvc.Item) itemsvc.Item {
	c := itemsvc.ConvertLegacy(it)
	it.Abilities, it.Count = c.Abilities, c.Quantity
	return it
}

// potion is a potion at 0 after its last dose was spent: the use removed it.
var potion = itemsvc.Item{
	ID: "potion_healing", Position: 4, Name: localized.Of("Potion of healing"), Type: "potion", Behaviour: itemsvc.BehaviourStock,
	Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicPotion, State: itemsvc.StateCarried, Count: 0, MaxCount: 4,
}

const potionBefore = `{"count":1,"item":{"position":4,"name":{"en":"Potion of healing"},"type":"potion",
	"behaviour":"stock","magicalBonus":0,"ammoKind":"","maxCount":4,"effectDice":"","effectSave":"","notes":"","powers":[],
	"piece":{"category":"magic","magicKind":"potion","state":"carried"}}}`

func TestUseSnapshot_BeforeIsOneMore(t *testing.T) {
	for _, b := range []itemsvc.Behaviour{itemsvc.BehaviourStock, itemsvc.BehaviourDaily} {
		snap, err := itemsvc.UseSnapshot(itemsvc.Item{ID: "arrows_plus3", Name: localized.Of("Arrows +3"), Behaviour: b, Count: 0})
		require.NoError(t, err)
		assert.Equal(t, "items:arrows_plus3", snap.Key)
		assert.Equal(t, "arrows_plus3", snaptest.ID(t, snap.Before))
		assert.JSONEq(t, `{"count":1}`, snaptest.State(t, snap.Before), "kept at 0: no item data")
		assert.JSONEq(t, `{"count":0}`, snaptest.State(t, snap.After))
	}
}

func TestUseSnapshot_MarksARemovedItem(t *testing.T) {
	snap, err := itemsvc.UseSnapshot(potion)
	require.NoError(t, err)
	assert.JSONEq(t, potionBefore, snaptest.State(t, snap.Before))
	assert.Equal(t, snaptest.Absent("potion_healing"), snap.After, "the use removed it")
}

func TestRestorers_WriteTheBeforeCountsBack(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	snap, err := itemsvc.UseSnapshot(itemsvc.Item{ID: "arrows_plus3", Count: 0})
	require.NoError(t, err)

	dal.EXPECT().Get(mock.Anything, int64(1), "arrows_plus3").Return(itemsvc.Item{ID: "arrows_plus3", Category: itemsvc.CategoryAmmunition, Count: 0}, nil).Once()
	dal.EXPECT().SetCount(mock.Anything, int64(1), "arrows_plus3", 1).Return(nil).Once()
	r := svc.Module().Restorer(itemsvc.StateKeyItems)
	require.NotNil(t, r)
	require.NoError(t, r.Restore(context.Background(), 1, snap.Before))
}

// Popping the use that removed a potion inserts it back whole, with 1 dose.
func TestRestorers_PutBackARemovedItem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	snap, err := itemsvc.UseSnapshot(potion)
	require.NoError(t, err)

	dal.EXPECT().SetCount(mock.Anything, int64(1), "potion_healing", 1).Return(fmt.Errorf("%w: potion_healing", itemsvc.ErrItemNotFound)).Once()
	back := potion
	back.Count = 1
	dal.EXPECT().Get(mock.Anything, int64(1), "potion_healing").Return(itemsvc.Item{}, itemsvc.ErrItemNotFound).Maybe()
	dal.EXPECT().Insert(mock.Anything, int64(1), converted(back)).Return(nil).Once()
	require.NoError(t, svc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, snap.Before))
}

// A count-only state (a use that kept the item) cannot recreate a missing item.
func TestRestorers_MissingItemWithoutDataFails(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(itemsvc.Item{}, itemsvc.ErrItemNotFound)
	dal.EXPECT().SetCount(mock.Anything, int64(1), "x", 2).Return(fmt.Errorf("%w: x", itemsvc.ErrItemNotFound))
	err := svc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, snaptest.EnvID("x", `{"count":2}`))
	assert.ErrorIs(t, err, itemsvc.ErrItemNotFound)
}

func TestRestorers_RejectBadSnapshots(t *testing.T) {
	for _, state := range []json.RawMessage{
		snaptest.EnvID("x", `[{"count":1}]`), snaptest.Env(`{"count":1}`), snaptest.EnvID("x", `{"count":-1}`), snaptest.Absent(""),
	} {
		dal := mocks.NewMockDAL(t) // nothing is written
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		err := svc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, state)
		assert.Error(t, err, string(state))
	}
}

// fixture holds the collaborators UseItem needs besides the DAL.
type fixture struct {
	itemsvc.Deps
	dice       *mocks.MockDice
	log        *mocks.MockLog
	chars      *mocks.MockCharacters
	initiative *mocks.MockInitiative
}

// noTx runs the unit of work directly.
type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// bookRuleset resolves every character to the default ruleset (ADR 0041 step 9).
type bookRuleset struct{}

func (bookRuleset) ForCharacter(context.Context, int64) (ruleset.Ruleset, error) {
	return ruleset.Default(), nil
}

// noClasses knows no class: nothing is limited.
type noClasses struct{}

func (noClasses) Class(string) (classes.Class, error) {
	return classes.Class{}, classes.ErrUnknownClass
}

func deps(t *testing.T) fixture {
	dice, log, chars, initiative := mocks.NewMockDice(t), mocks.NewMockLog(t), mocks.NewMockCharacters(t), mocks.NewMockInitiative(t)
	return fixture{
		Deps: itemsvc.Deps{Dice: dice, Log: log, Tx: noTx{}, Chars: chars, Classes: noClasses{}, Initiative: initiative, Rulesets: bookRuleset{}},
		dice: dice, log: log, chars: chars, initiative: initiative,
	}
}

// readsCharacter is deps whose character, any number of reads, is a class that does not
// specialize: a weapon piece read derives its Specialized from it (ADR 0037 step 9).
func readsCharacter(t *testing.T) fixture {
	f := deps(t)
	f.chars.EXPECT().View(mock.Anything, mock.Anything).Return(viewtest.Single("fighter", 1), nil).Maybe()
	f.chars.EXPECT().WeaponProficiency(mock.Anything, mock.Anything).Return(skills.WeaponProficiency{}, nil).Maybe()
	return f
}

// ADR 0049 step 2: the inventory's minus is spend one: a count change with no ability, no effect and
// no initiative (the mocks of the dice and the initiative fail on any call), logged as an item_use
// entry without an abilityId whose snapshot restores the count. Whatever the item carries (a flask
// with a dice ability), the minus rolls nothing.
func TestUseItem_ADR0049Step2_SpendOneRollsAndCorrectsNothing(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	f.InitiativeItems = table56(t)
	svc, err := itemsvc.New(dal, f.Deps)
	require.NoError(t, err)
	flask := itemsvc.Item{
		ID: "fire_flask", Name: localized.Of("Fire flasks"), Type: "flask", Category: itemsvc.CategoryGear, Count: 4,
		Abilities: []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}, Effects: []itemsvc.Effect{{Kind: itemsvc.EffectDice, Dice: "5d6"}}}},
	}
	dal.EXPECT().Get(mock.Anything, int64(1), "fire_flask").Return(flask, nil)
	spent := flask
	spent.Count = 3
	dal.EXPECT().Decrement(mock.Anything, int64(1), "fire_flask").Return(spent, nil)
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(9), nil)

	res, err := svc.UseItem(context.Background(), 1, "fire_flask")
	require.NoError(t, err)
	assert.Equal(t, &itemsvc.UseResult{ItemID: "fire_flask", Name: "Fire flasks", Type: "flask", Remaining: 3, LogEntryID: 9}, res)

	assert.Equal(t, actionlog.KindItemUse, got.Kind)
	require.Len(t, got.Snapshots, 1)
	assert.Equal(t, "items:fire_flask", got.Snapshots[0].Key)
	assert.JSONEq(t, `{"count":4}`, snaptest.State(t, got.Snapshots[0].Before))
	assert.JSONEq(t, `{"count":3}`, snaptest.State(t, got.Snapshots[0].After))
	assert.JSONEq(t, `{"itemId":"fire_flask","name":"Fire flasks","type":"flask","removed":false,"remaining":3,"maxCount":0,"rolls":[],"total":0}`, string(got.Payload),
		"no abilityId: a spend")
}

func TestUseItem_EmptyAndUnknownLogNothing(t *testing.T) {
	for _, target := range []error{itemsvc.ErrItemEmpty, itemsvc.ErrItemNotFound} {
		dal := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(dal, deps(t).Deps) // log and dice mocks fail on any call
		if errors.Is(target, itemsvc.ErrItemNotFound) {
			dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(itemsvc.Item{}, fmt.Errorf("%w: x", target))
		} else {
			dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(itemsvc.Item{ID: "x", Category: itemsvc.CategoryGear}, nil)
			dal.EXPECT().Decrement(mock.Anything, int64(1), "x").Return(itemsvc.Item{}, fmt.Errorf("%w: x", target))
		}
		_, err := svc.UseItem(context.Background(), 1, "x")
		assert.True(t, errors.Is(err, target))
	}
}

// The inventory's minus spends arrows too (lost, broken).
func TestUseItem_SpendsStockAmmo(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	arrows := itemsvc.Item{ID: "arrows", Name: localized.Of("Arrows"), Category: itemsvc.CategoryAmmunition, AmmoKind: "arrow", Count: 0}
	dal.EXPECT().Decrement(mock.Anything, int64(1), "arrows").Return(arrows, nil)
	dal.EXPECT().Get(mock.Anything, int64(1), "arrows").Return(arrows, nil).Maybe()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(3), nil)
	res, err := svc.UseItem(context.Background(), 1, "arrows")
	require.NoError(t, err)
	assert.Equal(t, 0, res.Remaining)
	assert.False(t, res.Removed, "ammunition stays listed at 0")
}

// ADR 0049 step 2: the minus that takes a potion's last dose removes it in the same entry, whose
// snapshot carries the whole potion so popping it brings it back; still no initiative.
func TestUseItem_ADR0049Step2_LastDoseRemovesThePotion(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Decrement(mock.Anything, int64(1), "potion_healing").Return(potion, nil)
	dal.EXPECT().Get(mock.Anything, int64(1), "potion_healing").Return(potion, nil).Maybe()
	dal.EXPECT().Delete(mock.Anything, int64(1), "potion_healing").Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(5), nil)

	res, err := svc.UseItem(context.Background(), 1, "potion_healing")
	require.NoError(t, err)
	assert.True(t, res.Removed)
	assert.Equal(t, 0, res.Remaining)
	assert.Empty(t, got.Summary, "the UI words the use from its payload")
	require.Len(t, got.Snapshots, 1)
	assert.JSONEq(t, potionBefore, snaptest.State(t, got.Snapshots[0].Before))
	assert.True(t, snaptest.IsAbsent(t, got.Snapshots[0].After))
	assert.Contains(t, string(got.Payload), `"removed":true`)
}

// ADR 0049 step 2: an item with no stock (a ring, a wand, a weapon) or one whose counter is an
// ability's has nothing to spend: the minus is refused, nothing moves or is logged.
func TestUseItem_ADR0049Step2_RefusesWhatHasNoStock(t *testing.T) {
	perDay := []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 1}}}
	for name, it := range map[string]itemsvc.Item{
		"a wand":                         {ID: "x", Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicWand, Count: 1},
		"a gem":                          {ID: "x", Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicMisc, Count: 1},
		"gear holding a per_day ability": {ID: "x", Category: itemsvc.CategoryGear, Count: 1, Abilities: perDay},
	} {
		t.Run(name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t) // a Decrement fails the strict mock
			svc, _ := itemsvc.New(dal, deps(t).Deps)
			dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(it, nil)
			_, err := svc.UseItem(context.Background(), 1, "x")
			assert.ErrorIs(t, err, itemsvc.ErrNotUsable)
		})
	}
}

func TestRestock_AddsToStockAndLogs(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	arrows := itemsvc.Item{
		ID: "arrows_plus3", Name: localized.Of("Arrows +3"), Type: "arrow", Category: itemsvc.CategoryAmmunition, Count: 1,
	}
	dal.EXPECT().Get(mock.Anything, int64(1), "arrows_plus3").Return(arrows, nil)
	dal.EXPECT().Increment(mock.Anything, int64(1), "arrows_plus3", 1).Return(arrows, nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(7), nil)

	res, err := svc.Restock(context.Background(), 1, "arrows_plus3", 1)
	require.NoError(t, err)
	assert.Equal(t, &itemsvc.RestockResult{ItemID: "arrows_plus3", Name: "Arrows +3", Type: "arrow", Added: 1, Count: 1, LogEntryID: 7}, res)
	assert.Equal(t, actionlog.KindItemRestock, got.Kind)
	assert.False(t, actionlog.IsRoll(got.Kind), "a restock leaves the result card alone")
	assert.JSONEq(t, `{"itemId":"arrows_plus3","name":"Arrows +3","type":"arrow","added":1,"count":1}`, string(got.Payload))
	require.Len(t, got.Snapshots, 1)
	assert.Equal(t, "items:arrows_plus3", got.Snapshots[0].Key)
	assert.JSONEq(t, `{"count":0}`, snaptest.State(t, got.Snapshots[0].Before))
	assert.JSONEq(t, `{"count":1}`, snaptest.State(t, got.Snapshots[0].After))
	assert.JSONEq(t, `{"itemId":"arrows_plus3","name":"Arrows +3","type":"arrow","added":1,"count":1}`, string(got.Payload))
}

// ADR 0049 steps 2 and 4: an item with no stock, or holding a per_day or charges ability, is not
// restocked: refused before any write, nothing logged.
func TestRestock_ADR0049Step4_RejectsWhatHasNoStockOrACounter(t *testing.T) {
	for name, it := range map[string]itemsvc.Item{
		"a wand":                  {ID: "x", Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicWand, Count: 1},
		"gear with a charges one": {ID: "x", Category: itemsvc.CategoryGear, Count: 1, Abilities: []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 3, Left: 1, Cost: 1}}}},
	} {
		t.Run(name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t)               // an Increment fails the strict mock
			svc, _ := itemsvc.New(dal, deps(t).Deps) // log mock fails on any call
			dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(it, nil)
			_, err := svc.Restock(context.Background(), 1, "x", 1)
			assert.ErrorIs(t, err, itemsvc.ErrNotRestockable)
		})
	}
}

func TestRestock_RejectsOutOfRangeCounts(t *testing.T) {
	for _, n := range []int{0, -1, itemsvc.MaxRestock + 1} {
		dal := mocks.NewMockDAL(t) // nothing is written
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		_, err := svc.Restock(context.Background(), 1, "x", n)
		assert.ErrorIs(t, err, domain.ErrInvalidInput, n)
	}
}

func TestRestock_UnknownItem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "nope").Return(itemsvc.Item{}, fmt.Errorf("%w: nope", itemsvc.ErrItemNotFound))
	_, err := svc.Restock(context.Background(), 1, "nope", 2)
	assert.ErrorIs(t, err, itemsvc.ErrItemNotFound)
}

// ADR 0033 step 1.1 and ADR 0049 step 3: a spell spends a piece by its component code, never by
// the free-text type label, whatever its category, and never an item with a per_day or charges ability.
func TestItem_Step1_1_Spendable(t *testing.T) {
	base := itemsvc.Item{Component: itemsvc.ComponentGem, Category: itemsvc.CategoryGear, Behaviour: itemsvc.BehaviourStock, Count: 1}
	tests := []struct {
		name string
		mod  func(*itemsvc.Item)
		want bool
	}{
		{"stock with some left", func(*itemsvc.Item) {}, true},
		{"a charges ability", func(i *itemsvc.Item) {
			i.Abilities = []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 3, Left: 2, Cost: 1}}}
		}, false},
		{"an unlimited ability", func(i *itemsvc.Item) {
			i.Abilities = []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}}}
		}, true},
		{"any category", func(i *itemsvc.Item) { i.Category, i.MagicKind = itemsvc.CategoryMagic, itemsvc.MagicMisc }, true},
		{"no component", func(i *itemsvc.Item) { i.Component = "" }, false},
		{"the type label alone", func(i *itemsvc.Item) { i.Component, i.Type = "", "gem" }, false},
		{"a per_day ability", func(i *itemsvc.Item) {
			i.Abilities = []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 1}}}
		}, false},
		{"none left", func(i *itemsvc.Item) { i.Count = 0 }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it := base
			tt.mod(&it)
			assert.Equal(t, tt.want, it.Spendable("gem"))
		})
	}
}

// A composition without class definitions is refused: it would drop every class warning.
func TestNew_RequiresClasses(t *testing.T) {
	f := deps(t)
	f.Classes = nil
	_, err := itemsvc.New(mocks.NewMockDAL(t), f.Deps)
	require.Error(t, err)
}

// ADR 0049 step 2: a weapon, an armour and a shield are one object,
// never used or restocked, whatever their behaviour; nothing is logged and the
// transaction rolls the count change back.
func TestUseItem_RefusesOneObjectCategories(t *testing.T) {
	for _, c := range []itemsvc.Category{itemsvc.CategoryWeapon, itemsvc.CategoryArmour, itemsvc.CategoryShield} {
		for _, b := range []itemsvc.Behaviour{itemsvc.BehaviourStock, itemsvc.BehaviourCharges, itemsvc.BehaviourDaily} {
			t.Run(string(c)+"/"+string(b), func(t *testing.T) {
				dal := mocks.NewMockDAL(t)
				f := deps(t) // the log mock fails on any call
				f.chars.EXPECT().WeaponProficiency(mock.Anything, mock.Anything).Return(skills.WeaponProficiency{}, nil).Maybe()
				svc, _ := itemsvc.New(dal, f.Deps)
				// Only Get is expected: a Decrement would fail the strict mock, so the count never moves.
				dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(itemsvc.Item{ID: "x", Category: c, Behaviour: b, Count: 2}, nil)
				_, err := svc.UseItem(context.Background(), 1, "x")
				assert.ErrorIs(t, err, itemsvc.ErrNotUsable)
			})
		}
	}
}

func TestRestock_RefusesOneObjectCategories(t *testing.T) {
	for _, c := range []itemsvc.Category{itemsvc.CategoryWeapon, itemsvc.CategoryArmour, itemsvc.CategoryShield} {
		t.Run(string(c), func(t *testing.T) {
			dal := mocks.NewMockDAL(t)
			f := deps(t)
			f.chars.EXPECT().WeaponProficiency(mock.Anything, mock.Anything).Return(skills.WeaponProficiency{}, nil).Maybe()
			svc, _ := itemsvc.New(dal, f.Deps)
			// Only Get is expected: an Increment would fail the strict mock.
			dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(itemsvc.Item{ID: "x", Category: c, Behaviour: itemsvc.BehaviourStock, Count: 2}, nil)
			_, err := svc.Restock(context.Background(), 1, "x", 1)
			assert.ErrorIs(t, err, itemsvc.ErrNotRestockable)
		})
	}
}

// ADR 0049 step 2: only a stock (ammunition, gear, a potion, a scroll) has a quantity the player moves.
func TestItem_ADR0049Step2_UsableIsHavingAStock(t *testing.T) {
	for _, c := range itemsvc.Categories {
		want := c == itemsvc.CategoryAmmunition || c == itemsvc.CategoryGear
		assert.Equal(t, want, itemsvc.Item{Category: c}.Usable(), string(c))
	}
	for _, k := range itemsvc.MagicKinds {
		want := k == itemsvc.MagicPotion || k == itemsvc.MagicScroll
		assert.Equal(t, want, itemsvc.Item{Category: itemsvc.CategoryMagic, MagicKind: k}.Usable(), k)
	}
}

// A weapon, an armour or a shield with a component is never spent by a spell (castsvc
// consume_item, prepsvc), the item's own rule applying outside the inventory too.
func TestSpendable_NeverAOneObjectPiece(t *testing.T) {
	for _, c := range itemsvc.Categories {
		got := itemsvc.Item{Category: c, Component: itemsvc.ComponentGem, Behaviour: itemsvc.BehaviourStock, Count: 1}.Spendable(itemsvc.ComponentGem)
		assert.Equal(t, c != itemsvc.CategoryWeapon && c != itemsvc.CategoryArmour && c != itemsvc.CategoryShield, got, string(c))
	}
}
