// Package hphdl exposes HP endpoints over HTTP. Handlers depend on narrow,
// per-handler interfaces (clean_architecture.md §7.3); they translate between
// HTTP DTOs and domain types.
package hphdl

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/go-chi/chi/v5"
)

// Router is the small subset of chi.Router we need for registration.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

// Narrow consumer-side interfaces — one per endpoint. Each handler takes
// exactly the methods it calls. The fat hpsvc.SVC satisfies them implicitly.
type stateGetter interface {
	GetState(ctx context.Context, characterID int64) (*hpsvc.HPState, error)
}
type eventAppender interface {
	AppendEvent(ctx context.Context, characterID int64, delta int, source string) (*hpsvc.HPState, error)
}
type undoer interface {
	UndoLast(ctx context.Context, characterID int64) (*hpsvc.HPState, error)
}

// CharacterIDFunc resolves the active character ID from the request context.
// In single-character mode this is just a constant; future routes can derive
// it from auth or path params without changing handler signatures.
type CharacterIDFunc func(r *http.Request) int64

// RegisterRoutes wires HP endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc hpsvc.SVC) {
	r.Get("/api/hp", GetHP(charID, svc))
	r.Post("/api/hp/event", AppendEvent(charID, svc))
	r.Post("/api/hp/undo", UndoLast(charID, svc))
}

// GetHP returns the current HP state.
func GetHP(charID CharacterIDFunc, svc stateGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := svc.GetState(r.Context(), charID(r))
		if err != nil {
			render.Error(r.Context(), w, err)
			return
		}
		render.OK(r.Context(), w, toDTO(state))
	}
}

type appendBody struct {
	Delta  int    `json:"delta"`
	Source string `json:"source"`
}

// AppendEvent records a delta against the ledger.
func AppendEvent(charID CharacterIDFunc, svc eventAppender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body appendBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			render.BadRequest(r.Context(), w, "invalid request body")
			return
		}
		state, err := svc.AppendEvent(r.Context(), charID(r), body.Delta, body.Source)
		if err != nil {
			render.Error(r.Context(), w, err)
			return
		}
		render.OK(r.Context(), w, toDTO(state))
	}
}

// UndoLast appends a correction row that negates the last event.
func UndoLast(charID CharacterIDFunc, svc undoer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := svc.UndoLast(r.Context(), charID(r))
		if err != nil {
			render.Error(r.Context(), w, err)
			return
		}
		render.OK(r.Context(), w, toDTO(state))
	}
}

// HPStateDTO is the wire shape — preserves the existing API contract used by
// the React frontend (currentHp, maxHp, events with PascalCase HpEvent).
type HPStateDTO struct {
	CurrentHp int          `json:"currentHp"`
	MaxHp     int          `json:"maxHp"`
	Events    []HpEventDTO `json:"events"`
}

// HpEventDTO mirrors the fields of the previous unexported HpEvent JSON
// encoding (no struct tags meant Go used the field names verbatim).
type HpEventDTO struct {
	ID          int64
	CharacterID int64
	Delta       int
	Source      string
	CreatedAt   time.Time
}

func toDTO(s *hpsvc.HPState) HPStateDTO {
	if s == nil {
		return HPStateDTO{Events: []HpEventDTO{}}
	}
	events := make([]HpEventDTO, 0, len(s.Events))
	for _, e := range s.Events {
		events = append(events, HpEventDTO{
			ID:          e.ID,
			CharacterID: e.CharacterID,
			Delta:       e.Delta,
			Source:      e.Source,
			CreatedAt:   e.CreatedAt,
		})
	}
	return HPStateDTO{CurrentHp: s.CurrentHp, MaxHp: s.MaxHp, Events: events}
}

// Compile-time check that *chi.Mux satisfies our small Router interface.
var _ Router = (*chi.Mux)(nil)
