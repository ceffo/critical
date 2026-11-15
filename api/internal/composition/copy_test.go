package composition_test

// ADR 0029 step 12 end to end over the in-memory storage: a user copies a
// character they own or one another user owns, and the copy is theirs, as the
// source stands now, with an empty log.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

type copiedView struct {
	listView
	CopiedID *int64 `json:"copiedId"`
}

type copyLogView struct {
	Entries []json.RawMessage `json:"entries"`
	TopID   int64             `json:"topId"`
}

// playedBram is Bob's Bram after a session: damage taken, XP awarded, a round
// passed, a sword carried and a dice favourite saved.
func playedBram(t *testing.T, tb table, m *memory.Store) (bob string, bram int64) {
	t.Helper()
	bob, _, bram = playedBramOf(t, tb, m)
	return bob, bram
}

// playedBramOf is playedBram with Bob's id too.
func playedBramOf(t *testing.T, tb table, m *memory.Store) (bob string, bobID uuid.UUID, bram int64) {
	t.Helper()
	bob, bobID = tb.login("bob@example.com")
	bram, err := m.AddCharacterOf(context.Background(), bobID, charactersvc.Character{
		Name: "Bram", Class: "fighter", Str: 16, Dex: 12, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 30,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, tb.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(bram)+`}`, nil))
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/hp/event", `{"delta":-7,"source":"orc"}`, nil))
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/hp/event", `{"delta":-4,"source":"orc again"}`, nil))
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/xp", `{"base":150,"reason":"the cellar"}`, nil))
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/round/advance", "", nil))
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/items", `{"catalogueCode":"long_sword","equip":true}`, nil))
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/dice/favourites", `{"formula":"2d6+1"}`, nil))
	return bob, bobID, bram
}

// read is a GET of path as token, decoded to a generic value.
func (tb table) read(token, path string) any {
	tb.t.Helper()
	var out any
	require.Equal(tb.t, http.StatusOK, tb.call(token, "GET", path, "", &out), path)
	return out
}

// withoutIdentity drops what a copy is allowed to differ in: its id and name.
func withoutIdentity(v any) any {
	m := map[string]any{}
	for k, e := range v.(map[string]any) {
		if k != "id" && k != "name" {
			m[k] = e
		}
	}
	return m
}

// Step 12.1 to 12.4: Ann copies Bob's played Bram. The copy is Ann's, named as
// the page said, not current; it has Bram's HP, XP, round, items and favourites
// and an empty log; Bram is unchanged.
func TestCopy_Step12_AnnCopiesBobsCharacterAsHerOwn(t *testing.T) {
	tb, m := newTable(t)
	bob, bram := playedBram(t, tb, m)
	ann, _ := tb.login("")
	var annList copiedView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/characters", "", &annList))
	annCurrent := *annList.CurrentID

	bramSheet, bramHP, bramXP := tb.read(bob, "/api/character"), tb.read(bob, "/api/hp"), tb.read(bob, "/api/xp")
	bramRound, bramItems, bramDice := tb.read(bob, "/api/round"), tb.read(bob, "/api/items"), tb.read(bob, "/api/dice/favourites")
	var bramLog copyLogView
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/log", "", &bramLog))
	require.NotEmpty(t, bramLog.Entries)

	var got copiedView
	require.Equal(t, http.StatusCreated, tb.call(ann, "POST", "/api/characters/"+itoa(bram)+"/copy", `{"name":"Bram (copy)"}`, &got))
	require.NotNil(t, got.CopiedID)
	copyID := *got.CopiedID
	assert.NotEqual(t, bram, copyID)
	assert.Equal(t, annCurrent, *got.CurrentID, "a copy is not made current")
	var names []string
	for _, c := range got.Characters {
		names = append(names, c.Name)
		assert.NotEqual(t, bram, c.ID, "Bram stays Bob's: the list is Ann's")
	}
	assert.Contains(t, names, "Bram (copy)")

	// Ann switches to the copy, then everything reads as Bram's, but the log.
	require.Equal(t, http.StatusOK, tb.call(ann, "PUT", "/api/characters/current", `{"id":`+itoa(copyID)+`}`, nil))
	assert.Equal(t, withoutIdentity(bramSheet), withoutIdentity(tb.read(ann, "/api/character")), "same sheet and derived figures")
	assert.Equal(t, bramRound, tb.read(ann, "/api/round"))
	assert.Equal(t, bramItems, tb.read(ann, "/api/items"))
	assert.Equal(t, stripIDs(bramDice), stripIDs(tb.read(ann, "/api/dice/favourites")))
	assert.Equal(t, stripIDs(bramXP), stripIDs(tb.read(ann, "/api/xp")), "the same awards, as the copy's own rows")
	var copyLog copyLogView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/log", "", &copyLog))
	assert.Empty(t, copyLog.Entries, "a copy keeps no history")
	var hp struct {
		CurrentHp, MaxHp int
	}
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/hp", "", &hp))
	assert.Equal(t, 19, hp.CurrentHp, "Bram's HP, with no entry behind it")
	assert.Equal(t, 30, hp.MaxHp)

	// Bram is as Bob left him.
	assert.Equal(t, bramSheet, tb.read(bob, "/api/character"))
	assert.Equal(t, bramHP, tb.read(bob, "/api/hp"))
	var after copyLogView
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/log", "", &after))
	assert.Equal(t, bramLog, after)
}

// Step 12.5: the copy is independent. Playing and undoing on it never touch
// Bram, and Bram's play never reaches the copy; the opening balance is not an
// entry, so undoing everything on the copy leaves its HP where the copy began.
func TestCopy_Step12_CopyAndSourceAreIndependent(t *testing.T) {
	tb, m := newTable(t)
	bob, bram := playedBram(t, tb, m)
	ann, _ := tb.login("")
	var got copiedView
	require.Equal(t, http.StatusCreated, tb.call(ann, "POST", "/api/characters/"+itoa(bram)+"/copy", `{"name":"Bram (copy)"}`, &got))
	require.Equal(t, http.StatusOK, tb.call(ann, "PUT", "/api/characters/current", `{"id":`+itoa(*got.CopiedID)+`}`, nil))
	hpOf := func(token string) int {
		var hp hpView
		require.Equal(t, http.StatusOK, tb.call(token, "GET", "/api/hp", "", &hp))
		return hp.CurrentHp
	}
	require.Equal(t, 19, hpOf(ann))

	// Damage on the copy: Bram keeps 19.
	require.Equal(t, http.StatusOK, tb.call(ann, "POST", "/api/hp/event", `{"delta":-5,"source":"trap"}`, nil))
	assert.Equal(t, 14, hpOf(ann))
	assert.Equal(t, 19, hpOf(bob))
	// Heal on Bram: the copy keeps 14.
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/hp/event", `{"delta":6,"source":"potion"}`, nil))
	assert.Equal(t, 25, hpOf(bob))
	assert.Equal(t, 14, hpOf(ann))
	// Undo on the copy pops only its own entry, and there is nothing more to undo:
	// its opening balance is no entry. Bram's log keeps its three.
	var copyLog copyLogView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/log", "", &copyLog))
	require.Len(t, copyLog.Entries, 1, "the trap alone: Bram's journal is not the copy's")
	require.Equal(t, http.StatusOK, tb.call(ann, "POST", "/api/log/undo", `{"expectedTopId":`+itoa(copyLog.TopID)+`}`, nil))
	assert.Equal(t, 19, hpOf(ann), "undo gives back the trap's damage, down to the copy's opening balance")
	assert.Equal(t, 25, hpOf(bob))
	var bobLog copyLogView
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/log", "", &bobLog))
	for _, e := range bobLog.Entries {
		assert.NotContains(t, string(e), `"status":"voided"`)
	}
}

// Step 12.1: the copy needs a readable source and a name. A retired character,
// an unknown id and a disabled owner's character are 404; a blank name is 400;
// a copy of one's own character works; a view cannot copy (step 11.4).
func TestCopy_Step12_RefusalsAndOwnCharacter(t *testing.T) {
	tb, m := newTable(t)
	bob, bobID, bram := playedBramOf(t, tb, m)
	_ = bob
	ann, _ := tb.login("")
	var before listView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/characters", "", &before))

	copyOf := func(token string, id int64, body string) int {
		return tb.call(token, "POST", "/api/characters/"+itoa(id)+"/copy", body, nil)
	}
	assert.Equal(t, http.StatusNotFound, copyOf(ann, 9999, `{"name":"x"}`), "unknown")
	assert.Equal(t, http.StatusBadRequest, copyOf(ann, bram, `{"name":"  "}`), "blank name")
	assert.Equal(t, http.StatusBadRequest, copyOf(ann, bram, `{}`), "no name")
	assert.Equal(t, http.StatusBadRequest, copyOf(ann, bram, `{"name":"x","owner":"y"}`), "unknown field")
	assert.Equal(t, http.StatusBadRequest, tb.call(ann, "POST", "/api/characters/zero/copy", `{"name":"x"}`, nil), "not an id")
	assert.Equal(t, http.StatusUnauthorized, copyOf("", bram, `{"name":"x"}`), "no session")

	// The page that views Bram sends the header: refused as every write of a view.
	rec := tb.viewing(ann, bram, "POST", "/api/characters/"+itoa(bram)+"/copy", `{"name":"x"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"read_only"`)

	var after listView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/characters", "", &after))
	assert.Equal(t, before, after, "a refused copy makes nothing")

	// Her own character: a copy, owned by her, named as sent.
	own := *before.CurrentID
	var got copiedView
	require.Equal(t, http.StatusCreated, copyOf2(tb, ann, own, `{"name":"Kelsen (copy)"}`, &got))
	assert.Len(t, got.Characters, len(before.Characters)+1)

	// A retired source and a disabled owner's are 404, as to read them (step 11.1).
	old, err := m.AddCharacterOf(context.Background(), bobID, charactersvc.Character{Name: "Old", Class: "thief", Dex: 10, Wis: 10, HPMax: 6})
	require.NoError(t, err)
	require.NoError(t, m.RetireCharacter(context.Background(), old))
	assert.Equal(t, http.StatusNotFound, copyOf(ann, old, `{"name":"x"}`), "retired")
	require.Equal(t, http.StatusOK, tb.call(ann, "PATCH", "/api/admin/users/"+bobID.String(), `{"disabled":true}`, nil))
	assert.Equal(t, http.StatusNotFound, copyOf(ann, bram, `{"name":"x"}`), "owner disabled")
}

func copyOf2(tb table, token string, id int64, body string, out any) int {
	return tb.call(token, "POST", "/api/characters/"+itoa(id)+"/copy", body, out)
}

// stripIDs drops every "id" key: the copy's own rows get their own ids.
func stripIDs(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			if k != "id" {
				out[k] = stripIDs(e)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = stripIDs(e)
		}
		return out
	}
	return v
}
