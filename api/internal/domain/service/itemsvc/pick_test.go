package itemsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// pickRows is a small catalogue (ADR 0033 step 21): one row per list, the
// mace's damage as printed, the blowgun firing two kinds.
var pickRows = ruletablesvc.Equipment{
	Weapons: []ruletablesvc.Weapon{
		{
			Code: "footmans_mace", Name: localized.Text{"en": "Footman's mace", "fr": "Masse d'armes de fantassin"}, WeaponType: "other_melee",
			Kind: "melee", Size: "m", Types: []string{"b"}, Speed: 7, DamageSM: "1d6+1", DamageL: "1d6",
		},
		{
			Code: "blowgun", Name: localized.Text{"en": "Blowgun"}, WeaponType: "other_missile", Kind: "missile", Size: "l",
			Speed: 5, TwoHanded: true, Fires: []string{"blowgun_dart", "needle"},
		},
	},
	Ammunition: []ruletablesvc.Ammunition{{
		Code: "sheaf_arrow", Name: localized.Text{"en": "Sheaf arrow"}, AmmoKind: "arrow",
		Types: []string{"p"}, DamageSM: "1d8", DamageL: "1d8",
	}},
	Armour: []ruletablesvc.Armour{{
		Code: "elven_chain", Name: localized.Text{"en": "Elven chain mail", "fr": "Cotte de mailles elfique"},
		BaseAC: 5, SkillColumn: "elven_chain", ArmorRow: "chain",
	}},
	Shields: []ruletablesvc.Shield{{Code: "body_shield", Name: localized.Text{"en": "Body shield"}, Kind: "body", ACMelee: 1, ACMissile: 2}},
}

func pickSvc(t *testing.T) (itemsvc.SVC, *mocks.MockDAL, fixture) {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	f.Equipment = &pickRows
	svc, err := itemsvc.New(dal, f.Deps)
	require.NoError(t, err)
	return svc, dal, f
}

// ADR 0033 steps 23 and 24: a piece picked from the catalogue copies the
// row's category, numbers and codes and keeps its code; the name is the
// row's language map, a typed name replaces the languages it holds; only the
// count, notes and magic come from the player, every other field sent is
// ignored. The add is one item_add entry naming the code; the piece is
// carried.
func TestCreate_Step24_PickCopiesTheRow(t *testing.T) {
	base := itemsvc.Item{State: itemsvc.StateCarried, Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1}
	piece := func(mut func(*itemsvc.Item)) itemsvc.Item {
		it := base
		mut(&it)
		return it
	}
	cases := []struct {
		name string
		in   itemsvc.ItemInput
		want itemsvc.Item
	}{
		{
			"weapon: damage as printed, magic from the player, typed fields ignored",
			itemsvc.ItemInput{CatalogueCode: "footmans_mace", MagicToHit: 1, MagicDamage: 2, DamageSM: "9d9", BaseAC: 3, Category: itemsvc.CategoryGear},
			piece(func(i *itemsvc.Item) {
				i.ID, i.CatalogueCode, i.Category, i.Name = "footman_s_mace", "footmans_mace", itemsvc.CategoryWeapon, pickRows.Weapons[0].Name
				i.WeaponType, i.WeaponKind, i.Size, i.DamageTypes, i.Speed = "other_melee", "melee", "m", []string{"b"}, 7
				i.DamageSM, i.DamageL, i.MagicToHit, i.MagicDamage = "1d6+1", "1d6", 1, 2
			}),
		},
		{
			"launcher of two kinds: the first, no dice",
			itemsvc.ItemInput{CatalogueCode: "blowgun"},
			piece(func(i *itemsvc.Item) {
				i.ID, i.CatalogueCode, i.Category, i.Name = "blowgun", "blowgun", itemsvc.CategoryWeapon, pickRows.Weapons[1].Name
				i.WeaponType, i.WeaponKind, i.Size, i.Speed, i.TwoHanded, i.Fires = "other_missile", "missile", "l", 5, true, "blowgun_dart"
			}),
		},
		{
			"ammunition: a count and its magic",
			itemsvc.ItemInput{CatalogueCode: "sheaf_arrow", Count: 20, MagicBonus: 1, Notes: " quiver "},
			piece(func(i *itemsvc.Item) {
				i.ID, i.CatalogueCode, i.Category, i.Name = "sheaf_arrow", "sheaf_arrow", itemsvc.CategoryAmmunition, pickRows.Ammunition[0].Name
				i.AmmoKind, i.DamageTypes, i.DamageSM, i.DamageL, i.MagicBonus = "arrow", []string{"p"}, "1d8", "1d8", 1
				i.Count, i.MaxCount, i.Notes = 20, 20, "quiver"
			}),
		},
		{
			"armour renamed in French: English kept, no magic",
			itemsvc.ItemInput{CatalogueCode: "elven_chain", Name: localized.Text{"fr": "Cotte elfique de Kelsen"}, MagicToHit: 3},
			piece(func(i *itemsvc.Item) {
				i.ID, i.CatalogueCode, i.Category = "elven_chain_mail", "elven_chain", itemsvc.CategoryArmour
				i.Name = localized.Text{"en": "Elven chain mail", "fr": "Cotte elfique de Kelsen"}
				i.BaseAC, i.SkillColumn, i.ArmorRow = 5, "elven_chain", "chain"
			}),
		},
		{
			"shield",
			itemsvc.ItemInput{CatalogueCode: "body_shield"},
			piece(func(i *itemsvc.Item) {
				i.ID, i.CatalogueCode, i.Category, i.Name = "body_shield", "body_shield", itemsvc.CategoryShield, pickRows.Shields[0].Name
				i.ShieldKind, i.ACMelee, i.ACMissile = "body", 1, 2
			}),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, f := pickSvc(t)
			dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil).Once()
			dal.EXPECT().Insert(mock.Anything, int64(1), c.want).Return(nil).Once()
			dal.EXPECT().DeleteReviews(mock.Anything, int64(1), c.want.ID).Return(nil).Once()
			var got actionlog.NewEntry
			f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(5), nil).Once()

			res, err := svc.Create(context.Background(), 1, c.in)
			require.NoError(t, err)
			assert.Equal(t, c.want, res.Item)
			assert.Equal(t, actionlog.KindItemAdd, got.Kind)
			assert.Contains(t, string(got.Payload), `"catalogueCode":"`+c.in.CatalogueCode+`"`)
		})
	}
}

// ADR 0033 step 25: a code no catalogue row has is refused on its field with
// the code unknown, nothing written; before catalogue format 7 every code is
// unknown. A typed piece (no code) is created as before.
func TestCreate_Step25_UnknownCatalogueCode(t *testing.T) {
	for name, eq := range map[string]*ruletablesvc.Equipment{"format 7": &pickRows, "before format 7": nil} {
		t.Run(name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t) // no write expected
			f := deps(t)
			f.Equipment = eq
			svc, _ := itemsvc.New(dal, f.Deps)
			_, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{CatalogueCode: "katana"})
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, "catalogueCode", fe.Field)
			assert.Equal(t, itemsvc.CodeUnknown, fe.Message)
		})
	}
}

// ADR 0033 step 24 ("wear it now"): Equip makes the add and the move one
// transaction (one undo): the item_add, then the equipment_state entry, a
// worn armour in the way taken off in it. A piece that cannot be equipped is
// refused before anything is written.
func TestCreate_Step24_EquipNowIsOneTransaction(t *testing.T) {
	_, dal, f := pickSvc(t)
	tx := &countTx{}
	f.Tx = tx
	svc, _ := itemsvc.New(dal, f.Deps)
	worn := leather
	dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{worn}, nil)
	dal.EXPECT().DeleteReviews(mock.Anything, int64(1), "elven_chain_mail").Return(nil).Once()
	var inserted itemsvc.Item
	dal.EXPECT().Get(mock.Anything, int64(1), "elven_chain_mail").RunAndReturn(func(context.Context, int64, string) (itemsvc.Item, error) {
		return inserted, nil
	}).Once()
	off := worn
	off.State = itemsvc.StateCarried
	dal.EXPECT().Update(mock.Anything, int64(1), off).Return(nil).Once()
	dal.EXPECT().Update(mock.Anything, int64(1), mock.MatchedBy(func(it itemsvc.Item) bool {
		return it.ID == "elven_chain_mail" && it.State == itemsvc.StateWorn
	})).Return(nil).Once()
	var kinds []actionlog.Kind
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) {
		kinds = append(kinds, e.Kind)
	}).Return(int64(7), nil).Times(2)
	dal.EXPECT().Insert(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) {
		inserted = it
	}).Return(nil).Once()

	res, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{CatalogueCode: "elven_chain", Equip: true})
	require.NoError(t, err)
	assert.Equal(t, 1, tx.n, "one transaction: one undo")
	assert.Equal(t, []actionlog.Kind{actionlog.KindItemAdd, actionlog.KindEquipmentState}, kinds)
	assert.Equal(t, itemsvc.StateWorn, res.Item.State)

	t.Run("ammunition cannot be equipped", func(t *testing.T) {
		svc, _, _ := pickSvc(t) // no write expected
		_, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{CatalogueCode: "sheaf_arrow", Equip: true})
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "equip", fe.Field)
	})
}

// countTx runs the unit of work directly and counts the transactions.
type countTx struct{ n int }

func (c *countTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	c.n++
	return fn(ctx)
}

// ADR 0033 step 21: the catalogue served is the one loaded, empty before
// format 7.
func TestCatalogue_Step21_ServedAsLoaded(t *testing.T) {
	svc, _, _ := pickSvc(t)
	assert.Equal(t, pickRows, svc.Catalogue())
	none, _ := itemsvc.New(mocks.NewMockDAL(t), deps(t).Deps)
	assert.Equal(t, ruletablesvc.Equipment{}, none.Catalogue())
}
