package spellhdl_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/spellhdl"
	"github.com/ceffo/critical/api/pkg/formula"
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

// fakeDamage serves 10d6+2 fire, once per round, at any level for
// "fireball" and nothing for other spells; it records the levels asked. Its
// modes are ice_storm's: hail (3d10 cold) and sleet (no damage).
type fakeDamage struct{ levels *[]int }

func (f fakeDamage) ModesAt(_ context.Context, id string, level int) ([]spelleffectsvc.ModeAt, error) {
	if id != "ice_storm" {
		return nil, nil
	}
	return []spelleffectsvc.ModeAt{
		{Key: "hail", Damage: &spelleffectsvc.DamageAt{
			CasterLevel: level, Dice: formula.Sum{Dice: []formula.Dice{{Count: 3, Sides: 10}}},
			Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1}, Type: "cold",
		}},
		{Key: "sleet"},
	}, nil
}

func (f fakeDamage) DamageAt(_ context.Context, id string, level int) (*spelleffectsvc.DamageAt, error) {
	if f.levels != nil {
		*f.levels = append(*f.levels, level)
	}
	if level > spelleffectsvc.MaxCasterLevel {
		return nil, domain.ErrInvalidInput // as castsvc
	}
	if id != "fireball" {
		return nil, nil
	}
	return &spelleffectsvc.DamageAt{
		CasterLevel: level, Dice: formula.Sum{Dice: []formula.Dice{{Count: 10, Sides: 6}}, Constant: 2},
		Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1}, Type: "fire",
	}, nil
}

func search(t *testing.T, url string) (int, []spellhdl.SpellDTO) {
	return searchAs(t, url, fakeChars{level: 15})
}

func searchAs(t *testing.T, url string, chars fakeChars) (int, []spellhdl.SpellDTO) {
	t.Helper()
	raw := spellstats.Raw{CastingTime: "1", Range: "60 yds. + 10 yds./level", Duration: "3 rds./level", SavingThrow: "None"}
	svc := fakeSvc{spells: []spellsvc.Spell{{
		ID: "magic_missile", Name: "Magic Missile", Range: raw.Range, Duration: raw.Duration, Stats: spellstats.Parse(raw),
	}, {ID: "fireball", Name: "Fireball", Stats: spellstats.Parse(spellstats.Raw{})}}}
	h := render.Handle(spellhdl.Search(func(*http.Request) int64 { return 1 }, svc, chars, fakeDamage{}))
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
	require.Len(t, out, 2)
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
	for _, q := range []string{"abc", "0", "-3", "41"} {
		code, _ := search(t, "/api/spells?caster_level="+q)
		assert.Equal(t, http.StatusBadRequest, code, q)
	}
}

func TestSearch_CharacterLookupFailureStillReturnsSpellsWithoutValues(t *testing.T) {
	code, out := searchAs(t, "/api/spells", fakeChars{err: errors.New("no character")})
	require.Equal(t, http.StatusOK, code)
	require.Len(t, out, 2)
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
	h := render.Handle(spellhdl.Search(func(*http.Request) int64 { return 1 }, fakeSvc{got: &got}, fakeChars{level: 5}, fakeDamage{}))
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec.Code, got
}

func TestSearch_ReadsEveryFilterFromTheQuery(t *testing.T) {
	code, f := filterFor(t, "/api/spells?q=+fire+&class=Wizard&level=3,1,3&school=Evocation&components=V,s&save=1/2&casting_time=Round")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, spellsvc.Filter{
		Query: "fire", Class: "wizard", Levels: []int{3, 1}, School: spellsvc.SchoolEvocation,
		Components: []string{"V", "S"}, Save: spellstats.SaveHalf, CastingTime: "round",
	}, f)
}

func TestSearch_ClientNameMatchesCarryTheirTier(t *testing.T) {
	code, f := filterFor(t, "/api/spells?q=mains&match_exact=fireball&match_prefix=burning_hands,+detect_magic&match_contains=fortify,")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "mains", f.Query)
	assert.Equal(t, []spellsvc.NameMatch{
		{Key: "fireball", Tier: spellsvc.TierExact},
		{Key: "burning_hands", Tier: spellsvc.TierPrefix},
		{Key: "detect_magic", Tier: spellsvc.TierPrefix},
		{Key: "fortify", Tier: spellsvc.TierContains},
	}, f.Names)
}

func TestSearch_ClassValues(t *testing.T) {
	for in, want := range map[string]spellsvc.Filter{
		"mine":   {Class: "wizard", Mine: true},
		"Wizard": {Class: "wizard"},
		"priest": {Class: "priest"},
		"all":    {},
		"both":   {},
	} {
		code, f := filterFor(t, "/api/spells?class="+in)
		require.Equal(t, http.StatusOK, code, in)
		assert.Equal(t, want, f, in)
	}
}

func TestSearch_SphereIsCanonicalised(t *testing.T) {
	code, f := filterFor(t, "/api/spells?class=priest&sphere=elemental+fire")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, spellsvc.Filter{Class: "priest", Sphere: "Elemental Fire"}, f)
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
	for _, q := range []string{
		"class=bard", "level=x", "level=-1", "level=10", "level=1,x", "school=Evoc", "school=Dimension",
		"sphere=Fire", "components=V,Z", "save=maybe", "casting_time=soon",
	} {
		code, _ := filterFor(t, "/api/spells?"+q)
		assert.Equal(t, http.StatusBadRequest, code, q)
	}
}

func pageFor(t *testing.T, url string) (int, spellsvc.PageRequest, spellhdl.SpellPageDTO) {
	t.Helper()
	var got spellsvc.PageRequest
	h := render.Handle(spellhdl.Search(func(*http.Request) int64 { return 1 }, fakeSvc{gotReq: &got}, fakeChars{level: 5}, fakeDamage{}))
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

func TestFilterOptions_ListsSchoolGroupsAndSpheres(t *testing.T) {
	rec := httptest.NewRecorder()
	render.Handle(spellhdl.FilterOptions())(rec, httptest.NewRequest(http.MethodGet, "/api/spells/filters", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var out spellhdl.FilterOptionsDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, []string{
		"abjuration", "alteration", "conjuration", "divination", "enchantment",
		"illusion", "evocation", "necromancy", "other",
	}, out.Schools)
	assert.Contains(t, out.Spheres, "Elemental Fire")
	assert.Contains(t, out.Spheres, "Charm")
}

// The damage a cast rolls is served at the level the values are for, as dice
// groups plus a modifier; a spell that rolls none has no damage.
func TestSearch_ServesDamageAtTheCasterLevel(t *testing.T) {
	code, out := search(t, "/api/spells")
	require.Equal(t, http.StatusOK, code)
	require.Len(t, out, 2)
	assert.Nil(t, out[0].Damage, "magic_missile: the fake serves none")
	assert.Equal(t, &spellhdl.SpellDamageDTO{
		CasterLevel: 15,
		Dice:        spellhdl.DiceSumDTO{Groups: []spellhdl.DiceGroupDTO{{Count: 10, Sides: 6}}, Modifier: 2},
		Instances:   spellhdl.DiceSumDTO{Groups: []spellhdl.DiceGroupDTO{}, Modifier: 1},
		Rounds:      spellhdl.DiceSumDTO{Groups: []spellhdl.DiceGroupDTO{}, Modifier: 1},
		Type:        "fire",
	}, out[1].Damage)

	code, out = search(t, "/api/spells?caster_level=7")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 7, out[1].Damage.CasterLevel)
}

func TestSearch_NoCasterLevelNoDamage(t *testing.T) {
	var levels []int
	h := render.Handle(spellhdl.Search(func(*http.Request) int64 { return 1 },
		fakeSvc{spells: []spellsvc.Spell{{ID: "fireball", Name: "Fireball"}}},
		fakeChars{err: errors.New("no character")}, fakeDamage{levels: &levels}))
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/spells", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var page spellhdl.SpellPageDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	assert.Nil(t, page.Items[0].Damage)
	assert.Empty(t, levels, "no level, no damage lookup")
}

func TestSearch_CharacterLevelAboveTheMaxServesNoDamage(t *testing.T) {
	code, out := searchAs(t, "/api/spells", fakeChars{level: spelleffectsvc.MaxCasterLevel + 1})
	require.Equal(t, http.StatusOK, code, "the page does not fail")
	require.Len(t, out, 2)
	assert.NotNil(t, out[1].Values)
	assert.Nil(t, out[1].Damage)
}

// ADR 0008 step 13: the served damage carries each part's dice and save
// effect key; a damage without parts serves none.
func TestDamageToDTO_Parts(t *testing.T) {
	d2d4 := formula.Sum{Dice: []formula.Dice{{Count: 2, Sides: 4}}}
	got := spellhdl.DamageToDTO(&spelleffectsvc.DamageAt{
		CasterLevel: 15, Dice: formula.Sum{Dice: d2d4.Dice, Constant: 15},
		Instances: formula.Sum{Constant: 3}, Rounds: formula.Sum{Constant: 1}, Type: "force",
		Parts: []spelleffectsvc.DamagePartAt{
			{Dice: d2d4, Save: spelleffectsvc.SaveNone},
			{Dice: formula.Sum{Constant: 15}, Save: spelleffectsvc.SaveNegates},
		},
	})
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	assert.JSONEq(t, `{"caster_level":15,"type":"force",
		"dice":{"groups":[{"count":2,"sides":4}],"modifier":15},
		"instances":{"groups":[],"modifier":3},"rounds":{"groups":[],"modifier":1},
		"parts":[{"dice":{"groups":[{"count":2,"sides":4}],"modifier":0},"save":"none"},
		         {"dice":{"groups":[],"modifier":15},"save":"negates"}]}`, string(raw))

	raw, err = json.Marshal(spellhdl.DamageToDTO(&spelleffectsvc.DamageAt{CasterLevel: 1}))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "parts")
}

// ADR 0008 step 14: the lookup serves every cast-time mode with the damage a
// cast in it rolls at the caster level (none for a mode without damage); a
// spell without modes serves none.
func TestSearch_ServesModesAtTheCasterLevel(t *testing.T) {
	h := render.Handle(spellhdl.Search(func(*http.Request) int64 { return 1 },
		fakeSvc{spells: []spellsvc.Spell{{ID: "ice_storm", Name: "Ice Storm"}, {ID: "fireball", Name: "Fireball"}}},
		fakeChars{level: 15}, fakeDamage{}))
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/spells?caster_level=9", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var page struct {
		Items []struct {
			ID    string          `json:"id"`
			Modes json.RawMessage `json:"modes"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page.Items, 2)
	assert.JSONEq(t, `[
		{"key":"hail","damage":{"caster_level":9,"type":"cold","dice":{"groups":[{"count":3,"sides":10}],"modifier":0},
			"instances":{"groups":[],"modifier":1},"rounds":{"groups":[],"modifier":1}}},
		{"key":"sleet"}]`, string(page.Items[0].Modes))
	assert.Nil(t, page.Items[1].Modes, "fireball has no modes")
}
