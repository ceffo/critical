package router_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/rest/router"
)

// POST /api/roll/save/{id}/adjust passes the entry id, adjustment and reason
// to the service and returns the recomputed save with its log entry id.
func TestAdjustSave_WireShape(t *testing.T) {
	h, m := newRouter(t)
	m.cmb.EXPECT().AdjustSave(mock.Anything, int64(1), int64(7), combatsvc.SaveAdjustment{Value: 4, Reason: "Gaia fire"}).
		Return(&combatsvc.RollResult{
			D20Roll: 12, BaseThac0: 14, EffectiveThac0: 10, Hit: true, HitMargin: 2,
			Modifiers:    []combatsvc.Modifier{},
			SaveCategory: combatsvc.SaveSpell, Adjustment: 4, AdjustmentReason: "Gaia fire", SaveBonus: 0, SaveTotal: 16,
			SaveRollTotal: 12, SaveAdjustedTarget: 10, LogEntryID: 7,
		}, nil)
	rec := do(h, "POST", "/api/roll/save/7/adjust", `{"adjustment":4,"reason":"Gaia fire"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"d20Roll":12,"d100CritRoll":0,"isCrit":false,"critMultiplier":0,"critPlaceholder":false,"critFrom":0,"critTo":0,
		"baseThac0":14,"modifiers":[],
		"effectiveThac0":10,"targetACNeeded":0,"hit":true,"hitMargin":2,
		"baseDamage":0,"damageDice":"","spellsBuff":0,"weaponBuff":0,"multipliers":0,"diceMultiplier":0,"strengthBonus":0,"bonusDamage":0,
		"damageTerms":[],"totalDamage":0,"unroundedDamage":0,
		"targetAc":0,"saveCategory":"spell","adjustment":4,"adjustmentReason":"Gaia fire","saveBonus":0,"saveTotal":16,
		"rollTotal":12,"adjustedTarget":10,"logEntryId":7}`, rec.Body.String())
}

// A save always carries its adjustment, 0 included, so the card can show it.
func TestRollSave_SendsZeroAdjustment(t *testing.T) {
	h, m := newRouter(t)
	m.cmb.EXPECT().ResolveSavingThrow(mock.Anything, int64(1), combatsvc.SaveSpell).
		Return(&combatsvc.RollResult{D20Roll: 12, SaveCategory: combatsvc.SaveSpell, LogEntryID: 7}, nil)
	rec := do(h, "POST", "/api/roll/save", `{"category":"spell"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"adjustment":0`)
	assert.Contains(t, rec.Body.String(), `"logEntryId":7`)
}

// savedEntryJSON is the part of a GET /api/log entry the save tests read.
type savedEntryJSON struct {
	ID               int64          `json:"id"`
	Payload          map[string]any `json:"payload"`
	EffectivePayload map[string]any `json:"effectivePayload"`
	Corrections      []struct {
		Payload map[string]any `json:"payload"`
	} `json:"corrections"`
}

// ADR 0006 step 8, through the real stack: a save logged before the
// adjustment moved the target, and its adjustment amend, have neither
// rollTotal nor adjustedTarget in the database; GET /api/log serves both,
// computed from the stored numbers, with the stored verdict and margin.
func TestLog_SaveLoggedBeforeTheTargetMovedIsServedInTheNewShape(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.New(ctx, composition.Config{
		DSN:       filepath.Join(t.TempDir(), "critical.db"),
		SeedsDir:  filepath.Join("..", "..", "..", "testdata", "seeds"),
		Catalogue: filepath.Join("..", "..", "..", "testdata", "catalogue.json"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bs.Close(ctx) })
	srv := httptest.NewServer(router.New(slog.New(slog.NewTextHandler(io.Discard, nil)), bs))
	t.Cleanup(srv.Close)

	// The payloads the code wrote before: d20 13 + Luck 1 vs 14, then -2.
	rolled := `{"category":"spell","d20Roll":13,"modifiers":[{"kind":"buff","detail":"Luck","value":1}],"target":14,` +
		`"rolledPassed":true,"adjustment":0,"total":14,"passed":true,"margin":0}`
	amended := `{"category":"spell","d20Roll":13,"modifiers":[{"kind":"buff","detail":"Luck","value":1}],"target":14,` +
		`"rolledPassed":true,"adjustment":-2,"adjustmentReason":"cursed","total":12,"passed":false,"margin":-2,` +
		`"previousPassed":true,"previousAdjustment":0}`
	id, err := bs.Log.Record(ctx, actionlog.NewEntry{CharacterID: bs.CharacterID, Kind: actionlog.KindSave, Payload: json.RawMessage(rolled)})
	require.NoError(t, err)
	_, err = bs.Log.AmendLatest(ctx, bs.CharacterID, id, actionlog.KindSave, func(actionlog.Entry) (actionlog.PayloadAmendment, error) {
		return actionlog.PayloadAmendment{Payload: json.RawMessage(amended), Change: actionlog.Change{
			Field: combatsvc.ChangeSaveAdjustment, From: 0, To: -2,
		}}, nil
	})
	require.NoError(t, err)

	res, err := http.Get(srv.URL + "/api/log") //nolint:noctx // test server
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)
	var log struct {
		Entries []savedEntryJSON `json:"entries"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&log))
	i := slices.IndexFunc(log.Entries, func(e savedEntryJSON) bool { return e.ID == id })
	require.GreaterOrEqual(t, i, 0, "the save is listed (after its correction step)")
	e := log.Entries[i]

	assert.Equal(t, 14.0, e.Payload["rollTotal"])
	assert.Equal(t, 14.0, e.Payload["adjustedTarget"])
	assert.Equal(t, true, e.Payload["passed"])

	p := e.EffectivePayload
	assert.Equal(t, 14.0, p["rollTotal"], "d20 + buff, the adjustment left out")
	assert.Equal(t, 16.0, p["adjustedTarget"], "a -2 raises the target by 2")
	assert.Equal(t, 14.0, p["target"])
	assert.Equal(t, -2.0, p["adjustment"])
	assert.Equal(t, false, p["passed"], "the stored verdict")
	assert.Equal(t, -2.0, p["margin"], "the stored margin")
	assert.Equal(t, 12.0, p["total"])

	require.Len(t, e.Corrections, 1)
	assert.Equal(t, 16.0, e.Corrections[0].Payload["adjustedTarget"], "an amend's payload is served in the new shape too")
}
