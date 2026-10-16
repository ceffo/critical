// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to domain values; no database, no SQL
// text (ADR 0025 step 6.2).
package xpdal_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/xpdal"
	"github.com/ceffo/critical/api/internal/adapters/xpdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/xpdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
)

// Every operation is scoped to the character it is given, the bonus is stored
// as 0 or 1, and rows map back to entries in the query's order (newest first).
func TestCRUD_ScopesToTheCharacterAndMapsEntries(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := xpdal.New(q)

	q.EXPECT().InsertXPEntry(ctx, queries.InsertXPEntryParams{
		CharacterID: 1, EntryDate: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1200, Bonus: 1,
	}).Return(11, nil).Once()
	id, err := dal.Insert(ctx, 1, xpsvc.Entry{Date: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1200, Bonus: true})
	require.NoError(t, err)
	assert.Equal(t, int64(11), id)

	q.EXPECT().ListXPEntries(ctx, int64(1)).Return([]queries.ListXPEntriesRow{
		{ID: 12, EntryDate: "2026-10-17", Reason: "quest", Base: 500, Bonus: 0},
		{ID: 11, EntryDate: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1200, Bonus: 1},
	}, nil).Once()
	got, err := dal.List(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []xpsvc.Entry{
		{ID: 12, Date: "2026-10-17", Reason: "quest", Base: 500},
		{ID: 11, Date: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1200, Bonus: true},
	}, got)

	q.EXPECT().UpdateXPEntry(ctx, queries.UpdateXPEntryParams{
		Session: "S42", Reason: "ogre chief", Base: 1300, Bonus: 0, CharacterID: 1, ID: 11,
	}).Return(1, nil).Once()
	require.NoError(t, dal.Update(ctx, 1, xpsvc.Entry{ID: 11, Date: "2026-10-10", Session: "S42", Reason: "ogre chief", Base: 1300}))

	q.EXPECT().GetXPEntry(ctx, queries.GetXPEntryParams{CharacterID: 1, ID: 11}).Return(queries.GetXPEntryRow{
		ID: 11, EntryDate: "2026-10-10", Session: "S42", Reason: "ogre chief", Base: 1300, Bonus: 0,
	}, nil).Once()
	one, err := dal.Get(ctx, 1, 11)
	require.NoError(t, err)
	assert.Equal(t, xpsvc.Entry{ID: 11, Date: "2026-10-10", Session: "S42", Reason: "ogre chief", Base: 1300}, one)

	q.EXPECT().DeleteXPEntry(ctx, queries.DeleteXPEntryParams{CharacterID: 1, ID: 12}).Return(1, nil).Once()
	require.NoError(t, dal.Delete(ctx, 1, 12))
}

// No row (an unknown id, or another character's entry) is ErrEntryNotFound.
func TestNotFound_UnknownIDOrOtherCharacter(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := xpdal.New(q)

	q.EXPECT().GetXPEntry(ctx, queries.GetXPEntryParams{CharacterID: 2, ID: 11}).Return(queries.GetXPEntryRow{}, sql.ErrNoRows).Once()
	_, err := dal.Get(ctx, 2, 11)
	require.ErrorIs(t, err, xpsvc.ErrEntryNotFound)

	q.EXPECT().UpdateXPEntry(ctx, queries.UpdateXPEntryParams{Base: 10, CharacterID: 2, ID: 11}).Return(0, nil).Once()
	require.ErrorIs(t, dal.Update(ctx, 2, xpsvc.Entry{ID: 11, Base: 10}), xpsvc.ErrEntryNotFound)

	q.EXPECT().DeleteXPEntry(ctx, queries.DeleteXPEntryParams{CharacterID: 2, ID: 11}).Return(0, nil).Once()
	require.ErrorIs(t, dal.Delete(ctx, 2, 11), xpsvc.ErrEntryNotFound)
}

func TestStorageErrors_AreReturnedNotTakenForNotFound(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	q := mocks.NewMockQuerier(t)
	dal := xpdal.New(q)

	q.EXPECT().ListXPEntries(ctx, int64(1)).Return(nil, boom).Once()
	q.EXPECT().GetXPEntry(ctx, queries.GetXPEntryParams{CharacterID: 1, ID: 11}).Return(queries.GetXPEntryRow{}, boom).Once()
	q.EXPECT().InsertXPEntry(ctx, queries.InsertXPEntryParams{CharacterID: 1, Base: 5}).Return(0, boom).Once()
	q.EXPECT().UpdateXPEntry(ctx, queries.UpdateXPEntryParams{Base: 5, CharacterID: 1, ID: 11}).Return(0, boom).Once()
	q.EXPECT().DeleteXPEntry(ctx, queries.DeleteXPEntryParams{CharacterID: 1, ID: 11}).Return(0, boom).Once()

	_, err := dal.List(ctx, 1)
	require.ErrorIs(t, err, boom)
	_, err = dal.Get(ctx, 1, 11)
	require.ErrorIs(t, err, boom)
	require.NotErrorIs(t, err, xpsvc.ErrEntryNotFound)
	_, err = dal.Insert(ctx, 1, xpsvc.Entry{Base: 5})
	require.ErrorIs(t, err, boom)
	err = dal.Update(ctx, 1, xpsvc.Entry{ID: 11, Base: 5})
	require.ErrorIs(t, err, boom)
	require.NotErrorIs(t, err, xpsvc.ErrEntryNotFound)
	err = dal.Delete(ctx, 1, 11)
	require.ErrorIs(t, err, boom)
	require.NotErrorIs(t, err, xpsvc.ErrEntryNotFound)
}

// Undoing a delete writes the entry back with its own id and date, so a later
// undo of the add before it finds it again. An id still present is refused by
// the store and the error is returned.
func TestReinsert_KeepsIDAndDate(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := xpdal.New(q)
	e := xpsvc.Entry{ID: 11, Date: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1200, Bonus: true}
	params := queries.RestoreXPEntryParams{
		ID: 11, CharacterID: 1, EntryDate: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1200, Bonus: 1,
	}
	q.EXPECT().RestoreXPEntry(ctx, params).Return(nil).Once()
	require.NoError(t, dal.Reinsert(ctx, 1, e))

	taken := errors.New("UNIQUE constraint failed: xp_entries.id")
	q.EXPECT().RestoreXPEntry(ctx, params).Return(taken).Once()
	require.ErrorIs(t, dal.Reinsert(ctx, 1, e), taken, "an id still present is refused")
}
