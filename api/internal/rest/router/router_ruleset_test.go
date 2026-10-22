package router_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
)

// ADR 0041 step 8: GET /api/ruleset serves the default ruleset as structured data, house rules
// included; the old /api/crit-table is gone.
func TestRuleset_Get(t *testing.T) {
	h, m := newRouter(t)
	rs := ruleset.Default()
	rs.ID, rs.CatalogueVersion = 1, "v3"
	m.rulesets.EXPECT().Get(mock.Anything).Return(rs, nil)
	rec := do(h, "GET", "/api/ruleset", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":1,"name":"default","catalogueVersion":"v3",
		"critTable":{"rows":[{"from":1,"to":100,"multiplier":2}],"placeholder":true},
		"racialLift":{"multiclassAnyRace":true,"dualclassAnyRace":true},
		"levelLimits":"warn",
		"successEffects":[{"proficiencyId":"tumbling","ac":-4,"rounds":1}]}`, rec.Body.String())
}

func TestRuleset_CritTableRouteIsRemoved(t *testing.T) {
	h, _ := newRouter(t)
	assert.Equal(t, http.StatusNotFound, do(h, "GET", "/api/crit-table", "").Code)
	assert.Equal(t, http.StatusNotFound, do(h, "PUT", "/api/crit-table", `{"rows":[]}`).Code)
}

func TestRuleset_Changes(t *testing.T) {
	h, m := newRouter(t)
	by := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	m.rulesets.EXPECT().Changes(mock.Anything).Return([]ruleset.Change{{
		ID: 2, At: at, By: by, Rule: ruleset.RuleRacialLift,
		Before: `{"multiclassAnyRace":true,"dualclassAnyRace":true}`, After: `{"multiclassAnyRace":false,"dualclassAnyRace":true}`,
	}}, nil)
	rec := do(h, "GET", "/api/ruleset/changes", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"changes":[{"id":2,"at":"2026-10-21T12:00:00Z","by":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","rule":"racial_lift",
		"before":{"multiclassAnyRace":true,"dualclassAnyRace":true},"after":{"multiclassAnyRace":false,"dualclassAnyRace":true}}]}`, rec.Body.String())
}

func TestRulesetCritTable_Put(t *testing.T) {
	const path = "/api/ruleset/crit-table"
	t.Run("returns the saved table", func(t *testing.T) {
		h, m := newRouter(t)
		rows := []ruleset.CritRow{{From: 1, To: 40, Multiplier: 2}, {From: 41, To: 100, Multiplier: 3}}
		m.rulesets.EXPECT().SetCritTable(mock.Anything, mock.Anything, rows).Return(ruleset.CritTable{Rows: rows}, nil)
		rec := do(h, "PUT", path, `{"rows":[{"from":1,"to":40,"multiplier":2},{"from":41,"to":100,"multiplier":3}]}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"rows":[{"from":1,"to":40,"multiplier":2},{"from":41,"to":100,"multiplier":3}],"placeholder":false}`, rec.Body.String())
	})
	t.Run("400 with the service's message naming the bad row", func(t *testing.T) {
		h, m := newRouter(t)
		m.rulesets.EXPECT().SetCritTable(mock.Anything, mock.Anything, mock.Anything).
			Return(ruleset.CritTable{}, fmt.Errorf("%w: row 2 (50-100) overlaps the row before it", domain.ErrInvalidInput))
		rec := do(h, "PUT", path, `{"rows":[{"from":1,"to":50,"multiplier":2},{"from":50,"to":100,"multiplier":3}]}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `row 2 (50-100) overlaps`)
	})
	t.Run("a decimal multiplier is passed through as entered", func(t *testing.T) {
		h, m := newRouter(t)
		rows := []ruleset.CritRow{{From: 1, To: 50, Multiplier: 1.5}, {From: 51, To: 100, Multiplier: 2.5}}
		m.rulesets.EXPECT().SetCritTable(mock.Anything, mock.Anything, rows).Return(ruleset.CritTable{Rows: rows}, nil)
		rec := do(h, "PUT", path, `{"rows":[{"from":1,"to":50,"multiplier":1.5},{"from":51,"to":100,"multiplier":2.5}]}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"rows":[{"from":1,"to":50,"multiplier":1.5},{"from":51,"to":100,"multiplier":2.5}],"placeholder":false}`, rec.Body.String())
	})
	t.Run("400 on a body that is not json", func(t *testing.T) {
		h, _ := newRouter(t) // the service is never called
		rec := do(h, "PUT", path, `nope`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `"error"`)
	})
	// The service is never called for these: the handler refuses them first.
	for name, tc := range map[string]struct{ body, want string }{
		"rows null":         {`{"rows":null}`, "at least one range"},
		"rows missing":      {`{}`, "at least one range"},
		"rows empty":        {`{"rows":[]}`, "at least one range"},
		"text multiplier":   {`{"rows":[{"from":1,"to":50,"multiplier":2},{"from":51,"to":90,"multiplier":"abc"}]}`, "invalid request body"},
		"float from":        {`{"rows":[{"from":1.5,"to":100,"multiplier":2}]}`, "from and to must be whole numbers"},
		"string multiplier": {`{"rows":[{"from":1,"to":100,"multiplier":"x"}]}`, "invalid request body"},
	} {
		t.Run("400 on "+name, func(t *testing.T) {
			h, _ := newRouter(t)
			rec := do(h, "PUT", path, tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

func TestRulesetRacialLift_Put(t *testing.T) {
	const path = "/api/ruleset/racial-lift"
	t.Run("stores both switches", func(t *testing.T) {
		h, m := newRouter(t)
		lift := ruleset.RacialLift{MulticlassAnyRace: false, DualclassAnyRace: true}
		m.rulesets.EXPECT().SetRacialLift(mock.Anything, mock.Anything, lift).Return(lift, nil)
		rec := do(h, "PUT", path, `{"multiclassAnyRace":false,"dualclassAnyRace":true}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"multiclassAnyRace":false,"dualclassAnyRace":true}`, rec.Body.String())
	})
	for name, body := range map[string]string{
		"a switch missing": `{"multiclassAnyRace":true}`,
		"not json":         `nope`,
		"a text switch":    `{"multiclassAnyRace":"yes","dualclassAnyRace":true}`,
	} {
		t.Run("400 on "+name, func(t *testing.T) {
			h, _ := newRouter(t)
			assert.Equal(t, http.StatusBadRequest, do(h, "PUT", path, body).Code)
		})
	}
}

func TestRulesetSuccessEffects_Put(t *testing.T) {
	const path = "/api/ruleset/success-effects"
	t.Run("stores the effects and answers them", func(t *testing.T) {
		h, m := newRouter(t)
		effects := []ruleset.SuccessEffect{{ProficiencyID: "tumbling", AC: -4, Rounds: 1}}
		m.rulesets.EXPECT().SetSuccessEffects(mock.Anything, mock.Anything, effects).Return(effects, nil)
		rec := do(h, "PUT", path, `{"effects":[{"proficiencyId":"tumbling","ac":-4,"rounds":1}]}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"effects":[{"proficiencyId":"tumbling","ac":-4,"rounds":1}]}`, rec.Body.String())
	})
	t.Run("an empty list removes them all", func(t *testing.T) {
		h, m := newRouter(t)
		m.rulesets.EXPECT().SetSuccessEffects(mock.Anything, mock.Anything, []ruleset.SuccessEffect{}).Return([]ruleset.SuccessEffect{}, nil)
		rec := do(h, "PUT", path, `{"effects":[]}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"effects":[]}`, rec.Body.String())
	})
	t.Run("400 on an unknown proficiency, with the service's message", func(t *testing.T) {
		h, m := newRouter(t)
		m.rulesets.EXPECT().SetSuccessEffects(mock.Anything, mock.Anything, mock.Anything).
			Return(nil, domain.FieldErrorf("successEffects", "%q is not a known proficiency", "nope"))
		rec := do(h, "PUT", path, `{"effects":[{"proficiencyId":"nope","ac":-1,"rounds":1}]}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `not a known proficiency`)
	})
	t.Run("400 when effects is missing", func(t *testing.T) {
		h, _ := newRouter(t)
		assert.Equal(t, http.StatusBadRequest, do(h, "PUT", path, `{}`).Code)
	})
}
