// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to domain values; no database, no SQL
// text (ADR 0025 step 6.2).
package characterdal_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/adapters/characterdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/characterdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// Every column of the row lands in its field, the languages decoded from
// their JSON list.
func TestGetCharacter_MapsEveryColumn(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)

	q.EXPECT().GetCharacter(ctx, int64(3)).Return(queries.Character{
		ID: 3, Name: "Bard", Str: 18, Dex: 14, Con: 12, Int: 10, Wis: 9, Cha: 16,
		HpMax: 42, Thac0Base: 16, Level: 5, XpTotal: 1200,
		SaveParaPoison: 13, SaveRodStaffWand: 14, SavePetriPoly: 12, SaveBreath: 16, SaveSpell: 15,
		Class: "bard", Race: "half-elf", Alignment: "CG", Age: 31, Sex: "M",
		Height: "5'10", Weight: "160 lb", Hair: "black", Eyes: "green", Languages: `["Common","Elvish"]`,
	}, nil).Once()
	got, err := characterdal.New(q).GetCharacter(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, &charactersvc.Character{
		ID: 3, Name: "Bard", Str: 18, Dex: 14, Con: 12, Int: 10, Wis: 9, Cha: 16,
		HPMax: 42, Thac0Base: 16, Level: 5, XPTotal: 1200,
		SaveParaPoison: 13, SaveRodStaffWand: 14, SavePetriPoly: 12, SaveBreath: 16, SaveSpell: 15,
		Class: "bard",
		Details: charactersvc.Details{
			Race: "half-elf", Alignment: "CG", Age: 31, Sex: "M",
			Height: "5'10", Weight: "160 lb", Hair: "black", Eyes: "green", Languages: []string{"Common", "Elvish"},
		},
	}, got)
}

// No row is ErrCharacterNotFound, on every read and on the details update.
func TestNotFound_IsErrCharacterNotFound(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := characterdal.New(q)

	q.EXPECT().GetCharacter(ctx, int64(9)).Return(queries.Character{}, sql.ErrNoRows).Once()
	got, err := dal.GetCharacter(ctx, 9)
	assert.Nil(t, got)
	require.ErrorIs(t, err, domain.ErrCharacterNotFound)

	q.EXPECT().GetFirstCharacterID(ctx).Return(0, sql.ErrNoRows).Once()
	_, err = dal.FirstID(ctx)
	require.ErrorIs(t, err, domain.ErrCharacterNotFound)

	q.EXPECT().GetCharacterDetails(ctx, int64(9)).Return(queries.GetCharacterDetailsRow{}, sql.ErrNoRows).Once()
	_, err = dal.GetDetails(ctx, 9)
	require.ErrorIs(t, err, domain.ErrCharacterNotFound)

	q.EXPECT().UpdateCharacterDetails(ctx, queries.UpdateCharacterDetailsParams{Languages: "[]", ID: 9}).Return(0, nil).Once()
	require.ErrorIs(t, dal.UpdateDetails(ctx, 9, charactersvc.Details{}), domain.ErrCharacterNotFound)
}

func TestFirstID_IsTheQueriedID(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)

	q.EXPECT().GetFirstCharacterID(ctx).Return(4, nil).Once()
	id, err := characterdal.New(q).FirstID(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(4), id)
}

// What UpdateDetails stores for a character is what GetDetails maps back:
// every field, the languages as a JSON list in their order (none as an empty
// list, read back as an empty list).
func TestDetails_UpdateStoresWhatGetMapsBack(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		details charactersvc.Details
		stored  string
		back    []string
	}{
		"languages": {
			details: charactersvc.Details{
				Race: "half-elf", Alignment: "CG", Age: 31, Sex: "M", Height: "5'10", Weight: "160 lb",
				Hair: "black", Eyes: "green", Languages: []string{"Elvish", "Common"},
			},
			stored: `["Elvish","Common"]`, back: []string{"Elvish", "Common"},
		},
		"none": {details: charactersvc.Details{Race: "human"}, stored: `[]`, back: []string{}},
	} {
		t.Run(name, func(t *testing.T) {
			q := mocks.NewMockQuerier(t)
			dal := characterdal.New(q)
			d := tc.details
			q.EXPECT().UpdateCharacterDetails(ctx, queries.UpdateCharacterDetailsParams{
				Race: d.Race, Alignment: d.Alignment, Age: int64(d.Age), Sex: d.Sex, Height: d.Height,
				Weight: d.Weight, Hair: d.Hair, Eyes: d.Eyes, Languages: tc.stored, ID: 1,
			}).Return(1, nil).Once()
			require.NoError(t, dal.UpdateDetails(ctx, 1, d))

			q.EXPECT().GetCharacterDetails(ctx, int64(1)).Return(queries.GetCharacterDetailsRow{
				Race: d.Race, Alignment: d.Alignment, Age: int64(d.Age), Sex: d.Sex, Height: d.Height,
				Weight: d.Weight, Hair: d.Hair, Eyes: d.Eyes, Languages: tc.stored,
			}, nil).Once()
			got, err := dal.GetDetails(ctx, 1)
			require.NoError(t, err)
			d.Languages = tc.back
			assert.Equal(t, d, got)
		})
	}
}

// Storage errors are returned as they are, never taken for not found; a
// stored language list that is not JSON is an error.
func TestStorageErrors_AreReturnedNotTakenForNotFound(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := characterdal.New(q)

	q.EXPECT().GetCharacter(ctx, int64(1)).Return(queries.Character{}, assert.AnError).Once()
	_, err := dal.GetCharacter(ctx, 1)
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, domain.ErrCharacterNotFound)

	q.EXPECT().GetFirstCharacterID(ctx).Return(0, assert.AnError).Once()
	_, err = dal.FirstID(ctx)
	require.ErrorIs(t, err, assert.AnError)

	q.EXPECT().GetCharacterDetails(ctx, int64(1)).Return(queries.GetCharacterDetailsRow{}, assert.AnError).Once()
	_, err = dal.GetDetails(ctx, 1)
	require.ErrorIs(t, err, assert.AnError)

	q.EXPECT().UpdateCharacterDetails(ctx, queries.UpdateCharacterDetailsParams{Languages: "[]", ID: 1}).Return(0, assert.AnError).Once()
	err = dal.UpdateDetails(ctx, 1, charactersvc.Details{})
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, domain.ErrCharacterNotFound)

	q.EXPECT().GetCharacterDetails(ctx, int64(1)).Return(queries.GetCharacterDetailsRow{Languages: "Common"}, nil).Once()
	_, err = dal.GetDetails(ctx, 1)
	require.Error(t, err)
}
