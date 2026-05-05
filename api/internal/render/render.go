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

// BadRequest is a shortcut for callers that already know the input is bad
// without an underlying error to wrap.
func BadRequest(ctx context.Context, w http.ResponseWriter, msg string) {
	JSON(ctx, w, http.StatusBadRequest, map[string]string{"error": msg})
}

// NotFound renders 404 with msg.
func NotFound(ctx context.Context, w http.ResponseWriter, msg string) {
	JSON(ctx, w, http.StatusNotFound, map[string]string{"error": msg})
}

func mapError(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, err.Error()
	case errors.Is(err, domain.ErrCharacterNotFound), errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, err.Error()
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
