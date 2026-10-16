// The store runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to texts; no database, no SQL text
// (ADR 0025 step 6.2).
package textdal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/textdal"
	"github.com/ceffo/critical/api/internal/adapters/textdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/textdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// A key's rows read as one body per language; a key with no row is an empty
// text.
func TestGet_RowsAreOneBodyPerLanguage(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	s := textdal.New(q)

	q.EXPECT().GetText(ctx, int64(7)).Return([]queries.GetTextRow{
		{Lang: "en", Body: "Short Bow"}, {Lang: "fr", Body: "Arc court"},
	}, nil).Once()
	got, err := s.Get(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, localized.Text{"en": "Short Bow", "fr": "Arc court"}, got)

	q.EXPECT().GetText(ctx, int64(8)).Return([]queries.GetTextRow{}, nil).Once()
	got, err = s.Get(ctx, 8)
	require.NoError(t, err)
	assert.Empty(t, got)
}

// Add takes the next key, then writes one row per language in code order,
// none for a blank body (ADR 0023 step 9: a new key on insert).
func TestAdd_NextKeyThenOneRowPerLanguage(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	s := textdal.New(q)

	mock.InOrder(
		q.EXPECT().NextTextKey(ctx).Return(42, nil).Once(),
		q.EXPECT().InsertText(ctx, queries.InsertTextParams{TextID: 42, Lang: "de", Body: "Stab"}).Return(nil).Once(),
		q.EXPECT().InsertText(ctx, queries.InsertTextParams{TextID: 42, Lang: "fr", Body: "Baguette"}).Return(nil).Once(),
	)
	key, err := s.Add(ctx, localized.Text{"fr": "Baguette", "de": "Stab", "en": ""})
	require.NoError(t, err)
	assert.Equal(t, int64(42), key)
}

// Put replaces the rows under the same key: every row deleted first, then
// one per language (ADR 0023 step 9: replaced under the same key on update).
func TestPut_DeletesTheKeysRowsThenWritesTheNewOnes(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	s := textdal.New(q)

	mock.InOrder(
		q.EXPECT().DeleteText(ctx, int64(5)).Return(nil).Once(),
		q.EXPECT().InsertText(ctx, queries.InsertTextParams{TextID: 5, Lang: "en", Body: "Rope"}).Return(nil).Once(),
	)
	require.NoError(t, s.Put(ctx, 5, localized.Text{"en": "Rope"}))
}

func TestDelete_RemovesTheKeysRows(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)

	q.EXPECT().DeleteText(ctx, int64(5)).Return(nil).Once()
	require.NoError(t, textdal.New(q).Delete(ctx, 5))
}

// A failed step stops the write: no row goes in after a failed key read or
// delete, and the error is returned.
func TestStorageErrors_StopTheWriteAndAreReturned(t *testing.T) {
	ctx := context.Background()
	text := localized.Text{"en": "Rope"}

	q := mocks.NewMockQuerier(t)
	q.EXPECT().NextTextKey(ctx).Return(0, assert.AnError).Once()
	_, err := textdal.New(q).Add(ctx, text)
	require.ErrorIs(t, err, assert.AnError)

	q = mocks.NewMockQuerier(t)
	q.EXPECT().DeleteText(ctx, int64(5)).Return(assert.AnError).Once()
	require.ErrorIs(t, textdal.New(q).Put(ctx, 5, text), assert.AnError)

	q = mocks.NewMockQuerier(t)
	q.EXPECT().NextTextKey(ctx).Return(9, nil).Once()
	q.EXPECT().InsertText(ctx, queries.InsertTextParams{TextID: 9, Lang: "en", Body: "Rope"}).Return(assert.AnError).Once()
	_, err = textdal.New(q).Add(ctx, text)
	require.ErrorIs(t, err, assert.AnError)

	q = mocks.NewMockQuerier(t)
	q.EXPECT().GetText(ctx, int64(5)).Return(nil, assert.AnError).Once()
	_, err = textdal.New(q).Get(ctx, 5)
	require.ErrorIs(t, err, assert.AnError)
}
