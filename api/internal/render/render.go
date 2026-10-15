// Package render is the HTTP transport-level helper layer. It centralizes JSON
// encoding and the single mapping table from domain sentinel errors to HTTP
// statuses (clean_architecture.md §7.5). Handlers call render.JSON and
// render.Error — they never write status codes by hand.
package render

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
)

// JSON writes v as JSON with the given status code. Encoding errors are logged,
// not surfaced — the response headers have already been flushed.
func JSON(ctx context.Context, w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		Logger(ctx).Warn("response encode", "error", err)
	}
}

// OK writes 200 with v.
func OK(ctx context.Context, w http.ResponseWriter, v any) { JSON(ctx, w, http.StatusOK, v) }

// HandlerFunc is the handler shape used by every REST handler: parse the
// request, call a service, render the result, and return any failure instead
// of rendering it. Handle adapts it to http.HandlerFunc and renders the error
// through the central mapping table.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Handle adapts h to a net/http handler. A non-nil error from h is rendered
// with Error.
func Handle(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			Error(r.Context(), w, err)
		}
	}
}

// Error renders an error response, mapping known domain sentinels to specific
// HTTP statuses. Anything unknown becomes a 500.
func Error(ctx context.Context, w http.ResponseWriter, err error) {
	status, code, msg := mapError(err)
	if status >= 500 {
		Logger(ctx).Error("internal error", "error", err)
	} else {
		Logger(ctx).Info("client error", "status", status, "error", err)
	}
	body := map[string]string{"error": msg, "code": code}
	// A field error's message stays verbatim for the web: invalid_input is
	// deliberately not localized, the message says what is wrong with the field.
	var fe *domain.FieldError
	if errors.As(err, &fe) {
		body["error"], body["field"] = fe.Message, fe.Field
	}
	JSON(ctx, w, status, body)
}

// CodeBadRequest is the error code of a request that could not be parsed.
const CodeBadRequest = "bad_request"

// StatusError is an error that carries its own HTTP status and client-facing
// message. Handlers return it for request-parsing failures (bad JSON, bad path
// params), where there is no domain sentinel to map.
type StatusError struct {
	Status  int
	Message string
}

func (e *StatusError) Error() string { return e.Message }

// BadRequestError returns a StatusError rendered as 400 with msg.
func BadRequestError(msg string) error {
	return &StatusError{Status: http.StatusBadRequest, Message: msg}
}

// mapping is one row of the sentinel -> HTTP status + error code table. A
// specific sentinel goes above the generic one it wraps (the first match wins). message returns the
// client-facing text; nil means err.Error().
type mapping struct {
	target  error
	status  int
	code    string // stable, machine-readable; the web words it (en + fr)
	message func(err error) string
}

func fixed(msg string) func(error) string { return func(error) string { return msg } }

// mappings is the single registry of sentinel errors and their HTTP
// responses, checked in order with errors.Is. Adding a new sentinel means
// adding a row here; handlers stay untouched (clean_architecture.md §7.5).
//
// Service-local sentinels (combatsvc, spellsvc, actionlogsvc) are registered directly: render
// is transport-layer code and may depend inward on the service packages, while
// services never import render, so layering holds without widening domain.
var mappings = []mapping{
	{target: buffsvc.ErrNoInitiative, status: http.StatusConflict, code: "no_initiative"},
	{target: buffsvc.ErrInitiativeNotOnTop, status: http.StatusConflict, code: "initiative_not_on_top"},
	{target: dicesvc.ErrTooManyFavourites, status: http.StatusConflict, code: "too_many_favourites"},
	{target: spelleffectsvc.ErrUnknownMode, status: http.StatusBadRequest, code: "unknown_spell_mode"},
	{target: spelleffectsvc.ErrNoExposure, status: http.StatusBadRequest, code: "no_exposure"},
	{target: domain.ErrInvalidInput, status: http.StatusBadRequest, code: "invalid_input"},
	{target: domain.ErrConflict, status: http.StatusConflict, code: "conflict"},
	{target: domain.ErrCharacterNotFound, status: http.StatusNotFound, code: "character_not_found"},
	{target: domain.ErrNotFound, status: http.StatusNotFound, code: "not_found"},
	{target: combatsvc.ErrWeaponNotFound, status: http.StatusNotFound, code: "weapon_not_found", message: fixed("weapon not found")},
	{target: weaponsvc.ErrWeaponNotFound, status: http.StatusNotFound, code: "weapon_not_found", message: fixed("weapon not found")},
	{target: itemsvc.ErrItemNotFound, status: http.StatusNotFound, code: "item_not_found"},
	{target: itemsvc.ErrItemEmpty, status: http.StatusConflict, code: "item_empty"},
	{target: itemsvc.ErrNotRestockable, status: http.StatusConflict, code: "item_not_restockable"},
	{target: spellsvc.ErrSlotAlreadyUsed, status: http.StatusConflict, code: "slot_already_used"},
	{target: spellsvc.ErrInvalidSlotIndex, status: http.StatusBadRequest, code: "invalid_slot_index"},
	{target: prepsvc.ErrNotInSpellbook, status: http.StatusBadRequest, code: "not_in_spellbook"},
	{target: prepsvc.ErrOverLimit, status: http.StatusConflict, code: "prep_over_limit"},
	{target: actionlogsvc.ErrAlreadyVoided, status: http.StatusConflict, code: "entry_already_voided"},
	{target: actionlogsvc.ErrNotAmendable, status: http.StatusBadRequest, code: "entry_not_amendable"},
	{target: actionlogsvc.ErrNotLatest, status: http.StatusConflict, code: "entry_not_latest"},
	{target: actionlogsvc.ErrTopChanged, status: http.StatusConflict, code: "log_top_changed"},
	{target: actionlogsvc.ErrNothingToUndo, status: http.StatusConflict, code: "nothing_to_undo"},
}

func mapError(err error) (int, string, string) {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Status, CodeBadRequest, se.Message
	}
	for _, m := range mappings {
		if errors.Is(err, m.target) {
			if m.message != nil {
				return m.status, m.code, m.message(err)
			}
			return m.status, m.code, err.Error()
		}
	}
	return http.StatusInternalServerError, "internal", err.Error()
}

// Logger extracts a *slog.Logger from ctx, or returns the default.
func Logger(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// WithLogger returns a copy of ctx that carries log under the package-private
// key used by Logger. Middleware uses this to attach a request-scoped logger.
func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, log)
}

type loggerKey struct{}
