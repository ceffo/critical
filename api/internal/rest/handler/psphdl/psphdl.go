// Package psphdl exposes the PSP pool's hand edit over HTTP (ADR 0043 step 5),
// as hphdl does for HP. The handler depends on a narrow interface
// (clean_architecture.md §7.3) and translates between the HTTP DTOs and the
// domain types.
package psphdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the small subset of chi.Router we need for registration.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
}

// changeAppender is the one method the endpoint calls; actionlogsvc.SVC
// satisfies it.
type changeAppender interface {
	AppendPSPChange(ctx context.Context, characterID int64, delta int, source string) (actionlog.PSPState, error)
}

// CharacterIDFunc resolves the active character ID from the request context.
type CharacterIDFunc func(r *http.Request) int64

// RegisterRoutes wires the PSP endpoint onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc changeAppender) {
	r.Post("/api/psp/event", render.Handle(AppendChange(charID, svc)))
}

type changeBody struct {
	Delta  int    `json:"delta"`
	Source string `json:"source"`
}

// AppendChange logs a psp_change entry: a signed delta, bounded by the pool,
// and an optional source. It answers the pool after the change.
func AppendChange(charID CharacterIDFunc, svc changeAppender) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body changeBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		state, err := svc.AppendPSPChange(r.Context(), charID(r), body.Delta, body.Source)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, StateDTO{Current: state.Current, Max: state.Max})
		return nil
	}
}

// StateDTO is the pool after a change, the derived character's `psp` shape
// (characterhdl.PSPDTO, web/src/types).
type StateDTO struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}

// Compile-time check that *chi.Mux satisfies our small Router interface.
var _ Router = (*chi.Mux)(nil)
