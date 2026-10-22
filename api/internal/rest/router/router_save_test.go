package router_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// POST /api/roll/save/{id}/adjust passes the entry id, adjustment and reason
// to the service and returns the recomputed save (combatsvc.SaveBreakdown)
// with its log entry id.
func TestAdjustSave_WireShape(t *testing.T) {
	h, m := newRouter(t)
	prevPassed, prevAdj := false, 0
	m.cmb.EXPECT().AdjustSave(mock.Anything, int64(1), int64(7), combatsvc.SaveAdjustment{Value: 4, Reason: "Gaia fire"}).
		Return(&combatsvc.SaveBreakdown{
			Category: "spell", D20Roll: 12, Modifiers: []combatsvc.BreakdownModifier{}, Target: 14, RolledPassed: false,
			Adjustment: 4, AdjustmentReason: "Gaia fire", RollTotal: 12, AdjustedTarget: 10, Total: 16, Passed: true, Margin: 2,
			PreviousPassed: &prevPassed, PreviousAdjustment: &prevAdj, LogEntryID: 7,
		}, nil)
	rec := do(h, "POST", "/api/roll/save/7/adjust", `{"adjustment":4,"reason":"Gaia fire"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, `{"category":"spell","d20Roll":12,"modifiers":[],"mindAffecting":false,"target":14,"rolledPassed":false,`+
		`"adjustment":4,"adjustmentReason":"Gaia fire","rollTotal":12,"adjustedTarget":10,"total":16,"passed":true,"margin":2,`+
		`"previousPassed":false,"previousAdjustment":0,"logEntryId":7}`+"\n", rec.Body.String())
}

// A save always carries its adjustment, 0 included, so the card can show it.
func TestRollSave_SendsZeroAdjustment(t *testing.T) {
	h, m := newRouter(t)
	m.cmb.EXPECT().ResolveSavingThrow(mock.Anything, int64(1), combatsvc.SaveSpell, combatsvc.SaveOptions{}).
		Return(&combatsvc.SaveBreakdown{Category: "spell", D20Roll: 12, Modifiers: []combatsvc.BreakdownModifier{}, LogEntryID: 7}, nil)
	rec := do(h, "POST", "/api/roll/save", `{"category":"spell"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"adjustment":0`)
	assert.Contains(t, rec.Body.String(), `"logEntryId":7`)
}

// ADR 0006 steps 9.3 to 9.5 on the wire: the optional mindAffecting flag of
// POST /api/roll/save reaches the service (absent is false), and the answer
// carries the wisdom modifier and mindAffecting.
func TestRollSave_Step9_MindAffectingFlag(t *testing.T) {
	h, m := newRouter(t)
	m.cmb.EXPECT().ResolveSavingThrow(mock.Anything, int64(1), combatsvc.SaveRodStaffWand, combatsvc.SaveOptions{MindAffecting: true}).
		Return(&combatsvc.SaveBreakdown{
			Category: "rod_staff_wand", D20Roll: 10, MindAffecting: true, Target: 13, RollTotal: 13, AdjustedTarget: 13,
			Total: 13, Passed: true, RolledPassed: true,
			Modifiers: []combatsvc.BreakdownModifier{{Kind: "wisdom", Detail: "17", Value: 3}}, LogEntryID: 9,
		}, nil).Once()
	rec := do(h, "POST", "/api/roll/save", `{"category":"rod_staff_wand","mindAffecting":true}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, true, got["mindAffecting"])
	assert.Equal(t, []any{map[string]any{"kind": "wisdom", "detail": "17", "value": 3.0}}, got["modifiers"])

	m.cmb.EXPECT().ResolveSavingThrow(mock.Anything, int64(1), combatsvc.SaveSpell, combatsvc.SaveOptions{}).
		Return(&combatsvc.SaveBreakdown{Category: "spell", D20Roll: 11, Modifiers: []combatsvc.BreakdownModifier{}, LogEntryID: 10}, nil).Once()
	rec = do(h, "POST", "/api/roll/save", `{"category":"spell"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"mindAffecting":false`)
}

// savedEntryJSON is the part of a log entry the save tests read.
type savedEntryJSON struct {
	ID               int64          `json:"id"`
	Payload          map[string]any `json:"payload"`
	EffectivePayload map[string]any `json:"effectivePayload"`
	Corrections      []struct {
		Payload map[string]any `json:"payload"`
	} `json:"corrections"`
}

// newShapeSave is a save entry as the log service hands it to the router
// (ADR 0006 step 8: the service's payload view has already filled rollTotal
// and adjustedTarget): d20 13 + Luck 1 vs 14, amended by -2.
func newShapeSave() actionlog.Entry {
	rolled := `{"category":"spell","d20Roll":13,"target":14,"rolledPassed":true,"adjustment":0,"total":14,"passed":true,"margin":0,` +
		`"rollTotal":14,"adjustedTarget":14}`
	amended := `{"category":"spell","d20Roll":13,"target":14,"rolledPassed":true,"adjustment":-2,"total":12,"passed":false,"margin":-2,` +
		`"rollTotal":14,"adjustedTarget":16}`
	return actionlog.Entry{
		ID: 7, TransactionID: 7, CharacterID: 1, Kind: actionlog.KindSave, CreatedAt: at,
		Payload: json.RawMessage(rolled),
		Corrections: []actionlog.Correction{
			{ID: 1, ActionID: 7, Kind: actionlog.CorrectionAmend, Payload: json.RawMessage(amended), CreatedAt: at},
		},
	}
}

func decodeEntry(t *testing.T, body []byte) savedEntryJSON {
	t.Helper()
	var e savedEntryJSON
	require.NoError(t, json.Unmarshal(body, &e))
	return e
}

// ADR 0006 step 8, at the wire: the router serves the payload of a save the
// service returns, both the stored one and the effective one, with rollTotal
// and adjustedTarget, and its amend's payload too. (Computing the two fields
// for a save logged before they were stored is the service's view, tested in
// combatsvc and actionlogsvc.)
func TestLog_ASaveIsServedWithRollTotalAndAdjustedTarget(t *testing.T) {
	h, m := newRouter(t)
	save := newShapeSave()
	m.log.EXPECT().List(mock.Anything, int64(1), 0).Return([]actionlog.Entry{save}, nil)
	m.log.EXPECT().TopID(mock.Anything, int64(1)).Return(7, true, nil)
	m.log.EXPECT().RedoTarget(mock.Anything, int64(1)).Return(nil, nil)

	rec := do(h, "GET", "/api/log", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var log struct {
		Entries []savedEntryJSON `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &log))
	require.Len(t, log.Entries, 1)
	e := log.Entries[0]

	assert.Equal(t, 14.0, e.Payload["rollTotal"])
	assert.Equal(t, 14.0, e.Payload["adjustedTarget"])
	assert.Equal(t, true, e.Payload["passed"])

	p := e.EffectivePayload
	assert.Equal(t, 14.0, p["rollTotal"], "d20 + buff, the adjustment left out")
	assert.Equal(t, 16.0, p["adjustedTarget"], "a -2 raises the target by 2")
	assert.Equal(t, -2.0, p["adjustment"])
	assert.Equal(t, false, p["passed"], "the stored verdict")
	assert.Equal(t, -2.0, p["margin"], "the stored margin")
	assert.Equal(t, 12.0, p["total"])

	require.Len(t, e.Corrections, 1)
	assert.Equal(t, 16.0, e.Corrections[0].Payload["adjustedTarget"], "an amend's payload is served in the new shape too")
}

// Every endpoint that returns an entry serves a save's two fields, not only
// the list: the amend, the undo, the redo and the redo target of the list.
func TestLog_ASaveIsServedWithItsFieldsByEveryEndpoint(t *testing.T) {
	save := newShapeSave()
	save.Corrections = nil
	assertShape := func(what string, e savedEntryJSON) {
		t.Helper()
		assert.Equal(t, int64(7), e.ID, what)
		for field, p := range map[string]map[string]any{"payload": e.Payload, "effectivePayload": e.EffectivePayload} {
			assert.Equal(t, 14.0, p["rollTotal"], what+" "+field)
			assert.Equal(t, 14.0, p["adjustedTarget"], what+" "+field)
		}
	}

	h, m := newRouter(t)
	m.log.EXPECT().Amend(mock.Anything, int64(1), int64(7), actionlogsvc.Amendment{Note: "ruling"}).Return(&save, nil).Once()
	rec := do(h, "POST", "/api/log/7/amend", `{"note":"ruling"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assertShape("amend", decodeEntry(t, rec.Body.Bytes()))

	h, m = newRouter(t)
	m.log.EXPECT().Undo(mock.Anything, int64(1), int64(7)).Return(&save, nil).Once()
	rec = do(h, "POST", "/api/log/undo", `{"expectedTopId":7}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assertShape("undo", decodeEntry(t, rec.Body.Bytes()))

	h, m = newRouter(t)
	m.log.EXPECT().List(mock.Anything, int64(1), 0).Return(nil, nil)
	m.log.EXPECT().TopID(mock.Anything, int64(1)).Return(0, false, nil)
	m.log.EXPECT().RedoTarget(mock.Anything, int64(1)).Return(&save, nil)
	rec = do(h, "GET", "/api/log", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var log struct {
		Redo *savedEntryJSON `json:"redo"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &log))
	require.NotNil(t, log.Redo)
	assertShape("redo target", *log.Redo)

	h, m = newRouter(t)
	m.log.EXPECT().Redo(mock.Anything, int64(1), int64(7)).Return(&save, nil).Once()
	rec = do(h, "POST", "/api/log/redo", `{"expectedRedoId":7}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assertShape("redo", decodeEntry(t, rec.Body.Bytes()))
}
