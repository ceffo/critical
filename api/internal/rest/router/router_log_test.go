package router_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var at = time.Date(2026, 10, 17, 20, 0, 0, 0, time.UTC)

func TestLogEndpoints_WireShape(t *testing.T) {
	minus12 := -12
	amended := actionlog.Entry{
		ID: 4, CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "orc", HPDelta: -8, CreatedAt: at,
		Corrections: []actionlog.Correction{{ID: 1, ActionID: 4, Kind: actionlog.CorrectionAmend, HPDelta: &minus12, Summary: "orc axe", CreatedAt: at}},
	}
	h, m := newRouter(t)
	m.log.EXPECT().List(mock.Anything, int64(1), 0).Return([]actionlog.Entry{amended}, nil)
	rec := do(h, "GET", "/api/log", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"entries":[{
		"id":4,"characterId":1,"round":2,"kind":"hp_change","summary":"orc","hpDelta":-8,
		"effectiveHpDelta":-12,"effectiveSummary":"orc axe","status":"amended","payload":{},"effectivePayload":{},
		"snapshots":[],
		"corrections":[{"id":1,"kind":"amend","hpDelta":-12,"source":"orc axe","createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}]}`, rec.Body.String())

	h, m = newRouter(t)
	m.log.EXPECT().Amend(mock.Anything, int64(1), int64(4), actionlogsvc.Amendment{HPDelta: -12, Summary: "orc axe"}).Return(&amended, nil)
	rec = do(h, "POST", "/api/log/4/amend", `{"delta":-12,"source":"orc axe"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "amended", got["status"])

	voided := actionlog.Entry{
		ID: 4, CharacterID: 1, Kind: actionlog.KindRoundAdvance, Summary: "next",
		Snapshots:   []actionlog.Snapshot{{Key: "round", Before: json.RawMessage(`1`), After: json.RawMessage(`2`)}},
		Corrections: []actionlog.Correction{{ID: 2, ActionID: 4, Kind: actionlog.CorrectionVoid, CreatedAt: at}}, CreatedAt: at,
	}
	h, m = newRouter(t)
	m.log.EXPECT().Void(mock.Anything, int64(1), int64(4)).Return(&voided, nil)
	rec = do(h, "POST", "/api/log/4/void", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"id":4,"characterId":1,"round":0,"kind":"round_advance","summary":"next","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"next","status":"voided","payload":{},"effectivePayload":{},
		"snapshots":[{"key":"round","before":1,"after":2}],
		"corrections":[{"id":2,"kind":"void","createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}`, rec.Body.String())
}

// GET /api/hp events use the camelCase keys of web/src/types/hp.ts.
func TestHPEndpoint_WireShape(t *testing.T) {
	h, m := newRouter(t)
	m.hp.EXPECT().GetState(mock.Anything, int64(1)).Return(&hpsvc.HPState{
		CurrentHp: 15, MaxHp: 20,
		Events: []hpsvc.HpEvent{{ID: 4, CharacterID: 1, Delta: -5, Source: "orc", CreatedAt: at}},
	}, nil)
	rec := do(h, "GET", "/api/hp", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"currentHp":15,"maxHp":20,"events":[
		{"id":4,"characterId":1,"delta":-5,"source":"orc","createdAt":"2026-10-17T20:00:00Z"}]}`, rec.Body.String())
}

// An adjusted save lists the amend's payload as the effective payload.
func TestLogEndpoints_AdjustedSaveShape(t *testing.T) {
	save := actionlog.Entry{
		ID: 7, CharacterID: 1, Round: 1, Kind: actionlog.KindSave, Summary: "Save vs Spells: d20 12 vs 14, FAILED",
		Payload: json.RawMessage(`{"passed":false}`), CreatedAt: at,
		Corrections: []actionlog.Correction{{
			ID: 3, ActionID: 7, Kind: actionlog.CorrectionAmend, CreatedAt: at,
			Summary: "Save vs Spells: d20 12 vs 14, adjusted +4 Gaia fire: FAILED → PASSED",
			Payload: json.RawMessage(`{"passed":true}`),
		}},
	}
	h, m := newRouter(t)
	m.log.EXPECT().List(mock.Anything, int64(1), 0).Return([]actionlog.Entry{save}, nil)
	rec := do(h, "GET", "/api/log", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"entries":[{
		"id":7,"characterId":1,"round":1,"kind":"save","summary":"Save vs Spells: d20 12 vs 14, FAILED","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"Save vs Spells: d20 12 vs 14, adjusted +4 Gaia fire: FAILED → PASSED",
		"status":"amended","payload":{"passed":false},"effectivePayload":{"passed":true},
		"snapshots":[],
		"corrections":[{"id":3,"kind":"amend","source":"Save vs Spells: d20 12 vs 14, adjusted +4 Gaia fire: FAILED → PASSED",
			"payload":{"passed":true},"createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}]}`, rec.Body.String())
}
