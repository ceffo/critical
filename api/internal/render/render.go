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
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
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
	status, msg := mapError(err)
	if status >= 500 {
		Logger(ctx).Error("internal error", "error", err)
	} else {
		Logger(ctx).Info("client error", "status", status, "error", err)
	}
	JSON(ctx, w, status, map[string]string{"error": msg})
}

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

// mapping is one row of the sentinel -> HTTP status table. message returns the
// client-facing text; nil means err.Error().
type mapping struct {
	target  error
	status  int
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
	{target: domain.ErrInvalidInput, status: http.StatusBadRequest},
	{target: domain.ErrConflict, status: http.StatusConflict},
	{target: domain.ErrCharacterNotFound, status: http.StatusNotFound},
	{target: domain.ErrNotFound, status: http.StatusNotFound},
	{target: combatsvc.ErrWeaponNotFound, status: http.StatusNotFound, message: fixed("weapon not found")},
	{target: itemsvc.ErrItemNotFound, status: http.StatusNotFound},
	{target: itemsvc.ErrItemEmpty, status: http.StatusConflict},
	{target: spellsvc.ErrSlotAlreadyUsed, status: http.StatusConflict},
	{target: spellsvc.ErrInvalidSlotIndex, status: http.StatusBadRequest},
	{target: prepsvc.ErrNotInSpellbook, status: http.StatusBadRequest},
	{target: prepsvc.ErrOverLimit, status: http.StatusConflict},
	{target: actionlogsvc.ErrAlreadyVoided, status: http.StatusConflict},
	{target: actionlogsvc.ErrVoidBlocked, status: http.StatusConflict},
	{target: actionlogsvc.ErrNotAmendable, status: http.StatusBadRequest},
	{target: actionlogsvc.ErrNotLatest, status: http.StatusConflict},
	{target: actionlogsvc.ErrAboveMaxHP, status: http.StatusConflict},
}

func mapError(err error) (int, string) {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Status, se.Message
	}
	for _, m := range mappings {
		if errors.Is(err, m.target) {
			if m.message != nil {
				return m.status, m.message(err)
			}
			return m.status, err.Error()
		}
	}
	return http.StatusInternalServerError, err.Error()
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
