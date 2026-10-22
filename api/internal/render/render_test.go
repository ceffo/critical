package render_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// A login that this server does not offer is a missing route: 404 not_found, never a 500.
func TestError_LoginDisabledIs404(t *testing.T) {
	for name, err := range map[string]error{
		"dev login":    authsvc.ErrDevLoginDisabled,
		"google login": authsvc.ErrGoogleLoginDisabled,
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			render.Error(t.Context(), rec, err)
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.Contains(t, rec.Body.String(), `"code":"not_found"`)
		})
	}
}

// ADR 0037 step 10: a save that grows an overspend is 409 over_budget, the
// body naming the pool's field, the points spent and the limit (the web words
// them), even wrapped.
func TestErrorMapping_Step10_OverBudget(t *testing.T) {
	err := fmt.Errorf("set points: %w", &domain.BudgetError{Field: "skillPoints.open_locks", Spent: 31, Limit: 30})
	rec := httptest.NewRecorder()
	render.Error(t.Context(), rec, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"set points: skillPoints.open_locks over budget: 31 of 30","code":"over_budget","field":"skillPoints.open_locks","spent":31,"limit":30}`, rec.Body.String())
	assert.Equal(t, "over_budget", render.Code(err))
}

// ADR 0043 step 9: a list of known powers over the rules is 409 with the code
// of the rule it broke and the figures the web words; a save for a character
// with no psionics is 409 not_a_psionicist.
func TestErrorMapping_Step9_KnownPowers(t *testing.T) {
	err := fmt.Errorf("save: %w", &psionics.Refusal{Code: psionics.CodeAllowanceExceeded, Slot: psionics.SlotScience, Count: 8, Limit: 7})
	rec := httptest.NewRecorder()
	render.Error(t.Context(), rec, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"save: known powers refused: psionic_allowance_exceeded (power \"\", discipline \"\", science 8 against 7)","code":"psionic_allowance_exceeded","power":"","discipline":"","slot":"science","count":8,"limit":7}`, rec.Body.String())
	assert.Equal(t, "psionic_allowance_exceeded", render.Code(err))

	rec = httptest.NewRecorder()
	render.Error(t.Context(), rec, psionicsvc.ErrNotPsionicist)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "not_a_psionicist", render.Code(psionicsvc.ErrNotPsionicist))
}
