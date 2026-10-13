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
	t.Run("400 on a body that is not json", func(t *testing.T) {
		h, _ := newRouter(t) // the service is never called
		rec := do(h, "PUT", "/api/crit-table", `nope`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `"error"`)
	})
}
