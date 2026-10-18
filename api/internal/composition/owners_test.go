package composition_test

// ADR 0029 step 7 end to end over the in-memory storage: two users, each with
// a character, play through the router and the real services; every route a
// test sends acts on the caller's current character only.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
	"github.com/ceffo/critical/api/internal/rest/router"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// table is the router over the root over m, and the users who log in on it.
type table struct {
	t *testing.T
	h http.Handler
}

// call sends a request with the session token (empty: none) and decodes the
// JSON answer into out (nil: ignored); it returns the status.
func (tb table) call(token, method, path, body string, out any) int {
	tb.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: token})
	}
	rec := httptest.NewRecorder()
	tb.h.ServeHTTP(rec, req)
	if out != nil {
		require.NoError(tb.t, json.Unmarshal(rec.Body.Bytes(), out), rec.Body.String())
	}
	return rec.Code
}

// login is the dev login of email (empty: the root admin): the session token
// and the user.
func (tb table) login(email string) (string, uuid.UUID) {
	tb.t.Helper()
	req := httptest.NewRequest("POST", "/api/auth/dev", strings.NewReader(`{"email":"`+email+`"}`))
	rec := httptest.NewRecorder()
	tb.h.ServeHTTP(rec, req)
	require.Equal(tb.t, http.StatusOK, rec.Code, rec.Body.String())
	var me struct {
		ID string `json:"id"`
	}
	require.NoError(tb.t, json.Unmarshal(rec.Body.Bytes(), &me))
	for _, c := range rec.Result().Cookies() {
		if c.Name == mw.SessionCookie {
			return c.Value, uuid.MustParse(me.ID)
		}
	}
	tb.t.Fatal("no session cookie")
	return "", uuid.Nil
}

type sheetView struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type hpView struct {
	CurrentHp int `json:"currentHp"`
}

type roundView struct {
	Round int `json:"round"`
}

type logView struct {
	Entries []struct {
		ID int64 `json:"id"`
	} `json:"entries"`
}

type listView struct {
	Characters []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"characters"`
	CurrentID *int64 `json:"currentId"`
}

func newTable(t *testing.T) (table, *memory.Store) {
	t.Helper()
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return table{t: t, h: router.New(log, bs, router.Options{})}, m
}

// Step 7.1: Ann (the root admin, who got the seeded character) and Bob (a new
// player, given a character of his own) each read and write only their
// current character: Bob's damage, round and log entries never reach Ann's
// sheet, and Ann's undo never pops Bob's entry.
func TestOwners_Step7_1_EachUserPlaysOnlyTheirCurrentCharacter(t *testing.T) {
	tb, m := newTable(t)
	ctx := context.Background()
	ann, _ := tb.login("")
	bob, bobID := tb.login("bob@example.com")

	// Bob has no character yet: the stable code the web turns into "create
	// your first character" (step 7.2), on every per-character route.
	var refused map[string]string
	assert.Equal(t, http.StatusNotFound, tb.call(bob, "GET", "/api/character", "", &refused))
	assert.Equal(t, "no_character", refused["code"])
	var none listView
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters", "", &none))
	assert.Empty(t, none.Characters)
	assert.Nil(t, none.CurrentID)

	bram, err := m.AddCharacterOf(ctx, bobID, charactersvc.Character{
		Name: "Bram", Class: "fighter", Level: 3, Str: 16, Dex: 12, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 30, Thac0Base: 18,
	})
	require.NoError(t, err)
	var list listView
	require.Equal(t, http.StatusOK, tb.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(bram)+`}`, &list))
	require.NotNil(t, list.CurrentID)
	assert.Equal(t, bram, *list.CurrentID)

	var annSheet, bobSheet sheetView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/character", "", &annSheet))
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/character", "", &bobSheet))
	assert.Equal(t, "Test Bard", annSheet.Name)
	assert.Equal(t, "Bram", bobSheet.Name, "the switch holds at once on this server")

	var annHP, bobHP hpView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/hp", "", &annHP))
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/hp/event", `{"delta":-7,"source":"orc"}`, &bobHP))
	assert.Equal(t, 23, bobHP.CurrentHp)
	var annAfter hpView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/hp", "", &annAfter))
	assert.Equal(t, annHP.CurrentHp, annAfter.CurrentHp, "Bob's damage is not Ann's")

	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/round/advance", "", nil))
	var annRound, bobRound roundView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/round", "", &annRound))
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/round", "", &bobRound))
	assert.Equal(t, 0, annRound.Round)
	assert.Equal(t, 1, bobRound.Round)

	var annLog, bobLog logView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/log", "", &annLog))
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/log", "", &bobLog))
	assert.Empty(t, annLog.Entries, "Bob's entries are not in Ann's log")
	bobEntries := len(bobLog.Entries)
	assert.GreaterOrEqual(t, bobEntries, 2, "the damage and the round")

	// Ann's undo sees her own (empty) stack: Bob's top entry is no one else's
	// to pop.
	assert.NotEqual(t, http.StatusOK, tb.call(ann, "POST", "/api/log/undo",
		`{"expectedTopId":`+itoa(bobLog.Entries[0].ID)+`}`, nil))
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/log", "", &bobLog))
	assert.Len(t, bobLog.Entries, bobEntries)

	var annWeapons, bobWeapons []any
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/weapons", "", &annWeapons))
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/weapons", "", &bobWeapons))
	assert.NotEmpty(t, annWeapons, "the seeded character's weapons")
	assert.Empty(t, bobWeapons)
}

// Step 7.1: selecting another user's character answers 404, as an unknown id,
// and changes nothing; step 7.3: a retired character is not listed and cannot
// be selected.
func TestOwners_Step7_1_AnotherUsersCharacterIsNotFound(t *testing.T) {
	tb, m := newTable(t)
	ctx := context.Background()
	ann, _ := tb.login("")
	bob, bobID := tb.login("bob@example.com")
	var annList listView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/characters", "", &annList))
	require.Len(t, annList.Characters, 1)
	kelsen := annList.Characters[0].ID

	var refused map[string]string
	assert.Equal(t, http.StatusNotFound, tb.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(kelsen)+`}`, &refused))
	assert.Equal(t, "character_not_found", refused["code"])
	assert.Equal(t, http.StatusNotFound, tb.call(bob, "PUT", "/api/characters/current", `{"id":999}`, &refused))
	assert.Equal(t, "character_not_found", refused["code"], "the same answer as an unknown id")
	assert.Equal(t, http.StatusNotFound, tb.call(bob, "GET", "/api/character", "", &refused))
	assert.Equal(t, "no_character", refused["code"], "nothing changed")

	bram, err := m.AddCharacterOf(ctx, bobID, charactersvc.Character{Name: "Bram", Class: "fighter", Level: 3, HPMax: 30})
	require.NoError(t, err)
	old, err := m.AddCharacterOf(ctx, bobID, charactersvc.Character{Name: "Old", Class: "thief", Level: 1, HPMax: 6})
	require.NoError(t, err)
	require.NoError(t, m.RetireCharacter(ctx, old))
	var bobList listView
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters", "", &bobList))
	require.Len(t, bobList.Characters, 1, "the retired one is not listed")
	assert.Equal(t, bram, bobList.Characters[0].ID)
	assert.Equal(t, http.StatusNotFound, tb.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(old)+`}`, nil))

	// Ann's list and current character are untouched.
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/characters", "", &annList))
	require.NotNil(t, annList.CurrentID)
	assert.Equal(t, kelsen, *annList.CurrentID)
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
