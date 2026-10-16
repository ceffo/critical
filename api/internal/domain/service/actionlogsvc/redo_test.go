package actionlogsvc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc/mocks"
)

// ADR 0019 steps 1 to 3: redo replays the newest popped transaction, oldest
// entry first, writing each snapshot's "after" back through its restorer and
// recording a redo correction, all inside the one DB transaction of the
// expected-target check. The rollback itself is checked on SQLite in
// actionlogdal (TestRedo_RestorerFailsMidReplay_RollsBackEverything).
func TestRedo_ReplaysTheNewestPoppedTransaction(t *testing.T) {
	void := func(id int64) actionlog.Correction {
		return actionlog.Correction{ActionID: id, Kind: actionlog.CorrectionVoid}
	}
	// A popped Next Round: the advance (#10) and its initiative (#11).
	advance := actionlog.Entry{
		ID: 10, TransactionID: 10, CharacterID: 1, Kind: actionlog.KindRoundAdvance,
		Snapshots: []actionlog.Snapshot{
			{Key: "buffs", Before: json.RawMessage(`["haste"]`), After: json.RawMessage(`[]`)},
			{Key: "round", Before: json.RawMessage(`1`), After: json.RawMessage(`2`)},
		},
		Corrections: []actionlog.Correction{void(10)},
	}
	initiative := actionlog.Entry{
		ID: 11, TransactionID: 10, CharacterID: 1, Kind: actionlog.KindInitiative,
		Snapshots:   []actionlog.Snapshot{{Key: "initiative", Before: json.RawMessage(`null`), After: json.RawMessage(`{"d10":4}`)}},
		Corrections: []actionlog.Correction{void(11)},
	}
	// register records "replay <key> <state>" and "redo #id" in order.
	register := func(svc actionlogsvc.SVC, dal *mocks.MockDAL, calls *[]string, failKey string) {
		for _, key := range []string{"buffs", "round", "initiative"} {
			svc.RegisterRestorer(key, restorerFunc(func(_ context.Context, _ int64, state json.RawMessage) error {
				if key == failKey {
					return errBoom
				}
				*calls = append(*calls, "replay "+key+" "+string(state))
				return nil
			}))
		}
		dal.EXPECT().InsertCorrection(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, c actionlog.Correction) error {
			*calls = append(*calls, fmt.Sprintf("%s #%d", c.Kind, c.ActionID))
			return nil
		}).Maybe()
	}
	// target makes the DAL name transaction #10 as the redo target, with
	// live top #9 below it.
	target := func(dal *mocks.MockDAL) {
		dal.EXPECT().LatestPopped(mock.Anything, int64(1)).Return(10, 10, true, nil).Once()
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(9, true, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(10)).Return(&advance, nil).Once()
	}

	t.Run("oldest first, the after written back", func(t *testing.T) {
		svc, dal, _, tx := newSvc(t)
		var calls []string
		register(svc, dal, &calls, "")
		target(dal)
		dal.EXPECT().ListTransaction(mock.Anything, int64(1), int64(10)).
			Return([]actionlog.Entry{initiative, advance}, nil).Once()
		done := advance
		done.Corrections = []actionlog.Correction{void(10), {ActionID: 10, Kind: actionlog.CorrectionRedo}}
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(10)).Return(&done, nil).Once()

		got, err := svc.Redo(context.Background(), 1, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{
			"replay buffs []", "replay round 2", "redo #10",
			`replay initiative {"d10":4}`, "redo #11",
		}, calls, "each entry replayed then redone, oldest first")
		assert.Equal(t, int64(10), got.ID, "answers with the transaction's first entry")
		assert.Equal(t, actionlog.StatusActive, got.Status(), "live again")
		assert.Equal(t, 1, tx.calls, "check, replays and redos in one transaction")
	})

	t.Run("restorer fails mid-replay: no partial state", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		tx := &errTx{}
		svc, err := actionlogsvc.NewWithConfig(actionlogsvc.DefaultConfig(), dal, tx, mocks.NewMockRounds(t), mocks.NewMockMaxHP(t), nil)
		require.NoError(t, err)
		var calls []string
		register(svc, dal, &calls, "initiative")
		target(dal)
		dal.EXPECT().ListTransaction(mock.Anything, int64(1), int64(10)).
			Return([]actionlog.Entry{initiative, advance}, nil).Once()

		_, err = svc.Redo(context.Background(), 1, 10)
		require.ErrorIs(t, err, errBoom)
		assert.Equal(t, []string{"replay buffs []", "replay round 2", "redo #10"}, calls,
			"stops at the failing restore: #11 is never redone")
		require.ErrorIs(t, tx.bodyErr, errBoom, "the error leaves the transaction body, so the DB transaction rolls back #10's replay and redo")
	})

	t.Run("stale expected redo changes nothing", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		var calls []string
		register(svc, dal, &calls, "")
		// #11 is in the target transaction but is not its first entry: refused too.
		for _, expected := range []int64{11, 7} {
			target(dal)
			_, err := svc.Redo(context.Background(), 1, expected)
			require.ErrorIs(t, err, actionlogsvc.ErrRedoChanged)
			assert.Contains(t, err.Error(), fmt.Sprintf("the transaction to redo is #10 (round_advance, round 0), not #%d", expected))
		}
		assert.Empty(t, calls, "nothing replayed, nothing redone")
	})

	t.Run("nothing popped: nothing to redo", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestPopped(mock.Anything, int64(1)).Return(0, 0, false, nil).Once()
		_, err := svc.Redo(context.Background(), 1, 10)
		require.ErrorIs(t, err, actionlogsvc.ErrNothingToRedo)
	})

	// A transaction pushed after the pop is above it: while it is live, the
	// popped transaction is below the live top and is no longer redoable.
	t.Run("a newer transaction is live: nothing to redo", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestPopped(mock.Anything, int64(1)).Return(11, 10, true, nil).Once()
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(12, true, nil).Once()
		_, err := svc.Redo(context.Background(), 1, 10)
		require.ErrorIs(t, err, actionlogsvc.ErrNothingToRedo)

		dal.EXPECT().LatestPopped(mock.Anything, int64(1)).Return(11, 10, true, nil).Once()
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(12, true, nil).Once()
		got, err := svc.RedoTarget(context.Background(), 1)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	// A transaction voided only in part (an entry voided on its own before
	// transactions were popped whole): its first entry is still the live top,
	// so it is not above it and is not redoable.
	t.Run("the live top is the popped entry's transaction: nothing to redo", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestPopped(mock.Anything, int64(1)).Return(11, 10, true, nil).Once()
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(10, true, nil).Once()
		got, err := svc.RedoTarget(context.Background(), 1)
		require.NoError(t, err)
		assert.Nil(t, got)

		dal.EXPECT().LatestPopped(mock.Anything, int64(1)).Return(11, 10, true, nil).Once()
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(10, true, nil).Once()
		_, err = svc.Redo(context.Background(), 1, 10)
		require.ErrorIs(t, err, actionlogsvc.ErrNothingToRedo)
	})

	t.Run("redo target with an empty live stack", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestPopped(mock.Anything, int64(1)).Return(11, 10, true, nil).Once()
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(0, false, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(10)).Return(&advance, nil).Once()
		got, err := svc.RedoTarget(context.Background(), 1)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, int64(10), got.ID, "named by the transaction's first entry")
	})

	t.Run("lookup fails", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestPopped(mock.Anything, int64(1)).Return(0, 0, false, errBoom).Once()
		_, err := svc.Redo(context.Background(), 1, 10)
		require.ErrorIs(t, err, errBoom)
	})
}

// ADR 0019 step 1: an entry's newest void or redo decides whether it is
// popped, so an entry can be popped, replayed and popped again.
func TestEntry_NewestVoidOrRedoDecides(t *testing.T) {
	note := "lucky"
	c := func(kind actionlog.CorrectionKind) actionlog.Correction { return actionlog.Correction{Kind: kind} }
	for _, tc := range []struct {
		name  string
		corrs []actionlog.Correction
		want  actionlog.Status
	}{
		{"never popped", nil, actionlog.StatusActive},
		{"popped", []actionlog.Correction{c(actionlog.CorrectionVoid)}, actionlog.StatusVoided},
		{"popped, redone", []actionlog.Correction{c(actionlog.CorrectionVoid), c(actionlog.CorrectionRedo)}, actionlog.StatusActive},
		{"popped, redone, popped", []actionlog.Correction{c(actionlog.CorrectionVoid), c(actionlog.CorrectionRedo), c(actionlog.CorrectionVoid)}, actionlog.StatusVoided},
		{"a note after the void does not un-pop", []actionlog.Correction{c(actionlog.CorrectionVoid), {Kind: actionlog.CorrectionAmend, Note: &note}}, actionlog.StatusVoided},
		{"redone value amend reads amended", []actionlog.Correction{{Kind: actionlog.CorrectionAmend, Payload: json.RawMessage(`{"passed":true}`)}, c(actionlog.CorrectionVoid), c(actionlog.CorrectionRedo)}, actionlog.StatusAmended},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := actionlog.Entry{HPDelta: -4, Corrections: tc.corrs}
			assert.Equal(t, tc.want, e.Status())
			assert.Equal(t, tc.want == actionlog.StatusVoided, e.Voided())
			if tc.want == actionlog.StatusVoided {
				assert.Equal(t, 0, e.EffectiveHPDelta())
			} else {
				assert.Equal(t, -4, e.EffectiveHPDelta())
			}
		})
	}
}
