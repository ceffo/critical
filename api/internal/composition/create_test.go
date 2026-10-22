package composition_test

// Creating, switching and retiring characters end to end over the in-memory
// storage (ADR 0029 steps 7.2 and 7.3, ADR 0030 steps 1 and 21): the
// router, the real services and the fixture's rule tables.

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

const bardStart = `{"name":"Lyra","class":"bard","raceCode":"half_elf",` +
	`"str":10,"dex":15,"con":12,"int":14,"wis":9,"cha":16,"hpMax":6,"xp":2500`

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
		TargetLevel   int `json:"targetLevel"`
		RealizedLevel int `json:"realizedLevel"`
		THAC0         int `json:"thac0"`
	} `json:"derived"`
}

// A player with no character creates a bard: it is theirs and current at
// once, its level and THAC0 come from the rules at the XP entered (2,500 XP
// is a level 3 bard, THAC0 19), and it owns nothing: no weapon, item,
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
		`"str":9,"dex":16,"con":10,"int":13,"wis":11,"cha":17,"hpMax":4,"xp":0,` +
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
			ExceptionalStrength []string `json:"exceptionalStrengthRaces"` // retired: the draft answers it (ADR 0040 step 3)
			Skills              []string `json:"skills"`
			SkillArmors         []string `json:"skillArmors"` // retired (ADR 0033 step 14)
		} `json:"classes"`
		Ability        struct{ Min, Max int } `json:"ability"`
		StrExceptional struct{ Min, Max int } `json:"strExceptional"`
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters/options", "", &got))
	// ADR 0040 step 8: the classes and races in the catalogue's order.
	order := make([]string, 0, len(got.Classes))
	for _, c := range got.Classes {
		order = append(order, c.Class)
	}
	require.Equal(t, []string{"fighter", "mage", "cleric", "thief", "bard"}, order)
	fighter, mage, cleric, thief, bard := got.Classes[0], got.Classes[1], got.Classes[2], got.Classes[3], got.Classes[4]
	assert.Equal(t, []string{"half_elf", "human"}, bard.Races)
	assert.Equal(t, []string{"dwarf", "elf", "gnome", "half_elf", "halfling", "human"}, fighter.Races)
	assert.Equal(t, []string{"elf", "half_elf", "human"}, mage.Races, "DMG Table 7")
	assert.Equal(t, []string{"dwarf", "elf", "gnome", "half_elf", "halfling", "human"}, cleric.Races, "DMG Table 7")
	assert.Equal(t, []string{"dwarf", "elf", "gnome", "half_elf", "halfling", "human"}, thief.Races, "DMG Table 7")
	assert.Equal(t, []string{
		"pick_pockets", "open_locks", "find_remove_traps", "move_silently",
		"hide_in_shadows", "detect_noise", "climb_walls", "read_languages",
	}, thief.Skills, "PHB Table 26")
	assert.Nil(t, thief.SkillArmors, "no armour is entered: the skills read the worn one (ADR 0033 step 14)")
	assert.Equal(t, []string{"climb_walls", "detect_noise", "pick_pockets", "read_languages"}, bard.Skills, "PHB Table 33")
	assert.Nil(t, bard.SkillArmors)
	for _, c := range got.Classes {
		assert.Nil(t, c.ExceptionalStrength, "%s: the 18/xx is the draft's exceptionalStrength (ADR 0040 step 3)", c.Class)
	}
	assert.Equal(t, struct{ Min, Max int }{1, 100}, got.StrExceptional)
	assert.Equal(t, 3, got.Ability.Min)
	assert.Equal(t, 19, got.Ability.Max)
}

// ADR 0034 step 4.1 over the fixture's tables: each class's level to XP table is ADR 0030 step 3's
// XP table read at each level's lower bound (PHB Tables 14, 20, 23 and 25), so a level picked in the
// form and sent as that XP derives that very level; the hit die, the score methods and Table 8 ride
// along for the class, score and race steps.
func TestCreate_Options_LevelXPTableHitDiceMethodsAndRaceAdjustments(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	var got struct {
		Classes []struct {
			Class   string `json:"class"`
			HitDie  int    `json:"hitDie"`
			LevelXP []struct {
				Level int `json:"level"`
				XP    int `json:"xp"`
			} `json:"levelXp"`
		} `json:"classes"`
		ScoreMethods []struct {
			Code   string `json:"code"`
			Assign string `json:"assign"`
		} `json:"scoreMethods"`
		RaceAdjustments []struct {
			Race        string         `json:"race"`
			Adjustments map[string]int `json:"adjustments"`
		} `json:"raceAdjustments"`
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters/options", "", &got))
	want := map[string]struct{ die, xp5 int }{
		"bard": {6, 10_000}, "fighter": {10, 16_000}, "mage": {4, 20_000}, "cleric": {8, 13_000}, "thief": {6, 10_000},
	}
	require.Len(t, got.Classes, 5)
	for _, c := range got.Classes {
		require.Len(t, c.LevelXP, 20, c.Class)
		assert.Equal(t, 1, c.LevelXP[0].Level, c.Class)
		assert.Equal(t, 0, c.LevelXP[0].XP, c.Class)
		assert.Equal(t, 5, c.LevelXP[4].Level, c.Class)
		assert.Equal(t, want[c.Class].xp5, c.LevelXP[4].XP, c.Class)
		assert.Equal(t, want[c.Class].die, c.HitDie, c.Class)
	}
	codes := make([]string, 0, len(got.ScoreMethods))
	for _, m := range got.ScoreMethods {
		codes = append(codes, m.Code)
	}
	assert.Equal(t, []string{"I", "II", "III", "IV", "V", "VI"}, codes)
	assert.Equal(t, "add_to_base", got.ScoreMethods[5].Assign)
	require.Len(t, got.RaceAdjustments, 6)
	assert.Equal(t, "dwarf", got.RaceAdjustments[0].Race, "the catalogue's race order (ADR 0040 step 8)")
	for _, r := range got.RaceAdjustments {
		if r.Race == "elf" {
			assert.Equal(t, 1, r.Adjustments["dex"], "PHB Table 8")
			assert.Equal(t, -1, r.Adjustments["con"], "PHB Table 8")
		}
	}

	// A picked level sent as its XP derives that level (ADR 0030 step 3).
	var hp hpRollView
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/rolls/hp",
		fmt.Sprintf(`{"class":"bard","con":12,"xp":%d}`, got.Classes[0].LevelXP[4].XP), &hp))
	assert.Equal(t, 5, hp.Level)
}

// identity is the alignment and sex every test character enters (both required, ADR 0032 steps 2
// and 3), valid for each creatable class.
func identity() charactersvc.DetailsPatch {
	alignment, sex := "neutral_good", "male"
	return charactersvc.DetailsPatch{Alignment: &alignment, Sex: &sex}
}

// The roster card's HP is the HP panel's: after damage, a heal, an undone
// entry (voided) and an amended note, the list and GET /api/hp agree (ADR 0007
// step 6, ADR 0011 step 1), and the list carries the identity the card shows.
func TestList_TheCardsHPIsTheHPPanels(t *testing.T) {
	tb, _ := serverOn(t, memory.New())
	tok, _ := tb.login("hp@example.com")
	var created listView
	body := strings.Replace(newBard, `"hpMax":6`, `"hpMax":40`, 1)
	require.Equal(t, http.StatusCreated, tb.call(tok, "POST", "/api/characters", body, &created))
	id := created.Characters[0].ID

	var hp hpView
	for _, ev := range []string{
		`{"delta":-12,"source":"orc"}`, `{"delta":-9,"source":"trap"}`, `{"delta":5,"source":"potion"}`,
	} {
		require.Equal(t, http.StatusOK, tb.showing(tok, id, "POST", "/api/hp/event", ev, &hp))
	}
	// A voided entry: damage, then undone.
	require.Equal(t, http.StatusOK, tb.showing(tok, id, "POST", "/api/hp/event", `{"delta":-20,"source":"fall"}`, &hp))
	type logView struct {
		Entries []struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"entries"`
		TopID int64 `json:"topId"`
	}
	var log logView
	require.Equal(t, http.StatusOK, tb.showing(tok, id, "GET", "/api/log", "", &log))
	require.Equal(t, http.StatusOK, tb.showing(tok, id, "POST", "/api/log/undo", `{"expectedTopId":`+itoa(log.TopID)+`}`, nil))
	// An amended entry: its note changes, never its HP.
	log = logView{}
	require.Equal(t, http.StatusOK, tb.showing(tok, id, "GET", "/api/log", "", &log))
	var amend int64
	for _, e := range log.Entries {
		if e.Status == "active" && amend == 0 {
			amend = e.ID
		}
	}
	require.NotZero(t, amend)
	require.Equal(t, http.StatusOK, tb.showing(tok, id, "POST", "/api/log/"+itoa(amend)+"/amend", `{"note":"corrected"}`, nil))

	require.Equal(t, http.StatusOK, tb.showing(tok, id, "GET", "/api/hp", "", &hp))
	require.Equal(t, 40-12-9+5, hp.CurrentHp, "the panel: the voided entry counts for nothing")

	var list struct {
		Characters []struct {
			Race      string   `json:"race"`
			Alignment string   `json:"alignment"`
			Sex       string   `json:"sex"`
			Missing   []string `json:"missing"`
			CurrentHP int      `json:"currentHp"`
			MaxHP     int      `json:"maxHp"`
		} `json:"characters"`
	}
	require.Equal(t, http.StatusOK, tb.call(tok, "GET", "/api/characters", "", &list))
	require.Len(t, list.Characters, 1)
	c := list.Characters[0]
	assert.Equal(t, hp.CurrentHp, c.CurrentHP, "the card shows the panel's HP")
	assert.Equal(t, 40, c.MaxHP)
	assert.Equal(t, "half_elf", c.Race)
	assert.Equal(t, "neutral_good", c.Alignment)
	assert.Equal(t, "male", c.Sex)
	assert.Equal(t, []string{}, c.Missing)
}

// ADR 0033 step 14 retires the entered skill armour: a create or a PATCH body
// that names skillArmor is an unknown field, refused (400) with nothing
// written; the skills read the worn armour instead (derived.skillArmor).
func TestCharacters_Step14_SkillArmourIsNoEntry(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	withArmour := newBard[:len(newBard)-1] + `,"skillArmor":"leather"}`
	assert.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", withArmour, nil))
	var list listView
	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", newBard, &list))
	require.Len(t, list.Characters, 1, "the refused body created nothing")
	id := *list.CurrentID
	assert.Equal(t, http.StatusBadRequest, tb.call(bob, "PATCH", "/api/characters/"+itoa(id), `{"skillArmor":"elven_chain"}`, nil))
	var shown struct {
		Derived struct {
			SkillArmor string `json:"skillArmor"`
		} `json:"derived"`
	}
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/character", "", &shown))
	assert.Equal(t, "none", shown.Derived.SkillArmor, "a new bard wears nothing")
}
