// Package middleware contains request-scoped concerns shared by every router:
// structured request logging, panic recovery, and (eventually) tracing.
package middleware

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/ceffo/critical/api/internal/domain"
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
// and at d the client gets domain.ErrUnavailable at once (503
// database_unavailable, by the central mapping), or, on a path listed in
// late, that path's own answer (a browser navigation such as the login
// callback redirects instead of showing JSON). The handler finishes whenever
// its driver call returns, and its late response is dropped (its writes fail
// with http.ErrHandlerTimeout).
//
// A handler that returns after d with a server error (a driver call cut by the
// context) gets the deadline's answer too. A panic in the handler is raised
// again on the request's goroutine, with the handler's stack, for Recoverer;
// one raised after the client got the deadline's answer is logged.
func Deadline(d time.Duration, late map[string]http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			r = r.WithContext(ctx)
			timedOut := func() {
				if h, ok := late[r.URL.Path]; ok {
					h.ServeHTTP(w, r)
					return
				}
				render.Error(ctx, w, fmt.Errorf("%w: request deadline of %s: %w", domain.ErrUnavailable, d, ctx.Err()))
			}
			bw := &bufferedWriter{header: make(http.Header)}
			done := make(chan struct{})
			go func() {
				defer close(done)
				defer func() {
					var hp *handlerPanic
					if p := recover(); p != nil {
						hp = &handlerPanic{value: p, stack: debug.Stack()}
					}
					if bw.finish(hp) && hp != nil && !isAbort(hp.value) {
						render.Logger(ctx).Error("panic after the request deadline",
							"value", hp.value, "stack", string(hp.stack))
					}
				}()
				next.ServeHTTP(bw, r)
			}()
			select {
			case <-done:
			case <-ctx.Done():
				if !bw.expireUnlessFinished() {
					timedOut()
					return
				}
			}
			if p := bw.panicked(); p != nil {
				if isAbort(p.value) {
					panic(p.value)
				}
				panic(fmt.Sprintf("%v\n\nhandler goroutine:\n%s", p.value, p.stack))
			}
			if ctx.Err() != nil && bw.serverError() {
				timedOut()
				return
			}
			bw.copyTo(w)
		})
	}
}

func isAbort(v any) bool {
	err, ok := v.(error)
	return ok && errors.Is(err, http.ErrAbortHandler)
}

type handlerPanic struct {
	value any
	stack []byte
}

// bufferedWriter holds a handler's response until Deadline sends it, or drops
// it once the deadline has passed.
type bufferedWriter struct {
	header http.Header

	mu       sync.Mutex
	status   int
	body     bytes.Buffer
	expired  bool
	finished bool
	panic    *handlerPanic
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

// finish records that the handler has returned, with its panic if it raised
// one. It reports whether the deadline's answer was already sent (the
// response, and the panic, reach no client).
func (b *bufferedWriter) finish(p *handlerPanic) (late bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.expired {
		return true
	}
	b.finished = true
	b.panic = p
	return false
}

// expireUnlessFinished drops whatever the handler writes from now on, unless
// it has already returned: it reports whether it had.
func (b *bufferedWriter) expireUnlessFinished() (finished bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.finished {
		return true
	}
	b.expired = true
	return false
}

func (b *bufferedWriter) panicked() *handlerPanic {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.panic
}

func (b *bufferedWriter) serverError() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.status >= http.StatusInternalServerError
}

// copyTo sends the response the handler wrote, once it has returned.
func (b *bufferedWriter) copyTo(w http.ResponseWriter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	maps.Copy(w.Header(), b.header)
	w.WriteHeader(cmp.Or(b.status, http.StatusOK))
	_, _ = w.Write(b.body.Bytes())
}
