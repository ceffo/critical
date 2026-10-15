package actionlogsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
)

// ADR 0010 steps 2 to 4: Undo pops the newest non-voided entry of any kind,
// only when it is the entry the client expects, in one transaction.
func TestUndo(t *testing.T) {
	attack := func() *actionlog.Entry {
		return &actionlog.Entry{
			ID: 9, CharacterID: 1, Round: 3, Kind: actionlog.KindAttack,
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
