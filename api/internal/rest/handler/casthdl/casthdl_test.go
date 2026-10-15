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
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/casthdl"
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

// fakeCast records the requests and answers with the mode it was given.
type fakeCast struct {
	cast    *castsvc.CastRequest
	receive *castsvc.ReceiveRequest
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
