// Package profdeschdl exposes GET /api/proficiencies/{id}/description (ADR 0037 step 19.4): a
// non-weapon proficiency's description as a language map. It is a catalogue read no character
// owns, so it needs a session but no current character.
package profdeschdl

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/profdescsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
}

type describer interface {
	Description(ctx context.Context, id string) (localized.Text, error)
}

// RegisterRoutes wires the description read onto r.
func RegisterRoutes(r Router, svc profdescsvc.SVC) {
	r.Get("/api/proficiencies/{id}/description", render.Handle(Get(svc)))
}

// DescriptionDTO is the answer: the proficiency id and its description, a language map (ADR
// 0023) whose texts are in ADR 0021 step 1's Markdown subset. The client picks the language and
// falls back to English per field.
type DescriptionDTO struct {
	ID          string         `json:"id"`
	Description localized.Text `json:"description"`
}

// Get is GET /api/proficiencies/{id}/description. An id the catalogue does not hold is a 404
// not_found.
func Get(svc describer) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := chi.URLParam(r, "id")
		d, err := svc.Description(r.Context(), id)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, DescriptionDTO{ID: id, Description: d})
		return nil
	}
}
