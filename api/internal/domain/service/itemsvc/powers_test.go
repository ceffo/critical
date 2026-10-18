package itemsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

// hammer is the Power Gem's thunder hammer (ADR 0016 step 16).
var hammer = itemsvc.Power{
	ID: "thunder_hammer", MinStr: ptr(18),
	Effects: buffsvc.BuffEffects{WeaponBuff: ptr(5), Multiplier: ptr(2.0), WeaponKind: buffsvc.WeaponMelee},
}

// gem is the Power Gem as seeded: daily, 1 a day, with its hammer.
var gem = itemsvc.Item{
	ID: "power_gem_dispel", Position: 5, Name: localized.Of("Power Gem"), Type: "gem", Behaviour: itemsvc.BehaviourDaily,
	Count: 1, MaxCount: 1, Notes: "Dispel Magic once a day", Powers: []itemsvc.Power{hammer},
}

// ADR 0016 step 9: a power never has a Strength effect (str_bonus, str_max;
// min_str is its trigger, not an effect), so it cannot change the STR its
// trigger reads; its id is a slug unique within the item, its trigger a
// score in 1..25, its effects pass the buffs' checks.
func TestItemPowers_Step9_StrengthEffectsRefused(t *testing.T) {
	tests := []struct {
		name   string
		powers []itemsvc.Power
		key    string
	}{
		{"str_bonus", []itemsvc.Power{{ID: "belt", Effects: buffsvc.BuffEffects{StrBonus: ptr(4)}}}, "str_bonus"},
		{"str_max", []itemsvc.Power{{ID: "belt", Effects: buffsvc.BuffEffects{StrMax: ptr(19)}}}, "str_max"},
		{"min_str as an effect", []itemsvc.Power{{ID: "belt", Effects: buffsvc.BuffEffects{MinStr: ptr(18)}}}, "min_str"},
		{"trigger out of range", []itemsvc.Power{{ID: "belt", MinStr: ptr(26)}}, "min_str"},
		{"id not a slug", []itemsvc.Power{{ID: "Thunder Hammer"}}, "slug"},
		{"duplicate id", []itemsvc.Power{hammer, hammer}, "duplicate"},
		{"unknown weapon kind", []itemsvc.Power{{ID: "x", Effects: buffsvc.BuffEffects{WeaponKind: "thrown"}}}, "weapon_kind"},
		{"weapon +0", []itemsvc.Power{{ID: "x", Effects: buffsvc.BuffEffects{WeaponBuff: ptr(0)}}}, "weapon_buff"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := itemsvc.ValidatePowers(tt.powers)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.Contains(t, err.Error(), tt.key)
		})
	}
	t.Run("the hammer is valid", func(t *testing.T) {
		require.NoError(t, itemsvc.ValidatePowers([]itemsvc.Power{hammer}))
	})
}

// ADR 0016 step 9: the powers are served whatever the item's count: the
// gem's hammer is there at 0 dispels left, after the gem's other items,
// with the item's id and stored name.
func TestItemPowers_Step9_CountDoesNotGateThePowers(t *testing.T) {
	spent := gem
	spent.Count = 0
	ring := itemsvc.Item{
		ID: "ring", Name: localized.Of("Ring"), Behaviour: itemsvc.BehaviourStock, Count: 1,
		Powers: []itemsvc.Power{{ID: "luck", Effects: buffsvc.BuffEffects{ToHit: ptr(1)}}},
	}
	arrows := itemsvc.Item{ID: "arrows", Name: localized.Of("Arrows"), Behaviour: itemsvc.BehaviourStock, Count: 0}
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().List(mock.Anything, int64(1)).Return([]itemsvc.Item{arrows, ring, spent}, nil)

	got, err := svc.ItemPowers(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []buffsvc.ItemPower{
		{ItemID: "ring", ItemName: "Ring", PowerID: "luck", Effects: buffsvc.BuffEffects{ToHit: ptr(1)}},
		{ItemID: "power_gem_dispel", ItemName: "Power Gem", PowerID: "thunder_hammer", MinStr: ptr(18), Effects: hammer.Effects},
	}, got)
}

// ADR 0016 step 15: an item's powers are part of its items:<id> snapshot,
// so every entry carries them and popping it restores them with the item: a
// removed gem comes back with its hammer, an edit popped keeps it.
func TestItemPowers_Step15_SnapshotCarriesThePowers(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "power_gem_dispel").Return(gem, nil).Once()
	dal.EXPECT().Delete(mock.Anything, int64(1), "power_gem_dispel").Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(3), nil)

	_, err := svc.Delete(context.Background(), 1, "power_gem_dispel")
	require.NoError(t, err)
	require.Len(t, got.Snapshots, 1)
	var before []map[string]any
	require.NoError(t, json.Unmarshal(got.Snapshots[0].Before, &before))
	assert.JSONEq(t, `[{"id":"thunder_hammer","trigger":{"min_str":18},"effects":{"weapon_buff":5,"multiplier":2,"weapon_kind":"melee"}}]`,
		mustJSON(t, before[0]["item"].(map[string]any)["powers"]))

	// Popping the remove inserts the gem back whole, hammer included.
	restore := mocks.NewMockDAL(t)
	rsvc, _ := itemsvc.New(restore, deps(t).Deps)
	restore.EXPECT().Update(mock.Anything, int64(1), gem).Return(itemsvc.ErrItemNotFound).Once()
	restore.EXPECT().Insert(mock.Anything, int64(1), gem).Return(nil).Once()
	require.NoError(t, rsvc.Restorers()[itemsvc.StateKeyItems].Restore(context.Background(), 1, got.Snapshots[0].Before))
}

// ADR 0016 step 15: an edit never sets powers (the patch has no field for
// them): editing the gem's name keeps its hammer, in the row and in the
// snapshot's "after"; a created item has none.
func TestItemPowers_Step15_EditBodyCannotSetPowers(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "power_gem_dispel").Return(gem, nil).Once()
	renamed := gem
	renamed.Name = localized.Text{"en": gem.Name["en"], "fr": "Gemme de puissance"}
	dal.EXPECT().Update(mock.Anything, int64(1), renamed).Return(nil).Once()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(4), nil).Once()

	res, err := svc.Update(context.Background(), 1, "power_gem_dispel", itemsvc.ItemPatch{Name: localized.Text{"fr": "Gemme de puissance"}})
	require.NoError(t, err)
	assert.Equal(t, []itemsvc.Power{hammer}, res.Item.Powers)

	dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil).Once()
	dal.EXPECT().Insert(mock.Anything, int64(1), mock.MatchedBy(func(it itemsvc.Item) bool { return it.Powers == nil })).Return(nil).Once()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(5), nil).Once()
	created, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{Name: localized.Of("Gem"), Behaviour: itemsvc.BehaviourDaily, Count: 1, MaxCount: 1})
	require.NoError(t, err)
	assert.Nil(t, created.Item.Powers)
}

// ADR 0016 step 15, an entry recorded before powers existed: its snapshot
// has no "powers" key, and popping it leaves the powers the item has now.
func TestItemPowers_Step15_OldSnapshotKeepsTheStoredPowers(t *testing.T) {
	old := json.RawMessage(`[{"id":"power_gem_dispel","count":1,"replace":true,"item":{"position":5,"name":"Power Gem: Dispel Magic",
		"type":"gem","behaviour":"daily","magicalBonus":0,"ammoKind":"","maxCount":1,"effectDice":"","effectSave":"","notes":"old"}}]`)
	dal := mocks.NewMockDAL(t)
	svc, _ := itemsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "power_gem_dispel").Return(gem, nil).Once()
	want := gem
	want.Name, want.Notes = localized.Of("Power Gem: Dispel Magic"), "old"
	dal.EXPECT().Update(mock.Anything, int64(1), want).Return(nil).Once()
	require.NoError(t, svc.Restorers()[itemsvc.StateKeyItems].Restore(context.Background(), 1, old))
}

// The stored encoding round-trips and refuses an unknown key (a typo in the
// seed or a migration would otherwise drop an effect silently).
func TestPowers_EncodeDecode(t *testing.T) {
	raw, err := itemsvc.EncodePowers([]itemsvc.Power{hammer})
	require.NoError(t, err)
	assert.JSONEq(t, `[{"id":"thunder_hammer","trigger":{"min_str":18},"effects":{"weapon_buff":5,"multiplier":2,"weapon_kind":"melee"}}]`, raw)
	back, err := itemsvc.DecodePowers(raw)
	require.NoError(t, err)
	assert.Equal(t, []itemsvc.Power{hammer}, back)

	none, err := itemsvc.EncodePowers(nil)
	require.NoError(t, err)
	assert.Equal(t, "[]", none)
	for _, empty := range []string{"", "[]"} {
		got, err := itemsvc.DecodePowers(empty)
		require.NoError(t, err)
		assert.Nil(t, got)
	}
	_, err = itemsvc.DecodePowers(`[{"id":"x","effects":{"weapon_bluff":5}}]`)
	require.Error(t, err)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}
