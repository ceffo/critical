package detailssvc_test

import (
	"context"
	"errors"
	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

type inlineTx struct{}

func (inlineTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func ptr[T any](v T) *T { return &v }

var stored = charactersvc.Details{Hair: "red", Age: 100, Languages: []string{"elvish", "common"}}

// bard and thief are the kinds of the characters read: elves, whose native language is elvish.
var (
	bard  = charactersvc.Kind{Class: "bard", Race: "elf"}
	thief = charactersvc.Kind{Class: "thief", Race: "elf"}
)

// phbLanguages is a small language catalogue with the PHB's elf, dwarf and human lists.
func phbLanguages(t *testing.T) *ruletablesvc.Languages {
	t.Helper()
	var list []ruletablesvc.Language
	for _, c := range []string{"common", "dwarvish", "elvish", "gnomish", "halfling", "goblin", "hobgoblin", "orcish", "gnoll", "kobold", "burrowers"} {
		list = append(list, ruletablesvc.Language{Code: c, Name: localized.Text{"en": c}})
	}
	rows := []ruletablesvc.RaceLanguageRow{
		{Race: "human", Language: "common", Native: true},
		{Race: "dwarf", Language: "dwarvish", Native: true},
		{Race: "dwarf", Language: "common"},
		{Race: "elf", Language: "elvish", Native: true},
		{Race: "elf", Language: "common"},
		{Race: "gnome", Language: "gnomish", Native: true},
		{Race: "half_elf", Language: "common", Native: true},
		{Race: "halfling", Language: "halfling", Native: true},
	}
	l, err := ruletablesvc.NewLanguages(list, rows, cataloguetest.Definitions(t))
	require.NoError(t, err)
	return l
}

func newSvc(t *testing.T) (detailssvc.SVC, *mocks.MockDAL) {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	svc, err := detailssvc.New(dal, inlineTx{}, phbLanguages(t), cataloguetest.Definitions(t))
	require.NoError(t, err)
	return svc, dal
}

func TestNew_NilDependency(t *testing.T) {
	_, err := detailssvc.New(nil, inlineTx{}, nil, nil)
	require.Error(t, err)
	_, err = detailssvc.New(mocks.NewMockDAL(t), nil, nil, nil)
	require.Error(t, err)
}

func TestUpdate_AppliesOnlyThePresentFieldsTrimmed(t *testing.T) {
	svc, dal := newSvc(t)
	want := charactersvc.Details{Hair: "Half-elf", Age: 100, Eyes: "green", Languages: []string{"elvish", "common"}}
	dal.EXPECT().GetDetails(mockAny, int64(1)).Return(stored, bard, nil)
	dal.EXPECT().UpdateDetails(mockAny, int64(1), want).Return(nil)
	got, err := svc.Update(context.Background(), 1, detailssvc.Patch{Hair: ptr("  Half-elf "), Eyes: ptr("green")})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestUpdate_ClearsTextAndAge(t *testing.T) {
	svc, dal := newSvc(t)
	want := charactersvc.Details{Languages: []string{"elvish", "common"}}
	dal.EXPECT().GetDetails(mockAny, int64(1)).Return(stored, bard, nil)
	dal.EXPECT().UpdateDetails(mockAny, int64(1), want).Return(nil)
	got, err := svc.Update(context.Background(), 1, detailssvc.Patch{Hair: ptr(""), ClearAge: true})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestUpdate_Validation(t *testing.T) {
	long := func(n int) string { return strings.Repeat("é", n) }
	tests := []struct {
		name  string
		patch detailssvc.Patch
		field string
		code  string
	}{
		{"zero height", detailssvc.Patch{Height: ptr(0.0)}, "height", charactersvc.CodeOutOfRange},
		{"negative height", detailssvc.Patch{Height: ptr(-170.0)}, "height", charactersvc.CodeOutOfRange},
		{"huge height", detailssvc.Patch{Height: ptr(300.1)}, "height", charactersvc.CodeOutOfRange},
		{"zero weight", detailssvc.Patch{Weight: ptr(0.0)}, "weight", charactersvc.CodeOutOfRange},
		{"huge weight", detailssvc.Patch{Weight: ptr(500.1)}, "weight", charactersvc.CodeOutOfRange},
		{"hair too long", detailssvc.Patch{Hair: ptr(long(41))}, "hair", charactersvc.CodeTooLong},
		{"eyes too long", detailssvc.Patch{Eyes: ptr(long(41))}, "eyes", charactersvc.CodeTooLong},
		{"zero age is not a clear", detailssvc.Patch{Age: ptr(0)}, "age", charactersvc.CodeOutOfRange},
		{"negative age", detailssvc.Patch{Age: ptr(-1)}, "age", charactersvc.CodeOutOfRange},
		{"huge age", detailssvc.Patch{Age: ptr(10_001)}, "age", charactersvc.CodeOutOfRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, dal := newSvc(t)
			dal.EXPECT().GetDetails(mockAny, int64(1)).Return(stored, bard, nil)
			_, err := svc.Update(context.Background(), 1, tt.patch)
			var fe *domain.FieldError
			require.True(t, errors.As(err, &fe), "got %v", err)
			assert.Equal(t, tt.field, fe.Field)
			assert.Equal(t, tt.code, fe.Message)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
		})
	}
}

func TestUpdate_AcceptsTheBounds(t *testing.T) {
	svc, dal := newSvc(t)
	dal.EXPECT().GetDetails(mockAny, int64(1)).Return(stored, bard, nil)
	dal.EXPECT().UpdateDetails(mockAny, int64(1), mockAnyDetails).Return(nil)
	_, err := svc.Update(context.Background(), 1, detailssvc.Patch{
		Hair: ptr(strings.Repeat("é", 40)), Height: ptr(300.0), Weight: ptr(500.0), Age: ptr(10_000),
	})
	require.NoError(t, err)
}

func TestUpdate_UnknownCharacter(t *testing.T) {
	svc, dal := newSvc(t)
	dal.EXPECT().GetDetails(mockAny, int64(9)).Return(charactersvc.Details{}, charactersvc.Kind{}, domain.ErrCharacterNotFound)
	_, err := svc.Update(context.Background(), 9, detailssvc.Patch{Hair: ptr("Elf")})
	assert.ErrorIs(t, err, domain.ErrCharacterNotFound)
}

var (
	mockAny        = mock.Anything
	mockAnyDetails = mock.Anything // the stored value is checked elsewhere
)

// ADR 0032 step 2, the edit path: the card's alignment edit applies the class limit of the
// character it reads (a bard: lawful good refused, lawful neutral accepted), the refusal writes
// nothing, and the sex is a code too (step 3).
func TestUpdate_Step2And3_AlignmentIsCheckedAgainstTheClassOnEdit(t *testing.T) {
	svc, dal := newSvc(t)
	dal.EXPECT().GetDetails(mockAny, int64(1)).Return(stored, bard, nil).Twice()
	_, err := svc.Update(context.Background(), 1, detailssvc.Patch{Alignment: ptr("lawful_good")})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "alignment", fe.Field)
	assert.Equal(t, charactersvc.CodeAlignmentNotAllowed, fe.Message)

	want := stored
	want.Alignment, want.Sex = "lawful_neutral", "female"
	dal.EXPECT().UpdateDetails(mockAny, int64(1), want).Return(nil).Once() // the one write
	got, err := svc.Update(context.Background(), 1, detailssvc.Patch{Alignment: ptr("lawful_neutral"), Sex: ptr("female")})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestUpdate_Step8_AKeptForbiddenAlignmentDoesNotBlockAnUnrelatedEdit(t *testing.T) {
	svc, dal := newSvc(t)
	kept := stored
	kept.Alignment, kept.Sex = "lawful_good", "male"
	dal.EXPECT().GetDetails(mockAny, int64(1)).Return(kept, thief, nil).Times(3)
	want := kept
	want.Hair = "black"
	dal.EXPECT().UpdateDetails(mockAny, int64(1), want).Return(nil).Twice()
	_, err := svc.Update(context.Background(), 1, detailssvc.Patch{Hair: ptr("black")})
	require.NoError(t, err)
	_, err = svc.Update(context.Background(), 1, detailssvc.Patch{Alignment: ptr("lawful_good"), Hair: ptr("black")})
	require.NoError(t, err, "the card's full submit sends the stored alignment")
	_, err = svc.Update(context.Background(), 1, detailssvc.Patch{Alignment: ptr("lawful_good2")})
	require.Error(t, err)
}

// ADR 0037 step 18: the languages are the Skills tab's (skillsvc); the card's edit leaves them as
// stored, and its answer reads them with the race's native language (ADR 0032 step 4.1), a
// character stored before it was added included.
func TestUpdate_Step18_TheLanguagesAreLeftAsStored(t *testing.T) {
	svc, dal := newSvc(t)
	held := charactersvc.Details{Hair: "red", Languages: []string{"common", "gnomish"}}
	dal.EXPECT().GetDetails(mockAny, int64(1)).Return(held, bard, nil).Once()
	want := held
	want.Hair = "brown"
	dal.EXPECT().UpdateDetails(mockAny, int64(1), want).Return(nil).Once()
	got, err := svc.Update(context.Background(), 1, detailssvc.Patch{Hair: ptr("brown")})
	require.NoError(t, err)
	assert.Equal(t, []string{"elvish", "common", "gnomish"}, got.Languages, "a character stored without its native language reads with it")
}
