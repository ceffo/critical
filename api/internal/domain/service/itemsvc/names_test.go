package itemsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

// wandBoth is the wand named in English and French.
var wandBoth = func() itemsvc.Item {
	w := wand4
	w.Name = localized.Text{"en": "Wand of lightning", "fr": "Baguette de foudre"}
	return w
}()

// ADR 0023 step 11: an edit's name replaces the bodies of the languages it
// sends and keeps the others; a blank body removes its language; the log
// lists one name change per language changed and records the name as one
// string (the English body); a name left with no language is refused.
func TestUpdate_Step11_NameEditKeepsTheOtherLanguages(t *testing.T) {
	tests := map[string]struct {
		from    localized.Text // the stored name (default: English and French)
		patch   localized.Text
		want    localized.Text
		changes string
		logged  string
	}{
		"French typed": {
			patch: localized.Text{"fr": " Baguette d'éclairs "}, want: localized.Text{"en": "Wand of lightning", "fr": "Baguette d'éclairs"},
			changes: `[{"field":"name","before":"Baguette de foudre","after":"Baguette d'éclairs"}]`, logged: "Wand of lightning",
		},
		"new language": {
			from:  localized.Text{"en": "Wand of lightning"},
			patch: localized.Text{"fr": "Baguette de foudre"}, want: localized.Text{"en": "Wand of lightning", "fr": "Baguette de foudre"},
			changes: `[{"field":"name","before":"","after":"Baguette de foudre"}]`, logged: "Wand of lightning",
		},
		"English removed": {
			patch: localized.Text{"en": " "}, want: localized.Text{"fr": "Baguette de foudre"},
			changes: `[{"field":"name","before":"Wand of lightning","after":""}]`, logged: "Baguette de foudre",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t)
			f := deps(t)
			svc, _ := itemsvc.New(dal, f.Deps)
			before := wandBoth
			if tt.from != nil {
				before.Name = tt.from
			}
			dal.EXPECT().Get(mock.Anything, int64(1), "wand_lightning").Return(before, nil)
			after := before
			after.Name = tt.want
			dal.EXPECT().Update(mock.Anything, int64(1), after).Return(nil).Once()
			var got actionlog.NewEntry
			f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(3), nil)

			res, err := svc.Update(context.Background(), 1, "wand_lightning", itemsvc.ItemPatch{Name: tt.patch})
			require.NoError(t, err)
			assert.Equal(t, tt.want, res.Item.Name)
			var p struct {
				Name    string          `json:"name"`
				Changes json.RawMessage `json:"changes"`
			}
			require.NoError(t, json.Unmarshal(got.Payload, &p))
			assert.JSONEq(t, tt.changes, string(p.Changes))
			assert.Equal(t, tt.logged, p.Name)
		})
	}

	t.Run("no language left", func(t *testing.T) {
		dal := mocks.NewMockDAL(t) // no Update
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "wand_lightning").Return(wandBoth, nil)
		_, err := svc.Update(context.Background(), 1, "wand_lightning", itemsvc.ItemPatch{Name: localized.Text{"en": "", "fr": " "}})
		var fe *domain.FieldError
		require.True(t, errors.As(err, &fe), "got %v", err)
		assert.Equal(t, "name", fe.Field)
	})
}

// ADR 0023 step 11: an item created with a name typed in French only is
// stored in French only and gets its id from that name.
func TestCreate_Step11_NameInThePlayersLanguage(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	dal.EXPECT().Insert(mock.Anything, int64(1), mock.MatchedBy(func(it itemsvc.Item) bool {
		return it.ID == "corde" && it.Name.Equal(localized.Text{"fr": "Corde"})
	})).Return(nil).Once()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
	res, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{Name: localized.Text{"fr": " Corde "}, Behaviour: itemsvc.BehaviourStock, Count: 1})
	require.NoError(t, err)
	assert.Equal(t, localized.Text{"fr": "Corde"}, res.Item.Name)
}

// ADR 0023 step 11: popping restores the whole name of a snapshot; a
// snapshot written before names were localized holds a plain string, which
// restores the English body only and keeps the other languages the item has
// now (an item that is gone comes back with that one).
func TestRestorers_Step11_PlainNameSnapshotKeepsTheOtherLanguages(t *testing.T) {
	restore := func(dal *mocks.MockDAL, state string) error {
		svc, _ := itemsvc.New(dal, deps(t).Deps)
		return svc.Restorers()[itemsvc.StateKeyItems].Restore(context.Background(), 1, json.RawMessage(state))
	}
	const tail = `"type":"wand","behaviour":"charges","magicalBonus":0,"ammoKind":"","maxCount":4,"effectDice":"6d6","effectSave":"rod_staff_wand","notes":"","powers":[]}}]`
	t.Run("plain string", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Get(mock.Anything, int64(1), "wand_lightning").Return(wandBoth, nil).Once()
		want := wandBoth
		want.Name = localized.Text{"en": "Old wand", "fr": "Baguette de foudre"}
		dal.EXPECT().Update(mock.Anything, int64(1), want).Return(nil).Once()
		require.NoError(t, restore(dal, `[{"id":"wand_lightning","count":4,"replace":true,"item":{"position":4,"name":"Old wand",`+tail))
	})
	t.Run("language map", func(t *testing.T) {
		dal := mocks.NewMockDAL(t) // no Get: the snapshot is the whole truth
		want := wandBoth
		want.Name = localized.Text{"en": "Old wand"}
		dal.EXPECT().Update(mock.Anything, int64(1), want).Return(nil).Once()
		require.NoError(t, restore(dal, `[{"id":"wand_lightning","count":4,"replace":true,"item":{"position":4,"name":{"en":"Old wand"},`+tail))
	})
	t.Run("plain string, item gone", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		dal.EXPECT().Get(mock.Anything, int64(1), "wand_lightning").Return(itemsvc.Item{}, itemsvc.ErrItemNotFound).Once()
		want := wand4
		want.Name = localized.Text{"en": "Old wand"}
		dal.EXPECT().Update(mock.Anything, int64(1), want).Return(itemsvc.ErrItemNotFound).Once()
		dal.EXPECT().Insert(mock.Anything, int64(1), want).Return(nil).Once()
		require.NoError(t, restore(dal, `[{"id":"wand_lightning","count":4,"replace":true,"item":{"position":4,"name":"Old wand",`+tail))
	})
}

// unknownLanguages are names in a code that is not a UI language.
var unknownLanguages = map[string]localized.Text{
	"unknown code": {"zz": "Corde"},
	"empty code":   {"": "Corde"},
	"blank body":   {"en": "Corde", "de": ""},
	"region":       {"fr-FR": "Corde"},
}

// ADR 0023 step 11: a create whose name holds a code that is not a UI
// language is refused with a name field error and stores nothing.
func TestCreate_Step11_RefusesAnUnknownLanguage(t *testing.T) {
	for name, text := range unknownLanguages {
		t.Run(name, func(t *testing.T) {
			svc, _ := itemsvc.New(mocks.NewMockDAL(t), deps(t).Deps)
			_, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{Name: text, Behaviour: itemsvc.BehaviourStock, Count: 1})
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
			svc, _ := itemsvc.New(mocks.NewMockDAL(t), deps(t).Deps)
			_, err := svc.Update(context.Background(), 1, "wand_lightning", itemsvc.ItemPatch{Name: text})
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, "name", fe.Field)
		})
	}
}
