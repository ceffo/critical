package router_test

// ADR 0029 step 11: any user reads any character, the owner alone plays it. A
// request names a viewed character in mw.ViewedHeader.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

// asViewing is as with the viewed character named.
func asViewing(h http.Handler, token, viewed, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: token})
	req.Header.Set(mw.ViewedHeader, viewed)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func viewerRouter(t *testing.T) (http.Handler, *mocksSet) {
	t.Helper()
	return newRouterWith(t, usersAuth(t, map[string]authsvc.User{
		"ann": {ID: annID, Email: "ann@example.com", Role: authsvc.RolePlayer, CurrentCharacterID: 3},
	}))
}

// Step 11.3: every route that changes something is refused for a request that
// names a viewed character, walked from the router so a new route is covered.
// The strict mocks expect no call: the refusal comes before any service, the
// lookup of the viewed character included.
func TestViewedCharacter_Step11_3_EveryWriteIsRefused(t *testing.T) {
	h, _ := viewerRouter(t)
	routes, ok := h.(chi.Routes)
	require.True(t, ok)
	refused := 0
	require.NoError(t, chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, "/api/") || strings.HasPrefix(route, "/api/auth/") || method == http.MethodGet || method == http.MethodHead {
			return nil
		}
		refused++
		rec := asViewing(h, "ann", "8", method, urlParam.ReplaceAllString(route, "1"), "{}")
		assert.Equal(t, http.StatusForbidden, rec.Code, method+" "+route)
		assert.JSONEq(t, `{"error":"a viewed character is read-only","code":"read_only"}`, rec.Body.String(), method+" "+route)
		return nil
	}))
	assert.Greater(t, refused, 40, "the walk found the write routes")
}

// Step 11.1: a viewed character is read whoever owns it, and the read is of
// that character, not of the user's current one (3).
func TestViewedCharacter_Step11_1_ReadsAnotherUsersCharacter(t *testing.T) {
	h, m := viewerRouter(t)
	m.char.EXPECT().CanView(mock.Anything, int64(8)).Return(nil).Once()
	m.char.EXPECT().GetCharacter(mock.Anything, int64(8)).Return(&charactersvc.Character{ID: 8, Name: "Bram"}, nil).Once()

	// A stale X-Character-Id plays no part in a view.
	req := httptest.NewRequest(http.MethodGet, "/api/character", nil)
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: "ann"})
	req.Header.Set(mw.ViewedHeader, "8")
	req.Header.Set(mw.CharacterHeader, "99")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"name":"Bram"`)
}

// Step 11.1: a character that may not be read is 404 character_not_found and
// no service reads it; a value that is no id is a 400.
func TestViewedCharacter_Step11_1_UnreadableIsNotFound(t *testing.T) {
	h, m := viewerRouter(t)
	m.char.EXPECT().CanView(mock.Anything, int64(8)).Return(domain.ErrCharacterNotFound).Once()

	rec := asViewing(h, "ann", "8", http.MethodGet, "/api/character", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"character_not_found"`)

	for _, bad := range []string{"0", "-4", "abc"} {
		rec = asViewing(h, "ann", bad, http.MethodGet, "/api/character", "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, bad)
		assert.Contains(t, rec.Body.String(), `"code":"invalid_input"`, bad)
	}
}

// Nothing is read without a session, a viewed character or not.
func TestViewedCharacter_Step11_1_NeedsASession(t *testing.T) {
	h, _ := viewerRouter(t)
	for _, path := range []string{"/api/character", "/api/characters/others"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(mw.ViewedHeader, "8")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, path)
	}
}

// Step 11.2: GET /api/characters/others lists what the service says for the
// session's user, each with its owner and no email.
func TestCharacters_Step11_2_OthersListsWithOwners(t *testing.T) {
	h, m := viewerRouter(t)
	m.char.EXPECT().ListOthers(mock.Anything, annID).Return([]charactersvc.Shared{{
		Summary: charactersvc.Summary{
			ID: 8, Name: "Bram", Classes: []classes.Entry{classes.SingleEntry("thief", 2, 1250)}, Race: "elf",
			Missing: []string{}, CurrentHP: 9, MaxHP: 12,
		},
		OwnerID: bobID, OwnerName: "Bob",
	}}, nil).Once()

	rec := as(h, "ann", http.MethodGet, "/api/characters/others", "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"characters":[{"id":8,"name":"Bram","classes":[{"class":"thief","level":2,"startingXp":1250,"state":"active","leftAtLevel":0}],`+
		`"race":"elf","alignment":"","sex":"","missing":[],"currentHp":9,"maxHp":12,`+
		`"owner":{"id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","name":"Bob"}}]}`, rec.Body.String())
}
