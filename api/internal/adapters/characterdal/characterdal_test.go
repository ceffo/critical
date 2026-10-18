// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how results map to domain values; no database, no SQL
// text (ADR 0025 step 6.2).
package characterdal_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/adapters/characterdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/characterdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// Every column of the row lands in its field, the languages decoded from
// their JSON list, the race code apart from the free-text race.
func TestGetCharacter_MapsEveryColumn(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)

	q.EXPECT().GetCharacter(ctx, int64(3)).Return(queries.GetCharacterRow{
		ID: 3, Name: "Bard", Str: 18, Dex: 14, Con: 12, Int: 10, Wis: 9, Cha: 16,
		HpMax: 42, Thac0Base: 16, Level: 5, XpTotal: 1200,
		SaveParaPoison: 13, SaveRodStaffWand: 14, SavePetriPoly: 12, SaveBreath: 16, SaveSpell: 15,
		Class: "bard", RaceCode: "half_elf", Race: "half-elf", Alignment: "CG", Age: 31, Sex: "M",
		Height: "5'10", Weight: "160 lb", Hair: "black", Eyes: "green", Languages: `["Common","Elvish"]`,
	}, nil).Once()
	got, err := characterdal.New(q).GetCharacter(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, &charactersvc.Character{
		ID: 3, Name: "Bard", Str: 18, Dex: 14, Con: 12, Int: 10, Wis: 9, Cha: 16,
		HPMax: 42, XPTotal: 1200, Class: "bard", Race: "half_elf",
		// The stored level, THAC0 and saves are the sheet's figures (ADR 0030 step 24), never inputs.
		Sheet: charactersvc.Sheet{Level: 5, THAC0: 16, Saves: charactersvc.Saves{
			ParaPoison: 13, RodStaffWand: 14, PetriPoly: 12, Breath: 16, Spell: 15,
		}},
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

	q.EXPECT().GetCharacter(ctx, int64(9)).Return(queries.GetCharacterRow{}, sql.ErrNoRows).Once()
	got, err := dal.GetCharacter(ctx, 9)
	assert.Nil(t, got)
	require.ErrorIs(t, err, domain.ErrCharacterNotFound)

	q.EXPECT().GetCharacterDetails(ctx, int64(9)).Return(queries.GetCharacterDetailsRow{}, sql.ErrNoRows).Once()
	_, err = dal.GetDetails(ctx, 9)
	require.ErrorIs(t, err, domain.ErrCharacterNotFound)

	q.EXPECT().UpdateCharacterDetails(ctx, queries.UpdateCharacterDetailsParams{Languages: "[]", ID: 9}).Return(0, nil).Once()
	require.ErrorIs(t, dal.UpdateDetails(ctx, 9, charactersvc.Details{}), domain.ErrCharacterNotFound)
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

	q.EXPECT().GetCharacter(ctx, int64(1)).Return(queries.GetCharacterRow{}, assert.AnError).Once()
	_, err := dal.GetCharacter(ctx, 1)
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, domain.ErrCharacterNotFound)

	q.EXPECT().SetCurrentCharacter(ctx, mock.Anything).Return(0, assert.AnError).Once()
	err = dal.SetCurrentCharacter(ctx, uuid.New(), 1)
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, domain.ErrCharacterNotFound)

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

// The owner's characters map row by row; the set-current update that matches
// no row (a character of another user, retired or unknown) is
// ErrCharacterNotFound.
func TestOwners_ListAndSetCurrent(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := characterdal.New(q)
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	q.EXPECT().CharactersOf(ctx, uuid.NullUUID{UUID: owner, Valid: true}).Return([]queries.CharactersOfRow{
		{ID: 1, Name: "Kelsen", Class: "bard"}, {ID: 4, Name: "Other", Class: "fighter"},
	}, nil).Once()
	list, err := dal.CharactersOf(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, []charactersvc.Summary{
		{ID: 1, Name: "Kelsen", Class: "bard"}, {ID: 4, Name: "Other", Class: "fighter"},
	}, list)

	q.EXPECT().SetCurrentCharacter(ctx, queries.SetCurrentCharacterParams{
		CharacterID: sql.NullInt64{Int64: 4, Valid: true}, UserID: owner,
	}).Return(1, nil).Once()
	require.NoError(t, dal.SetCurrentCharacter(ctx, owner, 4))

	q.EXPECT().SetCurrentCharacter(ctx, queries.SetCurrentCharacterParams{
		CharacterID: sql.NullInt64{Int64: 9, Valid: true}, UserID: owner,
	}).Return(0, nil).Once()
	require.ErrorIs(t, dal.SetCurrentCharacter(ctx, owner, 9), domain.ErrCharacterNotFound)
}
