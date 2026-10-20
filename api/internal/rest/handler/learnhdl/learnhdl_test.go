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

// ADR 0030 step 20.8: with grantLevel, the add claims that level's free
// spell and answers the entry's origin level_grant with its level.
func TestAdd_Step20_8_GrantLevelClaimsTheFreeSpell(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().AddFreeSpell(mock.Anything, int64(3), "web", 3).
		Return(&learnsvc.Added{SpellID: "web", SpellLevel: 2, Origin: spellbook.OriginLevelGrant, GrantLevel: 3}, nil).Once()
	rec, err := post(t, learnhdl.Add(func(*http.Request) int64 { return 3 }, svc), `{"spellId":"web","grantLevel":3}`)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"spellId":"web","spellLevel":2,"origin":"level_grant","grantLevel":3}`, rec.Body.String())
}

// A body without a spell, with a negative grant level or with a field the
// endpoint does not take, is refused before the service.
func TestAdd_RefusesABadBody(t *testing.T) {
	for _, body := range []string{`{}`, `{"spellId":"sleep","grantLevel":-1}`, `{"spellId":"sleep","origin":"given"}`, `not json`} {
		_, err := post(t, learnhdl.Add(func(*http.Request) int64 { return 3 }, mocks.NewMockSVC(t)), body)
		require.Error(t, err, body)
	}
}
