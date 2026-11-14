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
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

func ptr[T any](v T) *T { return &v }

// buffTable is the seeded character holding a stored potion (3 doses) with the given abilities'
// effects, written straight to the store, past the editor's validation, so a test can hold what
// the editor refuses.
func buffTable(t *testing.T, name localized.Text, effects []itemsvc.Effect, dies ...int) (*composition.BackingServices, context.Context, int64) {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	cfg := fixtureConfig()
	cfg.Dice = &attackDice{t: t, dies: dies, calls: map[string]int{}} // a buff rolls nothing: only the dies a test hands out
	bs, err := composition.Build(ctx, composition.MemoryStorage(store), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id := rootCharacter(t, bs)
	require.NoError(t, store.Items().Insert(ctx, id, itemsvc.Item{
		ID: "potion_hero", Position: 90, Name: localized.Of("Potion of heroism"), Type: "potion", Category: itemsvc.CategoryMagic,
		MagicKind: itemsvc.MagicPotion, State: itemsvc.StateCarried, Count: 3, Behaviour: itemsvc.BehaviourStock, MaxCount: 3,
		Abilities: []itemsvc.Ability{{ID: "use", Name: name, Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}, Effects: effects}},
	}))
	return bs, ctx, id
}

func activeBuffs(t *testing.T, bs *composition.BackingServices, ctx context.Context, id int64) []buffsvc.BuffEffect {
	t.Helper()
	active, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	return active
}

var heroism = itemsvc.Effect{
	Kind: itemsvc.EffectBuff, Name: "Heroism", Rounds: 10,
	Buff: buffsvc.BuffEffects{ToHit: ptr(1), SaveBonus: ptr(1)},
}

// ADR 0049 steps 10 and 12: a buff effect puts a timed buff on the list through buffsvc, the same
// row a hand-made buff of the same name, rounds and effects is: the same fields, the round it
// started in, no spell and no charges. Nothing is rolled and the aggregate folds it.
func TestUseAbility_ADR0049Step12_BuffIsTheSameRowAsAHandMadeOne(t *testing.T) {
	bs, ctx, id := buffTable(t, nil, []itemsvc.Effect{heroism})
	cctx, changes := actionlog.WithChanges(ctx)
	res, err := bs.Items.UseAbility(cctx, id, "potion_hero", "use")
	require.NoError(t, err)
	assert.Contains(t, changes.Keys(), "buffs", "the answer tells the web the buff list moved")
	require.Len(t, res.Effects, 1)
	require.NotNil(t, res.Effects[0].Buff)
	assert.Equal(t, "Heroism", res.Effects[0].Buff.Name)
	assert.Equal(t, 10, res.Effects[0].Buff.Rounds)
	assert.Equal(t, 2, itemOf(t, bs, ctx, id, "potion_hero").Count, "a dose is spent")

	made := activeBuffs(t, bs, ctx, id)
	require.Len(t, made, 1)
	assert.Equal(t, res.Effects[0].Buff.ID, made[0].ID)

	// The same buff entered by hand, on a second character-less path: Activate.
	_, err = bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Hand heroism", Duration: 10, Effects: heroism.Buff})
	require.NoError(t, err)
	both := activeBuffs(t, bs, ctx, id)
	require.Len(t, both, 2)
	byName := func(n string) buffsvc.BuffEffect {
		for _, b := range both {
			if b.Name == n {
				return b
			}
		}
		require.Failf(t, "buff not found", "%s", n)
		return buffsvc.BuffEffect{}
	}
	item, hand := byName("Heroism"), byName("Hand heroism")
	item.ID, item.Name, hand.ID, hand.Name = "", "", "", ""
	assert.Equal(t, hand, item, "the same entry kind of row: rounds, effects, start round, charges")

	m, err := bs.Buffs.Aggregate(ctx, id, buffsvc.BaseStrength{StrScore: buffsvc.StrScore{Score: 10}}, buffsvc.WeaponNone)
	require.NoError(t, err)
	assert.Equal(t, 2, m.SaveBonus, "both rows fold into the aggregate, as any buff")

	entries, err := bs.Log.List(ctx, id, 2)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindBuffChange, entries[0].Kind)
	assert.Equal(t, actionlog.KindItemUse, entries[1].Kind, "the use is one entry; the buff has none of its own")
}

// ADR 0049 step 12 and ADR 0004 step 6: a buff of the same name is renewed, not stacked: a second
// potion in a later round starts the buff again with its full rounds.
func TestUseAbility_ADR0049Step12_ABuffOfTheSameNameIsRenewed(t *testing.T) {
	bs, ctx, id := buffTable(t, nil, []itemsvc.Effect{heroism}, 4, 5) // the two Next Rounds roll the initiative die
	_, err := bs.Items.UseAbility(ctx, id, "potion_hero", "use")
	require.NoError(t, err)
	_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	_, _, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 8, activeBuffs(t, bs, ctx, id)[0].Duration, "two rounds elapsed")

	_, err = bs.Items.UseAbility(ctx, id, "potion_hero", "use")
	require.NoError(t, err)
	active := activeBuffs(t, bs, ctx, id)
	require.Len(t, active, 1, "renewed, not a second row")
	assert.Equal(t, 10, active[0].Duration)
	assert.Equal(t, 2, active[0].StartRound, "it starts again in the round of the second use")
}

// ADR 0049 step 10: the buff's name is its own, else its ability's, else the item's.
func TestUseAbility_ADR0049Step10_BuffNameDefaultsToTheAbilitiesThenTheItems(t *testing.T) {
	unnamed := itemsvc.Effect{Kind: itemsvc.EffectBuff, Rounds: 5, Buff: buffsvc.BuffEffects{AC: ptr(-2)}}
	bs, ctx, id := buffTable(t, localized.Of("Drink the potion"), []itemsvc.Effect{unnamed})
	_, err := bs.Items.UseAbility(ctx, id, "potion_hero", "use")
	require.NoError(t, err)
	assert.Equal(t, "Drink the potion", activeBuffs(t, bs, ctx, id)[0].Name)

	bs, ctx, id = buffTable(t, nil, []itemsvc.Effect{unnamed})
	_, err = bs.Items.UseAbility(ctx, id, "potion_hero", "use")
	require.NoError(t, err)
	assert.Equal(t, "Potion of heroism", activeBuffs(t, bs, ctx, id)[0].Name)
}

// ADR 0049 step 12: one undo pops the use: the dose comes back and the buff leaves the list; two
// buff effects of one use are one snapshot of the list, so the one undo pops both; redo puts them
// back.
func TestUseAbility_ADR0049Step12_OneUndoPopsTheBuffsWithTheUse(t *testing.T) {
	bs, ctx, id := buffTable(t, nil, []itemsvc.Effect{
		heroism, {Kind: itemsvc.EffectBuff, Name: "Speed", Rounds: 5, Buff: buffsvc.BuffEffects{Initiative: ptr(-2)}},
	})
	_, err := bs.Items.UseAbility(ctx, id, "potion_hero", "use")
	require.NoError(t, err)
	require.Len(t, activeBuffs(t, bs, ctx, id), 2)
	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)

	_, err = bs.Log.Undo(ctx, id, entries[0].ID)
	require.NoError(t, err)
	assert.Empty(t, activeBuffs(t, bs, ctx, id), "both buffs are popped")
	assert.Equal(t, 3, itemOf(t, bs, ctx, id, "potion_hero").Count, "and the dose is back")

	redo, err := bs.Log.RedoTarget(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, redo)
	_, err = bs.Log.Redo(ctx, id, redo.ID)
	require.NoError(t, err)
	assert.Len(t, activeBuffs(t, bs, ctx, id), 2, "redo puts them back")
	assert.Equal(t, 2, itemOf(t, bs, ctx, id, "potion_hero").Count)
}

// ADR 0049 step 12: a failure in any effect rolls the whole use back: the first buff was added,
// then a second whose rounds buffsvc refuses (a stored value the editor would refuse): no buff,
// the dose is back and nothing is logged.
func TestUseAbility_ADR0049Step12_AFailingEffectRollsBackTheBuffAndTheLimit(t *testing.T) {
	bs, ctx, id := buffTable(t, nil, []itemsvc.Effect{
		heroism, {Kind: itemsvc.EffectBuff, Name: "Broken", Rounds: -1},
	})
	before, err := bs.Log.List(ctx, id, 100)
	require.NoError(t, err)

	_, err = bs.Items.UseAbility(ctx, id, "potion_hero", "use")
	require.Error(t, err)
	assert.Empty(t, activeBuffs(t, bs, ctx, id), "the first buff is rolled back")
	assert.Equal(t, 3, itemOf(t, bs, ctx, id, "potion_hero").Count, "the dose is back")
	after, err := bs.Log.List(ctx, id, 100)
	require.NoError(t, err)
	assert.Len(t, after, len(before), "nothing is logged")
}
