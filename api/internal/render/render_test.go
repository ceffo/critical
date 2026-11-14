package render_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/saves"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
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

// ADR 0050 steps 10 and 11: a specialist's preparation with too few spells of
// his school is 400 school_slot_needs_school_spell; an opposed spell is 400
// spell_school_opposed from every service that refuses it (learn, prepare, a
// blank extra slot's pick), wrapped.
func TestErrorMapping_Step10_11_SchoolSlotAndOpposed(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{fmt.Errorf("prepare: %w", prepsvc.ErrSchoolSlot), "school_slot_needs_school_spell"},
		{fmt.Errorf("prepare: %w: %q", spellbook.ErrSchoolOpposed, "alarm"), "spell_school_opposed"},
		{fmt.Errorf("learn: %w", learnsvc.ErrSchoolOpposed), "spell_school_opposed"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			rec := httptest.NewRecorder()
			render.Error(t.Context(), rec, tc.err)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, tc.code, render.Code(tc.err))
		})
	}
}

// ADR 0050 step 13: a save's specialist caster flag without a wizard spell
// named is 400 caster_specialist_needs_spell, not the invalid_input it wraps.
func TestErrorMapping_Step13_CasterSpecialistNeedsSpell(t *testing.T) {
	err := fmt.Errorf("save: %w", saves.ErrCasterSpecialistNeedsSpell)
	rec := httptest.NewRecorder()
	render.Error(t.Context(), rec, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "caster_specialist_needs_spell", render.Code(err))
}
