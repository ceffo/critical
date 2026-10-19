package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// ADR 0033 steps 10, 11 and 13 through the real services on the fixture's
// tables: the AC follows its inputs with no entry of its own. A fighter of
// Dexterity 16 (PHB Table 2: -2) is AC 8 unarmoured; wearing chain mail (AC
// 5) and a small shield (+1) makes it 2, each one equipment_state entry and
// nothing else; the Armor spell's base_ac 6 gives nothing over the armour;
// taking the armour off makes it the base (6 - 1 - 2 = 3); Protection from
// Evil (ac -2) makes it 1, its dismissal 3 again. The override is one
// ac_override entry, undone and redone, the derived AC beside it.
func TestAC_Steps10_13_FollowsItsInputsAndTheOverrideUndoes(t *testing.T) {
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
	top := func() actionlog.Entry {
		t.Helper()
		es, err := bs.Log.List(ctx, id, 1)
		require.NoError(t, err)
		require.Len(t, es, 1)
		return es[0]
	}
	require.Equal(t, 8, ac().Value, "10, Dexterity 16 -2")

	_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{
		Name: localized.Of("Chain mail"), Category: itemsvc.CategoryArmour, Behaviour: itemsvc.BehaviourStock, Count: 1,
		BaseAC: 5, SkillColumn: "metal", ArmorRow: "chain",
	})
	require.NoError(t, err)
	_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{
		Name: localized.Of("Small shield"), Category: itemsvc.CategoryShield, Behaviour: itemsvc.BehaviourStock, Count: 1,
		ShieldKind: "small", ACMelee: 1, ACMissile: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, 8, ac().Value, "carried pieces give nothing")
	_, err = bs.Items.SetState(ctx, id, "chain_mail", itemsvc.StateWorn, false)
	require.NoError(t, err)
	_, err = bs.Items.SetState(ctx, id, "small_shield", itemsvc.StateWorn, false)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindEquipmentState, top().Kind, "wearing is the only entry")
	assert.Equal(t, 2, ac().Value, "5, shield -1, Dexterity -2")

	_, err = bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Armor", Effects: buffsvc.BuffEffects{BaseAC: new(6)}})
	require.NoError(t, err)
	assert.Equal(t, 2, ac().Value, "base_ac gives nothing over a worn armour")
	_, err = bs.Items.SetState(ctx, id, "chain_mail", itemsvc.StateCarried, false)
	require.NoError(t, err)
	assert.Equal(t, 3, ac().Value, "the Armor spell's 6, shield -1, Dexterity -2")

	buffs, err := bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Protection from Evil", Effects: buffsvc.BuffEffects{AC: new(-2)}})
	require.NoError(t, err)
	assert.Equal(t, 1, ac().Value)
	_, err = bs.Buffs.Dismiss(ctx, id, buffs[len(buffs)-1].ID)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindBuffChange, top().Kind)
	assert.Equal(t, 3, ac().Value)

	set, err := bs.AC.SetOverride(ctx, id, acsvc.Override{Value: 4, Label: "Shield spell"})
	require.NoError(t, err)
	assert.Equal(t, &acsvc.Override{Value: 4, Label: "Shield spell"}, set.Override)
	assert.Equal(t, 3, set.Value, "the derived AC beside it")
	e := top()
	assert.Equal(t, actionlog.KindACOverride, e.Kind)
	_, err = bs.Log.Undo(ctx, id, e.ID)
	require.NoError(t, err)
	assert.Nil(t, ac().Override, "the undo clears it")
	_, err = bs.Log.Redo(ctx, id, e.ID)
	require.NoError(t, err)
	assert.Equal(t, &acsvc.Override{Value: 4, Label: "Shield spell"}, ac().Override, "the redo sets it again")
	cleared, err := bs.AC.ClearOverride(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, cleared.Override)
	assert.Equal(t, actionlog.KindACOverride, top().Kind)
}

// ADR 0033 step 20 through the real services: a mage that wears chain mail and a shield and
// wields a long sword is warned on read, on the pieces (itemsvc) and on the AC (armour and shield
// only), and nothing is refused or logged beyond the moves; taking the armour off clears it.
func TestClassWarnings_Step20_ServedOnReadNeverRefused(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Ilya", Class: "mage", Race: "human", Str: 9, Dex: 12, Con: 10, Int: 16, Wis: 10, Cha: 8, HPMax: 4,
	})
	require.NoError(t, err)
	_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{
		Name: localized.Of("Chain mail"), Category: itemsvc.CategoryArmour, Behaviour: itemsvc.BehaviourStock, Count: 1,
		BaseAC: 5, SkillColumn: "metal", ArmorRow: "chain", Equip: true,
	})
	require.NoError(t, err)
	_, err = bs.Items.Create(ctx, id, itemsvc.ItemInput{
		Name: localized.Of("Sword"), Category: itemsvc.CategoryWeapon, Behaviour: itemsvc.BehaviourStock, Count: 1,
		WeaponKind: "melee", WeaponType: "long_sword", DamageTypes: []string{"s"}, DamageSM: "1d8", DamageL: "1d12", Equip: true,
	})
	require.NoError(t, err)

	got, err := bs.Items.Warnings(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []itemsvc.Warning{{ItemID: "chain_mail", Code: "armour_not_allowed"}, {ItemID: "sword", Code: "weapon_not_allowed"}}, got)
	a, err := bs.AC.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []itemsvc.Warning{{ItemID: "chain_mail", Code: "armour_not_allowed"}}, a.Warnings, "the AC card carries the armour's only")
	assert.Equal(t, 5, a.Value, "the armour still counts: nothing is changed")

	_, err = bs.Items.SetState(ctx, id, "chain_mail", itemsvc.StateCarried, false)
	require.NoError(t, err)
	a, err = bs.AC.Get(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, a.Warnings)
}
