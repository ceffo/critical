package composition_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

// updateGolden rewrites the golden files from the code as it is: `go test ./internal/composition
// -run TestCharacter_Golden -update`. Review the diff before committing it.
var updateGolden = flag.Bool("update", false, "rewrite the golden files")

// The answer of GET /api/character stays what it was, byte for byte (indentation aside), for the
// e2e fixture's seeded character and for a thief whose languages and picks overspend the
// non-weapon pool: the derived block, its proficiency figures, the skill points' figures, the
// level grants and the warnings, wherever the rules that build them live (ADR 0037 step 17).
func TestCharacter_Golden_DerivedBlockUnchanged(t *testing.T) {
	tb, m := newTable(t)
	ann, annID := tb.login("")
	thief, err := m.AddCharacterOf(context.Background(), annID, charactersvc.Character{
		Name: "Ilse", Class: "thief", Race: "elf", Str: 10, Dex: 17, Con: 12, Int: 9, Wis: 10, Cha: 11,
		HPMax: 6, XPTotal: 2600, RealizedLevel: 1,
		SkillPoints: map[string]int{"pick_pockets": 30, "open_locks": 20, "climb_walls": 15},
		Details:     charactersvc.Details{Alignment: "neutral", Sex: "female", Languages: []string{"elvish", "common", "dwarvish", "gnomish", "orcish"}},
		Proficiencies: charactersvc.Picks{
			Nonweapon: []charactersvc.Pick{{ID: "tumbling", Slots: 2}, {ID: "appraising", Slots: 1}},
			Weapon:    []charactersvc.WeaponPick{{Type: "dagger", Slots: 1}, {Type: "short_bow", Slots: 1}},
		},
	})
	require.NoError(t, err)

	golden(t, "character_seeded.json", raw(t, tb, ann, 0))
	golden(t, "character_thief.json", raw(t, tb, ann, thief))
}

// raw is the body of GET /api/character for the character shown (0: the current one), switching
// to it first.
func raw(t *testing.T, tb table, token string, id int64) []byte {
	t.Helper()
	if id != 0 {
		require.Equal(t, http.StatusOK, tb.call(token, "PUT", "/api/characters/current", `{"id":`+itoa(id)+`}`, nil))
	}
	req := httptest.NewRequest("GET", "/api/character", nil)
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: token})
	rec := httptest.NewRecorder()
	tb.h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return rec.Body.Bytes()
}

// golden compares body, indented, to testdata/golden/name (or writes it with -update).
func golden(t *testing.T, name string, body []byte) {
	t.Helper()
	var got bytes.Buffer
	require.NoError(t, json.Indent(&got, bytes.TrimSpace(body), "", "  "))
	got.WriteByte('\n')
	path := filepath.Join("testdata", "golden", name)
	if *updateGolden {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, got.Bytes(), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(want), got.String(), "GET /api/character changed for %s", name)
}
