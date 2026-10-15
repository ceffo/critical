// Package actionloghdl exposes the action log over HTTP: list entries with
// their correction status and the top of the stack, set an entry's note, pop
// (void) the latest entry, undo with an expected top.
package actionloghdl

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/render"
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
	TopID(ctx context.Context, characterID int64) (id int64, ok bool, err error)
}
type amender interface {
	Amend(ctx context.Context, characterID, id int64, a actionlogsvc.Amendment) (*actionlog.Entry, error)
}
type undoer interface {
	Undo(ctx context.Context, characterID, expectedTopID int64) (*actionlog.Entry, error)
}

// RegisterRoutes wires the action log endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc actionlogsvc.SVC) {
	r.Get("/api/log", render.Handle(List(charID, svc)))
	r.Post("/api/log/undo", render.Handle(Undo(charID, svc)))
	r.Post("/api/log/{id}/amend", render.Handle(Amend(charID, svc)))
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
		top, ok, err := svc.TopID(r.Context(), charID(r))
		if err != nil {
			return err
		}
		out := LogDTO{Entries: make([]EntryDTO, 0, len(entries))}
		if ok {
			out.TopID = &top
		}
		for _, e := range entries {
			out.Entries = append(out.Entries, toDTO(e))
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// Amend sets the note of an entry, any kind. Body: {"note": "the DM's ruling"}
// ("" clears it). The note is the only amendable field: a
// body with any other field (delta, source, ...) is a 400.
func Amend(charID CharacterIDFunc, svc amender) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := entryID(r)
		if err != nil {
			return err
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
			return render.BadRequestError("invalid request body")
		}
		for k := range body {
			if k != "note" {
				return render.BadRequestError("only the note of an entry can be amended (got " + strconv.Quote(k) + "): undo the entry and log it again")
			}
		}
		raw, ok := body["note"]
		var note string
		if !ok || json.Unmarshal(raw, &note) != nil {
			return render.BadRequestError("note is required (a string)")
		}
		e, err := svc.Amend(r.Context(), charID(r), id, actionlogsvc.Amendment{Note: note})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTO(*e))
		return nil
	}
}

// Undo pops the newest non-voided entry, any kind (ADR 0010). Body:
// {"expectedTopId": 12}, the topId the client read from GET /api/log. 409 when
// another entry is on top or there is nothing to pop; nothing changes then.
func Undo(charID CharacterIDFunc, svc undoer) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body struct {
			ExpectedTopID *int64 `json:"expectedTopId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if body.ExpectedTopID == nil || *body.ExpectedTopID <= 0 {
			return render.BadRequestError("expectedTopId is required (a positive entry id)")
		}
		e, err := svc.Undo(r.Context(), charID(r), *body.ExpectedTopID)
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
	// TopID is the newest non-voided entry, the one POST /api/log/undo pops
	// (null when there is none). It may be older than every listed entry.
	TopID *int64 `json:"topId"`
}

// EntryDTO is one log entry on the wire. hpDelta is the original value;
// effectiveHpDelta/effectiveSummary apply amends (and 0 for voided).
type EntryDTO struct {
	ID               int64  `json:"id"`
	CharacterID      int64  `json:"characterId"`
	Round            int    `json:"round"`
	Kind             string `json:"kind"`
	Summary          string `json:"summary"`
	HPDelta          int    `json:"hpDelta"`
	EffectiveHPDelta int    `json:"effectiveHpDelta"`
	EffectiveSummary string `json:"effectiveSummary"`
	// Note is the latest note amend's note ("" when none).
	Note    string          `json:"note"`
	Status  string          `json:"status"`
	Payload json.RawMessage `json:"payload"`
	// EffectivePayload is the payload of the latest amend in effect when one
	// set it (a corrected save or attack), the original payload otherwise.
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

// CorrectionDTO is one amend or void. note is set on note amends; hpDelta,
// source and payload on value amends (post-roll adjustments, and hp_change
// amends logged before amends became note-only). stepId is the correction
// entry a post-roll correction belongs to; undone is true once that entry was
// popped (the amend no longer counts).
type CorrectionDTO struct {
	ID        int64           `json:"id"`
	Kind      string          `json:"kind"`
	HPDelta   *int            `json:"hpDelta,omitempty"`
	Source    string          `json:"source,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Note      *string         `json:"note,omitempty"`
	StepID    *int64          `json:"stepId,omitempty"`
	Undone    bool            `json:"undone,omitempty"`
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
		EffectiveHPDelta: e.EffectiveHPDelta(), EffectiveSummary: e.EffectiveSummary(), Note: e.Note(),
		Status: string(e.Status()), Payload: payload, EffectivePayload: effective, CreatedAt: e.CreatedAt,
		Snapshots:   make([]SnapshotDTO, 0, len(e.Snapshots)),
		Corrections: make([]CorrectionDTO, 0, len(e.Corrections)),
	}
	for _, s := range e.Snapshots {
		out.Snapshots = append(out.Snapshots, SnapshotDTO{Key: s.Key, Before: s.Before, After: s.After})
	}
	for _, c := range e.Corrections {
		out.Corrections = append(out.Corrections, CorrectionDTO{
			ID: c.ID, Kind: string(c.Kind), HPDelta: c.HPDelta, Source: c.Summary, Payload: c.Payload, Note: c.Note,
			StepID: c.StepID, Undone: c.StepUndone, CreatedAt: c.CreatedAt,
		})
	}
	return out
}

var _ Router = (*chi.Mux)(nil)
