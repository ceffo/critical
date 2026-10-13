package combatsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fixture save targets: spell 14 (see bard()).

// rollSave rolls a save vs spells with d20 and saveBonus and returns the
// entry the log recorded, as the log would return it (id 42, no corrections).
func rollSave(t *testing.T, f *fixture, d20, saveBonus int) *actionlog.Entry {
	t.Helper()
	f.withChar(bard())
	f.dice.EXPECT().RollD20().Return(d20)
	mods := strMods(12)
	mods.SaveBonus = saveBonus
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(mods, nil)
	f.records()
	_, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell)
	require.NoError(t, err)
	require.Len(t, f.entries, 1)
	n := f.entries[0]
	return &actionlog.Entry{ID: 42, CharacterID: charID, Kind: n.Kind, Summary: n.Summary, Payload: n.Payload}
}

// amends makes the log's AmendLatest apply fn to e and append the amend to
// e's corrections, like actionlogsvc does once its checks pass.
func amends(f *fixture, e *actionlog.Entry) {
	f.log.EXPECT().AmendLatest(mock.Anything, charID, e.ID, actionlog.KindSave, mock.Anything).RunAndReturn(
		func(_ context.Context, _ int64, _ int64, _ actionlog.Kind, fn actionlog.AmendFunc) (*actionlog.Entry, error) {
			a, err := fn(*e)
			if err != nil {
				return nil, err
			}
			e.Corrections = append(e.Corrections, actionlog.Correction{
				ActionID: e.ID, Kind: actionlog.CorrectionAmend, Summary: a.Summary, Payload: a.Payload,
			})
			return e, nil
		})
}

func payloadOf(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

func TestResolveSavingThrow_LogPayload(t *testing.T) {
	f := newFixture(t)
	e := rollSave(t, f, 12, 1)
	assert.Equal(t, "Save vs Spells: d20 12 vs 14, FAILED", e.Summary)
	assert.Equal(t, map[string]any{
		"category": "spell", "label": "Spells", "d20Roll": 12.0, "target": 14.0,
		"modifiers":    []any{map[string]any{"kind": "buff", "source": "Buffs", "value": 1.0}},
		"rolledPassed": false, "adjustment": 0.0, "total": 13.0, "passed": false, "margin": -1.0,
	}, payloadOf(t, e.Payload))
}

func TestAdjustSave_Recompute(t *testing.T) {
	cases := []struct {
		name       string
		d20, bonus int
		steps      []combatsvc.SaveAdjustment
		wantPassed bool
		wantMargin int
		wantMods   []combatsvc.Modifier
		wantText   string
		wantLog    string
	}{
		{
			name: "fail to pass", d20: 12,
			steps:      []combatsvc.SaveAdjustment{{Value: 4, Reason: " Gaia fire "}},
			wantPassed: true, wantMargin: 2,
			wantMods: []combatsvc.Modifier{{Kind: combatsvc.ModAdjustment, Source: "Adjustment", Detail: "Gaia fire", Value: 4}},
			wantText: "Roll: 12 + Mod: 0 + Adj: +4 = 16 vs Target: 14 (Spells)",
			wantLog:  "Save vs Spells: d20 12 vs 14, adjusted +4 Gaia fire: FAILED → PASSED",
		},
		{
			name: "pass to fail, buff kept", d20: 13, bonus: 1,
			steps:      []combatsvc.SaveAdjustment{{Value: -2}},
			wantPassed: false, wantMargin: -2,
			wantMods: []combatsvc.Modifier{
				{Kind: combatsvc.ModBuff, Source: "Buffs", Value: 1},
				{Kind: combatsvc.ModAdjustment, Source: "Adjustment", Value: -2},
			},
			wantText: "Roll: 13 + Mod: 1 + Adj: -2 = 12 vs Target: 14 (Spells)",
			wantLog:  "Save vs Spells: d20 13 vs 14, adjusted -2: PASSED → FAILED",
		},
		{
			name: "back to 0 restores the rolled verdict, reason dropped", d20: 12,
			steps:      []combatsvc.SaveAdjustment{{Value: 4, Reason: "Gaia fire"}, {Value: 0, Reason: "oops"}},
			wantPassed: false, wantMargin: -2,
			wantMods: []combatsvc.Modifier{},
			wantText: "Roll: 12 + Mod: 0 = 12 vs Target: 14 (Spells)",
			wantLog:  "Save vs Spells: d20 12 vs 14, adjustment removed: PASSED → FAILED",
		},
		{
			name: "a new adjustment replaces the previous one", d20: 12,
			steps:      []combatsvc.SaveAdjustment{{Value: 4, Reason: "Gaia fire"}, {Value: 1, Reason: "luck"}},
			wantPassed: false, wantMargin: -1,
			wantMods: []combatsvc.Modifier{{Kind: combatsvc.ModAdjustment, Source: "Adjustment", Detail: "luck", Value: 1}},
			wantText: "Roll: 12 + Mod: 0 + Adj: +1 = 13 vs Target: 14 (Spells)",
			wantLog:  "Save vs Spells: d20 12 vs 14, adjusted +1 luck: PASSED → FAILED",
		},
		{
			name: "natural 1 is plain arithmetic", d20: 1,
			steps:      []combatsvc.SaveAdjustment{{Value: 13}},
			wantPassed: true, wantMargin: 0,
			wantMods: []combatsvc.Modifier{{Kind: combatsvc.ModAdjustment, Source: "Adjustment", Value: 13}},
			wantText: "Roll: 1 + Mod: 0 + Adj: +13 = 14 vs Target: 14 (Spells)",
			wantLog:  "Save vs Spells: d20 1 vs 14, adjusted +13: FAILED → PASSED",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			e := rollSave(t, f, c.d20, c.bonus)
			amends(f, e)
			var res *combatsvc.RollResult
			for _, step := range c.steps {
				var err error
				res, err = f.svc.AdjustSave(context.Background(), charID, 42, step)
				require.NoError(t, err)
			}
			last := c.steps[len(c.steps)-1]
			assert.Equal(t, c.d20, res.D20Roll, "the d20 never changes")
			assert.Equal(t, 14, res.EffectiveThac0)
			assert.Equal(t, c.wantPassed, res.Hit)
			assert.Equal(t, c.wantMargin, res.HitMargin)
			assert.Equal(t, last.Value, res.Adjustment)
			assert.Equal(t, c.wantMods, res.Modifiers)
			assert.Equal(t, c.wantText, res.DamageBreakdown)
			assert.Equal(t, int64(42), res.LogEntryID)
			assert.Equal(t, combatsvc.SaveSpell, res.SaveCategory)

			require.Len(t, e.Corrections, len(c.steps), "one audited amend per adjustment")
			assert.Equal(t, c.wantLog, e.EffectiveSummary())
			p := payloadOf(t, e.EffectivePayload())
			assert.Equal(t, float64(c.d20), p["d20Roll"])
			assert.Equal(t, c.wantPassed, p["passed"])
			assert.Equal(t, float64(c.wantMargin), p["margin"])
			assert.Equal(t, float64(last.Value), p["adjustment"])
			assert.Contains(t, p, "previousPassed")
			assert.Equal(t, payloadOf(t, e.Payload)["passed"], p["rolledPassed"], "rolled verdict kept")
		})
	}
}

func TestAdjustSave_Rejections(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name    string
		adj     combatsvc.SaveAdjustment
		logErr  error // returned by AmendLatest; nil: not reached
		wantErr error
	}{
		{"above +20", combatsvc.SaveAdjustment{Value: 21}, nil, domain.ErrInvalidInput},
		{"below -20", combatsvc.SaveAdjustment{Value: -21}, nil, domain.ErrInvalidInput},
		{"reason too long", combatsvc.SaveAdjustment{Value: 1, Reason: string(make([]rune, 81))}, nil, domain.ErrInvalidInput},
		{"not the latest", combatsvc.SaveAdjustment{Value: 4}, actionlogsvc.ErrNotLatest, actionlogsvc.ErrNotLatest},
		{"voided", combatsvc.SaveAdjustment{Value: 4}, actionlogsvc.ErrAlreadyVoided, actionlogsvc.ErrAlreadyVoided},
		{"not a save", combatsvc.SaveAdjustment{Value: 4}, actionlogsvc.ErrNotAmendable, actionlogsvc.ErrNotAmendable},
		{"storage", combatsvc.SaveAdjustment{Value: 4}, boom, boom},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			if c.logErr != nil {
				f.log.EXPECT().AmendLatest(mock.Anything, charID, int64(9), actionlog.KindSave, mock.Anything).Return(nil, c.logErr)
			}
			res, err := f.svc.AdjustSave(context.Background(), charID, 9, c.adj)
			assert.Nil(t, res)
			assert.ErrorIs(t, err, c.wantErr)
		})
	}

	t.Run("undecodable payload", func(t *testing.T) {
		f := newFixture(t)
		e := &actionlog.Entry{ID: 9, Kind: actionlog.KindSave, Summary: "x", Payload: json.RawMessage(`[]`)}
		amends(f, e)
		res, err := f.svc.AdjustSave(context.Background(), charID, 9, combatsvc.SaveAdjustment{Value: 1})
		assert.Nil(t, res)
		assert.ErrorContains(t, err, "decode save #9")
		assert.Empty(t, e.Corrections)
	})
}
