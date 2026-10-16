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

// ADR 0010 steps 2 to 4: Undo pops the newest non-voided entry of any kind,
// only when it is the entry the client expects, in one transaction.
func TestUndo(t *testing.T) {
	attack := func() *actionlog.Entry {
		return &actionlog.Entry{
			ID: 9, TransactionID: 9, CharacterID: 1, Round: 3, Kind: actionlog.KindAttack,
			Snapshots: []actionlog.Snapshot{{Key: "items:arrows_plus3", Before: json.RawMessage(`{"count":20}`), After: json.RawMessage(`{"count":19}`)}},
		}
	}
	voidRow := actionlog.Correction{ActionID: 9, Kind: actionlog.CorrectionVoid}

	t.Run("matching expected top pops it and restores its snapshots", func(t *testing.T) {
		svc, dal, _, tx := newSvc(t)
		var restored []string
		svc.RegisterRestorer("items", restorerFunc(func(_ context.Context, charID int64, state json.RawMessage) error {
			assert.Equal(t, int64(1), charID)
			restored = append(restored, string(state))
			return nil
		}))
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(9, true, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(9)).Return(attack(), nil).Once()
		dal.EXPECT().ListTransaction(mock.Anything, int64(1), int64(9)).Return([]actionlog.Entry{*attack()}, nil).Once()
		dal.EXPECT().InsertCorrection(mock.Anything, voidRow).Return(nil).Once()
		done := attack()
		done.Corrections = []actionlog.Correction{voidRow}
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(9)).Return(done, nil).Once()

		got, err := svc.Undo(context.Background(), 1, 9)
		require.NoError(t, err)
		assert.Equal(t, 1, tx.calls, "check, restore and void in one transaction")
		assert.Equal(t, actionlog.StatusVoided, got.Status())
		assert.Equal(t, []string{`{"count":20}`}, restored)
	})

	// Stale: another action landed after the client read the log. Nothing is
	// restored and no void is written (the mock DAL fails on InsertCorrection).
	t.Run("stale expected top is refused and nothing changes", func(t *testing.T) {
		svc, dal, _, tx := newSvc(t)
		svc.RegisterRestorer("items", restorerFunc(func(context.Context, int64, json.RawMessage) error {
			t.Fatal("nothing may be restored on a stale undo")
			return nil
		}))
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(9, true, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(9)).Return(attack(), nil).Once()

		_, err := svc.Undo(context.Background(), 1, 8)
		require.ErrorIs(t, err, actionlogsvc.ErrTopChanged)
		assert.Contains(t, err.Error(), "the newest entry is #9 (attack, round 3), not #8")
		assert.Equal(t, 1, tx.calls)
	})

	// The expected entry was already popped (by another client): the top is
	// now an older entry, which must not be popped in its place.
	t.Run("expected top already popped is refused", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(9, true, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(9)).Return(attack(), nil).Once()

		_, err := svc.Undo(context.Background(), 1, 10)
		require.ErrorIs(t, err, actionlogsvc.ErrTopChanged)
	})

	t.Run("empty log", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(0, false, nil).Once()

		_, err := svc.Undo(context.Background(), 1, 9)
		require.ErrorIs(t, err, actionlogsvc.ErrNothingToUndo)
	})

	t.Run("restorer fails: no void written", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		svc.RegisterRestorer("items", restorerFunc(func(context.Context, int64, json.RawMessage) error { return errBoom }))
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(9, true, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(9)).Return(attack(), nil).Once()
		dal.EXPECT().ListTransaction(mock.Anything, int64(1), int64(9)).Return([]actionlog.Entry{*attack()}, nil).Once()

		_, err := svc.Undo(context.Background(), 1, 9)
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("latest lookup fails", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(0, false, errBoom).Once()

		_, err := svc.Undo(context.Background(), 1, 9)
		require.ErrorIs(t, err, errBoom)
	})
}

// errTx runs the body directly and keeps the error it returned: the error a
// real runner rolls the DB transaction back on.
type errTx struct{ bodyErr error }

func (e *errTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	e.bodyErr = fn(ctx)
	return e.bodyErr
}

// ADR 0018 step 2: undo pops every non-voided entry of the top entry's
// transaction, newest first, restoring each snapshot then voiding the entry,
// all inside the one DB transaction of the expected-top check. The rollback
// itself is checked on SQLite in actionlogdal
// (TestUndo_RestorerFailsMidTransaction_RollsBackEverything).
func TestUndo_PopsTheWholeTransaction(t *testing.T) {
	// Next Round: the advance (#10) starts the transaction, its initiative
	// (#11) joins it; #12 of the same transaction was popped already (as an
	// older row could be) and is skipped.
	advance := actionlog.Entry{
		ID: 10, TransactionID: 10, CharacterID: 1, Kind: actionlog.KindRoundAdvance,
		Snapshots: []actionlog.Snapshot{
			{Key: "buffs", Before: json.RawMessage(`["haste"]`), After: json.RawMessage(`[]`)},
			{Key: "round", Before: json.RawMessage(`1`), After: json.RawMessage(`2`)},
		},
	}
	initiative := actionlog.Entry{
		ID: 11, TransactionID: 10, CharacterID: 1, Kind: actionlog.KindInitiative,
		Snapshots: []actionlog.Snapshot{{Key: "initiative", Before: json.RawMessage(`null`), After: json.RawMessage(`{"d10":4}`)}},
	}
	popped := actionlog.Entry{
		ID: 12, TransactionID: 10, CharacterID: 1, Kind: actionlog.KindInitiative,
		Snapshots:   []actionlog.Snapshot{{Key: "initiative", Before: json.RawMessage(`{"d10":4}`), After: json.RawMessage(`{"d10":7}`)}},
		Corrections: []actionlog.Correction{{ActionID: 12, Kind: actionlog.CorrectionVoid}},
	}
	voidOf := func(id int64) actionlog.Correction {
		return actionlog.Correction{ActionID: id, Kind: actionlog.CorrectionVoid}
	}
	// register records "restore <key> <before>" and "void #id" in order.
	register := func(svc actionlogsvc.SVC, dal *mocks.MockDAL, calls *[]string, failKey string) {
		for _, key := range []string{"buffs", "round", "initiative"} {
			svc.RegisterRestorer(key, restorerFunc(func(_ context.Context, _ int64, state json.RawMessage) error {
				if key == failKey {
					return errBoom
				}
				*calls = append(*calls, "restore "+key+" "+string(state))
				return nil
			}))
		}
		dal.EXPECT().InsertCorrection(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, c actionlog.Correction) error {
			*calls = append(*calls, fmt.Sprintf("void #%d", c.ActionID))
			return nil
		}).Maybe()
	}

	t.Run("newest first", func(t *testing.T) {
		svc, dal, _, tx := newSvc(t)
		var calls []string
		register(svc, dal, &calls, "")
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(11, true, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(11)).Return(&initiative, nil).Once()
		dal.EXPECT().ListTransaction(mock.Anything, int64(1), int64(10)).
			Return([]actionlog.Entry{popped, initiative, advance}, nil).Once()
		done := advance
		done.Corrections = []actionlog.Correction{voidOf(10)}
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(10)).Return(&done, nil).Once()

		got, err := svc.Undo(context.Background(), 1, 11)
		require.NoError(t, err)
		assert.Equal(t, []string{
			"restore initiative null", "void #11",
			`restore buffs ["haste"]`, "restore round 1", "void #10",
		}, calls, "each entry restored then voided, newest first; the popped one left alone")
		assert.Equal(t, int64(10), got.ID, "answers with the transaction's first entry")
		assert.Equal(t, actionlog.StatusVoided, got.Status())
		assert.Equal(t, 1, tx.calls, "check, restores and voids in one transaction")
	})

	t.Run("restorer fails mid-transaction: no void, no partial state", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		tx := &errTx{}
		svc, err := actionlogsvc.NewWithConfig(actionlogsvc.DefaultConfig(), dal, tx, mocks.NewMockRounds(t), mocks.NewMockMaxHP(t), nil)
		require.NoError(t, err)
		var calls []string
		register(svc, dal, &calls, "round")
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(11, true, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(11)).Return(&initiative, nil).Once()
		dal.EXPECT().ListTransaction(mock.Anything, int64(1), int64(10)).
			Return([]actionlog.Entry{initiative, advance}, nil).Once()

		_, err = svc.Undo(context.Background(), 1, 11)
		require.ErrorIs(t, err, errBoom)
		assert.Equal(t, []string{"restore initiative null", "void #11", `restore buffs ["haste"]`}, calls,
			"stops at the failing restore: #10 is never voided")
		require.ErrorIs(t, tx.bodyErr, errBoom, "the error leaves the transaction body, so the DB transaction rolls back #11's restore and void")
	})

	t.Run("stale expected top changes nothing", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		var calls []string
		register(svc, dal, &calls, "")
		// #10 is in the top's transaction but is not the top: refused too.
		for _, expected := range []int64{10, 9} {
			dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(11, true, nil).Once()
			dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(11)).Return(&initiative, nil).Once()
			_, err := svc.Undo(context.Background(), 1, expected)
			require.ErrorIs(t, err, actionlogsvc.ErrTopChanged)
		}
		assert.Empty(t, calls, "nothing restored, nothing voided")
	})
}

// ADR 0010 step 1: the top the client reads is the newest non-voided entry.
func TestTopID(t *testing.T) {
	svc, dal, _, _ := newSvc(t)
	dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(9, true, nil).Once()
	id, ok, err := svc.TopID(context.Background(), 1)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, int64(9), id)

	dal.EXPECT().LatestActiveID(mock.Anything, int64(2)).Return(0, false, errBoom).Once()
	_, _, err = svc.TopID(context.Background(), 2)
	require.ErrorIs(t, err, errBoom)
}
