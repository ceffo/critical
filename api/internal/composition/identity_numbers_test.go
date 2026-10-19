package composition_test

// The identity numbers end to end (ADR 0032 step 5): a player with no
// character yet rolls a half-elf woman's height, weight and age over the
// router with the real dice, creates the character with them (no warning),
// then types values outside the table (accepted, with a warning carrying the
// value and the range), and a value that is no number is refused.

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type identityRollView struct {
	RaceCode string `json:"raceCode"`
	Sex      string `json:"sex"`
	Rolls    []struct {
		Figure string  `json:"figure"`
		Dice   []int   `json:"dice"`
		Sides  int     `json:"sides"`
		Base   int     `json:"base"`
		Total  int     `json:"total"`
		Unit   string  `json:"unit"`
		Value  float64 `json:"value"`
	} `json:"rolls"`
}

type numbersView struct {
	Height   *float64 `json:"height"`
	Weight   *float64 `json:"weight"`
	Age      *int     `json:"age"`
	Warnings []struct {
		Code  string   `json:"code"`
		Field string   `json:"field"`
		Value *float64 `json:"value"`
		Min   *float64 `json:"min"`
		Max   *float64 `json:"max"`
	} `json:"warnings"`
}

func numbersBody(height, weight, age string) string {
	return `{"name":"Numbers","class":"bard","raceCode":"half_elf","str":10,"dex":10,"con":10,"int":12,"wis":10,"cha":10,` +
		`"hpMax":10,"xp":2500,"skillArmor":"leather","details":{"alignment":"neutral_good","sex":"female",` +
		`"height":` + height + `,"weight":` + weight + `,"age":` + age + `}}`
}

func TestIdentityRoll_Step5_RollCreateWarnTypeAndRefuse(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")

	var roll identityRollView
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/rolls/identity",
		`{"raceCode":"half_elf","sex":"female","figures":["height","weight","age"]}`, &roll))
	require.Len(t, roll.Rolls, 3)
	h, w, a := roll.Rolls[0], roll.Rolls[1], roll.Rolls[2]
	assert.Equal(t, "height", h.Figure)
	assert.Equal(t, 58, h.Base, "a half-elf woman's base height")
	assert.Len(t, h.Dice, 2)
	assert.Equal(t, 6, h.Sides)
	assert.Equal(t, "in", h.Unit)
	assert.Equal(t, 3, len(w.Dice))
	assert.Equal(t, 85, w.Base)
	assert.Equal(t, "lb", w.Unit)
	assert.Equal(t, 15, a.Base)
	assert.Len(t, a.Dice, 1)
	assert.Equal(t, "year", a.Unit)
	assert.Greater(t, h.Value, 100.0, "centimetres")

	// The rolled numbers are accepted by the creation and warn nothing.
	var list listView
	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", numbersBody(
		fmtF(h.Value), fmtF(w.Value), fmtI(int(a.Value))), &list))
	var got numbersView
	require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "GET", "/api/character", "", &got))
	assert.InDelta(t, h.Value, *got.Height, 1e-9)
	assert.InDelta(t, w.Value, *got.Weight, 1e-9)
	assert.Equal(t, int(a.Value), *got.Age)
	assert.Empty(t, got.Warnings, "a rolled value is inside its range")

	// Typed outside the table: accepted, with the value and the range.
	require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "PATCH", "/api/character/details",
		`{"height":210,"weight":45.5,"age":400}`, nil))
	require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "GET", "/api/character", "", &got))
	require.Len(t, got.Warnings, 2)
	assert.Equal(t, "height_outside_range", got.Warnings[0].Code)
	assert.Equal(t, 210.0, *got.Warnings[0].Value)
	assert.Equal(t, 152.4, *got.Warnings[0].Min)
	assert.Equal(t, 177.8, *got.Warnings[0].Max)
	assert.Equal(t, "age_outside_range", got.Warnings[1].Code)

	// Null clears; a string is no number; a figure beyond the sanity bound is refused.
	require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "PATCH", "/api/character/details", `{"height":null}`, nil))
	require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "GET", "/api/character", "", &got))
	assert.Nil(t, got.Height)
	for _, c := range []struct{ body, field, code string }{
		{`{"height":"tall"}`, "height", "not_a_number"},
		{`{"weight":true}`, "weight", "not_a_number"},
		{`{"height":0}`, "height", "out_of_range"},
		{`{"weight":501}`, "weight", "out_of_range"},
	} {
		var refused map[string]string
		require.Equal(t, http.StatusBadRequest, tb.showing(bob, *list.CurrentID, "PATCH", "/api/character/details", c.body, &refused), c.body)
		assert.Equal(t, "invalid_input", refused["code"], c.body)
		assert.Equal(t, c.field, refused["field"], c.body)
		assert.Equal(t, c.code, refused["error"], c.body)
	}
	var refused map[string]string
	require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", numbersBody(`"tall"`, `70`, `30`), &refused))
	assert.Equal(t, "details.height", refused["field"], "the creation names the field under details")
	assert.Equal(t, "not_a_number", refused["error"])
}

func TestIdentityRoll_Step5_3_RefusalsAreFieldErrors(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	for _, c := range []struct{ body, field, code string }{
		{`{"sex":"male","figures":["age"]}`, "age", "race_required"},
		{`{"raceCode":"elf","figures":["height"]}`, "height", "sex_required"},
		{`{"raceCode":"orc","sex":"male","figures":["age"]}`, "race", "unknown"},
		{`{"raceCode":"elf","sex":"male","figures":[]}`, "figures", "empty"},
		{`{"raceCode":"elf","sex":"male","figures":["hair"]}`, "figures", "unknown"},
	} {
		var refused map[string]string
		require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters/rolls/identity", c.body, &refused), c.body)
		assert.Equal(t, "invalid_input", refused["code"], c.body)
		assert.Equal(t, c.field, refused["field"], c.body)
		assert.Equal(t, c.code, refused["error"], c.body)
	}
	var refused map[string]string
	require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters/rolls/identity",
		`{"raceCode":"elf","sex":"male","figures":["age"],"level":3}`, &refused))
	assert.Equal(t, "bad_request", refused["code"])
}

func fmtF(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
func fmtI(v int) string     { return strconv.Itoa(v) }
