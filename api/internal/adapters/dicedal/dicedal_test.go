// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to domain values; no database, no SQL
// text (ADR 0025 step 6.2).
package dicedal_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/dicedal"
	"github.com/ceffo/critical/api/internal/adapters/dicedal/queries"
	"github.com/ceffo/critical/api/internal/adapters/dicedal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
)

func TestList_MapsTheCharactersFavouritesInTheirOrder(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().ListDiceFavourites(ctx, int64(1)).Return([]queries.ListDiceFavouritesRow{
		{ID: 4, Formula: "d10"}, {ID: 2, Formula: "d6"},
	}, nil).Once()

	got, err := dicedal.New(q).List(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []dicesvc.Favourite{{ID: 4, Formula: "d10"}, {ID: 2, Formula: "d6"}}, got)
}

// A new favourite goes after the character's last position, gaps left by
// deletes included; the first one takes position 1.
func TestInsert_GoesAfterTheLastPosition(t *testing.T) {
	cases := []struct {
		name     string
		last     int64
		lastErr  error
		position int64
	}{
		{"first favourite", 0, sql.ErrNoRows, 1},
		{"after a gap", 3, nil, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			q := mocks.NewMockQuerier(t)
			mock.InOrder(
				q.EXPECT().LastDiceFavouritePosition(ctx, int64(1)).Return(c.last, c.lastErr).Once(),
				q.EXPECT().InsertDiceFavourite(ctx, queries.InsertDiceFavouriteParams{
					CharacterID: 1, Formula: "2d6+1", Position: c.position,
				}).Return(9, nil).Once(),
			)

			id, err := dicedal.New(q).Insert(ctx, 1, "2d6+1")
			require.NoError(t, err)
			assert.Equal(t, int64(9), id)
		})
	}
}

// The store refuses the same formula twice for one character (its unique
// key); the error is returned.
func TestInsert_RefusesTheSameFormulaTwice(t *testing.T) {
	ctx := context.Background()
	taken := errors.New("UNIQUE constraint failed")
	q := mocks.NewMockQuerier(t)
	q.EXPECT().LastDiceFavouritePosition(ctx, int64(1)).Return(1, nil).Once()
	q.EXPECT().InsertDiceFavourite(ctx, queries.InsertDiceFavouriteParams{CharacterID: 1, Formula: "d20", Position: 2}).
		Return(0, taken).Once()

	_, err := dicedal.New(q).Insert(ctx, 1, "d20")
	require.ErrorIs(t, err, taken)
}

func TestInsert_PositionLookupFailureInsertsNothing(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	q := mocks.NewMockQuerier(t)
	q.EXPECT().LastDiceFavouritePosition(ctx, int64(1)).Return(0, boom).Once()

	_, err := dicedal.New(q).Insert(ctx, 1, "d20")
	require.ErrorIs(t, err, boom)
}

// Delete and SetPosition are scoped to the character and report whether a row
// was touched: another character's favourite is out of reach.
func TestDeleteAndSetPosition_ReportWhetherTheFavouriteExisted(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := dicedal.New(q)

	q.EXPECT().DeleteDiceFavourite(ctx, queries.DeleteDiceFavouriteParams{CharacterID: 1, ID: 5}).Return(1, nil).Once()
	q.EXPECT().DeleteDiceFavourite(ctx, queries.DeleteDiceFavouriteParams{CharacterID: 2, ID: 5}).Return(0, nil).Once()
	q.EXPECT().SetDiceFavouritePosition(ctx, queries.SetDiceFavouritePositionParams{Position: 3, CharacterID: 1, ID: 6}).Return(1, nil).Once()
	q.EXPECT().SetDiceFavouritePosition(ctx, queries.SetDiceFavouritePositionParams{Position: 1, CharacterID: 2, ID: 6}).Return(0, nil).Once()

	removed, err := dal.Delete(ctx, 1, 5)
	require.NoError(t, err)
	assert.True(t, removed)
	removed, err = dal.Delete(ctx, 2, 5)
	require.NoError(t, err)
	assert.False(t, removed, "another character's favourite is out of reach")
	moved, err := dal.SetPosition(ctx, 1, 6, 3)
	require.NoError(t, err)
	assert.True(t, moved)
	moved, err = dal.SetPosition(ctx, 2, 6, 1)
	require.NoError(t, err)
	assert.False(t, moved, "another character's favourite is out of reach")
}

func TestStorageErrors_AreReturned(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	q := mocks.NewMockQuerier(t)
	dal := dicedal.New(q)
	q.EXPECT().ListDiceFavourites(ctx, int64(1)).Return(nil, boom).Once()
	q.EXPECT().DeleteDiceFavourite(ctx, queries.DeleteDiceFavouriteParams{CharacterID: 1, ID: 5}).Return(0, boom).Once()
	q.EXPECT().SetDiceFavouritePosition(ctx, queries.SetDiceFavouritePositionParams{Position: 1, CharacterID: 1, ID: 5}).Return(0, boom).Once()

	_, err := dal.List(ctx, 1)
	require.ErrorIs(t, err, boom)
	_, err = dal.Delete(ctx, 1, 5)
	require.ErrorIs(t, err, boom)
	_, err = dal.SetPosition(ctx, 1, 5, 1)
	require.ErrorIs(t, err, boom)
}
