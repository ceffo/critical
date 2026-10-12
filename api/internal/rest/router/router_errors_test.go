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
	buffmocks "github.com/ceffo/critical/api/internal/domain/service/buffsvc/mocks"
	charmocks "github.com/ceffo/critical/api/internal/domain/service/charactersvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	combatmocks "github.com/ceffo/critical/api/internal/domain/service/combatsvc/mocks"
	hpmocks "github.com/ceffo/critical/api/internal/domain/service/hpsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	spellmocks "github.com/ceffo/critical/api/internal/domain/service/spellsvc/mocks"
	"github.com/ceffo/critical/api/internal/rest/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mocksSet struct {
	char  *charmocks.MockSVC
	hp    *hpmocks.MockSVC
	buffs *buffmocks.MockSVC
	spell *spellmocks.MockSVC
	cmb   *combatmocks.MockSVC
}

func newRouter(t *testing.T) (http.Handler, *mocksSet) {
	t.Helper()
	m := &mocksSet{
		char:  charmocks.NewMockSVC(t),
		hp:    hpmocks.NewMockSVC(t),
		buffs: buffmocks.NewMockSVC(t),
		spell: spellmocks.NewMockSVC(t),
		cmb:   combatmocks.NewMockSVC(t),
	}
	bs := &composition.BackingServices{
		CharacterID: 1,
		Character:   m.char, HP: m.hp, Buffs: m.buffs, Spells: m.spell, Combat: m.cmb,
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
	{"GET buffs", "GET", "/api/buffs", "", func(m *mocksSet, err error) {
		m.buffs.On("ListActive", anyArgs(2)...).Return(nil, err)
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
		m.buffs.On("AdvanceRound", anyArgs(2)...).Return(nil, nil, 0, err)
	}},
	{"GET spells/slots", "GET", "/api/spells/slots", "", func(m *mocksSet, err error) {
		m.spell.On("GetSlots", anyArgs(2)...).Return(nil, err)
	}},
	{"GET spells", "GET", "/api/spells?q=a", "", func(m *mocksSet, err error) {
		m.spell.On("Search", anyArgs(3)...).Return(nil, err)
	}},
	{"POST spells/{id}/cast", "POST", "/api/spells/2/cast", "", func(m *mocksSet, err error) {
		m.spell.On("Cast", anyArgs(3)...).Return(nil, err)
	}},
	{"POST spells/reset", "POST", "/api/spells/reset", "", func(m *mocksSet, err error) {
		m.spell.On("ResetSlots", anyArgs(2)...).Return(nil, err)
	}},
	{"POST roll/attack", "POST", "/api/roll/attack", `{"weapon_id":"w","target_ac":3}`, func(m *mocksSet, err error) {
		m.cmb.On("ResolveAttack", anyArgs(4)...).Return(nil, err)
	}},
	{"POST roll/save", "POST", "/api/roll/save", `{"category":"spell"}`, func(m *mocksSet, err error) {
		m.cmb.On("ResolveSavingThrow", anyArgs(3)...).Return(nil, err)
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
		{"slot already used", "POST spells/{id}/cast", fmt.Errorf("%w: 2", spellsvc.ErrSlotAlreadyUsed), 409, `{"error":"slot already used: 2"}`},
		{"invalid slot index", "POST spells/{id}/cast", fmt.Errorf("%w: 2", spellsvc.ErrInvalidSlotIndex), 400, `{"error":"invalid slot index: 2"}`},
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
		{"buffs missing name", "POST", "/api/buffs", `{}`, "name is required"},
		{"attack bad json", "POST", "/api/roll/attack", "nope", "invalid request body"},
		{"save bad json", "POST", "/api/roll/save", "nope", "invalid request body"},
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
