package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCritTable_Get(t *testing.T) {
	h, m := newRouter(t)
	m.cmb.EXPECT().GetCritTable(mock.Anything).Return(combatsvc.CritTable{
		Rows: []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 2}}, Placeholder: true,
	}, nil)
	rec := do(h, "GET", "/api/crit-table", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"rows":[{"from":1,"to":100,"multiplier":2}],"placeholder":true}`, rec.Body.String())
}

func TestCritTable_Put(t *testing.T) {
	t.Run("returns the saved table", func(t *testing.T) {
		h, m := newRouter(t)
		rows := []combatsvc.CritRow{{From: 1, To: 40, Multiplier: 2}, {From: 41, To: 100, Multiplier: 3}}
		m.cmb.EXPECT().SetCritTable(mock.Anything, rows).Return(combatsvc.CritTable{Rows: rows}, nil)
		rec := do(h, "PUT", "/api/crit-table", `{"rows":[{"from":1,"to":40,"multiplier":2},{"from":41,"to":100,"multiplier":3}]}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"rows":[{"from":1,"to":40,"multiplier":2},{"from":41,"to":100,"multiplier":3}],"placeholder":false}`, rec.Body.String())
	})
	t.Run("400 with the service's message naming the bad row", func(t *testing.T) {
		h, m := newRouter(t)
		m.cmb.EXPECT().SetCritTable(mock.Anything, mock.Anything).
			Return(combatsvc.CritTable{}, fmt.Errorf("%w: row 2 (50-100) overlaps the row before it", domain.ErrInvalidInput))
		rec := do(h, "PUT", "/api/crit-table", `{"rows":[{"from":1,"to":50,"multiplier":2},{"from":50,"to":100,"multiplier":3}]}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `row 2 (50-100) overlaps`)
	})
	t.Run("a decimal multiplier is passed through as entered", func(t *testing.T) {
		h, m := newRouter(t)
		rows := []combatsvc.CritRow{{From: 1, To: 50, Multiplier: 1.5}, {From: 51, To: 100, Multiplier: 2.5}}
		m.cmb.EXPECT().SetCritTable(mock.Anything, rows).Return(combatsvc.CritTable{Rows: rows}, nil)
		rec := do(h, "PUT", "/api/crit-table", `{"rows":[{"from":1,"to":50,"multiplier":1.5},{"from":51,"to":100,"multiplier":2.5}]}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"rows":[{"from":1,"to":50,"multiplier":1.5},{"from":51,"to":100,"multiplier":2.5}],"placeholder":false}`, rec.Body.String())
	})
	t.Run("a multiplier below 1 is refused with the row named", func(t *testing.T) {
		h, m := newRouter(t)
		m.cmb.EXPECT().SetCritTable(mock.Anything, mock.Anything).
			Return(combatsvc.CritTable{}, fmt.Errorf("%w: row 1 (1-100) has multiplier 0.99, it must be a number of at least 1", domain.ErrInvalidInput))
		rec := do(h, "PUT", "/api/crit-table", `{"rows":[{"from":1,"to":100,"multiplier":0.99}]}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `row 1 (1-100) has multiplier 0.99`)
	})
	t.Run("400 on a body that is not json", func(t *testing.T) {
		h, _ := newRouter(t) // the service is never called
		rec := do(h, "PUT", "/api/crit-table", `nope`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `"error"`)
	})
	// The service is never called for these: the handler refuses them first.
	for name, tc := range map[string]struct{ body, want string }{
		"rows null":         {`{"rows":null}`, "at least one range"},
		"rows missing":      {`{}`, "at least one range"},
		"rows empty":        {`{"rows":[]}`, "at least one range"},
		"text multiplier":   {`{"rows":[{"from":1,"to":50,"multiplier":2},{"from":51,"to":90,"multiplier":"abc"}]}`, "invalid request body"},
		"float from":        {`{"rows":[{"from":1.5,"to":100,"multiplier":2}]}`, "from and to must be whole numbers"},
		"string multiplier": {`{"rows":[{"from":1,"to":100,"multiplier":"x"}]}`, "invalid request body"},
	} {
		t.Run("400 on "+name, func(t *testing.T) {
			h, _ := newRouter(t)
			rec := do(h, "PUT", "/api/crit-table", tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), `"error"`)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}
