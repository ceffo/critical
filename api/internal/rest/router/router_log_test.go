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
	m.log.EXPECT().TopID(mock.Anything, int64(1)).Return(4, true, nil)
	rec := do(h, "GET", "/api/log", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"topId":4,"entries":[{
		"id":4,"characterId":1,"round":2,"kind":"hp_change","summary":"orc","hpDelta":-8,
		"effectiveHpDelta":-12,"effectiveSummary":"orc axe","note":"","status":"amended","payload":{},"effectivePayload":{},
		"snapshots":[],
		"corrections":[{"id":1,"kind":"amend","hpDelta":-12,"source":"orc axe","createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}]}`, rec.Body.String())

	// Note-only amend: the note is the only field read.
	note := "DM: axe, not club"
	noted := actionlog.Entry{
		ID: 5, CharacterID: 1, Round: 2, Kind: actionlog.KindBuffChange, Summary: "Activated Haste", CreatedAt: at,
		Corrections: []actionlog.Correction{{ID: 3, ActionID: 5, Kind: actionlog.CorrectionAmend, Note: &note, CreatedAt: at}},
	}
	h, m = newRouter(t)
	m.log.EXPECT().Amend(mock.Anything, int64(1), int64(5), actionlogsvc.Amendment{Note: note}).Return(&noted, nil).Once()
	rec = do(h, "POST", "/api/log/5/amend", `{"note":"DM: axe, not club"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"id":5,"characterId":1,"round":2,"kind":"buff_change","summary":"Activated Haste","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"Activated Haste","note":"DM: axe, not club","status":"active",
		"payload":{},"effectivePayload":{},"snapshots":[],
		"corrections":[{"id":3,"kind":"amend","note":"DM: axe, not club","createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}`, rec.Body.String())

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
		"effectiveHpDelta":0,"effectiveSummary":"next","note":"","status":"voided","payload":{},"effectivePayload":{},
		"snapshots":[{"key":"round","before":1,"after":2}],
		"corrections":[{"id":2,"kind":"void","createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}`, rec.Body.String())
}

// POST /api/log/undo sends the expected top to the service and answers with
// the voided entry, shaped as GET /api/log entries (ADR 0010 step 1).
func TestLogUndo_WireShape(t *testing.T) {
	voided := actionlog.Entry{
		ID: 9, CharacterID: 1, Round: 3, Kind: actionlog.KindAttack, Payload: json.RawMessage(`{"hit":true}`),
		Snapshots:   []actionlog.Snapshot{{Key: "items:arrows_plus3", Before: json.RawMessage(`{"count":20}`), After: json.RawMessage(`{"count":19}`)}},
		Corrections: []actionlog.Correction{{ID: 2, ActionID: 9, Kind: actionlog.CorrectionVoid, CreatedAt: at}}, CreatedAt: at,
	}
	h, m := newRouter(t)
	m.log.EXPECT().Undo(mock.Anything, int64(1), int64(9)).Return(&voided, nil).Once()
	rec := do(h, "POST", "/api/log/undo", `{"expectedTopId":9}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"id":9,"characterId":1,"round":3,"kind":"attack","summary":"","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"","note":"","status":"voided","payload":{"hit":true},"effectivePayload":{"hit":true},
		"snapshots":[{"key":"items:arrows_plus3","before":{"count":20},"after":{"count":19}}],
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
	m.log.EXPECT().TopID(mock.Anything, int64(1)).Return(0, false, nil)
	rec := do(h, "GET", "/api/log", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"topId":null,"entries":[{
		"id":7,"characterId":1,"round":1,"kind":"save","summary":"Save vs Spells: d20 12 vs 14, FAILED","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"Save vs Spells: d20 12 vs 14, adjusted +4 Gaia fire: FAILED → PASSED",
		"note":"","status":"amended","payload":{"passed":false},"effectivePayload":{"passed":true},
		"snapshots":[],
		"corrections":[{"id":3,"kind":"amend","source":"Save vs Spells: d20 12 vs 14, adjusted +4 Gaia fire: FAILED → PASSED",
			"payload":{"passed":true},"createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}]}`, rec.Body.String())
}
