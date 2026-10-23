package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// initiativeTable is a character in the app on the fixture catalogue with
// queued dice, for the rounds of ADR 0035 item 6.
type initiativeTable struct {
	ctx  context.Context
	bs   *composition.BackingServices
	dice *attackDice
	id   int64
}

func newInitiativeTable(t *testing.T, dice *attackDice, c charactersvc.NewCharacter) *initiativeTable {
	t.Helper()
	ctx := context.Background()
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	c.Details = identity()
	id, err := bs.Character.Create(ctx, s.User.ID, c)
	require.NoError(t, err)
	return &initiativeTable{ctx: ctx, bs: bs, dice: dice, id: id}
}

// mage is a 3rd-level mage (5,000 XP: two 1st and one 2nd) who learns the
// spells (one d100 of 1 each) and prepares them in that order.
func mageWith(t *testing.T, dice *attackDice, spells ...string) *initiativeTable {
	t.Helper()
	for range spells {
		dice.d100s = append([]int{1}, dice.d100s...)
	}
	tb := newInitiativeTable(t, dice, charactersvc.NewCharacter{
		Name: "Elminster", Class: "mage", Race: "human", Str: 9, Dex: 12, Con: 11, Int: 15, Wis: 12, Cha: 10, HPMax: 9, XP: 5000,
	})
	for _, s := range spells {
		got, err := tb.bs.Learn.Learn(tb.ctx, tb.id, s)
		require.NoError(t, err)
		require.True(t, got.Learned, s)
	}
	_, err := tb.bs.Prep.Prepare(tb.ctx, tb.id, spells)
	require.NoError(t, err)
	return tb
}

func (tb *initiativeTable) initiative(t *testing.T) *buffsvc.Initiative {
	t.Helper()
	got, err := tb.bs.Buffs.GetInitiative(tb.ctx, tb.id)
	require.NoError(t, err)
	return got
}

func (tb *initiativeTable) top(t *testing.T) actionlog.Entry {
	t.Helper()
	entries, err := tb.bs.Log.List(tb.ctx, tb.id, 1)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	return entries[0]
}

func (tb *initiativeTable) countKind(t *testing.T, kind actionlog.Kind) int {
	t.Helper()
	entries, err := tb.bs.Log.List(tb.ctx, tb.id, 100)
	require.NoError(t, err)
	n := 0
	for _, e := range entries {
		if e.Kind == kind && !e.Voided() {
			n++
		}
	}
	return n
}

var hasteTerm = buffsvc.InitiativeModifier{Source: buffsvc.InitiativeEffect, Value: -2, Name: "Haste", SpellID: "haste"}

// ADR 0035 items 6.2 and 6.4 with ADR 0013 steps 2 and 4: a mage with a long
// sword (speed 5) and Haste on rolls 7: 7 + 5 − 2 = 10. Casting Magic
// Missile (1 segment) corrects the round to 7 + 1 − 2 = 6, no die rolled.
// Undo gives the weapon term and the rolled entry back; redo the correction;
// a reroll (d10 3) keeps the spell term: 3 + 1 − 2 = 2.
func TestInitiative_ADR0035Item6_MagicMissileWithHasteUndoRedoReroll(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{7, 4, 2, 3}, calls: map[string]int{}}
	tb := mageWith(t, dice, "magic_missile")
	sword, err := tb.bs.Items.Create(tb.ctx, tb.id, itemsvc.ItemInput{CatalogueCode: "long_sword", Equip: true})
	require.NoError(t, err)
	_, err = tb.bs.Cast.AddEffect(tb.ctx, tb.id, castsvc.AddEffectRequest{SpellID: "haste"})
	require.NoError(t, err)

	_, _, _, rolled, _, err := tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
	require.NoError(t, err)
	assert.Equal(t, []buffsvc.InitiativeModifier{
		{Source: buffsvc.InitiativeWeapon, Value: 5, ItemID: sword.Item.ID, Name: "Fixture long sword"}, hasteTerm,
	}, rolled.Modifiers)
	assert.Equal(t, 10, rolled.Total())

	_, err = tb.bs.Cast.Cast(tb.ctx, tb.id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	missile := buffsvc.InitiativeModifier{Source: buffsvc.InitiativeSpell, Value: 1, SpellID: "magic_missile", Name: "Magic Missile"}
	corrected := tb.initiative(t)
	assert.Equal(t, 7, corrected.D10)
	assert.Equal(t, []buffsvc.InitiativeModifier{missile, hasteTerm}, corrected.Modifiers)
	assert.Equal(t, 6, corrected.Total(), "d10 + 1 − 2")
	assert.Equal(t, actionlog.KindInitiativeCorrect, tb.top(t).Kind)

	first, err := tb.bs.Log.Undo(tb.ctx, tb.id, tb.top(t).ID)
	require.NoError(t, err)
	back := tb.initiative(t)
	assert.Equal(t, rolled.Modifiers, back.Modifiers, "undo: the weapon term back")
	assert.Equal(t, rolled.LogEntryID, back.LogEntryID, "undo: the rolled entry back")

	_, err = tb.bs.Log.Redo(tb.ctx, tb.id, first.ID)
	require.NoError(t, err)
	again := tb.initiative(t)
	assert.Equal(t, corrected.Modifiers, again.Modifiers, "redo: the correction back")
	assert.Equal(t, tb.top(t).ID, again.LogEntryID)

	rerolled, err := tb.bs.Buffs.RerollInitiative(tb.ctx, tb.id)
	require.NoError(t, err)
	assert.Equal(t, 3, rerolled.D10)
	assert.Equal(t, []buffsvc.InitiativeModifier{missile, hasteTerm}, rerolled.Modifiers, "the reroll keeps the spell term")
	assert.Equal(t, 2, rerolled.Total())
	assert.Equal(t, 4, dice.calls["die"], "the d10, the two missiles' d4 (3rd level), the reroll's d10")
}

// ADR 0035 item 6.2: a spell cast, then a wand used in the same round: the
// wand's +3 (Table 56) replaces the spell's term; one undo pops the use and
// its correction, back to the spell's correction.
func TestInitiative_ADR0035Item6_2_AWandAfterASpellReplacesItsTerm(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{6, 2, 3}, calls: map[string]int{}}
	tb := mageWith(t, dice, "magic_missile")
	wand, err := tb.bs.Items.Create(tb.ctx, tb.id, itemsvc.ItemInput{
		Name: localized.Of("Wand of frost"), Category: itemsvc.CategoryMagic, MagicKind: "wand",
		Behaviour: itemsvc.BehaviourCharges, Count: 5, MaxCount: 5,
	})
	require.NoError(t, err)
	_, _, _, _, _, err = tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
	require.NoError(t, err)
	_, err = tb.bs.Cast.Cast(tb.ctx, tb.id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	spell := tb.initiative(t)
	assert.Equal(t, 7, spell.Total(), "6 + 1")

	_, err = tb.bs.Items.UseItem(tb.ctx, tb.id, wand.Item.ID)
	require.NoError(t, err)
	used := tb.initiative(t)
	assert.Equal(t, []buffsvc.InitiativeModifier{
		{Source: buffsvc.InitiativeItem, Value: 3, ItemID: wand.Item.ID, Name: "Wand of frost", MagicKind: "wand"},
	}, used.Modifiers, "the wand's +3 replaces the spell term")
	assert.Equal(t, 9, used.Total())

	_, err = tb.bs.Log.Undo(tb.ctx, tb.id, tb.top(t).ID)
	require.NoError(t, err)
	assert.Equal(t, spell, tb.initiative(t), "one undo: the spell's correction")
	assert.Equal(t, actionlog.KindInitiativeCorrect, tb.top(t).Kind, "the spell's correction is on top again")
}

// ADR 0035 item 6.2: a failed cast (spell failure d100) took its casting
// time and still corrects; a cast at round 0, with no initiative, logs no
// initiative_correct.
func TestInitiative_ADR0035Item6_2_AFailedCastCorrectsARoundZeroCastDoesNot(t *testing.T) {
	// A 1st-level cleric of Wisdom 9 (Table 5: 20% failure) prepares Bless
	// (casting time 1); the d100 of 20 fails.
	dice := &attackDice{t: t, d100s: []int{50, 20}, dies: []int{4}, calls: map[string]int{}}
	tb := newInitiativeTable(t, dice, charactersvc.NewCharacter{
		Name: "Ansel", Class: "cleric", Race: "human", Str: 12, Dex: 10, Con: 13, Int: 10, Wis: 9, Cha: 11, HPMax: 7,
	})
	_, err := tb.bs.Prep.Prepare(tb.ctx, tb.id, []string{"bless"})
	require.NoError(t, err)

	res, err := tb.bs.Cast.Cast(tb.ctx, tb.id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	require.NotNil(t, res.Failure)
	assert.False(t, res.Failure.Failed)
	assert.Equal(t, 0, tb.countKind(t, actionlog.KindInitiativeCorrect), "round 0, no initiative: no correction")
	assert.Nil(t, tb.initiative(t))
	_, err = tb.bs.Log.Undo(tb.ctx, tb.id, tb.top(t).ID) // the slot back
	require.NoError(t, err)

	_, _, _, rolled, _, err := tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
	require.NoError(t, err)
	assert.Equal(t, 4, rolled.D10)
	failed, err := tb.bs.Cast.Cast(tb.ctx, tb.id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	require.True(t, failed.Failure.Failed)
	assert.Equal(t, 1, tb.countKind(t, actionlog.KindInitiativeCorrect), "a failed cast still corrects")
	got := tb.initiative(t)
	require.NotNil(t, got)
	assert.Equal(t, buffsvc.InitiativeSpell, got.Modifiers[0].Source)
	assert.Equal(t, "bless", got.Modifiers[0].SpellID)
}

// ADR 0035 item 6.2 with ADR 0033 step 15.1: a spell of a round (Armor,
// "1 rd.") adds nothing and reads end of round; the total is the d10 plus
// the effects (Haste −2); the next round's initiative has no end of round.
func TestInitiative_ADR0035Item6_2_ARoundSpellIsEndOfRoundUntilNextRound(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{8, 5}, calls: map[string]int{}}
	tb := mageWith(t, dice, "armor")
	_, err := tb.bs.Items.Create(tb.ctx, tb.id, itemsvc.ItemInput{CatalogueCode: "long_sword", Equip: true})
	require.NoError(t, err)
	_, err = tb.bs.Cast.AddEffect(tb.ctx, tb.id, castsvc.AddEffectRequest{SpellID: "haste"})
	require.NoError(t, err)
	_, _, _, _, _, err = tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
	require.NoError(t, err)

	_, err = tb.bs.Cast.Cast(tb.ctx, tb.id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	got := tb.initiative(t)
	assert.Equal(t, []buffsvc.InitiativeModifier{
		{Source: buffsvc.InitiativeSpell, SpellID: "armor", Name: "Armor", Timing: buffsvc.TimingEndOfRound}, hasteTerm,
	}, got.Modifiers)
	assert.Equal(t, 6, got.Total(), "d10 8 + effects (−2)")

	_, _, _, next, _, err := tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
	require.NoError(t, err)
	for _, m := range next.Modifiers {
		assert.NotEqual(t, buffsvc.TimingEndOfRound, m.Timing, "Next Round: no end of round")
	}
	assert.Equal(t, 5+5-2, next.Total(), "d10 5 + the sword 5 − Haste 2")
}

// ADR 0035 item 6 (DMG "Magical Weapon Speeds", ADR 0002 step 6): a +2/+1
// long sword (speed 5, lesser plus 1) with a melee "weapon becomes +3"
// buff: 5 − 3 = 2. A buff expiring on this advance doesn't count: 5 − 1 = 4.
func TestInitiative_ADR0035Item6_AWeaponBuffSpeedsTheSwordUntilItExpires(t *testing.T) {
	for _, tt := range []struct {
		name     string
		duration int
		speed    int
	}{
		{"the buff on", 2, 2},
		{"the buff expiring on this advance", 1, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dice := &attackDice{t: t, dies: []int{3}, calls: map[string]int{}}
			tb := newInitiativeTable(t, dice, charactersvc.NewCharacter{
				Name: "Bram", Class: "fighter", Race: "human", Str: 16, Dex: 16, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 10,
			})
			sword, err := tb.bs.Items.Create(tb.ctx, tb.id, itemsvc.ItemInput{CatalogueCode: "long_sword", MagicToHit: 2, MagicDamage: 1, Equip: true})
			require.NoError(t, err)
			three := 3
			_, err = tb.bs.Buffs.Activate(tb.ctx, tb.id, buffsvc.BuffEffect{
				Name: "Weapon +3", Duration: tt.duration, Effects: buffsvc.BuffEffects{WeaponBuff: &three, WeaponKind: buffsvc.WeaponMelee},
			})
			require.NoError(t, err)
			_, _, _, rolled, _, err := tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
			require.NoError(t, err)
			assert.Equal(t, []buffsvc.InitiativeModifier{
				{Source: buffsvc.InitiativeWeapon, Value: tt.speed, ItemID: sword.Item.ID, Name: "Fixture long sword"},
			}, rolled.Modifiers)
			assert.Equal(t, 3+tt.speed, rolled.Total())
		})
	}
}

// ADR 0035 item 6.2 (the gap of the scroll): Table 56's scroll row is the
// casting time of its spell, which an item doesn't hold: the term is not
// counted (timing spell_casting_time), never a computed +0.
func TestInitiative_ADR0035Item6_2_AScrollUsedIsNotCounted(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{8}, calls: map[string]int{}}
	tb := newInitiativeTable(t, dice, charactersvc.NewCharacter{
		Name: "Bram", Class: "fighter", Race: "human", Str: 16, Dex: 16, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 10,
	})
	_, err := tb.bs.Items.Create(tb.ctx, tb.id, itemsvc.ItemInput{CatalogueCode: "long_sword", Equip: true})
	require.NoError(t, err)
	scroll, err := tb.bs.Items.Create(tb.ctx, tb.id, itemsvc.ItemInput{
		Name: localized.Of("Scroll of fireball"), Category: itemsvc.CategoryMagic, MagicKind: "scroll",
		Behaviour: itemsvc.BehaviourStock, Count: 1,
	})
	require.NoError(t, err)
	_, _, _, rolled, _, err := tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
	require.NoError(t, err)
	assert.Equal(t, 13, rolled.Total(), "8 + the sword's 5")

	_, err = tb.bs.Items.UseItem(tb.ctx, tb.id, scroll.Item.ID)
	require.NoError(t, err)
	got := tb.initiative(t)
	assert.Equal(t, []buffsvc.InitiativeModifier{
		{Source: buffsvc.InitiativeItem, ItemID: scroll.Item.ID, Name: "Scroll of fireball", MagicKind: "scroll", Timing: buffsvc.TimingSpellCastingTime},
	}, got.Modifiers)
	assert.Equal(t, 8, got.Total())
}

// ADR 0035 items 6.2 and 6.4 through the real services, the cycle's two sides
// together: an item used out of combat records no initiative_correct; a
// reroll right after a cast is allowed (the correction is the last entry);
// a second item used replaces the first's term and keeps the d10 and the
// effects recorded at the roll.
func TestInitiative_ADR0035Item6_2_ItemOutOfCombatCastThenRerollTwoItems(t *testing.T) {
	dice := &attackDice{t: t, dies: []int{7, 4, 2, 3}, calls: map[string]int{}}
	tb := mageWith(t, dice, "magic_missile")
	wand, err := tb.bs.Items.Create(tb.ctx, tb.id, itemsvc.ItemInput{
		Name: localized.Of("Wand of frost"), Category: itemsvc.CategoryMagic, MagicKind: "wand",
		Behaviour: itemsvc.BehaviourCharges, Count: 5, MaxCount: 5,
	})
	require.NoError(t, err)
	potion, err := tb.bs.Items.Create(tb.ctx, tb.id, itemsvc.ItemInput{
		Name: localized.Of("Potion of healing"), Category: itemsvc.CategoryMagic, MagicKind: "potion",
		Behaviour: itemsvc.BehaviourStock, Count: 2,
	})
	require.NoError(t, err)
	_, err = tb.bs.Cast.AddEffect(tb.ctx, tb.id, castsvc.AddEffectRequest{SpellID: "haste"})
	require.NoError(t, err)

	_, err = tb.bs.Items.UseItem(tb.ctx, tb.id, wand.Item.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, tb.countKind(t, actionlog.KindInitiativeCorrect), "round 0, no initiative: no correction")
	assert.Nil(t, tb.initiative(t))

	_, _, _, _, _, err = tb.bs.Buffs.AdvanceRound(tb.ctx, tb.id)
	require.NoError(t, err)
	_, err = tb.bs.Cast.Cast(tb.ctx, tb.id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	rerolled, err := tb.bs.Buffs.RerollInitiative(tb.ctx, tb.id)
	require.NoError(t, err, "a reroll right after a cast is allowed")
	assert.Equal(t, 3, rerolled.D10)

	_, err = tb.bs.Items.UseItem(tb.ctx, tb.id, wand.Item.ID)
	require.NoError(t, err)
	_, err = tb.bs.Items.UseItem(tb.ctx, tb.id, potion.Item.ID)
	require.NoError(t, err)
	got := tb.initiative(t)
	assert.Equal(t, 3, got.D10, "the correction keeps the die")
	assert.Equal(t, []buffsvc.InitiativeModifier{
		{Source: buffsvc.InitiativeItem, Value: 4, ItemID: potion.Item.ID, Name: "Potion of healing", MagicKind: "potion"}, hasteTerm,
	}, got.Modifiers, "the second item replaces the first's term and keeps the effects")
	assert.Equal(t, 5, got.Total(), "3 + 4 − 2")
	assert.Equal(t, 4, dice.calls["die"])
}
