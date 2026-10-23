package casthdl_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/casthdl"
	"github.com/ceffo/critical/api/internal/rest/wire"
	"github.com/ceffo/critical/api/pkg/formula"
)

const (
	charID    = int64(1)
	entryID   = int64(42)
	bardLevel = 15
)

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// castWire is the real cast service on mocked ports (the seed's spell
// effects) behind the cast routes, with a log that keeps the payload of the
// one entry it records (id entryID).
type castWire struct {
	slots   *mocks.MockSlots
	texts   *mocks.MockTexts
	buffs   *mocks.MockBuffs
	dice    *mocks.MockDice
	router  http.Handler
	payload json.RawMessage
}

func newCastWire(t *testing.T) *castWire {
	t.Helper()
	effects, err := spelleffectsadpt.New(filepath.Join("..", "..", "..", "..", "testdata", "seeds"), testGroups)
	require.NoError(t, err)
	w := &castWire{
		slots: mocks.NewMockSlots(t), texts: mocks.NewMockTexts(t), buffs: mocks.NewMockBuffs(t), dice: mocks.NewMockDice(t),
	}
	chars := mocks.NewMockCharacters(t)
	bard := viewtest.Single("bard", bardLevel, viewtest.WithIdentity(charID, "Test", ""), viewtest.WithScores(12, 0, 0, 0, 0, 0))
	bard.Entries[0].Group = "rogue"
	chars.EXPECT().View(mock.Anything, charID).Return(bard, nil).Maybe()
	log := mocks.NewMockLog(t)
	log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, e actionlog.NewEntry) (int64, error) {
			w.payload = e.Payload
			return entryID, nil
		}).Once()
	log.EXPECT().HPTotal(mock.Anything, charID).Return(30, nil).Maybe()
	svc, err := castsvc.New(castsvc.Deps{
		Slots: w.slots, Effects: effects, Texts: w.texts, Buffs: w.buffs, Initiative: noInitiative(t),
		Items: mocks.NewMockItems(t), Attacks: mocks.NewMockAttacks(t),
		Characters: chars, Dice: w.dice, Log: log, Tx: noTx{},
	})
	require.NoError(t, err)
	r := chi.NewRouter()
	casthdl.RegisterRoutes(r, func(*http.Request) int64 { return charID }, svc)
	w.router = r
	return w
}

// post sends body to path and returns the answer without the encoder's
// final newline.
func (w *castWire) post(t *testing.T, path, body string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	w.router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return string(bytes.TrimSuffix(rec.Body.Bytes(), []byte("\n")))
}

// answerTail checks that answer is the logged payload byte for byte, then
// the entry id, and returns the keys that follow (the answer's extras).
func answerTail(t *testing.T, payload json.RawMessage, answer string) map[string]json.RawMessage {
	t.Helper()
	require.True(t, bytes.HasSuffix(payload, []byte("}")))
	assert.NotContains(t, string(payload), "logEntryId", "the payload carries no entry id")
	head := string(payload[:len(payload)-1]) + `,"logEntryId":42`
	require.True(t, strings.HasPrefix(answer, head), "the answer starts with the logged payload\npayload: %s\nanswer:  %s", payload, answer)
	var tail map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte("{"+strings.TrimPrefix(answer[len(head):], ",")), &tail))
	return tail
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

// The cast's answer and its log entry's payload are one struct
// (castsvc.CastBreakdown): the bytes the endpoint sends are the bytes it
// logged, plus the entry's id, then the catalogue spell and the slots left
// (never logged). Checked on a damage cast from a slot (ten d6, the save noted).
func TestCast_AnswerIsTheLoggedPayload(t *testing.T) {
	w := newCastWire(t)
	slots := []spellsvc.SpellSlot{{Level: 3, SpellID: "fireball", PreparedSpellName: "Fireball"}}
	w.slots.EXPECT().GetSlots(mock.Anything, charID).Return(slots, nil).Once()
	w.slots.EXPECT().SaveSlots(mock.Anything, charID, mock.Anything).Return(nil).Once()
	spell := spellsvc.Spell{ID: "fireball", Name: localized.Of("Fireball"), Class: "wizard", Level: 3, Description: localized.Of("A burst of flame.")}
	w.texts.EXPECT().Describe(mock.Anything, "fireball").Return(spell, true, nil).Times(2) // the text, then the casting time (ADR 0035 item 6.2)
	for _, v := range []int{6, 5, 4, 3, 2, 1, 6, 5, 4, 3} {
		w.dice.EXPECT().RollDie(6).Return(v).Once()
	}

	answer := w.post(t, "/api/spells/0/cast", "")
	require.NotEmpty(t, w.payload)
	tail := answerTail(t, w.payload, answer)
	for _, key := range []string{`"damage":`, `"rolls":`, `"save":`, `"slotIndex":0`, `"level":3`} {
		assert.Contains(t, string(w.payload), key)
	}
	assert.ElementsMatch(t, []string{"catalogueSpell", "slots"}, keysOf(tail))
	assert.JSONEq(t, marshal(t, wire.SpellToDTO(spell, bardLevel)), string(tail["catalogueSpell"]))
	assert.JSONEq(t, `[{"level":3,"prepared_spell_id":"fireball","prepared_spell_name":"Fireball","used":true}]`, string(tail["slots"]))
}

// The same on a received buff spell: the buff's rolls and tracked effects
// are logged, and the answer adds no slots (none spent).
func TestReceive_AnswerIsTheLoggedPayload(t *testing.T) {
	w := newCastWire(t)
	w.texts.EXPECT().Describe(mock.Anything, "strength").Return(spellsvc.Spell{ID: "strength", Name: localized.Of("Strength")}, true, nil).Once()
	w.buffs.EXPECT().Add(mock.Anything, charID, mock.Anything).RunAndReturn(
		func(_ context.Context, _ int64, b buffsvc.BuffEffect) (buffsvc.BuffEffect, actionlog.Snapshot, error) {
			b.ID = "7"
			return b, actionlog.Snapshot{Key: buffsvc.StateKeyBuffs, Before: json.RawMessage(`[]`), After: json.RawMessage(`[7]`)}, nil
		}).Once()
	w.dice.EXPECT().RollDie(6).Return(4).Once()

	answer := w.post(t, "/api/spells/receive", `{"spellId":"strength","casterLevel":9}`)
	tail := answerTail(t, w.payload, answer)
	for _, key := range []string{`"durationRoll":`, `"strGainRoll":`, `"effects":{`, `"untracked":[]`} {
		assert.Contains(t, string(w.payload), key)
	}
	assert.ElementsMatch(t, []string{"catalogueSpell"}, keysOf(tail))
}

// The exposure's answer and its entry's payload are one struct
// (castsvc.ExposureBreakdown): the logged bytes, the entry id and the HP the
// ledger applied.
func TestExpose_AnswerIsTheLoggedPayload(t *testing.T) {
	w := newCastWire(t)
	w.dice.EXPECT().RollDie(mock.Anything).Return(3).Maybe()

	answer := w.post(t, "/api/spells/exposure", `{"spellId":"wall_of_fire","casterLevel":12}`)
	tail := answerTail(t, w.payload, answer)
	assert.Contains(t, string(w.payload), `"exposure":{`, "the entry records the exposure served")
	assert.ElementsMatch(t, []string{"hpDelta"}, keysOf(tail))
	assert.JSONEq(t, `0`, string(tail["hpDelta"]), "not on the Bard")
}

// The spell in a cast result carries the caster level of that cast and the
// values computed for it: the Bard's own level for a self-cast, the party
// member's for a received spell.
func TestCastAnswer_CatalogueSpellUsesTheCastersLevel(t *testing.T) {
	spell := spellsvc.Spell{
		Name: localized.Of("Mirror Image"),
		Stats: spellstats.Parse(spellstats.Raw{
			Range: "60 yds. + 10 yds./level", Duration: "3 rds./level", SavingThrow: "None",
		}),
	}
	tests := []struct {
		name         string
		path, body   string
		level        int
		wantDuration float64
		wantRange    float64
	}{
		{"self-cast at the character's level", "/api/spells/0/cast", "", 15, 45, 210},
		{"received from a level 9 caster", "/api/spells/receive", `{"spellId":"mirror_image","casterLevel":9}`, 9, 27, 150},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := fakeCast{result: castsvc.CastResult{CasterLevel: tt.level, Spell: spell}}
			var got struct {
				CasterLevel    int            `json:"casterLevel"`
				CatalogueSpell *wire.SpellDTO `json:"catalogueSpell"`
			}
			require.NoError(t, json.Unmarshal([]byte(serve(t, svc, tt.path, tt.body)), &got))
			assert.Equal(t, tt.level, got.CasterLevel)
			require.NotNil(t, got.CatalogueSpell)
			assert.Equal(t, tt.level, got.CatalogueSpell.CasterLevel)
			require.NotNil(t, got.CatalogueSpell.Values)
			assert.Equal(t, tt.wantDuration, got.CatalogueSpell.Values.Duration.Amount)
			assert.Equal(t, tt.wantRange, got.CatalogueSpell.Values.Range.Amount)
		})
	}
}

// fakeCast records the requests and answers with result (the mode it was
// given for a cast and a receive).
type fakeCast struct {
	result  castsvc.CastResult
	cast    *castsvc.CastRequest
	receive *castsvc.ReceiveRequest
	expose  *castsvc.ExposureRequest
	effect  *castsvc.AddEffectRequest
}

func (f fakeCast) Expose(_ context.Context, _ int64, req castsvc.ExposureRequest) (castsvc.ExposureResult, error) {
	*f.expose = req
	return castsvc.ExposureResult{
		EntryID: 7, SpellID: req.SpellID, Name: "Wall of Fire", CasterLevel: req.CasterLevel, Mode: req.Mode,
		Round: req.Round, OnSelf: req.OnSelf, HPDelta: -4,
		Damage: &castsvc.DamageResult{
			Dice: "2d4", Type: "fire", Instances: 1, Rounds: 1, Total: 5,
			Rolls: []castsvc.Roll{{Formula: "2d4", Dice: []castsvc.DieRoll{{Sides: 4, Value: 2}, {Sides: 4, Value: 3}}, Total: 5}},
		},
	}, nil
}

func (f fakeCast) Cast(_ context.Context, _ int64, _ int, req castsvc.CastRequest) (castsvc.CastResult, error) {
	if f.cast != nil {
		*f.cast = req
	}
	res := f.result
	res.Source, res.Mode = castsvc.SourceCast, req.Mode
	return res, nil
}

func (f fakeCast) AddEffect(_ context.Context, _ int64, req castsvc.AddEffectRequest) (castsvc.CastResult, error) {
	*f.effect = req
	return castsvc.CastResult{EntryID: 3, Source: castsvc.SourceEffect, SpellID: req.SpellID, CasterLevel: 15}, nil
}

func (f fakeCast) Receive(_ context.Context, _ int64, req castsvc.ReceiveRequest) (castsvc.CastResult, error) {
	if f.receive != nil {
		*f.receive = req
	}
	res := f.result
	res.Source, res.Mode = castsvc.SourceReceived, req.Mode
	return res, nil
}

// serve sends body to path on the cast routes over svc and returns the
// answer.
func serve(t *testing.T, svc fakeCast, path, body string) string {
	t.Helper()
	charID := func(*http.Request) int64 { return 1 }
	r := chi.NewRouter()
	r.Post("/api/spells/{id}/cast", render.Handle(casthdl.Cast(charID, svc)))
	r.Post("/api/spells/receive", render.Handle(casthdl.Receive(charID, svc)))
	r.Post("/api/spells/effect", render.Handle(casthdl.AddEffect(charID, svc)))
	r.Post("/api/spells/exposure", render.Handle(casthdl.Expose(charID, svc)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return rec.Body.String()
}

// ADR 0008 step 14: the cast and receive bodies carry the mode key to the
// service, and the result names the mode the spell was cast in; a spell
// without modes serves none.
func TestCastAndReceive_CarryTheMode(t *testing.T) {
	svc := fakeCast{cast: &castsvc.CastRequest{}, receive: &castsvc.ReceiveRequest{}}

	body := serve(t, svc, "/api/spells/0/cast", `{"mode":"ice_sheet"}`)
	assert.Equal(t, "ice_sheet", svc.cast.Mode)
	assert.Contains(t, body, `"mode":"ice_sheet"`)

	body = serve(t, svc, "/api/spells/receive", `{"spellId":"ice_storm","casterLevel":9,"mode":"sleet"}`)
	assert.Equal(t, castsvc.ReceiveRequest{SpellID: "ice_storm", CasterLevel: 9, Mode: "sleet"}, *svc.receive)
	assert.Contains(t, body, `"mode":"sleet"`)

	assert.NotContains(t, serve(t, svc, "/api/spells/0/cast", ""), `"mode"`)
}

// ADR 0008 step 16: POST /api/spells/exposure carries the spell, caster
// level, mode, round and who is exposed to the service and answers with the
// roll; the cast result serves the exposure damage, absent without one.
func TestExpose_Handler(t *testing.T) {
	svc := fakeCast{expose: &castsvc.ExposureRequest{}}
	body := serve(t, svc, "/api/spells/exposure", `{"spellId":"wall_of_fire","casterLevel":9,"mode":"within_10_ft","round":2,"onSelf":true}`)
	assert.Equal(t, castsvc.ExposureRequest{SpellID: "wall_of_fire", CasterLevel: 9, Mode: "within_10_ft", Round: 2, OnSelf: true}, *svc.expose)
	assert.JSONEq(t, `{"spellId":"wall_of_fire","spell":"Wall of Fire","casterLevel":9,"mode":"within_10_ft",
		"round":2,"onSelf":true,"damage":{"dice":"2d4","type":"fire","instances":1,"rounds":1,"total":5,
		"rolls":[{"formula":"2d4","dice":[{"sides":4,"value":2},{"sides":4,"value":3}],"total":5}]},
		"logEntryId":7,"hpDelta":-4}`, body)

	rec := httptest.NewRecorder()
	r := chi.NewRouter()
	r.Post("/api/spells/exposure", render.Handle(casthdl.Expose(func(*http.Request) int64 { return 1 }, svc)))
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/spells/exposure", strings.NewReader(`{`)))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a malformed body")

	withExposure := fakeCast{result: castsvc.CastResult{Exposure: &spelleffectsvc.ExposureAt{
		Modes: []spelleffectsvc.ModeAt{{Key: "within_10_ft", Damage: &spelleffectsvc.DamageAt{
			CasterLevel: 9, Dice: formula.Sum{Dice: []formula.Dice{{Count: 2, Sides: 4}}},
			Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1}, Type: "fire",
		}}},
	}}}
	var got struct {
		Exposure json.RawMessage `json:"exposure"`
	}
	require.NoError(t, json.Unmarshal([]byte(serve(t, withExposure, "/api/spells/0/cast", "")), &got))
	assert.JSONEq(t, `{"modes":[{"key":"within_10_ft","damage":{"caster_level":9,"type":"fire",
		"dice":{"groups":[{"count":2,"sides":4}],"modifier":0},
		"instances":{"groups":[],"modifier":1},"rounds":{"groups":[],"modifier":1}}}]}`, string(got.Exposure))
	assert.NotContains(t, serve(t, fakeCast{}, "/api/spells/0/cast", ""), `"exposure"`)
}

// ADR 0004 step 19: POST /api/spells/effect carries only the spell to the
// service (the level is the Bard's, read server-side) and answers with the
// cast result, action "effect".
func TestAddEffect_Handler(t *testing.T) {
	svc := fakeCast{effect: &castsvc.AddEffectRequest{}}
	body := serve(t, svc, "/api/spells/effect", `{"spellId":"strength","casterLevel":3}`)
	assert.Equal(t, castsvc.AddEffectRequest{SpellID: "strength"}, *svc.effect, "a caster level sent is ignored")
	assert.JSONEq(t, `{"action":"effect","spellId":"strength","spell":"","casterLevel":15,"level":0,"untracked":[],
		"logEntryId":3}`, body)

	rec := httptest.NewRecorder()
	r := chi.NewRouter()
	r.Post("/api/spells/effect", render.Handle(casthdl.AddEffect(func(*http.Request) int64 { return 1 }, svc)))
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/spells/effect", strings.NewReader(`{`)))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a malformed body")
}

// noInitiative is the initiative port out of combat: a cast corrects
// nothing (ADR 0035 item 6.2).
func noInitiative(t *testing.T) *mocks.MockInitiative {
	i := mocks.NewMockInitiative(t)
	i.EXPECT().CorrectInitiative(mock.Anything, charID, mock.Anything).Return(nil, nil).Maybe()
	return i
}
