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
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc/mocks"
)

// newScopedSvc is the log as the composition root wires it: its runner opens
// an action-log transaction per outermost InTx (Transactions).
func newScopedSvc(t *testing.T) (actionlogsvc.SVC, *mocks.MockDAL) {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	svc, err := actionlogsvc.NewWithConfig(actionlogsvc.DefaultConfig(), dal,
		actionlogsvc.Transactions(&passTx{}), mocks.NewMockRounds(t), mocks.NewMockMaxima(t))
	require.NoError(t, err)
	return svc, dal
}

func snap(key string) actionlog.Snapshot {
	return actionlog.Snapshot{Key: key, Before: json.RawMessage(`{"id":"x","state":1}`), After: json.RawMessage(`{"id":"x","state":2}`)}
}

// ADR 0018 step 11: a request answers with the state keys of the transaction
// it wrote: the log, HP when an entry moves it, and every snapshot's key.
func TestChanges_ARecordedTransactionNamesItsKeys(t *testing.T) {
	svc, dal := newScopedSvc(t)
	dal.EXPECT().HPTotal(mock.Anything, int64(1)).Return(-10, nil).Once()
	next := int64(10)
	dal.EXPECT().InsertEntry(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, actionlog.NewEntry, int64) (int64, error) { next++; return next, nil })
	ctx, changes := actionlog.WithChanges(context.Background())
	runner := actionlogsvc.Transactions(&passTx{})

	require.NoError(t, runner.InTx(ctx, func(ctx context.Context) error {
		if _, err := svc.Record(ctx, actionlog.NewEntry{
			CharacterID: 1, Round: 1, Kind: actionlog.KindCast,
			Payload: json.RawMessage(`{}`), Snapshots: []actionlog.Snapshot{snap("spell_slots"), snap("items:arrows")},
		}); err != nil {
			return err
		}
		_, err := svc.Record(ctx, actionlog.NewEntry{
			CharacterID: 1, Round: 1, Kind: actionlog.KindHPChange,
			Summary: "potion", HPDelta: 4, Snapshots: []actionlog.Snapshot{snap("items:arrows")},
		})
		return err
	}))
	assert.Equal(t, []string{"hp", "items:arrows", "log", "spell_slots"}, changes.Keys())

	t.Run("a roll that changes no state names the log only", func(t *testing.T) {
		ctx, changes := actionlog.WithChanges(context.Background())
		_, err := svc.Record(ctx, actionlog.NewEntry{CharacterID: 1, Round: 1, Kind: actionlog.KindSave, Payload: json.RawMessage(`{}`)})
		require.NoError(t, err)
		assert.Equal(t, []string{"log"}, changes.Keys())
	})

	t.Run("a transaction that rolls back names nothing", func(t *testing.T) {
		ctx, changes := actionlog.WithChanges(context.Background())
		err := runner.InTx(ctx, func(ctx context.Context) error {
			if _, err := svc.Record(ctx, actionlog.NewEntry{
				CharacterID: 1, Round: 1, Kind: actionlog.KindAttack,
				Payload: json.RawMessage(`{}`), Snapshots: []actionlog.Snapshot{snap("items:arrows")},
			}); err != nil {
				return err
			}
			return errBoom
		})
		require.ErrorIs(t, err, errBoom)
		assert.Empty(t, changes.Keys())
	})

	t.Run("a request without changes to fill records as before", func(t *testing.T) {
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Round: 1, Kind: actionlog.KindSave, Payload: json.RawMessage(`{}`)})
		require.NoError(t, err)
	})
}

// ADR 0018 step 11 with ADR 0018 step 2 and ADR 0019 step 1: an undo and a
// redo are two more actions; they name the keys of every entry they pop or
// replay, HP included when an entry moved it.
func TestChanges_UndoAndRedoNameTheKeysOfTheirTransaction(t *testing.T) {
	advance := actionlog.Entry{
		ID: 10, TransactionID: 10, CharacterID: 1, Kind: actionlog.KindRoundAdvance,
		Snapshots: []actionlog.Snapshot{snap("buffs"), snap("round")},
	}
	exposure := actionlog.Entry{
		ID: 11, TransactionID: 10, CharacterID: 1, Kind: actionlog.KindExposure,
		Payload: json.RawMessage(`{}`), HPDelta: -3,
	}
	voided := func(e actionlog.Entry) actionlog.Entry {
		e.Corrections = []actionlog.Correction{{ActionID: e.ID, Kind: actionlog.CorrectionVoid}}
		return e
	}
	register := func(svc actionlogsvc.SVC) {
		for _, family := range []string{"buffs", "round"} {
			registerCodec(t, svc, family, func(context.Context, int64, json.RawMessage) error { return nil })
		}
	}

	t.Run("undo", func(t *testing.T) {
		svc, dal := newScopedSvc(t)
		register(svc)
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(11, true, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(11)).Return(&exposure, nil).Once()
		dal.EXPECT().ListTransaction(mock.Anything, int64(1), int64(10)).Return([]actionlog.Entry{exposure, advance}, nil).Once()
		dal.EXPECT().InsertCorrection(mock.Anything, mock.Anything).Return(nil).Times(2)
		done := voided(advance)
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(10)).Return(&done, nil).Once()

		ctx, changes := actionlog.WithChanges(context.Background())
		_, err := svc.Undo(ctx, 1, 11)
		require.NoError(t, err)
		assert.Equal(t, []string{"buffs", "hp", "log", "round"}, changes.Keys())
	})

	t.Run("redo", func(t *testing.T) {
		svc, dal := newScopedSvc(t)
		register(svc)
		dal.EXPECT().LatestPopped(mock.Anything, int64(1)).Return(11, 10, true, nil).Once()
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(0, false, nil).Once()
		va, ve := voided(advance), voided(exposure)
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(10)).Return(&va, nil).Once()
		dal.EXPECT().ListTransaction(mock.Anything, int64(1), int64(10)).Return([]actionlog.Entry{ve, va}, nil).Once()
		dal.EXPECT().InsertCorrection(mock.Anything, mock.Anything).Return(nil).Times(2)
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(10)).Return(&advance, nil).Once()

		ctx, changes := actionlog.WithChanges(context.Background())
		_, err := svc.Redo(ctx, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"buffs", "hp", "log", "round"}, changes.Keys())
	})

	t.Run("a refused undo names nothing", func(t *testing.T) {
		svc, dal := newScopedSvc(t)
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(11, true, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(11)).Return(&exposure, nil).Once()

		ctx, changes := actionlog.WithChanges(context.Background())
		_, err := svc.Undo(ctx, 1, 9)
		require.ErrorIs(t, err, actionlogsvc.ErrTopChanged)
		assert.Empty(t, changes.Keys())
	})
}

// A note, or a correction after the roll, changes the journal only.
func TestChanges_AnAmendNamesTheLog(t *testing.T) {
	svc, dal := newScopedSvc(t)
	e := hpEntry(5)
	dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(e, nil).Times(2)
	dal.EXPECT().InsertCorrection(mock.Anything, mock.Anything).Return(nil).Once()

	ctx, changes := actionlog.WithChanges(context.Background())
	_, err := svc.Amend(ctx, 1, 5, actionlogsvc.Amendment{Note: "orc chief"})
	require.NoError(t, err)
	assert.Equal(t, []string{"log"}, changes.Keys())
}
