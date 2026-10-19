package composition_test

// The creation's rolls end to end (ADR 0032 steps 6 and 7): a player with no
// character yet rolls the scores and the max HP over the router, the real
// services and the fixture's rule tables, then creates the character with
// the rolled values, or with values they edited: a rolled value is validated
// as a typed one.

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scoresView struct {
	Method struct {
		Code string `json:"code"`
	} `json:"method"`
	Rolls []struct {
		Dice    []int  `json:"dice"`
		Dropped *int   `json:"dropped"`
		Total   int    `json:"total"`
		Ability string `json:"ability"`
	} `json:"rolls"`
	Adjustments map[string]int `json:"adjustments"`
	Scores      map[string]int `json:"scores"`
}

type hpRollView struct {
	Level  int `json:"level"`
	Levels []struct {
		Level   int  `json:"level"`
		HitDie  int  `json:"hitDie"`
		Rolled  *int `json:"rolled"`
		Gain    int  `json:"gain"`
		Fixed   bool `json:"fixed"`
		Counted *int `json:"counted"`
	} `json:"levels"`
	Total int `json:"total"`
}

func bardBody(s map[string]int, hpMax int) string {
	return fmt.Sprintf(`{"name":"Rolled","class":"bard","raceCode":"half_elf",`+
		`"str":%d,"dex":%d,"con":%d,"int":%d,"wis":%d,"cha":%d,"hpMax":%d,"xp":2500,"skillArmor":"leather",`+
		`"details":{"alignment":"neutral_good","sex":"female"}}`,
		s["str"], s["dex"], s["con"], s["int"], s["wis"], s["cha"], hpMax)
}

// Steps 6 and 7 composed with the creation: method I's scores for a
// half-elf (Table 8: no adjustment) and the HP of a 2,500 XP bard (level 3:
// three d6) are accepted by the creation as rolled, and again after the
// player edited them.
func TestCreationRolls_RollThenCreateWithTheRolledOrEditedValues(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")

	var scores scoresView
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/rolls/scores",
		`{"method":"I","raceCode":"half_elf"}`, &scores))
	require.Len(t, scores.Rolls, 6)
	for _, r := range scores.Rolls {
		assert.Len(t, r.Dice, 3)
		assert.Equal(t, r.Dice[0]+r.Dice[1]+r.Dice[2], r.Total)
		assert.Equal(t, r.Total, scores.Scores[r.Ability], "half-elves adjust nothing")
	}
	assert.Equal(t, map[string]int{"str": 0, "dex": 0, "con": 0, "int": 0, "wis": 0, "cha": 0}, scores.Adjustments)

	var hp hpRollView
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/rolls/hp",
		fmt.Sprintf(`{"class":"bard","con":%d,"xp":2500}`, scores.Scores["con"]), &hp))
	assert.Equal(t, 3, hp.Level, "PHB Table 25: 2,500 XP is level 3")
	require.Len(t, hp.Levels, 3)
	sum := 0
	for i, l := range hp.Levels {
		assert.Equal(t, i+1, l.Level)
		assert.Equal(t, 6, l.HitDie, "a rogue's d6")
		require.NotNil(t, l.Rolled)
		sum += l.Gain
	}
	assert.Equal(t, sum, hp.Total)

	var list listView
	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", bardBody(scores.Scores, hp.Total), &list),
		"the rolled values are accepted")
	var got createdSheet
	require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "GET", "/api/character", "", &got))
	assert.Equal(t, hp.Total, got.HPMax)

	edited := map[string]int{}
	for k, v := range scores.Scores {
		edited[k] = min(v+1, 19)
	}
	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", bardBody(edited, hp.Total+1), &list),
		"edited values are accepted too")
	require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "GET", "/api/character", "", &got))
	assert.Equal(t, hp.Total+1, got.HPMax)
}

// Every method answers over HTTP with its number of totals, and method V
// names its dropped die.
func TestCreationRolls_EveryMethodAnswers(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	for method, n := range map[string]int{"I": 6, "II": 12, "III": 6, "IV": 12, "V": 6, "VI": 7} {
		var s scoresView
		require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/rolls/scores", `{"method":"`+method+`"}`, &s), method)
		assert.Equal(t, method, s.Method.Code)
		assert.Len(t, s.Rolls, n, method)
		assert.Nil(t, s.Adjustments, "no race, no adjustments")
		if method == "V" {
			for _, r := range s.Rolls {
				require.NotNil(t, r.Dropped)
				assert.Len(t, r.Dice, 4)
			}
		}
	}
}

// The refusals are 400 invalid_input with the field and the code; a body
// missing the HP inputs or carrying an unknown field is a bad request.
func TestCreationRolls_RefusalsAreFieldErrors(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	for _, c := range []struct{ path, body, field, code string }{
		{"/api/characters/rolls/scores", `{"method":"VII"}`, "method", "unknown"},
		{"/api/characters/rolls/scores", `{"method":"I","raceCode":"orc"}`, "race", "unknown"},
		{"/api/characters/rolls/hp", `{"class":"paladin","con":10,"xp":0}`, "class", "unknown"},
		{"/api/characters/rolls/hp", `{"class":"bard","con":20,"xp":0}`, "con", "out_of_range"},
		{"/api/characters/rolls/hp", `{"class":"bard","con":10,"xp":-1}`, "xp", "out_of_range"},
	} {
		var refused map[string]string
		require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", c.path, c.body, &refused), c.body)
		assert.Equal(t, "invalid_input", refused["code"], c.body)
		assert.Equal(t, c.field, refused["field"], c.body)
		assert.Equal(t, c.code, refused["error"], c.body)
	}
	for _, body := range []string{`{"class":"bard","con":10}`, `{"class":"bard","con":10,"xp":0,"level":3}`} {
		var refused map[string]string
		require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters/rolls/hp", body, &refused), body)
		assert.Equal(t, "bad_request", refused["code"], body)
	}
}

type arrangeView struct {
	Method      string         `json:"method"`
	Adjustments map[string]int `json:"adjustments"`
	Rolled      map[string]int `json:"rolled"`
	Scores      map[string]int `json:"scores"`
}

// Step 6's arrangement over HTTP: method VI's dice placed for a half-elf
// give the base 8 plus the dice, the creation accepts those scores as typed;
// a dwarf's Charisma 3 is 2 (Table 8), which the creation refuses on "cha";
// a placement above method VI's 18 is a field error and nothing is created.
func TestCreationRolls_ArrangeThenCreate(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")

	var a arrangeView
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/rolls/scores/arrangement",
		`{"method":"VI","raceCode":"half_elf","placed":{"dex":[6,3],"cha":[5,4],"int":[2]}}`, &a))
	assert.Equal(t, "VI", a.Method)
	assert.Equal(t, map[string]int{"str": 8, "dex": 17, "con": 8, "int": 10, "wis": 8, "cha": 17}, a.Scores)
	var list listView
	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", bardBody(a.Scores, 6), &list))

	var low arrangeView
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/rolls/scores/arrangement",
		`{"method":"III","raceCode":"dwarf","placed":{"cha":[3]}}`, &low))
	assert.Equal(t, map[string]int{"cha": 2}, low.Scores)
	assert.Equal(t, map[string]int{"cha": 3}, low.Rolled)
	two := map[string]int{}
	for k, v := range a.Scores {
		two[k] = v
	}
	two["cha"] = low.Scores["cha"]
	var tooLow map[string]string
	require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", bardBody(two, 6), &tooLow))
	assert.Equal(t, "cha", tooLow["field"])
	assert.Equal(t, "out_of_range", tooLow["error"])

	var refused map[string]string
	require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters/rolls/scores/arrangement",
		`{"method":"VI","placed":{"str":[6,5]}}`, &refused))
	assert.Equal(t, "invalid_input", refused["code"])
	assert.Equal(t, "placed.str", refused["field"])
	assert.Equal(t, "above_max", refused["error"])
}
