package router_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
)

// ADR 0008 step 17.7: the cast and the receive answers carry the heal: the
// dice and what each rolled, the constant, the total, and on the bearer the
// HP around it; cast on someone else the heal says onOther and has no HP.
func TestCast_Step17_HealOnTheWire(t *testing.T) {
	hp := func(n int) *int { return &n }
	roll := &castsvc.Roll{Formula: "2d8+1", Dice: []castsvc.DieRoll{{Sides: 8, Value: 3}, {Sides: 8, Value: 6}}, Total: 10}

	t.Run("cast on the bearer", func(t *testing.T) {
		h, m := newRouter(t)
		m.cast.EXPECT().Cast(mock.Anything, int64(1), 2, mock.Anything).Return(castsvc.CastResult{
			Source: castsvc.SourceCast, SpellID: "cure_serious_wounds", Name: "Cure Serious Wounds",
			Heal: &castsvc.HealResult{Roll: roll, Total: 10, HPDelta: 10, HPBefore: hp(4), HPAfter: hp(14)},
		}, nil)
		rec := do(h, "POST", "/api/spells/2/cast", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.JSONEq(t, `{"dice":"2d8+1","rolls":[{"sides":8,"value":3},{"sides":8,"value":6}],"constant":1,"total":10,"hpBefore":4,"hpAfter":14}`,
			heal(t, rec.Body.Bytes()))
	})

	t.Run("received, full, on someone else", func(t *testing.T) {
		h, m := newRouter(t)
		m.cast.EXPECT().Receive(mock.Anything, int64(1), mock.Anything).Return(castsvc.CastResult{
			Source: castsvc.SourceReceived, SpellID: "heal", Name: "Heal",
			Heal: &castsvc.HealResult{Full: true, OnOther: true},
		}, nil)
		rec := do(h, "POST", "/api/spells/receive", `{"spellId":"heal","casterLevel":12}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.JSONEq(t, `{"constant":0,"total":0,"full":true,"onOther":true}`, heal(t, rec.Body.Bytes()))
	})

	t.Run("absent for another spell", func(t *testing.T) {
		h, m := newRouter(t)
		m.cast.EXPECT().Cast(mock.Anything, int64(1), 0, mock.Anything).Return(castsvc.CastResult{Source: castsvc.SourceCast, SpellID: "bless"}, nil)
		rec := do(h, "POST", "/api/spells/0/cast", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), `"heal"`)
	})
}

// heal is the "heal" member of an answer, as JSON.
func heal(t *testing.T, body []byte) string {
	t.Helper()
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &doc))
	require.Contains(t, doc, "heal")
	return string(doc["heal"])
}
