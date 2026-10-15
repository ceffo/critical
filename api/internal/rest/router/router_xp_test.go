package router_test

import (
	"net/http"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func intPtr(n int) *int { return &n }

var ledger = &xpsvc.Ledger{
	Opening: 1000, Total: 1017, BonusEligible: true,
	Progress: xpsvc.Progress{
		Level: 15, CurrentFloor: 1_100_000, NextLevel: 16, NextThreshold: intPtr(1_320_000),
		ToNext: 1000, Progress: 0.5, ProgressPercent: 50,
		Levels: []xpsvc.LevelCell{{Level: 15, Threshold: 1_100_000, Status: xpsvc.CellCurrent}},
	},
	Entries: []xpsvc.Entry{{ID: 2, Date: "2026-10-17", Session: "S42", Reason: "dragon", Base: 15, Bonus: true, BonusAmount: 2, Total: 17}},
}

const ledgerJSON = `{"opening":1000,"total":1017,"level":15,"currentFloor":1100000,"nextLevel":16,
	"nextThreshold":1320000,"toNext":1000,"progress":0.5,"progressPercent":50,"bonusEligible":true,
	"levels":[{"level":15,"threshold":1100000,"status":"current"}],
	"entries":[{"id":2,"date":"2026-10-17","session":"S42","reason":"dragon","base":15,"bonus":true,"bonusAmount":2,"total":17}]}`

func TestXP_Get(t *testing.T) {
	h, m := newRouter(t)
	m.xp.EXPECT().Get(mock.Anything, int64(1)).Return(ledger, nil)
	rec := do(h, "GET", "/api/xp", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, ledgerJSON, rec.Body.String())
}

func TestXP_GetAtLastLevelHasNullCeiling(t *testing.T) {
	h, m := newRouter(t)
	m.xp.EXPECT().Get(mock.Anything, int64(1)).Return(&xpsvc.Ledger{
		Progress: xpsvc.Progress{Level: 20, CurrentFloor: 2_200_000, Progress: 1, ProgressPercent: 100},
	}, nil)
	rec := do(h, "GET", "/api/xp", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"opening":0,"total":0,"level":20,"currentFloor":2200000,"nextLevel":null,"nextThreshold":null,
		"toNext":0,"progress":1,"progressPercent":100,"bonusEligible":false,"levels":[],"entries":[]}`, rec.Body.String())
}

func TestXP_Preview(t *testing.T) {
	h, m := newRouter(t)
	m.xp.EXPECT().Preview(1205, true).Return(xpsvc.Amount{Base: 1205, BonusAmount: 121, Total: 1326}, nil)
	rec := do(h, "GET", "/api/xp/preview?base=1205&bonus=true", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"base":1205,"bonusAmount":121,"total":1326}`, rec.Body.String())

	for _, q := range []string{"", "?base=x", "?base=10&bonus=maybe"} {
		rec = do(h, "GET", "/api/xp/preview"+q, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, q)
	}
}

func TestXP_AddPassesTheBody(t *testing.T) {
	h, m := newRouter(t)
	m.xp.EXPECT().Add(mock.Anything, int64(1), xpsvc.EntryInput{Base: 1200, Bonus: true, Reason: "troll", Session: "S42"}).
		Return(ledger, nil)
	rec := do(h, "POST", "/api/xp", `{"base":1200,"bonus":true,"reason":"troll","session":"S42"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, ledgerJSON, rec.Body.String())

	for _, body := range []string{`{"bonus":true}`, `{"base":1,"extra":1}`, `nope`} {
		rec = do(h, "POST", "/api/xp", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}

func TestXP_PatchPassesOnlyTheFieldsSent(t *testing.T) {
	h, m := newRouter(t)
	bonus := false
	m.xp.EXPECT().Update(mock.Anything, int64(1), int64(3), xpsvc.EntryPatch{Bonus: &bonus}).Return(ledger, nil)
	rec := do(h, "PATCH", "/api/xp/3", `{"bonus":false}`)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = do(h, "PATCH", "/api/xp/abc", `{"bonus":false}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestXP_Delete(t *testing.T) {
	h, m := newRouter(t)
	m.xp.EXPECT().Delete(mock.Anything, int64(1), int64(3)).Return(ledger, nil)
	rec := do(h, "DELETE", "/api/xp/3", "")
	require.Equal(t, http.StatusOK, rec.Code)
	rec = do(h, "DELETE", "/api/xp/x", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
