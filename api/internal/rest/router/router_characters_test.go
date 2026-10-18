package router_test

// ADR 0029 step 7: the character a request acts on is its user's current
// character, resolved once by the router (mw.CurrentCharacter).

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	authmocks "github.com/ceffo/critical/api/internal/domain/service/authsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

var (
	annID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	bobID = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
)

// usersAuth is an auth service whose session tokens are the keys of users.
func usersAuth(t *testing.T, users map[string]authsvc.User) *authmocks.MockSVC {
	t.Helper()
	a := authmocks.NewMockSVC(t)
	a.EXPECT().DevLoginEnabled().Return(true).Maybe()
	for token, u := range users {
		a.EXPECT().Authenticate(mock.Anything, token).Return(&authsvc.Principal{User: u}, nil).Maybe()
	}
	return a
}

// as sends a request with the session token.
func as(h http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The routes a user without a current character still reaches: the login's,
// the health check, the reference data no character owns (step 7.5) and the
// user's characters.
var characterFree = map[string]bool{
	"GET /api/health": true, "GET /api/spells/filters": true, "GET /api/spells/names": true,
	"GET /api/crit-table": true, "PUT /api/crit-table": true, "GET /api/xp/preview": true,
	"GET /api/characters": true, "PUT /api/characters/current": true,
}

var urlParam = regexp.MustCompile(`\{[^}]+\}`)

// Step 7.2: every other route (walked from the router, so a new one is
// covered) acts on a current character: a user who has none is refused with
// no_character before any service runs (the strict mocks expect no call).
func TestCurrentCharacter_Step7_2_EveryCharacterRouteNeedsOne(t *testing.T) {
	h, _ := newRouterWith(t, usersAuth(t, map[string]authsvc.User{
		"new": {ID: annID, Email: "new@example.com", Role: authsvc.RolePlayer},
	}))
	routes, ok := h.(chi.Routes)
	require.True(t, ok)
	guarded := 0
	require.NoError(t, chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := method + " " + route
		if !strings.HasPrefix(route, "/api/") || strings.HasPrefix(route, "/api/auth/") || characterFree[key] {
			return nil
		}
		guarded++
		rec := as(h, "new", method, urlParam.ReplaceAllString(route, "1"), "{}")
		assert.Equal(t, http.StatusNotFound, rec.Code, key)
		assert.JSONEq(t, `{"error":"no current character: not found","code":"no_character"}`, rec.Body.String(), key)
		return nil
	}))
	assert.Greater(t, guarded, 50, "the walk found the character routes")
}

// Step 7.1: each user's request reaches the services with that user's
// current character, never another's or a fixed one.
func TestCurrentCharacter_Step7_1_EachUserActsOnTheirOwn(t *testing.T) {
	h, m := newRouterWith(t, usersAuth(t, map[string]authsvc.User{
		"ann": {ID: annID, Email: "ann@example.com", Role: authsvc.RolePlayer, CurrentCharacterID: 3},
		"bob": {ID: bobID, Email: "bob@example.com", Role: authsvc.RolePlayer, CurrentCharacterID: 8},
	}))
	m.char.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(&charactersvc.Character{ID: 3, Name: "Kelsen"}, nil).Once()
	m.char.EXPECT().GetCharacter(mock.Anything, int64(8)).Return(&charactersvc.Character{ID: 8, Name: "Bram"}, nil).Once()
	m.hp.EXPECT().AppendEvent(mock.Anything, int64(8), -4, "orc").Return(nil, nil).Once()

	ann := as(h, "ann", "GET", "/api/character", "")
	require.Equal(t, http.StatusOK, ann.Code)
	assert.Contains(t, ann.Body.String(), `"name":"Kelsen"`)
	bob := as(h, "bob", "GET", "/api/character", "")
	require.Equal(t, http.StatusOK, bob.Code)
	assert.Contains(t, bob.Body.String(), `"name":"Bram"`)
	assert.Equal(t, http.StatusOK, as(h, "bob", "POST", "/api/hp/event", `{"delta":-4,"source":"orc"}`).Code)
}

// GET /api/characters is the user's own list (the service is asked for the
// session's user only) with the current one's id; null without one.
func TestCharacters_ListIsTheUsersOwn(t *testing.T) {
	h, m := newRouterWith(t, usersAuth(t, map[string]authsvc.User{
		"ann": {ID: annID, Email: "ann@example.com", Role: authsvc.RolePlayer, CurrentCharacterID: 3},
		"new": {ID: bobID, Email: "new@example.com", Role: authsvc.RolePlayer},
	}))
	m.char.EXPECT().List(mock.Anything, annID).Return([]charactersvc.Summary{
		{ID: 3, Name: "Kelsen", Class: "bard", Level: 15}, {ID: 5, Name: "Vex", Class: "thief", Level: 2},
	}, nil).Once()
	m.char.EXPECT().List(mock.Anything, bobID).Return([]charactersvc.Summary{}, nil).Once()

	rec := as(h, "ann", "GET", "/api/characters", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"characters":[{"id":3,"name":"Kelsen","class":"bard","level":15},`+
		`{"id":5,"name":"Vex","class":"thief","level":2}],"currentId":3}`, rec.Body.String())

	rec = as(h, "new", "GET", "/api/characters", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"characters":[],"currentId":null}`, rec.Body.String())
}

// PUT /api/characters/current sets the session's user's current character
// and answers with the list; another user's character is the 404 of an
// unknown one (step 7.1); a body without an id is a 400.
func TestCharacters_SetCurrent(t *testing.T) {
	h, m := newRouterWith(t, usersAuth(t, map[string]authsvc.User{
		"ann": {ID: annID, Email: "ann@example.com", Role: authsvc.RolePlayer, CurrentCharacterID: 3},
	}))
	set := m.char.EXPECT().SetCurrent(mock.Anything, annID, int64(5)).Return(nil).Once()
	m.char.EXPECT().List(mock.Anything, annID).Return([]charactersvc.Summary{
		{ID: 3, Name: "Kelsen", Class: "bard", Level: 15}, {ID: 5, Name: "Vex", Class: "thief", Level: 2},
	}, nil).Once().NotBefore(set)
	rec := as(h, "ann", "PUT", "/api/characters/current", `{"id":5}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"currentId":5`)

	m.char.EXPECT().SetCurrent(mock.Anything, annID, int64(8)).Return(domain.ErrCharacterNotFound).Once()
	rec = as(h, "ann", "PUT", "/api/characters/current", `{"id":8}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"character not found","code":"character_not_found"}`, rec.Body.String())

	for _, body := range []string{`{}`, `{"id":"5"}`, `{"id":5,"x":1}`, `nope`} {
		rec = as(h, "ann", "PUT", "/api/characters/current", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}
