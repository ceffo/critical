package router_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
)

var at = time.Date(2026, 10, 17, 20, 0, 0, 0, time.UTC)

// Every entry exposes its transaction id (ADR 0018 step 1): #4 here was
// recorded by the request whose first entry is #3.
func TestLogEndpoints_WireShape(t *testing.T) {
	minus12 := -12
	amended := actionlog.Entry{
		ID: 4, TransactionID: 3, CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "orc", HPDelta: -8, CreatedAt: at,
		Corrections: []actionlog.Correction{{ID: 1, ActionID: 4, Kind: actionlog.CorrectionAmend, HPDelta: &minus12, Summary: "orc axe", CreatedAt: at}},
	}
	h, m := newRouter(t)
	m.log.EXPECT().List(mock.Anything, int64(1), 0).Return([]actionlog.Entry{amended}, nil)
	m.log.EXPECT().TopID(mock.Anything, int64(1)).Return(4, true, nil)
	m.log.EXPECT().RedoTarget(mock.Anything, int64(1)).Return(nil, nil)
	rec := do(h, "GET", "/api/log", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"topId":4,"redo":null,"entries":[{
		"id":4,"transactionId":3,"characterId":1,"round":2,"kind":"hp_change","summary":"orc","hpDelta":-8,
		"effectiveHpDelta":-12,"effectiveSummary":"orc axe","note":"","status":"amended","payload":{},"effectivePayload":{},
		"snapshots":[],
		"corrections":[{"id":1,"kind":"amend","hpDelta":-12,"source":"orc axe","createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}]}`, rec.Body.String())

	// Note-only amend: the note is the only field read.
	note := "DM: axe, not club"
	noted := actionlog.Entry{
		ID: 5, TransactionID: 5, CharacterID: 1, Round: 2, Kind: actionlog.KindBuffChange, Summary: "Activated Haste", CreatedAt: at,
		Corrections: []actionlog.Correction{{ID: 3, ActionID: 5, Kind: actionlog.CorrectionAmend, Note: &note, CreatedAt: at}},
	}
	h, m = newRouter(t)
	m.log.EXPECT().Amend(mock.Anything, int64(1), int64(5), actionlogsvc.Amendment{Note: note}).Return(&noted, nil).Once()
	rec = do(h, "POST", "/api/log/5/amend", `{"note":"DM: axe, not club"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"id":5,"transactionId":5,"characterId":1,"round":2,"kind":"buff_change","summary":"Activated Haste","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"Activated Haste","note":"DM: axe, not club","status":"active",
		"payload":{},"effectivePayload":{},"snapshots":[],
		"corrections":[{"id":3,"kind":"amend","note":"DM: axe, not club","createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}`, rec.Body.String())
}

// ADR 0018 step 3: the HP undo and the per-row void are retired; the global
// undo is the only way to pop.
func TestRetiredUndoEndpointsAreGone(t *testing.T) {
	for _, path := range []string{"/api/hp/undo", "/api/log/4/void"} {
		h, _ := newRouter(t)
		rec := do(h, "POST", path, "")
		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rec.Code, path)
	}
}

// POST /api/log/undo sends the expected top to the service and answers with
// the popped transaction's first entry, voided, shaped as GET /api/log
// entries (ADR 0010 step 1, ADR 0018 step 2).
func TestLogUndo_WireShape(t *testing.T) {
	voided := actionlog.Entry{
		ID: 9, TransactionID: 9, CharacterID: 1, Round: 3, Kind: actionlog.KindAttack, Payload: json.RawMessage(`{"hit":true}`),
		Snapshots:   []actionlog.Snapshot{{Key: "items:arrows_plus3", Before: json.RawMessage(`{"count":20}`), After: json.RawMessage(`{"count":19}`)}},
		Corrections: []actionlog.Correction{{ID: 2, ActionID: 9, Kind: actionlog.CorrectionVoid, CreatedAt: at}}, CreatedAt: at,
	}
	h, m := newRouter(t)
	m.log.EXPECT().Undo(mock.Anything, int64(1), int64(9)).Return(&voided, nil).Once()
	rec := do(h, "POST", "/api/log/undo", `{"expectedTopId":9}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"id":9,"transactionId":9,"characterId":1,"round":3,"kind":"attack","summary":"","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"","note":"","status":"voided","payload":{"hit":true},"effectivePayload":{"hit":true},
		"snapshots":[{"key":"items:arrows_plus3","before":{"count":20},"after":{"count":19}}],
		"corrections":[{"id":2,"kind":"void","createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}`, rec.Body.String())
}

// ADR 0019 steps 2 and 3: GET /api/log names the redo target (the popped
// transaction's first entry, shaped as a log entry, even when it is not among
// the listed entries); POST /api/log/redo sends the expected target and
// answers with that entry, live again.
func TestLogRedo_WireShape(t *testing.T) {
	popped := actionlog.Entry{
		ID: 9, TransactionID: 9, CharacterID: 1, Round: 3, Kind: actionlog.KindRoundAdvance, Payload: json.RawMessage(`{"fromRound":3,"toRound":4}`),
		Corrections: []actionlog.Correction{{ID: 2, ActionID: 9, Kind: actionlog.CorrectionVoid, CreatedAt: at}}, CreatedAt: at,
	}
	h, m := newRouter(t)
	m.log.EXPECT().List(mock.Anything, int64(1), 0).Return([]actionlog.Entry{}, nil)
	m.log.EXPECT().TopID(mock.Anything, int64(1)).Return(8, true, nil)
	m.log.EXPECT().RedoTarget(mock.Anything, int64(1)).Return(&popped, nil)
	rec := do(h, "GET", "/api/log", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"topId":8,"entries":[],"redo":{
		"id":9,"transactionId":9,"characterId":1,"round":3,"kind":"round_advance","summary":"","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"","note":"","status":"voided",
		"payload":{"fromRound":3,"toRound":4},"effectivePayload":{"fromRound":3,"toRound":4},"snapshots":[],
		"corrections":[{"id":2,"kind":"void","createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}}`, rec.Body.String())

	redone := popped
	redone.Corrections = append(append([]actionlog.Correction{}, popped.Corrections...), actionlog.Correction{ID: 3, ActionID: 9, Kind: actionlog.CorrectionRedo, CreatedAt: at})
	h, m = newRouter(t)
	m.log.EXPECT().Redo(mock.Anything, int64(1), int64(9)).Return(&redone, nil).Once()
	rec = do(h, "POST", "/api/log/redo", `{"expectedRedoId":9}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"id":9,"transactionId":9,"characterId":1,"round":3,"kind":"round_advance","summary":"","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"","note":"","status":"active",
		"payload":{"fromRound":3,"toRound":4},"effectivePayload":{"fromRound":3,"toRound":4},"snapshots":[],
		"corrections":[{"id":2,"kind":"void","createdAt":"2026-10-17T20:00:00Z"},{"id":3,"kind":"redo","createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}`, rec.Body.String())

	for _, body := range []string{`{}`, `{"expectedRedoId":0}`, `not json`} {
		h, _ = newRouter(t)
		rec = do(h, "POST", "/api/log/redo", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}

// GET /api/hp events use the camelCase keys of web/src/types/hp.ts.
func TestHPEndpoint_WireShape(t *testing.T) {
	h, m := newRouter(t)
	m.log.EXPECT().GetHPState(mock.Anything, int64(1)).Return(&actionlog.HPState{
		CurrentHp: 15, MaxHp: 20, FloorHp: -10, Condition: actionlog.ConditionOK,
		Events: []actionlog.HPEvent{{ID: 4, CharacterID: 1, Delta: -5, Source: "orc", CreatedAt: at}},
	}, nil)
	rec := do(h, "GET", "/api/hp", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"currentHp":15,"maxHp":20,"floor":-10,"condition":"ok","events":[
		{"id":4,"characterId":1,"delta":-5,"source":"orc","createdAt":"2026-10-17T20:00:00Z"}]}`, rec.Body.String())
}

// An adjusted save lists the amend's payload as the effective payload.
func TestLogEndpoints_AdjustedSaveShape(t *testing.T) {
	save := actionlog.Entry{
		ID: 7, TransactionID: 7, CharacterID: 1, Round: 1, Kind: actionlog.KindSave, Summary: "Save vs Spells: d20 12 vs 14, FAILED",
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
	m.log.EXPECT().RedoTarget(mock.Anything, int64(1)).Return(nil, nil)
	rec := do(h, "GET", "/api/log", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"topId":null,"redo":null,"entries":[{
		"id":7,"transactionId":7,"characterId":1,"round":1,"kind":"save","summary":"Save vs Spells: d20 12 vs 14, FAILED","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"Save vs Spells: d20 12 vs 14, adjusted +4 Gaia fire: FAILED → PASSED",
		"note":"","status":"amended","payload":{"passed":false},"effectivePayload":{"passed":true},
		"snapshots":[],
		"corrections":[{"id":3,"kind":"amend","source":"Save vs Spells: d20 12 vs 14, adjusted +4 Gaia fire: FAILED → PASSED",
			"payload":{"passed":true},"createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}]}`, rec.Body.String())
}

// ADR 0010 step 6: a correction step is an entry of kind correction whose
// payload names the corrected entry and the change; the corrected entry's
// amend carries stepId, and undone once the step was popped (its payload no
// longer counts).
func TestLogEndpoints_CorrectionStepShape(t *testing.T) {
	stepID, popped := int64(9), int64(8)
	attack := actionlog.Entry{
		ID: 7, TransactionID: 7, CharacterID: 1, Round: 1, Kind: actionlog.KindAttack, Payload: json.RawMessage(`{"targetAc":5}`), CreatedAt: at,
		Corrections: []actionlog.Correction{
			{ID: 3, ActionID: 7, Kind: actionlog.CorrectionAmend, Payload: json.RawMessage(`{"targetAc":7}`), StepID: &popped, StepUndone: true, CreatedAt: at},
			{ID: 5, ActionID: 7, Kind: actionlog.CorrectionAmend, Payload: json.RawMessage(`{"targetAc":2}`), StepID: &stepID, CreatedAt: at},
		},
	}
	step := actionlog.Entry{
		ID: 9, TransactionID: 9, CharacterID: 1, Round: 1, Kind: actionlog.KindCorrection, CreatedAt: at,
		Payload: json.RawMessage(`{"entryId":7,"entryKind":"attack","field":"targetAc","from":5,"to":2}`),
	}
	h, m := newRouter(t)
	m.log.EXPECT().List(mock.Anything, int64(1), 0).Return([]actionlog.Entry{step, attack}, nil)
	m.log.EXPECT().TopID(mock.Anything, int64(1)).Return(9, true, nil)
	m.log.EXPECT().RedoTarget(mock.Anything, int64(1)).Return(nil, nil)
	rec := do(h, "GET", "/api/log", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"topId":9,"redo":null,"entries":[{
		"id":9,"transactionId":9,"characterId":1,"round":1,"kind":"correction","summary":"","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"","note":"","status":"active",
		"payload":{"entryId":7,"entryKind":"attack","field":"targetAc","from":5,"to":2},
		"effectivePayload":{"entryId":7,"entryKind":"attack","field":"targetAc","from":5,"to":2},
		"snapshots":[],"corrections":[],"createdAt":"2026-10-17T20:00:00Z"
	},{
		"id":7,"transactionId":7,"characterId":1,"round":1,"kind":"attack","summary":"","hpDelta":0,
		"effectiveHpDelta":0,"effectiveSummary":"","note":"","status":"amended",
		"payload":{"targetAc":5},"effectivePayload":{"targetAc":2},"snapshots":[],
		"corrections":[
			{"id":3,"kind":"amend","payload":{"targetAc":7},"stepId":8,"undone":true,"createdAt":"2026-10-17T20:00:00Z"},
			{"id":5,"kind":"amend","payload":{"targetAc":2},"stepId":9,"createdAt":"2026-10-17T20:00:00Z"}],
		"createdAt":"2026-10-17T20:00:00Z"}]}`, rec.Body.String())
}
