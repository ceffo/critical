package itemsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// catalogueWeapons are catalogue rows as format 7 carries them (numbers of
// PHB Table 44).
var catalogueWeapons = []ruletablesvc.Weapon{
	{Code: "composite_short_bow", WeaponType: "short_bow", Kind: "missile", Size: "m", Speed: 6, Fires: []string{"arrow"}},
	{Code: "short_bow", WeaponType: "short_bow", Kind: "missile", Size: "m", Speed: 7, Fires: []string{"arrow"}},
	{Code: "long_sword", WeaponType: "long_sword", Kind: "melee", Size: "m", Types: []string{"s"}, Speed: 5, DamageSM: "1d8", DamageL: "1d12"},
	{Code: "dagger", WeaponType: "dagger", Kind: "melee", Size: "s", Types: []string{"p"}, Speed: 2, DamageSM: "1d4", DamageL: "1d3"},
}

var (
	legacySword = itemsvc.LegacyWeapon{
		ID: "longsword", Position: 1, Name: localized.Of("Long Sword +2"), DamageSM: "1d8", DamageL: "1d12",
		MagicToHit: 2, MagicDamage: 2, Kind: "melee", Type: "long_sword",
	}
	legacyBow = itemsvc.LegacyWeapon{
		ID: "shortbow", Position: 0, Name: localized.Of("Short Bow +2"), DamageSM: "1d6", DamageL: "1d6",
		MagicToHit: 2, MagicDamage: 2, Kind: "missile", TwoHanded: true, AmmoKind: "arrow", Type: "short_bow",
	}
	legacyMace = itemsvc.LegacyWeapon{
		ID: "mace", Position: 2, Name: localized.Of("Mace +1"), DamageSM: "1d6", DamageL: "1d6",
		MagicToHit: 1, MagicDamage: 1, Kind: "melee", Type: "other_melee",
	}
	legacyArrows = itemsvc.Item{
		ID: "arrows_plus1", Position: 0, Name: localized.Of("Arrows +1"), Type: "arrow", Behaviour: itemsvc.BehaviourStock,
		MagicBonus: 1, AmmoKind: "arrow", Count: 18, MaxCount: 18,
	}
	legacyGem = itemsvc.Item{
		ID: "power_gem_dispel", Position: 3, Name: localized.Of("Power Gem"), Type: "gem", Behaviour: itemsvc.BehaviourDaily,
		Count: 1, MaxCount: 1, Powers: []itemsvc.Power{hammer},
	}
	legacySack = itemsvc.Item{
		ID: "sack", Position: 1, Name: localized.Of("Bag of holding"), Type: "sac", Behaviour: itemsvc.BehaviourStock,
		Count: 1, MaxCount: 1, Notes: "kept",
	}
)

// ADR 0033 step 26.1: every item and weapon keeps its id, its fields and its
// name; the items keep their positions and the weapons follow them in their
// order. Step 26.6: every weapon is wielded, every piece with powers worn,
// the rest carried.
func TestConvert_Step26_1_IdsFieldsOrderAndStates(t *testing.T) {
	conv := itemsvc.Convert([]itemsvc.Item{legacyArrows, legacySack, legacyGem},
		[]itemsvc.LegacyWeapon{legacySword, legacyBow}, catalogueWeapons)

	ids := make([]string, 0, len(conv.Pieces))
	for _, p := range conv.Pieces {
		ids = append(ids, p.ID)
	}
	assert.Equal(t, []string{"arrows_plus1", "sack", "power_gem_dispel", "shortbow", "longsword"}, ids)
	assert.Equal(t, []int{0, 1, 3, 4, 5}, []int{
		conv.Pieces[0].Position, conv.Pieces[1].Position, conv.Pieces[2].Position,
		conv.Pieces[3].Position, conv.Pieces[4].Position,
	}, "the weapons after the items, bow (0) before sword (1)")

	sack := conv.Pieces[1]
	assert.Equal(t, legacySack.Name, sack.Name)
	assert.Equal(t, "sac", sack.Type, "the label is kept as stored")
	assert.Equal(t, "kept", sack.Notes)
	assert.Equal(t, itemsvc.StateCarried, sack.State)

	gem := conv.Pieces[2]
	assert.Equal(t, itemsvc.StateWorn, gem.State, "a piece with powers is worn: the Power Gem keeps working")
	assert.Equal(t, []itemsvc.Power{hammer}, gem.Powers)
	for _, w := range conv.Pieces[3:] {
		assert.Equal(t, itemsvc.CategoryWeapon, w.Category)
		assert.Equal(t, itemsvc.StateWielded, w.State, "every weapon is wielded: all are usable today")
		assert.Equal(t, 1, w.Count)
	}
	sword := conv.Pieces[4]
	assert.Equal(t, "1d8", sword.DamageSM)
	assert.Equal(t, 2, sword.MagicToHit)
	assert.Equal(t, 2, sword.MagicDamage)
	assert.Equal(t, "long_sword", sword.WeaponType)
	assert.Empty(t, conv.Renames)
}

// ADR 0033 step 26.3: an item whose id a weapon of the same character has
// takes the next free suffix (one the character does not hold), and the
// rename is recorded for its old snapshots; the weapon keeps the id.
func TestConvert_Step26_3_ItemAndWeaponOfTheSameId(t *testing.T) {
	dagger := itemsvc.Item{ID: "dagger", Name: localized.Of("Dagger (spare)"), Type: "flask", Behaviour: itemsvc.BehaviourStock, Count: 1}
	taken := itemsvc.Item{ID: "dagger_2", Position: 1, Name: localized.Of("Rope"), Type: "flask", Behaviour: itemsvc.BehaviourStock, Count: 1}
	weapon := itemsvc.LegacyWeapon{ID: "dagger", Name: localized.Of("Dagger +3"), DamageSM: "1d4", DamageL: "1d3", Kind: "melee", Type: "dagger"}
	conv := itemsvc.Convert([]itemsvc.Item{dagger, taken}, []itemsvc.LegacyWeapon{weapon}, catalogueWeapons)
	assert.Equal(t, map[string]string{"dagger": "dagger_3"}, conv.Renames)
	assert.Equal(t, "dagger_3", conv.Pieces[0].ID)
	assert.Equal(t, "dagger", conv.Pieces[2].ID)
	assert.Equal(t, itemsvc.CategoryWeapon, conv.Pieces[2].Category)
}

// ADR 0033 step 26.4: a weapon is matched to the catalogue row of its type,
// which adds the code, the size, the damage types and the speed and never
// overwrites a stored stat (the stored dice, kind, two-handed, magic stay);
// other_* and no type are not matched and go on the review list, as every
// weapon without the catalogue's equipment (format 6).
func TestConvert_Step26_4_WeaponMatchedByType(t *testing.T) {
	sword, reasons := itemsvc.ConvertWeapon(legacySword, catalogueWeapons)
	assert.Empty(t, reasons)
	assert.Equal(t, "long_sword", sword.CatalogueCode)
	assert.Equal(t, "m", sword.Size)
	assert.Equal(t, []string{"s"}, sword.DamageTypes)
	assert.Equal(t, 5, sword.Speed)

	bow, _ := itemsvc.ConvertWeapon(legacyBow, catalogueWeapons)
	assert.Equal(t, "short_bow", bow.CatalogueCode, "the row whose code is the type, not the composite bow")
	assert.Equal(t, 7, bow.Speed)
	assert.Equal(t, "1d6", bow.DamageSM, "the stored dice are kept (the catalogue's launcher has none)")
	assert.True(t, bow.TwoHanded)
	assert.Equal(t, "arrow", bow.Fires)

	for _, w := range []itemsvc.LegacyWeapon{legacyMace, {ID: "x", Kind: "melee", DamageSM: "1d4", DamageL: "1d4"}} {
		piece, reasons := itemsvc.ConvertWeapon(w, catalogueWeapons)
		assert.Equal(t, []string{itemsvc.ReasonWeaponUnmatched}, reasons, w.ID)
		assert.Empty(t, piece.CatalogueCode)
		assert.Zero(t, piece.Speed)
	}
	_, reasons = itemsvc.ConvertWeapon(legacySword, nil)
	assert.Equal(t, []string{itemsvc.ReasonWeaponUnmatched}, reasons, "no catalogue equipment: nothing to match")
}

// ADR 0033 step 26.5: an item is categorised from its type label when it is
// obvious (singular or plural; an item with an ammo kind is ammunition); the
// rest is gear on the review list. Ammunition with no dice gets the dice of
// the first launcher that fires its kind, so the attack rolls the same.
func TestConvert_Step26_5_ItemsByLabelAndLauncherDice(t *testing.T) {
	tests := []struct {
		label, ammo string
		category    itemsvc.Category
		kind        string // magic kind or ammo kind
		reasons     []string
	}{
		{"arrow", "", itemsvc.CategoryAmmunition, "arrow", nil},
		{"Arrows", "", itemsvc.CategoryAmmunition, "arrow", nil},
		{"bolt", "", itemsvc.CategoryAmmunition, "quarrel", nil},
		{"quarrels", "", itemsvc.CategoryAmmunition, "quarrel", nil},
		{"bullet", "", itemsvc.CategoryAmmunition, "bullet", nil},
		{"", "arrow", itemsvc.CategoryAmmunition, "arrow", nil},
		{"misc", "stones", itemsvc.CategoryAmmunition, "stones", []string{itemsvc.ReasonAmmoKindUnknown}},
		{"potion", "", itemsvc.CategoryMagic, itemsvc.MagicPotion, nil},
		{"scrolls", "", itemsvc.CategoryMagic, itemsvc.MagicScroll, nil},
		{"ring", "", itemsvc.CategoryMagic, itemsvc.MagicRing, nil},
		{"rod", "", itemsvc.CategoryMagic, itemsvc.MagicRod, nil},
		{"staff", "", itemsvc.CategoryMagic, itemsvc.MagicStaff, nil},
		{"wand", "", itemsvc.CategoryMagic, itemsvc.MagicWand, nil},
		{"gem", "", itemsvc.CategoryMagic, itemsvc.MagicMisc, nil},
		{"flask", "", itemsvc.CategoryGear, "", nil},
		{"sac", "", itemsvc.CategoryGear, "", []string{itemsvc.ReasonUncategorised}},
		{"", "", itemsvc.CategoryGear, "", []string{itemsvc.ReasonUncategorised}},
	}
	for _, tt := range tests {
		piece, reasons := itemsvc.ConvertItem(itemsvc.Item{ID: "x", Type: tt.label, AmmoKind: tt.ammo, Behaviour: itemsvc.BehaviourStock, Count: 1})
		assert.Equal(t, tt.category, piece.Category, tt.label)
		if tt.category == itemsvc.CategoryAmmunition {
			assert.Equal(t, tt.kind, piece.AmmoKind, tt.label)
		} else {
			assert.Equal(t, tt.kind, piece.MagicKind, tt.label)
		}
		assert.Equal(t, tt.reasons, reasons, tt.label)
		assert.Equal(t, itemsvc.StateCarried, piece.State, tt.label)
	}

	conv := itemsvc.Convert([]itemsvc.Item{legacyArrows}, []itemsvc.LegacyWeapon{legacySword, legacyBow}, catalogueWeapons)
	arrows := conv.Pieces[0]
	assert.Equal(t, "1d6", arrows.DamageSM, "the short bow's dice: the attack rolls the same")
	assert.Equal(t, "1d6", arrows.DamageL)
	assert.Equal(t, 1, arrows.MagicBonus)
}

// ADR 0033 steps 9 and 26.6: a piece with powers keeps them on: an
// uncategorised one becomes a miscellaneous magic piece, worn, on the review
// list; one that cannot be worn stays carried with the powers off, on the
// review list.
// ADR 0033 step 1.1: the conversion copies a type label that names a
// component (the label the spells matched, any case, singular or plural)
// into the component code; any other label leaves it empty.
func TestConvert_Step1_1_ComponentFromTheLabel(t *testing.T) {
	for label, want := range map[string]string{"gem": "gem", " Gem ": "gem", "GEMS": "gem", "sac": "", "": "", "gemme": ""} {
		t.Run(label, func(t *testing.T) {
			it := legacySack
			it.Type = label
			piece, _ := itemsvc.ConvertItem(it)
			assert.Equal(t, want, piece.Component)
			assert.Equal(t, label, piece.Type, "the label is kept as stored")
		})
	}
	conv := itemsvc.Convert([]itemsvc.Item{legacyGem}, nil, catalogueWeapons)
	assert.Equal(t, itemsvc.ComponentGem, conv.Pieces[0].Component, "the Power Gem is a gem (daily: never spent)")
}

func TestConvert_Step26_6_PowersStayOnOrAreReviewed(t *testing.T) {
	luck := []itemsvc.Power{{ID: "luck", Effects: buffsvc.BuffEffects{SaveBonus: ptr(1)}}}
	amulet, reasons := itemsvc.ConvertItem(itemsvc.Item{ID: "amulet", Type: "amulet", Behaviour: itemsvc.BehaviourStock, Count: 1, Powers: luck})
	assert.Equal(t, itemsvc.CategoryMagic, amulet.Category)
	assert.Equal(t, itemsvc.MagicMisc, amulet.MagicKind)
	assert.Equal(t, itemsvc.StateWorn, amulet.State)
	assert.Equal(t, []string{itemsvc.ReasonUncategorised}, reasons)

	wand, reasons := itemsvc.ConvertItem(itemsvc.Item{ID: "w", Type: "wand", Behaviour: itemsvc.BehaviourCharges, Count: 1, Powers: luck})
	assert.Equal(t, itemsvc.StateCarried, wand.State)
	assert.Equal(t, []string{itemsvc.ReasonPowersOff}, reasons)
}

// ADR 0033 step 26.2: an old weapon_* entry's "weapons:<id>" snapshot is
// restored into the piece of that id: an edit popped puts the weapon's stats
// back and keeps the piece's place, category fields and state; a removed
// weapon comes back converted (matched, wielded), last in the list; an add
// popped deletes the piece.
func TestRestorers_Step26_2_OldWeaponEntriesPopIntoThePiece(t *testing.T) {
	snap := snaptest.EnvID("longsword", `{"position":1,"name":{"en":"Long Sword +1"},"damageSm":"1d8","damageL":"1d12",
		"magicToHit":1,"magicDamage":1,"kind":"melee","twoHanded":false,"ammoKind":"","weaponType":"long_sword"}`)
	piece, _ := itemsvc.ConvertWeapon(legacySword, catalogueWeapons)
	piece.Position = 7
	restore := func(dal *mocks.MockDAL, state json.RawMessage) error {
		f := readsCharacter(t)
		f.Equipment = &ruletablesvc.Equipment{Weapons: catalogueWeapons}
		svc, _ := itemsvc.New(dal, f.Deps)
		return svc.Module().Restorer(itemsvc.StateKeyWeapons).Restore(context.Background(), 1, state)
	}
	t.Run("edit", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Get(mock.Anything, int64(1), "longsword").Return(piece, nil)
		want := piece
		want.Name, want.MagicToHit, want.MagicDamage = localized.Of("Long Sword +1"), 1, 1
		dal.EXPECT().Update(mock.Anything, int64(1), want).Return(nil).Once()
		require.NoError(t, restore(dal, snap))
	})
	t.Run("removed", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Get(mock.Anything, int64(1), "longsword").Return(itemsvc.Item{}, itemsvc.ErrItemNotFound)
		dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{{ID: "arrows", Position: 4}}, nil)
		dal.EXPECT().Insert(mock.Anything, int64(1), mock.MatchedBy(func(it itemsvc.Item) bool {
			return it.ID == "longsword" && it.Position == 5 && it.State == itemsvc.StateWielded &&
				it.CatalogueCode == "long_sword" && it.MagicToHit == 1 && it.Category == itemsvc.CategoryWeapon
		})).Return(nil).Once()
		require.NoError(t, restore(dal, snap))
	})
	t.Run("added", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Delete(mock.Anything, int64(1), "longsword").Return(nil).Once()
		require.NoError(t, restore(dal, snaptest.Absent("longsword")))
	})
}

// ADR 0033 step 26.4: a weapon whose type matches no row is matched by its
// name without the magic suffix, or by its id, when exactly one row of its
// kind names it; a qualified name ("Bastard sword, two-handed") is told by the
// two-handed flag; an ambiguous or missing name stays on the review list. The
// stored stats and the weapon's own name are kept.
func TestConvert_Step26_4_WeaponMatchedByName(t *testing.T) {
	rows := []ruletablesvc.Weapon{
		{
			Code: "bastard_sword_one_handed", WeaponType: "other_melee", Kind: "melee", Size: "m", Types: []string{"s"}, Speed: 6,
			Name: localized.Text{"en": "Bastard sword, one-handed", "fr": "Épée bâtarde à une main"},
		},
		{
			Code: "bastard_sword_two_handed", WeaponType: "other_melee", Kind: "melee", Size: "m", Types: []string{"s"}, Speed: 8, TwoHanded: true,
			Name: localized.Text{"en": "Bastard sword, two-handed", "fr": "Épée bâtarde à deux mains"},
		},
		{
			Code: "footmans_mace", WeaponType: "other_melee", Kind: "melee", Size: "m", Types: []string{"b"}, Speed: 7,
			Name: localized.Text{"en": "Footman's mace"},
		},
		{
			Code: "horsemans_mace", WeaponType: "other_melee", Kind: "melee", Size: "m", Types: []string{"b"}, Speed: 6,
			Name: localized.Text{"en": "Horseman's mace"},
		},
		{
			Code: "scimitar", WeaponType: "other_melee", Kind: "melee", Size: "m", Types: []string{"s"}, Speed: 5,
			Name: localized.Text{"en": "Scimitar", "fr": "Cimeterre"},
		},
		{Code: "spear", WeaponType: "other_melee", Kind: "melee", Speed: 6, Name: localized.Text{"en": "Spear"}},
		{Code: "spear_2", WeaponType: "other_melee", Kind: "melee", Speed: 6, Name: localized.Text{"en": "Spear"}},
	}
	w := func(id string, name localized.Text, twoHanded bool) itemsvc.LegacyWeapon {
		return itemsvc.LegacyWeapon{
			ID: id, Name: name, Kind: "melee", Type: "other_melee", TwoHanded: twoHanded,
			DamageSM: "2d4", MagicToHit: 2, MagicDamage: 2,
		}
	}

	two, reasons := itemsvc.ConvertWeapon(w("bastard_sword", localized.Text{"en": "Bastard Sword +2", "fr": "Épée bâtarde +2"}, true), rows)
	assert.Empty(t, reasons)
	assert.Equal(t, "bastard_sword_two_handed", two.CatalogueCode, "the flag tells the qualified rows apart")
	assert.Equal(t, 8, two.Speed)
	assert.Equal(t, []string{"s"}, two.DamageTypes)
	assert.Equal(t, "Bastard Sword +2", two.Name["en"], "its own name stays")
	assert.Equal(t, 2, two.MagicToHit, "its own bonus stays")
	assert.Equal(t, "2d4", two.DamageSM, "its own dice stay")

	one, _ := itemsvc.ConvertWeapon(w("bastard_sword", localized.Of("Bastard sword +1/+2"), false), rows)
	assert.Equal(t, "bastard_sword_one_handed", one.CatalogueCode, "a +N/+M suffix is dropped too")

	fr, _ := itemsvc.ConvertWeapon(w("sabre", localized.Text{"fr": "Cimeterre +1"}, false), rows)
	assert.Equal(t, "scimitar", fr.CatalogueCode, "the name in any language")

	byID, _ := itemsvc.ConvertWeapon(w("scimitar", localized.Of("My blade"), false), rows)
	assert.Equal(t, "scimitar", byID.CatalogueCode, "the id is the code")

	for name, lw := range map[string]itemsvc.LegacyWeapon{
		"mace: no row is named so": w("mace", localized.Of("Mace +1"), false),
		"two rows of one name":     w("spear", localized.Of("Spear"), false),
		"a name no row has":        w("x", localized.Of("Flamberge +3"), false),
	} {
		piece, reasons := itemsvc.ConvertWeapon(lw, rows)
		assert.Equal(t, []string{itemsvc.ReasonWeaponUnmatched}, reasons, name)
		assert.Empty(t, piece.CatalogueCode, name)
	}
}

// ADR 0033 step 26.3: ids that the action log still holds count as taken: a stored item steps aside
// for a weapon only in the log, a removed item whose id a weapon has is recorded without a piece, and
// the suffix is free of every id in play.
func TestConvertWithHistory_Step26_3_LogOnlyIdentities(t *testing.T) {
	stored := itemsvc.Item{ID: "dagger", Name: localized.Of("Spare"), Type: "sack", Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1}
	conv := itemsvc.ConvertWithHistory([]itemsvc.Item{stored}, nil, catalogueWeapons,
		itemsvc.History{WeaponIDs: []string{"dagger"}, ItemIDs: []string{"dagger_2"}})
	assert.Equal(t, map[string]string{"dagger": "dagger_3"}, conv.Renames, "dagger_2 is an item of the log: taken")
	require.Len(t, conv.Pieces, 1)
	assert.Equal(t, "dagger_3", conv.Pieces[0].ID)

	conv = itemsvc.ConvertWithHistory(nil, []itemsvc.LegacyWeapon{legacySword}, catalogueWeapons,
		itemsvc.History{ItemIDs: []string{"longsword", "other"}})
	assert.Equal(t, map[string]string{"longsword": "longsword_2"}, conv.Renames)
	require.Len(t, conv.Pieces, 1, "only the weapon is a piece")
	assert.Equal(t, "longsword", conv.Pieces[0].ID)

	plain := itemsvc.Convert([]itemsvc.Item{stored}, nil, catalogueWeapons)
	assert.Empty(t, plain.Renames, "no log, no weapon: nothing renamed")
}
