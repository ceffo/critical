package casthdl_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/casthdl"
	"github.com/ceffo/critical/api/pkg/formula"
)

// The spell in a cast result carries the caster level of that cast and the
// values computed for it: the Bard's own level for a self-cast, the party
// member's for a received spell.
func TestToDTO_SpellValuesUseTheCastersLevel(t *testing.T) {
	spell := spellsvc.Spell{
		Name: "Mirror Image",
		Stats: spellstats.Parse(spellstats.Raw{
			Range: "60 yds. + 10 yds./level", Duration: "3 rds./level", SavingThrow: "None",
		}),
	}
	tests := []struct {
		name         string
		source       castsvc.Source
		level        int
		wantDuration float64
		wantRange    float64
	}{
		{"self-cast at the character's level", castsvc.SourceCast, 15, 45, 210},
		{"received from a level 9 caster", castsvc.SourceReceived, 9, 27, 150},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dto := casthdl.ToDTO(castsvc.CastResult{Source: tt.source, CasterLevel: tt.level, Spell: spell})
			assert.Equal(t, tt.level, dto.CasterLevel)
			require.NotNil(t, dto.Spell)
			assert.Equal(t, tt.level, dto.Spell.CasterLevel)
			require.NotNil(t, dto.Spell.Values)
			assert.Equal(t, tt.wantDuration, dto.Spell.Values.Duration.Amount)
			assert.Equal(t, tt.wantRange, dto.Spell.Values.Range.Amount)
		})
	}
}

// ADR 0008 step 13: a cast's damage made of parts carries the parts, each
// instance's part rolls and the total on a save; without parts none of them.
func TestToDTO_DamageParts(t *testing.T) {
	onSave := 3
	dice := []castsvc.DieRoll{{Sides: 4, Value: 1}, {Sides: 4, Value: 2}}
	dto := casthdl.ToDTO(castsvc.CastResult{Damage: &castsvc.DamageResult{
		Dice: "2d4+level", Type: "force", Instances: 1, Rounds: 1, Total: 18, TotalOnSave: &onSave,
		Parts: []castsvc.DamagePart{{Formula: "2d4", Save: "none"}, {Formula: "level", Save: "negates"}},
		Rolls: []castsvc.Roll{{
			Formula: "2d4+level", Dice: dice, Total: 18,
			Parts: []castsvc.Roll{{Formula: "2d4", Dice: dice, Total: 3}, {Formula: "level", Total: 15}},
		}},
	}})
	raw, err := json.Marshal(dto.Damage)
	require.NoError(t, err)
	assert.JSONEq(t, `{"dice":"2d4+level","type":"force","instances":1,"rounds":1,"total":18,"totalOnSave":3,
		"parts":[{"formula":"2d4","save":"none"},{"formula":"level","save":"negates"}],
		"rolls":[{"formula":"2d4+level","dice":[{"sides":4,"value":1},{"sides":4,"value":2}],"total":18,"parts":[
			{"formula":"2d4","dice":[{"sides":4,"value":1},{"sides":4,"value":2}],"total":3},
			{"formula":"level","dice":[],"total":15}]}]}`, string(raw))

	raw, err = json.Marshal(casthdl.ToDTO(castsvc.CastResult{Damage: &castsvc.DamageResult{
		Rolls: []castsvc.Roll{{Formula: "1d4"}},
	}}).Damage)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "parts")
	assert.NotContains(t, string(raw), "totalOnSave")
}

// ADR 0008 step 15: a cast's damage made of steps carries the steps' unit and
// each roll's step; without steps neither.
func TestToDTO_DamageSteps(t *testing.T) {
	dto := casthdl.ToDTO(castsvc.CastResult{Damage: &castsvc.DamageResult{
		Type: "acid", Instances: 1, Rounds: 2, Total: 9, StepsPer: spelleffectsvc.StepPerRound,
		Rolls: []castsvc.Roll{
			{Formula: "(max(4-2*(step-1),0))d4", Step: 1, Total: 7, Dice: []castsvc.DieRoll{{Sides: 4, Value: 3}, {Sides: 4, Value: 4}}},
			{Formula: "(max(4-2*(step-1),0))d4", Step: 2, Total: 2, Dice: []castsvc.DieRoll{{Sides: 4, Value: 2}}},
		},
	}})
	raw, err := json.Marshal(dto.Damage)
	require.NoError(t, err)
	assert.JSONEq(t, `{"dice":"","type":"acid","instances":1,"rounds":2,"total":9,"stepsPer":"round","rolls":[
		{"formula":"(max(4-2*(step-1),0))d4","step":1,"dice":[{"sides":4,"value":3},{"sides":4,"value":4}],"total":7},
		{"formula":"(max(4-2*(step-1),0))d4","step":2,"dice":[{"sides":4,"value":2}],"total":2}]}`, string(raw))

	raw, err = json.Marshal(casthdl.ToDTO(castsvc.CastResult{Damage: &castsvc.DamageResult{
		Rolls: []castsvc.Roll{{Formula: "1d4"}},
	}}).Damage)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "step")
}

// fakeCast records the requests and answers with the mode it was given.
type fakeCast struct {
	cast    *castsvc.CastRequest
	receive *castsvc.ReceiveRequest
	expose  *castsvc.ExposureRequest
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
	*f.cast = req
	return castsvc.CastResult{Source: castsvc.SourceCast, Mode: req.Mode}, nil
}

func (f fakeCast) Receive(_ context.Context, _ int64, req castsvc.ReceiveRequest) (castsvc.CastResult, error) {
	*f.receive = req
	return castsvc.CastResult{Source: castsvc.SourceReceived, Mode: req.Mode}, nil
}

// ADR 0008 step 14: the cast and receive bodies carry the mode key to the
// service, and the result names the mode the spell was cast in; a spell
// without modes serves none.
func TestCastAndReceive_CarryTheMode(t *testing.T) {
	svc := fakeCast{cast: &castsvc.CastRequest{}, receive: &castsvc.ReceiveRequest{}}
	charID := func(*http.Request) int64 { return 1 }
	r := chi.NewRouter()
	r.Post("/api/spells/{id}/cast", render.Handle(casthdl.Cast(charID, svc)))
	r.Post("/api/spells/receive", render.Handle(casthdl.Receive(charID, svc)))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/spells/0/cast", strings.NewReader(`{"mode":"ice_sheet"}`)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "ice_sheet", svc.cast.Mode)
	assert.Contains(t, rec.Body.String(), `"mode":"ice_sheet"`)

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/spells/receive", strings.NewReader(`{"spellId":"ice_storm","casterLevel":9,"mode":"sleet"}`)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, castsvc.ReceiveRequest{SpellID: "ice_storm", CasterLevel: 9, Mode: "sleet"}, *svc.receive)
	assert.Contains(t, rec.Body.String(), `"mode":"sleet"`)

	raw, err := json.Marshal(casthdl.ToDTO(castsvc.CastResult{}))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"mode"`)
}

// ADR 0008 step 16: POST /api/spells/exposure carries the spell, caster
// level, mode, round and who is exposed to the service and answers with the
// roll; the cast result serves the exposure damage, absent without one.
func TestExpose_Handler(t *testing.T) {
	svc := fakeCast{expose: &castsvc.ExposureRequest{}}
	r := chi.NewRouter()
	r.Post("/api/spells/exposure", render.Handle(casthdl.Expose(func(*http.Request) int64 { return 1 }, svc)))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/spells/exposure",
		strings.NewReader(`{"spellId":"wall_of_fire","casterLevel":9,"mode":"within_10_ft","round":2,"onSelf":true}`)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, castsvc.ExposureRequest{SpellID: "wall_of_fire", CasterLevel: 9, Mode: "within_10_ft", Round: 2, OnSelf: true}, *svc.expose)
	assert.JSONEq(t, `{"entryId":7,"spellId":"wall_of_fire","name":"Wall of Fire","casterLevel":9,"mode":"within_10_ft",
		"round":2,"onSelf":true,"hpDelta":-4,"damage":{"dice":"2d4","type":"fire","instances":1,"rounds":1,"total":5,
		"rolls":[{"formula":"2d4","dice":[{"sides":4,"value":2},{"sides":4,"value":3}],"total":5}]}}`, rec.Body.String())

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/spells/exposure", strings.NewReader(`{`)))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a malformed body")

	raw, err := json.Marshal(casthdl.ToDTO(castsvc.CastResult{Exposure: &spelleffectsvc.ExposureAt{
		Modes: []spelleffectsvc.ModeAt{{Key: "within_10_ft", Damage: &spelleffectsvc.DamageAt{
			CasterLevel: 9, Dice: formula.Sum{Dice: []formula.Dice{{Count: 2, Sides: 4}}},
			Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1}, Type: "fire",
		}}},
	}}))
	require.NoError(t, err)
	var got struct {
		Exposure json.RawMessage `json:"exposure"`
	}
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.JSONEq(t, `{"modes":[{"key":"within_10_ft","damage":{"caster_level":9,"type":"fire",
		"dice":{"groups":[{"count":2,"sides":4}],"modifier":0},
		"instances":{"groups":[],"modifier":1},"rounds":{"groups":[],"modifier":1}}}]}`, string(got.Exposure))
	raw, err = json.Marshal(casthdl.ToDTO(castsvc.CastResult{}))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"exposure"`)
}
