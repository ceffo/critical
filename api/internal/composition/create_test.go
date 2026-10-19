package composition_test

// Creating, switching and retiring characters end to end over the in-memory
// storage (ADR 0029 steps 7.2 and 7.3, ADR 0030 steps 1, 21 and 24): the
// router, the real services and the fixture's rule tables.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

const bardStart = `{"name":"Lyra","class":"bard","raceCode":"half_elf",` +
	`"str":10,"dex":15,"con":12,"int":14,"wis":9,"cha":16,"hpMax":6,"xp":2500,"skillArmor":"leather"`

// bardWith is the body creating a bard with the given details object.
func bardWith(details string) string { return bardStart + `,"details":` + details + `}` }

var newBard = bardWith(`{"alignment":"neutral_good","sex":"male"}`)

type createdSheet struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Class    string `json:"class"`
	RaceCode string `json:"raceCode"`
	HPMax    int    `json:"hpMax"`
	Derived  struct {
		TargetLevel      int   `json:"targetLevel"`
		RealizedLevel    int   `json:"realizedLevel"`
		THAC0            int   `json:"thac0"`
		SheetDifferences []any `json:"sheetDifferences"`
	} `json:"derived"`
}

// A player with no character creates a bard: it is theirs and current at
// once, its level and THAC0 come from the rules at the XP entered (2,500 XP
// is a level 3 bard, THAC0 19), it shows no sheet difference (its sheet
// figures are the derived ones), and it owns nothing: no weapon, item,
// proficiency, bard skill or spellbook entry. Another user never sees it.
func TestCreate_ANewBardIsTheOwnersOnlyAndStartsWithNothing(t *testing.T) {
	m := memory.New()
	tb, bs := serverOn(t, m)
	ann, _ := tb.login("")
	bob, _ := tb.login("bob@example.com")

	var list listView
	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", newBard, &list))
	require.Len(t, list.Characters, 1)
	lyra := list.Characters[0].ID
	require.NotNil(t, list.CurrentID)
	assert.Equal(t, lyra, *list.CurrentID, "the new character is current")

	var got createdSheet
	require.Equal(t, http.StatusOK, tb.showing(bob, lyra, "GET", "/api/character", "", &got))
	assert.Equal(t, "Lyra", got.Name)
	assert.Equal(t, "bard", got.Class)
	assert.Equal(t, "half_elf", got.RaceCode)
	assert.Equal(t, 6, got.HPMax)
	assert.Equal(t, 3, got.Derived.TargetLevel, "PHB Table 25: 2,500 XP is level 3")
	assert.Equal(t, 3, got.Derived.RealizedLevel, "created realized at its starting level (ADR 0030 step 7.5)")
	assert.Equal(t, 19, got.Derived.THAC0, "PHB Table 53: a level 3 rogue")
	assert.Empty(t, got.Derived.SheetDifferences, "a created character states no sheet")

	var hp hpView
	require.Equal(t, http.StatusOK, tb.showing(bob, lyra, "GET", "/api/hp", "", &hp))
	assert.Equal(t, 6, hp.CurrentHp, "full HP, an empty ledger")

	assert.Equal(t, sheet{Skills: []string{}, Mine: []string{}}, emptied(sheetOf(t, bs, lyra)),
		"no weapons, items, proficiencies, skills or spellbook")

	var annList listView
	require.Equal(t, http.StatusOK, tb.call(ann, "GET", "/api/characters", "", &annList))
	for _, c := range annList.Characters {
		assert.NotEqual(t, lyra, c.ID, "Bob's character is not in Ann's list")
	}
	assert.Equal(t, http.StatusNotFound, tb.call(ann, "PUT", "/api/characters/current", `{"id":`+itoa(lyra)+`}`, nil))
	assert.Equal(t, http.StatusNotFound, tb.call(ann, "PATCH", "/api/characters/"+itoa(lyra), `{"retired":true}`, nil))
}

// The details entered with a new character are stored trimmed, as the
// card's edit stores them, and GET /api/character shows them; the ones left
// out are none.
func TestCreate_TheDetailsEnteredAreStored(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	body := bardWith(`{"alignment":" chaotic_neutral ","sex":"female","age":23,"eyes":"verts","languages":["elvish","gnoll"]}`)
	var list listView
	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", body, &list))
	var got struct {
		Alignment string   `json:"alignment"`
		Sex       string   `json:"sex"`
		Missing   []string `json:"missing"`
		Age       *int     `json:"age"`
		Eyes      string   `json:"eyes"`
		Hair      string   `json:"hair"`
		Languages []string `json:"languages"`
	}
	require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "GET", "/api/character", "", &got))
	assert.Equal(t, "chaotic_neutral", got.Alignment)
	assert.Equal(t, "female", got.Sex)
	assert.Empty(t, got.Missing)
	require.NotNil(t, got.Age)
	assert.Equal(t, 23, *got.Age)
	assert.Equal(t, "verts", got.Eyes)
	assert.Empty(t, got.Hair)
	assert.Equal(t, []string{"common", "elvish", "gnoll"}, got.Languages, "Common, a half-elf's native language, first (ADR 0032 step 4.1)")
}

// ADR 0032 steps 2 and 3, end to end: creating a character without an alignment or a sex, with
// one that is not a code, or with an alignment the class forbids is refused with the field and the
// code, and nothing is stored (no character, none made current).
func TestCreate_Step2And3_AlignmentAndSexAreRequiredCodesAndTheClassLimitRefuses(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	for body, want := range map[string][2]string{
		bardWith(`{"sex":"male"}`):                             {"details.alignment", "empty"},
		bardWith(`{"alignment":"neutral_good"}`):               {"details.sex", "empty"},
		bardWith(`{"alignment":"good","sex":"male"}`):          {"details.alignment", "unknown"},
		bardWith(`{"alignment":"neutral_good","sex":"other"}`): {"details.sex", "unknown"},
		bardWith(`{"alignment":"lawful_good","sex":"male"}`):   {"details.alignment", "alignment_not_allowed"},
		bardWith(`{"alignment":"Neutral Good","sex":"male"}`):  {"details.alignment", "unknown"},
		bardStart + `}`: {"details.alignment", "empty"},
		bardWith(`{"alignment":"neutral_good","sex":"male","race":"elf"}`): {"", "bad_request"},
	} {
		var refused map[string]string
		require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", body, &refused), body)
		assert.Equal(t, want[0], refused["field"], body)
		if want[0] != "" {
			assert.Equal(t, want[1], refused["error"], body)
		} else {
			assert.Equal(t, want[1], refused["code"], body, "the race detail is an unknown field")
		}
	}
	var list listView
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters", "", &list))
	assert.Empty(t, list.Characters, "a refused creation stores nothing")
	assert.Nil(t, list.CurrentID)

	// A bard accepts lawful neutral.
	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters",
		bardWith(`{"alignment":"lawful_neutral","sex":"female"}`), &list))
}

// emptied maps the nil lists of an empty sheet to their zero value.
func emptied(s sheet) sheet {
	if len(s.Items) == 0 {
		s.Items = nil
	}
	if len(s.Weapons) == 0 {
		s.Weapons = nil
	}
	if len(s.Spellbook) == 0 {
		s.Spellbook = nil
	}
	return s
}

// Invalid input is 400 with the field, and nothing is stored: a race the
// class does not admit (a dwarf bard, DMG Table 7), an ability outside 3 to
// 19, a class the app does not have, details the card would refuse, and an
// entered level or THAC0 (never entered, step 1).
func TestCreate_RefusedInputStoresNothing(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	for body, field := range map[string]string{
		`{"name":"D","class":"bard","raceCode":"dwarf","str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10,"hpMax":6,"xp":0}`:    "race",
		`{"name":"D","class":"bard","raceCode":"human","str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":20,"hpMax":6,"xp":0}`:    "cha",
		`{"name":"D","class":"paladin","raceCode":"human","str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10,"hpMax":6,"xp":0}`: "class",
	} {
		var refused map[string]string
		require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", body, &refused), body)
		assert.Equal(t, "invalid_input", refused["code"])
		assert.Equal(t, field, refused["field"], body)
	}
	// The details are checked as the card's edit checks them, their field
	// named under "details." (the race code is "race").
	for body, field := range map[string]string{
		bardWith(`{"alignment":"neutral_good","sex":"male","age":0}`):                                  "details.age",
		bardWith(`{"alignment":"neutral_good","sex":"male","languages":["common","common"]}`):          "details.languages",
		bardWith(`{"alignment":"neutral_good","sex":"male","hair":"` + strings.Repeat("e", 41) + `"}`): "details.hair",
	} {
		var refused map[string]string
		require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", body, &refused), body)
		assert.Equal(t, field, refused["field"], body)
	}
	for _, extra := range []string{`"level":3`, `"thac0":19`} {
		body := bardStart + `,` + extra + `}`
		assert.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", body, nil), extra)
	}
	var list listView
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters", "", &list))
	assert.Empty(t, list.Characters)
	assert.Nil(t, list.CurrentID)
}

// A second bard, a switch to the first and back; retiring the current one
// makes the first one left current, retiring the last leaves none (the page
// then offers to create one); a retired character is never listed again.
func TestCreate_SwitchAndRetire(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	var list listView
	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", newBard, &list))
	first := *list.CurrentID
	second := `{"name":"Mira","class":"bard","raceCode":"human",` +
		`"str":9,"dex":16,"con":10,"int":13,"wis":11,"cha":17,"hpMax":4,"xp":0,"skillArmor":"none",` +
		`"details":{"alignment":"true_neutral","sex":"female"}}`
	require.Equal(t, http.StatusCreated, tb.showing(bob, first, "POST", "/api/characters", second, &list))
	require.Len(t, list.Characters, 2)
	mira := *list.CurrentID
	assert.NotEqual(t, first, mira)

	require.Equal(t, http.StatusOK, tb.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(first)+`}`, &list))
	assert.Equal(t, first, *list.CurrentID)
	require.Equal(t, http.StatusOK, tb.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(mira)+`}`, &list))
	assert.Equal(t, mira, *list.CurrentID)

	require.Equal(t, http.StatusOK, tb.call(bob, "PATCH", "/api/characters/"+itoa(first), `{"name":"Lyra the Bold"}`, &list))
	assert.Equal(t, "Lyra the Bold", list.Characters[0].Name)
	assert.Equal(t, mira, *list.CurrentID, "a rename switches nothing")

	require.Equal(t, http.StatusOK, tb.call(bob, "PATCH", "/api/characters/"+itoa(mira), `{"retired":true}`, &list))
	require.Len(t, list.Characters, 1)
	assert.Equal(t, first, *list.CurrentID, "the first one left is current")
	var shown sheetView
	require.Equal(t, http.StatusOK, tb.showing(bob, first, "GET", "/api/character", "", &shown))
	assert.Equal(t, first, shown.ID)
	assert.Equal(t, http.StatusNotFound, tb.call(bob, "PUT", "/api/characters/current", `{"id":`+itoa(mira)+`}`, nil))
	assert.Equal(t, http.StatusNotFound, tb.call(bob, "PATCH", "/api/characters/"+itoa(mira), `{"name":"Back"}`, nil))

	require.Equal(t, http.StatusOK, tb.call(bob, "PATCH", "/api/characters/"+itoa(first), `{"retired":true}`, &list))
	assert.Empty(t, list.Characters)
	assert.Nil(t, list.CurrentID)
	var refused map[string]string
	assert.Equal(t, http.StatusNotFound, tb.call(bob, "GET", "/api/character", "", &refused))
	assert.Equal(t, "no_character", refused["code"])
}

// The form's options: the bard alone for now, as a human or a half-elf.
func TestCreate_Options(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	var got struct {
		Classes []struct {
			Class               string   `json:"class"`
			Races               []string `json:"races"`
			ExceptionalStrength []string `json:"exceptionalStrengthRaces"`
			Skills              []string `json:"skills"`
			SkillArmors         []string `json:"skillArmors"`
		} `json:"classes"`
		Ability        struct{ Min, Max int } `json:"ability"`
		StrExceptional struct{ Min, Max int } `json:"strExceptional"`
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters/options", "", &got))
	require.Len(t, got.Classes, 5)
	assert.Equal(t, "bard", got.Classes[0].Class)
	assert.Equal(t, []string{"human", "half_elf"}, got.Classes[0].Races)
	assert.Empty(t, got.Classes[0].ExceptionalStrength)
	assert.Equal(t, "fighter", got.Classes[1].Class)
	assert.Equal(t, []string{"human", "dwarf", "elf", "gnome", "half_elf", "halfling"}, got.Classes[1].Races)
	assert.Equal(t, []string{"human", "dwarf", "elf", "gnome", "half_elf"}, got.Classes[1].ExceptionalStrength, "no halfling (PHB Table 7)")
	assert.Equal(t, "mage", got.Classes[2].Class)
	assert.Equal(t, []string{"human", "elf", "half_elf"}, got.Classes[2].Races, "DMG Table 7")
	assert.Empty(t, got.Classes[2].ExceptionalStrength)
	assert.Equal(t, "cleric", got.Classes[3].Class)
	assert.Equal(t, []string{"human", "dwarf", "elf", "gnome", "half_elf", "halfling"}, got.Classes[3].Races, "DMG Table 7")
	assert.Empty(t, got.Classes[3].ExceptionalStrength)
	assert.Equal(t, "thief", got.Classes[4].Class)
	assert.Equal(t, []string{"human", "dwarf", "elf", "gnome", "half_elf", "halfling"}, got.Classes[4].Races, "DMG Table 7")
	assert.Equal(t, []string{
		"pick_pockets", "open_locks", "find_remove_traps", "move_silently",
		"hide_in_shadows", "detect_noise", "climb_walls", "read_languages",
	}, got.Classes[4].Skills, "PHB Table 26")
	assert.Equal(t, []string{"none", "leather", "padded_studded_leather", "elven_chain"}, got.Classes[4].SkillArmors)
	assert.Equal(t, []string{"climb_walls", "detect_noise", "pick_pockets", "read_languages"}, got.Classes[0].Skills, "PHB Table 33")
	assert.Equal(t, []string{"none", "leather", "padded_studded_leather", "elven_chain", "metal"}, got.Classes[0].SkillArmors)
	assert.Equal(t, struct{ Min, Max int }{1, 100}, got.StrExceptional)
	assert.Equal(t, 3, got.Ability.Min)
	assert.Equal(t, 19, got.Ability.Max)
}

// identity is the alignment and sex every test character enters (both required, ADR 0032 steps 2
// and 3), valid for each creatable class.
func identity() charactersvc.DetailsPatch {
	alignment, sex := "neutral_good", "male"
	return charactersvc.DetailsPatch{Alignment: &alignment, Sex: &sex}
}
