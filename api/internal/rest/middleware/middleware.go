// Package middleware contains request-scoped concerns shared by every router:
// structured request logging, panic recovery, and (eventually) tracing.
package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/ceffo/critical/api/internal/render"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// Logger is a minimal slog-based request logger. It also stashes a
// request-scoped *slog.Logger into the context so handlers can pull it via
// render.Logger(ctx) without taking a Logger parameter.
func Logger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := chimw.GetReqID(r.Context())
			if reqID == "" {
				reqID = fmt.Sprintf("%d", chimw.NextRequestID())
			}
			log := base.With("request_id", reqID, "method", r.Method, "path", r.URL.Path)
			ctx := render.WithLogger(r.Context(), log)
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r.WithContext(ctx))
			log.Info("request",
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// Recoverer catches panics in handlers and renders a 500.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				render.Logger(r.Context()).Error("panic", "value", rv, "stack", string(debug.Stack()))
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
