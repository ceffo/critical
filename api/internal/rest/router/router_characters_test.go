package router_test

// ADR 0029 step 7: the character a request acts on is its user's current
// character, resolved once by the router (mw.CurrentCharacter).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	authmocks "github.com/ceffo/critical/api/internal/domain/service/authsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
	"github.com/ceffo/critical/api/internal/rest/router"
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
// the health check, the reference data no character owns (step 7.5), the
// user's characters, the creation's rolls and the admin section (adminPrefix).
var characterFree = map[string]bool{
	"GET /api/health": true, "GET /api/changelog": true, "GET /api/spells/filters": true, "GET /api/spells/names": true,
	"GET /api/ruleset": true, "GET /api/ruleset/changes": true, "PUT /api/ruleset/crit-table": true,
	"PUT /api/ruleset/racial-lift": true, "PUT /api/ruleset/success-effects": true, "GET /api/xp/preview": true,
	"GET /api/characters": true, "PUT /api/characters/current": true,
	"GET /api/characters/options": true, "POST /api/characters": true, "POST /api/characters/draft": true, "PATCH /api/characters/{id}": true,
	"POST /api/characters/rolls/scores": true, "POST /api/characters/rolls/hp": true,
	"POST /api/characters/rolls/scores/arrangement": true, "POST /api/characters/rolls/identity": true,
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
		if !strings.HasPrefix(route, "/api/") || strings.HasPrefix(route, "/api/auth/") ||
			strings.HasPrefix(route, adminPrefix) || characterFree[key] {
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
	m.log.EXPECT().AppendHPEvent(mock.Anything, int64(8), -4, "orc").Return(nil, nil).Once()

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
		{
			ID: 3, Name: "Kelsen", Class: "bard", Level: 15, Race: "half_elf", Alignment: "chaotic_good", Sex: "male",
			Missing: []string{}, CurrentHP: 55, MaxHP: 85,
		},
		{ID: 5, Name: "Vex", Class: "thief", Level: 2, Race: "elf", Missing: []string{"alignment", "sex"}, CurrentHP: 9, MaxHP: 9},
	}, nil).Once()
	m.char.EXPECT().List(mock.Anything, bobID).Return([]charactersvc.Summary{}, nil).Once()

	rec := as(h, "ann", "GET", "/api/characters", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"characters":[{"id":3,"name":"Kelsen","class":"bard","level":15,"race":"half_elf",`+
		`"alignment":"chaotic_good","sex":"male","missing":[],"currentHp":55,"maxHp":85},`+
		`{"id":5,"name":"Vex","class":"thief","level":2,"race":"elf","alignment":"","sex":"",`+
		`"missing":["alignment","sex"],"currentHp":9,"maxHp":9}],"currentId":3}`, rec.Body.String())

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

// shown sends a request with the session token from a page that shows the
// character id (mw.CharacterHeader).
func shown(h http.Handler, token, id, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: token})
	req.Header.Set(mw.CharacterHeader, id)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Step 7.2: a request names the character its page shows and acts on that
// one or on none. The remembered one costs no query; another is checked
// against the database (only then), and still another is refused with
// character_changed before any service runs (the strict mocks expect no call).
func TestCurrentCharacter_Step7_2_TheShownCharacterOrNone(t *testing.T) {
	ann := authsvc.User{ID: annID, Email: "ann@example.com", Role: authsvc.RolePlayer, CurrentCharacterID: 3}
	a := usersAuth(t, map[string]authsvc.User{"ann": ann})
	h, m := newRouterWith(t, a)

	m.log.EXPECT().AppendHPEvent(mock.Anything, int64(3), -1, "a").Return(nil, nil).Once()
	assert.Equal(t, http.StatusOK, shown(h, "ann", "3", "POST", "/api/hp/event", `{"delta":-1,"source":"a"}`).Code)

	// Switched to 5 on the other server: this one remembers 3.
	a.EXPECT().FreshCurrentCharacter(mock.Anything, mock.Anything).Return(5, nil).Once()
	m.log.EXPECT().AppendHPEvent(mock.Anything, int64(5), -2, "b").Return(nil, nil).Once()
	assert.Equal(t, http.StatusOK, shown(h, "ann", "5", "POST", "/api/hp/event", `{"delta":-2,"source":"b"}`).Code)

	// The page shows 7, the database says 5: nothing runs.
	a.EXPECT().FreshCurrentCharacter(mock.Anything, mock.Anything).Return(5, nil).Once()
	rec := shown(h, "ann", "7", "POST", "/api/hp/event", `{"delta":-3,"source":"c"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"current character changed: conflict","code":"character_changed"}`, rec.Body.String())

	// The database unavailable while checking: 503, not 404 or 409.
	a.EXPECT().FreshCurrentCharacter(mock.Anything, mock.Anything).Return(0, fmt.Errorf("user: %w", domain.ErrUnavailable)).Once()
	rec = shown(h, "ann", "7", "GET", "/api/hp", "")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"database_unavailable"`)

	for _, bad := range []string{"x", "0", "-1", "3.0"} {
		assert.Equal(t, http.StatusBadRequest, shown(h, "ann", bad, "GET", "/api/hp", "").Code, bad)
	}
}

// ADR 0025 step 10 for the user's characters: a database too slow for the
// request deadline, or unavailable, answers 503 database_unavailable on the
// list and on the switch, never a 404 or a 500.
func TestCharacters_SlowDatabaseIsUnavailable(t *testing.T) {
	h, m := newRouterOpts(t, usersAuth(t, map[string]authsvc.User{
		"ann": {ID: annID, Email: "ann@example.com", Role: authsvc.RolePlayer, CurrentCharacterID: 3},
	}), router.Options{RequestDeadline: 30 * time.Millisecond})
	release := make(chan struct{})
	defer close(release)
	m.char.EXPECT().List(mock.Anything, annID).RunAndReturn(func(context.Context, uuid.UUID) ([]charactersvc.Summary, error) {
		<-release // a driver call that does not watch the context
		return nil, nil
	}).Once()
	m.char.EXPECT().SetCurrent(mock.Anything, annID, int64(5)).RunAndReturn(func(ctx context.Context, _ uuid.UUID, _ int64) error {
		<-ctx.Done() // a driver call that gives up at the deadline
		return ctx.Err()
	}).Once()
	for _, rec := range []*httptest.ResponseRecorder{
		as(h, "ann", "GET", "/api/characters", ""),
		as(h, "ann", "PUT", "/api/characters/current", `{"id":5}`),
	} {
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":"database_unavailable"`)
	}

	h, m = newRouterWith(t, usersAuth(t, map[string]authsvc.User{
		"ann": {ID: annID, Email: "ann@example.com", Role: authsvc.RolePlayer, CurrentCharacterID: 3},
	}))
	gone := fmt.Errorf("characters of: %w", domain.ErrUnavailable)
	m.char.EXPECT().List(mock.Anything, annID).Return(nil, gone).Once()
	m.char.EXPECT().SetCurrent(mock.Anything, annID, int64(5)).Return(gone).Once()
	for _, rec := range []*httptest.ResponseRecorder{
		as(h, "ann", "GET", "/api/characters", ""),
		as(h, "ann", "PUT", "/api/characters/current", `{"id":5}`),
	} {
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":"database_unavailable"`)
	}
}

// GET /api/character carries the derived block (ADR 0030 steps 3 to 8, 20.1,
// 22.1 and 24) as structured data: numbers and codes, the slots by ascending spell
// level, null at the last level; the race code apart from the free-text race.
// The stored sheet figures are not served.
func TestCharacter_ServesTheDerivedBlock(t *testing.T) {
	five, zero := 5, 0
	h, m := newRouterWith(t, usersAuth(t, map[string]authsvc.User{
		"ann": {ID: annID, Email: "ann@example.com", Role: authsvc.RolePlayer, CurrentCharacterID: 3},
	}))
	next := 1_320_000
	m.char.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(&charactersvc.Character{
		ID: 3, Name: "Kelsen", Class: "bard", Race: "dwarf", XPTotal: 1_293_418,
		Sheet: charactersvc.Sheet{Level: 15, THAC0: 14},
		Derived: charactersvc.Derived{
			XPTotal: 1_300_000, TargetLevel: 11, RealizedLevel: 9, NextLevelXP: &next, THAC0: 13,
			Saves:         charactersvc.Saves{ParaPoison: 10, RodStaffWand: 8, PetriPoly: 9, Breath: 13, Spell: 9},
			RaceSaveBonus: charactersvc.Saves{RodStaffWand: 4, Spell: 4},
			SpellSlots:    map[int]int{5: 2, 1: 3},
			LevelUp: charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{
				{Level: 10, Die: 6, Grants: skills.LevelGrants{SkillPoints: 15}},
				{Level: 11, FixedGain: 2, Grants: skills.LevelGrants{WeaponSlots: 1, NonweaponSlots: 2}, FreeSpells: []spellbook.FreeSpell{{Level: 11, SpellLevel: 6}}},
			}, ToSpend: skills.LevelGrants{WeaponSlots: 3, NonweaponSlots: 1, SkillPoints: 45}, FreeSpellsOwed: []spellbook.FreeSpell{{Level: 9, SpellLevel: 5}}},
			Spellbook: &charactersvc.SpellbookRules{ClassMaxSpellLevel: 6, IntMaxSpellLevel: 7, LearnChance: 60, MaxSpellsPerLevel: 9},
		},
		Details: charactersvc.Details{Alignment: "lawful_good", Languages: []string{}},
		Warnings: []charactersvc.Warning{
			{Code: "alignment_not_allowed", Field: "alignment"},
			{Code: "slots_over_budget", Field: "nonweapon", Pool: "nonweapon", Spent: &five, Available: &zero},
		},
	}, nil).Once()

	rec := as(h, "ann", "GET", "/api/character", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "dwarf", body["raceCode"])
	assert.NotContains(t, body, "race", "the free-text race detail is gone")
	// ADR 0032 step 8: the sex is missing, and the bard's lawful good is kept with a warning.
	assert.Equal(t, []any{"sex"}, body["missing"])
	assert.Equal(t, []any{
		map[string]any{"code": "alignment_not_allowed", "field": "alignment"},
		map[string]any{"code": "slots_over_budget", "field": "nonweapon", "pool": "nonweapon", "spent": float64(5), "available": float64(0)},
	}, body["warnings"], "a slot warning carries its figures, an available of 0 included (ADR 0037 step 11)")
	for _, gone := range []string{"level", "thac0Base", "saveParaPoison", "saveSpell"} {
		assert.NotContains(t, body, gone)
	}
	derived, err := json.Marshal(body["derived"])
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"xpTotal": 1300000, "targetLevel": 11, "realizedLevel": 9, "nextLevelXp": 1320000, "thac0": 13, "proficiencySlots": null,
		"saves": {"paraPoison": 10, "rodStaffWand": 8, "petriPoly": 9, "breath": 13, "spell": 9},
		"raceSaveBonus": {"paraPoison": 0, "rodStaffWand": 4, "petriPoly": 0, "breath": 0, "spell": 4},
		"spellSlots": [{"level": 1, "count": 3}, {"level": 5, "count": 2}],
		"levelUp": {"realizedAboveTarget": false, "pending": [
			{"level": 10, "die": 6, "fixedGain": 0, "grants": {"weaponSlots": 0, "nonWeaponSlots": 0, "skillPoints": 15, "freeSpells": []}},
			{"level": 11, "die": 0, "fixedGain": 2, "grants": {"weaponSlots": 1, "nonWeaponSlots": 2, "skillPoints": 0, "freeSpells": [{"level": 11, "spellLevel": 6}]}}
		], "toSpend": {"weaponSlots": 3, "nonWeaponSlots": 1, "skillPoints": 45, "freeSpells": [{"level": 9, "spellLevel": 5}]}},
		"specializes": false,
		"spellFailure": 0,
		"turning": null,
		"thiefSkills": null, "bardSkills": null,
		"skillArmor": "", "skillBudget": [], "skillPoints": null,
		"backstab": 0,
		"spellbook": {"classMaxSpellLevel": 6, "intMaxSpellLevel": 7, "learnChance": 60, "maxSpellsPerLevel": 9, "noMaximum": false}
	}`, string(derived))
}

// POST /api/characters/draft (ADR 0034, amendment of 2026-10-20): the body as far as it is
// entered goes to the service with the numbers it leaves out; the answer is the sheet's derived
// block in GET /api/character's shape (the skill budget named as GET /api/skills names it), the
// refusals with the creation's fields and codes (an over_budget with its figures), the missing
// fields, the class minimums missed and the skill point budget on its own (the derived block's shape).
func TestCharacters_DraftWireShape(t *testing.T) {
	h, m := newRouterWith(t, usersAuth(t, map[string]authsvc.User{
		"new": {ID: bobID, Email: "new@example.com", Role: authsvc.RolePlayer},
	}))
	m.char.EXPECT().Draft(mock.Anything, mock.MatchedBy(func(n charactersvc.NewCharacter) bool {
		return n.Class == "thief" && n.Race == "elf" && n.Dex == 25 && n.XP == 1250 && n.SkillPoints["open_locks"] == 40 &&
			n.Proficiencies.Equal(skills.Picks{
				Nonweapon: []skills.Pick{{ID: "tumbling", Slots: 2}}, Weapon: []skills.WeaponPick{{Type: "dagger", Slots: 1}},
			}) && slices.Equal(n.Languages, []string{"elvish", "orcish"})
	}), []string{"hpMax"}).Return(&charactersvc.Draft{
		Character: &charactersvc.Character{Derived: charactersvc.Derived{
			TargetLevel: 2, RealizedLevel: 2,
			SkillPoints: &ruletablesvc.SkillPointsBudget{
				Level: 2, First: 60, PerLevel: 30, Total: 90, Spent: 40, Left: 50,
				PerSkillFirst: 30, PerSkillPerLevel: 15, PerSkill: 45,
			},
		}, Details: charactersvc.Details{Languages: []string{"elvish", "orcish"}}},
		Refusals: []error{
			&domain.FieldError{Field: "dex", Message: charactersvc.CodeOutOfRange},
			&domain.BudgetError{Field: "nonweapon", Spent: 8, Limit: 7},
		},
		Missing:       []string{"name", "hpMax", "alignment", "sex"},
		UnmetMinimums: []charactersvc.ClassUnmet{{Class: "thief", Abilities: []string{}}, {Class: "bard", Abilities: []string{"cha"}}},
		SkillPoints: &ruletablesvc.SkillPointsBudget{
			Level: 2, First: 60, PerLevel: 30, Total: 90, Spent: 40, Left: 50,
			PerSkillFirst: 30, PerSkillPerLevel: 15, PerSkill: 45,
		},
	}, nil).Once()
	rec := as(h, "new", "POST", "/api/characters/draft", `{"class":"thief","raceCode":"elf","str":9,"dex":25,"con":12,"int":11,"wis":10,"cha":12,
		"xp":1250,"skillPoints":{"open_locks":40},
		"proficiencies":{"nonweapon":[{"id":"tumbling","slots":2}],"weapon":[{"type":"dagger","slots":1}]},
		"details":{"languages":["elvish","orcish"]}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.JSONEq(t, `[{"field":"dex","code":"out_of_range"},{"field":"nonweapon","code":"over_budget","spent":8,"limit":7}]`, string(got["refusals"]))
	assert.JSONEq(t, `["name","hpMax","alignment","sex"]`, string(got["missing"]))
	assert.JSONEq(t, `[{"class":"thief","abilities":[]},{"class":"bard","abilities":["cha"]}]`, string(got["unmetMinimums"]))
	assert.JSONEq(t, `["elvish","orcish"]`, string(got["languages"]))
	assert.JSONEq(t, `[]`, string(got["warnings"]))
	var derived map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got["derived"], &derived))
	assert.JSONEq(t, `{"level":2,"first":60,"perLevel":30,"available":90,"spent":40,"left":50,"perSkillFirst":30,"perSkillPerLevel":15,"perSkillCap":45}`,
		string(derived["skillPoints"]))
	assert.JSONEq(t, string(derived["skillPoints"]), string(got["skillPoints"]), "the draft's own budget, in the derived block's shape")
	assert.JSONEq(t, `2`, string(derived["targetLevel"]))
}

// ADR 0030 step 20.7 on the wire: the body's `spellbook` (catalogue ids, in order) is the new
// character's starting spellbook; the draft answers its limits at the starting level, the spells
// entered by spell level, and a refused spell on `spellbook.<id>` with its code.
func TestCharacters_DraftCarriesTheStartingSpellbook(t *testing.T) {
	h, m := newRouterWith(t, usersAuth(t, map[string]authsvc.User{
		"new": {ID: bobID, Email: "new@example.com", Role: authsvc.RolePlayer},
	}))
	m.char.EXPECT().Draft(mock.Anything, mock.MatchedBy(func(n charactersvc.NewCharacter) bool {
		return assert.ObjectsAreEqual([]string{"read_magic", "bless"}, n.Spellbook)
	}), mock.Anything).Return(&charactersvc.Draft{
		Spellbook: &charactersvc.StartingSpellbook{
			ClassMaxSpellLevel: 1, IntMaxSpellLevel: 7, MaxSpellsPerLevel: 9,
			Levels: []charactersvc.SpellLevelCount{{Level: 1, Count: 1}},
		},
		Refusals: []error{&domain.FieldError{Field: "spellbook.bless", Message: charactersvc.CodeNotAWizardSpell}},
	}, nil).Once()
	rec := as(h, "new", "POST", "/api/characters/draft", `{"class":"mage","spellbook":["read_magic","bless"]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.JSONEq(t, `{"classMaxSpellLevel":1,"intMaxSpellLevel":7,"maxSpellsPerLevel":9,"noMaximum":false,"levels":[{"level":1,"count":1}]}`,
		string(got["spellbook"]))
	assert.JSONEq(t, `[{"field":"spellbook.bless","code":"not_a_wizard_spell"}]`, string(got["refusals"]))
}

// A draft's content is never a 400: an empty body answers 200 with every number blank, a null
// derived block and empty lists; a height that is no number is the draft's one refusal (the
// service is not asked). A body that is no JSON or names an unknown field is still a 400.
func TestCharacters_DraftNeverRefusesContent(t *testing.T) {
	h, m := newRouterWith(t, usersAuth(t, map[string]authsvc.User{
		"new": {ID: bobID, Email: "new@example.com", Role: authsvc.RolePlayer},
	}))
	m.char.EXPECT().Draft(mock.Anything, mock.MatchedBy(func(n charactersvc.NewCharacter) bool { return n.Class == "" && n.Race == "" }),
		[]string{"str", "dex", "con", "int", "wis", "cha", "hpMax", "xp"}).
		Return(&charactersvc.Draft{Missing: []string{"name", "class"}}, nil).Once()
	rec := as(h, "new", "POST", "/api/characters/draft", `{}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"derived":null,"skillPoints":null,"spellbook":null,"warnings":[],"languages":[],"refusals":[],"missing":["name","class"],"unmetMinimums":[]}`, rec.Body.String())

	rec = as(h, "new", "POST", "/api/characters/draft", `{"details":{"height_cm":"tall"}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got struct {
		Refusals []map[string]any `json:"refusals"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Refusals, 1)
	assert.Equal(t, "details.height", got.Refusals[0]["field"])

	for _, body := range []string{`{"level":3}`, `not json`} {
		rec = as(h, "new", "POST", "/api/characters/draft", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}
