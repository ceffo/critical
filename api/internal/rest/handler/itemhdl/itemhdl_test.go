package itemhdl_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/itemhdl"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

type asker func(spellID string, level int) (prepsvc.SpellAsks, error)

func (a asker) AsksFor(spellID string, level int, _ []itemsvc.Item) (prepsvc.SpellAsks, error) {
	return a(spellID, level)
}

// ADR 0049 steps 9 and 10: the item list serves each spell effect's asks; one effect whose spell data
// cannot be read at its level is served without asks and the list still answers 200 with every item.
func TestList_ServesAsksAndSurvivesOneBadSpellEffect(t *testing.T) {
	cast := func(spell string) []itemsvc.Ability {
		return []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}, Effects: []itemsvc.Effect{{Kind: itemsvc.EffectSpell, SpellID: spell, Level: 13}}}}
	}
	items := []itemsvc.Item{
		{ID: "good", Name: localized.Of("Good"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicScroll, Count: 1, Abilities: cast("chromatic_orb")},
		{ID: "bad", Name: localized.Of("Bad"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicScroll, Count: 1, Abilities: cast("broken")},
		{ID: "rope", Name: localized.Of("Rope"), Category: itemsvc.CategoryGear, Count: 1, Abilities: []itemsvc.Ability{}},
	}
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().List(mock.Anything, int64(1)).Return(items, nil)
	svc.EXPECT().Reviews(mock.Anything, int64(1)).Return(nil, nil)
	svc.EXPECT().Warnings(mock.Anything, int64(1), items).Return(nil, nil)
	a := asker(func(spell string, _ int) (prepsvc.SpellAsks, error) {
		if spell == "broken" {
			return prepsvc.SpellAsks{}, errors.New("no steps at this level")
		}
		return prepsvc.SpellAsks{Cast: prepsvc.CastInputs{ConsumeItemType: "gem", AttackRoll: true}}, nil
	})

	rec := httptest.NewRecorder()
	render.Handle(itemhdl.List(func(*http.Request) int64 { return 1 }, svc, a)).ServeHTTP(rec,
		httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/items", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var got []wire.ItemDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 3)
	good := (*got[0].Abilities)[0].Effects[0].Asks
	require.NotNil(t, good)
	assert.True(t, good.AttackRoll)
	assert.Equal(t, "gem", good.ConsumeItemType)
	assert.Nil(t, (*got[1].Abilities)[0].Effects[0].Asks, "the bad effect is served without asks")
}
