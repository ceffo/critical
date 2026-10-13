package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRoundReset_WireShape(t *testing.T) {
	h, m := newRouter(t)
	m.buffs.EXPECT().ResetRound(mock.Anything, int64(1)).
		Return([]buffsvc.BuffEffect{{ID: "h", Name: "Haste", Duration: 4}}, 0, nil)
	rec := do(h, "POST", "/api/round/reset", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"roundNumber":0,"expiredBuffs":[],"activeBuffs":[
		{"id":"h","name":"Haste","charges":0,"duration":4,"effects":{}}]}`, rec.Body.String())
}

func TestRoundSet(t *testing.T) {
	t.Run("returns the round state", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().SetRound(mock.Anything, int64(1), 7).Return([]buffsvc.BuffEffect{}, 7, nil)
		rec := do(h, "PUT", "/api/round", `{"round":7}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"roundNumber":7,"activeBuffs":[],"expiredBuffs":[]}`, rec.Body.String())
	})

	t.Run("round 0 is accepted", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().SetRound(mock.Anything, int64(1), 0).Return([]buffsvc.BuffEffect{}, 0, nil)
		assert.Equal(t, http.StatusOK, do(h, "PUT", "/api/round", `{"round":0}`).Code)
	})

	for name, body := range map[string]string{
		"missing round": `{}`,
		"not json":      `nope`,
		"not a number":  `{"round":"x"}`,
	} {
		t.Run("400 on "+name, func(t *testing.T) {
			h, _ := newRouter(t) // the service is never called
			rec := do(h, "PUT", "/api/round", body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), `"error"`)
		})
	}

	t.Run("400 when the service rejects a negative round", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().SetRound(mock.Anything, int64(1), -1).
			Return(nil, 0, fmt.Errorf("%w: round must be >= 0", domain.ErrInvalidInput))
		rec := do(h, "PUT", "/api/round", `{"round":-1}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `"error"`)
	})
}
