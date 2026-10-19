package composition_test

// The creation preview end to end (ADR 0034, the identity step and the
// review): POST /api/characters/preview answers what POST /api/characters
// would store, read as GET /api/character reads it, and stores nothing.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// previewView is the part of a preview that a created character's read
// carries too: the derived block, the warnings and the languages.
type previewView struct {
	Derived   map[string]any   `json:"derived"`
	Warnings  []map[string]any `json:"warnings"`
	Languages []string         `json:"languages"`
	Missing   []string         `json:"missing"`
}

// The preview of a body is what a read of the character created from it
// shows: the same derived block (level, THAC0, saves, slots, skills, budget,
// no sheet difference) and the same warnings and languages, for every
// creatable class; and previewing stores nothing.
func TestPreview_IsWhatTheCreatedCharacterReads(t *testing.T) {
	bodies := map[string]string{
		// Five additional languages at Intelligence 14 (limit 4), a height
		// outside the half-elf male range, bard points over the budget.
		"bard": bardStart[:len(bardStart)-len(`,"skillArmor":"leather"`)] +
			`,"skillArmor":"padded_studded_leather","skillPoints":{"climb_walls":60,"detect_noise":30},"details":{"alignment":"neutral_good","sex":"male",` +
			`"languages":["elvish","gnomish","halfling","goblin","orcish"],"height_cm":250,"weight_kg":70,"age":20}}`,
		"fighter": `{"name":"Brom","class":"fighter","raceCode":"human","str":18,"strExceptional":76,"dex":12,"con":16,"int":9,"wis":10,"cha":8,` +
			`"hpMax":28,"xp":9000,"details":{"alignment":"lawful_good","sex":"male","age":200}}`,
		"thief": `{"name":"Nib","class":"thief","raceCode":"halfling","str":9,"dex":17,"con":12,"int":11,"wis":10,"cha":12,"hpMax":4,"xp":0,` +
			`"skillArmor":"none","skillPoints":{"pick_pockets":30,"open_locks":30},"details":{"alignment":"chaotic_neutral","sex":"female"}}`,
		"mage": `{"name":"Ivo","class":"mage","raceCode":"elf","str":8,"dex":14,"con":10,"int":17,"wis":12,"cha":11,"hpMax":3,"xp":2600,` +
			`"details":{"alignment":"true_neutral","sex":"male"}}`,
		"cleric": `{"name":"Sol","class":"cleric","raceCode":"dwarf","str":12,"dex":9,"con":15,"int":10,"wis":16,"cha":10,"hpMax":20,"xp":13000,` +
			`"details":{"alignment":"lawful_good","sex":"female","weight_kg":60}}`,
	}
	for class, body := range bodies {
		t.Run(class, func(t *testing.T) {
			tb, _ := newTable(t)
			bob, _ := tb.login("bob@example.com")

			var preview previewView
			require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/preview", body, &preview), body)
			assert.Empty(t, preview.Missing)
			var list listView
			require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters", "", &list))
			assert.Empty(t, list.Characters, "a preview stores nothing")

			require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", body, &list), body)
			var read previewView
			require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "GET", "/api/character", "", &read))
			assert.Equal(t, read.Derived, preview.Derived)
			assert.Equal(t, read.Warnings, preview.Warnings)
			assert.Equal(t, read.Languages, preview.Languages)
		})
	}
}

// The bard body above warns three times; the test above would pass on two
// empty answers, so the warnings are named here.
func TestPreview_CarriesTheWarnings(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	body := bardWith(`{"alignment":"neutral_good","sex":"male","languages":["elvish","gnomish","halfling","goblin","orcish"],"height_cm":250,"age":16}`)
	var preview previewView
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/preview", body, &preview))
	codes := []string{}
	for _, w := range preview.Warnings {
		codes = append(codes, w["code"].(string))
	}
	assert.ElementsMatch(t, []string{"height_outside_range", "languages_over_limit"}, codes)
	assert.Equal(t, []string{"common", "elvish", "gnomish", "halfling", "goblin", "orcish"}, preview.Languages, "the native language added")
	assert.InDelta(t, 3.0, preview.Derived["targetLevel"], 0, "2,500 XP is a level 3 bard")
	assert.InDelta(t, 19.0, preview.Derived["thac0"], 0)
}

// A blank name, alignment or sex is no refusal: the preview lists them as
// missing and still answers the figures and the warnings, so the identity
// step shows the languages' warning before the player picks the rest.
func TestPreview_ABlankNameAlignmentOrSexIsMissingNotRefused(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	body := `{"name":"  ","class":"bard","raceCode":"half_elf","str":10,"dex":15,"con":12,"int":9,"wis":9,"cha":16,"hpMax":6,"xp":0,` +
		`"skillArmor":"leather","details":{"alignment":"","sex":"","languages":["elvish","gnomish","goblin"]}}`
	var preview previewView
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/preview", body, &preview))
	assert.Equal(t, []string{"name", "alignment", "sex"}, preview.Missing)
	require.Len(t, preview.Warnings, 1)
	assert.Equal(t, "languages_over_limit", preview.Warnings[0]["code"])

	var refused map[string]string
	require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", body, &refused))
	assert.Equal(t, "name", refused["field"], "the creation still refuses a blank name")
}

// Every other refusal is the creation's, field and code alike.
func TestPreview_RefusesAsTheCreationRefuses(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	for _, body := range []string{
		bardWith(`{"alignment":"lawful_good","sex":"male"}`),
		bardWith(`{"alignment":"good","sex":"male"}`),
		bardWith(`{"alignment":"neutral_good","sex":"other"}`),
		bardWith(`{"alignment":"neutral_good","sex":"male","languages":["Elfique"]}`),
		bardWith(`{"alignment":"neutral_good","sex":"male","height_cm":400}`),
		bardWith(`{"alignment":"neutral_good","sex":"male","height_cm":"tall"}`),
		bardWith(`{"alignment":"neutral_good","sex":"male","age":0}`),
		`{"name":"D","class":"bard","raceCode":"dwarf","str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10,"hpMax":6,"xp":0}`,
		`{"name":"D","class":"bard","raceCode":"human","str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":20,"hpMax":6,"xp":0}`,
		`{"name":"D","class":"bard","raceCode":"human","str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10,"xp":0}`,
		`{"name":"0123456789012345678901234567890123456789X","class":"bard","raceCode":"human","str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10,"hpMax":6,"xp":0}`,
		bardStart + `,"level":3}`,
	} {
		var fromPreview, fromCreate map[string]string
		pst := tb.call(bob, "POST", "/api/characters/preview", body, &fromPreview)
		cst := tb.call(bob, "POST", "/api/characters", body, &fromCreate)
		require.Equal(t, http.StatusBadRequest, cst, body)
		assert.Equal(t, cst, pst, body)
		assert.Equal(t, fromCreate, fromPreview, body)
	}
}

// Options carry PHB Tables 10 and 11 by race (ADR 0032 step 5): the dice a
// roll draws and the ranges a value warns outside, the half-elf's here.
func TestCreate_Options_RaceIdentity(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	var raw struct {
		RaceIdentity []json.RawMessage `json:"raceIdentity"`
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters/options", "", &raw))
	require.Len(t, raw.RaceIdentity, 6)
	type figure struct {
		Sex   string  `json:"sex"`
		Count int     `json:"count"`
		Sides int     `json:"sides"`
		Min   float64 `json:"min"`
		Max   float64 `json:"max"`
	}
	var halfElf struct {
		Race     string   `json:"race"`
		Height   []figure `json:"height"`
		Weight   []figure `json:"weight"`
		Age      figure   `json:"age"`
		StartAge struct {
			Min float64 `json:"min"`
			Max float64 `json:"max"`
		} `json:"startAge"`
	}
	for _, r := range raw.RaceIdentity {
		require.NoError(t, json.Unmarshal(r, &halfElf))
		if halfElf.Race == "half_elf" {
			break
		}
	}
	require.Equal(t, "half_elf", halfElf.Race)
	// Table 10: 60/58 in + 2d6, 110/85 lb + 3d12; Table 11: 15 + 1d6, max 125 + 3d20.
	assert.Equal(t, []figure{
		{Sex: "male", Count: 2, Sides: 6, Min: 157.4, Max: 182.9},
		{Sex: "female", Count: 2, Sides: 6, Min: 152.4, Max: 177.8},
	}, halfElf.Height)
	assert.Equal(t, []figure{
		{Sex: "male", Count: 3, Sides: 12, Min: 51.2, Max: 66.3},
		{Sex: "female", Count: 3, Sides: 12, Min: 39.9, Max: 54.9},
	}, halfElf.Weight)
	assert.Equal(t, figure{Count: 1, Sides: 6, Min: 16, Max: 185}, halfElf.Age)
	assert.InDelta(t, 16, halfElf.StartAge.Min, 0)
	assert.InDelta(t, 21, halfElf.StartAge.Max, 0)
}
