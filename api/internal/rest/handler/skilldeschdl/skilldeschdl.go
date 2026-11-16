// Package skilldeschdl exposes GET /api/skill-sets/{set}/skills/{skill}/description (ADR 0037
// step 19.6): the description of a skill of a skill set as a language map. It is a catalogue read
// no character owns, so it needs a session but no current character.
package skilldeschdl

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/skilldescsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
}

type describer interface {
	Description(ctx context.Context, set, skill string) (localized.Text, error)
}

// RegisterRoutes wires the description read onto r.
func RegisterRoutes(r Router, svc skilldescsvc.SVC) {
	r.Get("/api/skill-sets/{set}/skills/{skill}/description", render.Handle(Get(svc)))
}

// DescriptionDTO is the answer: the skill set id, the skill id and the description, a language
// map (ADR 0023) whose texts are in ADR 0021 step 1's Markdown subset. The client picks the
// language and falls back to English per field.
type DescriptionDTO struct {
	Set         string         `json:"set"`
	Skill       string         `json:"skill"`
	Description localized.Text `json:"description"`
}

// Get is GET /api/skill-sets/{set}/skills/{skill}/description. A pair the catalogue does not hold
// is a 404 not_found.
func Get(svc describer) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		set, skill := chi.URLParam(r, "set"), chi.URLParam(r, "skill")
		d, err := svc.Description(r.Context(), set, skill)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, DescriptionDTO{Set: set, Skill: skill, Description: d})
		return nil
	}
}
