package composition_test

// Creating a multi-class or dual-class character from class entries (ADR 0042 creation steps
// 1, 2 and 4) end to end: POST /api/characters/draft and POST /api/characters over the real
// services, the fixture's rule tables and the in-memory storage.

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// classesBody is a body entering a character of race with the class entries in classes (a JSON
// array), with the scores and max HP given and an alignment every fixture class admits.
func classesBody(race, scores, classes string) string {
	return fmt.Sprintf(`{"name":"Party","raceCode":%q,%s,"hpMax":60,"classes":%s,`+
		`"details":{"alignment":"neutral_good","sex":"male"}}`, race, scores, classes)
}

const (
	strong = `"str":17,"dex":16,"con":15,"int":17,"wis":16,"cha":10`
	// weak misses the dual-class prime requisites (15 and 17).
	weak = `"str":14,"dex":12,"con":12,"int":16,"wis":10,"cha":10`
)

// refusalsOf is the draft's refusals as "field:code".
func refusalsOf(d draftView) []string {
	out := []string{}
	for _, r := range d.Refusals {
		out = append(out, r.Field+":"+r.Code)
	}
	return out
}

// createdEntries are a created character's class entries as GET /api/character serves them.
type createdEntries struct {
	Classes []struct {
		Class       string `json:"class"`
		Level       int    `json:"level"`
		StartingXP  int    `json:"startingXp"`
		State       string `json:"state"`
		LeftAtLevel int    `json:"leftAtLevel"`
	} `json:"classes"`
	HPMax   int `json:"hpMax"`
	Derived struct {
		Classes []struct {
			Class string `json:"class"`
			Level int    `json:"level"`
			XP    int    `json:"xp"`
			State string `json:"state"`
		} `json:"classes"`
	} `json:"derived"`
}

// create posts body to the creation, which must answer 201, and reads the new current character.
func (tb table) createFrom(t *testing.T, as, body string) createdEntries {
	t.Helper()
	var list listView
	require.Equal(t, http.StatusCreated, tb.call(as, "POST", "/api/characters", body, &list), body)
	require.NotNil(t, list.CurrentID)
	var got createdEntries
	require.Equal(t, http.StatusOK, tb.showing(as, *list.CurrentID, "GET", "/api/character", "", &got))
	return got
}

// Step 1: the entries as a whole. Each shape the book forbids is refused with a code, on the
// draft and on the creation alike.
func TestCreate_Step1_TheShapeOfTheEntries(t *testing.T) {
	for _, tt := range []struct {
		name, classes string
		want          []string
	}{
		{"a class twice", `[{"class":"fighter","xp":9000},{"class":"fighter","xp":9000}]`, []string{"classes.1.class:duplicate_class"}},
		{"four classes", `[{"class":"fighter","xp":9000},{"class":"cleric","xp":9000},{"class":"thief","xp":9000},{"class":"mage","xp":9000}]`, []string{"classes:too_many_classes"}},
		{"no active class", `[{"class":"fighter","leftAtLevel":9}]`, []string{"classes:no_active_class"}},
		{"multi and dual", `[{"class":"fighter","leftAtLevel":9},{"class":"cleric","xp":9000},{"class":"thief","xp":9000}]`, []string{"classes:multi_and_dual_class"}},
		{"back to the class left", `[{"class":"fighter","leftAtLevel":9},{"class":"fighter","xp":0}]`, []string{"classes.1.class:class_already_left"}},
		{"left at level 1", `[{"class":"fighter","leftAtLevel":1},{"class":"mage","xp":0}]`, []string{"classes.0.leftAtLevel:level_too_low"}},
		{"left at level 21", `[{"class":"fighter","leftAtLevel":21},{"class":"mage","xp":0}]`, []string{"classes.0.leftAtLevel:out_of_range"}},
		{"a level past 20", `[{"class":"fighter","xp":9000,"level":21}]`, []string{"classes.0.level:out_of_range"}},
		{"a class that does not exist", `[{"class":"paladin","xp":9000}]`, []string{"classes.0.class:not_creatable"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tb, _ := serverOn(t, memory.New())
			bob, _ := tb.login("bob@example.com")
			body := classesBody("human", strong, tt.classes)
			assert.Equal(t, tt.want, refusalsOf(tb.draft(t, bob, body)), "draft")
			assert.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", body, nil), "creation")
		})
	}
}

// A body naming both class and classes is a malformed request, not a refusal; a class entry
// without XP is listed missing by the draft.
func TestCreate_Step1_ClassAndClassesTogether(t *testing.T) {
	tb, _ := serverOn(t, memory.New())
	bob, _ := tb.login("bob@example.com")
	both := `{"name":"P","class":"fighter","raceCode":"human","classes":[{"class":"fighter","xp":9000}]}`
	assert.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters/draft", both, nil))
	d := tb.draft(t, bob, `{"name":"P","raceCode":"half_elf","classes":[{"class":"fighter"},{"class":"cleric","xp":9000}]}`)
	assert.Contains(t, d.Missing, "classes.0.xp")
	assert.NotContains(t, d.Missing, "classes.1.xp")
}

// Step 2.1: a multi-class character. With the lift (the default ruleset) a human fighter/thief
// is accepted; without it, refused; a half-elf fighter/cleric is accepted either way.
func TestCreate_Step2_1_TheLiftAllowsAnyRaceForMultiClass(t *testing.T) {
	humanFT := classesBody("human", strong, `[{"class":"fighter","xp":9000},{"class":"thief","xp":9000}]`)
	halfElfFC := classesBody("half_elf", strong, `[{"class":"fighter","xp":9000},{"class":"cleric","xp":9000}]`)
	tb, bs := serverOn(t, memory.New())
	bob, _ := tb.login("bob@example.com")
	assert.Empty(t, refusalsOf(tb.draft(t, bob, humanFT)), "with the lift")
	assert.Empty(t, refusalsOf(tb.draft(t, bob, halfElfFC)))

	ann, err := bs.Auth.DevLogin(context.Background(), "")
	require.NoError(t, err)
	_, err = bs.Rulesets.SetRacialLift(context.Background(), ann.User.ID, ruleset.RacialLift{})
	require.NoError(t, err)
	assert.Equal(t, []string{"classes:multiclass_not_allowed"}, refusalsOf(tb.draft(t, bob, humanFT)), "without the lift")
	assert.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", humanFT, nil))
	assert.Empty(t, refusalsOf(tb.draft(t, bob, halfElfFC)), "a half-elf is a demihuman")
}

// Step 2.2: a dual-class character. Prime requisites 15 in the class left and 17 in the new one
// (the fighter's Strength, the mage's Intelligence), the class left at level 2 or more, and
// without the lift only a human.
func TestCreate_Step2_2_DualClassPrimeRequisitesAndRace(t *testing.T) {
	dual := `[{"class":"fighter","leftAtLevel":9},{"class":"mage","xp":0}]`
	tb, bs := serverOn(t, memory.New())
	bob, _ := tb.login("bob@example.com")
	assert.Empty(t, refusalsOf(tb.draft(t, bob, classesBody("elf", strong, dual))), "an elf, with the lift")
	// Str 14 is under the 15 the fighter needs; Int 16 is under the 17 the mage needs.
	assert.Equal(t, []string{"classes.0.class:prime_requisite_too_low", "classes.1.class:prime_requisite_too_low"},
		refusalsOf(tb.draft(t, bob, classesBody("elf", weak, dual))))
	body := classesBody("elf", weak, dual)
	assert.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", body, nil))

	ann, err := bs.Auth.DevLogin(context.Background(), "")
	require.NoError(t, err)
	_, err = bs.Rulesets.SetRacialLift(context.Background(), ann.User.ID, ruleset.RacialLift{MulticlassAnyRace: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"classes:dualclass_not_allowed"}, refusalsOf(tb.draft(t, bob, classesBody("elf", strong, dual))))
	assert.Empty(t, refusalsOf(tb.draft(t, bob, classesBody("human", strong, dual))), "a human is the book's dual-class")
}

// Step 4: an existing character is entered as its sheet has it: each entry's level and XP, a class
// left at its left-at level (the XP that reaches it when none is entered), the max HP as entered.
func TestCreate_Step4_EntersLevelAndXPPerEntry(t *testing.T) {
	tb, _ := serverOn(t, memory.New())
	bob, _ := tb.login("bob@example.com")

	elf := tb.createFrom(t, bob, classesBody("elf", strong, `[{"class":"fighter","leftAtLevel":9},{"class":"mage","xp":135000,"level":8}]`))
	require.Len(t, elf.Classes, 2)
	assert.Equal(t, "fighter", elf.Classes[0].Class)
	assert.Equal(t, "frozen", elf.Classes[0].State)
	assert.Equal(t, 9, elf.Classes[0].Level)
	assert.Equal(t, 9, elf.Classes[0].LeftAtLevel)
	assert.Equal(t, 250_000, elf.Classes[0].StartingXP, "the XP that reaches level 9, none entered")
	assert.Equal(t, "mage", elf.Classes[1].Class)
	assert.Equal(t, "active", elf.Classes[1].State)
	assert.Equal(t, 8, elf.Classes[1].Level)
	assert.Equal(t, 135_000, elf.Classes[1].StartingXP)
	assert.Equal(t, 60, elf.HPMax, "the max HP as entered")

	// A level entered above the XP's own is kept (a level-up not pending).
	hf := tb.createFrom(t, bob, classesBody("half_elf", strong, `[{"class":"fighter","xp":1000000,"level":13},{"class":"cleric","xp":800000}]`))
	require.Len(t, hf.Classes, 2)
	assert.Equal(t, 13, hf.Classes[0].Level)
	assert.Equal(t, 1_000_000, hf.Classes[0].StartingXP)
	assert.Equal(t, 11, hf.Classes[1].Level, "no level entered: the level its XP reaches")
	assert.Equal(t, 800_000, hf.Classes[1].StartingXP)

	// A level entered below the one the XP reaches is kept as entered: the sheet says the character
	// has not taken that level yet, so the level-up is pending (900,000 XP reaches level 12).
	below := tb.createFrom(t, bob, classesBody("half_elf", strong, `[{"class":"fighter","xp":900000,"level":10},{"class":"cleric","xp":9000}]`))
	assert.Equal(t, 10, below.Classes[0].Level)
	assert.Equal(t, 900_000, below.Classes[0].StartingXP)
	assert.Equal(t, 10, below.Derived.Classes[0].Level, "the realized level, not the one the XP reaches")

	// A level equal to the XP's own is the same as leaving it out.
	equal := tb.createFrom(t, bob, classesBody("half_elf", strong, `[{"class":"fighter","xp":9000,"level":5},{"class":"cleric","xp":9000}]`))
	assert.Equal(t, 5, equal.Classes[0].Level)
}

// A class left keeps the XP typed even when it is under the threshold of its left-at level (the
// sheet is trusted, as for any entry); left blank it takes the threshold. A level typed on a
// class left must be its left-at level.
func TestCreate_Step4_TheClassLeftsXP(t *testing.T) {
	tb, _ := serverOn(t, memory.New())
	bob, _ := tb.login("bob@example.com")
	low := tb.createFrom(t, bob, classesBody("elf", strong, `[{"class":"fighter","leftAtLevel":9,"xp":1000},{"class":"mage","xp":0}]`))
	assert.Equal(t, 1_000, low.Classes[0].StartingXP)
	assert.Equal(t, 9, low.Classes[0].Level)
	blank := tb.createFrom(t, bob, classesBody("elf", strong, `[{"class":"fighter","leftAtLevel":9},{"class":"mage","xp":0}]`))
	assert.Equal(t, 250_000, blank.Classes[0].StartingXP)

	body := classesBody("elf", strong, `[{"class":"fighter","leftAtLevel":9,"level":8},{"class":"mage","xp":0}]`)
	assert.Equal(t, []string{"classes.0.level:out_of_range"}, refusalsOf(tb.draft(t, bob, body)))
}

// The party's characters, through the draft and the creation: the draft answers no refusal and a
// derived block, and the creation stores what the draft showed.
func TestCreate_ThePartyThroughTheDraftAndTheCreation(t *testing.T) {
	for _, tt := range []struct {
		name, race, scores, classes string
		levels                      []int
	}{
		{"the elf, a fighter 9 who became a mage 8", "elf", strong, `[{"class":"fighter","leftAtLevel":9,"xp":250000},{"class":"mage","xp":135000,"level":8}]`, []int{9, 8}},
		{"the half-elf fighter/cleric", "half_elf", strong, `[{"class":"fighter","xp":1000000,"level":13},{"class":"cleric","xp":800000,"level":14}]`, []int{13, 14}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tb, _ := serverOn(t, memory.New())
			bob, _ := tb.login("bob@example.com")
			body := classesBody(tt.race, tt.scores, tt.classes)
			d := tb.draft(t, bob, body)
			assert.Empty(t, d.Refusals)
			assert.Empty(t, d.Missing)
			require.NotNil(t, d.Derived)
			got := tb.createFrom(t, bob, body)
			require.Len(t, got.Classes, len(tt.levels))
			for i, lv := range tt.levels {
				assert.Equal(t, lv, got.Classes[i].Level)
			}
			assert.Equal(t, d.Derived["thac0"], float64(thac0Of(t, tb, bob)), "the draft shows what the creation stores")
		})
	}
}

// thac0Of reads the current character's THAC0.
func thac0Of(t *testing.T, tb table, as string) int {
	t.Helper()
	var got struct {
		Derived struct {
			THAC0 int `json:"thac0"`
		} `json:"derived"`
	}
	require.Equal(t, http.StatusOK, tb.call(as, "GET", "/api/character", "", &got))
	return got.Derived.THAC0
}

// ADR 0040 step 8 and ADR 0043 step 3: the human thief/psionicist, the party's third character,
// is creatable now that every kind the psionicist lists (psionics, save_bonus) is computed: the
// draft refuses nothing and the creation succeeds.
func TestCreate_Step8_ThePartyThiefPsionicistIsCreatable(t *testing.T) {
	tb, _ := serverOn(t, memory.New())
	bob, _ := tb.login("bob@example.com")
	body := classesBody("human", strong, `[{"class":"thief","xp":1000000},{"class":"psionicist","xp":1000000}]`)
	assert.Empty(t, refusalsOf(tb.draft(t, bob, body)))
	assert.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", body, nil))
}
