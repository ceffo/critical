package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0035 item 6 through the real services on the fixture's catalogue: a
// fighter wields the long sword +1/+2 (speed 5, lowered by the lesser plus:
// 4); Next round rolls a 7, the initiative is 11. Using a wand (PHB Table 56:
// +3) corrects the round to the same 7 + 3 = 10, no die rolled (item 6.2).
// One undo pops the use and its correction (back to 11); redo corrects again
// (ADR 0013 steps 2 and 4 still hold: undo and redo restore the initiative).
// A reroll right after the correction is allowed and keeps the wand's +3
// (item 6.4): 2 + 3 = 5; undoing it gives the 10 back.
func TestInitiative_ADR0035Item6_WeaponSpeedThenAnItemUsedCorrectsTheRound(t *testing.T) {
	ctx := context.Background()
	dice := &attackDice{t: t, dies: []int{7, 2}, calls: map[string]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Bram", Class: "fighter", Race: "human", Str: 16, Dex: 16, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 10,
	})
	require.NoError(t, err)
	sword, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: "long_sword", MagicToHit: 1, MagicDamage: 2, Equip: true})
	require.NoError(t, err)
	wand, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{
		Name: localized.Of("Wand of frost"), Category: itemsvc.CategoryMagic, MagicKind: "wand",
		Behaviour: itemsvc.BehaviourCharges, Count: 5, MaxCount: 5,
	})
	require.NoError(t, err)

	_, _, round, rolled, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 1, round)
	assert.Equal(t, 7, rolled.D10)
	assert.Equal(t, []buffsvc.InitiativeModifier{
		{Source: buffsvc.InitiativeWeapon, Value: 4, ItemID: sword.Item.ID, Name: "Fixture long sword"},
	}, rolled.Modifiers)
	assert.Equal(t, 11, rolled.Total())

	_, err = bs.Items.UseItem(ctx, id, wand.Item.ID)
	require.NoError(t, err)
	corrected, err := bs.Buffs.GetInitiative(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, corrected)
	assert.Equal(t, 7, corrected.D10, "the same die")
	assert.Equal(t, []buffsvc.InitiativeModifier{
		{Source: buffsvc.InitiativeItem, Value: 3, ItemID: wand.Item.ID, Name: "Wand of frost", MagicKind: "wand"},
	}, corrected.Modifiers, "the wand replaces the weapon's speed")
	assert.Equal(t, 10, corrected.Total())
	assert.Equal(t, map[string]int{"die": 1}, dice.calls, "the correction rolls nothing")

	top, ok, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, corrected.LogEntryID, top, "the correction is the use's last entry")
	first, err := bs.Log.Undo(ctx, id, top)
	require.NoError(t, err)
	back, err := bs.Buffs.GetInitiative(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, rolled.Total(), back.Total(), "one undo pops the use and its correction")
	assert.Equal(t, rolled.LogEntryID, back.LogEntryID)
	items, err := bs.Items.List(ctx, id)
	require.NoError(t, err)
	for _, it := range items {
		if it.ID == wand.Item.ID {
			assert.Equal(t, 5, it.Count, "the charge comes back with it")
		}
	}

	_, err = bs.Log.Redo(ctx, id, first.ID)
	require.NoError(t, err)
	again, err := bs.Buffs.GetInitiative(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 10, again.Total(), "redo corrects the round again")
	top, _, err = bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, top, again.LogEntryID, "the redone correction names its entry")

	rerolled, err := bs.Buffs.RerollInitiative(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 2, rerolled.D10)
	assert.Equal(t, again.Modifiers, rerolled.Modifiers, "the reroll keeps the wand's +3")
	assert.Equal(t, 5, rerolled.Total())
	top, _, err = bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	_, err = bs.Log.Undo(ctx, id, top)
	require.NoError(t, err)
	undone, err := bs.Buffs.GetInitiative(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 10, undone.Total(), "undo after the reroll gives the corrected roll back")
	assert.Equal(t, map[string]int{"die": 2}, dice.calls)
}

// ADR 0035 item 6.2 with ADR 0033 step 2 through the real services and the
// fixture's spell catalogue: a scroll named in the editor (burning hands,
// casting time 1) is stored, cleared and given back by undo; used in round 1
// (d10 7, no weapon in hand), it corrects the round to 7 + 1 = 8, its term
// carrying the spell.
func TestInitiative_ADR0035Item6_2_ANamedScrollCorrectsTheRoundByItsSpell(t *testing.T) {
	ctx := context.Background()
	dice := &attackDice{t: t, dies: []int{7}, calls: map[string]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Ilsa", Class: "mage", Race: "human", Str: 9, Dex: 14, Con: 12, Int: 17, Wis: 10, Cha: 10, HPMax: 4,
	})
	require.NoError(t, err)
	scroll, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{
		Name: localized.Of("Scroll"), Category: itemsvc.CategoryMagic, MagicKind: "scroll",
		Behaviour: itemsvc.BehaviourStock, Count: 1,
	})
	require.NoError(t, err)
	spellOf := func() string {
		t.Helper()
		items, err := bs.Items.List(ctx, id)
		require.NoError(t, err)
		for _, it := range items {
			if it.ID == scroll.Item.ID {
				return it.SpellID
			}
		}
		t.Fatalf("no scroll")
		return ""
	}

	named := "burning_hands"
	_, err = bs.Items.Update(ctx, id, scroll.Item.ID, itemsvc.ItemPatch{SpellID: &named})
	require.NoError(t, err)
	none := ""
	cleared, err := bs.Items.Update(ctx, id, scroll.Item.ID, itemsvc.ItemPatch{SpellID: &none})
	require.NoError(t, err)
	assert.Empty(t, spellOf())
	_, err = bs.Log.Undo(ctx, id, cleared.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, "burning_hands", spellOf(), "undo gives the spell back")

	_, _, round, rolled, _, err := bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 1, round)
	assert.Equal(t, 7, rolled.Total())

	_, err = bs.Items.UseItem(ctx, id, scroll.Item.ID)
	require.NoError(t, err)
	corrected, err := bs.Buffs.GetInitiative(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, corrected)
	assert.Equal(t, []buffsvc.InitiativeModifier{
		{Source: buffsvc.InitiativeItem, Value: 1, ItemID: scroll.Item.ID, Name: "Scroll", MagicKind: "scroll", SpellID: "burning_hands"},
	}, corrected.Modifiers)
	assert.Equal(t, 8, corrected.Total())
	assert.Equal(t, map[string]int{"die": 1}, dice.calls, "the correction rolls nothing")
}
