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

type fakeSvc struct {
	spells []spellsvc.Spell
	got    *spellsvc.Filter
	gotReq *spellsvc.PageRequest
}

func (fakeSvc) GetSlots(context.Context, int64) ([]spellsvc.SpellSlot, error)   { return nil, nil }
func (fakeSvc) ResetSlots(context.Context, int64) ([]spellsvc.SpellSlot, error) { return nil, nil }
func (f fakeSvc) Search(_ context.Context, flt spellsvc.Filter, p spellsvc.PageRequest) (spellsvc.Page, error) {
	if f.got != nil {
		*f.got = flt
	}
	if f.gotReq != nil {
		*f.gotReq = p
	}
	return spellsvc.Page{Items: f.spells, Total: len(f.spells) + 40, Page: p.Page, PageSize: p.PageSize}, nil
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
	var page spellhdl.SpellPageDTO
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	}
	return rec.Code, page.Items
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

func filterFor(t *testing.T, url string) (int, spellsvc.Filter) {
	t.Helper()
	var got spellsvc.Filter
	h := render.Handle(spellhdl.Search(func(*http.Request) int64 { return 1 }, fakeSvc{got: &got}, fakeChars{level: 5}))
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec.Code, got
}

func TestSearch_ReadsEveryFilterFromTheQuery(t *testing.T) {
	code, f := filterFor(t, "/api/spells?q=fire&class=Wizard&level=3&school=Evocation&components=V,s&save=1/2&casting_time=Round")
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, f.Level)
	assert.Equal(t, 3, *f.Level)
	f.Level = nil
	assert.Equal(t, spellsvc.Filter{
		Query: "fire", Class: "wizard", School: "Evocation",
		Components: []string{"V", "S"}, Save: spellstats.SaveHalf, CastingTime: "round",
	}, f)
}

func TestSearch_NoFilterParamsMeansNoFilter(t *testing.T) {
	code, f := filterFor(t, "/api/spells")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, spellsvc.Filter{}, f)
	code, f = filterFor(t, "/api/spells?class=both")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, spellsvc.Filter{}, f, "both classes is no class filter")
}

func TestSearch_SaveSpellings(t *testing.T) {
	for in, want := range map[string]string{"none": "none", "neg": "negates", "Neg.": "negates", "1/2": "half", "special": "special"} {
		code, f := filterFor(t, "/api/spells?save="+in)
		require.Equal(t, http.StatusOK, code, in)
		assert.Equal(t, want, f.Save, in)
	}
}

func TestSearch_MalformedFilterIs400(t *testing.T) {
	for _, q := range []string{"class=bard", "level=x", "level=-1", "components=V,Z", "save=maybe", "casting_time=soon"} {
		code, _ := filterFor(t, "/api/spells?"+q)
		assert.Equal(t, http.StatusBadRequest, code, q)
	}
}

func pageFor(t *testing.T, url string) (int, spellsvc.PageRequest, spellhdl.SpellPageDTO) {
	t.Helper()
	var got spellsvc.PageRequest
	h := render.Handle(spellhdl.Search(func(*http.Request) int64 { return 1 }, fakeSvc{gotReq: &got}, fakeChars{level: 5}))
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, url, nil))
	var out spellhdl.SpellPageDTO
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	}
	return rec.Code, got, out
}

func TestSearch_PagingParams(t *testing.T) {
	code, req, out := pageFor(t, "/api/spells")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, spellsvc.PageRequest{Page: 1, PageSize: 50}, req, "defaults")
	assert.Equal(t, 40, out.Total, "the service's total is passed through")
	assert.Equal(t, 1, out.Page)
	assert.Equal(t, 50, out.PageSize)
	assert.NotNil(t, out.Items)

	code, req, _ = pageFor(t, "/api/spells?page=3&page_size=200")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, spellsvc.PageRequest{Page: 3, PageSize: 200}, req)
}

func TestSearch_BadPagingIs400(t *testing.T) {
	for _, q := range []string{"page=0", "page=-1", "page=x", "page_size=0", "page_size=201", "page_size=-5", "page_size=abc"} {
		code, _, _ := pageFor(t, "/api/spells?"+q)
		assert.Equal(t, http.StatusBadRequest, code, q)
	}
}
