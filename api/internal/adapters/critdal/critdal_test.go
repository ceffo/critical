// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to domain values; no database, no SQL
// text (ADR 0025 step 6.2).
package critdal_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/critdal"
	"github.com/ceffo/critical/api/internal/adapters/critdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/critdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

func TestList_EmptyUntilSaved(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().ListCritRows(ctx).Return([]queries.CritTableRow{}, nil).Once()

	got, err := critdal.New(q).List(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}

// Decimal multipliers (x1.5) come back as stored.
func TestList_MapsRowsWithDecimalMultipliers(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().ListCritRows(ctx).Return([]queries.CritTableRow{
		{RollFrom: 1, RollTo: 50, Multiplier: 1.5},
		{RollFrom: 51, RollTo: 100, Multiplier: 2.25},
	}, nil).Once()

	got, err := critdal.New(q).List(ctx)
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.CritRow{{From: 1, To: 50, Multiplier: 1.5}, {From: 51, To: 100, Multiplier: 2.25}}, got)
}

// Replace clears the table first, then inserts every row in order.
func TestReplace_SwapsTheWholeTable(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	mock.InOrder(
		q.EXPECT().DeleteCritRows(ctx).Return(nil).Once(),
		q.EXPECT().InsertCritRow(ctx, queries.InsertCritRowParams{RollFrom: 1, RollTo: 50, Multiplier: 2}).Return(nil).Once(),
		q.EXPECT().InsertCritRow(ctx, queries.InsertCritRowParams{RollFrom: 51, RollTo: 100, Multiplier: 2.5}).Return(nil).Once(),
	)

	require.NoError(t, critdal.New(q).Replace(ctx, []combatsvc.CritRow{
		{From: 1, To: 50, Multiplier: 2}, {From: 51, To: 100, Multiplier: 2.5},
	}))
}

// A failing insert stops the swap and returns its error, so the caller's
// transaction rolls back to the previous table.
func TestReplace_StopsAtTheFirstFailingWrite(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("check constraint")
	q := mocks.NewMockQuerier(t)
	mock.InOrder(
		q.EXPECT().DeleteCritRows(ctx).Return(nil).Once(),
		q.EXPECT().InsertCritRow(ctx, queries.InsertCritRowParams{RollFrom: 1, RollTo: 50, Multiplier: 3}).Return(boom).Once(),
	)

	err := critdal.New(q).Replace(ctx, []combatsvc.CritRow{
		{From: 1, To: 50, Multiplier: 3}, {From: 51, To: 100, Multiplier: 0},
	})
	require.ErrorIs(t, err, boom)
}

func TestReplace_ClearFailureWritesNothing(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	q := mocks.NewMockQuerier(t)
	q.EXPECT().DeleteCritRows(ctx).Return(boom).Once()

	err := critdal.New(q).Replace(ctx, []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 2}})
	require.ErrorIs(t, err, boom)
}
