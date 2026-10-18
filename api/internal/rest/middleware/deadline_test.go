package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

func serve(h http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/skills", nil))
	return rec
}

// A handler that answers in time is sent as it wrote it: status, headers and
// body.
func TestDeadline_AnswerInTimeIsSentAsWritten(t *testing.T) {
	h := mw.Deadline(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := r.Context().Deadline()
		assert.True(t, ok, "the handler's context has the deadline")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":`))
		_, _ = w.Write([]byte(`true}`))
	}))
	rec := serve(h)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"ok":true}`, rec.Body.String())
}

// A handler held past the deadline by something that does not watch its
// context (a statement sent to a database that stopped answering) does not
// hold the client: 503 database_unavailable is rendered at the deadline, and what the
// handler writes afterwards is dropped.
func TestDeadline_AHandlerStuckPastTheDeadlineAnswersAtTheDeadline(t *testing.T) {
	release := make(chan struct{})
	late := make(chan error, 1)
	h := mw.Deadline(30 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release // ignores the context
		_, err := w.Write([]byte("late"))
		late <- err
	}))
	started := time.Now()
	rec := serve(h)
	assert.Less(t, time.Since(started), 500*time.Millisecond)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, map[string]string{"code": "database_unavailable", "error": "database unavailable"}, body)

	close(release)
	require.ErrorIs(t, <-late, http.ErrHandlerTimeout)
	assert.NotContains(t, rec.Body.String(), "late")
}

// A panic in the handler is raised again on the request's goroutine, naming
// the handler's stack, so Recoverer catches it.
func TestDeadline_AHandlerPanicReachesTheRecoverer(t *testing.T) {
	h := mw.Recoverer(mw.Deadline(time.Second)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))
	rec := serve(h)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	raw := mw.Deadline(time.Second)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	assert.PanicsWithValue(t, true, func() {
		defer func() {
			p := recover()
			s, ok := p.(string)
			panic(ok && strings.HasPrefix(s, "boom") && strings.Contains(s, "handler goroutine"))
		}()
		serve(raw)
	})
}
