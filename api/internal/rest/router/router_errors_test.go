package router_test

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	logmocks "github.com/ceffo/critical/api/internal/domain/service/actionlogsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	buffmocks "github.com/ceffo/critical/api/internal/domain/service/buffsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	castmocks "github.com/ceffo/critical/api/internal/domain/service/castsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	charmocks "github.com/ceffo/critical/api/internal/domain/service/charactersvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	combatmocks "github.com/ceffo/critical/api/internal/domain/service/combatsvc/mocks"
	detailsmocks "github.com/ceffo/critical/api/internal/domain/service/detailssvc/mocks"
	hpmocks "github.com/ceffo/critical/api/internal/domain/service/hpsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	itemmocks "github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/restsvc"
	restmocks "github.com/ceffo/critical/api/internal/domain/service/restsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	spellmocks "github.com/ceffo/critical/api/internal/domain/service/spellsvc/mocks"
	weaponmocks "github.com/ceffo/critical/api/internal/domain/service/weaponsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	xpmocks "github.com/ceffo/critical/api/internal/domain/service/xpsvc/mocks"
	"github.com/ceffo/critical/api/internal/rest/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mocksSet struct {
	char    *charmocks.MockSVC
	hp      *hpmocks.MockSVC
	buffs   *buffmocks.MockSVC
	spell   *spellmocks.MockSVC
	cast    *castmocks.MockSVC
	cmb     *combatmocks.MockSVC
	log     *logmocks.MockSVC
	items   *itemmocks.MockSVC
	weapons *weaponmocks.MockSVC
	rest    *restmocks.MockSVC
	xp      *xpmocks.MockSVC
	details *detailsmocks.MockSVC
}

func newRouter(t *testing.T) (http.Handler, *mocksSet) {
	t.Helper()
	m := &mocksSet{
		char:    charmocks.NewMockSVC(t),
		hp:      hpmocks.NewMockSVC(t),
		buffs:   buffmocks.NewMockSVC(t),
		spell:   spellmocks.NewMockSVC(t),
		cast:    castmocks.NewMockSVC(t),
		cmb:     combatmocks.NewMockSVC(t),
		log:     logmocks.NewMockSVC(t),
		items:   itemmocks.NewMockSVC(t),
		weapons: weaponmocks.NewMockSVC(t),
		rest:    restmocks.NewMockSVC(t),
		xp:      xpmocks.NewMockSVC(t),
		details: detailsmocks.NewMockSVC(t),
	}
	bs := &composition.BackingServices{
		CharacterID: 1,
		Character:   m.char, HP: m.hp, Buffs: m.buffs, Spells: m.spell, Cast: m.cast, Combat: m.cmb, Log: m.log, Items: m.items, Weapons: m.weapons, Rest: m.rest,
		XP: m.xp, Details: m.details,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return router.New(log, bs), m
}

func anyArgs(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = mock.Anything
	}
	return out
}

// endpoint describes one route and how to make its service call fail.
type endpoint struct {
	name   string
	method string
	path   string
	body   string
	fail   func(m *mocksSet, err error)
}

var endpoints = []endpoint{
	{"PATCH character/details", "PATCH", "/api/character/details", `{"race":"Elf"}`, func(m *mocksSet, err error) {
		m.details.On("Update", anyArgs(3)...).Return(charactersvc.Details{}, err)
	}},
	{"GET xp", "GET", "/api/xp", "", func(m *mocksSet, err error) {
		m.xp.On("Get", anyArgs(2)...).Return(nil, err)
	}},
	{"GET xp/preview", "GET", "/api/xp/preview?base=10&bonus=true", "", func(m *mocksSet, err error) {
		m.xp.On("Preview", anyArgs(2)...).Return(xpsvc.Amount{}, err)
	}},
	{"POST xp", "POST", "/api/xp", `{"base":10}`, func(m *mocksSet, err error) {
		m.xp.On("Add", anyArgs(3)...).Return(nil, err)
	}},
	{"PATCH xp/{id}", "PATCH", "/api/xp/3", `{"base":10}`, func(m *mocksSet, err error) {
		m.xp.On("Update", anyArgs(4)...).Return(nil, err)
	}},
	{"DELETE xp/{id}", "DELETE", "/api/xp/3", "", func(m *mocksSet, err error) {
		m.xp.On("Delete", anyArgs(3)...).Return(nil, err)
	}},
	{"GET character", "GET", "/api/character", "", func(m *mocksSet, err error) {
		m.char.On("GetCharacter", anyArgs(2)...).Return(nil, err)
	}},
	{"GET hp", "GET", "/api/hp", "", func(m *mocksSet, err error) {
		m.hp.On("GetState", anyArgs(2)...).Return(nil, err)
	}},
	{"POST hp/event", "POST", "/api/hp/event", `{"delta":-1,"source":"x"}`, func(m *mocksSet, err error) {
		m.hp.On("AppendEvent", anyArgs(4)...).Return(nil, err)
	}},
	{"POST hp/undo", "POST", "/api/hp/undo", "", func(m *mocksSet, err error) {
		m.hp.On("UndoLast", anyArgs(2)...).Return(nil, err)
	}},
	{"GET log", "GET", "/api/log?limit=10", "", func(m *mocksSet, err error) {
		m.log.On("List", anyArgs(3)...).Return(nil, err)
	}},
	{"POST log/{id}/amend", "POST", "/api/log/4/amend", `{"note":"x"}`, func(m *mocksSet, err error) {
		m.log.On("Amend", anyArgs(4)...).Return(nil, err)
	}},
	{"POST log/{id}/void", "POST", "/api/log/4/void", "", func(m *mocksSet, err error) {
		m.log.On("Void", anyArgs(3)...).Return(nil, err)
	}},
	{"GET log top", "GET", "/api/log", "", func(m *mocksSet, err error) {
		m.log.On("List", anyArgs(3)...).Return(nil, nil)
		m.log.On("TopID", anyArgs(2)...).Return(int64(0), false, err)
	}},
	{"POST log/undo", "POST", "/api/log/undo", `{"expectedTopId":4}`, func(m *mocksSet, err error) {
		m.log.On("Undo", anyArgs(3)...).Return(nil, err)
	}},
	{"GET buffs", "GET", "/api/buffs", "", func(m *mocksSet, err error) {
		m.buffs.On("ListStatus", anyArgs(2)...).Return(nil, err)
	}},
	{"PATCH buffs/{id}", "PATCH", "/api/buffs/b1", `{"duration":3}`, func(m *mocksSet, err error) {
		m.buffs.On("Edit", anyArgs(4)...).Return(nil, err)
	}},
	{"GET stats/str", "GET", "/api/stats/str", "", func(m *mocksSet, err error) {
		m.buffs.On("GetEffectiveStr", anyArgs(2)...).Return(buffsvc.Strength{}, err)
	}},
	{"POST buffs", "POST", "/api/buffs", `{"name":"bless"}`, func(m *mocksSet, err error) {
		m.buffs.On("Activate", anyArgs(3)...).Return(nil, err)
	}},
	{"DELETE buffs/{id}", "DELETE", "/api/buffs/b1", "", func(m *mocksSet, err error) {
		m.buffs.On("Dismiss", anyArgs(3)...).Return(nil, err)
	}},
	{"POST buffs/{id}/trigger", "POST", "/api/buffs/b1/trigger", "", func(m *mocksSet, err error) {
		m.buffs.On("Trigger", anyArgs(3)...).Return(nil, err)
	}},
	{"GET round", "GET", "/api/round", "", func(m *mocksSet, err error) {
		m.buffs.On("GetRound", anyArgs(2)...).Return(0, err)
	}},
	{"POST round/advance", "POST", "/api/round/advance", "", func(m *mocksSet, err error) {
		m.buffs.On("AdvanceRound", anyArgs(2)...).Return(nil, nil, 0, buffsvc.Initiative{}, err)
	}},
	{"POST round/initiative/reroll", "POST", "/api/round/initiative/reroll", "", func(m *mocksSet, err error) {
		m.buffs.On("RerollInitiative", anyArgs(2)...).Return(buffsvc.Initiative{}, err)
	}},
	{"POST round/reset", "POST", "/api/round/reset", "", func(m *mocksSet, err error) {
		m.buffs.On("ResetRound", anyArgs(2)...).Return(nil, 0, err)
	}},
	{"PUT round", "PUT", "/api/round", `{"round":3}`, func(m *mocksSet, err error) {
		m.buffs.On("SetRound", anyArgs(3)...).Return(nil, 0, err)
	}},
	{"POST round/pass-time", "POST", "/api/round/pass-time", `{"hours":1}`, func(m *mocksSet, err error) {
		m.buffs.On("PassTime", anyArgs(3)...).Return(nil, nil, 0, 0, err)
	}},
	{"GET spells/slots", "GET", "/api/spells/slots", "", func(m *mocksSet, err error) {
		m.spell.On("GetSlots", anyArgs(2)...).Return(nil, err)
	}},
	{"GET spells", "GET", "/api/spells?q=a", "", func(m *mocksSet, err error) {
		m.spell.On("Search", anyArgs(3)...).Return(spellsvc.Page{}, err)
	}},
	{"POST spells/{id}/cast", "POST", "/api/spells/2/cast", "", func(m *mocksSet, err error) {
		m.cast.On("Cast", anyArgs(4)...).Return(castsvc.CastResult{}, err)
	}},
	{"POST spells/receive", "POST", "/api/spells/receive", `{"spellId":"strength","casterLevel":9}`, func(m *mocksSet, err error) {
		m.cast.On("Receive", anyArgs(3)...).Return(castsvc.CastResult{}, err)
	}},
	{"POST spells/reset", "POST", "/api/spells/reset", "", func(m *mocksSet, err error) {
		m.rest.On("LongRest", anyArgs(2)...).Return(restsvc.Result{}, err)
	}},
	{"POST rest/long", "POST", "/api/rest/long", "", func(m *mocksSet, err error) {
		m.rest.On("LongRest", anyArgs(2)...).Return(restsvc.Result{}, err)
	}},
	{"GET weapons", "GET", "/api/weapons", "", func(m *mocksSet, err error) {
		m.cmb.On("ListWeapons", anyArgs(1)...).Return(nil, err)
	}},
	{"GET items", "GET", "/api/items", "", func(m *mocksSet, err error) {
		m.items.On("List", anyArgs(1)...).Return(nil, err)
	}},
	{"POST roll/attack", "POST", "/api/roll/attack", `{"weapon_id":"w","target_ac":3}`, func(m *mocksSet, err error) {
		m.cmb.On("ResolveAttack", anyArgs(4)...).Return(nil, err)
	}},
	{"POST roll/save", "POST", "/api/roll/save", `{"category":"spell"}`, func(m *mocksSet, err error) {
		m.cmb.On("ResolveSavingThrow", anyArgs(3)...).Return(nil, err)
	}},
	{"POST roll/save/{id}/adjust", "POST", "/api/roll/save/7/adjust", `{"adjustment":4,"reason":"Gaia fire"}`, func(m *mocksSet, err error) {
		m.cmb.On("AdjustSave", anyArgs(4)...).Return(nil, err)
	}},
	{"POST roll/attack/{id}/adjust", "POST", "/api/roll/attack/7/adjust", `{"targetAc":7}`, func(m *mocksSet, err error) {
		m.cmb.On("AdjustAttackAC", anyArgs(4)...).Return(nil, err)
	}},
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Service errors map to the same status and JSON body on every endpoint.
// Expected values were recorded against the pre-middleware handlers.
func TestServiceErrorsMapToStatusAndBody(t *testing.T) {
	errs := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{"invalid input", fmt.Errorf("%w: bad delta", domain.ErrInvalidInput), 400, `{"error":"invalid input: bad delta"}`},
		{"conflict", fmt.Errorf("%w: dup", domain.ErrConflict), 409, `{"error":"conflict: dup"}`},
		{"not found", fmt.Errorf("%w: x", domain.ErrNotFound), 404, `{"error":"not found: x"}`},
		{"character not found", domain.ErrCharacterNotFound, 404, `{"error":"character not found"}`},
		{"unknown", errors.New("boom"), 500, `{"error":"boom"}`},
	}
	for _, ep := range endpoints {
		for _, e := range errs {
			t.Run(ep.name+"/"+e.name, func(t *testing.T) {
				h, m := newRouter(t)
				ep.fail(m, e.err)
				rec := do(h, ep.method, ep.path, ep.body)
				assert.Equal(t, e.status, rec.Code)
				assert.JSONEq(t, e.body, rec.Body.String())
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
				assert.Equal(t, e.body+"\n", rec.Body.String())
			})
		}
	}
}

// Endpoint-specific sentinels keep their special statuses and messages.
func TestSpecialSentinels(t *testing.T) {
	cases := []struct {
		name   string
		ep     string
		err    error
		status int
		body   string
	}{
		{"weapon not found", "POST roll/attack", fmt.Errorf("%w: sword", combatsvc.ErrWeaponNotFound), 404, `{"error":"weapon not found"}`},
		{"item not found", "GET items", fmt.Errorf("%w: wand", itemsvc.ErrItemNotFound), 404, `{"error":"item not found: wand"}`},
		{"item empty", "GET items", fmt.Errorf("%w: wand", itemsvc.ErrItemEmpty), 409, `{"error":"item is empty: wand"}`},
		{"slot already used", "POST spells/{id}/cast", fmt.Errorf("%w: 2", spellsvc.ErrSlotAlreadyUsed), 409, `{"error":"slot already used: 2"}`},
		{"invalid slot index", "POST spells/{id}/cast", fmt.Errorf("%w: 2", spellsvc.ErrInvalidSlotIndex), 400, `{"error":"invalid slot index: 2"}`},
		{"void not the latest", "POST log/{id}/void", fmt.Errorf("%w: only the latest entry can be voided, void \"Round 2 → 3\" (round 2) first", actionlogsvc.ErrNotLatest), 409, `{"error":"not the latest entry: only the latest entry can be voided, void \"Round 2 → 3\" (round 2) first"}`},
		{"hp undo not the latest", "POST hp/undo", fmt.Errorf("%w: the latest entry is not a hp_change, void \"Activated Haste\" (round 2) first", actionlogsvc.ErrNotLatest), 409, `{"error":"not the latest entry: the latest entry is not a hp_change, void \"Activated Haste\" (round 2) first"}`},
		{"undo stale top", "POST log/undo", fmt.Errorf("%w: the newest entry is #5 (attack, round 2), not #4", actionlogsvc.ErrTopChanged), 409, `{"error":"the newest entry changed: the newest entry is #5 (attack, round 2), not #4"}`},
		{"undo empty log", "POST log/undo", fmt.Errorf("%w: expected #4 on top", actionlogsvc.ErrNothingToUndo), 409, `{"error":"nothing to undo: expected #4 on top"}`},
		{"void already voided", "POST log/{id}/void", fmt.Errorf("%w: #4", actionlogsvc.ErrAlreadyVoided), 409, `{"error":"entry already voided: #4"}`},
		{"amend already voided", "POST log/{id}/amend", fmt.Errorf("%w: #4", actionlogsvc.ErrAlreadyVoided), 409, `{"error":"entry already voided: #4"}`},
		{"adjust not latest", "POST roll/save/{id}/adjust", fmt.Errorf("%w: only the latest roll can be adjusted, void this save and roll again", actionlogsvc.ErrNotLatest), 409, `{"error":"not the latest entry: only the latest roll can be adjusted, void this save and roll again"}`},
		{"adjust voided", "POST roll/save/{id}/adjust", fmt.Errorf("%w: #7", actionlogsvc.ErrAlreadyVoided), 409, `{"error":"entry already voided: #7"}`},
		{"adjust not a save", "POST roll/save/{id}/adjust", fmt.Errorf("%w: #7 is attack, not save", actionlogsvc.ErrNotAmendable), 400, `{"error":"entry cannot be amended: #7 is attack, not save"}`},
		{"attack AC not latest", "POST roll/attack/{id}/adjust", fmt.Errorf("%w: only the latest roll can be adjusted, void this attack and roll again", actionlogsvc.ErrNotLatest), 409, `{"error":"not the latest entry: only the latest roll can be adjusted, void this attack and roll again"}`},
		{"attack AC voided", "POST roll/attack/{id}/adjust", fmt.Errorf("%w: #7", actionlogsvc.ErrAlreadyVoided), 409, `{"error":"entry already voided: #7"}`},
		{"attack AC not an attack", "POST roll/attack/{id}/adjust", fmt.Errorf("%w: #7 is save, not attack", actionlogsvc.ErrNotAmendable), 400, `{"error":"entry cannot be amended: #7 is save, not attack"}`},
		{"wrapped twice", "POST spells/{id}/cast", fmt.Errorf("ctx: %w", fmt.Errorf("%w: 2", spellsvc.ErrSlotAlreadyUsed)), 409, `{"error":"ctx: slot already used: 2"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var ep endpoint
			for _, e := range endpoints {
				if e.name == c.ep {
					ep = e
				}
			}
			h, m := newRouter(t)
			ep.fail(m, c.err)
			rec := do(h, ep.method, ep.path, ep.body)
			assert.Equal(t, c.status, rec.Code)
			assert.Equal(t, c.body+"\n", rec.Body.String())
		})
	}
}

// Request-parsing failures never reach the service.
func TestRequestParsingErrors(t *testing.T) {
	cases := []struct {
		name, method, path, body string
		body400                  string
	}{
		{"hp/event bad json", "POST", "/api/hp/event", "{", "invalid request body"},
		{"buffs bad json", "POST", "/api/buffs", "{", "invalid request body"},
		{"buff edit bad json", "PATCH", "/api/buffs/b1", "{", "invalid request body"},
		{"attack bad json", "POST", "/api/roll/attack", "nope", "invalid request body"},
		{"save bad json", "POST", "/api/roll/save", "nope", "invalid request body"},
		{"log bad limit", "GET", "/api/log?limit=abc", "", "limit must be a positive integer"},
		{"log zero limit", "GET", "/api/log?limit=0", "", "limit must be a positive integer"},
		{"amend bad id", "POST", "/api/log/abc/amend", `{"note":"x"}`, "invalid log entry id"},
		{"amend bad json", "POST", "/api/log/4/amend", "{", "invalid request body"},
		{"amend null body", "POST", "/api/log/4/amend", "null", "invalid request body"},
		{"amend missing note", "POST", "/api/log/4/amend", `{}`, "note is required (a string)"},
		{"amend note not a string", "POST", "/api/log/4/amend", `{"note":3}`, "note is required (a string)"},
		// Note-only amend: any value field is refused.
		{"amend value delta", "POST", "/api/log/4/amend", `{"delta":-3}`, `only the note of an entry can be amended (got \"delta\"): void the entry and log it again`},
		{"amend value with note", "POST", "/api/log/4/amend", `{"note":"x","source":"orc axe"}`, `only the note of an entry can be amended (got \"source\"): void the entry and log it again`},
		{"amend payload", "POST", "/api/log/4/amend", `{"payload":{}}`, `only the note of an entry can be amended (got \"payload\"): void the entry and log it again`},
		{"void bad id", "POST", "/api/log/0/void", "", "invalid log entry id"},
		{"undo bad json", "POST", "/api/log/undo", "{", "invalid request body"},
		{"undo no body", "POST", "/api/log/undo", "", "invalid request body"},
		{"undo missing expected top", "POST", "/api/log/undo", `{}`, "expectedTopId is required (a positive entry id)"},
		{"undo zero expected top", "POST", "/api/log/undo", `{"expectedTopId":0}`, "expectedTopId is required (a positive entry id)"},
		{"undo expected top not a number", "POST", "/api/log/undo", `{"expectedTopId":"4"}`, "invalid request body"},
		{"adjust bad id", "POST", "/api/roll/save/x/adjust", `{"adjustment":1}`, "invalid log entry id"},
		{"adjust bad json", "POST", "/api/roll/save/7/adjust", "{", "invalid request body"},
		{"adjust missing adjustment", "POST", "/api/roll/save/7/adjust", `{"reason":"x"}`, "adjustment is required"},
		{"cast non-numeric", "POST", "/api/spells/abc/cast", "", "invalid slot index"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, _ := newRouter(t) // mocks fail the test on any unexpected call
			rec := do(h, c.method, c.path, c.body)
			assert.Equal(t, 400, rec.Code)
			assert.Equal(t, `{"error":"`+c.body400+`"}`+"\n", rec.Body.String())
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		})
	}
}
