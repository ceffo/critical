// Package middleware contains request-scoped concerns shared by every router:
// structured request logging, panic recovery, and (eventually) tracing.
package middleware

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/ceffo/critical/api/internal/render"
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

// Deadline bounds every request to d. Its context ends after d, which ends
// whatever it waits for on the database that watches the context (a connection
// of the pool, the database starting again). A statement already sent to a
// database that stopped answering (a node starved of memory) is not bounded by
// it: lib/pq only asks the server to cancel it, and keeps waiting for the
// answer. So the handler runs on its own goroutine against a buffered response,
// and at d the client gets the error at once, rendered by the central mapping;
// the handler finishes whenever its driver call returns, and its late response
// is dropped (its writes fail with http.ErrHandlerTimeout).
//
// A panic in the handler is raised again on the request's goroutine, with the
// handler's stack, for Recoverer.
func Deadline(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			r = r.WithContext(ctx)
			bw := &bufferedWriter{header: make(http.Header)}
			done := make(chan struct{})
			panicked := make(chan handlerPanic, 1)
			go func() {
				defer func() {
					if p := recover(); p != nil {
						panicked <- handlerPanic{value: p, stack: debug.Stack()}
					}
				}()
				next.ServeHTTP(bw, r)
				close(done)
			}()
			select {
			case p := <-panicked:
				if p.value == http.ErrAbortHandler {
					panic(p.value)
				}
				panic(fmt.Sprintf("%v\n\nhandler goroutine:\n%s", p.value, p.stack))
			case <-done:
				bw.copyTo(w)
			case <-ctx.Done():
				bw.expire()
				render.Error(ctx, w, fmt.Errorf("request deadline of %s: %w", d, ctx.Err()))
			}
		})
	}
}

type handlerPanic struct {
	value any
	stack []byte
}

// bufferedWriter holds a handler's response until Deadline sends it, or drops
// it once the deadline has passed.
type bufferedWriter struct {
	header http.Header

	mu      sync.Mutex
	status  int
	body    bytes.Buffer
	expired bool
}

func (b *bufferedWriter) Header() http.Header { return b.header }

func (b *bufferedWriter) WriteHeader(status int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.status == 0 && !b.expired {
		b.status = status
	}
}

func (b *bufferedWriter) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.expired {
		return 0, http.ErrHandlerTimeout
	}
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(p)
}

func (b *bufferedWriter) expire() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expired = true
}

// copyTo sends the response the handler wrote, once it has returned.
func (b *bufferedWriter) copyTo(w http.ResponseWriter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	maps.Copy(w.Header(), b.header)
	w.WriteHeader(cmp.Or(b.status, http.StatusOK))
	_, _ = w.Write(b.body.Bytes())
}
