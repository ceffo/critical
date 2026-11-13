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
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0037 step 9 through the real services: a level 1 fighter wielding a long sword saves the
// long sword's specialization pick (2 slots, one save, one proficiencies entry); the sword reads
// specialized and the attack takes ADR 0030 step 12's +1 to hit and +2 damage (modifier
// specialization) and step 11's specialist rate (3/2 at level 1), with no stored flag. Undoing
// the save (step 13) takes the pick back, and with it every effect: the sword is no longer
// specialized, the attack has no specialization modifier and the rate is 1/1.
func TestResolveAttack_Step9_SpecializationFromThePick(t *testing.T) {
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
	sword, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: "long_sword", Equip: true})
	require.NoError(t, err)
	require.False(t, sword.Item.Specialized)

	type effects struct {
		specialized bool
		spec        []combatsvc.Modifier
		damage      int
		rate        combatsvc.AttackRateView
	}
	read := func() effects {
		items, err := bs.Items.List(ctx, id)
		require.NoError(t, err)
		form, err := bs.Combat.GetAttackForm(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, form.Preview)
		require.NotNil(t, form.AttackRate)
		var e effects
		for _, it := range items {
			if it.ID == sword.Item.ID {
				e.specialized = it.Specialized
			}
		}
		for _, m := range form.Preview.Modifiers {
			if m.Kind == combatsvc.ModSpecialization {
				e.spec = append(e.spec, m)
			}
		}
		e.damage = form.Preview.DamageBonus
		e.rate = *form.AttackRate
		e.rate.ThisRound = 0
		return e
	}
	plain := read()
	assert.False(t, plain.specialized)
	assert.Empty(t, plain.spec)
	assert.Equal(t, combatsvc.AttackRateView{Attacks: 1, Rounds: 1}, plain.rate)

	before, _, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	_, err = bs.Skills.SetProficiencies(ctx, id, skillsvc.Edit{Picks: skills.Picks{Weapon: []skills.WeaponPick{{Code: "long_sword", Slots: 2}}}})
	require.NoError(t, err)
	top, _, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	assert.Greater(t, top, before, "the save is one entry")

	spec := read()
	assert.True(t, spec.specialized, "derived from the pick")
	assert.Equal(t, []combatsvc.Modifier{{Kind: combatsvc.ModSpecialization, Value: 1}}, spec.spec)
	assert.Equal(t, plain.damage+2, spec.damage, "+2 damage")
	assert.Equal(t, combatsvc.AttackRateView{Attacks: 3, Rounds: 2}, spec.rate, "Table 35's specialist rate")

	_, err = bs.Log.Undo(ctx, id, top)
	require.NoError(t, err)
	undone := read()
	assert.Equal(t, plain, undone, "undo takes the pick and every effect back")
	after, _, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, after, "one entry popped")
}

// ADR 0037 step 21.5 through the real services: a level 1 fighter specialized in the long sword
// (2 slots) with a plain broad sword pick, both wielded from their catalogue rows: the long sword's
// attack takes the specialization's +1 to hit (modifier specialization), the broad sword's none
// (a long_sword or other_melee type pick covered both before), and neither takes the
// non-proficiency penalty, each being picked by its code.
func TestResolveAttack_Step21_5_SpecializationByCode(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Bram", Class: "fighter", Race: "human", Str: 16, Dex: 16, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 10,
		Proficiencies: skills.Picks{Weapon: []skills.WeaponPick{{Code: "long_sword", Slots: 2}, {Code: "broad_sword", Slots: 1}}},
	})
	require.NoError(t, err)
	long, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: "long_sword", Equip: true})
	require.NoError(t, err)
	broad, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{CatalogueCode: "broad_sword"})
	require.NoError(t, err)
	_, err = bs.Items.SetState(ctx, id, broad.Item.ID, itemsvc.StateWielded, false)
	require.NoError(t, err)

	kinds := func(weaponID string) []combatsvc.ModifierKind {
		res, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{
			WeaponID: weaponID, TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium,
		})
		require.NoError(t, err)
		var out []combatsvc.ModifierKind
		for _, m := range res.Modifiers {
			out = append(out, m.Kind)
		}
		return out
	}
	items, err := bs.Items.List(ctx, id)
	require.NoError(t, err)
	specialized := map[string]bool{}
	for _, it := range items {
		specialized[it.ID] = it.Specialized
	}
	assert.Equal(t, map[string]bool{long.Item.ID: true, broad.Item.ID: false}, specialized)
	assert.Contains(t, kinds(long.Item.ID), combatsvc.ModSpecialization)
	assert.NotContains(t, kinds(broad.Item.ID), combatsvc.ModSpecialization, "the broad sword is another weapon")
	assert.NotContains(t, kinds(broad.Item.ID), combatsvc.ModNonproficient, "picked by its code")
}
