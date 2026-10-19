// Package resthdl exposes the long rest: POST /api/rest/long, and
// POST /api/spells/reset as its backwards-compatible alias (slots-only body).
package resthdl

import (
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/restsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

// Router is the chi subset we need.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

// RegisterRoutes wires the rest endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc restsvc.SVC) {
	r.Post("/api/rest/long", render.Handle(LongRest(charID, svc)))
	r.Post("/api/spells/reset", render.Handle(ResetSlotsAlias(charID, svc)))
}

// LongRestDTO is the wire shape of a long rest.
type LongRestDTO struct {
	Slots []wire.SpellSlotDTO `json:"slots"`
	Items []wire.ItemDTO      `json:"items"`
	// EntryID is the long rest's action-log entry.
	EntryID int64 `json:"entryId"`
}

// LongRest resets slots and per-day items in one transaction.
func LongRest(charID CharacterIDFunc, svc restsvc.SVC) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		res, err := svc.LongRest(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, LongRestDTO{Slots: wire.SlotsToDTO(res.Slots), Items: wire.ItemsToDTO(res.Items, res.Warnings), EntryID: res.EntryID})
		return nil
	}
}

// ResetSlotsAlias is the legacy long-rest button: it runs the full long rest
// but answers with the slot list the UI already reads.
func ResetSlotsAlias(charID CharacterIDFunc, svc restsvc.SVC) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		res, err := svc.LongRest(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, wire.SlotsToDTO(res.Slots))
		return nil
	}
}
