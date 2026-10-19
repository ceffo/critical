package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0033 step 17 through the real services: the editor writes powers, and
// they join the buffs' aggregate (ADR 0016 step 11) and the derived AC (ADR
// 0033 step 10), named by their piece (ADR 0016 step 14). A fighter of
// Dexterity 16 (-2) wears chain mail +2 (AC 5, the "+N" is the magic power
// ac -2): AC 1, an armour_magic row of the chain mail. A ring edited into a
// ring of protection +1 (ac -1 without magic armour, save +1) is one
// item_edit entry whose snapshot carries the powers: its save bonus counts,
// its AC does not (magic armour, step 12). Editing the +N away (another
// item_edit) makes the ring's AC count: 5 - 1 - 2 = 2. Popping that edit gives
// the +N back (AC 1), the redo takes it off again.
func TestEditor_ADR0033Step17_PowersJoinTheAggregateAndTheACAndPop(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Bram", Class: "fighter", Race: "human", Str: 16, Dex: 16, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 10,
	})
	require.NoError(t, err)
	ac := func() *acsvc.AC {
		t.Helper()
		a, err := bs.AC.Get(ctx, id)
		require.NoError(t, err)
		return a
	}
	row := func(a *acsvc.AC, term acsvc.Term, itemID string) *acsvc.Row {
		t.Helper()
		for _, r := range a.Rows {
			if r.Term == term && r.ItemID == itemID {
				return &r
			}
		}
		return nil
	}
	top := func() actionlog.Entry {
		t.Helper()
		es, err := bs.Log.List(ctx, id, 1)
		require.NoError(t, err)
		require.Len(t, es, 1)
		return es[0]
	}
	saves := func() buffsvc.BuffModifiers {
		t.Helper()
		m, err := bs.Buffs.Aggregate(ctx, id, buffsvc.BaseStrength{StrScore: buffsvc.StrScore{Score: 16}}, buffsvc.WeaponNone)
		require.NoError(t, err)
		return m
	}

	_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{
		Name: localized.Of("Chain mail +2"), Category: itemsvc.CategoryArmour, Behaviour: itemsvc.BehaviourStock, Count: 1,
		BaseAC: 5, SkillColumn: "metal", ArmorRow: "chain", Powers: []itemsvc.Power{itemsvc.MagicPower(2)}, Equip: true,
	})
	require.NoError(t, err)
	a := ac()
	assert.Equal(t, 1, a.Value, "5, armour magic -2, Dexterity -2")
	if r := row(a, acsvc.TermArmourMagic, "chain_mail_2"); assert.NotNil(t, r, "the +N is the armour magic row, named by the piece") {
		assert.Equal(t, -2, r.Value)
	}

	_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{
		Name: localized.Of("Ring"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicRing,
		Behaviour: itemsvc.BehaviourStock, Count: 1, Equip: true,
	})
	require.NoError(t, err)
	ring := []itemsvc.Power{{Effects: buffsvc.BuffEffects{AC: new(-1), SaveBonus: new(1), WithoutMagicArmour: true}}}
	edited, err := bs.Items.Update(ctx, id, "ring", itemsvc.ItemPatch{Powers: &ring})
	require.NoError(t, err)
	require.Len(t, edited.Item.Powers, 1)
	assert.Equal(t, "power_1", edited.Item.Powers[0].ID, "a power sent without an id gets one")
	e := top()
	assert.Equal(t, actionlog.KindItemEdit, e.Kind, "a power change is the piece's item_edit")
	assert.Contains(t, string(e.Payload), `"field":"powers"`)

	m := saves()
	assert.Equal(t, 1, m.SaveBonus, "the ring's save bonus joins the aggregate")
	assert.Equal(t, []buffsvc.Contribution{{Buff: "Ring", ItemID: "ring", Value: 1}}, m.SaveBonusFrom, "named by its piece")
	a = ac()
	assert.Equal(t, 1, a.Value, "the ring's AC does not add over magic armour")
	if r := row(a, acsvc.TermProtection, "ring"); assert.NotNil(t, r) {
		assert.Equal(t, acsvc.OffMagicArmour, r.Off)
	}

	none := []itemsvc.Power{}
	_, err = bs.Items.Update(ctx, id, "chain_mail_2", itemsvc.ItemPatch{Powers: &none})
	require.NoError(t, err)
	assert.Equal(t, 2, ac().Value, "5, ring -1, Dexterity -2")
	edit := top()
	assert.Equal(t, actionlog.KindItemEdit, edit.Kind)

	_, err = bs.Log.Undo(ctx, id, edit.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, ac().Value, "popping the edit gives the +N back")
	got, err := bs.Items.List(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []itemsvc.Power{itemsvc.MagicPower(2)}, got[0].Powers)
	_, err = bs.Log.Redo(ctx, id, edit.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, ac().Value, "the redo takes it off again")
}

// ADR 0033 step 17, popped through the real ledger: a worn plain chain mail
// (AC 5, Dexterity -2: 3) edited to hold the magic power ac -2 is AC 1 with an
// armour_magic row; popping the edit gives AC 3 back. Editing it to gear drops
// the magic power (one powers change); popping that gives back the category
// and the magic power. A ledger entry's snapshot from before powers (no
// "powers" key) keeps the powers the piece holds.
func TestEditor_ADR0033Step17_PatchPopAndOldSnapshots(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Bram", Class: "fighter", Race: "human", Str: 16, Dex: 16, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 10,
	})
	require.NoError(t, err)
	acValue := func() int {
		t.Helper()
		a, err := bs.AC.Get(ctx, id)
		require.NoError(t, err)
		return a.Value
	}
	top := func() actionlog.Entry {
		t.Helper()
		es, err := bs.Log.List(ctx, id, 1)
		require.NoError(t, err)
		require.Len(t, es, 1)
		return es[0]
	}
	powers := func() []itemsvc.Power {
		t.Helper()
		got, err := bs.Items.List(ctx, id)
		require.NoError(t, err)
		require.Len(t, got, 1)
		return got[0].Powers
	}

	_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{
		Name: localized.Of("Chain mail"), Category: itemsvc.CategoryArmour, Behaviour: itemsvc.BehaviourStock, Count: 1,
		BaseAC: 5, SkillColumn: "metal", ArmorRow: "chain", Equip: true,
	})
	require.NoError(t, err)
	require.Equal(t, 3, acValue())

	plus2 := []itemsvc.Power{itemsvc.MagicPower(2)}
	_, err = bs.Items.Update(ctx, id, "chain_mail", itemsvc.ItemPatch{Powers: &plus2})
	require.NoError(t, err)
	a, err := bs.AC.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 1, a.Value)
	found := false
	for _, r := range a.Rows {
		if r.Term == acsvc.TermArmourMagic && r.ItemID == "chain_mail" {
			found = true
			assert.Equal(t, -2, r.Value)
		}
	}
	assert.True(t, found, "the +N is an armour_magic row of the piece")
	_, err = bs.Log.Undo(ctx, id, top().ID)
	require.NoError(t, err)
	assert.Equal(t, 3, acValue(), "popped: the AC is back")
	assert.Empty(t, powers())
	_, err = bs.Log.Redo(ctx, id, top().ID)
	require.NoError(t, err)
	assert.Equal(t, 1, acValue())

	gear := itemsvc.CategoryGear
	_, err = bs.Items.Update(ctx, id, "chain_mail", itemsvc.ItemPatch{Category: &gear})
	require.NoError(t, err)
	e := top()
	assert.Equal(t, actionlog.KindItemEdit, e.Kind)
	assert.Contains(t, string(e.Payload), `"field":"powers"`, "dropping the +N is a powers change")
	assert.Empty(t, powers())
	_, err = bs.Log.Undo(ctx, id, e.ID)
	require.NoError(t, err)
	got, err := bs.Items.List(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, itemsvc.CategoryArmour, got[0].Category)
	assert.Equal(t, plus2, got[0].Powers, "popped: the category and the magic power are back")

	// An old snapshot (no "powers" key) restores the rest and keeps the powers held now.
	old := snaptest.EnvID("chain_mail", `{"count":1,"replace":true,"item":{"position":0,"name":{"en":"Chain mail"},"namePartial":true,"type":"armour",
		"behaviour":"stock","magicalBonus":0,"ammoKind":"","maxCount":1,"effectDice":"","effectSave":"","notes":"old"}}`)
	require.NoError(t, bs.Items.Module().Restorer(itemsvc.StateKeyItems).Restore(ctx, id, old))
	got, err = bs.Items.List(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "old", got[0].Notes, "the old snapshot restored the item")
	assert.Equal(t, plus2, got[0].Powers, "and left its powers alone")
}
