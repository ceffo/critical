// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to domain values; no database, no SQL
// text (ADR 0025 step 6.2).
package hpdal_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/hpdal"
	"github.com/ceffo/critical/api/internal/adapters/hpdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain"
)

func TestGetMaxHP(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	q.EXPECT().GetCharacterMaxHP(context.Background(), int64(7)).Return(30, nil).Once()

	maxHP, err := hpdal.New(q).GetMaxHP(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, 30, maxHP)
}

func TestGetMaxHP_NotFound(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	q.EXPECT().GetCharacterMaxHP(context.Background(), int64(999)).Return(0, sql.ErrNoRows).Once()

	_, err := hpdal.New(q).GetMaxHP(context.Background(), 999)
	assert.ErrorIs(t, err, domain.ErrCharacterNotFound)
}

func TestGetMaxHP_StorageErrorIsWrapped(t *testing.T) {
	boom := errors.New("boom")
	q := mocks.NewMockQuerier(t)
	q.EXPECT().GetCharacterMaxHP(context.Background(), int64(7)).Return(0, boom).Once()

	_, err := hpdal.New(q).GetMaxHP(context.Background(), 7)
	require.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, domain.ErrCharacterNotFound)
}
