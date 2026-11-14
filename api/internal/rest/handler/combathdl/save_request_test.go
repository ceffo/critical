package combathdl_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/ceffo/critical/api/internal/domain/saves"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc/mocks"
	"github.com/ceffo/critical/api/internal/rest/handler/combathdl"
)

// ADR 0050 step 13: the save request's casterSpecialist flag reaches the
// service beside the spell it names, and the service's refusal of the flag
// without a wizard spell is 400 caster_specialist_needs_spell.
func TestRollSave_Step13_TheCasterSpecialistFlag(t *testing.T) {
	route := func(t *testing.T, svc *mocks.MockSVC) http.Handler {
		r := chi.NewRouter()
		combathdl.RegisterRoutes(r, func(*http.Request) int64 { return charID }, svc)
		return r
	}
	t.Run("the flag and the spell reach the service", func(t *testing.T) {
		svc := mocks.NewMockSVC(t)
		svc.EXPECT().ResolveSavingThrow(mock.Anything, charID, combatsvc.SavePetriPoly,
			combatsvc.SaveOptions{SpellID: "polymorph_other", CasterSpecialist: true}).
			Return(&combatsvc.SaveBreakdown{Category: "petri_poly"}, nil).Once()
		rec := httptest.NewRecorder()
		route(t, svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/roll/save",
			strings.NewReader(`{"category":"petri_poly","spellId":" polymorph_other ","casterSpecialist":true}`)))
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
	t.Run("refused without a wizard spell: 400 caster_specialist_needs_spell", func(t *testing.T) {
		svc := mocks.NewMockSVC(t)
		svc.EXPECT().ResolveSavingThrow(mock.Anything, charID, combatsvc.SaveSpell, combatsvc.SaveOptions{CasterSpecialist: true}).
			Return(nil, saves.ErrCasterSpecialistNeedsSpell).Once()
		rec := httptest.NewRecorder()
		route(t, svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/roll/save",
			strings.NewReader(`{"category":"spell","casterSpecialist":true}`)))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":"caster_specialist_needs_spell"`)
	})
}
