package router_test

import (
	"encoding/json"
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
)

// ADR 0049 step 4: a body's abilities, in the shape they are served, reach the service: a PATCH's
// replace the list (an empty one too), a POST's are the new piece's; a PATCH without them keeps
// them; an unknown key in an ability is refused. The item is served with them, and an item not
// converted yet is served without the key.
func TestItems_ADR0049Step4_AbilitiesInTheBodyAndOnTheWire(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	abilities := []itemsvc.Ability{{
		ID: "dispel", Name: localized.Of("Dispel"), Note: "once a day",
		Limit:   itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 1},
		Effects: []itemsvc.Effect{{Kind: itemsvc.EffectDice, Dice: "2d6", Save: "rod_staff_wand", Label: "fire"}},
	}}
	const wire = `[{"id":"dispel","name":{"en":"Dispel"},"note":"once a day","limit":{"kind":"per_day","max":1,"left":1},
		"effects":[{"kind":"dice","dice":"2d6","save":"rod_staff_wand","label":"fire"}]}]`

	// Served, an ability also says whether a use is paid now (step 5); never read from a body.
	const served = `[{"id":"dispel","name":{"en":"Dispel"},"note":"once a day","limit":{"kind":"per_day","max":1,"left":1},
		"effects":[{"kind":"dice","dice":"2d6","save":"rod_staff_wand","label":"fire"}],"usable":true}]`

	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Abilities: &abilities}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 13}, nil).Once()
	rec := do(h, "PATCH", "/api/items/w", `{"abilities":`+wire+`}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	none := []itemsvc.Ability{}
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Abilities: &none}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 14}, nil).Once()
	require.Equal(t, http.StatusOK, do(h, "PATCH", "/api/items/w", `{"abilities":[]}`).Code)

	three := 3
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Count: &three}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 15}, nil).Once()
	require.Equal(t, http.StatusOK, do(h, "PATCH", "/api/items/w", `{"count":3}`).Code)

	m.items.EXPECT().Create(mock.Anything, int64(1), itemsvc.ItemInput{
		Name: localized.Of("Wand"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicWand,
		Behaviour: itemsvc.BehaviourStock, Count: 1, Abilities: abilities,
	}).Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 16}, nil).Once()
	rec = do(h, "POST", "/api/items", `{"name":{"en":"Wand"},"category":"magic","magicKind":"wand","behaviour":"stock","count":1,"abilities":`+wire+`}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, http.StatusBadRequest, do(h, "PATCH", "/api/items/w", `{"abilities":[{"id":"x","limit":{"kind":"unlimited"},"bogus":1}]}`).Code)

	converted, plain := wandItem, wandItem
	converted.Abilities = abilities
	plain.ID = "plain"
	m.items.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{converted, plain}, nil)
	m.items.EXPECT().Reviews(mock.Anything, int64(1)).Return(nil, nil)
	m.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return(nil, nil)
	rec = do(h, "GET", "/api/items", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 2)
	assert.JSONEq(t, served, string(mustRaw(t, got[0]["abilities"])))
	assert.NotContains(t, got[1], "abilities", "an item not converted yet is served without the key")
}

func mustRaw(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// ADR 0049 step 5: an exhausted per_day or charges limit is served with its left (0), never
// without; unlimited and with_item serve none of max, left or cost.
func TestItems_ADR0049Step5_LeftIsAlwaysServedForTheKindsThatHaveIt(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	it := wandItem
	it.Abilities = []itemsvc.Ability{
		{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 2, Left: 0}},
		{ID: "b", Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 5, Left: 0, Cost: 1}},
		{ID: "c", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}},
		{ID: "d", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}},
	}
	m.items.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{it}, nil)
	m.items.EXPECT().Reviews(mock.Anything, int64(1)).Return(nil, nil)
	m.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return(nil, nil)
	rec := do(h, "GET", "/api/items", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var got []struct {
		Abilities []struct {
			Limit json.RawMessage `json:"limit"`
		} `json:"abilities"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got[0].Abilities, 4)
	want := []string{`{"kind":"per_day","max":2,"left":0}`, `{"kind":"charges","max":5,"left":0,"cost":1}`, `{"kind":"unlimited"}`, `{"kind":"with_item"}`}
	for n, w := range want {
		assert.JSONEq(t, w, string(got[0].Abilities[n].Limit))
	}
}

// ADR 0049 steps 5 and 14: every ability is served with `usable` and, when it cannot be used,
// the `reason`: the web re-derives nothing.
func TestItems_ADR0049Step14_UsableAndReasonPerAbility(t *testing.T) {
	h, m := newRouter(t)
	noWarnings(m)
	it := wandItem
	it.Abilities = []itemsvc.Ability{
		{ID: "ready", Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 2, Left: 1}},
		{ID: "spent", Limit: itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 2, Left: 0}},
		{ID: "short", Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 5, Left: 1, Cost: 2}},
		{ID: "free", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}},
	}
	m.items.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{it}, nil)
	m.items.EXPECT().Reviews(mock.Anything, int64(1)).Return(nil, nil)
	m.items.EXPECT().Warnings(mock.Anything, int64(1), mock.Anything).Return(nil, nil)
	rec := do(h, "GET", "/api/items", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var got []struct {
		Abilities []struct {
			ID     string `json:"id"`
			Usable bool   `json:"usable"`
			Reason string `json:"reason"`
		} `json:"abilities"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	type state struct {
		usable bool
		reason string
	}
	want := map[string]state{"ready": {true, ""}, "spent": {false, "exhausted"}, "short": {false, "exhausted"}, "free": {true, ""}}
	require.Len(t, got[0].Abilities, len(want))
	for _, a := range got[0].Abilities {
		assert.Equal(t, want[a.ID], state{a.Usable, a.Reason}, a.ID)
	}
}

// ADR 0049 step 7: POST /api/items/{id}/abilities/{abilityId}/use answers the limit after the use
// and each effect's result; an exhausted ability is a 409 `exhausted`, an unknown one a 404.
func TestItems_ADR0049Step7_UseAnAbility(t *testing.T) {
	h, m := newRouter(t)
	three, four := 3, 4
	m.items.EXPECT().UseAbility(mock.Anything, int64(1), "wand", "use", mock.Anything).Return(&itemsvc.AbilityUseResult{
		ItemID: "wand", AbilityID: "use", Name: "Wand of lightning", Type: "wand", Limit: itemsvc.LimitCharges,
		Left: &three, Max: &four, Remaining: 3, LogEntryID: 9,
		Effects: []itemsvc.EffectResult{{Kind: itemsvc.EffectDice, Dice: "6d6", Rolls: []int{1, 2, 3, 4, 5, 6}, Total: 21, Save: "rod_staff_wand"}},
	}, nil).Once()
	rec := do(h, "POST", "/api/items/wand/abilities/use/use", "{}")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"itemId":"wand","abilityId":"use","name":"Wand of lightning","type":"wand","limit":"charges","left":3,"max":4,
		"remaining":3,"removed":false,"logEntryId":9,
		"effects":[{"kind":"dice","dice":"6d6","rolls":[1,2,3,4,5,6],"total":21,"save":"rod_staff_wand"}]}`, rec.Body.String())

	m.items.EXPECT().UseAbility(mock.Anything, int64(1), "sword", "turn", mock.Anything).
		Return(nil, fmt.Errorf("%w: Sword (turn)", itemsvc.ErrAbilityExhausted)).Once()
	rec = do(h, "POST", "/api/items/sword/abilities/turn/use", "{}")
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"exhausted"`)

	m.items.EXPECT().UseAbility(mock.Anything, int64(1), "sword", "fly", mock.Anything).
		Return(nil, fmt.Errorf("%w: Sword", itemsvc.ErrAbilityNotFound)).Once()
	rec = do(h, "POST", "/api/items/sword/abilities/fly/use", "{}")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"ability_not_found"`)

	m.items.EXPECT().UseAbility(mock.Anything, int64(1), "nope", "use", mock.Anything).
		Return(nil, fmt.Errorf("%w: nope", itemsvc.ErrItemNotFound)).Once()
	assert.Equal(t, http.StatusNotFound, do(h, "POST", "/api/items/nope/abilities/use/use", "{}").Code)
}

// ADR 0049 steps 7 and 10: a heal effect (dice, or full) is written and served as sent, and the
// use's answer carries what it applied to the HP ledger.
func TestItems_ADR0049Step10_HealEffectOnTheWireAndItsAppliedResult(t *testing.T) {
	h, m := newRouter(t)
	abilities := []itemsvc.Ability{{
		ID: "drink", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem},
		Effects: []itemsvc.Effect{{Kind: itemsvc.EffectHeal, Dice: "2d4+2"}, {Kind: itemsvc.EffectHeal, Full: true}},
	}}
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Abilities: &abilities}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 13}, nil).Once()
	noWarnings(m)
	rec := do(h, "PATCH", "/api/items/w", `{"abilities":[{"id":"drink","limit":{"kind":"with_item"},
		"effects":[{"kind":"heal","dice":"2d4+2"},{"kind":"heal","full":true}]}]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	m.items.EXPECT().UseAbility(mock.Anything, int64(1), "potion", "drink", mock.Anything).Return(&itemsvc.AbilityUseResult{
		ItemID: "potion", AbilityID: "drink", Name: "Potion of healing", Type: "potion", Limit: itemsvc.LimitWithItem, Remaining: 2, LogEntryID: 9,
		Effects: []itemsvc.EffectResult{{
			Kind: itemsvc.EffectHeal, Dice: "2d4+2", Rolls: []int{3, 4}, Modifier: 2, Total: 9,
			Applied: &itemsvc.Applied{HPDelta: 6, HPBefore: 20, HPAfter: 26},
		}},
	}, nil).Once()
	rec = do(h, "POST", "/api/items/potion/abilities/drink/use", "{}")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"itemId":"potion","abilityId":"drink","name":"Potion of healing","type":"potion","limit":"with_item",
		"remaining":2,"removed":false,"logEntryId":9,
		"effects":[{"kind":"heal","dice":"2d4+2","rolls":[3,4],"modifier":2,"total":9,"applied":{"hpDelta":6,"hpBefore":20,"hpAfter":26}}]}`, rec.Body.String())
}

// ADR 0049 steps 10 and 12: a buff effect (name, rounds, the buffs' effect keys) is written and
// served as sent, and the use's answer names the row it put on the list with its modifiers.
func TestItems_ADR0049Step10_BuffEffectOnTheWireAndItsAppliedResult(t *testing.T) {
	h, m := newRouter(t)
	two := 2
	abilities := []itemsvc.Ability{{
		ID: "drink", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem},
		Effects: []itemsvc.Effect{{Kind: itemsvc.EffectBuff, Name: "Protection", Rounds: 60, Buff: buffsvc.BuffEffects{AC: &two, SaveBonus: &two}}},
	}}
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Abilities: &abilities}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 14}, nil).Once()
	noWarnings(m)
	rec := do(h, "PATCH", "/api/items/w", `{"abilities":[{"id":"drink","limit":{"kind":"with_item"},
		"effects":[{"kind":"buff","name":"Protection","rounds":60,"buff":{"ac":2,"save_bonus":2}}]}]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	m.items.EXPECT().UseAbility(mock.Anything, int64(1), "potion", "drink", mock.Anything).Return(&itemsvc.AbilityUseResult{
		ItemID: "potion", AbilityID: "drink", Name: "Potion of protection", Type: "potion", Limit: itemsvc.LimitWithItem, Remaining: 2, LogEntryID: 9,
		Effects: []itemsvc.EffectResult{{
			Kind: itemsvc.EffectBuff, Rolls: []int{},
			Buff: &itemsvc.AppliedBuff{ID: "b1", Name: "Protection", Rounds: 60, Effects: buffsvc.BuffEffects{AC: &two, SaveBonus: &two}},
		}},
	}, nil).Once()
	rec = do(h, "POST", "/api/items/potion/abilities/drink/use", "{}")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"itemId":"potion","abilityId":"drink","name":"Potion of protection","type":"potion","limit":"with_item",
		"remaining":2,"removed":false,"logEntryId":9,
		"effects":[{"kind":"buff","rolls":[],"total":0,"buff":{"id":"b1","name":"Protection","rounds":60,"effects":{"ac":2,"save_bonus":2}}}]}`, rec.Body.String())
}

// ADR 0049 steps 9, 10 and 12: a spell effect is written and served as sent ({spellId, level}), the
// use's body carries the answers by effect index with the cast's own field names (an absent body
// asks nothing), and the answer carries the cast breakdown of the spell.
func TestItems_ADR0049Step10_SpellEffectOnTheWireAndItsAsks(t *testing.T) {
	h, m := newRouter(t)
	abilities := []itemsvc.Ability{{
		ID: "zap", Limit: itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 5, Left: 5, Cost: 1},
		Effects: []itemsvc.Effect{{Kind: itemsvc.EffectSpell, SpellID: "chromatic_orb", Level: 3}},
	}}
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Abilities: &abilities}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 13}, nil).Once()
	noWarnings(m)
	rec := do(h, "PATCH", "/api/items/w", `{"abilities":[{"id":"zap","limit":{"kind":"charges","max":5,"left":5,"cost":1},
		"effects":[{"kind":"spell","spellId":"chromatic_orb","level":3}]}]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	ac := 12
	onSelf := false
	asks := itemsvc.UseAsks{Spells: []itemsvc.SpellAsks{{Effect: 0, OnSelf: &onSelf, TargetAC: &ac, ItemID: "opal", ExtraSlots: []int{1, 2}, Mode: "cone"}}}
	m.items.EXPECT().UseAbility(mock.Anything, int64(1), "w", "zap", asks).Return(&itemsvc.AbilityUseResult{
		ItemID: "w", AbilityID: "zap", Name: "Wand", Type: "wand", Limit: itemsvc.LimitCharges, LogEntryID: 9,
		Effects: []itemsvc.EffectResult{{Kind: itemsvc.EffectSpell, SpellID: "chromatic_orb", Rolls: []int{}, Cast: json.RawMessage(`{"action":"item","spellId":"chromatic_orb"}`)}},
	}, nil).Once()
	rec = do(h, "POST", "/api/items/w/abilities/zap/use", `{"spells":[{"effect":0,"onSelf":false,"targetAc":12,"itemId":"opal","extraSlots":[1,2],"mode":"cone"}]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"cast":{"action":"item","spellId":"chromatic_orb"}`)

	m.items.EXPECT().UseAbility(mock.Anything, int64(1), "w", "zap", itemsvc.UseAsks{}).
		Return(nil, fmt.Errorf("%w: Chromatic Orb needs the target's AC", domain.ErrInvalidInput)).Once()
	rec = do(h, "POST", "/api/items/w/abilities/zap/use", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code, "no body: the cast's defaults; its refusal is a 400")
	m.items.EXPECT().UseAbility(mock.Anything, int64(1), "w", "zap", itemsvc.UseAsks{}).
		Return(&itemsvc.AbilityUseResult{ItemID: "w", AbilityID: "zap", Limit: itemsvc.LimitCharges, Effects: []itemsvc.EffectResult{}}, nil).Twice()
	assert.Equal(t, http.StatusOK, do(h, "POST", "/api/items/w/abilities/zap/use", "").Code, "no body")
	assert.Equal(t, http.StatusOK, do(h, "POST", "/api/items/w/abilities/zap/use", "{}").Code, "an empty body")
	assert.Equal(t, http.StatusBadRequest, do(h, "POST", "/api/items/w/abilities/zap/use", "{nope").Code, "malformed JSON, the service is not called")
}
