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
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// Every column of the row lands in its field, the languages decoded from
// their JSON list, the race code apart from the free-text race.
func TestGetCharacter_MapsEveryColumn(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)

	q.EXPECT().GetCharacter(ctx, int64(3)).Return(queries.GetCharacterRow{
		ID: 3, Name: "Bard", Str: 18, Dex: 14, Con: 12, Int: 10, Wis: 9, Cha: 16,
		HpMax: 42, RealizedLevel: 6, Thac0Base: 16, Level: 5, XpTotal: 1200,
		SaveParaPoison: 13, SaveRodStaffWand: 14, SavePetriPoly: 12, SaveBreath: 16, SaveSpell: 15,
		Class: "bard", RaceCode: "half_elf", Alignment: "chaotic_good", Age: 31, Sex: "male",
		HeightCm: 177.8, WeightKg: 72.6, Hair: "black", Eyes: "green", Languages: `["Common","Elvish"]`,
		SkillPoints: `{}`,
	}, nil).Once()
	got, err := characterdal.New(q).GetCharacter(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, &charactersvc.Character{
		ID: 3, Name: "Bard", Str: 18, Dex: 14, Con: 12, Int: 10, Wis: 9, Cha: 16,
		HPMax: 42, RealizedLevel: 6, XPTotal: 1200, Class: "bard", Race: "half_elf",
		// The stored level, THAC0 and saves are the sheet's figures (ADR 0030 step 24), never inputs.
		Sheet: charactersvc.Sheet{Level: 5, THAC0: 16, Saves: charactersvc.Saves{
			ParaPoison: 13, RodStaffWand: 14, PetriPoly: 12, Breath: 16, Spell: 15,
		}},
		Details: charactersvc.Details{
			Alignment: "chaotic_good", Age: 31, Sex: "male",
			Height: 177.8, Weight: 72.6, Hair: "black", Eyes: "green", Languages: []string{"Common", "Elvish"},
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
	_, _, err = dal.GetDetails(ctx, 9)
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
				Alignment: "chaotic_good", Age: 31, Sex: "male", Height: 177.8, Weight: 72.6,
				Hair: "black", Eyes: "green", Languages: []string{"Elvish", "Common"},
			},
			stored: `["Elvish","Common"]`, back: []string{"Elvish", "Common"},
		},
		"none": {details: charactersvc.Details{Alignment: "true_neutral"}, stored: `[]`, back: []string{}},
	} {
		t.Run(name, func(t *testing.T) {
			q := mocks.NewMockQuerier(t)
			dal := characterdal.New(q)
			d := tc.details
			q.EXPECT().UpdateCharacterDetails(ctx, queries.UpdateCharacterDetailsParams{
				Alignment: d.Alignment, Age: int64(d.Age), Sex: d.Sex, HeightCm: d.Height,
				WeightKg: d.Weight, Hair: d.Hair, Eyes: d.Eyes, Languages: tc.stored, ID: 1,
			}).Return(1, nil).Once()
			require.NoError(t, dal.UpdateDetails(ctx, 1, d))

			q.EXPECT().GetCharacterDetails(ctx, int64(1)).Return(queries.GetCharacterDetailsRow{
				Class: "bard", RaceCode: "elf", Alignment: d.Alignment, Age: int64(d.Age), Sex: d.Sex, HeightCm: d.Height,
				WeightKg: d.Weight, Hair: d.Hair, Eyes: d.Eyes, Languages: tc.stored,
			}, nil).Once()
			got, kind, err := dal.GetDetails(ctx, 1)
			require.NoError(t, err)
			assert.Equal(t, charactersvc.Kind{Class: "bard", Race: "elf"}, kind, "the class and race come with the details")
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

	who := uuid.New()
	q.EXPECT().LockUser(ctx, who).Return(who, nil).Once()
	q.EXPECT().SetCurrentCharacter(ctx, mock.Anything).Return(0, assert.AnError).Once()
	err = dal.SetCurrentCharacter(ctx, who, 1)
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, domain.ErrCharacterNotFound)

	q.EXPECT().GetCharacterDetails(ctx, int64(1)).Return(queries.GetCharacterDetailsRow{}, assert.AnError).Once()
	_, _, err = dal.GetDetails(ctx, 1)
	require.ErrorIs(t, err, assert.AnError)

	q.EXPECT().UpdateCharacterDetails(ctx, queries.UpdateCharacterDetailsParams{Languages: "[]", ID: 1}).Return(0, assert.AnError).Once()
	err = dal.UpdateDetails(ctx, 1, charactersvc.Details{})
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, domain.ErrCharacterNotFound)

	q.EXPECT().GetCharacterDetails(ctx, int64(1)).Return(queries.GetCharacterDetailsRow{Languages: "Common"}, nil).Once()
	_, _, err = dal.GetDetails(ctx, 1)
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

	q.EXPECT().LockUser(ctx, owner).Return(owner, nil).Twice()
	q.EXPECT().SetCurrentCharacter(ctx, queries.SetCurrentCharacterParams{
		CharacterID: sql.NullInt64{Int64: 4, Valid: true}, UserID: owner,
	}).Return(1, nil).Once()
	require.NoError(t, dal.SetCurrentCharacter(ctx, owner, 4))

	q.EXPECT().SetCurrentCharacter(ctx, queries.SetCurrentCharacterParams{
		CharacterID: sql.NullInt64{Int64: 9, Valid: true}, UserID: owner,
	}).Return(0, nil).Once()
	require.ErrorIs(t, dal.SetCurrentCharacter(ctx, owner, 9), domain.ErrCharacterNotFound)
}

// A created character's row carries every entered value, its owner, and its
// sheet figures in the level, THAC0 and save columns (ADR 0030 step 24); its
// spell slots, buffs and round state follow, keyed by the new id, then the
// details entered with it.
func TestCreateCharacter_TheRowThenItsStateRows(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	owner := uuid.MustParse("0b6a8c7e-5f1d-4a59-9a3e-3c2d1e0f4a11")
	lock := q.EXPECT().LockUser(ctx, owner).Return(owner, nil).Once()
	row := q.EXPECT().CreateCharacter(ctx, queries.CreateCharacterParams{
		Name: "Lyra", Str: 10, Dex: 15, Con: 12, Int: 14, Wis: 9, Cha: 16, HpMax: 6, XpTotal: 2500,
		Thac0Base: 19, Level: 3, SaveParaPoison: 13, SaveRodStaffWand: 14, SavePetriPoly: 12,
		SaveBreath: 16, SaveSpell: 15, Class: "bard", RaceCode: "half_elf",
		OwnerUserID: uuid.NullUUID{UUID: owner, Valid: true}, RealizedLevel: sql.NullInt64{Int64: 3, Valid: true},
		SkillPoints: `{}`,
	}).Return(int64(7), nil).Once().NotBefore(lock)
	slots := q.EXPECT().CreateSpellSlots(ctx, int64(7)).Return(nil).Once().NotBefore(row)
	buffs := q.EXPECT().CreateActiveBuffs(ctx, int64(7)).Return(nil).Once().NotBefore(slots)
	round := q.EXPECT().CreateRoundState(ctx, int64(7)).Return(nil).Once().NotBefore(buffs)
	q.EXPECT().UpdateCharacterDetails(ctx, queries.UpdateCharacterDetailsParams{
		Alignment: "neutral_good", Age: 23, Languages: `["Elfique","Commun"]`, ID: 7,
	}).Return(1, nil).Once().NotBefore(round)

	id, err := characterdal.New(q).CreateCharacter(ctx, owner, charactersvc.Character{
		ID: 99, Name: "Lyra", Str: 10, Dex: 15, Con: 12, Int: 14, Wis: 9, Cha: 16, HPMax: 6, RealizedLevel: 3, XPTotal: 2500,
		Class: "bard", Race: "half_elf",
		Details: charactersvc.Details{Alignment: "neutral_good", Age: 23, Languages: []string{"Elfique", "Commun"}},
		Sheet: charactersvc.Sheet{Level: 3, THAC0: 19, Saves: charactersvc.Saves{
			ParaPoison: 13, RodStaffWand: 14, PetriPoly: 12, Breath: 16, Spell: 15,
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(7), id)
}

// ADR 0030 step 7.3: a level-up roll (and its undo) writes the max HP and
// the level it is set for; no row is ErrCharacterNotFound.
func TestSetRealizedLevel_WritesTheLevelAndItsMaxHP(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := characterdal.New(q)
	q.EXPECT().SetRealizedLevel(ctx, queries.SetRealizedLevelParams{
		HpMax: 91, RealizedLevel: sql.NullInt64{Int64: 16, Valid: true}, ID: 3,
	}).Return(1, nil).Once()
	require.NoError(t, dal.SetRealizedLevel(ctx, 3, 16, 91))

	q.EXPECT().SetRealizedLevel(ctx, queries.SetRealizedLevelParams{
		HpMax: 1, RealizedLevel: sql.NullInt64{Int64: 2, Valid: true}, ID: 9,
	}).Return(0, nil).Once()
	require.ErrorIs(t, dal.SetRealizedLevel(ctx, 9, 2, 1), domain.ErrCharacterNotFound)
}

// A rename or a retirement that matches no row (another user's character, a
// retired one, an unknown id) is ErrCharacterNotFound; the current
// character read with no row is none.
func TestRenameRetire_NoRowIsNotFound(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	owner := uuid.MustParse("0b6a8c7e-5f1d-4a59-9a3e-3c2d1e0f4a11")
	dal := characterdal.New(q)
	nullOwner := uuid.NullUUID{UUID: owner, Valid: true}

	q.EXPECT().RenameCharacter(ctx, queries.RenameCharacterParams{Name: "B", ID: 4, OwnerUserID: nullOwner}).Return(0, nil).Once()
	require.ErrorIs(t, dal.RenameCharacter(ctx, owner, 4, "B"), domain.ErrCharacterNotFound)
	q.EXPECT().RenameCharacter(ctx, queries.RenameCharacterParams{Name: "B", ID: 5, OwnerUserID: nullOwner}).Return(1, nil).Once()
	require.NoError(t, dal.RenameCharacter(ctx, owner, 5, "B"))

	q.EXPECT().LockUser(ctx, owner).Return(owner, nil).Twice()
	q.EXPECT().RetireCharacter(ctx, queries.RetireCharacterParams{ID: 4, OwnerUserID: nullOwner}).Return(0, nil).Once()
	require.ErrorIs(t, dal.RetireCharacter(ctx, owner, 4), domain.ErrCharacterNotFound)
	q.EXPECT().RetireCharacter(ctx, queries.RetireCharacterParams{ID: 5, OwnerUserID: nullOwner}).Return(1, nil).Once()
	require.NoError(t, dal.RetireCharacter(ctx, owner, 5))

	q.EXPECT().RepairCurrentCharacter(ctx, owner).Return(nil).Once()
	require.NoError(t, dal.RepairCurrentCharacter(ctx, owner))

	q.EXPECT().CurrentCharacter(ctx, owner).Return(0, sql.ErrNoRows).Once()
	current, err := dal.CurrentCharacter(ctx, owner)
	require.NoError(t, err)
	assert.Zero(t, current)
	q.EXPECT().CurrentCharacter(ctx, owner).Return(5, nil).Once()
	current, err = dal.CurrentCharacter(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, int64(5), current)
}

// A user row that is not there is no lock to take: the guarded update then
// matches nothing and says not found; a lock failure is an error.
func TestLockUser_UnknownUserIsNotAnError(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := characterdal.New(q)
	owner := uuid.New()

	q.EXPECT().LockUser(ctx, owner).Return(uuid.Nil, sql.ErrNoRows).Once()
	q.EXPECT().SetCurrentCharacter(ctx, mock.Anything).Return(0, nil).Once()
	require.ErrorIs(t, dal.SetCurrentCharacter(ctx, owner, 1), domain.ErrCharacterNotFound)

	q.EXPECT().LockUser(ctx, owner).Return(uuid.Nil, assert.AnError).Once()
	require.ErrorIs(t, dal.RetireCharacter(ctx, owner, 1), assert.AnError)
}

// A thief's skill points (ADR 0030 step 1) are stored as a JSON object by
// skill code and read back as the same map. No armour is entered or read: the
// skills read the worn one (ADR 0033 step 14).
func TestThiefSkillPoints_StoredAsJSONAndReadBack(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	owner := uuid.MustParse("0b6a8c7e-5f1d-4a59-9a3e-3c2d1e0f4a11")
	q.EXPECT().LockUser(ctx, owner).Return(owner, nil).Once()
	q.EXPECT().CreateCharacter(ctx, queries.CreateCharacterParams{
		Name: "Vex", Class: "thief", RaceCode: "halfling", OwnerUserID: uuid.NullUUID{UUID: owner, Valid: true},
		RealizedLevel: sql.NullInt64{Int64: 1, Valid: true},
		SkillPoints:   `{"open_locks":20,"pick_pockets":15}`,
	}).Return(int64(8), nil).Once()
	q.EXPECT().CreateSpellSlots(ctx, int64(8)).Return(nil).Once()
	q.EXPECT().CreateActiveBuffs(ctx, int64(8)).Return(nil).Once()
	q.EXPECT().CreateRoundState(ctx, int64(8)).Return(nil).Once()
	q.EXPECT().UpdateCharacterDetails(ctx, queries.UpdateCharacterDetailsParams{Languages: `[]`, ID: 8}).Return(1, nil).Once()
	points := map[string]int{"pick_pockets": 15, "open_locks": 20}
	_, err := characterdal.New(q).CreateCharacter(ctx, owner, charactersvc.Character{
		Name: "Vex", Class: "thief", Race: "halfling", RealizedLevel: 1, SkillPoints: points,
	})
	require.NoError(t, err)

	q.EXPECT().GetCharacter(ctx, int64(8)).Return(queries.GetCharacterRow{
		ID: 8, Class: "thief", Languages: `[]`, SkillPoints: `{"open_locks":20,"pick_pockets":15}`,
	}, nil).Once()
	got, err := characterdal.New(q).GetCharacter(ctx, 8)
	require.NoError(t, err)
	assert.Equal(t, points, got.SkillPoints)
}

// ADR 0033 step 13: the AC override is read as nil when the column is NULL,
// as a value and label otherwise; a missing row is ErrCharacterNotFound.
func TestGetACOverride_NullIsNone(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := characterdal.New(q)
	q.EXPECT().GetACOverride(ctx, int64(3)).Return(queries.GetACOverrideRow{}, nil).Once()
	o, err := dal.GetACOverride(ctx, 3)
	require.NoError(t, err)
	assert.Nil(t, o)

	q.EXPECT().GetACOverride(ctx, int64(3)).Return(queries.GetACOverrideRow{
		AcOverride: sql.NullInt32{Int32: -1, Valid: true}, AcOverrideLabel: "Shield spell",
	}, nil).Once()
	o, err = dal.GetACOverride(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, &acsvc.Override{Value: -1, Label: "Shield spell"}, o)

	q.EXPECT().GetACOverride(ctx, int64(9)).Return(queries.GetACOverrideRow{}, sql.ErrNoRows).Once()
	_, err = dal.GetACOverride(ctx, 9)
	require.ErrorIs(t, err, domain.ErrCharacterNotFound)
}

// ADR 0033 step 13: setting writes the value and label, clearing (nil)
// writes NULL and no label; no row is ErrCharacterNotFound.
func TestSetACOverride_WritesOrClears(t *testing.T) {
	ctx := context.Background()
	q := mocks.NewMockQuerier(t)
	dal := characterdal.New(q)
	q.EXPECT().SetACOverride(ctx, queries.SetACOverrideParams{
		AcOverride: sql.NullInt32{Int32: 4, Valid: true}, AcOverrideLabel: "Shield", ID: 3,
	}).Return(1, nil).Once()
	require.NoError(t, dal.SetACOverride(ctx, 3, &acsvc.Override{Value: 4, Label: "Shield"}))

	q.EXPECT().SetACOverride(ctx, queries.SetACOverrideParams{ID: 3}).Return(1, nil).Once()
	require.NoError(t, dal.SetACOverride(ctx, 3, nil))

	q.EXPECT().SetACOverride(ctx, queries.SetACOverrideParams{ID: 9}).Return(0, nil).Once()
	require.ErrorIs(t, dal.SetACOverride(ctx, 9, nil), domain.ErrCharacterNotFound)
}
