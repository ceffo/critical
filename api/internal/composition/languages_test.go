package composition_test

// The language catalogue end to end over the in-memory storage and the fixture catalogue (ADR 0032
// step 4): the creation options, the native language, the codes the server accepts and refuses and
// the warning over PHB Table 4's count.

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type languagesView struct {
	Languages []string `json:"languages"`
	Warnings  []struct {
		Code  string `json:"code"`
		Field string `json:"field"`
		Count *int   `json:"count"`
		Limit *int   `json:"limit"`
	} `json:"warnings"`
}

// Step 4.1 to 4.3 through the API: a half-elf bard (Intelligence 14: four additional languages)
// gets Common without picking it; a language outside the race's list is accepted; a free text, a
// repeat and a code of no catalogue language are refused with the field and the code, storing
// nothing; five additional languages are accepted with the warning carrying count and limit, and
// the native one stays however the list is edited.
func TestLanguages_Step4_NativeAnyCatalogueLanguageRefusalsAndTheWarning(t *testing.T) {
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
	assert.Empty(t, read().Warnings)

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

	// Step 4.2: five additional languages at Intelligence 14 (limit 4) are accepted with a warning.
	require.Equal(t, http.StatusOK, tb.showing(bob, lyra, "PATCH", "/api/character/details",
		`{"languages":["elvish","gnomish","halfling","goblin","orcish"]}`, nil))
	got := read()
	assert.Equal(t, []string{"common", "elvish", "gnomish", "halfling", "goblin", "orcish"}, got.Languages)
	require.Len(t, got.Warnings, 1)
	assert.Equal(t, "languages_over_limit", got.Warnings[0].Code)
	assert.Equal(t, "languages", got.Warnings[0].Field)
	require.NotNil(t, got.Warnings[0].Count)
	require.NotNil(t, got.Warnings[0].Limit)
	assert.Equal(t, 5, *got.Warnings[0].Count, "the native language is not counted")
	assert.Equal(t, 4, *got.Warnings[0].Limit)

	// Step 4.1: the native language cannot be removed.
	require.Equal(t, http.StatusOK, tb.showing(bob, lyra, "PATCH", "/api/character/details", `{"languages":null}`, nil))
	got = read()
	assert.Equal(t, []string{"common"}, got.Languages)
	assert.Empty(t, got.Warnings)
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
