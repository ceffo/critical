// Package spellprephdl exposes the daily spell preparation:
// GET /api/spells/preparation (spellbook + per-level limits) and
// PUT /api/spells/prepared (replace today's preparation).
package spellprephdl

import (
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/spellhdl"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

// RegisterRoutes wires the preparation endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc prepsvc.SVC) {
	r.Get("/api/spells/preparation", render.Handle(GetInfo(charID, svc)))
	r.Put("/api/spells/prepared", render.Handle(Prepare(charID, svc)))
}

// SpellbookEntryDTO is one spell of the spellbook.
type SpellbookEntryDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

// LevelLimitDTO is the preparation limit of one spell level.
type LevelLimitDTO struct {
	Level int `json:"level"`
	Max   int `json:"max"`
}

// InfoDTO is the wire shape of GET /api/spells/preparation.
type InfoDTO struct {
	Spellbook []SpellbookEntryDTO `json:"spellbook"`
	Limits    []LevelLimitDTO     `json:"limits"`
}

// PrepareRequest is the body of PUT /api/spells/prepared.
type PrepareRequest struct {
	Spells []string `json:"spells"`
}

// GetInfo returns the spellbook and the limits.
func GetInfo(charID CharacterIDFunc, svc prepsvc.SVC) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		info, err := svc.Info(r.Context(), charID(r))
		if err != nil {
			return err
		}
		out := InfoDTO{Spellbook: []SpellbookEntryDTO{}, Limits: []LevelLimitDTO{}}
		for _, e := range info.Spellbook {
			out.Spellbook = append(out.Spellbook, SpellbookEntryDTO{ID: e.ID, Name: e.Name, Level: e.Level})
		}
		for _, l := range info.Limits {
			out.Limits = append(out.Limits, LevelLimitDTO{Level: l.Level, Max: l.Max})
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// Prepare replaces today's preparation and answers with the new slot list.
func Prepare(charID CharacterIDFunc, svc prepsvc.SVC) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req PrepareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		slots, err := svc.Prepare(r.Context(), charID(r), req.Spells)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, spellhdl.SlotsToDTO(slots))
		return nil
	}
}
