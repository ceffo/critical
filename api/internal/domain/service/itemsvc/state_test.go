package itemsvc_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

var (
	chain = itemsvc.Item{
		ID: "chain_mail", Position: 0, Name: localized.Of("Chain mail"), Category: itemsvc.CategoryArmour,
		State: itemsvc.StateCarried, Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1,
		BaseAC: 5, SkillColumn: "metal", ArmorRow: "chain",
	}
	leather = itemsvc.Item{
		ID: "leather", Position: 1, Name: localized.Of("Leather"), Category: itemsvc.CategoryArmour,
		State: itemsvc.StateWorn, Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1,
		BaseAC: 8, SkillColumn: "leather", ArmorRow: "leather",
	}
	sword = itemsvc.Item{
		ID: "longsword", Position: 2, Name: localized.Of("Long Sword +2"), Category: itemsvc.CategoryWeapon,
		State: itemsvc.StateCarried, Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1,
		DamageSM: "1d8", DamageL: "1d12", MagicToHit: 2, MagicDamage: 2, WeaponKind: "melee", WeaponType: "long_sword",
	}
)

// ADR 0033 step 6: every piece may be carried; body armour, a shield and a
// ring or miscellaneous magic piece may be worn; only a weapon is wielded.
// Ammunition, gear, potions, scrolls, wands, rods and staves are carried
// only.
func TestAllows_Step6_StateByCategory(t *testing.T) {
	worn, wielded := itemsvc.StateWorn, itemsvc.StateWielded
	tests := []struct {
		c    itemsvc.Category
		kind string
		s    itemsvc.State
		want bool
	}{
		{itemsvc.CategoryArmour, "", worn, true},
		{itemsvc.CategoryShield, "", worn, true},
		{itemsvc.CategoryMagic, itemsvc.MagicRing, worn, true},
		{itemsvc.CategoryMagic, itemsvc.MagicMisc, worn, true},
		{itemsvc.CategoryMagic, itemsvc.MagicWand, worn, false},
		{itemsvc.CategoryMagic, itemsvc.MagicPotion, worn, false},
		{itemsvc.CategoryMagic, itemsvc.MagicStaff, worn, false},
		{itemsvc.CategoryWeapon, "", worn, false},
		{itemsvc.CategoryWeapon, "", wielded, true},
		{itemsvc.CategoryArmour, "", wielded, false},
		{itemsvc.CategoryAmmunition, "", worn, false},
		{itemsvc.CategoryAmmunition, "", wielded, false},
		{itemsvc.CategoryGear, "", worn, false},
		{itemsvc.CategoryGear, "", itemsvc.StateCarried, true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, itemsvc.Allows(tt.c, tt.kind, tt.s), "%s %s %s", tt.c, tt.kind, tt.s)
	}
}

// ADR 0033 step 6: Equip moves a weapon to wielded, a piece that may be worn
// to worn; a piece carried only has no equip state.
func TestEquipState_Step6(t *testing.T) {
	assert.Equal(t, itemsvc.StateWielded, sword.EquipState())
	assert.Equal(t, itemsvc.StateWorn, chain.EquipState())
	assert.Equal(t, itemsvc.StateWorn, gem.EquipState())
	assert.Equal(t, itemsvc.State(""), wand4.EquipState())
	assert.Equal(t, itemsvc.State(""), itemsvc.Item{Category: itemsvc.CategoryAmmunition}.EquipState())
}

// ADR 0033 step 6: a state the category does not allow is refused (state,
// not_allowed), a second body armour without the swap too (state,
// slot_taken); nothing is written or logged.
func TestSetState_Step6_Refusals(t *testing.T) {
	t.Run("not allowed", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(dal, readsCharacter(t).Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "longsword").Return(sword, nil)
		_, err := svc.SetState(context.Background(), 1, "longsword", itemsvc.StateWorn, false)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "state", fe.Field)
		assert.Equal(t, itemsvc.CodeNotAllowed, fe.Message)
	})
	t.Run("slot taken", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(dal, readsCharacter(t).Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "chain_mail").Return(chain, nil)
		dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{chain, leather, sword}, nil)
		_, err := svc.SetState(context.Background(), 1, "chain_mail", itemsvc.StateWorn, false)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "state", fe.Field)
		assert.Equal(t, itemsvc.CodeSlotTaken, fe.Message)
	})
}

// ADR 0033 step 7: wielding is one equipment_state entry with the piece's
// snapshot (before carried, after wielded), so popping it puts the weapon
// away; the same state again logs nothing.
func TestSetState_Step7_LoggedMoveWithSnapshot(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := readsCharacter(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "longsword").Return(sword, nil).Once()
	wielded := sword
	wielded.State = itemsvc.StateWielded
	dal.EXPECT().Update(mock.Anything, int64(1), wielded).Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(12), nil).Once()

	res, err := svc.SetState(context.Background(), 1, "longsword", itemsvc.StateWielded, false)
	require.NoError(t, err)
	assert.Equal(t, &itemsvc.StateResult{Items: []itemsvc.Item{wielded}, LogEntryID: 12}, res)
	assert.Equal(t, actionlog.KindEquipmentState, got.Kind)
	assert.False(t, actionlog.IsRoll(got.Kind))
	assert.JSONEq(t, `{"itemId":"longsword","name":"Long Sword +2","category":"weapon","before":"carried","after":"wielded"}`, string(got.Payload))
	require.Len(t, got.Snapshots, 1)
	assert.Equal(t, "items:longsword", got.Snapshots[0].Key)
	assert.Contains(t, string(got.Snapshots[0].Before), `"state":"carried"`)
	assert.Contains(t, string(got.Snapshots[0].After), `"state":"wielded"`)

	// Popping it writes the "before" back: the sword is carried again.
	restore := mocks.NewMockDAL(t)
	rsvc, _ := itemsvc.New(restore, readsCharacter(t).Deps)
	restore.EXPECT().Update(mock.Anything, int64(1), sword).Return(nil).Once()
	require.NoError(t, rsvc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, got.Snapshots[0].Before))
	// Redoing it writes the "after": wielded again.
	restore.EXPECT().Update(mock.Anything, int64(1), wielded).Return(nil).Once()
	require.NoError(t, rsvc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, got.Snapshots[0].After))

	t.Run("same state again", func(t *testing.T) {
		dal := mocks.NewMockDAL(t) // no Update, no Record
		svc, _ := itemsvc.New(dal, readsCharacter(t).Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "longsword").Return(wielded, nil)
		res, err := svc.SetState(context.Background(), 1, "longsword", itemsvc.StateWielded, false)
		require.NoError(t, err)
		assert.Zero(t, res.LogEntryID)
	})
}

// ADR 0033 step 7: wearing a second body armour with the swap takes the
// first off in the same entry: one entry, both pieces' snapshots, the one
// taken off in the payload's off list.
func TestSetState_Step7_SwapIsOneEntry(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := readsCharacter(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "chain_mail").Return(chain, nil)
	dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{chain, leather, sword}, nil)
	off := leather
	off.State = itemsvc.StateCarried
	on := chain
	on.State = itemsvc.StateWorn
	dal.EXPECT().Update(mock.Anything, int64(1), off).Return(nil).Once()
	dal.EXPECT().Update(mock.Anything, int64(1), on).Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(3), nil).Once()

	res, err := svc.SetState(context.Background(), 1, "chain_mail", itemsvc.StateWorn, true)
	require.NoError(t, err)
	assert.Equal(t, []itemsvc.Item{on, off}, res.Items)
	require.Len(t, got.Snapshots, 2)
	assert.Equal(t, "items:chain_mail", got.Snapshots[0].Key)
	assert.Equal(t, "items:leather", got.Snapshots[1].Key)
	assert.JSONEq(t, `{"itemId":"chain_mail","name":"Chain mail","category":"armour","before":"carried","after":"worn",
		"off":[{"itemId":"leather","name":"Leather","category":"armour","before":"worn","after":"carried"}]}`, string(got.Payload))
}

// ADR 0033 step 9: a piece's powers are on only while it is worn or
// wielded: a carried ring protects nothing, the worn one does.
func TestItemPowers_Step9_CarriedRingProtectsNothing(t *testing.T) {
	luck := []itemsvc.Power{{ID: "luck", Effects: buffsvc.BuffEffects{SaveBonus: ptr(1)}}}
	worn := itemsvc.Item{
		ID: "worn_ring", Name: localized.Of("Worn ring"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicRing,
		State: itemsvc.StateWorn, Behaviour: itemsvc.BehaviourStock, Count: 1, Powers: luck,
	}
	carried := worn
	carried.ID, carried.Name, carried.State = "carried_ring", localized.Of("Carried ring"), itemsvc.StateCarried
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{carried, worn}, nil)
	got, err := svc.ItemPowers(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []buffsvc.ItemPower{{ItemID: "worn_ring", ItemName: "Worn ring", PowerID: "luck", Effects: luck[0].Effects}}, got)
}

// ADR 0033 steps 2 to 5 and 25: a piece's category and the codes and
// numbers of its category are checked; the fields of the other categories
// are not.
func TestValidate_Steps2To5_CategoryFields(t *testing.T) {
	tests := []struct {
		name  string
		item  itemsvc.Item
		field string
	}{
		{"unknown category", itemsvc.Item{Category: "potion"}, "category"},
		{"magic without kind", itemsvc.Item{Category: itemsvc.CategoryMagic}, "magicKind"},
		{"kind on gear", itemsvc.Item{Category: itemsvc.CategoryGear, MagicKind: itemsvc.MagicRing}, "magicKind"},
		{"worn gear", itemsvc.Item{Category: itemsvc.CategoryGear, State: itemsvc.StateWorn}, "state"},
		{"weapon dice malformed", withWeapon(func(i *itemsvc.Item) { i.DamageSM = "1d8+" }), "damageSm"},
		{"weapon flat part out of range", withWeapon(func(i *itemsvc.Item) { i.DamageL = "1d8+21" }), "damageL"},
		{"weapon flat damage 0", withWeapon(func(i *itemsvc.Item) { i.DamageL = "0" }), "damageL"},
		{"weapon kind", withWeapon(func(i *itemsvc.Item) { i.WeaponKind = "thrown" }), "weaponKind"},
		{"melee fires", withWeapon(func(i *itemsvc.Item) { i.Fires = "arrow" }), "fires"},
		{"unknown weapon type", withWeapon(func(i *itemsvc.Item) { i.WeaponType = "katana" }), "weaponType"},
		{"size", withWeapon(func(i *itemsvc.Item) { i.Size = "h" }), "size"},
		{"speed", withWeapon(func(i *itemsvc.Item) { i.Speed = 21 }), "speed"},
		{"three damage types", withWeapon(func(i *itemsvc.Item) { i.DamageTypes = []string{"b", "p", "s"} }), "damageTypes"},
		{"damage type twice", withWeapon(func(i *itemsvc.Item) { i.DamageTypes = []string{"p", "p"} }), "damageTypes"},
		// ADR 0033 step 4: only a launcher may have no dice of its own, and then both or none.
		{"melee without dice", withWeapon(func(i *itemsvc.Item) { i.DamageSM, i.DamageL = "", "" }), "damageSm"},
		{"hurled without dice", withWeapon(func(i *itemsvc.Item) { i.WeaponKind, i.DamageSM, i.DamageL = "missile", "", "" }), "damageSm"},
		{"launcher with one dice", withLauncher(func(i *itemsvc.Item) { i.DamageSM = "1d6" }), "damageL"},
		// ADR 0016 step 8: only a catalogue piece with no damage type (the mancatcher) may have no
		// dice; a homebrew one is refused.
		{"homebrew without damage type nor dice", withWeapon(func(i *itemsvc.Item) { i.DamageTypes, i.DamageSM, i.DamageL = nil, "", "" }), "damageSm"},
		{"catalogue piece with a damage type and no dice", withWeapon(func(i *itemsvc.Item) { i.CatalogueCode, i.DamageSM, i.DamageL = "mancatcher", "", "" }), "damageSm"},
		{"ammo kind", itemsvc.Item{Category: itemsvc.CategoryAmmunition, AmmoKind: "stone"}, "ammoKind"},
		{"ammo one dice", itemsvc.Item{Category: itemsvc.CategoryAmmunition, AmmoKind: "arrow", DamageSM: "1d6"}, "damageL"},
		{"base AC", withArmour(func(i *itemsvc.Item) { i.BaseAC = 11 }), "baseAc"},
		{"skill column", withArmour(func(i *itemsvc.Item) { i.SkillColumn = "mithril" }), "skillColumn"},
		{"armour row", withArmour(func(i *itemsvc.Item) { i.ArmorRow = "mithril" }), "armorRow"},
		{"shield kind", itemsvc.Item{Category: itemsvc.CategoryShield, ShieldKind: "tower"}, "shieldKind"},
		{"shield bonus", itemsvc.Item{Category: itemsvc.CategoryShield, ShieldKind: "body", ACMissile: 6}, "acMissile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it := tt.item
			it.Name, it.Behaviour, it.Count = localized.Of("X"), itemsvc.BehaviourStock, 1
			if it.State == "" {
				it.State = itemsvc.StateCarried
			}
			var fe *domain.FieldError
			require.ErrorAs(t, it.Validate(), &fe)
			assert.Equal(t, tt.field, fe.Field)
		})
	}
	t.Run("valid pieces", func(t *testing.T) {
		for _, it := range []itemsvc.Item{
			withWeapon(nil), withLauncher(nil), withArmour(nil), chain, sword,
			// ADR 0016 step 8 (amended): the damage as printed, NdM±K or a flat K.
			withWeapon(func(i *itemsvc.Item) { i.DamageSM, i.DamageL = "1d4+1", "1" }),
			withWeapon(func(i *itemsvc.Item) { i.DamageSM, i.DamageL = "1d3-1", "1d2-1" }),
			// The catalogue's mancatcher: no damage type, no dice.
			withWeapon(func(i *itemsvc.Item) {
				i.CatalogueCode, i.DamageTypes, i.DamageSM, i.DamageL = "mancatcher", nil, "", ""
			}),
		} {
			assert.NoError(t, it.Validate(), it.ID)
		}
	})
}

func withWeapon(mut func(*itemsvc.Item)) itemsvc.Item {
	it := sword
	it.DamageTypes = []string{"s"}
	if mut != nil {
		mut(&it)
	}
	return it
}

// withLauncher is a catalogue long bow: no dice of its own (ADR 0033 step 4).
func withLauncher(mut func(*itemsvc.Item)) itemsvc.Item {
	it := withWeapon(func(i *itemsvc.Item) {
		i.ID, i.WeaponKind, i.Fires, i.DamageSM, i.DamageL, i.TwoHanded = "long_bow", "missile", "arrow", "", "", true
	})
	if mut != nil {
		mut(&it)
	}
	return it
}

func withArmour(mut func(*itemsvc.Item)) itemsvc.Item {
	it := chain
	if mut != nil {
		mut(&it)
	}
	return it
}

// ADR 0033 step 27: an edit that classifies the piece (its category, its
// category's codes) takes it off the review list; one that only renames it
// does not. Dismissing takes it off and logs nothing.
func TestReview_Step27_SettledByClassifyingEditOrDismiss(t *testing.T) {
	mace := itemsvc.Item{
		ID: "mace", Name: localized.Of("Mace +1"), Category: itemsvc.CategoryWeapon, State: itemsvc.StateWielded,
		Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1, DamageSM: "1d6", DamageL: "1d6", WeaponKind: "melee",
		WeaponType: "other_melee",
	}
	t.Run("classifying edit", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := readsCharacter(t)
		svc, _ := itemsvc.New(dal, f.Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "mace").Return(mace, nil)
		dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil)
		dal.EXPECT().DeleteReviews(mock.Anything, int64(1), "mace").Return(nil).Once()
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
		_, err := svc.Update(context.Background(), 1, "mace", itemsvc.ItemPatch{Speed: ptr(7), DamageTypes: &[]string{"b"}})
		require.NoError(t, err)
	})
	t.Run("rename only", func(t *testing.T) {
		dal := mocks.NewMockDAL(t) // no DeleteReviews
		f := readsCharacter(t)
		svc, _ := itemsvc.New(dal, f.Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "mace").Return(mace, nil)
		dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil)
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
		_, err := svc.Update(context.Background(), 1, "mace", itemsvc.ItemPatch{Name: localized.Text{"fr": "Masse +1"}})
		require.NoError(t, err)
	})
	t.Run("dismiss", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(dal, readsCharacter(t).Deps) // no Record
		dal.EXPECT().Get(mock.Anything, int64(1), "mace").Return(mace, nil)
		dal.EXPECT().DeleteReviews(mock.Anything, int64(1), "mace").Return(nil).Once()
		require.NoError(t, svc.DismissReview(context.Background(), 1, "mace"))
	})
	t.Run("listed in list order, pieces held only", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(dal, readsCharacter(t).Deps)
		dal.EXPECT().Reviews(mock.Anything, int64(1)).Return([]itemsvc.Review{
			{ItemID: "sack", Reason: itemsvc.ReasonUncategorised},
			{ItemID: "gone", Reason: itemsvc.ReasonUncategorised},
			{ItemID: "mace", Reason: itemsvc.ReasonWeaponUnmatched},
		}, nil)
		dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{mace, {ID: "sack"}}, nil)
		got, err := svc.Reviews(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, []itemsvc.Review{
			{ItemID: "mace", Reason: itemsvc.ReasonWeaponUnmatched}, {ItemID: "sack", Reason: itemsvc.ReasonUncategorised},
		}, got)
	})
}

// A new category that cannot keep the piece's state puts it back in the
// pack in the same edit (ADR 0033 step 6): a worn gem made gear is carried.
func TestUpdate_Step6_CategoryChangeDropsAStateItCannotKeep(t *testing.T) {
	gemWorn := gem
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), gem.ID).Return(gemWorn, nil)
	dal.EXPECT().Update(mock.Anything, int64(1), mock.MatchedBy(func(it itemsvc.Item) bool {
		return it.Category == itemsvc.CategoryGear && it.MagicKind == "" && it.State == itemsvc.StateCarried
	})).Return(nil).Once()
	dal.EXPECT().DeleteReviews(mock.Anything, int64(1), gem.ID).Return(nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
	gear := itemsvc.CategoryGear
	_, err := svc.Update(context.Background(), 1, gem.ID, itemsvc.ItemPatch{Category: &gear})
	require.NoError(t, err)
}

func character(specializes bool) *charactersvc.Character {
	return &charactersvc.Character{ID: 1, Derived: charactersvc.Derived{Specializes: specializes}}
}

// fighter is a fighter whose weapon picks are picks (ADR 0037 steps 7 and 9).
func fighter(picks ...skills.WeaponPick) *charactersvc.Character {
	c := character(true)
	c.Class = "fighter"
	c.Proficiencies.Weapon = picks
	return c
}

// ADR 0037 step 9 (amending ADR 0030 step 12): a weapon is specialized when its type has the
// fighter's specialization pick, derived on every read: two weapons of the type are both
// specialized, a weapon of a plain pick's type is not, whatever the stored flag said (the DAL
// no longer reads it); a class that does not specialize gets none from a stored pick; a
// creation and an edit answer the derived value (an edit moving the weapon to another type
// follows it), and the edit logs no "specialized" change.
func TestSpecialized_Step9_DerivedFromThePick(t *testing.T) {
	// The character row's specialization, as charactersvc reads it.
	// The fixture's book: whether a class specializes is its feature list (ADR 0040 step 3).
	book, err := ruletablesadpt.ReadProficiencies(filepath.Join("..", "..", "..", "..", "testdata", "seeds"), cataloguetest.Definitions(t))
	require.NoError(t, err)
	specializedIn := func(c *charactersvc.Character) skills.WeaponProficiency {
		return skills.WeaponProficiencyOf(book, c.ActiveClasses(), c.Proficiencies)
	}
	spec := skills.WeaponPick{Type: "long_sword", Slots: 2}
	plain := skills.WeaponPick{Type: "dagger", Slots: 1}
	second := sword
	second.ID, second.Position = "longsword_2", 3
	dagger := sword
	dagger.ID, dagger.Position, dagger.WeaponType = "dagger", 4, "dagger"
	dagger.Specialized = true // a stale value never wins

	t.Run("list", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		svc, _ := itemsvc.New(dal, f.Deps)
		dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{leather, sword, second, dagger}, nil)
		f.chars.EXPECT().WeaponProficiency(mock.Anything, int64(1)).Return(specializedIn(fighter(spec, plain)), nil).Once()
		got, err := svc.List(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, []bool{false, true, true, false}, []bool{got[0].Specialized, got[1].Specialized, got[2].Specialized, got[3].Specialized})
	})
	t.Run("a class that does not specialize", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		svc, _ := itemsvc.New(dal, f.Deps)
		dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{sword}, nil)
		bard := fighter(spec)
		bard.Class, bard.Derived.Specializes = "bard", false
		f.chars.EXPECT().WeaponProficiency(mock.Anything, int64(1)).Return(specializedIn(bard), nil).Once()
		got, err := svc.List(context.Background(), 1)
		require.NoError(t, err)
		assert.False(t, got[0].Specialized)
	})
	t.Run("no weapon reads no character", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{leather}, nil)
		_, err := svc.List(context.Background(), 1)
		require.NoError(t, err)
	})
	t.Run("create answers the derived value", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		svc, _ := itemsvc.New(dal, f.Deps)
		f.chars.EXPECT().WeaponProficiency(mock.Anything, int64(1)).Return(specializedIn(fighter(spec)), nil)
		dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
		dal.EXPECT().Insert(mock.Anything, int64(1), mock.Anything).Return(nil)
		dal.EXPECT().DeleteReviews(mock.Anything, int64(1), "long_sword").Return(nil)
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(3), nil)
		res, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{
			Name: localized.Of("Long Sword"), Category: itemsvc.CategoryWeapon, Behaviour: itemsvc.BehaviourStock, Count: 1,
			DamageSM: "1d8", DamageL: "1d12", WeaponKind: "melee", WeaponType: "long_sword",
		})
		require.NoError(t, err)
		assert.True(t, res.Item.Specialized)
	})
	t.Run("an edit to another type follows it", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		svc, _ := itemsvc.New(dal, f.Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "longsword").Return(sword, nil)
		f.chars.EXPECT().WeaponProficiency(mock.Anything, int64(1)).Return(specializedIn(fighter(spec)), nil)
		dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil)
		dal.EXPECT().DeleteReviews(mock.Anything, int64(1), "longsword").Return(nil)
		var got actionlog.NewEntry
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(4), nil)
		otherType := "short_sword"
		res, err := svc.Update(context.Background(), 1, "longsword", itemsvc.ItemPatch{WeaponType: &otherType})
		require.NoError(t, err)
		assert.False(t, res.Item.Specialized)
		assert.NotContains(t, string(got.Payload), `"specialized"`)
	})
}

// ADR 0016 step 7 (a weapon is a piece, ADR 0033 step 1): removing a weapon
// logs item_remove with its whole snapshot; the payload keeps its name.
func TestDelete_RemovesAndLogsWholeWeapon(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := readsCharacter(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "longsword").Return(sword, nil)
	dal.EXPECT().Delete(mock.Anything, int64(1), "longsword").Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(2), nil)
	res, err := svc.Delete(context.Background(), 1, "longsword")
	require.NoError(t, err)
	assert.Equal(t, sword, res.Item)
	assert.Equal(t, actionlog.KindItemRemove, got.Kind)
	assert.Contains(t, string(got.Payload), `"name":"Long Sword +2"`)
	assert.Contains(t, string(got.Payload), `"category":"weapon"`)
	assert.Contains(t, string(got.Snapshots[0].Before), `"weaponType":"long_sword"`)
	assert.Contains(t, string(got.Snapshots[0].After), `"absent":true`)
}

// ADR 0037 step 8 on the piece: every weapon read carries the penalty an attack with it takes
// (Table 34's for the class group when its type has no pick, from the same character read as
// the specialization), so the card and the attack form show it before a roll; a picked type,
// an untyped weapon and a non-weapon piece carry none, whatever the value read.
func TestNonproficientPenalty_Step8_DerivedOnTheWeaponPiece(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dagger := sword
	dagger.ID, dagger.Position, dagger.WeaponType = "dagger", 3, "dagger"
	untyped := sword
	untyped.ID, untyped.Position, untyped.WeaponType = "club", 4, ""
	stale := leather
	stale.NonproficientPenalty = -9
	dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{stale, sword, dagger, untyped}, nil)
	f.chars.EXPECT().WeaponProficiency(mock.Anything, int64(1)).
		Return(skills.WeaponProficiency{Picked: []string{"dagger"}, Penalty: -2}, nil).Once()
	got, err := svc.List(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []int{0, -2, 0, 0}, []int{
		got[0].NonproficientPenalty, got[1].NonproficientPenalty, got[2].NonproficientPenalty, got[3].NonproficientPenalty,
	})
}
