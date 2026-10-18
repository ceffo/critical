package composition_test

// ADR 0029 step 7.2 across servers and restarts, over the in-memory storage:
// production runs two servers over one database, each remembering sessions
// for up to authsvc.SessionCacheTTL.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// Step 7.2: a switch made on server 1 while server 2 still remembers the
// previous character. The web names the character it shows: server 2 reads
// the current character again and the write lands on the one on screen; a
// page still showing the previous one is refused (409 character_changed) and
// writes nothing.
func TestSwitch_Step7_2_AWriteOnTheOtherServerLandsOnTheShownCharacter(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	one, _ := serverOn(t, m)
	two, _ := serverOn(t, m)
	bob, bobID := one.login("bob@example.com")
	bram, err := m.AddCharacterOf(ctx, bobID, charactersvc.Character{Name: "Bram", Class: "fighter", HPMax: 30})
	require.NoError(t, err)
	bart, err := m.AddCharacterOf(ctx, bobID, charactersvc.Character{Name: "Bart", Class: "thief", HPMax: 12})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, one.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(bram)+`}`, nil))

	// Server 2 remembers Bob's session with Bram current.
	var sheet sheetView
	require.Equal(t, http.StatusOK, two.showing(bob, bram, "GET", "/api/character", "", &sheet))
	require.Equal(t, "Bram", sheet.Name)

	// The switch, on server 1; the page now shows Bart.
	require.Equal(t, http.StatusOK, one.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(bart)+`}`, nil))
	var hp hpView
	require.Equal(t, http.StatusOK, two.showing(bob, bart, "POST", "/api/hp/event", `{"delta":-5,"source":"trap"}`, &hp))
	assert.Equal(t, 7, hp.CurrentHp, "Bart's 12 HP, not Bram's 30")
	require.Equal(t, http.StatusOK, two.showing(bob, bart, "GET", "/api/character", "", &sheet))
	assert.Equal(t, "Bart", sheet.Name, "server 2 reads Bart from now on")

	// A tab still showing Bram is refused on either server, and Bram is untouched.
	var refused map[string]string
	for _, srv := range []table{one, two} {
		assert.Equal(t, http.StatusConflict, srv.showing(bob, bram, "POST", "/api/hp/event", `{"delta":-9,"source":"orc"}`, &refused))
		assert.Equal(t, "character_changed", refused["code"])
		assert.Equal(t, http.StatusConflict, srv.showing(bob, bram, "GET", "/api/hp", "", &refused), "a read too")
	}
	require.Equal(t, http.StatusOK, one.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(bram)+`}`, nil))
	require.Equal(t, http.StatusOK, one.showing(bob, bram, "GET", "/api/hp", "", &hp))
	assert.Equal(t, 30, hp.CurrentHp, "no write reached Bram")

	// /api/auth/me, which the page reads once at load, is the database's
	// current character on a server that remembered another.
	var me struct {
		CurrentCharacterID *int64 `json:"currentCharacterId"`
	}
	require.Equal(t, http.StatusOK, two.call(bob, "GET", "/api/auth/me", "", &me))
	require.NotNil(t, me.CurrentCharacterID)
	assert.Equal(t, bram, *me.CurrentCharacterID)

	// A character id that is no number is a bad request.
	req := httptest.NewRequest("GET", "/api/hp", nil)
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: bob})
	req.Header.Set(mw.CharacterHeader, "x")
	rec := httptest.NewRecorder()
	two.h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"invalid_input"`)
}

// Step 7.2: a user whose current character was retired, with others: every
// per-character route answers no_character, the list offers the others with no
// current one, and choosing one plays it. The root admin's is repaired by the
// next start (the adoption), which picks their first character not retired.
func TestSwitch_Step7_2_ARetiredCurrentCharacter(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	one, _ := serverOn(t, m)
	bob, bobID := one.login("bob@example.com")
	bram, err := m.AddCharacterOf(ctx, bobID, charactersvc.Character{Name: "Bram", Class: "fighter", HPMax: 30})
	require.NoError(t, err)
	bart, err := m.AddCharacterOf(ctx, bobID, charactersvc.Character{Name: "Bart", Class: "thief", HPMax: 12})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, one.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(bram)+`}`, nil))
	require.NoError(t, m.RetireCharacter(ctx, bram))
	one, _ = serverOn(t, m) // a server that does not remember Bob's session

	var refused map[string]string
	assert.Equal(t, http.StatusNotFound, one.call(bob, "GET", "/api/character", "", &refused))
	assert.Equal(t, "no_character", refused["code"])
	var list listView
	require.Equal(t, http.StatusOK, one.call(bob, "GET", "/api/characters", "", &list))
	require.Len(t, list.Characters, 1)
	assert.Equal(t, bart, list.Characters[0].ID)
	assert.Nil(t, list.CurrentID)
	require.Equal(t, http.StatusOK, one.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(bart)+`}`, nil))
	var sheet sheetView
	require.Equal(t, http.StatusOK, one.call(bob, "GET", "/api/character", "", &sheet))
	assert.Equal(t, "Bart", sheet.Name)

	// The root admin: Kelsen (the seeded character) retired, another one owned.
	root, rootID := one.login("")
	var rootList listView
	require.Equal(t, http.StatusOK, one.call(root, "GET", "/api/characters", "", &rootList))
	require.NotNil(t, rootList.CurrentID)
	kelsen := *rootList.CurrentID
	second, err := m.AddCharacterOf(ctx, rootID, charactersvc.Character{Name: "Second", Class: "mage", HPMax: 4})
	require.NoError(t, err)
	require.NoError(t, m.RetireCharacter(ctx, kelsen))
	again, _ := serverOn(t, m)
	require.Equal(t, http.StatusOK, again.call(root, "GET", "/api/characters", "", &rootList))
	require.NotNil(t, rootList.CurrentID, "the start repaired it")
	assert.Equal(t, second, *rootList.CurrentID)
}

// Step 5.3: a root admin set with a typo got Kelsen; set again with the right
// email, the new root owns Kelsen and plays it, and the disabled typo row
// keeps nothing.
func TestSwitch_Step5_3_ATypoRootHandsItsCharactersOver(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	_, bs := serverOn(t, m)
	typo, err := bs.Auth.SetRootAdmin(ctx, "dm@exmaple.com")
	require.NoError(t, err)
	require.NotZero(t, typo.CurrentCharacterID, "the typo root got Kelsen")
	kelsen := typo.CurrentCharacterID

	right, err := bs.Auth.SetRootAdmin(ctx, "dm@example.com")
	require.NoError(t, err)
	assert.True(t, right.Root)
	assert.Equal(t, kelsen, right.CurrentCharacterID, "Kelsen is the real root's current character")
	mine, err := bs.Character.List(ctx, right.ID)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, kelsen, mine[0].ID)

	old, err := m.Auth().UserByID(ctx, typo.ID)
	require.NoError(t, err)
	assert.True(t, old.Disabled)
	assert.False(t, old.Root)
	assert.Zero(t, old.CurrentCharacterID)
	theirs, err := bs.Character.List(ctx, typo.ID)
	require.NoError(t, err)
	assert.Empty(t, theirs)
}

// Step 5.3: a start without a root admin leaves the characters without an
// owner; the first start with one gives them to the root admin, current; a
// second start, or a second server, changes nothing.
func TestSwitch_Step5_3_StartsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	_, bs := serverOn(t, m)
	_, err := m.Auth().RootUser(ctx)
	require.Error(t, err, "no root admin yet")

	// A root admin written straight to the store (no adoption of its own).
	root := authsvc.User{ID: uuid.New(), Email: "dm@example.com", Role: authsvc.RoleAdmin, Root: true, CreatedAt: time.Now()}
	require.NoError(t, m.Auth().InsertUser(ctx, root))
	mine, err := bs.Character.List(ctx, root.ID)
	require.NoError(t, err)
	assert.Empty(t, mine, "the start without a root admin gave the characters to no one")

	for range 3 {
		serverOn(t, m)
		mine, err = bs.Character.List(ctx, root.ID)
		require.NoError(t, err)
		require.Len(t, mine, 1, "Kelsen, once")
		u, err := m.Auth().UserByID(ctx, root.ID)
		require.NoError(t, err)
		assert.Equal(t, mine[0].ID, u.CurrentCharacterID)
	}
}
