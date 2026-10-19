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
			Level, Total, Spent, Left, PerSkill int
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
	assert.Equal(t, 20, got.SkillBudget.Total)
	assert.Equal(t, 15, got.SkillBudget.Left)

	var fighter checks
	require.Equal(t, http.StatusOK, tb.call(bob, "POST", "/api/characters/checks", `{"class":"fighter","xp":0}`, &fighter))
	assert.Nil(t, fighter.SkillBudget)
}
