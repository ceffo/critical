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
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
)

// scores makes the abilities port answer score for ability a (once per
// check rolled).
func (f *fixture) scores(a charactersvc.Ability, score buffsvc.AbilityScore) {
	f.abils.EXPECT().GetAbilityScore(mock.Anything, charID, a).Return(score, nil)
}

// rollAbility rolls a check of a with the d20 showing value against the
// score in effect target (sheet score base) and returns the result and its
// entry (id 100) as the log holds it.
func (f *fixture) rollAbility(t *testing.T, a charactersvc.Ability, base, target, value int) (*skillsvc.AbilityCheckResult, *actionlog.Entry) {
	t.Helper()
	f.records()
	f.scores(a, buffsvc.AbilityScore{Base: base, Effective: target})
	f.dice.EXPECT().RollDie(20).Return(value).Once()
	r, err := f.svc.RollAbility(context.Background(), charID, string(a))
	require.NoError(t, err)
	require.NotEmpty(t, f.entries)
	n := len(f.entries)
	return r, &actionlog.Entry{ID: int64(99 + n), CharacterID: charID, Kind: actionlog.KindAbilityCheck, Payload: f.entries[n-1].Payload}
}

// abilityAmends makes the log's AmendLatest apply fn to e on the payload in
// effect and keep the amend and its change, as the real log does.
func (f *fixture) abilityAmends(e *actionlog.Entry, changes *[]actionlog.Change) {
	f.log.EXPECT().AmendLatest(mock.Anything, charID, e.ID, actionlog.KindAbilityCheck, mock.Anything).RunAndReturn(
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
		}).Maybe() // a refused adjustment never reaches the log
}

// ADR 0028 step 1: POST /api/roll/ability/{ability} rolls exactly one d20
// through the dice port for each of the six abilities, and no other die.
func TestAbilityCheck_Step1_OneD20PerCheck(t *testing.T) {
	for i, a := range charactersvc.Abilities {
		t.Run(string(a), func(t *testing.T) {
			f := newFixture(t)
			f.records()
			f.scores(a, buffsvc.AbilityScore{Base: 10 + i, Effective: 10 + i})
			f.dice.EXPECT().RollDie(20).Return(3 + i).Once() // another die or a second roll fails the mock
			r, err := f.svc.RollAbility(context.Background(), charID, string(a))
			require.NoError(t, err)
			f.dice.AssertNumberOfCalls(t, "RollDie", 1)
			assert.Equal(t, a, r.Ability)
			assert.Equal(t, 20, r.Die)
			assert.Equal(t, 3+i, r.Roll, "the die's value, not another")
		})
	}
}

// ADR 0028 step 1: any other ability is refused (invalid input) before a
// die is rolled, a score read or anything logged.
func TestAbilityCheck_Step1_UnknownAbilityRefusedBeforeTheRoll(t *testing.T) {
	for _, key := range []string{"luck", "", "STR", "dexterity"} {
		t.Run(key, func(t *testing.T) {
			f := newFixture(t)
			r, err := f.svc.RollAbility(context.Background(), charID, key)
			assert.Nil(t, r)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			f.dice.AssertNotCalled(t, "RollDie", mock.Anything)
			f.abils.AssertNotCalled(t, "GetAbilityScore", mock.Anything, mock.Anything, mock.Anything)
			f.log.AssertNotCalled(t, "Record", mock.Anything, mock.Anything)
			assert.Zero(t, f.tx.calls, "no transaction opened")
		})
	}
}

// ADR 0028 step 2: the target is the score in effect from the abilities
// port, the sheet's score kept beside it (distinct values, so a swap
// fails): STR raised by a Strength buff, DEX with no effect.
func TestAbilityCheck_Step2_TargetIsTheScoreInEffect(t *testing.T) {
	t.Run("STR raised by a Strength buff", func(t *testing.T) {
		f := newFixture(t)
		r, _ := f.rollAbility(t, charactersvc.AbilityStr, 12, 17, 15)
		assert.Equal(t, 17, r.Target, "the effective STR")
		assert.Equal(t, 12, r.AbilityScore, "the sheet's STR")
		assert.True(t, r.Success, "15 is under the effective 17, over the sheet's 12")
	})
	t.Run("DEX with no effect", func(t *testing.T) {
		f := newFixture(t)
		r, _ := f.rollAbility(t, charactersvc.AbilityDex, 15, 15, 16)
		assert.Equal(t, 15, r.Target)
		assert.Equal(t, 15, r.AbilityScore)
		assert.False(t, r.Success)
	})
	t.Run("the port's error fails the check before the roll", func(t *testing.T) {
		f := newFixture(t)
		boom := errors.New("boom")
		f.abils.EXPECT().GetAbilityScore(mock.Anything, charID, charactersvc.AbilityWis).Return(buffsvc.AbilityScore{}, boom)
		_, err := f.svc.RollAbility(context.Background(), charID, "wis")
		require.ErrorIs(t, err, boom)
		f.dice.AssertNotCalled(t, "RollDie", mock.Anything)
		f.log.AssertNotCalled(t, "Record", mock.Anything, mock.Anything)
	})
}

// ADR 0028 step 3: success at or under the target, plain arithmetic: a
// natural 1 succeeds against 1, fails against 0.
func TestAbilityCheck_Step3_AtOrUnderTheTarget(t *testing.T) {
	cases := []struct {
		name         string
		target, roll int
		success      bool
	}{
		{"at the target", 14, 14, true},
		{"one over", 14, 15, false},
		{"one under", 14, 13, true},
		{"natural 1 against 1", 1, 1, true},
		{"natural 1 against 0", 0, 1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			r, _ := f.rollAbility(t, charactersvc.AbilityCon, c.target+5, c.target, c.roll)
			assert.Equal(t, c.success, r.Success)
			assert.Equal(t, c.success, r.RolledSuccess)
			assert.Equal(t, c.target, r.AdjustedTarget, "no adjustment: the adjusted target is the target")
		})
	}
}

// ADR 0028 step 4: a natural 20 always fails, whatever the adjusted target.
func TestAbilityCheck_Step4_NaturalTwentyAlwaysFails(t *testing.T) {
	cases := []struct {
		name    string
		roll    int
		success bool
	}{
		{"20 fails against 22", 20, false},
		{"19 succeeds against 22", 19, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			r, _ := f.rollAbility(t, charactersvc.AbilityStr, 18, 22, c.roll)
			assert.Equal(t, 22, r.AdjustedTarget)
			assert.Equal(t, c.success, r.Success)
		})
	}
	t.Run("20 still fails after an easier adjustment", func(t *testing.T) {
		f := newFixture(t)
		_, e := f.rollAbility(t, charactersvc.AbilityDex, 15, 15, 20)
		f.abilityAmends(e, nil)
		r, err := f.svc.AdjustAbility(context.Background(), charID, e.ID, skillsvc.Adjustment{Value: 10})
		require.NoError(t, err)
		assert.Equal(t, 25, r.AdjustedTarget)
		assert.False(t, r.Success)
	})
}

// ADR 0028 step 5: each check is one ability_check entry, in its own
// transaction, with no snapshots, no summary and no HP delta; its payload is
// the breakdown the UI words, and the roll answers with the entry id.
func TestAbilityCheck_Step5_OneLoggedEntryPerCheck(t *testing.T) {
	f := newFixture(t)
	f.records()
	f.scores(charactersvc.AbilityStr, buffsvc.AbilityScore{Base: 12, Effective: 16})
	f.scores(charactersvc.AbilityCha, buffsvc.AbilityScore{Base: 15, Effective: 15})
	f.dice.EXPECT().RollDie(20).Return(14).Once()
	f.dice.EXPECT().RollDie(20).Return(17).Once()

	str, err := f.svc.RollAbility(context.Background(), charID, "str")
	require.NoError(t, err)
	cha, err := f.svc.RollAbility(context.Background(), charID, "cha")
	require.NoError(t, err)

	f.dice.AssertNumberOfCalls(t, "RollDie", 2)
	assert.Equal(t, 2, f.tx.calls, "one transaction per check")
	require.Len(t, f.entries, 2)
	assert.Equal(t, int64(100), str.LogEntryID)
	assert.Equal(t, int64(101), cha.LogEntryID)
	for _, e := range f.entries {
		assert.Equal(t, actionlog.KindAbilityCheck, e.Kind)
		assert.Empty(t, e.Summary, "the UI words the check")
		assert.Empty(t, e.Snapshots, "a check changes no state")
		assert.Zero(t, e.HPDelta)
		assert.Equal(t, charID, e.CharacterID)
	}
	assert.JSONEq(t, `{"ability":"str","die":20,"roll":14,"abilityScore":12,"target":16,
		"adjustment":0,"adjustedTarget":16,"success":true,"rolledSuccess":true}`, string(f.entries[0].Payload))
	assert.JSONEq(t, `{"ability":"cha","die":20,"roll":17,"abilityScore":15,"target":15,
		"adjustment":0,"adjustedTarget":15,"success":false,"rolledSuccess":false}`, string(f.entries[1].Payload))
	assert.True(t, actionlog.IsRoll(actionlog.KindAbilityCheck), "a check replaces the result card")

	t.Run("a failed record fails the check", func(t *testing.T) {
		f := newFixture(t)
		boom := errors.New("boom")
		f.scores(charactersvc.AbilityDex, buffsvc.AbilityScore{Base: 15, Effective: 15})
		f.dice.EXPECT().RollDie(20).Return(4).Once()
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(0, boom)
		r, err := f.svc.RollAbility(context.Background(), charID, "dex")
		assert.Nil(t, r)
		require.ErrorIs(t, err, boom)
	})
}

// ADR 0028 step 6: the adjustment after the roll moves the target, +
// easier, - harder; the verdict is recomputed from the stored roll, which
// never changes (one d20 at the roll, none at the adjustment). A new
// adjustment replaces the previous one, 0 removes it and drops the reason,
// the verdict as rolled is kept.
func TestAbilityCheck_Step6_AdjustmentMovesTheTarget(t *testing.T) {
	cases := []struct {
		name           string
		target, roll   int
		steps          []skillsvc.Adjustment
		adjustedTarget int
		reason         string
		success        bool
		rolledSuccess  bool
		prevSuccess    bool
		prevAdj        int
	}{
		{
			name: "DEX 15 at -2: a 14 fails", target: 15, roll: 14,
			steps:          []skillsvc.Adjustment{{Value: -2, Reason: "  ledge  "}},
			adjustedTarget: 13, reason: "ledge", success: false, rolledSuccess: true, prevSuccess: true, prevAdj: 0,
		},
		{
			name: "fail to success", target: 15, roll: 17,
			steps:          []skillsvc.Adjustment{{Value: 3}},
			adjustedTarget: 18, success: true, rolledSuccess: false, prevSuccess: false, prevAdj: 0,
		},
		{
			name: "at the adjusted target succeeds", target: 15, roll: 13,
			steps:          []skillsvc.Adjustment{{Value: -2}},
			adjustedTarget: 13, success: true, rolledSuccess: true, prevSuccess: true, prevAdj: 0,
		},
		{
			name: "a new adjustment replaces the previous one", target: 15, roll: 16,
			steps:          []skillsvc.Adjustment{{Value: 2, Reason: "rope"}, {Value: -1, Reason: "wet"}},
			adjustedTarget: 14, reason: "wet", success: false, rolledSuccess: false, prevSuccess: true, prevAdj: 2,
		},
		{
			name: "0 removes it and drops the reason", target: 15, roll: 14,
			steps:          []skillsvc.Adjustment{{Value: -4, Reason: "rope"}, {Value: 0, Reason: "oops"}},
			adjustedTarget: 15, success: true, rolledSuccess: true, prevSuccess: false, prevAdj: -4,
		},
		{
			name: "natural 20 with an easier adjustment still fails", target: 15, roll: 20,
			steps:          []skillsvc.Adjustment{{Value: 8}},
			adjustedTarget: 23, success: false, rolledSuccess: false, prevSuccess: false, prevAdj: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			_, e := f.rollAbility(t, charactersvc.AbilityDex, 16, c.target, c.roll)
			f.abilityAmends(e, nil)
			var r *skillsvc.AbilityCheckResult
			for _, step := range c.steps {
				var err error
				r, err = f.svc.AdjustAbility(context.Background(), charID, e.ID, step)
				require.NoError(t, err)
			}
			f.dice.AssertNumberOfCalls(t, "RollDie", 1) // an adjustment rolls nothing
			last := c.steps[len(c.steps)-1]
			assert.Equal(t, c.roll, r.Roll, "the roll never changes")
			assert.Equal(t, c.target, r.Target, "the stored target")
			assert.Equal(t, 16, r.AbilityScore)
			assert.Equal(t, last.Value, r.Adjustment)
			assert.Equal(t, c.reason, r.AdjustmentReason)
			assert.Equal(t, c.adjustedTarget, r.AdjustedTarget)
			assert.Equal(t, c.success, r.Success)
			assert.Equal(t, c.rolledSuccess, r.RolledSuccess)
			assert.Equal(t, e.ID, r.LogEntryID)

			p := payloadOf(t, e.EffectivePayload())
			assert.Equal(t, float64(c.roll), p["roll"])
			assert.Equal(t, float64(c.adjustedTarget), p["adjustedTarget"])
			assert.Equal(t, c.success, p["success"])
			assert.Equal(t, c.rolledSuccess, p["rolledSuccess"], "the verdict as rolled is kept")
			assert.Equal(t, c.prevSuccess, p["previousSuccess"])
			assert.Equal(t, float64(c.prevAdj), p["previousAdjustment"])
			if c.reason == "" {
				assert.NotContains(t, p, "adjustmentReason")
			}
		})
	}
}

// ADR 0028 step 6: only the latest roll, and it must be an ability check.
// The log refuses (actionlogsvc TestAmendLatest); the service asks it for an
// ability_check and passes the refusal on untouched (409 voided / not
// latest, 400 not an ability check), rolling nothing.
func TestAbilityCheck_Step6_LatestCheckOnly(t *testing.T) {
	boom := errors.New("boom")
	for _, want := range []error{actionlogsvc.ErrNotLatest, actionlogsvc.ErrAlreadyVoided, actionlogsvc.ErrNotAmendable, boom} {
		t.Run(want.Error(), func(t *testing.T) {
			f := newFixture(t)
			f.log.EXPECT().AmendLatest(mock.Anything, charID, int64(9), actionlog.KindAbilityCheck, mock.Anything).Return(nil, want)
			r, err := f.svc.AdjustAbility(context.Background(), charID, 9, skillsvc.Adjustment{Value: 2})
			assert.Nil(t, r)
			assert.ErrorIs(t, err, want)
			f.dice.AssertNotCalled(t, "RollDie", mock.Anything)
		})
	}

	t.Run("undecodable payload", func(t *testing.T) {
		f := newFixture(t)
		e := &actionlog.Entry{ID: 9, Kind: actionlog.KindAbilityCheck, Payload: json.RawMessage(`[]`)}
		f.abilityAmends(e, nil)
		r, err := f.svc.AdjustAbility(context.Background(), charID, 9, skillsvc.Adjustment{Value: 1})
		assert.Nil(t, r)
		assert.ErrorContains(t, err, "decode ability check #9")
		assert.Empty(t, e.Corrections)
	})
}

// ADR 0028 step 7: each adjustment is an amend correction carrying the
// recomputed payload (previousSuccess, previousAdjustment); the entry is
// never edited; the correction step names abilityAdjustment from and to.
// Two adjustments then one pop leave the first in effect.
func TestAbilityCheck_Step7_AdjustmentIsAnAmendCorrection(t *testing.T) {
	f := newFixture(t)
	_, e := f.rollAbility(t, charactersvc.AbilityDex, 15, 15, 14)
	var changes []actionlog.Change
	f.abilityAmends(e, &changes)
	for _, v := range []int{-2, 1} {
		_, err := f.svc.AdjustAbility(context.Background(), charID, e.ID, skillsvc.Adjustment{Value: v})
		require.NoError(t, err)
	}
	assert.JSONEq(t, `{"ability":"dex","die":20,"roll":14,"abilityScore":15,"target":15,
		"adjustment":0,"adjustedTarget":15,"success":true,"rolledSuccess":true}`, string(e.Payload), "the entry is never edited")
	require.Len(t, e.Corrections, 2, "one amend per adjustment")
	for _, c := range e.Corrections {
		assert.Equal(t, actionlog.CorrectionAmend, c.Kind)
	}
	assert.JSONEq(t, `{"ability":"dex","die":20,"roll":14,"abilityScore":15,"target":15,
		"adjustment":-2,"adjustedTarget":13,"success":false,"rolledSuccess":true,
		"previousSuccess":true,"previousAdjustment":0}`, string(e.Corrections[0].Payload))
	assert.JSONEq(t, `{"ability":"dex","die":20,"roll":14,"abilityScore":15,"target":15,
		"adjustment":1,"adjustedTarget":16,"success":true,"rolledSuccess":true,
		"previousSuccess":false,"previousAdjustment":-2}`, string(e.Corrections[1].Payload))
	assert.Equal(t, []actionlog.Change{
		{Field: skillsvc.ChangeAbilityAdjustment, From: 0, To: -2},
		{Field: skillsvc.ChangeAbilityAdjustment, From: -2, To: 1},
	}, changes)
	assert.Equal(t, "abilityAdjustment", skillsvc.ChangeAbilityAdjustment, "the field the UI words")

	// Undo pops the newest adjustment's step (each amend belongs to its
	// own correction step): its amend is out of effect and the check reads
	// the first adjustment.
	for i := range e.Corrections {
		step := int64(200 + i)
		e.Corrections[i].StepID = &step
	}
	e.Corrections[1].StepUndone = true
	p := payloadOf(t, e.EffectivePayload())
	assert.Equal(t, float64(-2), p["adjustment"])
	assert.Equal(t, false, p["success"])
}

// ADR 0028 step 8: the adjustment is -20..+20, the reason optional, trimmed,
// at most 80 characters (spaces only read as none). Anything else is refused
// (invalid input) before the log is amended.
func TestAbilityCheck_Step8_AdjustmentRanges(t *testing.T) {
	long := strings.Repeat("é", 81)
	cases := []struct {
		name   string
		adj    skillsvc.Adjustment
		ok     bool
		reason string
	}{
		{"+20", skillsvc.Adjustment{Value: 20}, true, ""},
		{"-20", skillsvc.Adjustment{Value: -20}, true, ""},
		{"+21", skillsvc.Adjustment{Value: 21}, false, ""},
		{"-21", skillsvc.Adjustment{Value: -21}, false, ""},
		{"reason of 80", skillsvc.Adjustment{Value: 1, Reason: "  " + long[:80*2] + "  "}, true, long[:80*2]},
		{"reason of 81", skillsvc.Adjustment{Value: 1, Reason: long}, false, ""},
		{"reason of spaces only reads as none", skillsvc.Adjustment{Value: 1, Reason: "   "}, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			_, e := f.rollAbility(t, charactersvc.AbilityInt, 14, 14, 9)
			f.abilityAmends(e, nil)
			r, err := f.svc.AdjustAbility(context.Background(), charID, e.ID, c.adj)
			if c.ok {
				require.NoError(t, err)
				assert.Equal(t, c.reason, r.AdjustmentReason)
				assert.Len(t, e.Corrections, 1)
				return
			}
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.Empty(t, e.Corrections, "nothing amended")
			f.log.AssertNotCalled(t, "AmendLatest", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
