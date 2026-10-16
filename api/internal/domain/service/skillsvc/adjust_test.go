package skillsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
)

// amends makes the log's AmendLatest apply fn to e (as the real log does,
// on the payload in effect) and keep the amend and its change.
func (f *fixture) amends(e *actionlog.Entry, changes *[]actionlog.Change) {
	f.log.EXPECT().AmendLatest(mock.Anything, charID, e.ID, actionlog.KindSkillCheck, mock.Anything).RunAndReturn(
		func(_ context.Context, _, _ int64, _ actionlog.Kind, fn actionlog.AmendFunc) (*actionlog.Entry, error) {
			a, err := fn(*e)
			if err != nil {
				return nil, err
			}
			e.Corrections = append(e.Corrections, actionlog.Correction{
				ActionID: e.ID, Kind: actionlog.CorrectionAmend, Payload: a.Payload,
			})
			if changes != nil {
				*changes = append(*changes, a.Change)
			}
			return e, nil
		}).Maybe() // a too long reason is refused before the log is asked
}

// rolled rolls skill id with a pre-roll adjustment and returns its entry
// (id 100) as the log holds it.
func (f *fixture) rolled(t *testing.T, id string, die, value int, pre skillsvc.Adjustment) *actionlog.Entry {
	t.Helper()
	f.records()
	f.dice.EXPECT().RollDie(die).Return(value).Once()
	_, err := f.svc.Roll(context.Background(), charID, id, pre)
	require.NoError(t, err)
	require.Len(t, f.entries, 1)
	return &actionlog.Entry{ID: 100, CharacterID: charID, Kind: actionlog.KindSkillCheck, Payload: f.entries[0].Payload}
}

func payloadOf(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

// ADR 0014 step 5: an adjustment set before the roll moves the target, +
// easier, - harder; the check is judged against target + adjustment with
// exactly one die. A natural 20 still fails a proficiency check, and beyond
// the die's range the arithmetic decides (no clamp).
func TestSkillCheck_Step5_PreRollAdjustmentMovesTheTarget(t *testing.T) {
	cases := []struct {
		name           string
		id             string
		die, roll, adj int
		target         int
		adjustedTarget int
		success        bool
	}{
		{"harder: -2 turns a 15 into a failure", "musical_instrument", 20, 15, -2, 16, 14, false},
		{"at the adjusted target succeeds", "musical_instrument", 20, 14, -2, 16, 14, true},
		{"easier: +3 turns a 14 into a success", "riding_land", 20, 14, 3, 11, 14, true},
		{"natural 20 fails above 20", "musical_instrument", 20, 20, 5, 16, 21, false},
		{"19 succeeds at the same adjusted target", "musical_instrument", 20, 19, 5, 16, 21, true},
		{"bard skill, in percentage points", "climb_walls", 100, 80, -10, 85, 75, false},
		{"bard skill made easier", "pick_pockets", 100, 60, 15, 50, 65, true},
		{"adjusted target under 0 always fails", "pick_pockets", 100, 1, -60, 50, -10, false},
		{"adjusted target over 100 always succeeds", "climb_walls", 100, 100, 20, 85, 105, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.records()
			f.dice.EXPECT().RollDie(c.die).Return(c.roll).Once() // the other die or a second roll fails the mock
			r, err := f.svc.Roll(context.Background(), charID, c.id, skillsvc.Adjustment{Value: c.adj, Reason: "  DM  "})
			require.NoError(t, err)
			f.dice.AssertNumberOfCalls(t, "RollDie", 1)
			assert.Equal(t, c.roll, r.Roll)
			assert.Equal(t, c.target, r.Target, "target stays the unadjusted one")
			assert.Equal(t, c.adj, r.Adjustment)
			assert.Equal(t, "DM", r.AdjustmentReason, "reason trimmed")
			assert.Equal(t, c.adjustedTarget, r.AdjustedTarget)
			assert.Equal(t, c.success, r.Success)
			assert.Equal(t, c.success, r.RolledSuccess, "the rolled verdict includes the pre-roll adjustment")

			p := payloadOf(t, f.entries[0].Payload)
			assert.Equal(t, float64(c.target), p["target"])
			assert.Equal(t, float64(c.adj), p["adjustment"])
			assert.Equal(t, "DM", p["adjustmentReason"])
			assert.Equal(t, float64(c.adjustedTarget), p["adjustedTarget"])
			assert.Equal(t, c.success, p["success"])
			assert.Equal(t, c.success, p["rolledSuccess"])
			assert.NotContains(t, p, "previousSuccess")
		})
	}

	t.Run("no adjustment drops the reason", func(t *testing.T) {
		f := newFixture(t)
		f.records()
		f.dice.EXPECT().RollDie(20).Return(16).Once()
		r, err := f.svc.Roll(context.Background(), charID, "musical_instrument", skillsvc.Adjustment{Reason: "nothing"})
		require.NoError(t, err)
		assert.Empty(t, r.AdjustmentReason)
		assert.NotContains(t, payloadOf(t, f.entries[0].Payload), "adjustmentReason")
		assert.True(t, r.Success)
	})
}

// ADR 0014 step 6: after the roll the adjustment of the latest check is set
// again: the verdict is recomputed from the stored roll and the new adjusted
// target, the roll never changes, a new adjustment replaces the previous one,
// 0 removes it and drops the reason, and the verdict as rolled is kept.
func TestSkillCheck_Step6_PostRollAdjustmentRecomputesTheVerdict(t *testing.T) {
	cases := []struct {
		name           string
		id             string
		die, roll      int
		pre            skillsvc.Adjustment
		steps          []skillsvc.Adjustment
		target         int
		adjustedTarget int
		reason         string
		success        bool
		rolledSuccess  bool
		prevSuccess    bool
		prevAdj        int
	}{
		{
			name: "fail to success", id: "musical_instrument", die: 20, roll: 18,
			steps:  []skillsvc.Adjustment{{Value: 2, Reason: " rope "}},
			target: 16, adjustedTarget: 18, reason: "rope", success: true, rolledSuccess: false,
			prevSuccess: false, prevAdj: 0,
		},
		{
			name: "success to fail, bard skill", id: "climb_walls", die: 100, roll: 80,
			steps:  []skillsvc.Adjustment{{Value: -10}},
			target: 85, adjustedTarget: 75, success: false, rolledSuccess: true,
			prevSuccess: true, prevAdj: 0,
		},
		{
			name: "a new adjustment replaces the previous one", id: "musical_instrument", die: 20, roll: 18,
			steps:  []skillsvc.Adjustment{{Value: 2, Reason: "rope"}, {Value: 1, Reason: "wet"}},
			target: 16, adjustedTarget: 17, reason: "wet", success: false, rolledSuccess: false,
			prevSuccess: true, prevAdj: 2,
		},
		{
			name: "0 removes the pre-roll adjustment and drops the reason", id: "musical_instrument", die: 20, roll: 18,
			pre:    skillsvc.Adjustment{Value: 2, Reason: "rope"},
			steps:  []skillsvc.Adjustment{{Value: 0, Reason: "oops"}},
			target: 16, adjustedTarget: 16, success: false, rolledSuccess: true,
			prevSuccess: true, prevAdj: 2,
		},
		{
			name: "replaces the pre-roll adjustment", id: "musical_instrument", die: 20, roll: 15,
			pre:    skillsvc.Adjustment{Value: -2, Reason: "slippery"},
			steps:  []skillsvc.Adjustment{{Value: -1, Reason: "less slippery"}},
			target: 16, adjustedTarget: 15, reason: "less slippery", success: true, rolledSuccess: false,
			prevSuccess: false, prevAdj: -2,
		},
		{
			name: "natural 20 still fails", id: "musical_instrument", die: 20, roll: 20,
			steps:  []skillsvc.Adjustment{{Value: 5}},
			target: 16, adjustedTarget: 21, success: false, rolledSuccess: false,
			prevSuccess: false, prevAdj: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			e := f.rolled(t, c.id, c.die, c.roll, c.pre)
			f.amends(e, nil)
			var r *skillsvc.CheckResult
			for _, step := range c.steps {
				var err error
				r, err = f.svc.Adjust(context.Background(), charID, 100, step)
				require.NoError(t, err)
			}
			f.dice.AssertNumberOfCalls(t, "RollDie", 1) // an adjustment rolls nothing
			last := c.steps[len(c.steps)-1]
			assert.Equal(t, c.roll, r.Roll, "the roll never changes")
			assert.Equal(t, c.target, r.Target)
			assert.Equal(t, last.Value, r.Adjustment)
			assert.Equal(t, c.reason, r.AdjustmentReason)
			assert.Equal(t, c.adjustedTarget, r.AdjustedTarget)
			assert.Equal(t, c.success, r.Success)
			assert.Equal(t, c.rolledSuccess, r.RolledSuccess)
			assert.Equal(t, int64(100), r.LogEntryID)

			p := payloadOf(t, e.EffectivePayload())
			assert.Equal(t, float64(c.roll), p["roll"])
			assert.Equal(t, float64(c.target), p["target"])
			assert.Equal(t, float64(c.adjustedTarget), p["adjustedTarget"])
			assert.Equal(t, c.success, p["success"])
			assert.Equal(t, c.rolledSuccess, p["rolledSuccess"], "rolled verdict kept")
			assert.Equal(t, payloadOf(t, e.Payload)["success"], p["rolledSuccess"])
			assert.Equal(t, c.prevSuccess, p["previousSuccess"], "verdict before the last adjustment")
			assert.Equal(t, float64(c.prevAdj), p["previousAdjustment"], "adjustment before the last adjustment")
			if c.reason == "" {
				assert.NotContains(t, p, "adjustmentReason")
			}
		})
	}

	t.Run("an entry logged before adjustments", func(t *testing.T) {
		f := newFixture(t)
		e := &actionlog.Entry{ID: 100, Kind: actionlog.KindSkillCheck, Payload: json.RawMessage(
			`{"id":"climb_walls","kind":"bardSkill","die":100,"roll":90,"target":85,"success":false,"modifier":0}`)}
		f.amends(e, nil)
		r, err := f.svc.Adjust(context.Background(), charID, 100, skillsvc.Adjustment{Value: 5})
		require.NoError(t, err)
		assert.Equal(t, 90, r.AdjustedTarget)
		assert.True(t, r.Success)
		assert.False(t, r.RolledSuccess, "its verdict as logged is the rolled one")
	})
}

// ADR 0014 step 6: only the latest roll, and it must be a skill check. The
// log refuses (actionlogsvc TestAmendLatest); the service passes the refusal
// on untouched (409 voided / not latest, 400 not a skill check).
func TestSkillCheck_Step6_LatestCheckOnly(t *testing.T) {
	boom := errors.New("boom")
	for _, want := range []error{actionlogsvc.ErrNotLatest, actionlogsvc.ErrAlreadyVoided, actionlogsvc.ErrNotAmendable, boom} {
		t.Run(want.Error(), func(t *testing.T) {
			f := newFixture(t)
			f.log.EXPECT().AmendLatest(mock.Anything, charID, int64(9), actionlog.KindSkillCheck, mock.Anything).Return(nil, want)
			r, err := f.svc.Adjust(context.Background(), charID, 9, skillsvc.Adjustment{Value: 2})
			assert.Nil(t, r)
			assert.ErrorIs(t, err, want)
		})
	}

	t.Run("undecodable payload", func(t *testing.T) {
		f := newFixture(t)
		e := &actionlog.Entry{ID: 9, Kind: actionlog.KindSkillCheck, Payload: json.RawMessage(`[]`)}
		f.amends(e, nil)
		r, err := f.svc.Adjust(context.Background(), charID, 9, skillsvc.Adjustment{Value: 1})
		assert.Nil(t, r)
		assert.ErrorContains(t, err, "decode skill check #9")
		assert.Empty(t, e.Corrections)
	})
}

// ADR 0014 step 7: each post-roll adjustment is an amend correction carrying
// the recomputed payload with the verdict and adjustment before it; its
// correction step names the field and the adjustment from and to. The
// composition test proves it is its own undoable step.
func TestSkillCheck_Step7_AdjustmentIsAnAmendCorrection(t *testing.T) {
	f := newFixture(t)
	e := f.rolled(t, "musical_instrument", 20, 15, skillsvc.Adjustment{Value: -2})
	var changes []actionlog.Change
	f.amends(e, &changes)
	for _, v := range []int{3, 0} {
		_, err := f.svc.Adjust(context.Background(), charID, 100, skillsvc.Adjustment{Value: v})
		require.NoError(t, err)
	}
	assert.JSONEq(t, `{"id":"musical_instrument","kind":"proficiency","die":20,"roll":15,"target":16,
		"adjustment":-2,"adjustedTarget":14,"success":false,"rolledSuccess":false,
		"ability":"dex","abilityScore":17,"modifier":-1}`, string(e.Payload), "the entry is never edited")
	require.Len(t, e.Corrections, 2, "one amend per adjustment")
	for _, c := range e.Corrections {
		assert.Equal(t, actionlog.CorrectionAmend, c.Kind)
	}
	assert.JSONEq(t, `{"id":"musical_instrument","kind":"proficiency","die":20,"roll":15,"target":16,
		"adjustment":3,"adjustedTarget":19,"success":true,"rolledSuccess":false,
		"previousSuccess":false,"previousAdjustment":-2,
		"ability":"dex","abilityScore":17,"modifier":-1}`, string(e.Corrections[0].Payload))
	assert.JSONEq(t, `{"id":"musical_instrument","kind":"proficiency","die":20,"roll":15,"target":16,
		"adjustment":0,"adjustedTarget":16,"success":true,"rolledSuccess":false,
		"previousSuccess":true,"previousAdjustment":3,
		"ability":"dex","abilityScore":17,"modifier":-1}`, string(e.Corrections[1].Payload))
	assert.Equal(t, []actionlog.Change{
		{Field: skillsvc.ChangeSkillAdjustment, From: -2, To: 3},
		{Field: skillsvc.ChangeSkillAdjustment, From: 3, To: 0},
	}, changes)
}

// ADR 0014 step 8: a proficiency's adjustment is -20..+20 (d20 points), a bard
// skill's -100..+100 (percentage points), the reason at most 80 characters
// once trimmed. Anything else is refused (400) before a die is rolled or the
// log is amended.
func TestSkillCheck_Step8_AdjustmentRanges(t *testing.T) {
	long := strings.Repeat("é", 81)
	cases := []struct {
		name string
		id   string
		die  int
		adj  skillsvc.Adjustment
		ok   bool
	}{
		{"proficiency +20", "musical_instrument", 20, skillsvc.Adjustment{Value: 20}, true},
		{"proficiency -20", "musical_instrument", 20, skillsvc.Adjustment{Value: -20}, true},
		{"proficiency above +20", "musical_instrument", 20, skillsvc.Adjustment{Value: 21}, false},
		{"proficiency below -20", "musical_instrument", 20, skillsvc.Adjustment{Value: -21}, false},
		{"bard skill +100", "climb_walls", 100, skillsvc.Adjustment{Value: 100}, true},
		{"bard skill -100", "climb_walls", 100, skillsvc.Adjustment{Value: -100}, true},
		{"bard skill above +100", "climb_walls", 100, skillsvc.Adjustment{Value: 101}, false},
		{"bard skill below -100", "climb_walls", 100, skillsvc.Adjustment{Value: -101}, false},
		{"reason of 80", "musical_instrument", 20, skillsvc.Adjustment{Value: 1, Reason: "  " + long[:80*2] + "  "}, true},
		{"reason too long", "musical_instrument", 20, skillsvc.Adjustment{Value: 1, Reason: long}, false},
	}
	for _, c := range cases {
		t.Run("before the roll: "+c.name, func(t *testing.T) {
			f := newFixture(t)
			if c.ok {
				f.records()
				f.dice.EXPECT().RollDie(c.die).Return(1).Once()
			}
			_, err := f.svc.Roll(context.Background(), charID, c.id, c.adj)
			if c.ok {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			f.dice.AssertNotCalled(t, "RollDie", mock.Anything)
			f.log.AssertNotCalled(t, "Record", mock.Anything, mock.Anything)
		})
		t.Run("after the roll: "+c.name, func(t *testing.T) {
			f := newFixture(t)
			e := f.rolled(t, c.id, c.die, 1, skillsvc.Adjustment{})
			f.amends(e, nil)
			_, err := f.svc.Adjust(context.Background(), charID, 100, c.adj)
			if c.ok {
				require.NoError(t, err)
				assert.Len(t, e.Corrections, 1)
				return
			}
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.Empty(t, e.Corrections, "nothing amended")
		})
	}

	t.Run("a too long reason never reaches the log", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.svc.Adjust(context.Background(), charID, 100, skillsvc.Adjustment{Value: 1, Reason: long})
		require.ErrorIs(t, err, domain.ErrInvalidInput)
		f.log.AssertNotCalled(t, "AmendLatest", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}
