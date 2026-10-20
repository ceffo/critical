package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// noWeapon is the wire tail of a piece that is no weapon, armour or shield
// (ADR 0033 step 1: one flat shape, the other categories' fields zero).
const noWeapon = `"damageTypes":[],"weaponType":"","weaponKind":"","size":"","speed":0,"magicToHit":0,"magicDamage":0,
	"twoHanded":false,"fires":"","specialized":false,"baseAc":0,"skillColumn":"","armorRow":"","shieldKind":"","acMelee":0,"acMissile":0`

// ADR 0033 steps 1, 2, 6 and 27: a piece is served with its category, state
// and the fields of every category, and the list carries each piece's
// review reasons.
func TestItems_ListWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.items.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{{
		ID: "arrows_plus1", Position: 0, Name: localized.Of("Arrows +1"), Type: "arrow", Behaviour: itemsvc.BehaviourStock,
		Category: itemsvc.CategoryAmmunition, State: itemsvc.StateCarried,
		MagicBonus: 1, AmmoKind: "arrow", Count: 18, MaxCount: 18, DamageSM: "1d6", DamageL: "1d6",
	}, {
		ID: "mace", Position: 1, Name: localized.Of("Mace +1"), Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1,
		Category: itemsvc.CategoryWeapon, State: itemsvc.StateWielded, DamageSM: "1d6", DamageL: "1d6",
		WeaponKind: "melee", WeaponType: "other_melee", MagicToHit: 1, MagicDamage: 1,
	}}, nil)
	m.items.EXPECT().Reviews(mock.Anything, int64(1)).Return([]itemsvc.Review{{ItemID: "mace", Reason: itemsvc.ReasonWeaponUnmatched}}, nil)
	m.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return([]itemsvc.Warning{{ItemID: "mace", Code: itemsvc.WarnWeaponNotAllowed}}, nil)
	rec := do(h, "GET", "/api/items", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[{"id":"arrows_plus1","name":{"en":"Arrows +1"},"category":"ammunition","magicKind":"","catalogueCode":"","component":"","spellId":"",
		"state":"carried","equipState":"","type":"arrow","behaviour":"stock","magicalBonus":1,
		"ammoKind":"arrow","count":18,"maxCount":18,"effectDice":"","effectSave":"","notes":"","powers":[],
		"damageSm":"1d6","damageL":"1d6",`+noWeapon+`,"review":[],"warnings":[]},
		{"id":"mace","name":{"en":"Mace +1"},"category":"weapon","magicKind":"","catalogueCode":"","component":"","spellId":"","state":"wielded","equipState":"wielded","type":"",
		"behaviour":"stock","magicalBonus":0,"ammoKind":"","count":1,"maxCount":1,"effectDice":"","effectSave":"","notes":"",
		"powers":[],"damageTypes":[],"damageSm":"1d6","damageL":"1d6","weaponType":"other_melee","weaponKind":"melee","size":"",
		"speed":0,"magicToHit":1,"magicDamage":1,"twoHanded":false,"fires":"","specialized":false,"baseAc":0,"skillColumn":"",
		"armorRow":"","shieldKind":"","acMelee":0,"acMissile":0,"review":["weapon_unmatched"],"warnings":["weapon_not_allowed"]}]`, rec.Body.String())
}

// ADR 0033 steps 6 and 7: POST /api/items/{id}/state moves a piece and
// answers the pieces changed with the entry; a refusal names the field and
// its code.
func TestItems_SetState(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	worn := wandItem
	worn.ID, worn.Category, worn.MagicKind, worn.State = "ring", itemsvc.CategoryMagic, itemsvc.MagicRing, itemsvc.StateWorn
	m.items.EXPECT().SetState(mock.Anything, int64(1), "ring", itemsvc.StateWorn, true).
		Return(&itemsvc.StateResult{Items: []itemsvc.Item{worn}, LogEntryID: 21}, nil)
	rec := do(h, "POST", "/api/items/ring/state", `{"state":"worn","swap":true}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"logEntryId":21`)
	assert.Contains(t, rec.Body.String(), `"state":"worn"`)

	m.items.EXPECT().SetState(mock.Anything, int64(1), "sword", itemsvc.StateWorn, false).
		Return(nil, &domain.FieldError{Field: "state", Message: itemsvc.CodeNotAllowed})
	rec = do(h, "POST", "/api/items/sword/state", `{"state":"worn"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"not_allowed","field":"state","code":"invalid_input"}`, rec.Body.String())
	assert.Equal(t, http.StatusBadRequest, do(h, "POST", "/api/items/sword/state", `{"state":"worn","bogus":1}`).Code)
}

// ADR 0033 step 27: DELETE /api/items/{id}/review dismisses the piece's
// review and answers the list.
func TestItems_DismissReview(t *testing.T) {
	h, m := newRouter(t)
	m.items.EXPECT().DismissReview(mock.Anything, int64(1), "mace").Return(nil).Once()
	m.items.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{}, nil)
	m.items.EXPECT().Reviews(mock.Anything, int64(1)).Return([]itemsvc.Review{}, nil)
	m.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return(nil, nil)
	rec := do(h, "DELETE", "/api/items/mace/review", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())

	m.items.EXPECT().DismissReview(mock.Anything, int64(1), "x").Return(fmt.Errorf("%w: x", itemsvc.ErrItemNotFound))
	assert.Equal(t, http.StatusNotFound, do(h, "DELETE", "/api/items/x/review", "").Code)
}

func TestItems_UseWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.items.EXPECT().UseItem(mock.Anything, int64(1), "wand_lightning").Return(&itemsvc.UseResult{
		ItemID: "wand_lightning", Name: "Wand of lightning", Type: "wand", Behaviour: itemsvc.BehaviourCharges,
		Removed: true, Remaining: 0, MaxCount: 4, Rolls: []int{}, LogEntryID: 9,
	}, nil)
	rec := do(h, "POST", "/api/items/wand_lightning/use", "{}")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"itemId":"wand_lightning","name":"Wand of lightning","type":"wand","behaviour":"charges",
		"removed":true,"remaining":0,"maxCount":4,"dice":"","rolls":[],"modifier":0,"total":0,"save":"","notes":"","logEntryId":9}`, rec.Body.String())
}

func TestItems_Restock(t *testing.T) {
	t.Run("wire shape", func(t *testing.T) {
		h, m := newRouter(t)
		m.items.EXPECT().Restock(mock.Anything, int64(1), "arrows_plus3", 1).Return(&itemsvc.RestockResult{
			ItemID: "arrows_plus3", Name: "Arrows +3", Type: "arrow", Added: 1, Count: 1, LogEntryID: 4,
		}, nil)
		rec := do(h, "POST", "/api/items/arrows_plus3/restock", `{"count":1}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"itemId":"arrows_plus3","name":"Arrows +3","type":"arrow","added":1,"count":1,"logEntryId":4}`, rec.Body.String())
	})

	for name, body := range map[string]string{"missing count": `{}`, "not json": `nope`, "not a number": `{"count":"x"}`} {
		t.Run("400 on "+name, func(t *testing.T) {
			h, _ := newRouter(t) // the service is never called
			rec := do(h, "POST", "/api/items/arrows_plus3/restock", body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), `"error"`)
		})
	}

	for name, c := range map[string]struct {
		err    error
		status int
	}{
		"charges or daily": {fmt.Errorf("%w: Wand (charges)", itemsvc.ErrNotRestockable), http.StatusConflict},
		"unknown item":     {fmt.Errorf("%w: x", itemsvc.ErrItemNotFound), http.StatusNotFound},
		"bad count":        {fmt.Errorf("%w: restock count", domain.ErrInvalidInput), http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			h, m := newRouter(t)
			m.items.EXPECT().Restock(mock.Anything, int64(1), "x", 2).Return(nil, c.err)
			rec := do(h, "POST", "/api/items/x/restock", `{"count":2}`)
			assert.Equal(t, c.status, rec.Code)
			assert.Contains(t, rec.Body.String(), `"error"`)
		})
	}
}

const wandJSON = `{"id":"w","name":{"en":"Wand"},"category":"magic","magicKind":"wand","catalogueCode":"","component":"","spellId":"","state":"carried","equipState":"",
	"type":"wand","behaviour":"charges","magicalBonus":0,"ammoKind":"","count":3,
	"maxCount":4,"effectDice":"","effectSave":"","notes":"","powers":[],"damageSm":"","damageL":"",` + noWeapon + `,"review":[],"warnings":[]}`

var wandItem = itemsvc.Item{
	ID: "w", Name: localized.Of("Wand"), Type: "wand", Behaviour: itemsvc.BehaviourCharges, Count: 3, MaxCount: 4,
	Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicWand, State: itemsvc.StateCarried,
}

func TestItems_Create(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	m.items.EXPECT().Create(mock.Anything, int64(1), itemsvc.ItemInput{
		Name: localized.Of("Arrows +3"), Category: itemsvc.CategoryAmmunition, Behaviour: itemsvc.BehaviourStock, Count: 6,
		MagicBonus: 3, AmmoKind: "arrow", DamageSM: "1d6", DamageL: "1d6",
	}).Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 12}, nil)
	rec := do(h, "POST", "/api/items", `{"name":{"en":"Arrows +3"},"category":"ammunition","type":"ignored","behaviour":"stock",
		"count":6,"magicalBonus":3,"ammoKind":"arrow","damageSm":"1d6","damageL":"1d6"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"item":`+wandJSON+`,"logEntryId":12}`, rec.Body.String())
}

// An invalid field comes back as 400 with the message and the field name.
func TestItems_CreateInvalidNamesTheField(t *testing.T) {
	h, m := newRouter(t)
	m.items.EXPECT().Create(mock.Anything, int64(1), mock.Anything).
		Return(nil, fmt.Errorf("wrapped: %w", &domain.FieldError{Field: "effectDice", Message: "Effect dice must look like 2d6 or 1d8+2"}))
	rec := do(h, "POST", "/api/items", `{"name":{"en":"X"},"behaviour":"stock","effectDice":"2x6"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"Effect dice must look like 2d6 or 1d8+2","field":"effectDice","code":"invalid_input"}`, rec.Body.String())
}

func TestItems_UpdatePatchesOnlyTheFieldsSent(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	three := 3
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Count: &three}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 13}, nil)
	rec := do(h, "PATCH", "/api/items/w", `{"count":3}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"item":`+wandJSON+`,"logEntryId":13}`, rec.Body.String())
}

// ADR 0016 step 15: an edit or create body's powers key is accepted and
// ignored (an item served back as is still decodes, any other unknown key
// stays refused); the patch and the input carry no powers. An item is
// served with its powers, read only.
// ADR 0033 step 17 (amending ADR 0016 step 15): a body's powers, in the
// shape they are served, reach the service: a PATCH's replace the list (an
// empty list too), a POST's are the new piece's; a PATCH without them keeps
// them; an unknown key in a power is refused. The list is served back.
func TestItems_ADR0033Step17_PowersInTheBody(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	three := 3
	ring := []itemsvc.Power{{ID: "protection", Effects: buffsvc.BuffEffects{AC: new(-1), SaveBonus: new(1), WithoutMagicArmour: true}}}
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Count: &three, Powers: &ring}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 13}, nil).Once()
	rec := do(h, "PATCH", "/api/items/w", `{"count":3,"powers":[{"id":"protection","effects":{"ac":-1,"save_bonus":1,"without_magic_armour":true}}]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	none := []itemsvc.Power(nil)
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Powers: &none}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 14}, nil).Once()
	rec = do(h, "PATCH", "/api/items/w", `{"powers":[]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Count: &three}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 15}, nil).Once()
	rec = do(h, "PATCH", "/api/items/w", `{"count":3}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	m.items.EXPECT().Create(mock.Anything, int64(1), itemsvc.ItemInput{
		Name: localized.Of("Plate +1"), Category: itemsvc.CategoryArmour, Behaviour: itemsvc.BehaviourStock, Count: 1,
		Powers: []itemsvc.Power{itemsvc.MagicPower(1)},
	}).Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 16}, nil).Once()
	rec = do(h, "POST", "/api/items", `{"name":{"en":"Plate +1"},"category":"armour","behaviour":"stock","count":1,"powers":[{"id":"magic","effects":{"ac":-1}}]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = do(h, "PATCH", "/api/items/w", `{"powers":[{"id":"x","effects":{"bogus":1}}]}`)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	m.items.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{{
		ID: "power_gem_dispel", Name: localized.Of("Power Gem"), Type: "gem", Behaviour: itemsvc.BehaviourDaily, Count: 1, MaxCount: 1,
		Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicMisc, State: itemsvc.StateWorn,
		Powers: []itemsvc.Power{{ID: "thunder_hammer", MinStr: new(18), Effects: buffsvc.BuffEffects{
			WeaponBuff: new(5), Multiplier: new(2.0), WeaponKind: buffsvc.WeaponMelee,
		}}},
	}}, nil)
	m.items.EXPECT().Reviews(mock.Anything, int64(1)).Return(nil, nil)
	m.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return(nil, nil)
	rec = do(h, "GET", "/api/items", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[{"id":"power_gem_dispel","name":{"en":"Power Gem"},"category":"magic","magicKind":"misc","catalogueCode":"","component":"","spellId":"",
		"state":"worn","equipState":"worn","type":"gem","behaviour":"daily","magicalBonus":0,
		"ammoKind":"","count":1,"maxCount":1,"effectDice":"","effectSave":"","notes":"",
		"powers":[{"id":"thunder_hammer","minStr":18,"effects":{"weapon_buff":5,"multiplier":2,"weapon_kind":"melee"}}],
		"damageSm":"","damageL":"",`+noWeapon+`,"review":[],"warnings":[]}]`,
		rec.Body.String())
}

func TestItems_Delete(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	m.items.EXPECT().Delete(mock.Anything, int64(1), "w").Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 14}, nil)
	rec := do(h, "DELETE", "/api/items/w", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"item":`+wandJSON+`,"logEntryId":14}`, rec.Body.String())
}

// ADR 0037 step 9: a body's specialized is accepted and ignored (the pick specializes): a create
// sending it creates the piece without it, an edit sending only it patches nothing (the service
// then refuses the edit as changing nothing).
func TestItems_Step9_SpecializedInTheBodyIsIgnored(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	m.items.EXPECT().Create(mock.Anything, int64(1), itemsvc.ItemInput{
		Name: localized.Of("Long Sword"), Category: itemsvc.CategoryWeapon, Behaviour: itemsvc.BehaviourStock, Count: 1,
		WeaponType: "long_sword",
	}).Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 12}, nil)
	rec := do(h, "POST", "/api/items", `{"name":{"en":"Long Sword"},"category":"weapon","behaviour":"stock","count":1,
		"weaponType":"long_sword","specialized":true}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 13}, nil)
	rec = do(h, "PATCH", "/api/items/w", `{"specialized":true}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestItems_EditErrors(t *testing.T) {
	for _, body := range []string{`nope`, `{"bogus":1}`, `{"count":"x"}`} {
		h, _ := newRouter(t)
		assert.Equal(t, http.StatusBadRequest, do(h, "PATCH", "/api/items/w", body).Code, body)
		assert.Equal(t, http.StatusBadRequest, do(h, "POST", "/api/items", body).Code, body)
	}
	h, m := newRouter(t)
	m.items.EXPECT().Update(mock.Anything, int64(1), "x", mock.Anything).Return(nil, fmt.Errorf("%w: x", itemsvc.ErrItemNotFound))
	m.items.EXPECT().Delete(mock.Anything, int64(1), "x").Return(nil, fmt.Errorf("%w: x", itemsvc.ErrItemNotFound))
	assert.Equal(t, http.StatusNotFound, do(h, "PATCH", "/api/items/x", `{"count":1}`).Code)
	assert.Equal(t, http.StatusNotFound, do(h, "DELETE", "/api/items/x", "").Code)
}

// ADR 0023 steps 9 and 11: an item's name is served as a language map, and
// the editor's name is a language map passed on as sent (the service keeps
// the languages it does not hold); a plain string is refused.
func TestItems_Step9_NameIsALanguageMap(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	both := wandItem
	both.Name = localized.Text{"en": "Wand", "fr": "Baguette"}
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Name: localized.Text{"fr": "Baguette"}}).
		Return(&itemsvc.ChangeResult{Item: both, LogEntryID: 13}, nil)
	rec := do(h, "PATCH", "/api/items/w", `{"name":{"fr":"Baguette"}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"name":{"en":"Wand","fr":"Baguette"}`)

	rec = do(h, "PATCH", "/api/items/w", `{"name":"Baguette"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// ADR 0033 step 1.1: the component code is read from a create and a patch
// body and served; a patch without it leaves it as it is (nil).
func TestItems_Step1_1_ComponentOnTheWire(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	gem := wandItem
	gem.Component = itemsvc.ComponentGem
	m.items.EXPECT().Create(mock.Anything, int64(1), itemsvc.ItemInput{
		Name: localized.Of("Ruby"), Category: itemsvc.CategoryGear, Component: "gem", Behaviour: itemsvc.BehaviourStock, Count: 2,
	}).Return(&itemsvc.ChangeResult{Item: gem, LogEntryID: 15}, nil)
	rec := do(h, "POST", "/api/items", `{"name":{"en":"Ruby"},"category":"gear","component":"gem","behaviour":"stock","count":2}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"component":"gem"`)

	none := ""
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Component: &none}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 16}, nil)
	rec = do(h, "PATCH", "/api/items/w", `{"component":""}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// ADR 0033 step 2: the spell a scroll holds is read from a create and a
// patch body ("" clears it) and served; a patch without it leaves it (nil).
func TestItems_ADR0033Step2_ScrollSpellOnTheWire(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	named := wandItem
	named.MagicKind, named.SpellID = itemsvc.MagicScroll, "burning_hands"
	m.items.EXPECT().Create(mock.Anything, int64(1), itemsvc.ItemInput{
		Name: localized.Of("Scroll"), Category: itemsvc.CategoryMagic, MagicKind: "scroll", SpellID: "burning_hands",
		Behaviour: itemsvc.BehaviourStock, Count: 1,
	}).Return(&itemsvc.ChangeResult{Item: named, LogEntryID: 15}, nil)
	rec := do(h, "POST", "/api/items", `{"name":{"en":"Scroll"},"category":"magic","magicKind":"scroll","spellId":"burning_hands","behaviour":"stock","count":1}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"spellId":"burning_hands"`)

	none := ""
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{SpellID: &none}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 16}, nil)
	rec = do(h, "PATCH", "/api/items/w", `{"spellId":""}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// ADR 0033 step 24: a POST with a catalogue code passes the code, the
// player's fields and equip on; the service copies the row.
func TestItems_Step24_CreatePickedFromTheCatalogue(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	m.items.EXPECT().Create(mock.Anything, int64(1), itemsvc.ItemInput{
		CatalogueCode: "long_sword", Equip: true, Name: localized.Text{"fr": "Épée de Kelsen"}, Count: 1, MagicToHit: 1, MagicDamage: 1,
	}).Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 12}, nil)
	rec := do(h, "POST", "/api/items", `{"catalogueCode":"long_sword","equip":true,"name":{"fr":"Épée de Kelsen"},"count":1,
		"magicToHit":1,"magicDamage":1}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"item":`+wandJSON+`,"logEntryId":12}`, rec.Body.String())
}

// ADR 0033 step 25: an unknown catalogue code is a 400 on its field with the
// code unknown.
func TestItems_Step25_UnknownCatalogueCode(t *testing.T) {
	h, m := newRouter(t)
	m.items.EXPECT().Create(mock.Anything, int64(1), mock.Anything).
		Return(nil, &domain.FieldError{Field: "catalogueCode", Message: itemsvc.CodeUnknown})
	rec := do(h, "POST", "/api/items", `{"catalogueCode":"katana"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"unknown","field":"catalogueCode","code":"invalid_input"}`, rec.Body.String())
}

// ADR 0033 steps 21 and 24: the catalogue rows are served with their codes,
// numbers and language maps, lists never null.
func TestItems_Step21_CatalogueWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.items.EXPECT().Catalogue().Return(ruletablesvc.Equipment{
		Weapons: []ruletablesvc.Weapon{{
			Code: "long_bow", Name: localized.Text{"en": "Long bow", "fr": "Arc long"}, WeaponType: "long_bow",
			Kind: "missile", Size: "l", Speed: 8, TwoHanded: true, Fires: []string{"arrow"},
		}},
		Ammunition: []ruletablesvc.Ammunition{{
			Code: "sheaf_arrow", Name: localized.Of("Sheaf arrow"), AmmoKind: "arrow",
			Types: []string{"p"}, DamageSM: "1d8", DamageL: "1d8",
		}},
		Armour:  []ruletablesvc.Armour{{Code: "chain_mail", Name: localized.Of("Chain mail"), BaseAC: 5, SkillColumn: "metal", ArmorRow: "chain"}},
		Shields: []ruletablesvc.Shield{{Code: "buckler", Name: localized.Of("Buckler"), Kind: "buckler", ACMelee: 1, ACMissile: 1}},
	})
	rec := do(h, "GET", "/api/equipment/catalogue", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"weapons":[{"code":"long_bow","name":{"en":"Long bow","fr":"Arc long"},"weaponType":"long_bow","weaponKind":"missile",
			"size":"l","damageTypes":[],"speed":8,"damageSm":"","damageL":"","twoHanded":true,"fires":["arrow"]}],
		"ammunition":[{"code":"sheaf_arrow","name":{"en":"Sheaf arrow"},"ammoKind":"arrow","damageTypes":["p"],"damageSm":"1d8","damageL":"1d8"}],
		"armour":[{"code":"chain_mail","name":{"en":"Chain mail"},"baseAc":5,"skillColumn":"metal","armorRow":"chain"}],
		"shields":[{"code":"buckler","name":{"en":"Buckler"},"shieldKind":"buckler","acMelee":1,"acMissile":1}]}`, rec.Body.String())
}

// noWarnings answers every class warnings read with none, for the tests of something else.
func noWarnings(m *mocksSet) {
	m.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return(nil, nil).Maybe()
}

// ADR 0033 step 20: every answer that serves a piece carries its true warnings: a mage that wears
// plate gets the warning on the state answer, on the create and edit answers, and on the list.
func TestItems_Step20_EveryAnswerCarriesItsWarnings(t *testing.T) {
	h, m := newRouter(t)
	plate := itemsvc.Item{
		ID: "plate", Name: localized.Of("Plate"), Category: itemsvc.CategoryArmour, State: itemsvc.StateWorn,
		Behaviour: itemsvc.BehaviourStock, Count: 1, SkillColumn: "none_allowed",
	}
	warn := []itemsvc.Warning{{ItemID: "plate", Code: itemsvc.WarnArmourNotAllowed}}
	m.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return(warn, nil)
	m.items.EXPECT().SetState(mock.Anything, int64(1), "plate", itemsvc.StateWorn, false).
		Return(&itemsvc.StateResult{Items: []itemsvc.Item{plate}, LogEntryID: 3}, nil)
	m.items.EXPECT().Update(mock.Anything, int64(1), "plate", mock.Anything).
		Return(&itemsvc.ChangeResult{Item: plate, LogEntryID: 4}, nil)
	m.items.EXPECT().Create(mock.Anything, int64(1), mock.Anything).
		Return(&itemsvc.ChangeResult{Item: plate, LogEntryID: 5}, nil)
	m.items.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{plate}, nil)
	m.items.EXPECT().Reviews(mock.Anything, int64(1)).Return(nil, nil)
	for _, c := range [][3]string{
		{"POST", "/api/items/plate/state", `{"state":"worn"}`},
		{"PATCH", "/api/items/plate", `{"count":1}`},
		{"POST", "/api/items", `{"name":{"en":"Plate"},"category":"armour"}`},
		{"GET", "/api/items", ""},
	} {
		rec := do(h, c[0], c[1], c[2])
		require.Equal(t, http.StatusOK, rec.Code, c[1]+rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"warnings":["armour_not_allowed"]`, c[0]+" "+c[1])
	}
}
