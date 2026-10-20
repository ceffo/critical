package composition_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ADR 0034 (the class and skills steps) through the real services and the wire: the options carry
// each class's PHB Table 13 minimums, and the checks answer a raw draft with the classes whose
// minimum the scores miss and a thief's or a bard's budget at the level the XP gives.
func TestCreationChecks_OverTheWire(t *testing.T) {
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

	type checks struct {
		UnmetMinimums []struct {
			Class     string   `json:"class"`
			Abilities []string `json:"abilities"`
		} `json:"unmetMinimums"`
		SkillBudget *struct {
			Level, Available, Spent, Left, PerSkillCap int
		} `json:"skillBudget"`
	}
	var got checks
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/checks",
		`{"class":"bard","scores":{"dex":11,"int":13,"cha":15},"xp":0,"skillPoints":{"climb_walls":5}}`, &got))
	unmet := map[string][]string{}
	for _, u := range got.UnmetMinimums {
		unmet[u.Class] = u.Abilities
	}
	assert.Equal(t, []string{"dex"}, unmet["bard"])
	assert.Empty(t, unmet["fighter"])
	require.NotNil(t, got.SkillBudget)
	assert.Equal(t, 1, got.SkillBudget.Level)
	assert.Equal(t, 20, got.SkillBudget.Available)
	assert.Equal(t, 15, got.SkillBudget.Left)

	var fighter checks
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/checks", `{"class":"fighter","xp":0}`, &fighter))
	assert.Nil(t, fighter.SkillBudget)
}

type checksAnswer struct {
	UnmetMinimums []struct {
		Class     string   `json:"class"`
		Abilities []string `json:"abilities"`
	} `json:"unmetMinimums"`
	SkillBudget *struct {
		Level, Available, Spent, Left, PerSkillCap int
	} `json:"skillBudget"`
}

func (a checksAnswer) unmet() map[string][]string {
	out := map[string][]string{}
	for _, u := range a.UnmetMinimums {
		out[u.Class] = u.Abilities
	}
	return out
}

// PHB Table 13 on the wire: Dex 11 misses the bard's 12 and only the bard's (the thief asks 9), Dex 12
// clears it, and abilities with no score yet are never flagged.
func TestCreationChecks_Table13_DexFlagsTheBardOnlyAndPartialScoresFlagNothing(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	var a checksAnswer
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/checks", `{"class":"bard","scores":{"dex":11,"int":13,"cha":15}}`, &a))
	assert.Equal(t, []string{"dex"}, a.unmet()["bard"])
	for _, c := range []string{"fighter", "mage", "cleric", "thief"} {
		assert.Empty(t, a.unmet()[c], c)
	}
	var b checksAnswer
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/checks", `{"class":"bard","scores":{"dex":12,"int":13,"cha":15}}`, &b))
	assert.Empty(t, b.unmet()["bard"])
	var c checksAnswer
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/checks", `{"class":"bard","scores":{"str":3}}`, &c))
	assert.Empty(t, c.unmet()["bard"])
}

// ADR 0030 steps 17 and 18: at the level 2 threshold a thief has 90 points in all and 45 in one
// skill; the bard's has no per-skill limit.
func TestCreationChecks_Steps17And18_LevelTwoBudgets(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	var thief, bard checksAnswer
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/checks", `{"class":"thief","xp":1250}`, &thief))
	require.NotNil(t, thief.SkillBudget)
	assert.Equal(t, 2, thief.SkillBudget.Level)
	assert.Equal(t, 90, thief.SkillBudget.Available)
	assert.Equal(t, 45, thief.SkillBudget.PerSkillCap)
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/checks", `{"class":"bard","xp":1250}`, &bard))
	require.NotNil(t, bard.SkillBudget)
	assert.Equal(t, 35, bard.SkillBudget.Available)
	assert.Zero(t, bard.SkillBudget.PerSkillCap)
}

// A draft too raw to read is no error: a negative XP or a class nobody can create has no budget.
func TestCreationChecks_RawDraftsGetNoBudgetAndNoError(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	for _, body := range []string{`{"class":"thief","xp":-5}`, `{"class":"paladin","xp":0}`, `{"class":"","xp":0}`} {
		var a checksAnswer
		require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/checks", body, &a), body)
		assert.Nil(t, a.SkillBudget, body)
	}
}
