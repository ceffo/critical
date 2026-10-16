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

var bastardBoth = func() weaponsvc.Weapon {
	w := bastard
	w.Name = localized.Text{"en": "Bastard Sword +2", "fr": "Épée bâtarde +2"}
	return w
}()

// ADR 0023 step 11: a weapon name edit replaces the languages sent and
// keeps the others, with one name change per language in the log.
func TestUpdate_Step11_NameEditKeepsTheOtherLanguages(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := weaponsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, "bastard_sword").Return(bastardBoth, nil)
	after := bastardBoth
	after.Name = localized.Text{"en": "Bastard Sword +2", "fr": "Épée bâtarde"}
	dal.EXPECT().Update(mock.Anything, after).Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(3), nil)

	res, err := svc.Update(context.Background(), 1, "bastard_sword", weaponsvc.WeaponPatch{Name: localized.Text{"fr": "Épée bâtarde"}})
	require.NoError(t, err)
	assert.Equal(t, after.Name, res.Weapon.Name)
	assert.Contains(t, string(got.Payload), `"name":"Bastard Sword +2"`)
	assert.Contains(t, string(got.Payload), `"changes":[{"field":"name","before":"Épée bâtarde +2","after":"Épée bâtarde"}]`)
}

// ADR 0023 step 11: a weapon snapshot written before names were localized
// (a plain string) restores the English body only and keeps the other
// languages; a language map restores the whole name.
func TestRestorers_Step11_PlainNameSnapshotKeepsTheOtherLanguages(t *testing.T) {
	restore := func(dal *mocks.MockDAL, state string) error {
		svc, _ := weaponsvc.New(dal, deps(t).Deps)
		return svc.Restorers()[weaponsvc.StateKeyWeapons].Restore(context.Background(), 1, json.RawMessage(state))
	}
	const tail = `"damageSm":"2d4","damageL":"2d8","magicToHit":2,"magicDamage":2,"kind":"melee","twoHanded":true,"ammoKind":""}`
	t.Run("plain string", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Get(mock.Anything, "bastard_sword").Return(bastardBoth, nil).Once()
		want := bastardBoth
		want.Name = localized.Text{"en": "Old sword", "fr": "Épée bâtarde +2"}
		dal.EXPECT().Update(mock.Anything, want).Return(nil).Once()
		require.NoError(t, restore(dal, `{"id":"bastard_sword","position":0,"name":"Old sword",`+tail))
	})
	t.Run("language map", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		want := bastardBoth
		want.Name = localized.Text{"en": "Old sword"}
		dal.EXPECT().Update(mock.Anything, want).Return(nil).Once()
		require.NoError(t, restore(dal, `{"id":"bastard_sword","position":0,"name":{"en":"Old sword"},`+tail))
	})
}

// unknownLanguages are names in a code that is not a UI language.
var unknownLanguages = map[string]localized.Text{
	"unknown code": {"zz": "Hache"},
	"empty code":   {"": "Hache"},
	"blank body":   {"en": "Axe", "de": ""},
	"region":       {"fr-FR": "Hache"},
}

// ADR 0023 step 11: a create whose name holds a code that is not a UI
// language is refused with a name field error and stores nothing.
func TestCreate_Step11_RefusesAnUnknownLanguage(t *testing.T) {
	for name, text := range unknownLanguages {
		t.Run(name, func(t *testing.T) {
			svc, _ := weaponsvc.New(mocks.NewMockDAL(t), deps(t).Deps)
			_, err := svc.Create(context.Background(), 1, weaponsvc.WeaponInput{Name: text, DamageSM: "1d6", DamageL: "1d4", Kind: "melee"})
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, "name", fe.Field)
		})
	}
}

// ADR 0023 step 11: so is an edit's, before anything is read or written.
func TestUpdate_Step11_RefusesAnUnknownLanguage(t *testing.T) {
	for name, text := range unknownLanguages {
		t.Run(name, func(t *testing.T) {
			svc, _ := weaponsvc.New(mocks.NewMockDAL(t), deps(t).Deps)
			_, err := svc.Update(context.Background(), 1, "bastard_sword", weaponsvc.WeaponPatch{Name: text})
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, "name", fe.Field)
		})
	}
}
