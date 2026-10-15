package detailssvc_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type inlineTx struct{}

func (inlineTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func ptr[T any](v T) *T { return &v }

var stored = charactersvc.Details{Race: "Elf", Age: 100, Languages: []string{"Elfique", "Commun"}}

func newSvc(t *testing.T) (detailssvc.SVC, *mocks.MockDAL) {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	svc, err := detailssvc.New(dal, inlineTx{})
	require.NoError(t, err)
	return svc, dal
}

func TestNew_NilDependency(t *testing.T) {
	_, err := detailssvc.New(nil, inlineTx{})
	require.Error(t, err)
}

func TestUpdate_AppliesOnlyThePresentFieldsTrimmed(t *testing.T) {
	svc, dal := newSvc(t)
	want := charactersvc.Details{Race: "Half-elf", Age: 100, Eyes: "green", Languages: []string{"Elfique", "Commun"}}
	dal.EXPECT().GetDetails(mockAny, int64(1)).Return(stored, nil)
	dal.EXPECT().UpdateDetails(mockAny, int64(1), want).Return(nil)
	got, err := svc.Update(context.Background(), 1, detailssvc.Patch{Race: ptr("  Half-elf "), Eyes: ptr("green")})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestUpdate_ClearsTextAgeAndLanguages(t *testing.T) {
	svc, dal := newSvc(t)
	want := charactersvc.Details{Languages: []string{}}
	dal.EXPECT().GetDetails(mockAny, int64(1)).Return(stored, nil)
	dal.EXPECT().UpdateDetails(mockAny, int64(1), want).Return(nil)
	got, err := svc.Update(context.Background(), 1, detailssvc.Patch{Race: ptr(""), Age: ptr(0), Languages: &[]string{}})
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
		{"race too long", detailssvc.Patch{Race: ptr(long(41))}, "race", detailssvc.CodeTooLong},
		{"alignment too long", detailssvc.Patch{Alignment: ptr(long(41))}, "alignment", detailssvc.CodeTooLong},
		{"sex too long", detailssvc.Patch{Sex: ptr(long(41))}, "sex", detailssvc.CodeTooLong},
		{"height too long", detailssvc.Patch{Height: ptr(long(21))}, "height", detailssvc.CodeTooLong},
		{"weight too long", detailssvc.Patch{Weight: ptr(long(21))}, "weight", detailssvc.CodeTooLong},
		{"hair too long", detailssvc.Patch{Hair: ptr(long(41))}, "hair", detailssvc.CodeTooLong},
		{"eyes too long", detailssvc.Patch{Eyes: ptr(long(41))}, "eyes", detailssvc.CodeTooLong},
		{"negative age", detailssvc.Patch{Age: ptr(-1)}, "age", detailssvc.CodeOutOfRange},
		{"huge age", detailssvc.Patch{Age: ptr(10_001)}, "age", detailssvc.CodeOutOfRange},
		{"blank language", detailssvc.Patch{Languages: &[]string{"Commun", "  "}}, "languages", detailssvc.CodeEmpty},
		{"language too long", detailssvc.Patch{Languages: &[]string{long(31)}}, "languages", detailssvc.CodeTooLong},
		{"duplicate language ignoring case", detailssvc.Patch{Languages: &[]string{"Nain", " nain"}}, "languages", detailssvc.CodeDuplicate},
		{"too many languages", detailssvc.Patch{Languages: func() *[]string {
			l := make([]string, 21)
			for i := range l {
				l[i] = strings.Repeat("a", i+1)
			}
			return &l
		}()}, "languages", detailssvc.CodeTooMany},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, dal := newSvc(t)
			dal.EXPECT().GetDetails(mockAny, int64(1)).Return(stored, nil)
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
	dal.EXPECT().GetDetails(mockAny, int64(1)).Return(stored, nil)
	dal.EXPECT().UpdateDetails(mockAny, int64(1), mockAnyDetails).Return(nil)
	_, err := svc.Update(context.Background(), 1, detailssvc.Patch{
		Race: ptr(strings.Repeat("é", 40)), Height: ptr(strings.Repeat("é", 20)), Age: ptr(10_000),
		Languages: &[]string{strings.Repeat("é", 30)},
	})
	require.NoError(t, err)
}

func TestUpdate_UnknownCharacter(t *testing.T) {
	svc, dal := newSvc(t)
	dal.EXPECT().GetDetails(mockAny, int64(9)).Return(charactersvc.Details{}, domain.ErrCharacterNotFound)
	_, err := svc.Update(context.Background(), 9, detailssvc.Patch{Race: ptr("Elf")})
	assert.ErrorIs(t, err, domain.ErrCharacterNotFound)
}

var (
	mockAny        = mock.Anything
	mockAnyDetails = mock.Anything // the stored value is checked elsewhere
)
