package composition_test

// The language catalogue end to end over the in-memory storage and the fixture catalogue (ADR 0032
// step 4): the creation options, the native language, the codes the server accepts and refuses and
// the languages spending non-weapon slots (ADR 0037 step 6).

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type languagesView struct {
	Languages []string      `json:"languages"`
	Warnings  []warningView `json:"warnings"`
	Derived   struct {
		ProficiencySlots struct {
			NonWeapon struct {
				Intelligence int `json:"intelligence"`
				Available    int `json:"available"`
				Spent        int `json:"spent"`
				Languages    int `json:"languages"`
			} `json:"nonWeapon"`
		} `json:"proficiencySlots"`
	} `json:"derived"`
}

// Step 4.1 to 4.3 through the API: a half-elf bard (level 3, Intelligence 14: 3 + 0 + 4 = 7
// non-weapon slots) gets Common without picking it; a language outside the race's list is accepted;
// a free text, a repeat and a code of no catalogue language are refused with the field and the
// code, storing nothing; ADR 0037 step 6: five other languages spend 5 of the 7 slots (Table 4
// counted once, no languages_over_limit), eight are refused over budget, and the native one stays
// however the list is edited.
func TestLanguages_Step4_NativeAnyCatalogueLanguageRefusalsAndTheSlots(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	var list listView
	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", newBard, &list))
	lyra := *list.CurrentID

	read := func() languagesView {
		var got languagesView
		require.Equal(t, http.StatusOK, tb.showing(bob, lyra, "GET", "/api/character", "", &got))
		return got
	}
	assert.Equal(t, []string{"common"}, read().Languages, "step 4.1: the native language without picking it")
	assert.Empty(t, withoutSlots(read().Warnings))

	// Step 4.3: the burrowers' speech is on no half-elf list, and it is accepted.
	require.Equal(t, http.StatusOK, tb.showing(bob, lyra, "PATCH", "/api/character/details", `{"languages":["burrowers","elvish"]}`, nil))
	assert.Equal(t, []string{"common", "burrowers", "elvish"}, read().Languages)

	for body, code := range map[string]string{
		`{"languages":["Elfique"]}`:            "unknown",
		`{"languages":["draconic"]}`:           "unknown",
		`{"languages":["goblin","goblin"]}`:    "duplicate",
		`{"languages":["goblin",""]}`:          "empty",
		`{"languages":["common","dwarvish "]}`: "unknown",
	} {
		var refused map[string]string
		require.Equal(t, http.StatusBadRequest, tb.showing(bob, lyra, "PATCH", "/api/character/details", body, &refused), body)
		assert.Equal(t, "languages", refused["field"], body)
		assert.Equal(t, code, refused["error"], body)
	}
	assert.Equal(t, []string{"common", "burrowers", "elvish"}, read().Languages, "a refusal stores nothing")

	// ADR 0037 step 6: five other languages spend five of the seven slots, with no warning of their own.
	require.Equal(t, http.StatusOK, tb.showing(bob, lyra, "PATCH", "/api/character/details",
		`{"languages":["elvish","gnomish","halfling","goblin","orcish"]}`, nil))
	got := read()
	assert.Equal(t, []string{"common", "elvish", "gnomish", "halfling", "goblin", "orcish"}, got.Languages)
	assert.Empty(t, withoutSlots(got.Warnings), "no languages_over_limit any more")
	pool := got.Derived.ProficiencySlots.NonWeapon
	assert.Equal(t, 4, pool.Intelligence, "Table 4's count is the pool's Intelligence term, once")
	assert.Equal(t, 7, pool.Available)
	assert.Equal(t, 5, pool.Languages)
	assert.Equal(t, 5, pool.Spent)

	// Eight are over the seven slots: refused (409 over_budget on languages), nothing stored.
	var refused map[string]any
	require.Equal(t, http.StatusConflict, tb.showing(bob, lyra, "PATCH", "/api/character/details",
		`{"languages":["elvish","gnomish","halfling","goblin","orcish","hobgoblin","gnoll","dwarvish"]}`, &refused))
	assert.Equal(t, "over_budget", refused["code"])
	assert.Equal(t, "languages", refused["field"])
	assert.Equal(t, float64(8), refused["spent"])
	assert.Equal(t, float64(7), refused["limit"])
	assert.Len(t, read().Languages, 6, "a refusal stores nothing")

	// Step 4.1: the native language cannot be removed.
	require.Equal(t, http.StatusOK, tb.showing(bob, lyra, "PATCH", "/api/character/details", `{"languages":null}`, nil))
	got = read()
	assert.Equal(t, []string{"common"}, got.Languages)
	assert.Empty(t, withoutSlots(got.Warnings))
}

// Step 4.3: the creation options hold the catalogue with its names, each race's own language and
// list, and PHB Table 4 as bands of Intelligence scores.
func TestLanguages_Step4_TheCreationOptionsOfferTheCatalogue(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	var got struct {
		Languages []struct {
			Code string            `json:"code"`
			Name map[string]string `json:"name"`
		} `json:"languages"`
		RaceLanguages []struct {
			Race      string   `json:"race"`
			Native    string   `json:"native"`
			Languages []string `json:"languages"`
		} `json:"raceLanguages"`
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters/options", "", &got))
	require.NotEmpty(t, got.Languages)
	for _, l := range got.Languages {
		assert.NotEmpty(t, l.Name["en"], l.Code)
		assert.NotEmpty(t, l.Name["fr"], l.Code, "en and fr names for every catalogue language")
	}
	require.Len(t, got.RaceLanguages, 6)
	for _, r := range got.RaceLanguages {
		assert.Contains(t, r.Languages, r.Native, r.Race)
	}
}
