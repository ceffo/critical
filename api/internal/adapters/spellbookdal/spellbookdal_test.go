// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to domain values; no database, no SQL
// text (ADR 0025 step 6.2).
package spellbookdal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spellbookdal"
	"github.com/ceffo/critical/api/internal/adapters/spellbookdal/queries/mocks"
)

// Keys reads the given character's spellbook and returns its keys as the
// query orders them.
func TestKeys_ReadsTheCharactersSpellbook(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().ListSpellbook(ctx, int64(4)).Return([]string{"alarm", "fireball"}, nil).Once()

	keys, err := spellbookdal.New(q).Keys(ctx, 4)
	require.NoError(t, err)
	assert.Equal(t, []string{"alarm", "fireball"}, keys)
}

func TestKeys_PropagatesTheError(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().ListSpellbook(ctx, int64(4)).Return(nil, assert.AnError).Once()

	_, err := spellbookdal.New(q).Keys(ctx, 4)
	require.ErrorIs(t, err, assert.AnError)
}
