package spellhdl_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/spellhdl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSvc struct{ spells []spellsvc.Spell }

func (fakeSvc) GetSlots(context.Context, int64) ([]spellsvc.SpellSlot, error)   { return nil, nil }
func (fakeSvc) ResetSlots(context.Context, int64) ([]spellsvc.SpellSlot, error) { return nil, nil }
func (f fakeSvc) Search(context.Context, string, string) ([]spellsvc.Spell, error) {
	return f.spells, nil
}

type fakeChars struct {
	level int
	err   error
}

func (f fakeChars) GetCharacter(context.Context, int64) (*charactersvc.Character, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &charactersvc.Character{Level: f.level}, nil
}

func search(t *testing.T, url string) (int, []spellhdl.SpellDTO) {
	return searchAs(t, url, fakeChars{level: 15})
}

func searchAs(t *testing.T, url string, chars fakeChars) (int, []spellhdl.SpellDTO) {
	t.Helper()
	raw := spellstats.Raw{CastingTime: "1", Range: "60 yds. + 10 yds./level", Duration: "3 rds./level", SavingThrow: "None"}
	svc := fakeSvc{spells: []spellsvc.Spell{{
		Name: "Magic Missile", Range: raw.Range, Duration: raw.Duration, Stats: spellstats.Parse(raw),
	}}}
	h := render.Handle(spellhdl.Search(func(*http.Request) int64 { return 1 }, svc, chars))
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, url, nil))
	var out []spellhdl.SpellDTO
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	}
	return rec.Code, out
}

func TestSearch_ReturnsRawTextTypedStatsAndValuesAtCharacterLevel(t *testing.T) {
	code, out := search(t, "/api/spells")
	require.Equal(t, http.StatusOK, code)
	require.Len(t, out, 1)
	assert.Equal(t, "60 yds. + 10 yds./level", out[0].Range, "raw text kept for existing callers")
	assert.Equal(t, spellstats.RangeDistance, out[0].Stats.Range.Kind)
	assert.Equal(t, 15, out[0].CasterLevel)
	require.NotNil(t, out[0].Values)
	assert.Equal(t, 210.0, out[0].Values.Range.Amount)
	assert.Equal(t, "yard", out[0].Values.Range.Unit)
	assert.Equal(t, 45.0, out[0].Values.Duration.Amount)
}

func TestSearch_CasterLevelQueryOverridesTheCharacter(t *testing.T) {
	code, out := search(t, "/api/spells?caster_level=5")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 5, out[0].CasterLevel)
	require.NotNil(t, out[0].Values)
	assert.Equal(t, 110.0, out[0].Values.Range.Amount)
}

func TestSearch_BadCasterLevelIs400(t *testing.T) {
	for _, q := range []string{"abc", "0", "-3"} {
		code, _ := search(t, "/api/spells?caster_level="+q)
		assert.Equal(t, http.StatusBadRequest, code, q)
	}
}

func TestSearch_CharacterLookupFailureStillReturnsSpellsWithoutValues(t *testing.T) {
	code, out := searchAs(t, "/api/spells", fakeChars{err: errors.New("no character")})
	require.Equal(t, http.StatusOK, code)
	require.Len(t, out, 1)
	assert.Equal(t, "Magic Missile", out[0].Name)
	assert.Nil(t, out[0].Values)
	assert.Equal(t, spellstats.RangeDistance, out[0].Stats.Range.Kind, "typed stats still returned")

	code, out = searchAs(t, "/api/spells?caster_level=15", fakeChars{err: errors.New("no character")})
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, out[0].Values, "explicit level needs no character")
}
