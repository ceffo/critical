package skillsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// The buff a successful check puts on (ADR 0037 step 20). The rule lives in the rules data
// (ruletablesvc.SuccessBuffFor): Tumbling, AC -4 for 1 round.

// tumbler is the bard with Tumbling picked as well as the fixture's catalogue: Dex 17, target 17.
func tumbler() *charactersvc.Character {
	c := bard()
	picks := append([]skills.PickFigures{}, catalog...)
	picks = append(picks, skills.PickFigures{ID: "tumbling", Ability: "dex", Slots: 1, Cost: 1, Known: true})
	c.Derived.Proficiencies = &skills.ProficiencySlots{Picks: picks}
	c.Derived.Proficiencies.SetTargets(func(a string) (int, bool) { return c.Score(charactersvc.Ability(a)) })
	return c
}

func newTumbler(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.chars.ExpectedCalls = nil
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(tumbler(), nil).Maybe()
	return f
}

// buffsSnap is the buff list's snapshot the fake buff service hands back.
var buffsSnap = actionlog.Snapshot{Key: buffsvc.StateKeyBuffs, Before: json.RawMessage(`{"b":1}`), After: json.RawMessage(`{"a":1}`)}

// wantBuff is the buff the rules data puts on.
func wantBuff(b buffsvc.BuffEffect) bool {
	return b.Name == "tumbling" && b.ProficiencyID == "tumbling" && b.Duration == 1 && b.Charges == 0 &&
		b.Effects.AC != nil && *b.Effects.AC == -4 && b.Effects.ToHit == nil && b.SpellID == "" && !b.OnOther
}

// adds makes the buff service hold active and put the buff on as row id.
func (f *fixture) adds(active []buffsvc.BuffEffect, id string) {
	f.buffs.EXPECT().ListActive(mock.Anything, charID).Return(active, nil).Once()
	f.buffs.EXPECT().Add(mock.Anything, charID, mock.MatchedBy(wantBuff)).RunAndReturn(
		func(_ context.Context, _ int64, b buffsvc.BuffEffect) (buffsvc.BuffEffect, actionlog.Snapshot, error) {
			b.ID, b.StartRound = id, 3
			return b, buffsSnap, nil
		}).Once()
}

// buffJSON is the payload's buff as put on, row id.
func buffJSON(id string) string {
	return `{"id":"` + id + `","name":"tumbling","proficiencyId":"tumbling","effects":{"ac":-4},"rounds":1}`
}

// ADR 0037 steps 20.1, 20.4 and 20.5: a success puts the buff on (named from the proficiency,
// AC -4, 1 round) in the check's one entry, which carries the buff list's snapshot and the buff
// in its payload. The check's d20 is the only roll.
func TestSkillCheck_Step20_SuccessAddsTheTumblingBuff(t *testing.T) {
	f := newTumbler(t)
	f.adds(nil, "row-1")
	f.records()
	f.dice.EXPECT().RollDie(20).Return(12).Once()

	r := f.roll(t, "tumbling")

	f.dice.AssertNumberOfCalls(t, "RollDie", 1)
	assert.True(t, r.Success)
	require.NotNil(t, r.Buff)
	assert.Equal(t, "row-1", r.Buff.Buff.ID)
	assert.Equal(t, "tumbling", r.Buff.Buff.ProficiencyID)
	assert.Equal(t, -4, *r.Buff.Buff.Effects.AC)
	assert.Equal(t, 1, r.Buff.Buff.Duration)
	assert.False(t, r.Buff.Renewed)
	require.Len(t, f.entries, 1)
	assert.Equal(t, actionlog.KindSkillCheck, f.entries[0].Kind)
	assert.Equal(t, []actionlog.Snapshot{buffsSnap}, f.entries[0].Snapshots)
	p := payloadOf(t, f.entries[0].Payload)
	got, _ := json.Marshal(p["buff"])
	assert.JSONEq(t, buffJSON("row-1"), string(got))
	assert.NotContains(t, p, "renewed")
	assert.Equal(t, 1, f.tx.calls, "the buff and the entry share the check's transaction")
}

// ADR 0037 step 20.2: a failed check and a natural 20 (always a failure, ADR 0014 step 2) put
// nothing on: the buff service is never asked (its mock refuses any call) and the entry keeps
// no snapshots.
func TestSkillCheck_Step20_FailureAndNaturalTwentyAddNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		roll int
		pre  skillsvc.Adjustment
	}{
		{"over the target", 18, skillsvc.Adjustment{}},
		{"natural 20 under an adjusted target of 22", 20, skillsvc.Adjustment{Value: 5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTumbler(t)
			e := f.rolled(t, "tumbling", 20, tc.roll, tc.pre)
			f.dice.AssertNumberOfCalls(t, "RollDie", 1)
			assert.Empty(t, f.entries[0].Snapshots)
			assert.NotContains(t, payloadOf(t, e.Payload), "buff")
		})
	}
}

// ADR 0037 step 20.3: a success while the buff is on renews it (buffsvc.Add replaces the row
// of the same name, a new one starting again); the payload says it was renewed.
func TestSkillCheck_Step20_SuccessAgainRenews(t *testing.T) {
	f := newTumbler(t)
	old := buffsvc.BuffEffect{ID: "row-1", Name: "tumbling", ProficiencyID: "tumbling", Duration: 1}
	f.adds([]buffsvc.BuffEffect{old}, "row-2")
	f.records()
	f.dice.EXPECT().RollDie(20).Return(5).Once()

	r := f.roll(t, "tumbling")

	require.NotNil(t, r.Buff)
	assert.True(t, r.Buff.Renewed)
	assert.Equal(t, "row-2", r.Buff.Buff.ID)
	p := payloadOf(t, f.entries[0].Payload)
	assert.Equal(t, true, p["renewed"])
	f.buffs.AssertNumberOfCalls(t, "Add", 1)
}

// ADR 0037 step 20.6: the rule is data keyed by proficiency id; a proficiency the data has no
// row for, a bard skill and a thieving skill put nothing on, however they succeed.
func TestSkillCheck_Step20_OtherProficienciesAddNothing(t *testing.T) {
	for _, tc := range []struct {
		id        string
		die, roll int
	}{
		{"musical_instrument", 20, 3},
		{"acrobatics", 20, 1},
		{"climb_walls", 100, 10},
	} {
		t.Run(tc.id, func(t *testing.T) {
			f := newTumbler(t)
			e := f.rolled(t, tc.id, tc.die, tc.roll, skillsvc.Adjustment{})
			assert.Equal(t, true, payloadOf(t, e.Payload)["success"])
			assert.Empty(t, f.entries[0].Snapshots)
			assert.NotContains(t, payloadOf(t, e.Payload), "buff")
		})
	}
}

// amendsWithSnaps is amends keeping the amendments' snapshots too.
func (f *fixture) amendsWithSnaps(e *actionlog.Entry, snaps *[][]actionlog.Snapshot) {
	f.log.EXPECT().AmendLatest(mock.Anything, charID, e.ID, actionlog.KindSkillCheck, mock.Anything).RunAndReturn(
		func(_ context.Context, _, _ int64, _ actionlog.Kind, fn actionlog.AmendFunc) (*actionlog.Entry, error) {
			a, err := fn(*e)
			if err != nil {
				return nil, err
			}
			e.Corrections = append(e.Corrections, actionlog.Correction{ActionID: e.ID, Kind: actionlog.CorrectionAmend, Payload: a.Payload})
			*snaps = append(*snaps, a.Snapshots)
			return e, nil
		})
}

// inRound makes the buff service's round counter read round (the fixture's checks are rolled in
// round 0).
func (f *fixture) inRound(round int) {
	f.buffs.EXPECT().GetRound(mock.Anything, charID).Return(round, nil).Once()
}

func (f *fixture) adjust(t *testing.T, e *actionlog.Entry, value int) *skillsvc.CheckResult {
	t.Helper()
	r, err := f.svc.Adjust(context.Background(), charID, e.ID, skillsvc.Adjustment{Value: value})
	require.NoError(t, err)
	return r
}

// ADR 0037 step 20.8: an adjustment that turns the success into a failure takes the check's
// buff off in the same correction, which carries the buff list's snapshot; the payload in effect
// has no buff any more. A buff already gone (ended, dismissed) is no error and no snapshot.
func TestSkillCheck_Step20_AdjustmentToFailureRemovesTheBuff(t *testing.T) {
	t.Run("the buff on", func(t *testing.T) {
		f := newTumbler(t)
		f.adds(nil, "row-1")
		e := f.rolled(t, "tumbling", 20, 15, skillsvc.Adjustment{})
		var snaps [][]actionlog.Snapshot
		f.amendsWithSnaps(e, &snaps)
		removed := actionlog.Snapshot{Key: buffsvc.StateKeyBuffs, Before: json.RawMessage(`{"r":1}`), After: json.RawMessage(`{"r":2}`)}
		f.inRound(0)
		f.buffs.EXPECT().Remove(mock.Anything, charID, "row-1", []buffsvc.BuffEffect(nil)).Return(true, removed, nil).Once()

		r := f.adjust(t, e, -3)

		assert.False(t, r.Success)
		assert.Nil(t, r.Buff)
		assert.Equal(t, [][]actionlog.Snapshot{{removed}}, snaps)
		p := payloadOf(t, e.EffectivePayload())
		assert.NotContains(t, p, "buff")
		assert.NotContains(t, p, "renewed")
		f.dice.AssertNumberOfCalls(t, "RollDie", 1)
	})
	t.Run("the buff already gone", func(t *testing.T) {
		f := newTumbler(t)
		f.adds(nil, "row-1")
		e := f.rolled(t, "tumbling", 20, 15, skillsvc.Adjustment{})
		var snaps [][]actionlog.Snapshot
		f.amendsWithSnaps(e, &snaps)
		f.inRound(0)
		f.buffs.EXPECT().Remove(mock.Anything, charID, "row-1", []buffsvc.BuffEffect(nil)).Return(false, actionlog.Snapshot{}, nil).Once()

		r := f.adjust(t, e, -3)

		assert.Nil(t, r.Buff)
		assert.Equal(t, [][]actionlog.Snapshot{nil}, snaps)
	})
}

// ADR 0037 steps 20.8 and 20.3: an adjustment that turns a failure into a success puts the buff
// on in the correction (renewing one already on), with the buff list's snapshot.
func TestSkillCheck_Step20_AdjustmentToSuccessAddsOrRenewsTheBuff(t *testing.T) {
	for _, tc := range []struct {
		name    string
		active  []buffsvc.BuffEffect
		renewed bool
	}{
		{"none on: added", nil, false},
		{"one on: renewed", []buffsvc.BuffEffect{{ID: "row-0", Name: "tumbling", ProficiencyID: "tumbling", Duration: 1}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTumbler(t)
			e := f.rolled(t, "tumbling", 20, 19, skillsvc.Adjustment{})
			var snaps [][]actionlog.Snapshot
			f.amendsWithSnaps(e, &snaps)
			f.inRound(0)
			f.adds(tc.active, "row-1")

			r := f.adjust(t, e, 2)

			assert.True(t, r.Success)
			require.NotNil(t, r.Buff)
			assert.Equal(t, tc.renewed, r.Buff.Renewed)
			assert.Equal(t, [][]actionlog.Snapshot{{buffsSnap}}, snaps)
			p := payloadOf(t, e.EffectivePayload())
			got, _ := json.Marshal(p["buff"])
			assert.JSONEq(t, buffJSON("row-1"), string(got))
			f.dice.AssertNumberOfCalls(t, "RollDie", 1)
		})
	}
}

// ADR 0037 step 20.8: an adjustment that leaves the verdict as it was touches the buff list not
// at all (the buff service's mock refuses any call): a success stays a success with its buff, a
// failure stays one, and a natural 20 stays a failure whatever the adjustment.
func TestSkillCheck_Step20_AdjustmentKeepingTheVerdictLeavesTheBuff(t *testing.T) {
	t.Run("a success kept", func(t *testing.T) {
		f := newTumbler(t)
		f.adds(nil, "row-1")
		e := f.rolled(t, "tumbling", 20, 15, skillsvc.Adjustment{})
		var snaps [][]actionlog.Snapshot
		f.amendsWithSnaps(e, &snaps)

		r := f.adjust(t, e, -2) // 15 against 15: still a success

		assert.True(t, r.Success)
		require.NotNil(t, r.Buff)
		assert.Equal(t, "row-1", r.Buff.Buff.ID)
		assert.Equal(t, [][]actionlog.Snapshot{nil}, snaps)
		got, _ := json.Marshal(payloadOf(t, e.EffectivePayload())["buff"])
		assert.JSONEq(t, buffJSON("row-1"), string(got))
		f.buffs.AssertNumberOfCalls(t, "Add", 1) // the roll's only
	})
	for _, tc := range []struct {
		name  string
		roll  int
		value int
	}{
		{"a failure kept", 19, 1},
		{"a natural 20 under +20", 20, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTumbler(t)
			e := f.rolled(t, "tumbling", 20, tc.roll, skillsvc.Adjustment{})
			var snaps [][]actionlog.Snapshot
			f.amendsWithSnaps(e, &snaps)

			r := f.adjust(t, e, tc.value)

			assert.False(t, r.Success)
			assert.Nil(t, r.Buff)
			assert.Equal(t, [][]actionlog.Snapshot{nil}, snaps)
		})
	}
}

// ADR 0037 steps 20.3 and 20.8: a success that renewed a row keeps that row, every field of it,
// in its payload; an adjustment that turns it into a failure takes the check's row off and puts
// the renewed one back in its place, so the list reads as if the check had put nothing on (an
// earlier success's buff, or one made by hand, stays on).
func TestSkillCheck_Step20_AdjustmentToFailureGivesBackTheRenewedRow(t *testing.T) {
	ac := -4
	for _, tc := range []struct {
		name string
		old  buffsvc.BuffEffect
	}{
		{"an earlier success's row", buffsvc.BuffEffect{ID: "row-0", Name: "tumbling", ProficiencyID: "tumbling", Duration: 1, StartRound: 0, Effects: buffsvc.BuffEffects{AC: &ac}}},
		{"a row made by hand", buffsvc.BuffEffect{ID: "hand", Name: "Tumbling", Duration: 5, Note: "DM", Effects: buffsvc.BuffEffects{AC: &ac}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTumbler(t)
			f.adds([]buffsvc.BuffEffect{tc.old}, "row-1")
			e := f.rolled(t, "tumbling", 20, 15, skillsvc.Adjustment{})
			p := payloadOf(t, e.Payload)
			assert.Equal(t, true, p["renewed"])
			assert.Contains(t, p, "replaced")
			var snaps [][]actionlog.Snapshot
			f.amendsWithSnaps(e, &snaps)
			f.inRound(0)
			swapped := actionlog.Snapshot{Key: buffsvc.StateKeyBuffs, Before: json.RawMessage(`{"r":1}`), After: json.RawMessage(`{"r":0}`)}
			f.buffs.EXPECT().Remove(mock.Anything, charID, "row-1", []buffsvc.BuffEffect{tc.old}).Return(true, swapped, nil).Once()

			r := f.adjust(t, e, -3)

			assert.False(t, r.Success)
			assert.Nil(t, r.Buff)
			assert.Equal(t, [][]actionlog.Snapshot{{swapped}}, snaps)
			p = payloadOf(t, e.EffectivePayload())
			assert.NotContains(t, p, "buff")
			assert.NotContains(t, p, "renewed")
			assert.NotContains(t, p, "replaced")
			f.dice.AssertNumberOfCalls(t, "RollDie", 1)
		})
	}
}

// ADR 0037 step 20.8 (supervisor ruling 2026-10-20): once a Next Round has passed the check's
// round, its buff's one round is over. A failure turned into a success puts nothing on (the buff
// service is only asked the round); a success turned into a failure takes off only what the check
// still has on, giving nothing back, and a row already ended is no error and no snapshot.
func TestSkillCheck_Step20_AdjustmentAfterTheChecksRound(t *testing.T) {
	t.Run("to success: no buff", func(t *testing.T) {
		f := newTumbler(t)
		e := f.rolled(t, "tumbling", 20, 19, skillsvc.Adjustment{})
		var snaps [][]actionlog.Snapshot
		f.amendsWithSnaps(e, &snaps)
		f.inRound(1)

		r := f.adjust(t, e, 2)

		assert.True(t, r.Success)
		assert.Nil(t, r.Buff)
		assert.Equal(t, [][]actionlog.Snapshot{nil}, snaps)
		assert.NotContains(t, payloadOf(t, e.EffectivePayload()), "buff")
		f.buffs.AssertNotCalled(t, "Add", mock.Anything, mock.Anything, mock.Anything)
	})
	t.Run("to failure: the row still on taken off, nothing given back", func(t *testing.T) {
		f := newTumbler(t)
		f.adds([]buffsvc.BuffEffect{{ID: "row-0", Name: "tumbling", ProficiencyID: "tumbling", Duration: 1}}, "row-1")
		e := f.rolled(t, "tumbling", 20, 15, skillsvc.Adjustment{})
		var snaps [][]actionlog.Snapshot
		f.amendsWithSnaps(e, &snaps)
		f.inRound(2)
		f.buffs.EXPECT().Remove(mock.Anything, charID, "row-1", []buffsvc.BuffEffect(nil)).Return(true, buffsSnap, nil).Once()

		r := f.adjust(t, e, -3)

		assert.False(t, r.Success)
		assert.Equal(t, [][]actionlog.Snapshot{{buffsSnap}}, snaps)
	})
	t.Run("to failure: the row already ended", func(t *testing.T) {
		f := newTumbler(t)
		f.adds(nil, "row-1")
		e := f.rolled(t, "tumbling", 20, 15, skillsvc.Adjustment{})
		var snaps [][]actionlog.Snapshot
		f.amendsWithSnaps(e, &snaps)
		f.inRound(1)
		f.buffs.EXPECT().Remove(mock.Anything, charID, "row-1", []buffsvc.BuffEffect(nil)).Return(false, actionlog.Snapshot{}, nil).Once()

		r := f.adjust(t, e, -3)

		assert.False(t, r.Success)
		assert.Equal(t, [][]actionlog.Snapshot{nil}, snaps)
	})
}
