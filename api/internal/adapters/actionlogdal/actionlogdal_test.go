// The adapter runs on a mocked Querier: the tests assert the operations
// called, their arguments and order, and how rows and errors map to entries;
// no database, no SQL text (ADR 0025 step 6.2). The mock fails on any call a
// test does not expect, so a write test also proves the log is append-only:
// no other statement than the inserts it names. What only the SQL decides
// (the newest-first order, the rollback of a transaction) is checked by sqlc
// at generate time and by running the app (step 7); the services' flows on
// mocked DALs. Which entries are popped and the HP sum are the domain's rules
// (actionlog.StandingVoids, actionlog.HPTotal), applied here.
package actionlogdal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/actionlogdal"
	"github.com/ceffo/critical/api/internal/adapters/actionlogdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/actionlogdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

const charID = int64(3)

var (
	errBoom = errors.New("boom")
	at      = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
)

func mocked(t *testing.T) (*actionlogdal.DAL, *mocks.MockQuerier) {
	t.Helper()
	q := mocks.NewMockQuerier(t)
	return actionlogdal.New(q), q
}

func intp(v int) *int            { return &v }
func strp(v string) *string      { return &v }
func i64p(v int64) *int64        { return &v }
func some(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }
func text(v string) sql.NullString {
	return sql.NullString{String: v, Valid: true}
}

// row is an action_log row of the character, in its own transaction.
func row(id int64, kind actionlog.Kind, delta int64, payload string) queries.GetActionRow {
	return queries.GetActionRow{
		ID: id, CharacterID: charID, Round: 2, Kind: string(kind), Summary: "s", HpDelta: delta,
		Payload: payload, TransactionID: some(id), CreatedAt: at,
	}
}

// expectAttach expects the snapshots and corrections of the entries from
// minID on.
func expectAttach(q *mocks.MockQuerier, minID int64, snaps []queries.ListSnapshotsFromRow, corrs []queries.ListCorrectionsFromRow) {
	q.EXPECT().ListSnapshotsFrom(mock.Anything, queries.ListSnapshotsFromParams{CharacterID: charID, ActionID: minID}).Return(snaps, nil).Once()
	q.EXPECT().ListCorrectionsFrom(mock.Anything, queries.ListCorrectionsFromParams{CharacterID: charID, ActionID: minID}).Return(corrs, nil).Once()
}

func void(id, actionID int64) queries.ListCorrectionsFromRow {
	return queries.ListCorrectionsFromRow{ID: id, ActionID: actionID, Kind: "void", CreatedAt: at}
}

func redo(id, actionID int64) queries.ListCorrectionsFromRow {
	return queries.ListCorrectionsFromRow{ID: id, ActionID: actionID, Kind: "redo", CreatedAt: at}
}

// ADR 0018 step 1: the first entry of a transaction starts it under its own
// id: it is inserted without one, then given its id, then its snapshots are
// inserted in order. An empty payload is stored as {}.
func TestInsertEntry_NewTransactionStartsUnderTheEntrysID(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	mock.InOrder(
		q.EXPECT().InsertAction(ctx, queries.InsertActionParams{
			CharacterID: charID, Round: 4, Kind: "round_advance", Summary: "next round", HpDelta: -2, Payload: "{}",
		}).Return(int64(10), nil).Once(),
		q.EXPECT().StartActionTransaction(ctx, int64(10)).Return(nil).Once(),
		q.EXPECT().InsertSnapshot(ctx, queries.InsertSnapshotParams{ActionID: 10, StateKey: "buffs", BeforeEnvelope: `["haste"]`, AfterEnvelope: `[]`}).Return(nil).Once(),
		q.EXPECT().InsertSnapshot(ctx, queries.InsertSnapshotParams{ActionID: 10, StateKey: "round", BeforeEnvelope: `1`, AfterEnvelope: `2`}).Return(nil).Once(),
	)
	id, err := dal.InsertEntry(ctx, actionlog.NewEntry{
		CharacterID: charID, Round: 4, Kind: actionlog.KindRoundAdvance, Summary: "next round", HPDelta: -2,
		Snapshots: []actionlog.Snapshot{
			{Key: "buffs", Before: json.RawMessage(`["haste"]`), After: json.RawMessage(`[]`)},
			{Key: "round", Before: json.RawMessage(`1`), After: json.RawMessage(`2`)},
		},
	}, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(10), id)
}

// ADR 0018 step 1: a later entry joins the given transaction; nothing else
// is written.
func TestInsertEntry_JoinsTheGivenTransaction(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	q.EXPECT().InsertAction(ctx, queries.InsertActionParams{
		CharacterID: charID, Kind: "initiative", Summary: "", Payload: `{"d10":4}`, TransactionID: some(10),
	}).Return(int64(11), nil).Once()
	id, err := dal.InsertEntry(ctx, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindInitiative, Payload: json.RawMessage(`{"d10":4}`)}, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(11), id)
}

// A failing write stops the insert: nothing after it is written and the
// error is returned, for the caller's transaction to roll back.
func TestInsertEntry_StopsAtTheFailingWrite(t *testing.T) {
	ctx := context.Background()
	entry := actionlog.NewEntry{
		CharacterID: charID, Kind: actionlog.KindHPChange, Summary: "orc", HPDelta: -8,
		Snapshots: []actionlog.Snapshot{{Key: "round", Before: json.RawMessage(`1`), After: json.RawMessage(`2`)}},
	}
	params := queries.InsertActionParams{CharacterID: charID, Kind: "hp_change", Summary: "orc", HpDelta: -8, Payload: "{}"}

	t.Run("insert", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().InsertAction(ctx, params).Return(0, errBoom).Once()
		_, err := dal.InsertEntry(ctx, entry, 0)
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("start transaction", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().InsertAction(ctx, params).Return(5, nil).Once()
		q.EXPECT().StartActionTransaction(ctx, int64(5)).Return(errBoom).Once()
		_, err := dal.InsertEntry(ctx, entry, 0)
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("snapshot", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().InsertAction(ctx, params).Return(5, nil).Once()
		q.EXPECT().StartActionTransaction(ctx, int64(5)).Return(nil).Once()
		q.EXPECT().InsertSnapshot(ctx, mock.Anything).Return(errBoom).Once()
		_, err := dal.InsertEntry(ctx, entry, 0)
		require.ErrorIs(t, err, errBoom)
	})
}

// A correction is one inserted row; a field the correction leaves out is
// stored NULL (an empty payload leaves the entry's payload in effect).
func TestInsertCorrection_StoresTheFieldsItSetsAndNullForTheRest(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		in   actionlog.Correction
		want queries.InsertCorrectionParams
	}{
		{
			"void",
			actionlog.Correction{ActionID: 7, Kind: actionlog.CorrectionVoid},
			queries.InsertCorrectionParams{ActionID: 7, Kind: "void"},
		},
		{
			"redo",
			actionlog.Correction{ActionID: 7, Kind: actionlog.CorrectionRedo},
			queries.InsertCorrectionParams{ActionID: 7, Kind: "redo"},
		},
		{
			"value amend",
			actionlog.Correction{ActionID: 7, Kind: actionlog.CorrectionAmend, HPDelta: intp(-6), Summary: "orc (fixed)"},
			queries.InsertCorrectionParams{ActionID: 7, Kind: "amend", HpDelta: some(-6), Summary: text("orc (fixed)")},
		},
		{
			"amend to zero keeps the zero",
			actionlog.Correction{ActionID: 7, Kind: actionlog.CorrectionAmend, HPDelta: intp(0)},
			queries.InsertCorrectionParams{ActionID: 7, Kind: "amend", HpDelta: some(0)},
		},
		{
			"note amend",
			actionlog.Correction{ActionID: 7, Kind: actionlog.CorrectionAmend, Note: strp("DM said 11")},
			queries.InsertCorrectionParams{ActionID: 7, Kind: "amend", Note: text("DM said 11")},
		},
		{
			"empty note clears",
			actionlog.Correction{ActionID: 7, Kind: actionlog.CorrectionAmend, Note: strp("")},
			queries.InsertCorrectionParams{ActionID: 7, Kind: "amend", Note: text("")},
		},
		{
			"payload amend of a correction step",
			actionlog.Correction{
				ActionID: 7, Kind: actionlog.CorrectionAmend, Summary: "save adjusted",
				Payload: json.RawMessage(`{"passed":true}`), StepID: i64p(9),
			},
			queries.InsertCorrectionParams{ActionID: 7, Kind: "amend", Summary: text("save adjusted"), Payload: text(`{"passed":true}`), StepID: some(9)},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dal, q := mocked(t)
			q.EXPECT().InsertCorrection(ctx, c.want).Return(nil).Once()
			require.NoError(t, dal.InsertCorrection(ctx, c.in))
		})
	}
	t.Run("error", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().InsertCorrection(ctx, mock.Anything).Return(errBoom).Once()
		require.ErrorIs(t, dal.InsertCorrection(ctx, actionlog.Correction{ActionID: 7, Kind: actionlog.CorrectionVoid}), errBoom)
	})
}

// ADR 0018 step 6: a pop keeps what a replay needs. Voiding an entry inserts
// one correction row and touches nothing else (the mock refuses any other
// statement); the entry then reads back with its kind, payload and
// snapshots as they were, voided by that row.
func TestPop_KeepsEntriesAndSnapshots(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	q.EXPECT().InsertCorrection(ctx, queries.InsertCorrectionParams{ActionID: 10, Kind: "void"}).Return(nil).Once()
	require.NoError(t, dal.InsertCorrection(ctx, actionlog.Correction{ActionID: 10, Kind: actionlog.CorrectionVoid}))

	q.EXPECT().GetAction(ctx, queries.GetActionParams{CharacterID: charID, ID: 10}).
		Return(row(10, actionlog.KindRoundAdvance, 0, `{"from":0,"to":1}`), nil).Once()
	expectAttach(q, 10, []queries.ListSnapshotsFromRow{{ID: 1, ActionID: 10, StateKey: "round", BeforeEnvelope: `0`, AfterEnvelope: `1`}},
		[]queries.ListCorrectionsFromRow{void(5, 10)})
	e, err := dal.GetEntry(ctx, charID, 10)
	require.NoError(t, err)
	assert.Equal(t, actionlog.StatusVoided, e.Status())
	assert.Equal(t, actionlog.KindRoundAdvance, e.Kind)
	assert.JSONEq(t, `{"from":0,"to":1}`, string(e.Payload))
	assert.JSONEq(t, `{"from":0,"to":1}`, string(e.EffectivePayload()), "the void rewrites no payload")
	assert.Equal(t, []actionlog.Snapshot{{Key: "round", Before: json.RawMessage(`0`), After: json.RawMessage(`1`)}}, e.Snapshots)
}

// GetEntry maps the row, its snapshots and corrections (read from the
// entry's id on); no row is domain.ErrNotFound.
func TestGetEntry_WithSnapshotsAndCorrections(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	q.EXPECT().GetAction(ctx, queries.GetActionParams{CharacterID: charID, ID: 4}).
		Return(queries.GetActionRow{
			ID: 4, CharacterID: charID, Round: 3, Kind: "buff_change", Summary: "bless",
			Payload: `{"buff":"bless"}`, TransactionID: some(2), CreatedAt: at,
		}, nil).Once()
	expectAttach(q, 4, []queries.ListSnapshotsFromRow{
		{ID: 1, ActionID: 4, StateKey: "buffs", BeforeEnvelope: `[]`, AfterEnvelope: `[{"id":"b"}]`},
		{ID: 2, ActionID: 4, StateKey: "round", BeforeEnvelope: `2`, AfterEnvelope: `3`},
		{ID: 3, ActionID: 6, StateKey: "round", BeforeEnvelope: `3`, AfterEnvelope: `4`},
	}, []queries.ListCorrectionsFromRow{
		{ID: 8, ActionID: 4, Kind: "amend", HpDelta: some(-6), Summary: text("bless (fixed)"), Note: text("n"), CreatedAt: at},
		void(9, 6),
	})

	e, err := dal.GetEntry(ctx, charID, 4)
	require.NoError(t, err)
	assert.Equal(t, &actionlog.Entry{
		ID: 4, TransactionID: 2, CharacterID: charID, Round: 3, Kind: actionlog.KindBuffChange, Summary: "bless",
		Payload: json.RawMessage(`{"buff":"bless"}`), CreatedAt: at,
		Snapshots: []actionlog.Snapshot{
			{Key: "buffs", Before: json.RawMessage(`[]`), After: json.RawMessage(`[{"id":"b"}]`)},
			{Key: "round", Before: json.RawMessage(`2`), After: json.RawMessage(`3`)},
		},
		Corrections: []actionlog.Correction{{
			ID: 8, ActionID: 4, Kind: actionlog.CorrectionAmend, HPDelta: intp(-6),
			Summary: "bless (fixed)", Note: strp("n"), CreatedAt: at,
		}},
	}, e, "rows of a newer entry are not attached")

	q.EXPECT().GetAction(ctx, queries.GetActionParams{CharacterID: charID, ID: 999}).Return(queries.GetActionRow{}, sql.ErrNoRows).Once()
	_, err = dal.GetEntry(ctx, charID, 999)
	require.ErrorIs(t, err, domain.ErrNotFound)

	q.EXPECT().GetAction(ctx, queries.GetActionParams{CharacterID: charID, ID: 5}).Return(queries.GetActionRow{}, errBoom).Once()
	_, err = dal.GetEntry(ctx, charID, 5)
	require.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, domain.ErrNotFound)
}

// The lists keep the query's order (newest first), pass the limit, kind and
// transaction on, and read the snapshots and corrections once, from the
// smallest id listed. A row without a transaction id is its own transaction.
func TestListEntries(t *testing.T) {
	ctx := context.Background()
	older := row(2, actionlog.KindHPChange, -8, "{}")
	older.TransactionID = sql.NullInt64{}
	newer := row(5, actionlog.KindHPChange, 3, "{}")
	corrs := []queries.ListCorrectionsFromRow{{ID: 7, ActionID: 2, Kind: "amend", HpDelta: some(-6), CreatedAt: at}, void(8, 5)}

	t.Run("all", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().ListActions(ctx, queries.ListActionsParams{CharacterID: charID, Limit: 10}).
			Return([]queries.ListActionsRow{queries.ListActionsRow(newer), queries.ListActionsRow(older)}, nil).Once()
		expectAttach(q, 2, nil, corrs)
		got, err := dal.ListEntries(ctx, charID, 10)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, []int64{5, 2}, []int64{got[0].ID, got[1].ID})
		assert.Equal(t, []int64{5, 2}, []int64{got[0].TransactionID, got[1].TransactionID})
		assert.Equal(t, actionlog.StatusVoided, got[0].Status())
		assert.Equal(t, actionlog.StatusAmended, got[1].Status())
		assert.Equal(t, -8, got[1].HPDelta, "the original stays visible")
		assert.Equal(t, -6, got[1].EffectiveHPDelta())
	})
	t.Run("one transaction", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().ListTransactionActions(ctx, queries.ListTransactionActionsParams{CharacterID: charID, TransactionID: some(2)}).
			Return([]queries.ListTransactionActionsRow{queries.ListTransactionActionsRow(newer), queries.ListTransactionActionsRow(older)}, nil).Once()
		expectAttach(q, 2, nil, nil)
		got, err := dal.ListTransaction(ctx, charID, 2)
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})
	t.Run("empty reads nothing more", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().ListActions(ctx, queries.ListActionsParams{CharacterID: charID, Limit: 10}).Return([]queries.ListActionsRow{}, nil).Once()
		got, err := dal.ListEntries(ctx, charID, 10)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
	t.Run("errors", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().ListActions(ctx, mock.Anything).Return(nil, errBoom).Once()
		_, err := dal.ListEntries(ctx, charID, 10)
		require.ErrorIs(t, err, errBoom)

		q.EXPECT().ListTransactionActions(ctx, mock.Anything).Return(nil, errBoom).Once()
		_, err = dal.ListTransaction(ctx, charID, 2)
		require.ErrorIs(t, err, errBoom)

		q.EXPECT().ListActions(ctx, mock.Anything).Return([]queries.ListActionsRow{queries.ListActionsRow(older)}, nil).Once()
		q.EXPECT().ListSnapshotsFrom(ctx, mock.Anything).Return(nil, errBoom).Once()
		_, err = dal.ListEntries(ctx, charID, 10)
		require.ErrorIs(t, err, errBoom)

		q.EXPECT().ListActions(ctx, mock.Anything).Return([]queries.ListActionsRow{queries.ListActionsRow(older)}, nil).Once()
		q.EXPECT().ListSnapshotsFrom(ctx, mock.Anything).Return(nil, nil).Once()
		q.EXPECT().ListCorrectionsFrom(ctx, mock.Anything).Return(nil, errBoom).Once()
		_, err = dal.ListEntries(ctx, charID, 10)
		require.ErrorIs(t, err, errBoom)
	})
}

func pop(id, actionID int64, kind string) queries.ListPopCorrectionsRow {
	return queries.ListPopCorrectionsRow{ID: id, ActionID: actionID, Kind: kind}
}

// expectPops expects the read of the character's voids and redos.
func expectPops(q *mocks.MockQuerier, pops ...queries.ListPopCorrectionsRow) {
	q.EXPECT().ListPopCorrections(mock.Anything, charID).Return(pops, nil).Once()
}

// ADR 0019 step 1 for the active entries of a kind: an entry whose newest
// void or redo is a void is skipped, a redone one is kept. At most one row
// per popped entry is skipped, so the adapter asks for limit plus that many
// and keeps limit.
func TestListActiveEntries_SkipsThePoppedEntries(t *testing.T) {
	ctx := context.Background()
	kindRow := func(id int64) queries.ListActionsByKindRow {
		return queries.ListActionsByKindRow(row(id, actionlog.KindHPChange, -1, "{}"))
	}
	ids := func(es []actionlog.Entry) []int64 {
		out := []int64{}
		for _, e := range es {
			out = append(out, e.ID)
		}
		return out
	}

	t.Run("popped skipped, redone kept", func(t *testing.T) {
		dal, q := mocked(t)
		expectPops(q, pop(8, 5, "void"), pop(9, 6, "void"), pop(10, 6, "redo"))
		q.EXPECT().ListActionsByKind(ctx, queries.ListActionsByKindParams{CharacterID: charID, Kind: "hp_change", Limit: 3}).
			Return([]queries.ListActionsByKindRow{kindRow(6), kindRow(5), kindRow(2)}, nil).Once()
		expectAttach(q, 2, nil, nil)
		got, err := dal.ListActiveEntries(ctx, charID, actionlog.KindHPChange, 2)
		require.NoError(t, err)
		assert.Equal(t, []int64{6, 2}, ids(got))
	})
	t.Run("nothing popped: limit rows, limit kept", func(t *testing.T) {
		dal, q := mocked(t)
		expectPops(q)
		q.EXPECT().ListActionsByKind(ctx, queries.ListActionsByKindParams{CharacterID: charID, Kind: "hp_change", Limit: 2}).
			Return([]queries.ListActionsByKindRow{kindRow(6), kindRow(5)}, nil).Once()
		expectAttach(q, 5, nil, nil)
		got, err := dal.ListActiveEntries(ctx, charID, actionlog.KindHPChange, 2)
		require.NoError(t, err)
		assert.Equal(t, []int64{6, 5}, ids(got))
	})
	t.Run("popped entry of another kind: one row too many read, limit kept", func(t *testing.T) {
		dal, q := mocked(t)
		expectPops(q, pop(8, 7, "void"))
		q.EXPECT().ListActionsByKind(ctx, queries.ListActionsByKindParams{CharacterID: charID, Kind: "hp_change", Limit: 2}).
			Return([]queries.ListActionsByKindRow{kindRow(6), kindRow(5)}, nil).Once()
		expectAttach(q, 6, nil, nil)
		got, err := dal.ListActiveEntries(ctx, charID, actionlog.KindHPChange, 1)
		require.NoError(t, err)
		assert.Equal(t, []int64{6}, ids(got))
	})
	t.Run("errors", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().ListPopCorrections(ctx, charID).Return(nil, errBoom).Once()
		_, err := dal.ListActiveEntries(ctx, charID, actionlog.KindHPChange, 1)
		require.ErrorIs(t, err, errBoom)

		expectPops(q)
		q.EXPECT().ListActionsByKind(ctx, mock.Anything).Return(nil, errBoom).Once()
		_, err = dal.ListActiveEntries(ctx, charID, actionlog.KindHPChange, 1)
		require.ErrorIs(t, err, errBoom)
	})
}

// ADR 0018 step 3, ADR 0019 step 1: the top of the stack is the newest entry
// not popped, any kind; a redone entry is on the stack again. Among the
// newest (popped + 1) ids one is not popped when the log has one.
func TestLatestActiveID_NewestEntryNotPopped(t *testing.T) {
	ctx := context.Background()
	idsParams := func(limit int64) queries.ListActionIDsParams {
		return queries.ListActionIDsParams{CharacterID: charID, Limit: limit}
	}
	cases := []struct {
		name  string
		pops  []queries.ListPopCorrectionsRow
		limit int64
		ids   []int64
		want  int64
		found bool
	}{
		{"empty log", nil, 1, []int64{}, 0, false},
		{"top not popped", nil, 1, []int64{12}, 12, true},
		{"popped top skipped", []queries.ListPopCorrectionsRow{pop(20, 12, "void"), pop(21, 11, "void")}, 3, []int64{12, 11, 9}, 9, true},
		{"redone top is the top again", []queries.ListPopCorrectionsRow{pop(20, 12, "void"), pop(21, 12, "redo")}, 1, []int64{12}, 12, true},
		{"every entry popped", []queries.ListPopCorrectionsRow{pop(20, 12, "void"), pop(21, 11, "void")}, 3, []int64{12, 11}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dal, q := mocked(t)
			expectPops(q, c.pops...)
			q.EXPECT().ListActionIDs(ctx, idsParams(c.limit)).Return(c.ids, nil).Once()
			id, ok, err := dal.LatestActiveID(ctx, charID)
			require.NoError(t, err)
			assert.Equal(t, []any{c.want, c.found}, []any{id, ok})
		})
	}
	t.Run("errors", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().ListPopCorrections(ctx, charID).Return(nil, errBoom).Once()
		_, _, err := dal.LatestActiveID(ctx, charID)
		require.ErrorIs(t, err, errBoom)

		expectPops(q)
		q.EXPECT().ListActionIDs(ctx, idsParams(1)).Return(nil, errBoom).Once()
		_, _, err = dal.LatestActiveID(ctx, charID)
		require.ErrorIs(t, err, errBoom)
	})
}

// ADR 0019 step 2: the redo target is the entry whose standing void is the
// newest, read back for its transaction; an entry without a transaction id
// is its own transaction; nothing popped reads nothing more.
func TestLatestPopped_EntryOfTheNewestStandingVoid(t *testing.T) {
	ctx := context.Background()

	t.Run("newest standing void", func(t *testing.T) {
		dal, q := mocked(t)
		expectPops(q, pop(15, 9, "void"), pop(20, 7, "void"), pop(21, 8, "void"), pop(22, 8, "redo"))
		r := row(7, actionlog.KindHPChange, 0, "{}")
		r.TransactionID = some(6)
		q.EXPECT().GetAction(ctx, queries.GetActionParams{CharacterID: charID, ID: 7}).Return(r, nil).Once()
		id, tx, ok, err := dal.LatestPopped(ctx, charID)
		require.NoError(t, err)
		assert.Equal(t, []any{int64(7), int64(6), true}, []any{id, tx, ok})
	})
	t.Run("own transaction", func(t *testing.T) {
		dal, q := mocked(t)
		expectPops(q, pop(15, 4, "void"))
		r := row(4, actionlog.KindHPChange, 0, "{}")
		r.TransactionID = sql.NullInt64{}
		q.EXPECT().GetAction(ctx, queries.GetActionParams{CharacterID: charID, ID: 4}).Return(r, nil).Once()
		id, tx, ok, err := dal.LatestPopped(ctx, charID)
		require.NoError(t, err)
		assert.Equal(t, []any{int64(4), int64(4), true}, []any{id, tx, ok})
	})
	t.Run("nothing popped", func(t *testing.T) {
		dal, q := mocked(t)
		expectPops(q, pop(15, 4, "void"), pop(16, 4, "redo"))
		_, _, ok, err := dal.LatestPopped(ctx, charID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("errors", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().ListPopCorrections(ctx, charID).Return(nil, errBoom).Once()
		_, _, _, err := dal.LatestPopped(ctx, charID)
		require.ErrorIs(t, err, errBoom)

		expectPops(q, pop(15, 4, "void"))
		q.EXPECT().GetAction(ctx, mock.Anything).Return(queries.GetActionRow{}, errBoom).Once()
		_, _, _, err = dal.LatestPopped(ctx, charID)
		require.ErrorIs(t, err, errBoom)
	})
}

// LatestID asks for the newest entry of the kind, voided or not, so an older
// save never becomes the latest by voiding a newer one.
func TestLatestID(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	params := queries.LatestActionIDParams{CharacterID: charID, Kind: "save"}

	q.EXPECT().LatestActionID(ctx, params).Return(int64(8), nil).Once()
	id, ok, err := dal.LatestID(ctx, charID, actionlog.KindSave)
	require.NoError(t, err)
	assert.Equal(t, []any{int64(8), true}, []any{id, ok})
	q.EXPECT().LatestActionID(ctx, params).Return(0, sql.ErrNoRows).Once()
	_, ok, err = dal.LatestID(ctx, charID, actionlog.KindSave)
	require.NoError(t, err)
	assert.False(t, ok)
	q.EXPECT().LatestActionID(ctx, params).Return(0, errBoom).Once()
	_, _, err = dal.LatestID(ctx, charID, actionlog.KindSave)
	require.ErrorIs(t, err, errBoom)
}

// ADR 0007 step 2: HPTotal reads every entry's own delta and the
// corrections from the oldest entry on, and sums the effective deltas by the
// domain rule (actionlog.HPTotal, whose cases are tested in the domain). The
// flows here follow the log as it grows: damage, amend the delta, pop, redo,
// pop again; a note amend; an amend whose correction step is popped then
// redone. An empty log reads no corrections.
func TestHPTotal(t *testing.T) {
	ctx := context.Background()
	deltas := []queries.ListHPDeltasRow{{ID: 2, HpDelta: -8}, {ID: 4, HpDelta: 3}, {ID: 5, HpDelta: 0}}
	amend := func(id, actionID, d int64) queries.ListCorrectionsFromRow {
		return queries.ListCorrectionsFromRow{ID: id, ActionID: actionID, Kind: "amend", HpDelta: some(d), CreatedAt: at}
	}
	stepAmend := func(id, actionID, d, step int64) queries.ListCorrectionsFromRow {
		c := amend(id, actionID, d)
		c.StepID = some(step)
		return c
	}
	noteAmend := queries.ListCorrectionsFromRow{ID: 10, ActionID: 2, Kind: "amend", Note: text("DM ruling"), CreatedAt: at}
	cases := []struct {
		name  string
		corrs []queries.ListCorrectionsFromRow
		want  int
	}{
		{"damage and heal", nil, -5},
		{"delta amended", []queries.ListCorrectionsFromRow{amend(10, 2, -12)}, -9},
		{"amended entry popped", []queries.ListCorrectionsFromRow{amend(10, 2, -12), void(11, 2)}, 3},
		{"popped entry redone, with its amend", []queries.ListCorrectionsFromRow{amend(10, 2, -12), void(11, 2), redo(12, 2)}, -9},
		{"popped again", []queries.ListCorrectionsFromRow{amend(10, 2, -12), void(11, 2), redo(12, 2), void(13, 2)}, 3},
		{"note amend keeps the delta", []queries.ListCorrectionsFromRow{noteAmend}, -5},
		{"amend of a live correction step", []queries.ListCorrectionsFromRow{stepAmend(10, 2, -1, 5)}, 2},
		{"its correction step popped", []queries.ListCorrectionsFromRow{stepAmend(10, 2, -1, 5), void(11, 5)}, -5},
		{"its correction step redone", []queries.ListCorrectionsFromRow{stepAmend(10, 2, -1, 5), void(11, 5), redo(12, 5)}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dal, q := mocked(t)
			q.EXPECT().ListHPDeltas(ctx, charID).Return(deltas, nil).Once()
			q.EXPECT().ListCorrectionsFrom(ctx, queries.ListCorrectionsFromParams{CharacterID: charID, ActionID: 2}).Return(c.corrs, nil).Once()
			total, err := dal.HPTotal(ctx, charID)
			require.NoError(t, err)
			assert.Equal(t, c.want, total)
		})
	}
	t.Run("empty log reads no corrections", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().ListHPDeltas(ctx, charID).Return(nil, nil).Once()
		total, err := dal.HPTotal(ctx, charID)
		require.NoError(t, err)
		assert.Zero(t, total)
	})
	t.Run("errors", func(t *testing.T) {
		dal, q := mocked(t)
		q.EXPECT().ListHPDeltas(ctx, charID).Return(nil, errBoom).Once()
		_, err := dal.HPTotal(ctx, charID)
		require.ErrorIs(t, err, errBoom)

		q.EXPECT().ListHPDeltas(ctx, charID).Return(deltas, nil).Once()
		q.EXPECT().ListCorrectionsFrom(ctx, mock.Anything).Return(nil, errBoom).Once()
		_, err = dal.HPTotal(ctx, charID)
		require.ErrorIs(t, err, errBoom)
	})
}

// ADR 0007 step 7: a value amend stored before migration 014 (no note) still
// reads as the amended delta and summary, with an empty note; a note amend
// read after it sets the note and leaves the value amend in effect.
func TestLegacyValueAmend_ReadsAfterMigration014(t *testing.T) {
	ctx := context.Background()
	legacy := queries.ListCorrectionsFromRow{ID: 7, ActionID: 2, Kind: "amend", HpDelta: some(-12), Summary: text("orc axe"), CreatedAt: at}
	read := func(t *testing.T, corrs ...queries.ListCorrectionsFromRow) *actionlog.Entry {
		t.Helper()
		dal, q := mocked(t)
		q.EXPECT().GetAction(ctx, queries.GetActionParams{CharacterID: charID, ID: 2}).Return(row(2, actionlog.KindHPChange, -8, "{}"), nil).Once()
		expectAttach(q, 2, nil, corrs)
		e, err := dal.GetEntry(ctx, charID, 2)
		require.NoError(t, err)
		return e
	}

	e := read(t, legacy)
	assert.Equal(t, actionlog.StatusAmended, e.Status())
	assert.Equal(t, -8, e.HPDelta)
	assert.Equal(t, -12, e.EffectiveHPDelta())
	assert.Equal(t, "orc axe", e.EffectiveSummary())
	assert.Empty(t, e.Note())
	assert.Nil(t, e.Corrections[0].Note)
	assert.Empty(t, e.Corrections[0].Payload, "a NULL payload reads empty")

	e = read(t, legacy, queries.ListCorrectionsFromRow{ID: 9, ActionID: 2, Kind: "amend", Note: text("DM: the axe crit"), CreatedAt: at})
	assert.Equal(t, "DM: the axe crit", e.Note())
	assert.Equal(t, -12, e.EffectiveHPDelta(), "the note does not hide the value amend")
	assert.Nil(t, e.Corrections[1].HPDelta)
}

// ADR 0010 step 6: an amend linked to a correction step reads its StepID,
// and StepUndone once the step is voided (popped), even when the step's own
// entry is not among those read; the entry then reads the payload before it,
// while an amend without a step stays in effect.
func TestCorrectionStep_PoppedStepPutsItsAmendOutOfEffect(t *testing.T) {
	ctx := context.Background()
	amends := []queries.ListCorrectionsFromRow{
		{ID: 7, ActionID: 2, Kind: "amend", Payload: text(`{"a":1}`), CreatedAt: at},
		{ID: 8, ActionID: 2, Kind: "amend", Payload: text(`{"a":2}`), StepID: some(3), CreatedAt: at},
	}
	read := func(t *testing.T, corrs []queries.ListCorrectionsFromRow) *actionlog.Entry {
		t.Helper()
		dal, q := mocked(t)
		q.EXPECT().GetAction(ctx, queries.GetActionParams{CharacterID: charID, ID: 2}).Return(row(2, actionlog.KindSave, 0, `{"a":0}`), nil).Once()
		expectAttach(q, 2, nil, corrs)
		e, err := dal.GetEntry(ctx, charID, 2)
		require.NoError(t, err)
		return e
	}

	e := read(t, amends)
	require.Len(t, e.Corrections, 2)
	assert.Nil(t, e.Corrections[0].StepID, "an amend written before steps existed")
	assert.Equal(t, i64p(3), e.Corrections[1].StepID)
	assert.False(t, e.Corrections[1].StepUndone)
	assert.JSONEq(t, `{"a":2}`, string(e.EffectivePayload()))

	e = read(t, append(amends, void(9, 3)))
	require.Len(t, e.Corrections, 2, "the step's void is the step's, not the entry's")
	assert.True(t, e.Corrections[1].StepUndone)
	assert.JSONEq(t, `{"a":1}`, string(e.EffectivePayload()), "the amend without a step is in effect again")
	assert.Equal(t, actionlog.StatusAmended, e.Status())
}

// ADR 0019 step 1 in the adapter: the corrections are read in id order, and
// the newest void or redo of a correction step decides whether its amend is
// in effect: popped, redone, popped again; every pop and redo of an entry is
// kept and read back.
func TestRedo_NewestVoidOrRedoDecidesInStorage(t *testing.T) {
	ctx := context.Background()
	amend := queries.ListCorrectionsFromRow{ID: 7, ActionID: 2, Kind: "amend", Payload: text(`{"a":1}`), StepID: some(3), CreatedAt: at}
	cases := []struct {
		name   string
		corrs  []queries.ListCorrectionsFromRow
		undone bool
		status actionlog.Status
	}{
		{"step live", []queries.ListCorrectionsFromRow{amend}, false, actionlog.StatusAmended},
		{"step popped", []queries.ListCorrectionsFromRow{amend, void(8, 3)}, true, actionlog.StatusActive},
		{"step redone", []queries.ListCorrectionsFromRow{amend, void(8, 3), redo(9, 3)}, false, actionlog.StatusAmended},
		{"step popped again", []queries.ListCorrectionsFromRow{amend, void(8, 3), redo(9, 3), void(10, 3)}, true, actionlog.StatusActive},
		{"entry popped, redone, popped", []queries.ListCorrectionsFromRow{amend, void(8, 2), redo(9, 2), void(10, 2)}, false, actionlog.StatusVoided},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dal, q := mocked(t)
			q.EXPECT().GetAction(ctx, queries.GetActionParams{CharacterID: charID, ID: 2}).Return(row(2, actionlog.KindSave, 0, `{"a":0}`), nil).Once()
			expectAttach(q, 2, nil, c.corrs)
			e, err := dal.GetEntry(ctx, charID, 2)
			require.NoError(t, err)
			assert.Equal(t, c.undone, e.Corrections[0].StepUndone)
			assert.Equal(t, c.status, e.Status())
			kinds := []actionlog.CorrectionKind{}
			for _, corr := range c.corrs {
				if corr.ActionID == 2 {
					kinds = append(kinds, actionlog.CorrectionKind(corr.Kind))
				}
			}
			got := []actionlog.CorrectionKind{}
			for _, corr := range e.Corrections {
				got = append(got, corr.Kind)
			}
			assert.Equal(t, kinds, got, "append-only: every correction of the entry is kept, in order")
		})
	}
}

// The API's createdAt keeps the format it always had, UTC in whole seconds:
// PostgreSQL's microseconds and the session's time zone are dropped when an
// entry and its corrections are read.
func TestGetEntry_CreatedAtReadsAsUTCWholeSeconds(t *testing.T) {
	ctx := context.Background()
	dal, q := mocked(t)
	stored := time.Date(2026, 10, 16, 14, 0, 0, 123456000, time.FixedZone("CEST", 2*3600))
	r := row(4, actionlog.KindHPChange, -3, "{}")
	r.CreatedAt = stored
	q.EXPECT().GetAction(ctx, queries.GetActionParams{CharacterID: charID, ID: 4}).Return(r, nil).Once()
	expectAttach(q, 4, nil, []queries.ListCorrectionsFromRow{
		{ID: 8, ActionID: 4, Kind: "amend", Note: text("n"), CreatedAt: stored},
	})

	e, err := dal.GetEntry(ctx, charID, 4)
	require.NoError(t, err)
	want := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, want, e.CreatedAt)
	require.Len(t, e.Corrections, 1)
	assert.Equal(t, want, e.Corrections[0].CreatedAt)
	out, err := json.Marshal(e.CreatedAt)
	require.NoError(t, err)
	assert.JSONEq(t, `"2026-10-16T12:00:00Z"`, string(out))
}
