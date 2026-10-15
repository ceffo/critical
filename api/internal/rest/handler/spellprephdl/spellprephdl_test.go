package spellprephdl_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc/mocks"
	"github.com/ceffo/critical/api/internal/rest/handler/spellprephdl"
)

// The spellbook entry carries the items the service says the spell can spend,
// as id, name and count; the handler decides none of it.
func TestGetInfo_ServesTheSpendableItems(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().Info(mock.Anything, int64(1)).Return(prepsvc.Info{Spellbook: []prepsvc.SpellbookEntry{
		{ID: "alarm", Name: "Alarm", Level: 1},
		{
			ID: "chromatic_orb", Name: "Chromatic Orb", Level: 1,
			Cast:  prepsvc.CastInputs{ConsumeItemType: "gem", AttackRoll: true},
			Items: []itemsvc.Item{{ID: "ruby", Name: "Ruby", Type: "gem", Count: 2}},
		},
	}}, nil)

	h := spellprephdl.GetInfo(func(*http.Request) int64 { return 1 }, svc)
	rec := httptest.NewRecorder()
	require.NoError(t, h(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/spells/preparation", nil)))
	require.Equal(t, http.StatusOK, rec.Code)

	var got spellprephdl.InfoDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	spellbook := got.Spellbook
	require.Len(t, spellbook, 2)
	assert.Empty(t, spellbook[0].Items)
	assert.Equal(t, []spellprephdl.SpendableItemDTO{{ID: "ruby", Name: "Ruby", Count: 2}}, spellbook[1].Items)
}
