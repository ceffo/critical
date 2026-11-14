package composition_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// spellTable is a first-level mage of 20 max HP on queued dice, for the spell effect of an item
// ability (ADR 0049 step 12) through the real services and the in-memory store.
func spellTable(t *testing.T, dice *attackDice) *initiativeTable {
	t.Helper()
	return newInitiativeTable(t, dice, charactersvc.NewCharacter{
		Name: "Elminster", Class: "mage", Race: "human", Str: 9, Dex: 12, Con: 11, Int: 15, Wis: 12, Cha: 10, HPMax: 20,
	})
}

func (tb *initiativeTable) hp(t *testing.T) int {
	t.Helper()
	st, err := tb.bs.Log.GetHPState(tb.ctx, tb.id)
	require.NoError(t, err)
	return st.CurrentHp
}

func (tb *initiativeTable) wound(t *testing.T, by int) {
	t.Helper()
	_, err := tb.bs.Log.AppendHPEvent(tb.ctx, tb.id, -by, "goblin")
	require.NoError(t, err)
}

func (tb *initiativeTable) undo(t *testing.T) {
	t.Helper()
	_, err := tb.bs.Log.Undo(tb.ctx, tb.id, tb.top(t).ID)
	require.NoError(t, err)
}

func (tb *initiativeTable) logLen(t *testing.T) int {
	t.Helper()
	entries, err := tb.bs.Log.List(tb.ctx, tb.id, 200)
	require.NoError(t, err)
	return len(entries)
}

func (tb *initiativeTable) item(t *testing.T, in itemsvc.ItemInput) itemsvc.Item {
	t.Helper()
	res, err := tb.bs.Items.Create(tb.ctx, tb.id, in)
	require.NoError(t, err)
	return res.Item
}

func spellEffect(spell string, level int) itemsvc.Effect {
	return itemsvc.Effect{Kind: itemsvc.EffectSpell, SpellID: spell, Level: level}
}

func wandOf(effects ...itemsvc.Effect) itemsvc.ItemInput {
	return itemsvc.ItemInput{
		Name: localized.Of("Wand"), Category: itemsvc.CategoryMagic, MagicKind: "wand", Behaviour: itemsvc.BehaviourStock, Count: 1,
		Abilities: []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 5, Left: 5, Cost: 1}, Effects: effects}},
	}
}

func scrollOf(effects ...itemsvc.Effect) itemsvc.ItemInput {
	return itemsvc.ItemInput{
		Name: localized.Of("Scroll"), Category: itemsvc.CategoryMagic, MagicKind: "scroll", Behaviour: itemsvc.BehaviourStock, Count: 1,
		Abilities: []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}, Effects: effects}},
	}
}

// cast reads the cast breakdown an effect's result carries.
func cast(t *testing.T, e itemsvc.EffectResult) map[string]any {
	t.Helper()
	require.NotEmpty(t, e.Cast)
	var m map[string]any
	require.NoError(t, json.Unmarshal(e.Cast, &m))
	return m
}

// ADR 0049 steps 12 and 15: a wand of a spell. Use casts Magic Missile at the effect's fixed
// level 5 (three missiles of 1d4+1, the catalogue's min(5, 1+(level-1)/2)), through castsvc's one
// effect path: it spends a charge and no slot, rolls no spell failure, and corrects the round's
// initiative once, by the wand's Table 56 row (+3), not by the spell's casting time. One undo pops
// the charge and the correction.
func TestUseAbility_ADR0049Step12_AWandCastsItsSpellAtAFixedLevelAndPricesTheWand(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{6, 2, 3, 4}, calls: map[string]int{}} // the round's d10, then three missiles
	tb := spellTable(t, dice)
	wand := tb.item(t, wandOf(spellEffect("magic_missile", 5)))
	_, _, _, _, _, err := tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
	require.NoError(t, err)

	res, err := tb.bs.Items.UseAbility(tb.ctx, tb.id, wand.ID, "use", itemsvc.UseAsks{})
	require.NoError(t, err)
	require.Len(t, res.Effects, 1)
	assert.Equal(t, itemsvc.EffectSpell, res.Effects[0].Kind)
	c := cast(t, res.Effects[0])
	assert.Equal(t, "item", c["action"])
	assert.EqualValues(t, 5, c["casterLevel"])
	assert.EqualValues(t, 12, c["damage"].(map[string]any)["total"], "(2+1) + (3+1) + (4+1)")
	assert.Equal(t, 4, itemOf(t, tb.bs, tb.ctx, tb.id, wand.ID).Abilities[0].Limit.Left, "one charge")
	assert.Equal(t, map[string]int{"die": 4}, dice.calls, "the d10 and three missiles: no failure die, no second roll")
	assert.Equal(t, 1, tb.countKind(t, actionlog.KindInitiativeCorrect))
	assert.Equal(t, []buffsvc.InitiativeModifier{
		{Source: buffsvc.InitiativeItem, Value: 3, ItemID: wand.ID, Name: "Wand", MagicKind: "wand"},
	}, tb.initiative(t).Modifiers, "the wand's +3, not Magic Missile's casting time")
	assert.Equal(t, 9, tb.initiative(t).Total())
	assert.Equal(t, 0, tb.countKind(t, actionlog.KindCast), "an item's spell is no cast entry: the use's entry holds it")

	tb.undo(t) // the use and its initiative correction
	assert.Equal(t, 5, itemOf(t, tb.bs, tb.ctx, tb.id, wand.ID).Abilities[0].Limit.Left, "undo: the charge is back")
	assert.Nil(t, tb.initiative(t).Modifiers)
}

// ADR 0049 steps 9, 12 and 2: a scroll of Cure Light Wounds spends the scroll and heals the
// bearer through the HP ledger; the last scroll is removed in the same entry, and one undo gives
// back the scroll and the points.
func TestUseAbility_ADR0049Step12_AScrollCastsItsSpellAndIsSpent(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{6}, calls: map[string]int{}}
	tb := spellTable(t, dice)
	tb.wound(t, 9)
	scroll := tb.item(t, scrollOf(spellEffect("cure_light_wounds", 1)))
	require.Equal(t, 11, tb.hp(t))

	res, err := tb.bs.Items.UseAbility(tb.ctx, tb.id, scroll.ID, "use", itemsvc.UseAsks{})
	require.NoError(t, err)
	assert.True(t, res.Removed, "the last scroll is gone")
	assert.Equal(t, 17, tb.hp(t), "11 + the d8 of 6")
	require.NotNil(t, res.Effects[0].Applied)
	assert.Equal(t, itemsvc.Applied{HPDelta: 6, HPBefore: 11, HPAfter: 17}, *res.Effects[0].Applied)
	assert.Equal(t, 1, tb.countKind(t, actionlog.KindItemUse))

	tb.undo(t)
	assert.Equal(t, 11, tb.hp(t), "undo: the points are back")
	assert.Equal(t, 1, itemOf(t, tb.bs, tb.ctx, tb.id, scroll.ID).Count, "undo: the scroll is back")
}

// ADR 0049 step 12: effects run in list order against one ledger: a heal effect fills 3 of the 20
// max, then a spell's heal reads the ledger as the first one left it, so it heals 0 (shown as such),
// and the entry's delta is the 3 the ledger can take.
func TestUseAbility_ADR0049Step12_ASpellHealReadsWhatAnEarlierHealFilled(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{4, 8}, calls: map[string]int{}}
	tb := spellTable(t, dice)
	tb.wound(t, 3)
	scroll := tb.item(t, scrollOf(itemsvc.Effect{Kind: itemsvc.EffectHeal, Dice: "1d6"}, spellEffect("cure_light_wounds", 1)))

	res, err := tb.bs.Items.UseAbility(tb.ctx, tb.id, scroll.ID, "use", itemsvc.UseAsks{})
	require.NoError(t, err)
	assert.Equal(t, 20, tb.hp(t))
	require.NotNil(t, res.Effects[1].Applied)
	assert.Equal(t, itemsvc.Applied{HPDelta: 0, HPBefore: 20, HPAfter: 20}, *res.Effects[1].Applied)
	heal := cast(t, res.Effects[1])["heal"].(map[string]any)
	assert.EqualValues(t, 20, heal["hpBefore"], "the spell sees 20, the first heal's result")
	assert.EqualValues(t, 8, heal["total"], "the roll is shown in full")
}

// ADR 0049 steps 9, 10 and 12: the asks of a spell effect are the cast's own. Chromatic Orb needs a
// target AC (to-hit d20) and a gem. With no answer the use is refused before anything is written or
// rolled; answered, it rolls the to-hit, spends the gem and the wand's charge in one entry, and one
// undo gives both back.
func TestUseAbility_ADR0049Steps9And10_TheAsksAreServedAndAnswered(t *testing.T) {
	dice := &attackDice{t: t, d20s: []int{15}, calls: map[string]int{}}
	tb := spellTable(t, dice)
	gem := tb.item(t, itemsvc.ItemInput{Name: localized.Of("Opal"), Category: itemsvc.CategoryGear, Component: "gem", Behaviour: itemsvc.BehaviourStock, Count: 2})
	wand := tb.item(t, wandOf(spellEffect("chromatic_orb", 3)))
	before := tb.logLen(t)

	_, err := tb.bs.Items.UseAbility(tb.ctx, tb.id, wand.ID, "use", itemsvc.UseAsks{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidInput), "a 400: the AC is asked: %v", err)
	assert.Equal(t, 5, itemOf(t, tb.bs, tb.ctx, tb.id, wand.ID).Abilities[0].Limit.Left, "no charge spent")
	assert.Equal(t, 2, itemOf(t, tb.bs, tb.ctx, tb.id, gem.ID).Count, "no gem spent")
	assert.Equal(t, before, tb.logLen(t), "nothing logged")
	assert.Empty(t, dice.calls, "no die rolled")

	ac := 12
	res, err := tb.bs.Items.UseAbility(tb.ctx, tb.id, wand.ID, "use", itemsvc.UseAsks{Spells: []itemsvc.SpellAsks{{Effect: 0, TargetAC: &ac}}})
	require.NoError(t, err)
	assert.Equal(t, 4, itemOf(t, tb.bs, tb.ctx, tb.id, wand.ID).Abilities[0].Limit.Left)
	assert.Equal(t, 1, itemOf(t, tb.bs, tb.ctx, tb.id, gem.ID).Count, "the only gem type left is spent")
	assert.NotNil(t, cast(t, res.Effects[0])["attack"], "the to-hit is shown")
	assert.Equal(t, map[string]int{"d20": 1}, dice.calls)
	assert.Equal(t, before+1, tb.logLen(t), "one entry")

	tb.undo(t)
	assert.Equal(t, 5, itemOf(t, tb.bs, tb.ctx, tb.id, wand.ID).Abilities[0].Limit.Left)
	assert.Equal(t, 2, itemOf(t, tb.bs, tb.ctx, tb.id, gem.ID).Count, "one undo gives the gem back too")
}

// ADR 0049 step 12: a failure in any effect rolls the use back, the limit included: the first
// effect heals and the second asks an AC nobody gave.
func TestUseAbility_ADR0049Step12_AFailingSpellEffectRollsTheWholeUseBack(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{6}, calls: map[string]int{}}
	tb := spellTable(t, dice)
	tb.wound(t, 9)
	wand := tb.item(t, wandOf(spellEffect("cure_light_wounds", 1), spellEffect("chromatic_orb", 3)))
	before := tb.logLen(t)

	_, err := tb.bs.Items.UseAbility(tb.ctx, tb.id, wand.ID, "use", itemsvc.UseAsks{})
	require.Error(t, err)
	assert.Equal(t, 5, itemOf(t, tb.bs, tb.ctx, tb.id, wand.ID).Abilities[0].Limit.Left, "the charge is not spent")
	assert.Equal(t, 11, tb.hp(t), "no points healed")
	assert.Equal(t, before, tb.logLen(t), "nothing logged")
}

// ADR 0049 steps 9 and 12: a spell that grants slots adds them, blank, through the asks' split; the
// slots' snapshot rides the use's entry, so one undo takes them back.
func TestUseAbility_ADR0049Step12_GrantedSlotsRideTheUseAndUndo(t *testing.T) {
	tb := spellTable(t, &attackDice{t: t, calls: map[string]int{}})
	wand := tb.item(t, wandOf(spellEffect("rarys_mnemonic_enhancer", 7)))
	slots := func() int {
		got, err := tb.bs.Spells.GetSlots(tb.ctx, tb.id)
		require.NoError(t, err)
		return len(got)
	}
	start := slots()

	_, err := tb.bs.Items.UseAbility(tb.ctx, tb.id, wand.ID, "use", itemsvc.UseAsks{Spells: []itemsvc.SpellAsks{{Effect: 0, ExtraSlots: []int{1, 2}}}})
	require.NoError(t, err)
	assert.Equal(t, start+2, slots(), "the 1 and the 2 the player split three levels into")
	tb.undo(t)
	assert.Equal(t, start, slots(), "undo: the slots are gone")

	_, err = tb.bs.Items.UseAbility(tb.ctx, tb.id, wand.ID, "use", itemsvc.UseAsks{Spells: []itemsvc.SpellAsks{{Effect: 0, ExtraSlots: []int{2, 2}}}})
	require.Error(t, err, "a split that does not add up to the spell's levels is refused")
	assert.Equal(t, start, slots())
}

// ADR 0049 step 10: the editor's list is checked: a spell the catalogue does not know, a level out of 1
// to 30; and the asks of a use name spell effects only, once each.
func TestAbilities_ADR0049Step10_ASpellEffectIsValidated(t *testing.T) {
	tb := spellTable(t, &attackDice{t: t, calls: map[string]int{}})
	for name, e := range map[string]itemsvc.Effect{
		"unknown spell": spellEffect("no_such_spell", 5), "level 0": spellEffect("magic_missile", 0), "level 31": spellEffect("magic_missile", 31),
		"stray dice": {Kind: itemsvc.EffectSpell, SpellID: "magic_missile", Level: 5, Dice: "1d6"},
	} {
		_, err := tb.bs.Items.Create(tb.ctx, tb.id, wandOf(e))
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe, name)
		assert.Equal(t, "abilities", fe.Field, name)
	}

	wand := tb.item(t, wandOf(itemsvc.Effect{Kind: itemsvc.EffectDice, Dice: "1d6"}, spellEffect("magic_missile", 5)))
	for name, asks := range map[string]itemsvc.UseAsks{
		"a dice effect":    {Spells: []itemsvc.SpellAsks{{Effect: 0}}},
		"out of range":     {Spells: []itemsvc.SpellAsks{{Effect: 2}}},
		"answered twice":   {Spells: []itemsvc.SpellAsks{{Effect: 1}, {Effect: 1}}},
		"a negative index": {Spells: []itemsvc.SpellAsks{{Effect: -1}}},
	} {
		_, err := tb.bs.Items.UseAbility(tb.ctx, tb.id, wand.ID, "use", asks)
		assert.True(t, errors.Is(err, domain.ErrInvalidInput), "%s: %v", name, err)
	}
	assert.Equal(t, 5, itemOf(t, tb.bs, tb.ctx, tb.id, wand.ID).Abilities[0].Limit.Left, "nothing was spent")
}
