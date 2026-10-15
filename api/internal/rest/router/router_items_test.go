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

const wandJSON = `{"id":"w","name":"Wand","type":"wand","behaviour":"charges","magicalBonus":0,"ammoKind":"","count":3,
	"maxCount":4,"effectDice":"","effectSave":"","notes":""}`

var wandItem = itemsvc.Item{ID: "w", Name: "Wand", Type: "wand", Behaviour: itemsvc.BehaviourCharges, Count: 3, MaxCount: 4}

func TestItems_Create(t *testing.T) {
	h, m := newRouter(t)
	m.items.EXPECT().Create(mock.Anything, int64(1), itemsvc.ItemInput{
		Name: "Arrows +3", Type: "arrow", Behaviour: itemsvc.BehaviourStock, Count: 6, MagicBonus: 3, AmmoKind: "arrow",
	}).Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 12}, nil)
	rec := do(h, "POST", "/api/items", `{"name":"Arrows +3","type":"arrow","behaviour":"stock","count":6,"magicalBonus":3,"ammoKind":"arrow"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"item":`+wandJSON+`,"logEntryId":12}`, rec.Body.String())
}

// An invalid field comes back as 400 with the message and the field name.
func TestItems_CreateInvalidNamesTheField(t *testing.T) {
	h, m := newRouter(t)
	m.items.EXPECT().Create(mock.Anything, int64(1), mock.Anything).
		Return(nil, fmt.Errorf("wrapped: %w", &domain.FieldError{Field: "effectDice", Message: "Effect dice must look like 2d6 or 1d8+2"}))
	rec := do(h, "POST", "/api/items", `{"name":"X","behaviour":"stock","effectDice":"2x6"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"Effect dice must look like 2d6 or 1d8+2","field":"effectDice","code":"invalid_input"}`, rec.Body.String())
}

func TestItems_UpdatePatchesOnlyTheFieldsSent(t *testing.T) {
	h, m := newRouter(t)
	three := 3
	m.items.EXPECT().Update(mock.Anything, int64(1), "w", itemsvc.ItemPatch{Count: &three}).
		Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 13}, nil)
	rec := do(h, "PATCH", "/api/items/w", `{"count":3}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"item":`+wandJSON+`,"logEntryId":13}`, rec.Body.String())
}

func TestItems_Delete(t *testing.T) {
	h, m := newRouter(t)
	m.items.EXPECT().Delete(mock.Anything, int64(1), "w").Return(&itemsvc.ChangeResult{Item: wandItem, LogEntryID: 14}, nil)
	rec := do(h, "DELETE", "/api/items/w", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"item":`+wandJSON+`,"logEntryId":14}`, rec.Body.String())
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
