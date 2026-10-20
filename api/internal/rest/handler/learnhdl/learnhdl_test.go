package learnhdl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
	"github.com/ceffo/critical/api/internal/rest/handler/learnhdl"
)

func post(t *testing.T, h func(http.ResponseWriter, *http.Request) error, body string) (*httptest.ResponseRecorder, error) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/character/spellbook/add", strings.NewReader(body))
	return rec, h(rec, req)
}

// ADR 0030 step 20.6: POST /api/character/spellbook/add {spellId} answers the
// entry the add wrote: the spell, its level and its origin (step 20.5).
func TestAdd_AnswersTheAddedEntry(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().Add(mock.Anything, int64(3), "sleep").
		Return(&learnsvc.Added{SpellID: "sleep", SpellLevel: 1, Origin: spellbook.OriginGiven}, nil).Once()
	rec, err := post(t, learnhdl.Add(func(*http.Request) int64 { return 3 }, svc), `{"spellId":"sleep"}`)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"spellId":"sleep","spellLevel":1,"origin":"given"}`, rec.Body.String())
}

// A body without a spell, or with a field the endpoint does not take, is
// refused before the service.
func TestAdd_RefusesABadBody(t *testing.T) {
	for _, body := range []string{`{}`, `{"spellId":"sleep","grantLevel":5}`, `not json`} {
		_, err := post(t, learnhdl.Add(func(*http.Request) int64 { return 3 }, mocks.NewMockSVC(t)), body)
		require.Error(t, err, body)
	}
}
