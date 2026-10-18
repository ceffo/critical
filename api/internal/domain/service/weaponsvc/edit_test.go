package weaponsvc_test

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
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc/mocks"
)

func ptr[T any](v T) *T { return &v }

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type fixture struct {
	weaponsvc.Deps
	log *mocks.MockLog
}

func deps(t *testing.T) fixture {
	log := mocks.NewMockLog(t)
	return fixture{Deps: weaponsvc.Deps{Log: log, Tx: noTx{}}, log: log}
}

func TestNew_NilDependency(t *testing.T) {
	_, err := weaponsvc.New(nil, weaponsvc.Deps{})
	require.Error(t, err)
}

var bastard = weaponsvc.Weapon{
	ID: "bastard_sword", Position: 0, Name: localized.Of("Bastard Sword +2"), DamageSM: "2d4", DamageL: "2d8",
	MagicToHit: 2, MagicDamage: 2, Kind: "melee", TwoHanded: true,
}

// Every bad field is rejected with a *domain.FieldError naming the wire
// field, before anything is stored or logged.
func TestCreate_ValidationNamesTheField(t *testing.T) {
	good := weaponsvc.WeaponInput{Name: localized.Of("Long Bow"), DamageSM: "1d8", DamageL: "1d8", Kind: "missile", AmmoKind: "arrow"}
	cases := map[string]struct {
		mut   func(*weaponsvc.WeaponInput)
		field string
	}{
		"empty name":       {func(i *weaponsvc.WeaponInput) { i.Name = localized.Of(" ") }, "name"},
		"bad damage 2x6":   {func(i *weaponsvc.WeaponInput) { i.DamageSM = "2x6" }, "damage_sm"},
		"empty damage":     {func(i *weaponsvc.WeaponInput) { i.DamageSM = "" }, "damage_sm"},
		"modifier dice":    {func(i *weaponsvc.WeaponInput) { i.DamageSM = "1d8+2" }, "damage_sm"},
		"zero dice":        {func(i *weaponsvc.WeaponInput) { i.DamageL = "0d6" }, "damage_l"},
		"one sided":        {func(i *weaponsvc.WeaponInput) { i.DamageL = "1d1" }, "damage_l"},
		"huge dice":        {func(i *weaponsvc.WeaponInput) { i.DamageL = "101d6" }, "damage_l"},
		"to-hit too high":  {func(i *weaponsvc.WeaponInput) { i.MagicToHit = 11 }, "magic_to_hit"},
		"damage too low":   {func(i *weaponsvc.WeaponInput) { i.MagicDamage = -11 }, "magic_damage"},
		"unknown kind":     {func(i *weaponsvc.WeaponInput) { i.Kind = "x" }, "kind"},
		"melee fires ammo": {func(i *weaponsvc.WeaponInput) { i.Kind = "melee" }, "ammo_kind"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := weaponsvc.New(mocks.NewMockDAL(t), deps(t).Deps) // nothing is touched
			in := good
			c.mut(&in)
			_, err := svc.Create(context.Background(), 1, in)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, c.field, fe.Field)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.NotEmpty(t, fe.Message)
		})
	}
}

// A new weapon gets a unique id from its name and the next position, and one
// weapon_add entry whose snapshot says "did not exist" before.
func TestCreate_InsertsLogsAndSnapshots(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := weaponsvc.New(dal, f.Deps)
	dal.EXPECT().List(mock.Anything, int64(1)).Return([]weaponsvc.Weapon{{ID: "long_bow", Position: 5}, {ID: "x", Position: 3}}, nil)
	want := weaponsvc.Weapon{
		ID: "long_bow_2", Position: 6, Name: localized.Of("Long Bow"), DamageSM: "1d8", DamageL: "1d8", MagicToHit: 1, MagicDamage: 2,
		Kind: "missile", TwoHanded: true, AmmoKind: "arrow",
	}
	dal.EXPECT().Insert(mock.Anything, int64(1), want).Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(5), nil)

	res, err := svc.Create(context.Background(), 1, weaponsvc.WeaponInput{
		Name: localized.Of(" Long Bow "), DamageSM: "1D8", DamageL: "1d8", MagicToHit: 1, MagicDamage: 2, Kind: "missile", TwoHanded: true, AmmoKind: " Arrow ",
	})
	require.NoError(t, err)
	assert.Equal(t, &weaponsvc.ChangeResult{Weapon: want, LogEntryID: 5}, res)
	assert.Equal(t, actionlog.KindWeaponAdd, got.Kind)
	assert.False(t, actionlog.IsRoll(got.Kind), "a pop-only entry that is not a roll card")
	require.Len(t, got.Snapshots, 1)
	assert.Equal(t, "weapons:long_bow_2", got.Snapshots[0].Key)
	assert.JSONEq(t, `{"id":"long_bow_2","absent":true,"position":0,"name":null,"damageSm":"","damageL":"","magicToHit":0,"magicDamage":0,"kind":"","twoHanded":false,"ammoKind":""}`,
		string(got.Snapshots[0].Before))
	assert.Contains(t, string(got.Snapshots[0].After), `"name":{"en":"Long Bow"}`)
	assert.Contains(t, string(got.Payload), `"ammo_kind":"arrow"`)
}

// Bastard Sword +2 -> +3 to hit and damage: one weapon_edit entry with both
// whole weapons and both changed fields listed.
func TestUpdate_ChangesFieldsAndLogsBeforeAfter(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := weaponsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "bastard_sword").Return(bastard, nil)
	after := bastard
	after.MagicToHit, after.MagicDamage = 3, 3
	dal.EXPECT().Update(mock.Anything, int64(1), after).Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(8), nil)

	res, err := svc.Update(context.Background(), 1, "bastard_sword", weaponsvc.WeaponPatch{MagicToHit: ptr(3), MagicDamage: ptr(3)})
	require.NoError(t, err)
	assert.Equal(t, after, res.Weapon)
	assert.Equal(t, actionlog.KindWeaponEdit, got.Kind)
	assert.Empty(t, got.Summary, "the UI words the change from its payload")
	assert.Contains(t, string(got.Snapshots[0].Before), `"magicToHit":2`)
	assert.Contains(t, string(got.Snapshots[0].After), `"magicToHit":3`)
	assert.Contains(t, string(got.Payload), `"changes":[{"field":"magic_to_hit","before":"2","after":"3"},{"field":"magic_damage","before":"2","after":"3"}]`)
}

// Switching a bow to melee drops its ammo kind; a melee weapon cannot be
// given one.
func TestUpdate_KindAndAmmo(t *testing.T) {
	bow := weaponsvc.Weapon{ID: "bow", Name: localized.Of("Bow"), DamageSM: "1d6", DamageL: "1d6", Kind: "missile", AmmoKind: "arrow"}
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := weaponsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "bow").Return(bow, nil)
	melee := bow
	melee.Kind, melee.AmmoKind = "melee", ""
	dal.EXPECT().Update(mock.Anything, int64(1), melee).Return(nil).Once()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
	_, err := svc.Update(context.Background(), 1, "bow", weaponsvc.WeaponPatch{Kind: ptr("melee")})
	require.NoError(t, err)

	dal2 := mocks.NewMockDAL(t)
	svc2, _ := weaponsvc.New(dal2, deps(t).Deps)
	dal2.EXPECT().Get(mock.Anything, int64(1), "bow").Return(bow, nil)
	_, err = svc2.Update(context.Background(), 1, "bow", weaponsvc.WeaponPatch{Kind: ptr("melee"), AmmoKind: ptr("arrow")})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "ammo_kind", fe.Field)
}

func TestUpdate_RejectsBadValuesAndNoOps(t *testing.T) {
	for name, c := range map[string]struct {
		patch weaponsvc.WeaponPatch
		field string
	}{
		"empty name": {weaponsvc.WeaponPatch{Name: localized.Text{"en": " "}}, "name"},
		"bad dice":   {weaponsvc.WeaponPatch{DamageL: ptr("big")}, "damage_l"},
		"bonus":      {weaponsvc.WeaponPatch{MagicToHit: ptr(99)}, "magic_to_hit"},
		"nothing":    {weaponsvc.WeaponPatch{MagicToHit: ptr(2)}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t) // no Update
			svc, _ := weaponsvc.New(dal, deps(t).Deps)
			dal.EXPECT().Get(mock.Anything, int64(1), "bastard_sword").Return(bastard, nil)
			_, err := svc.Update(context.Background(), 1, "bastard_sword", c.patch)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			var fe *domain.FieldError
			if c.field != "" {
				require.ErrorAs(t, err, &fe)
				assert.Equal(t, c.field, fe.Field)
			}
		})
	}
}

func TestUpdateAndDelete_UnknownWeapon(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	svc, _ := weaponsvc.New(dal, deps(t).Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "x").Return(weaponsvc.Weapon{}, weaponsvc.ErrWeaponNotFound)
	_, err := svc.Update(context.Background(), 1, "x", weaponsvc.WeaponPatch{Name: localized.Text{"en": "y"}})
	assert.ErrorIs(t, err, weaponsvc.ErrWeaponNotFound)
	_, err = svc.Delete(context.Background(), 1, "x")
	assert.ErrorIs(t, err, weaponsvc.ErrWeaponNotFound)
}

func TestDelete_RemovesAndLogsWholeWeapon(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := weaponsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "bastard_sword").Return(bastard, nil)
	dal.EXPECT().Delete(mock.Anything, int64(1), "bastard_sword").Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(2), nil)

	res, err := svc.Delete(context.Background(), 1, "bastard_sword")
	require.NoError(t, err)
	assert.Equal(t, bastard, res.Weapon)
	assert.Equal(t, actionlog.KindWeaponRemove, got.Kind)
	assert.Contains(t, string(got.Payload), `"name":"Bastard Sword +2"`)
	assert.Contains(t, string(got.Snapshots[0].Before), `"name":{"en":"Bastard Sword +2"}`)
	assert.Contains(t, string(got.Snapshots[0].After), `"absent":true`)
}

// Restoring: a weapon added by hand is deleted, an edited one overwritten
// whole, a deleted one inserted back whole.
func TestRestorers_UndoCreateEditDelete(t *testing.T) {
	restore := func(dal *mocks.MockDAL, before json.RawMessage) error {
		svc, _ := weaponsvc.New(dal, deps(t).Deps)
		return svc.Restorers()[weaponsvc.StateKeyWeapons].Restore(context.Background(), 1, before)
	}
	t.Run("create", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Delete(mock.Anything, int64(1), "new").Return(nil).Once()
		require.NoError(t, restore(dal, json.RawMessage(`{"id":"new","absent":true}`)))
	})
	before := json.RawMessage(`{"id":"bastard_sword","position":0,"name":{"en":"Bastard Sword +2"},"damageSm":"2d4","damageL":"2d8",
		"magicToHit":2,"magicDamage":2,"kind":"melee","twoHanded":true,"ammoKind":""}`)
	t.Run("edit", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Update(mock.Anything, int64(1), bastard).Return(nil).Once()
		require.NoError(t, restore(dal, before))
	})
	t.Run("delete", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Update(mock.Anything, int64(1), bastard).Return(weaponsvc.ErrWeaponNotFound).Once()
		dal.EXPECT().Insert(mock.Anything, int64(1), bastard).Return(nil).Once()
		require.NoError(t, restore(dal, before))
	})
	t.Run("garbage", func(t *testing.T) {
		require.Error(t, restore(mocks.NewMockDAL(t), json.RawMessage(`{"name":"no id"}`)))
	})
}
