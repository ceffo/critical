package middleware

import (
	"net/http"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// StateKeysHeader names, on the answer to a state-changing request, the
// state keys of the action-log transactions it committed (ADR 0018 step 11):
// comma-separated, sorted, each once ("buffs,hp,items:arrows,log"); empty
// when the request wrote nothing to the log. The web refetches by key family
// and nothing else. It is set on a success only: an error answer carries none,
// and the web reads everything an action can change again, as it does for an
// answer without the header (a server of the release before).
const StateKeysHeader = "X-State-Keys"

// StateKeys gives every state-changing request (any method but GET, HEAD and
// OPTIONS) an actionlog.Changes, which the log fills as its transactions
// commit, and answers with its keys in StateKeysHeader. The body is
// untouched, so a client that ignores the header reads every answer as
// before.
func StateKeys(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		ctx, changes := actionlog.WithChanges(r.Context())
		next.ServeHTTP(&stateKeysWriter{ResponseWriter: w, changes: changes}, r.WithContext(ctx))
	})
}

// stateKeysWriter sets StateKeysHeader when the status is written: the
// handler's transactions have committed by then.
type stateKeysWriter struct {
	http.ResponseWriter
	changes *actionlog.Changes
	written bool
}

func (w *stateKeysWriter) WriteHeader(status int) {
	if !w.written {
		w.written = true
		if status < http.StatusBadRequest {
			w.Header().Set(StateKeysHeader, strings.Join(w.changes.Keys(), ","))
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *stateKeysWriter) Write(p []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the writer underneath.
func (w *stateKeysWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
