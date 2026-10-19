package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

func TestRoundReset_WireShape(t *testing.T) {
	h, m := newRouter(t)
	m.buffs.EXPECT().ResetRound(mock.Anything, int64(1)).
		Return([]buffsvc.BuffEffect{{ID: "h", Name: "Haste", Duration: 4}}, 0, nil)
	rec := do(h, "POST", "/api/round/reset", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"roundNumber":0,"expiredBuffs":[],"activeBuffs":[
		{"id":"h","name":"Haste","charges":0,"duration":4,"effects":{},"note":"","startRound":0,"chargesTotal":0}]}`, rec.Body.String())
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
			[]buffsvc.BuffEffect{{ID: "a", Name: "Long", Duration: 10, StartRound: 3}},
			[]buffsvc.BuffEffect{{ID: "b", Name: "Short"}},
			90, 0, buffsvc.EndResult{}, nil)
		rec := do(h, "POST", "/api/round/pass-time", `{"hours":1.5}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"roundNumber":0,"roundsPassed":90,
			"activeBuffs":[{"id":"a","name":"Long","charges":0,"duration":10,"effects":{},"note":"","startRound":3,"chargesTotal":0}],
			"expiredBuffs":[{"id":"b","name":"Short","charges":0,"duration":0,"effects":{},"note":"","startRound":0,"chargesTotal":0}]}`,
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
			Return(nil, nil, 0, 0, buffsvc.EndResult{}, fmt.Errorf("%w: 0.01 hours is less than one round (1 minute)", domain.ErrInvalidInput))
		rec := do(h, "POST", "/api/round/pass-time", `{"hours":0.01}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "less than one round")
	})
}

func TestRoundPassTimePreview(t *testing.T) {
	t.Run("returns the rounds and the buffs that would expire", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().PreviewPassTime(mock.Anything, int64(1), 1.5).Return(
			90, []buffsvc.BuffEffect{{ID: "b", Name: "Short", Duration: 60, StartRound: 2}}, nil)
		rec := do(h, "GET", "/api/round/pass-time/preview?hours=1.5", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"rounds":90,
			"expiring":[{"id":"b","name":"Short","charges":0,"duration":60,"effects":{},"note":"","startRound":2,"chargesTotal":0}]}`,
			rec.Body.String())
	})

	t.Run("nothing expiring is an empty list", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().PreviewPassTime(mock.Anything, int64(1), 0.5).Return(30, []buffsvc.BuffEffect{}, nil)
		rec := do(h, "GET", "/api/round/pass-time/preview?hours=0.5", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"rounds":30,"expiring":[]}`, rec.Body.String())
	})

	for name, q := range map[string]string{"missing hours": "", "not a number": "?hours=abc"} {
		t.Run("400 on "+name, func(t *testing.T) {
			h, _ := newRouter(t) // the service is never called
			rec := do(h, "GET", "/api/round/pass-time/preview"+q, "")
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), `"error"`)
		})
	}

	t.Run("400 with the service's message when hours give no round", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().PreviewPassTime(mock.Anything, int64(1), 0.01).
			Return(0, nil, fmt.Errorf("%w: 0.01 hours is less than one round (1 minute)", domain.ErrInvalidInput))
		rec := do(h, "GET", "/api/round/pass-time/preview?hours=0.01", "")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "less than one round")
	})
}

func TestRoundInitiative_WireShape(t *testing.T) {
	t.Run("GET /api/round carries the initiative", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().GetRound(mock.Anything, int64(1)).Return(3, nil)
		m.buffs.EXPECT().GetInitiative(mock.Anything, int64(1)).Return(&buffsvc.Initiative{D10: 4, LogEntryID: 12}, nil)
		rec := do(h, "GET", "/api/round", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"round":3,"initiative":{"value":4,"d10":4,"modifiers":[],"endOfRound":false,"logEntryId":12}}`, rec.Body.String())
	})

	// ADR 0035 item 6.3: the value is the total; the d10 and the modifiers
	// are the breakdown, endOfRound a casting time of a round or more.
	t.Run("GET /api/round carries the breakdown", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().GetRound(mock.Anything, int64(1)).Return(3, nil)
		m.buffs.EXPECT().GetInitiative(mock.Anything, int64(1)).Return(&buffsvc.Initiative{D10: 4, LogEntryID: 12, Modifiers: []buffsvc.InitiativeModifier{
			{Source: buffsvc.InitiativeSpell, SpellID: "fly", Name: "Fly", Timing: buffsvc.TimingEndOfRound},
			{Source: buffsvc.InitiativeEffect, Value: -2, Name: "Haste", SpellID: "haste"},
		}}, nil)
		rec := do(h, "GET", "/api/round", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"round":3,"initiative":{"value":2,"d10":4,"endOfRound":true,"logEntryId":12,"modifiers":[
			{"source":"spell","value":0,"spellId":"fly","name":"Fly","timing":"end_of_round"},
			{"source":"effect","value":-2,"name":"Haste","spellId":"haste"}]}}`, rec.Body.String())
	})

	t.Run("GET /api/round out of combat: null", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().GetRound(mock.Anything, int64(1)).Return(0, nil)
		m.buffs.EXPECT().GetInitiative(mock.Anything, int64(1)).Return(nil, nil)
		rec := do(h, "GET", "/api/round", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"round":0,"initiative":null}`, rec.Body.String())
	})

	t.Run("Next Round returns the new round's initiative", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().AdvanceRound(mock.Anything, int64(1)).
			Return([]buffsvc.BuffEffect{}, []buffsvc.BuffEffect{}, 1, buffsvc.Initiative{D10: 9, LogEntryID: 2}, buffsvc.EndResult{}, nil)
		// No GetInitiative: the advance returns the roll it made.
		rec := do(h, "POST", "/api/round/advance", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"roundNumber":1,"activeBuffs":[],"expiredBuffs":[],"initiative":{"value":9,"d10":9,"modifiers":[],"endOfRound":false,"logEntryId":2}}`, rec.Body.String())
	})

	t.Run("reroll returns the new roll", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().RerollInitiative(mock.Anything, int64(1)).Return(buffsvc.Initiative{D10: 2, LogEntryID: 13}, nil)
		rec := do(h, "POST", "/api/round/initiative/reroll", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"value":2,"d10":2,"modifiers":[],"endOfRound":false,"logEntryId":13}`, rec.Body.String())
	})

	t.Run("reroll: 409 when another transaction is newer", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().RerollInitiative(mock.Anything, int64(1)).
			Return(buffsvc.Initiative{}, fmt.Errorf("%w: %w", domain.ErrConflict, buffsvc.ErrInitiativeNotOnTop))
		rec := do(h, "POST", "/api/round/initiative/reroll", "")
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":"initiative_not_on_top"`)
	})
}
