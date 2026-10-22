package router_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
)

// PUT /api/psionics/picks, a psionicist's known powers (ADR 0043 step 9): the
// whole list reaches the service as sent and the answer is the known powers
// as saved, in the derived character's `knownPowers` shape.
func TestPicks_Step9_PutWireShape(t *testing.T) {
	h, m := newRouter(t)
	want := []psionics.Pick{{Kind: psionics.PickDiscipline, ID: "telepathy"}, {Kind: psionics.PickPower, ID: "contact"}}
	m.psionics.EXPECT().SetKnownPowers(mock.Anything, int64(1), want).Return(&charactersvc.PowerKnowledge{
		Allowance: psionics.Allowance{Disciplines: 3, Sciences: 1, Devotions: 4, DefenceModes: 1},
		Used:      psionics.Used{Disciplines: 1, Devotions: 1},
		Primary:   "telepathy",
		Picks:     want,
	}, nil).Once()
	rec := do(h, "PUT", "/api/psionics/picks", `{"picks":[{"kind":"discipline","id":"telepathy"},{"kind":"power","id":"contact"}]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"allowance":{"disciplines":3,"sciences":1,"devotions":4,"defenceModes":1},
		"used":{"disciplines":1,"sciences":0,"devotions":1,"defenceModes":0},
		"primary":"telepathy",
		"picks":[{"kind":"discipline","id":"telepathy"},{"kind":"power","id":"contact"}]}`, rec.Body.String())

	rec = do(h, "PUT", "/api/psionics/picks", `{"picks":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// A list over the rules is a 409 with the rule's own code and the figures the
// web words; a character with no psionics is a 409 too; a malformed list is a
// 400 field error on "picks".
func TestPicks_Step9_PutRefusals(t *testing.T) {
	h, m := newRouter(t)
	body := `{"picks":[{"kind":"discipline","id":"telepathy"}]}`
	over := &psionics.Refusal{Code: psionics.CodeAllowanceExceeded, Slot: psionics.SlotDevotion, Count: 4, Limit: 3}
	m.psionics.EXPECT().SetKnownPowers(mock.Anything, int64(1), mock.Anything).
		Return(nil, over).Once()
	rec := do(h, "PUT", "/api/psionics/picks", body)
	require.Equal(t, http.StatusConflict, rec.Code)
	wantBody, err := json.Marshal(map[string]any{
		"code": "psionic_allowance_exceeded", "error": over.Error(),
		"power": "", "discipline": "", "slot": "devotion", "count": 4, "limit": 3,
	})
	require.NoError(t, err)
	assert.JSONEq(t, string(wantBody), rec.Body.String())

	m.psionics.EXPECT().SetKnownPowers(mock.Anything, int64(1), mock.Anything).Return(nil, psionicsvc.ErrNotPsionicist).Once()
	rec = do(h, "PUT", "/api/psionics/picks", body)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"not_a_psionicist"`)

	m.psionics.EXPECT().SetKnownPowers(mock.Anything, int64(1), mock.Anything).
		Return(nil, &domain.FieldError{Field: "picks", Message: "unknown power"}).Once()
	rec = do(h, "PUT", "/api/psionics/picks", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `picks`)
}

// POST /api/psionics/powers/{id}/use, a power's check and its spend (ADR 0043
// steps 10 and 11): the power's id, the modifiers, the cost and the score
// reach the service as sent; the answer is the check's breakdown, the pool
// after the spend and the entry's id.
func TestUse_Step10_PostWireShape(t *testing.T) {
	h, m := newRouter(t)
	cost, score := 6, 14
	want := psionicsvc.UseRequest{
		Modifiers: []psionicsvc.Modifier{{Value: -2, Reason: "distance"}}, Cost: &cost, Score: &score,
	}
	m.psionics.EXPECT().Use(mock.Anything, int64(1), "mind-bar", want).Return(&psionicsvc.UseResult{
		Check: psionicsvc.CheckRecord{
			Power: "mind-bar", Ability: "int", AbilityScore: 15, PowerModifier: -4, Relearns: 1,
			Modifiers: want.Modifiers, Target: 10, Roll: 12, Outcome: psionics.OutcomeFailure, Cost: 6, Spent: 3,
		},
		PSP: actionlog.PSPState{Current: 20, Max: 40}, LogEntryID: 9,
	}, nil).Once()
	rec := do(h, "POST", "/api/psionics/powers/mind-bar/use", `{"modifiers":[{"value":-2,"reason":"distance"}],"cost":6,"score":14}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"check":{"power":"mind-bar","ability":"int","abilityScore":15,"powerModifier":-4,"relearns":1,
			"modifiers":[{"value":-2,"reason":"distance"}],"target":10,"d20":12,"outcome":"failure",
			"success":false,"cost":6,"spent":3},
		"psp":{"current":20,"max":40},
		"logEntryId":9}`, rec.Body.String())

	rec = do(h, "POST", "/api/psionics/powers/mind-bar/use", `{"modifiers":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// The use's refusals answer their own codes: 409 for the rules', 400 for a
// field error.
func TestUse_Step11_PostRefusals(t *testing.T) {
	h, m := newRouter(t)
	for err, want := range map[error]string{
		psionicsvc.ErrNotPsionicist: "not_a_psionicist",
		psionicsvc.ErrPowerNotKnown: "power_not_known",
		psionicsvc.ErrNeedsContact:  "needs_contact",
		psionicsvc.ErrNotEnoughPSP:  "not_enough_psp",
	} {
		m.psionics.EXPECT().Use(mock.Anything, int64(1), "contact", mock.Anything).Return(nil, err).Once()
		rec := do(h, "POST", "/api/psionics/powers/contact/use", `{}`)
		require.Equal(t, http.StatusConflict, rec.Code, want)
		assert.Contains(t, rec.Body.String(), `"code":"`+want+`"`)
	}
	m.psionics.EXPECT().Use(mock.Anything, int64(1), "contact", mock.Anything).
		Return(nil, &domain.FieldError{Field: "cost", Message: "give it"}).Once()
	rec := do(h, "POST", "/api/psionics/powers/contact/use", `{}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `cost`)
}
