// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to domain values; no database, no SQL
// text (ADR 0025 step 6.2).
package skilldal_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/skilldal"
	"github.com/ceffo/critical/api/internal/adapters/skilldal/queries"
	"github.com/ceffo/critical/api/internal/adapters/skilldal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
)

// The rows come back in the query's order (the sheet's position order) and
// map to the skill definitions one for one.
func TestList_MapsRowsInTheirOrder(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	q.EXPECT().ListProficiencies(ctx, int64(4)).Return([]queries.ListProficienciesRow{
		{ID: "a", Ability: "dex", Modifier: 3},
		{ID: "b", Ability: "int", Modifier: -2},
	}, nil).Once()
	q.EXPECT().ListBardSkills(ctx, int64(4)).Return([]queries.ListBardSkillsRow{
		{ID: "x", Percent: 85},
		{ID: "y", Percent: 50},
	}, nil).Once()
	dal := skilldal.New(q)

	profs, err := dal.ListProficiencies(ctx, 4)
	require.NoError(t, err)
	assert.Equal(t, []skillsvc.ProficiencyDef{
		{ID: "a", Ability: skillsvc.AbilityDex, Modifier: 3},
		{ID: "b", Ability: skillsvc.AbilityInt, Modifier: -2},
	}, profs)
	skills, err := dal.ListBardSkills(ctx, 4)
	require.NoError(t, err)
	assert.Equal(t, []skillsvc.BardSkillDef{{ID: "x", Percent: 85}, {ID: "y", Percent: 50}}, skills)
}

func TestList_StorageErrorsAreReturned(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	q := mocks.NewMockQuerier(t)
	q.EXPECT().ListProficiencies(ctx, int64(4)).Return(nil, boom).Once()
	q.EXPECT().ListBardSkills(ctx, int64(4)).Return(nil, boom).Once()
	dal := skilldal.New(q)

	_, err := dal.ListProficiencies(ctx, 4)
	require.ErrorIs(t, err, boom)
	_, err = dal.ListBardSkills(ctx, 4)
	require.ErrorIs(t, err, boom)
}
