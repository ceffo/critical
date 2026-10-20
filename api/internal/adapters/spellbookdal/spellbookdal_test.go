// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to domain values; no database, no SQL
// text (ADR 0025 step 6.2).
package spellbookdal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spellbookdal"
	"github.com/ceffo/critical/api/internal/adapters/spellbookdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/spellbookdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// Keys reads the given character's spellbook and returns its keys as the
// query orders them.
func TestKeys_ReadsTheCharactersSpellbook(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().ListSpellbook(ctx, int64(4)).Return([]queries.ListSpellbookRow{
		{SpellKey: "alarm", Origin: "starting"}, {SpellKey: "fireball", Origin: "learned"},
	}, nil).Once()

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

// ADR 0030 step 20.5: "The origin is stored with the entry and served with the
// spellbook". Each of the four origins is written with the entry (level_grant
// with the realized level that granted it) and read back as it was written.
func TestSpellbook_Step20_5_EveryEntryKeepsItsOrigin(t *testing.T) {
	ctx := context.Background()
	entries := []spellbook.Entry{
		{Key: "sleep", Origin: spellbook.OriginLearned},
		{Key: "alarm", Origin: spellbook.OriginGiven},
		{Key: "read_magic", Origin: spellbook.OriginStarting},
		{Key: "fireball", Origin: spellbook.OriginLevelGrant, GrantLevel: 5},
	}
	q := mocks.NewMockQuerier(t)
	var stored []queries.ListSpellbookRow
	q.EXPECT().AddSpellbookEntry(ctx, mock.Anything).RunAndReturn(func(_ context.Context, p queries.AddSpellbookEntryParams) error {
		assert.Equal(t, int64(4), p.CharacterID)
		stored = append(stored, queries.ListSpellbookRow{SpellKey: p.SpellKey, Origin: p.Origin, GrantLevel: p.GrantLevel})
		return nil
	}).Times(len(entries))
	q.EXPECT().ListSpellbook(ctx, int64(4)).RunAndReturn(func(context.Context, int64) ([]queries.ListSpellbookRow, error) {
		return stored, nil
	}).Once()

	d := spellbookdal.New(q)
	for _, e := range entries {
		require.NoError(t, d.AddEntry(ctx, 4, e))
	}
	assert.Equal(t, []queries.ListSpellbookRow{
		{SpellKey: "sleep", Origin: "learned"},
		{SpellKey: "alarm", Origin: "given"},
		{SpellKey: "read_magic", Origin: "starting"},
		{SpellKey: "fireball", Origin: "level_grant", GrantLevel: 5},
	}, stored, "the column values")
	got, err := d.Entries(ctx, 4)
	require.NoError(t, err)
	assert.Equal(t, entries, got)
}

// An origin the domain does not know is an error naming the entry, never an
// entry with an empty origin.
func TestEntries_RefusesAnUnknownOrigin(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().ListSpellbook(ctx, int64(4)).Return([]queries.ListSpellbookRow{{SpellKey: "alarm", Origin: "bought"}}, nil).Once()
	_, err := spellbookdal.New(q).Entries(ctx, 4)
	require.ErrorContains(t, err, `"alarm" has the unknown origin "bought"`)
}

// An add and a restore are two queries: AddEntry's insert leaves a spell
// already in the book as it is (DO NOTHING), only RestoreEntry, an undo or a
// redo, sets an existing entry's origin and grant level.
func TestRestoreEntry_RunsTheRestoreQuery(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().RestoreSpellbookEntry(ctx, queries.RestoreSpellbookEntryParams{
		CharacterID: 4, SpellKey: "fireball", Origin: "level_grant", GrantLevel: 5,
	}).Return(nil).Once()
	require.NoError(t, spellbookdal.New(q).RestoreEntry(ctx, 4, spellbook.Entry{Key: "fireball", Origin: spellbook.OriginLevelGrant, GrantLevel: 5}))

	q.EXPECT().RestoreSpellbookEntry(ctx, mock.Anything).Return(assert.AnError).Once()
	require.ErrorIs(t, spellbookdal.New(q).RestoreEntry(ctx, 4, spellbook.Entry{Key: "fireball"}), assert.AnError)
}
