package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// nextRound sets up a Next Round from round 5 with no buffs, where the
// previous round's initiative is prev (nil: none), and the new d10 shows d10.
func nextRound(f *fixture, prev *buffsvc.Initiative, d10 int) {
	f.state(nil, 5)
	f.records()
	f.saves([]buffsvc.BuffEffect{})
	f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 6).RunAndReturn(
		func(context.Context, int64, int) error { f.calls = append(f.calls, "save round"); return nil }).Once()
	f.initiativeIs(prev)
	if prev != nil {
		f.savesInitiative() // the previous round's initiative ends
	}
	f.rollsInitiative(d10)
}

func initiativePayload(t *testing.T, e actionlog.NewEntry) map[string]any {
	t.Helper()
	assert.Equal(t, actionlog.KindInitiative, e.Kind)
	return payload(t, e)
}

// ADR 0013 step 1: Next Round rolls exactly one d10 through the dice port.
func TestInitiative_Step1_NextRoundRollsOneD10(t *testing.T) {
	f := newFixture(t)
	nextRound(f, nil, 7) // the dice mock fails on any other roll or a second one

	_, _, round, initiative, err := f.svc.AdvanceRound(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, 6, round)
	assert.Equal(t, buffsvc.Initiative{D10: 7, LogEntryID: 2}, initiative, "Next Round returns the roll it made")
	f.dice.AssertNumberOfCalls(t, "RollDie", 1)
	require.Len(t, f.entries, 2)
	assert.Equal(t, map[string]any{"round": 6.0, "d10": 7.0}, initiativePayload(t, f.entries[1]))
	require.Len(t, f.initiatives, 1)
	assert.Equal(t, &buffsvc.Initiative{D10: 7, LogEntryID: 2}, f.initiatives[0],
		"the stored initiative is the die and the id of the entry that rolled it")
}

// ADR 0013 step 2: the initiative is its own entry, recorded after the round
// advance, stamped with the new round; the advance ends the previous round's
// initiative (snapshotted, so voiding the advance brings it back). All in one
// transaction.
func TestInitiative_Step2_OwnEntryAfterTheAdvance(t *testing.T) {
	prev := &buffsvc.Initiative{D10: 3, LogEntryID: 40}
	f := newFixture(t)
	nextRound(f, prev, 8)

	_, _, _, _, err := f.svc.AdvanceRound(context.Background(), charID)
	require.NoError(t, err)

	require.Len(t, f.entries, 2)
	advance, initiative := f.entries[0], f.entries[1]
	assert.Equal(t, actionlog.KindRoundAdvance, advance.Kind)
	assert.Equal(t, []string{buffsvc.StateKeyBuffs, buffsvc.StateKeyRound, buffsvc.StateKeyInitiative}, snapKeys(advance))
	assert.JSONEq(t, `{"d10":3,"entryId":40}`, snaptest.State(t, advance.Snapshots[2].Before))
	assert.True(t, snaptest.IsAbsent(t, advance.Snapshots[2].After))

	assert.Equal(t, actionlog.KindInitiative, initiative.Kind)
	assert.Equal(t, 6, initiative.Round, "stamped with the round it is rolled for")
	assert.Empty(t, initiative.Summary, "no display text: the UI words the payload")
	assert.Equal(t, []string{buffsvc.StateKeyInitiative}, snapKeys(initiative))
	assert.True(t, snaptest.IsAbsent(t, initiative.Snapshots[0].Before), "voiding it leaves the new round without one")
	assert.JSONEq(t, `{"d10":8}`, snaptest.State(t, initiative.Snapshots[0].After))

	assert.Equal(t, []string{"record", "save buffs", "save round", "save initiative", "record", "save initiative"}, f.calls)
	assert.Equal(t, []*buffsvc.Initiative{nil, {D10: 8, LogEntryID: 2}}, f.initiatives)
	assert.Equal(t, 1, f.tx.calls, "the advance and the initiative commit together")
}

// ADR 0013 step 3: the initiative is the plain d10, no modifier from buffs or
// anything else, for every face (lowest acts first is the display's).
func TestInitiative_Step3_PlainD10NoModifier(t *testing.T) {
	bless := buffsvc.BuffEffect{ID: "b", Name: "Bless", Duration: 9, Effects: buffsvc.BuffEffects{ToHit: intPtr(1), AC: intPtr(-2)}}
	for _, d10 := range []int{1, 6, 10} {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{bless}, 2)
		f.records()
		f.saves([]buffsvc.BuffEffect{{ID: "b", Name: "Bless", Duration: 8, Effects: bless.Effects}})
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 3).Return(nil).Once()
		f.initiativeIs(nil)
		f.rollsInitiative(d10)

		_, _, _, _, err := f.svc.AdvanceRound(context.Background(), charID)
		require.NoError(t, err)
		assert.Equal(t, float64(d10), initiativePayload(t, f.entries[1])["d10"])
		assert.Equal(t, d10, f.initiatives[0].D10)
	}
}

// ADR 0013 step 4 as amended by ADR 0018 step 5: a reroll voids nothing; it
// records a new initiative entry (its own transaction) whose snapshot keeps
// the replaced roll as "before", and is refused unless the initiative's entry
// is the newest one.
func TestInitiative_Step4_RerollReplacesTheNewestInitiative(t *testing.T) {
	ctx := context.Background()
	current := &buffsvc.Initiative{D10: 4, LogEntryID: 12}

	t.Run("records one new d10 with the replaced roll as before, voids nothing", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(3, nil).Once()
		f.initiativeIs(current)
		f.log.EXPECT().TopID(mock.Anything, charID).Return(12, true, nil).Once()
		f.initiativeIs(current)
		f.dice.EXPECT().RollDie(10).Return(9).Once()
		f.records()
		f.savesInitiative()

		got, err := f.svc.RerollInitiative(ctx, charID)
		require.NoError(t, err)
		assert.Equal(t, buffsvc.Initiative{D10: 9, LogEntryID: 1}, got)
		f.dice.AssertNumberOfCalls(t, "RollDie", 1)
		assert.Equal(t, []string{"record", "save initiative"}, f.calls)
		require.Len(t, f.entries, 1)
		assert.Equal(t, actionlog.KindInitiative, f.entries[0].Kind)
		assert.Equal(t, 3, f.entries[0].Round)
		assert.Equal(t, map[string]any{"round": 3.0, "d10": 9.0}, initiativePayload(t, f.entries[0]))
		require.Len(t, f.entries[0].Snapshots, 1)
		snap := f.entries[0].Snapshots[0]
		assert.Equal(t, buffsvc.StateKeyInitiative, snap.Key)
		assert.JSONEq(t, `{"d10":4,"entryId":12}`, snaptest.State(t, snap.Before), "undo gives the replaced roll back")
		assert.JSONEq(t, `{"d10":9}`, snaptest.State(t, snap.After))
		assert.Equal(t, 1, f.tx.calls)
	})

	refused := []struct {
		name string
		top  int64
		ok   bool
	}{
		{"another entry is newer", 13, true},
		{"nothing on the log", 0, false},
	}
	for _, tt := range refused {
		t.Run(tt.name+": refused, nothing rolled", func(t *testing.T) {
			f := newFixture(t)
			f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(3, nil).Once()
			f.initiativeIs(current)
			f.log.EXPECT().TopID(mock.Anything, charID).Return(tt.top, tt.ok, nil).Once()

			_, err := f.svc.RerollInitiative(ctx, charID)
			assert.ErrorIs(t, err, domain.ErrConflict)
			assert.ErrorIs(t, err, buffsvc.ErrInitiativeNotOnTop)
			assert.Empty(t, f.entries)
			f.dice.AssertNotCalled(t, "RollDie", mock.Anything)
		})
	}

	t.Run("no initiative: a conflict, nothing rolled", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(3, nil).Once()
		f.initiativeIs(nil)

		_, err := f.svc.RerollInitiative(ctx, charID)
		assert.ErrorIs(t, err, domain.ErrConflict)
		assert.ErrorIs(t, err, buffsvc.ErrNoInitiative)
		assert.Empty(t, f.entries)
	})
}

// ADR 0013 step 5: out of combat (round 0) there is no initiative. Every move
// to round 0 ends it, snapshotted so voiding the move brings it back; a Set
// round to another round keeps it, and there is nothing to reroll at round 0.
func TestInitiative_Step5_NoneOutOfCombat(t *testing.T) {
	ctx := context.Background()
	current := &buffsvc.Initiative{D10: 4, LogEntryID: 12}
	toZero := []struct {
		name string
		run  func(f *fixture) error
	}{
		{"End combat", func(f *fixture) error {
			_, _, err := f.svc.ResetRound(ctx, charID)
			return err
		}},
		{"Set round to 0", func(f *fixture) error {
			_, _, err := f.svc.SetRound(ctx, charID, 0)
			return err
		}},
		{"passing time", func(f *fixture) error {
			f.saves([]buffsvc.BuffEffect{})
			_, _, _, _, err := f.svc.PassTime(ctx, charID, 1)
			return err
		}},
	}
	for _, tt := range toZero {
		t.Run(tt.name+" ends the initiative", func(t *testing.T) {
			f := newFixture(t)
			f.state(nil, 4)
			f.records()
			f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 0).Return(nil).Once()
			f.initiativeIs(current)
			f.savesInitiative()

			require.NoError(t, tt.run(f))
			assert.Equal(t, []*buffsvc.Initiative{nil}, f.initiatives)
			require.Len(t, f.entries, 1)
			snaps := f.entries[0].Snapshots
			last := snaps[len(snaps)-1]
			assert.Equal(t, buffsvc.StateKeyInitiative, last.Key)
			assert.JSONEq(t, `{"d10":4,"entryId":12}`, snaptest.State(t, last.Before))
			assert.True(t, snaptest.IsAbsent(t, last.After))
		})
	}

	t.Run("Set round to another round keeps it", func(t *testing.T) {
		f := newFixture(t)
		f.state(nil, 4)
		f.records()
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 7).Return(nil).Once()
		// No GetInitiative / SaveInitiative expected: the mock fails on either.

		_, _, err := f.svc.SetRound(ctx, charID, 7)
		require.NoError(t, err)
		assert.Equal(t, []string{buffsvc.StateKeyRound}, snapKeys(f.entries[0]))
	})

	t.Run("nothing to reroll at round 0", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(0, nil).Once()
		f.initiativeIs(nil)

		_, err := f.svc.RerollInitiative(ctx, charID)
		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("voiding the move writes the initiative back", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().SaveInitiative(mock.Anything, charID, current).Return(nil).Once()
		err := f.svc.Restorers()[buffsvc.StateKeyInitiative].Restore(ctx, charID, snaptest.Env(`{"d10":4,"entryId":12}`))
		require.NoError(t, err)
	})
}

func TestInitiativeRestorer(t *testing.T) {
	ctx := context.Background()
	t.Run("null clears it", func(t *testing.T) {
		f := newFixture(t)
		f.dal.EXPECT().SaveInitiative(mock.Anything, charID, (*buffsvc.Initiative)(nil)).Return(nil).Once()
		require.NoError(t, f.svc.Restorers()[buffsvc.StateKeyInitiative].Restore(ctx, charID, snaptest.Absent("")))
	})
	for _, bad := range []string{`{"d10":0}`, `{"d10":11}`, `"seven"`} {
		t.Run("rejects "+bad, func(t *testing.T) {
			f := newFixture(t) // nothing is written
			assert.Error(t, f.svc.Restorers()[buffsvc.StateKeyInitiative].Restore(ctx, charID, snaptest.Env(bad)))
		})
	}
	// ADR 0019 step 1: the "after" of the entry that rolled the initiative
	// has no entry id (it was not assigned yet); a redo replays it with the
	// replayed entry's id, which the reroll guard reads (ADR 0018 step 5).
	t.Run("replay names the replayed entry", func(t *testing.T) {
		f := newFixture(t)
		replayer, ok := f.svc.Restorers()[buffsvc.StateKeyInitiative].(actionlog.Replayer)
		require.True(t, ok, "the initiative restorer replays with the entry id")
		f.dal.EXPECT().SaveInitiative(mock.Anything, charID, &buffsvc.Initiative{D10: 8, LogEntryID: 21}).Return(nil).Once()
		require.NoError(t, replayer.Replay(ctx, charID, 21, snaptest.Env(`{"d10":8}`)))
		f.dal.EXPECT().SaveInitiative(mock.Anything, charID, (*buffsvc.Initiative)(nil)).Return(nil).Once()
		require.NoError(t, replayer.Replay(ctx, charID, 21, snaptest.Absent("")), "an after with no initiative clears it")
		assert.Error(t, replayer.Replay(ctx, charID, 21, snaptest.Env(`{"d10":11}`)))
	})
}

func TestGetInitiative(t *testing.T) {
	f := newFixture(t)
	f.initiativeIs(&buffsvc.Initiative{D10: 5, LogEntryID: 3})
	got, err := f.svc.GetInitiative(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, &buffsvc.Initiative{D10: 5, LogEntryID: 3}, got)
}
