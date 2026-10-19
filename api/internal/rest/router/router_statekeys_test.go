package router_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

// ADR 0018 step 11: a state-changing answer names, in X-State-Keys, the keys
// its transactions committed; the body is the endpoint's own, unchanged. An
// error answer and a read carry no header, so the web reads everything again
// for them, as for a server that sends none.
func TestStateKeys_TheAnswerNamesWhatTheWriteChanged(t *testing.T) {
	redone := actionlog.Entry{ID: 9, TransactionID: 9, CharacterID: 1, Kind: actionlog.KindRoundAdvance, CreatedAt: at}

	h, m := newRouter(t)
	m.log.EXPECT().Redo(mock.Anything, int64(1), int64(9)).RunAndReturn(
		func(ctx context.Context, _, _ int64) (*actionlog.Entry, error) {
			actionlog.ChangesFrom(ctx).Add("round", "log", "buffs", "log")
			return &redone, nil
		}).Once()
	rec := do(h, "POST", "/api/log/redo", `{"expectedRedoId":9}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "buffs,log,round", rec.Header().Get(mw.StateKeysHeader))
	assert.Contains(t, rec.Body.String(), `"id":9`, "the body is the endpoint's own")

	t.Run("a write that logged nothing names no key", func(t *testing.T) {
		h, m := newRouter(t)
		m.log.EXPECT().Redo(mock.Anything, int64(1), int64(9)).Return(&redone, nil).Once()
		rec := do(h, "POST", "/api/log/redo", `{"expectedRedoId":9}`)
		require.Equal(t, http.StatusOK, rec.Code)
		v, ok := rec.Header()[mw.StateKeysHeader]
		require.True(t, ok, "the header is there, empty")
		assert.Equal(t, []string{""}, v)
	})

	t.Run("an error answer carries none", func(t *testing.T) {
		h, m := newRouter(t)
		m.log.EXPECT().Redo(mock.Anything, int64(1), int64(9)).Return(nil, actionlogsvc.ErrRedoChanged).Once()
		rec := do(h, "POST", "/api/log/redo", `{"expectedRedoId":9}`)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.NotContains(t, rec.Header(), mw.StateKeysHeader)
		assert.JSONEq(t, `{"error":"the transaction to redo changed","code":"log_redo_changed"}`, rec.Body.String(),
			"the error contract is unchanged")
	})

	t.Run("a read carries none", func(t *testing.T) {
		h, m := newRouter(t)
		m.log.EXPECT().GetHPState(mock.Anything, int64(1)).Return(&actionlog.HPState{}, nil)
		rec := do(h, "GET", "/api/hp", "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.NotContains(t, rec.Header(), mw.StateKeysHeader)
	})
}
