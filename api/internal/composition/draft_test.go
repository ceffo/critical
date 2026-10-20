package composition_test

// The creation draft end to end (ADR 0034, amendment of 2026-10-20: one draft endpoint): POST
// /api/characters/draft answers what POST /api/characters would store, read as GET
// /api/character reads it, every refusal the creation would give with its field and code, the
// fields still blank and the class minimums missed; it stores nothing and never answers a 400
// for a draft's content.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// draftView is a draft's answer; Derived carries the part a created character's read carries
// too.
type draftView struct {
	Derived       map[string]any   `json:"derived"`
	Warnings      []map[string]any `json:"warnings"`
	Languages     []string         `json:"languages"`
	Missing       []string         `json:"missing"`
	Refusals      []refusalView    `json:"refusals"`
	UnmetMinimums []struct {
		Class     string   `json:"class"`
		Abilities []string `json:"abilities"`
	} `json:"unmetMinimums"`
}

// refusalView is one of a draft's refusals.
type refusalView struct {
	Field string `json:"field"`
	Code  string `json:"code"`
	Spent *int   `json:"spent,omitempty"`
	Limit *int   `json:"limit,omitempty"`
}

func (d draftView) unmet() map[string][]string {
	out := map[string][]string{}
	for _, u := range d.UnmetMinimums {
		out[u.Class] = u.Abilities
	}
	return out
}

// readView is the part of a created character's read the draft answers too.
type readView struct {
	Derived   map[string]any   `json:"derived"`
	Warnings  []map[string]any `json:"warnings"`
	Languages []string         `json:"languages"`
}

// draft posts body as a draft and decodes the answer, which must be a 200.
func (tb *table) draft(t *testing.T, as, body string) draftView {
	t.Helper()
	var d draftView
	require.Equal(t, http.StatusOK, tb.call(as, "POST", "/api/characters/draft", body, &d), body)
	return d
}

// The draft of a complete body is what a read of the character created from it shows: the same
// derived block (level, THAC0, saves, slots, skills, budget) and the same warnings and languages,
// for every creatable class; and a draft stores nothing.
func TestDraft_IsWhatTheCreatedCharacterReads(t *testing.T) {
	bodies := map[string]string{
		// Five additional languages at Intelligence 14 (limit 4), a height
		// outside the half-elf male range; the bard spends its 50 points at level 3.
		"bard": bardStart +
			`,"skillPoints":{"climb_walls":30,"detect_noise":20},"details":{"alignment":"neutral_good","sex":"male",` +
			`"languages":["elvish","gnomish","halfling","goblin","orcish"],"height_cm":250,"weight_kg":70,"age":20}}`,
		"fighter": `{"name":"Brom","class":"fighter","raceCode":"human","str":18,"strExceptional":76,"dex":12,"con":16,"int":9,"wis":10,"cha":8,` +
			`"hpMax":28,"xp":9000,"details":{"alignment":"lawful_good","sex":"male","age":200}}`,
		"thief": `{"name":"Nib","class":"thief","raceCode":"halfling","str":9,"dex":17,"con":12,"int":11,"wis":10,"cha":12,"hpMax":4,"xp":0,` +
			`"skillPoints":{"pick_pockets":30,"open_locks":30},"details":{"alignment":"chaotic_neutral","sex":"female"}}`,
		"mage": `{"name":"Ivo","class":"mage","raceCode":"elf","str":8,"dex":14,"con":10,"int":17,"wis":12,"cha":11,"hpMax":3,"xp":2600,` +
			`"details":{"alignment":"true_neutral","sex":"male"}}`,
		"cleric": `{"name":"Sol","class":"cleric","raceCode":"dwarf","str":12,"dex":9,"con":15,"int":10,"wis":16,"cha":10,"hpMax":20,"xp":13000,` +
			`"details":{"alignment":"lawful_good","sex":"female","weight_kg":60}}`,
	}
	for class, body := range bodies {
		t.Run(class, func(t *testing.T) {
			tb, _ := newTable(t)
			bob, _ := tb.login("bob@example.com")

			d := tb.draft(t, bob, body)
			assert.Empty(t, d.Missing)
			assert.Empty(t, d.Refusals)
			require.NotNil(t, d.Derived)
			var list listView
			require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters", "", &list))
			assert.Empty(t, list.Characters, "a draft stores nothing")

			require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", body, &list), body)
			var read readView
			require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "GET", "/api/character", "", &read))
			assert.Equal(t, read.Derived, d.Derived)
			assert.Equal(t, read.Warnings, d.Warnings)
			assert.Equal(t, read.Languages, d.Languages)
		})
	}
}

// The bard body above warns; the test above would pass on two empty answers, so the warnings are
// named here. Its five languages beyond Common spend five of the seven non-weapon slots (ADR 0037
// step 6): two left to spend, no languages warning of their own.
func TestDraft_CarriesTheWarnings(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	d := tb.draft(t, bob, bardWith(`{"alignment":"neutral_good","sex":"male","languages":["elvish","gnomish","halfling","goblin","orcish"],"height_cm":250,"age":16}`))
	codes := []string{}
	for _, w := range withoutSlotMaps(d.Warnings) {
		codes = append(codes, w["code"].(string))
	}
	assert.ElementsMatch(t, []string{"height_outside_range"}, codes)
	assert.Contains(t, d.Warnings, map[string]any{"code": "slots_to_spend", "field": "nonweapon", "pool": "nonweapon", "left": float64(2)})
	assert.Equal(t, []string{"common", "elvish", "gnomish", "halfling", "goblin", "orcish"}, d.Languages, "the native language added")
	assert.InDelta(t, 3.0, d.Derived["targetLevel"], 0, "2,500 XP is a level 3 bard")
	assert.InDelta(t, 19.0, d.Derived["thac0"], 0)
}

// A blank name, alignment or sex is no refusal: the draft lists them as missing and still answers
// the figures and the warnings, so the proficiency step shows the slots the languages leave
// before the player picks the rest.
func TestDraft_ABlankNameAlignmentOrSexIsMissingNotRefused(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	body := `{"name":"  ","class":"bard","raceCode":"half_elf","str":10,"dex":15,"con":12,"int":9,"wis":9,"cha":16,"hpMax":6,"xp":0,` +
		`"details":{"alignment":"","sex":"","languages":["elvish","gnomish","goblin"]}}`
	d := tb.draft(t, bob, body)
	assert.Equal(t, []string{"name", "alignment", "sex"}, d.Missing)
	assert.Empty(t, d.Refusals)
	assert.Empty(t, withoutSlotMaps(d.Warnings))
	// Intelligence 9: 3 + 0 + 2 = 5 slots, the three languages beyond Common spend 3.
	assert.Contains(t, d.Warnings, map[string]any{"code": "slots_to_spend", "field": "nonweapon", "pool": "nonweapon", "left": float64(2)})

	var refused map[string]string
	require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", body, &refused))
	assert.Equal(t, "name", refused["field"], "the creation still refuses a blank name")
}

// Partial drafts, as the creation's first steps send them, are never a 400: a class alone, a class
// and scores without a race, an XP outside the range or a class nobody can create each answer
// 200, the blank fields missing, the refused ones refused, and no sheet while the class, race,
// scores or XP do not read.
func TestDraft_PartialDraftsAnswer200(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	for body, want := range map[string]struct {
		missing  []string
		refusals []refusalView
	}{
		`{}`:                       {missing: []string{"name", "class", "race", "str", "dex", "con", "int", "wis", "cha", "hpMax", "xp", "alignment", "sex"}},
		`{"class":"thief","xp":0}`: {missing: []string{"name", "race", "str", "dex", "con", "int", "wis", "cha", "hpMax", "alignment", "sex"}},
		`{"class":"bard","str":10,"dex":11,"con":12,"int":13,"wis":9,"cha":15,"xp":0}`: {missing: []string{"name", "race", "hpMax", "alignment", "sex"}},
		`{"class":"thief","xp":-5}`: {
			missing:  []string{"name", "race", "str", "dex", "con", "int", "wis", "cha", "hpMax", "alignment", "sex"},
			refusals: []refusalView{{Field: "xp", Code: "out_of_range"}},
		},
		`{"class":"paladin","xp":0}`: {
			missing:  []string{"name", "race", "str", "dex", "con", "int", "wis", "cha", "hpMax", "alignment", "sex"},
			refusals: []refusalView{{Field: "class", Code: "not_creatable"}},
		},
	} {
		d := tb.draft(t, bob, body)
		assert.Equal(t, want.missing, d.Missing, body)
		if want.refusals == nil {
			want.refusals = []refusalView{}
		}
		assert.Equal(t, want.refusals, d.Refusals, body)
		assert.Nil(t, d.Derived, body)
		assert.Empty(t, d.Warnings, body)
	}
}

// PHB Table 13 on the wire: Dex 11 misses the bard's 12 and only the bard's (the thief asks 9), Dex
// 12 clears it, and abilities with no score yet are never flagged. The options carry the minimums
// the class cards show.
func TestDraft_Table13_ScoresUnderAClassMinimumAreUnmet(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	var opts struct {
		Classes []struct {
			Class    string `json:"class"`
			Minimums []struct {
				Ability string `json:"ability"`
				Min     int    `json:"min"`
			} `json:"minimums"`
		} `json:"classes"`
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters/options", "", &opts))
	for _, c := range opts.Classes {
		if c.Class == "bard" {
			require.Len(t, c.Minimums, 3)
			assert.Equal(t, "dex", c.Minimums[0].Ability)
			assert.Equal(t, 12, c.Minimums[0].Min)
		}
	}

	a := tb.draft(t, bob, `{"class":"bard","dex":11,"int":13,"cha":15}`)
	assert.Equal(t, []string{"dex"}, a.unmet()["bard"])
	for _, c := range []string{"fighter", "mage", "cleric", "thief"} {
		assert.Empty(t, a.unmet()[c], c)
	}
	assert.Empty(t, tb.draft(t, bob, `{"class":"bard","dex":12,"int":13,"cha":15}`).unmet()["bard"])
	assert.Empty(t, tb.draft(t, bob, `{"class":"bard","str":3}`).unmet()["bard"])
}

// ADR 0030 steps 17 and 18 on the extras step: the derived block's skill point budget at the
// level the XP gives, with what is left (negative when over, the creation's refusal listed too).
// A thief has 60 points at level 1 and 30 more a level, 30 in one skill and 15 more a level; a
// bard 20 and 15 a level, with no limit in one skill; a fighter none.
func TestDraft_Steps17And18_TheBudgetAndWhatIsLeft(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	thief := `{"class":"thief","raceCode":"human","str":9,"dex":17,"con":12,"int":11,"wis":10,"cha":12,`
	bard := `{"class":"bard","raceCode":"human","str":10,"dex":15,"con":12,"int":14,"wis":9,"cha":16,`
	type budget struct {
		Level, Available, Spent, Left, PerSkillCap int
	}
	for _, tc := range []struct {
		name, body string
		want       budget
		refusals   []refusalView
	}{
		{"thief level 1", thief + `"xp":0,"skillPoints":{"pick_pockets":20,"open_locks":15}}`, budget{1, 60, 35, 25, 30}, nil},
		{"thief level 2", thief + `"xp":1250}`, budget{2, 90, 0, 90, 45}, nil},
		{
			"thief level 3 over budget", thief + `"xp":2500,"skillPoints":{"pick_pockets":60,"open_locks":60,"climb_walls":10}}`,
			budget{3, 120, 130, -10, 60},
			[]refusalView{{Field: "skillPoints", Code: "over_budget", Spent: new(130), Limit: new(120)}},
		},
		{"bard level 1", bard + `"xp":0,"skillPoints":{"climb_walls":5}}`, budget{1, 20, 5, 15, 0}, nil},
		{"bard level 2", bard + `"xp":1250}`, budget{2, 35, 0, 35, 0}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := tb.draft(t, bob, tc.body)
			require.NotNil(t, d.Derived)
			raw, err := json.Marshal(d.Derived["skillPoints"])
			require.NoError(t, err)
			var got budget
			require.NoError(t, json.Unmarshal(raw, &got))
			assert.Equal(t, tc.want, got)
			if tc.refusals == nil {
				tc.refusals = []refusalView{}
			}
			assert.Equal(t, tc.refusals, d.Refusals)
		})
	}
	d := tb.draft(t, bob, `{"class":"fighter","raceCode":"human","str":12,"dex":12,"con":12,"int":9,"wis":10,"cha":8,"xp":0}`)
	require.NotNil(t, d.Derived)
	assert.Nil(t, d.Derived["skillPoints"], "a fighter has no skill points")
}

// creationRefusal is what POST /api/characters answers a refused body with.
type creationRefusal struct {
	Error string `json:"error"`
	Code  string `json:"code"`
	Field string `json:"field"`
	Spent *int   `json:"spent"`
	Limit *int   `json:"limit"`
}

// Every refusal POST /api/characters gives appears in the draft's refusals with the same field and
// code (the creation's `error` for an invalid input, over_budget with the same figures), one row
// per refusal of the creation: the name, class, race, scores, 18/xx, skill points, HP, XP,
// details, languages, picks and the four budgets. A required field left blank is no refusal in a
// draft: it is listed as missing.
func TestDraft_RefusesAsTheCreationRefuses(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	bard := func(old, repl string) string { return strings.Replace(newBard, old, repl, 1) }
	details := func(extra string) string { return bardWith(`{"alignment":"neutral_good","sex":"male",` + extra + `}`) }
	picks := func(p string) string { return bard(`"xp":2500`, `"xp":2500,"proficiencies":`+p) }
	fighter := `{"name":"Brom","class":"fighter","raceCode":"human","str":18,"dex":12,"con":16,"int":9,"wis":10,"cha":8,"hpMax":10,"xp":0,` +
		`"details":{"alignment":"lawful_good","sex":"male"},`
	thief := `{"name":"Nib","class":"thief","raceCode":"halfling","str":9,"dex":17,"con":12,"int":11,"wis":10,"cha":12,"hpMax":4,"xp":0,` +
		`"details":{"alignment":"chaotic_neutral","sex":"female"},`
	mage := `{"name":"Ivo","class":"mage","raceCode":"elf","str":8,"dex":14,"con":10,"int":17,"wis":12,"cha":11,"hpMax":3,"xp":2600,` +
		`"details":{"alignment":"true_neutral","sex":"male"},`
	for _, tc := range []struct {
		name, body, field, code string
	}{
		{"a name too long", bard(`"name":"Lyra"`, `"name":"`+strings.Repeat("e", 41)+`"`), "name", "too_long"},
		{"a class nobody creates", bard(`"class":"bard"`, `"class":"paladin"`), "class", "not_creatable"},
		{"an unknown race", bard(`"raceCode":"half_elf"`, `"raceCode":"orc"`), "race", "unknown"},
		{"a race the class refuses", bard(`"raceCode":"half_elf"`, `"raceCode":"dwarf"`), "race", "race_not_allowed"},
		{"a score under 3", bard(`"str":10`, `"str":2`), "str", "out_of_range"},
		{"a score over 19", bard(`"cha":16`, `"cha":20`), "cha", "out_of_range"},
		{"an 18/xx for a bard", bard(`"str":10`, `"str":18,"strExceptional":50`), "strExceptional", "not_allowed"},
		{"an 18/xx over 100", fighter + `"strExceptional":101}`, "strExceptional", "out_of_range"},
		{"skill points for a fighter", fighter + `"skillPoints":{"climb_walls":5}}`, "skillPoints", "not_allowed"},
		{"a skill the class has not", thief + `"skillPoints":{"fly":5}}`, "skillPoints.fly", "unknown"},
		{"skill points over the most one skill holds", thief + `"skillPoints":{"open_locks":200}}`, "skillPoints.open_locks", "out_of_range"},
		{"max HP 0", bard(`"hpMax":6`, `"hpMax":0`), "hpMax", "out_of_range"},
		{"a negative XP", bard(`"xp":2500`, `"xp":-1`), "xp", "out_of_range"},
		{"an alignment that is no code", bardWith(`{"alignment":"good","sex":"male"}`), "details.alignment", "unknown"},
		{"an alignment the class refuses", bardWith(`{"alignment":"lawful_good","sex":"male"}`), "details.alignment", "alignment_not_allowed"},
		{"a sex that is no code", bardWith(`{"alignment":"neutral_good","sex":"other"}`), "details.sex", "unknown"},
		{"an age of 0", details(`"age":0`), "details.age", "out_of_range"},
		{"a height out of range", details(`"height_cm":400`), "details.height", "out_of_range"},
		{"a height that is no number", details(`"height_cm":"tall"`), "details.height", "not_a_number"},
		{"hair too long", details(`"hair":"` + strings.Repeat("e", 41) + `"`), "details.hair", "too_long"},
		{"a language twice", details(`"languages":["elvish","elvish"]`), "details.languages", "duplicate"},
		{"a language that is no code", details(`"languages":["Elfique"]`), "details.languages", "unknown"},
		{"an empty language", details(`"languages":[""]`), "details.languages", "empty"},
		{"an unknown proficiency", picks(`{"nonweapon":[{"id":"old_skill","slots":1}]}`), "nonweapon.old_skill", "unknown"},
		{"a proficiency twice", picks(`{"nonweapon":[{"id":"tumbling","slots":1},{"id":"tumbling","slots":1}]}`), "nonweapon.tumbling", "duplicate"},
		{"a weapon at 5 slots", picks(`{"weapon":[{"type":"dagger","slots":5}]}`), "weapon.dagger", "out_of_range"},
		{"a weapon the class may not pick", mage + `"proficiencies":{"weapon":[{"type":"long_sword","slots":1}]}}`, "weapon.long_sword", "weapon_not_allowed"},
		{"skill points over the budget", bard(`"xp":2500`, `"xp":2500,"skillPoints":{"climb_walls":30,"detect_noise":21}`), "skillPoints", "over_budget"},
		{"one skill over its cap", thief + `"skillPoints":{"open_locks":31}}`, "skillPoints.open_locks", "over_budget"},
		{"non-weapon picks over the slots", picks(`{"nonweapon":[{"id":"tumbling","slots":6},{"id":"tracking","slots":2}]}`), "nonweapon", "over_budget"},
		{"weapon picks over the slots", picks(`{"weapon":[{"type":"dagger","slots":1},{"type":"long_sword","slots":1},{"type":"short_bow","slots":1}]}`), "weapon", "over_budget"},
		{"languages over the slots", strings.Replace(
			details(`"languages":["elvish","gnomish","halfling","goblin","orcish"]`), `"xp":2500`,
			`"xp":2500,"proficiencies":{"nonweapon":[{"id":"tumbling","slots":3}]}`, 1), "details.languages", "over_budget"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var refused creationRefusal
			status := tb.call(bob, "POST", "/api/characters", tc.body, &refused)
			require.Contains(t, []int{http.StatusBadRequest, http.StatusConflict}, status, tc.body)
			code := refused.Error
			if refused.Code == "over_budget" {
				code = refused.Code
			}
			require.Equal(t, tc.field, refused.Field, "the creation's field")
			require.Equal(t, tc.code, code, "the creation's code")

			d := tb.draft(t, bob, tc.body)
			assert.Contains(t, d.Refusals, refusalView{Field: refused.Field, Code: code, Spent: refused.Spent, Limit: refused.Limit})
		})
	}
	var list listView
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters", "", &list))
	assert.Empty(t, list.Characters, "nothing was created")

	for _, tc := range []struct{ body, field string }{
		{bard(`"name":"Lyra"`, `"name":" "`), "name"},
		{bard(`"raceCode":"half_elf"`, `"raceCode":""`), "race"},
		{bard(`"str":10,`, ``), "str"},
		{bard(`"hpMax":6,`, ``), "hpMax"},
		{bardWith(`{"sex":"male"}`), "alignment"},
		{bardWith(`{"alignment":"neutral_good"}`), "sex"},
	} {
		var refused creationRefusal
		require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", tc.body, &refused), tc.body)
		assert.Equal(t, "empty", refused.Error, tc.body)
		d := tb.draft(t, bob, tc.body)
		assert.Contains(t, d.Missing, tc.field, tc.body)
		assert.Empty(t, d.Refusals, tc.body)
	}
	// An unknown field is no draft's content: still a bad request.
	assert.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters/draft", bardStart+`,"level":3}`, nil))
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
