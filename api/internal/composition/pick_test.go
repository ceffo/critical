package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0033 steps 4, 23 and 24 through the real services on the fixture's
// catalogue: a fighter picks the long bow, equipped at once, and twelve
// sheaf arrows (no weapon pick: the -2 of ADR 0037 step 8); the bow has no dice of its own, so the attack rolls the
// arrow's 1d8. Picking chain mail equipped at once is one undo: the undo
// takes the piece away, add and move together.
func TestPick_Steps4_24_PickedBowShootsTheArrowsDiceAndEquipNowUndoesAsOne(t *testing.T) {
	ctx := context.Background()
	dice := &attackDice{t: t, d20s: []int{19}, d100s: []int{96, 77}, dies: []int{6}, calls: map[string]int{}}
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

	bow, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: "long_bow", Equip: true})
	require.NoError(t, err)
	assert.Equal(t, itemsvc.StateWielded, bow.Item.State)
	assert.Equal(t, "long_bow", bow.Item.CatalogueCode)
	assert.Empty(t, bow.Item.DamageSM, "a launcher has no dice of its own")
	arrows, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: "sheaf_arrow", Count: 12})
	require.NoError(t, err)
	assert.Equal(t, 12, arrows.Item.Count)

	// ADR 0037 step 8: Bram was created with no weapon pick, so the long bow takes the warrior's
	// -2, the same in the attack form's preview and in the roll.
	nonprof := combatsvc.Modifier{Kind: combatsvc.ModNonproficient, Detail: "long_bow", Value: -2}
	form, err := bs.Combat.GetAttackForm(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, form.Preview)
	assert.Contains(t, form.Preview.Modifiers, nonprof)
	res, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{
		WeaponID: bow.Item.ID, AmmoItemID: arrows.Item.ID, TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium,
	})
	require.NoError(t, err)
	require.True(t, res.Hit)
	assert.Equal(t, form.Preview.Modifiers, res.Modifiers, "the preview is the roll's breakdown")
	assert.Equal(t, form.Preview.EffectiveThac0, res.EffectiveThac0)
	assert.Equal(t, "1d8", res.DamageDice, "the arrow's dice")
	assert.Equal(t, 6, res.TotalDamage)
	assert.Equal(t, map[string]int{"d20": 1, "d100": 2, "die": 1}, dice.calls)

	mail, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: "chain_mail", Equip: true})
	require.NoError(t, err)
	assert.Equal(t, itemsvc.StateWorn, mail.Item.State)
	top, ok, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	_, err = bs.Log.Undo(ctx, id, top)
	require.NoError(t, err)
	items, err := bs.Items.List(ctx, id)
	require.NoError(t, err)
	for _, it := range items {
		assert.NotEqual(t, mail.Item.ID, it.ID, "one undo takes the picked, equipped mail away")
	}
	assert.Len(t, items, 2)
}
