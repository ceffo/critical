// Package actionloghdl exposes the action log over HTTP: list entries with
// their correction status, amend an HP change, void an entry.
package actionloghdl

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/go-chi/chi/v5"
)

// Router is the small subset of chi.Router we need for registration.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character ID from the request.
type CharacterIDFunc func(r *http.Request) int64

type lister interface {
	List(ctx context.Context, characterID int64, limit int) ([]actionlog.Entry, error)
}
type amender interface {
	Amend(ctx context.Context, characterID, id int64, a actionlogsvc.Amendment) (*actionlog.Entry, error)
}
type voider interface {
	Void(ctx context.Context, characterID, id int64) (*actionlog.Entry, error)
}

// RegisterRoutes wires the action log endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc actionlogsvc.SVC) {
	r.Get("/api/log", render.Handle(List(charID, svc)))
	r.Post("/api/log/{id}/amend", render.Handle(Amend(charID, svc)))
	r.Post("/api/log/{id}/void", render.Handle(Void(charID, svc)))
}

// List returns entries newest first. Optional ?limit=N (default and maximum
// set by the service).
func List(charID CharacterIDFunc, svc lister) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		limit := 0
		if raw := r.URL.Query().Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				return render.BadRequestError("limit must be a positive integer")
			}
			limit = n
		}
		entries, err := svc.List(r.Context(), charID(r), limit)
		if err != nil {
			return err
		}
		out := LogDTO{Entries: make([]EntryDTO, 0, len(entries))}
		for _, e := range entries {
			out.Entries = append(out.Entries, toDTO(e))
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

type amendBody struct {
	Delta  *int   `json:"delta"`
	Source string `json:"source"`
}

// Amend records an amend correction on an hp_change entry. Body:
// {"delta": -7, "source": "orc axe"}; source is optional (keeps the current
// one when empty).
func Amend(charID CharacterIDFunc, svc amender) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := entryID(r)
		if err != nil {
			return err
		}
		var body amendBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if body.Delta == nil {
			return render.BadRequestError("delta is required")
		}
		e, err := svc.Amend(r.Context(), charID(r), id, actionlogsvc.Amendment{HPDelta: *body.Delta, Summary: body.Source})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTO(*e))
		return nil
	}
}

// Void records a void correction, restoring snapshotted state.
func Void(charID CharacterIDFunc, svc voider) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := entryID(r)
		if err != nil {
			return err
		}
		e, err := svc.Void(r.Context(), charID(r), id)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTO(*e))
		return nil
	}
}

func entryID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, render.BadRequestError("invalid log entry id")
	}
	return id, nil
}

// LogDTO is the GET /api/log response.
type LogDTO struct {
	Entries []EntryDTO `json:"entries"`
}

// EntryDTO is one log entry on the wire. hpDelta is the original value;
// effectiveHpDelta/effectiveSummary apply amends (and 0 for voided).
type EntryDTO struct {
	ID               int64           `json:"id"`
	CharacterID      int64           `json:"characterId"`
	Round            int             `json:"round"`
	Kind             string          `json:"kind"`
	Summary          string          `json:"summary"`
	HPDelta          int             `json:"hpDelta"`
	EffectiveHPDelta int             `json:"effectiveHpDelta"`
	EffectiveSummary string          `json:"effectiveSummary"`
	Status           string          `json:"status"`
	Payload          json.RawMessage `json:"payload"`
	// EffectivePayload is the latest amend's payload when one set it (an
	// adjusted save), the original payload otherwise.
	EffectivePayload json.RawMessage `json:"effectivePayload"`
	Snapshots        []SnapshotDTO   `json:"snapshots"`
	Corrections      []CorrectionDTO `json:"corrections"`
	CreatedAt        time.Time       `json:"createdAt"`
}

// SnapshotDTO is the before/after state under one key.
type SnapshotDTO struct {
	Key    string          `json:"key"`
	Before json.RawMessage `json:"before"`
	After  json.RawMessage `json:"after"`
}

// CorrectionDTO is one amend or void. hpDelta, source and payload are set on
// amends that changed them.
type CorrectionDTO struct {
	ID        int64           `json:"id"`
	Kind      string          `json:"kind"`
	HPDelta   *int            `json:"hpDelta,omitempty"`
	Source    string          `json:"source,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

func toDTO(e actionlog.Entry) EntryDTO {
	payload := e.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	effective := e.EffectivePayload()
	if len(effective) == 0 {
		effective = payload
	}
	out := EntryDTO{
		ID: e.ID, CharacterID: e.CharacterID, Round: e.Round, Kind: string(e.Kind),
		Summary: e.Summary, HPDelta: e.HPDelta,
		EffectiveHPDelta: e.EffectiveHPDelta(), EffectiveSummary: e.EffectiveSummary(),
		Status: string(e.Status()), Payload: payload, EffectivePayload: effective, CreatedAt: e.CreatedAt,
		Snapshots:   make([]SnapshotDTO, 0, len(e.Snapshots)),
		Corrections: make([]CorrectionDTO, 0, len(e.Corrections)),
	}
	for _, s := range e.Snapshots {
		out.Snapshots = append(out.Snapshots, SnapshotDTO{Key: s.Key, Before: s.Before, After: s.After})
	}
	for _, c := range e.Corrections {
		out.Corrections = append(out.Corrections, CorrectionDTO{
			ID: c.ID, Kind: string(c.Kind), HPDelta: c.HPDelta, Source: c.Summary, Payload: c.Payload, CreatedAt: c.CreatedAt,
		})
	}
	return out
}

var _ Router = (*chi.Mux)(nil)
