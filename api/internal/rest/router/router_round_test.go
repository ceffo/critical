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
		{"id":"h","name":"Haste","charges":0,"duration":4,"effects":{},"note":""}]}`, rec.Body.String())
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

func TestSavedBuffs_Wire(t *testing.T) {
	saved := []buffsvc.SavedBuff{{ID: 3, Name: "Form", Duration: 4, Note: "n", Effects: buffsvc.BuffEffects{WeaponBuff: func() *int { v := 5; return &v }()}}}
	want := `[{"id":3,"name":"Form","duration":4,"charges":0,"effects":{"weapon_buff":5},"note":"n"}]`

	t.Run("list", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().ListSaved(mock.Anything, int64(1)).Return(saved, nil)
		rec := do(h, "GET", "/api/buffs/saved", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, want, rec.Body.String())
	})
	t.Run("save", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().SaveBuff(mock.Anything, int64(1), mock.MatchedBy(func(s buffsvc.SavedBuff) bool {
			return s.Name == "Form" && s.Duration == 4 && s.Note == "n" && s.Effects.WeaponBuff != nil && *s.Effects.WeaponBuff == 5
		})).Return(saved, nil)
		rec := do(h, "POST", "/api/buffs/saved", `{"name":"Form","duration":4,"note":"n","effects":{"weapon_buff":5}}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, want, rec.Body.String())
	})
	t.Run("delete", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().DeleteSaved(mock.Anything, int64(1), int64(3)).Return([]buffsvc.SavedBuff{}, nil)
		rec := do(h, "DELETE", "/api/buffs/saved/3", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `[]`, rec.Body.String())
	})
	t.Run("delete unknown is 404, bad id 400", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().DeleteSaved(mock.Anything, int64(1), int64(9)).Return(nil, fmt.Errorf("%w: saved buff 9", domain.ErrNotFound))
		assert.Equal(t, http.StatusNotFound, do(h, "DELETE", "/api/buffs/saved/9", "").Code)
		assert.Equal(t, http.StatusBadRequest, do(h, "DELETE", "/api/buffs/saved/x", "").Code)
	})
}

func TestRoundPassTime(t *testing.T) {
	t.Run("returns Next Round's shape plus the rounds that passed", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().PassTime(mock.Anything, int64(1), 1.5).Return(
			[]buffsvc.BuffEffect{{ID: "a", Name: "Long", Duration: 10}},
			[]buffsvc.BuffEffect{{ID: "b", Name: "Short"}},
			90, 0, nil)
		rec := do(h, "POST", "/api/round/pass-time", `{"hours":1.5}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"roundNumber":0,"roundsPassed":90,
			"activeBuffs":[{"id":"a","name":"Long","charges":0,"duration":10,"effects":{},"note":""}],
			"expiredBuffs":[{"id":"b","name":"Short","charges":0,"duration":0,"effects":{},"note":""}]}`,
			rec.Body.String())
	})

	for name, body := range map[string]string{
		"missing hours": `{}`,
		"not json":      `nope`,
		"not a number":  `{"hours":"1.5"}`,
	} {
		t.Run("400 on "+name, func(t *testing.T) {
			h, _ := newRouter(t) // the service is never called
			rec := do(h, "POST", "/api/round/pass-time", body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), `"error"`)
		})
	}

	t.Run("400 with the service's message when hours give no round", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().PassTime(mock.Anything, int64(1), 0.01).
			Return(nil, nil, 0, 0, fmt.Errorf("%w: 0.01 hours is less than one round (1 minute)", domain.ErrInvalidInput))
		rec := do(h, "POST", "/api/round/pass-time", `{"hours":0.01}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "less than one round")
	})
}
