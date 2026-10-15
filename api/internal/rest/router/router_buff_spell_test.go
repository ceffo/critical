package router_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// ADR 0004 step 17: a buff from a spell is served with its spellId (omitted
// otherwise); a spellId in an activation body is ignored, only a spell cast
// stamps it.
func TestBuffs_SpellIDOnTheWire(t *testing.T) {
	t.Run("served when the buff comes from a spell", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().ListStatus(mock.Anything, int64(1)).Return([]buffsvc.BuffStatus{
			{BuffEffect: buffsvc.BuffEffect{ID: "h", Name: "Haste", Duration: 4, SpellID: "haste"}, Applies: true},
			{BuffEffect: buffsvc.BuffEffect{ID: "g", Name: "Power Gem"}, Applies: true},
		}, nil)
		rec := do(h, "GET", "/api/buffs", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `[
			{"id":"h","name":"Haste","charges":0,"duration":4,"effects":{},"note":"","startRound":0,"chargesTotal":0,"spellId":"haste","applies":true},
			{"id":"g","name":"Power Gem","charges":0,"duration":0,"effects":{},"note":"","startRound":0,"chargesTotal":0,"applies":true}]`,
			rec.Body.String())
	})

	t.Run("ignored in an activation body", func(t *testing.T) {
		h, m := newRouter(t)
		m.buffs.EXPECT().Activate(mock.Anything, int64(1), buffsvc.BuffEffect{Name: "Haste", Duration: 4}).
			Return([]buffsvc.BuffEffect{}, nil).Once()
		m.buffs.EXPECT().ListStatus(mock.Anything, int64(1)).Return([]buffsvc.BuffStatus{}, nil).Maybe()
		rec := do(h, "POST", "/api/buffs", `{"name":"Haste","duration":4,"spellId":"haste"}`)
		assert.Less(t, rec.Code, 300, rec.Body.String())
	})
}
