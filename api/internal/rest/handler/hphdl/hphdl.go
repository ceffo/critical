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
	r.Get("/api/hp", render.Handle(GetHP(charID, svc)))
	r.Post("/api/hp/event", render.Handle(AppendEvent(charID, svc)))
	r.Post("/api/hp/undo", render.Handle(UndoLast(charID, svc)))
}

// GetHP returns the current HP state.
func GetHP(charID CharacterIDFunc, svc stateGetter) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		state, err := svc.GetState(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTO(state))
		return nil
	}
}

type appendBody struct {
	Delta  int    `json:"delta"`
	Source string `json:"source"`
}

// AppendEvent records a delta against the ledger.
func AppendEvent(charID CharacterIDFunc, svc eventAppender) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body appendBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		state, err := svc.AppendEvent(r.Context(), charID(r), body.Delta, body.Source)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTO(state))
		return nil
	}
}

// UndoLast voids the newest non-voided HP change in the action log.
func UndoLast(charID CharacterIDFunc, svc undoer) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		state, err := svc.UndoLast(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTO(state))
		return nil
	}
}

// HPStateDTO is the wire shape used by the React frontend (web/src/types/hp.ts).
// Condition is one of ok, badlyWounded, unconscious, dead (ADR 0011 step 2);
// floor is the lowest HP (-10).
type HPStateDTO struct {
	CurrentHp int          `json:"currentHp"`
	MaxHp     int          `json:"maxHp"`
	Floor     int          `json:"floor"`
	Condition string       `json:"condition"`
	Events    []HpEventDTO `json:"events"`
}

// HpEventDTO matches HpEvent in web/src/types/hp.ts. (It used to be sent
// untagged, as PascalCase keys the panel could not read.) Delta and Source are
// the effective values after amends.
type HpEventDTO struct {
	ID          int64     `json:"id"`
	CharacterID int64     `json:"characterId"`
	Delta       int       `json:"delta"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"createdAt"`
}

func toDTO(s *hpsvc.HPState) HPStateDTO {
	if s == nil {
		return HPStateDTO{Condition: string(hpsvc.ConditionOK), Events: []HpEventDTO{}}
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
	return HPStateDTO{
		CurrentHp: s.CurrentHp, MaxHp: s.MaxHp, Floor: s.FloorHp, Condition: string(s.Condition), Events: events,
	}
}

// Compile-time check that *chi.Mux satisfies our small Router interface.
var _ Router = (*chi.Mux)(nil)
