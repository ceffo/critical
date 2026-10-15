package actionlogdal_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/actionlogdal"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
)

// ADR 0019 step 1 on SQLite: an entry popped, redone and popped again reads
// voided, live, voided, and every storage query that skips popped entries
// (the top, the active entries of a kind, the HP total, a correction step's
// amend) follows its newest void or redo.
func TestRedo_NewestVoidOrRedoDecidesInStorage(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := actionlogdal.New(db)
	ctx := context.Background()

	older := insert(t, dal, hp(charID, -3, "rat"))
	hit := insert(t, dal, hp(charID, -5, "orc"))
	save := insert(t, dal, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindSave, Payload: json.RawMessage(`{"a":0}`)})
	step := insert(t, dal, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindCorrection, Payload: json.RawMessage(`{"entryId":3}`)})
	correct(t, dal, actionlog.Correction{ActionID: save, Kind: actionlog.CorrectionAmend, Payload: json.RawMessage(`{"a":1}`), StepID: &step})
	hpStep := insert(t, dal, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindCorrection, Payload: json.RawMessage(`{}`)})
	correct(t, dal, actionlog.Correction{ActionID: older, Kind: actionlog.CorrectionAmend, HPDelta: intp(-4), StepID: &hpStep})

	type view struct {
		Top       int64
		ActiveHP  int
		HP        int
		SavePay   string
		StepUndne bool
		Popped    int64
	}
	read := func() view {
		t.Helper()
		var v view
		top, ok, err := dal.LatestActiveID(ctx, charID)
		require.NoError(t, err)
		if ok {
			v.Top = top
		}
		active, err := dal.ListActiveEntries(ctx, charID, actionlog.KindHPChange, 10)
		require.NoError(t, err)
		v.ActiveHP = len(active)
		v.HP, err = dal.HPTotal(ctx, charID)
		require.NoError(t, err)
		e, err := dal.GetEntry(ctx, charID, save)
		require.NoError(t, err)
		v.SavePay = string(e.EffectivePayload())
		v.StepUndne = e.Corrections[0].StepUndone
		id, _, ok, err := dal.LatestPopped(ctx, charID)
		require.NoError(t, err)
		if ok {
			v.Popped = id
		}
		return v
	}
	all := view{Top: hpStep, ActiveHP: 2, HP: -9, SavePay: `{"a":1}`}
	assert.Equal(t, all, read())

	// Pop the HP step, the save's step and the hit (newest first, as undo does).
	for _, id := range []int64{hpStep, step, hit} {
		correct(t, dal, actionlog.Correction{ActionID: id, Kind: actionlog.CorrectionVoid})
	}
	popped := view{Top: save, ActiveHP: 1, HP: -3, SavePay: `{"a":0}`, StepUndne: true, Popped: hit}
	assert.Equal(t, popped, read())

	// Redo them, oldest pop first: the hit, then the steps.
	correct(t, dal, actionlog.Correction{ActionID: hit, Kind: actionlog.CorrectionRedo})
	assert.Equal(t, view{Top: save, ActiveHP: 2, HP: -8, SavePay: `{"a":0}`, StepUndne: true, Popped: step}, read(),
		"the hit counts again; the step popped before it is the newest standing pop")
	correct(t, dal, actionlog.Correction{ActionID: step, Kind: actionlog.CorrectionRedo})
	correct(t, dal, actionlog.Correction{ActionID: hpStep, Kind: actionlog.CorrectionRedo})
	assert.Equal(t, all, read(), "every redone entry reads as before its pop")

	// Popped again after the redo: a second void on the same entry.
	correct(t, dal, actionlog.Correction{ActionID: hpStep, Kind: actionlog.CorrectionVoid})
	e, err := dal.GetEntry(ctx, charID, hpStep)
	require.NoError(t, err)
	assert.Equal(t, actionlog.StatusVoided, e.Status())
	assert.Equal(t, []actionlog.CorrectionKind{actionlog.CorrectionVoid, actionlog.CorrectionRedo, actionlog.CorrectionVoid},
		[]actionlog.CorrectionKind{e.Corrections[0].Kind, e.Corrections[1].Kind, e.Corrections[2].Kind}, "append-only: every pop and redo is kept")
	total, err := dal.HPTotal(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, -8, total, "the hp step's amend is out of effect again")
	id, txID, ok, err := dal.LatestPopped(ctx, charID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, []int64{hpStep, hpStep}, []int64{id, txID})
}

// ADR 0019 step 3 on SQLite: a redo whose later restore fails rolls back
// the replays and redo corrections already written in the same transaction.
func TestRedo_RestorerFailsMidReplay_RollsBackEverything(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	log := newLog(t, db, actionlogdal.New(db))
	rs := roundState{db: db}
	log.RegisterRestorer("round", rs)
	log.RegisterRestorer("fragile", roundState{db: db})
	ctx := context.Background()
	tx := actionlogsvc.Transactions(db)

	record := func(ctx context.Context, key string, from, to int) (int64, error) {
		return log.Record(ctx, actionlog.NewEntry{
			CharacterID: charID, Kind: actionlog.KindRoundAdvance, Payload: json.RawMessage(`{}`),
			Snapshots: []actionlog.Snapshot{{Key: key, Before: json.RawMessage(itoa(from)), After: json.RawMessage(itoa(to))}},
		})
	}
	var first, newer int64
	require.NoError(t, tx.InTx(ctx, func(ctx context.Context) error {
		if err := rs.set(ctx, charID, 1); err != nil {
			return err
		}
		var err error
		if first, err = record(ctx, "round", 0, 1); err != nil {
			return err
		}
		newer, err = record(ctx, "fragile", 1, 1)
		return err
	}))
	_, err := log.Undo(ctx, charID, newer)
	require.NoError(t, err)
	require.Equal(t, 0, round(t, db, charID))
	popped, err := log.List(ctx, charID, 0)
	require.NoError(t, err)

	log.RegisterRestorer("fragile", failingRestorer{})
	_, err = log.Redo(ctx, charID, first)
	require.ErrorIs(t, err, errBoom)
	assert.Equal(t, 0, round(t, db, charID), "the older entry's replay is rolled back")
	after, err := log.List(ctx, charID, 0)
	require.NoError(t, err)
	assert.Equal(t, popped, after, "no redo written")

	log.RegisterRestorer("fragile", roundState{db: db})
	redone, err := log.Redo(ctx, charID, first)
	require.NoError(t, err)
	assert.Equal(t, first, redone.ID, "answers with the transaction's first entry")
	assert.Equal(t, actionlog.StatusActive, redone.Status())
	assert.Equal(t, 1, round(t, db, charID))
	top, ok, err := log.TopID(ctx, charID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, newer, top, "both entries live again: the transaction is on top")
	target, err := log.RedoTarget(ctx, charID)
	require.NoError(t, err)
	assert.Nil(t, target, "nothing left to redo")
}
