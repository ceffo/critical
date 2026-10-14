package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestItems_ListWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.items.EXPECT().List(mock.Anything).Return([]itemsvc.Item{{
		ID: "arrows_plus1", Position: 0, Name: "Arrows +1", Type: "arrow", Behaviour: itemsvc.BehaviourStock,
		MagicBonus: 1, AmmoKind: "arrow", Count: 18, MaxCount: 18,
	}}, nil)
	rec := do(h, "GET", "/api/items", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[{"id":"arrows_plus1","name":"Arrows +1","type":"arrow","behaviour":"stock","magicalBonus":1,
		"ammoKind":"arrow","count":18,"maxCount":18,"effectDice":"","effectSave":"","notes":""}]`, rec.Body.String())
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
		"removed":true,"remaining":0,"maxCount":4,"dice":"","rolls":[],"total":0,"save":"","notes":"","breakdown":"","logEntryId":9}`, rec.Body.String())
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
