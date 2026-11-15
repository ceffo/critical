package composition_test

// ADR 0029 step 11 end to end over the in-memory storage: three users, any of
// whom reads another's character and none of whom changes it.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

// viewing is call from a page that views the character viewed.
func (tb table) viewing(token string, viewed int64, method, path, body string) *httptest.ResponseRecorder {
	tb.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: token})
	req.Header.Set(mw.ViewedHeader, itoa(viewed))
	rec := httptest.NewRecorder()
	tb.h.ServeHTTP(rec, req)
	return rec
}

type othersView struct {
	Characters []struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Owner struct {
			ID string `json:"id"`
		} `json:"owner"`
	} `json:"characters"`
}

type meView struct {
	CurrentCharacterID int64 `json:"currentCharacterId"`
}

// Steps 11.1 to 11.3 with two players: Ann (the root admin, who has Kelsen)
// reads Bob's Bram on every GET route, writes to none, and neither Bram's
// state nor Ann's current character moves.
func TestViewed_Step11_AnnReadsBobsCharacterAndChangesNothing(t *testing.T) {
	tb, m := newTable(t)
	ctx := context.Background()
	ann, _ := tb.login("")
	bob, bobID := tb.login("bob@example.com")
	bram, err := m.AddCharacterOf(ctx, bobID, charactersvc.Character{
		Name: "Bram", Class: "fighter", Str: 16, Dex: 12, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 30,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, tb.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(bram)+`}`, nil))
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/hp/event", `{"delta":-7,"source":"orc"}`, nil))

	var before struct {
		HP    hpView
		Round roundView
		Log   logView
		Me    meView
	}
	snapshot := func(token string) {
		require.Equal(t, http.StatusOK, tb.call(token, "GET", "/api/hp", "", &before.HP))
		require.Equal(t, http.StatusOK, tb.call(token, "GET", "/api/round", "", &before.Round))
		require.Equal(t, http.StatusOK, tb.call(token, "GET", "/api/log", "", &before.Log))
	}
	snapshot(bob)
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/auth/me", "", &before.Me))
	require.NotZero(t, before.Me.CurrentCharacterID)

	// The sheet and the action log are Bram's, whoever asks.
	rec := tb.viewing(ann, bram, "GET", "/api/character", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"name":"Bram"`)
	rec = tb.viewing(ann, bram, "GET", "/api/log", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "orc", "Bob's journal text is read, as decided")

	// Every GET route answers for a viewed character (none is refused as read-only
	// or unauthorised), and every write is refused before it changes anything.
	routes, ok := tb.h.(chi.Routes)
	require.True(t, ok)
	reads, writes := 0, 0
	require.NoError(t, chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, "/api/") || strings.HasPrefix(route, "/api/auth/") {
			return nil
		}
		path := strings.NewReplacer("{id}", "1", "{key}", "x", "{code}", "x").Replace(route)
		rec := tb.viewing(ann, bram, method, path, "{}")
		if method == http.MethodGet {
			reads++
			assert.Less(t, rec.Code, 500, method+" "+route)
			assert.NotContains(t, rec.Body.String(), `"code":"read_only"`, method+" "+route)
			assert.NotEqual(t, http.StatusForbidden, rec.Code, method+" "+route)
			return nil
		}
		writes++
		assert.Equal(t, http.StatusForbidden, rec.Code, method+" "+route)
		assert.Contains(t, rec.Body.String(), `"code":"read_only"`, method+" "+route)
		return nil
	}))
	assert.Greater(t, reads, 25)
	assert.Greater(t, writes, 40)

	// Bram is exactly as Bob left him, and Ann still plays Kelsen.
	var after struct {
		HP    hpView
		Round roundView
		Log   logView
		Me    meView
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/hp", "", &after.HP))
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/round", "", &after.Round))
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/log", "", &after.Log))
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/auth/me", "", &after.Me))
	assert.Equal(t, before.HP, after.HP)
	assert.Equal(t, before.Round, after.Round)
	assert.Equal(t, before.Log, after.Log)
	assert.Equal(t, before.Me, after.Me, "viewing never changes the current character")

	// Bob cannot be made to write by Ann's session, nor Ann's by Bob's.
	assert.Equal(t, http.StatusForbidden, tb.viewing(ann, bram, "POST", "/api/hp/event", `{"delta":-5,"source":"x"}`).Code)
}

// Step 11.2 and 8.1: the list is the other users' characters not retired, an
// enabled owner's; an unknown, retired or disabled owner's id is 404.
func TestViewed_Step11_ListAndUnreadableCharacters(t *testing.T) {
	tb, m := newTable(t)
	ctx := context.Background()
	ann, annID := tb.login("")
	bob, bobID := tb.login("bob@example.com")
	_, cyID := tb.login("cy@example.com")
	bram, err := m.AddCharacterOf(ctx, bobID, charactersvc.Character{Name: "Bram", Class: "fighter", Dex: 10, Wis: 10, HPMax: 30})
	require.NoError(t, err)
	old, err := m.AddCharacterOf(ctx, bobID, charactersvc.Character{Name: "Old", Class: "thief", Dex: 10, Wis: 10, HPMax: 6})
	require.NoError(t, err)
	require.NoError(t, m.RetireCharacter(ctx, old))
	cyChar, err := m.AddCharacterOf(ctx, cyID, charactersvc.Character{Name: "Cyra", Class: "thief", Dex: 10, Wis: 10, HPMax: 6})
	require.NoError(t, err)

	var list othersView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/characters/others", "", &list))
	names := map[string]string{}
	for _, c := range list.Characters {
		names[c.Name] = c.Owner.ID
	}
	assert.Equal(t, map[string]string{"Bram": bobID.String(), "Cyra": cyID.String()}, names,
		"others only: not Ann's Kelsen, not the retired Old")

	// Cy is disabled: the character leaves the list and cannot be read.
	require.Equal(t, http.StatusOK, tb.call(ann, "PATCH", "/api/admin/users/"+cyID.String(), `{"disabled":true}`, nil))
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/characters/others", "", &list))
	require.Len(t, list.Characters, 1)
	assert.Equal(t, "Bram", list.Characters[0].Name)
	for _, id := range []int64{cyChar, old, 999} {
		rec := tb.viewing(ann, id, "GET", "/api/character", "")
		assert.Equal(t, http.StatusNotFound, rec.Code, id)
		assert.Contains(t, rec.Body.String(), `"code":"character_not_found"`, id)
	}

	// Bob sees Ann's character, not his own; no session sees nothing.
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters/others", "", &list))
	require.Len(t, list.Characters, 1)
	assert.Equal(t, annID.String(), list.Characters[0].Owner.ID)
	assert.Equal(t, http.StatusUnauthorized, tb.call("", "GET", "/api/characters/others", "", nil))
	assert.Equal(t, http.StatusOK, tb.viewing(bob, bram, "GET", "/api/character", "").Code, "one's own character is a view too")
}
