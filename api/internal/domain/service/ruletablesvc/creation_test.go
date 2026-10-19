package ruletablesvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ADR 0032 step 6's table: I 3d6 six times in ability order (18 dice), II
// 3d6 twice per ability (36), III 3d6 six times arranged (18), IV 3d6 twelve
// times, six kept and arranged (36), V 4d6 six times, the lowest dropped (24),
// VI seven d6 added whole to abilities starting at 8, none above 18 (7).
func TestScoreMethods_Step6_TheDMGMethods(t *testing.T) {
	type want struct {
		rolls, dice, keep, base, max, drawn int
		drop                                bool
		assign                              string
	}
	cases := map[string]want{
		"I":   {6, 3, 6, 0, 0, 18, false, ruletablesvc.AssignInOrder},
		"II":  {12, 3, 6, 0, 0, 36, false, ruletablesvc.AssignKeepPerAbility},
		"III": {6, 3, 6, 0, 0, 18, false, ruletablesvc.AssignArrange},
		"IV":  {12, 3, 6, 0, 0, 36, false, ruletablesvc.AssignArrange},
		"V":   {6, 4, 6, 0, 0, 24, true, ruletablesvc.AssignArrange},
		"VI":  {7, 1, 7, 8, 18, 7, false, ruletablesvc.AssignAddToBase},
	}
	require.Len(t, ruletablesvc.ScoreMethods, len(cases))
	for code, w := range cases {
		m, ok := ruletablesvc.ScoreMethodFor(code)
		require.True(t, ok, code)
		got := want{m.Rolls, m.Dice, m.Keep, m.Base, m.Max, m.Rolls * m.Dice, m.DropLowest, m.Assign}
		assert.Equal(t, w, got, "method %s", code)
	}
	for _, code := range []string{"", "i", "VII", "3d6"} {
		_, ok := ruletablesvc.ScoreMethodFor(code)
		assert.False(t, ok, code)
	}
}

// ADR 0030 step 7.1, the arithmetic ADR 0032 step 7 shares: the die counted
// (Table 3 raises a low die from Constitution 20: 1 and 2 count as 3 at 21),
// the warriors' bracketed adjustment, and a gain of at least 1.
func TestHitDieGain_Step7_1_RaisedDieWarriorBonusAndFloor(t *testing.T) {
	con21 := ruletablesvc.ConstitutionRow{ConMin: 21, ConMax: 22, HP: 2, HPWarrior: 6, LowDieMin: 3}
	con18 := ruletablesvc.ConstitutionRow{ConMin: 18, ConMax: 18, HP: 2, HPWarrior: 4}
	con3 := ruletablesvc.ConstitutionRow{ConMin: 3, ConMax: 3, HP: -2, HPWarrior: -2}
	type out struct{ counted, adj, gain int }
	for name, c := range map[string]struct {
		group  string
		rolled int
		con    ruletablesvc.ConstitutionRow
		want   out
	}{
		"Constitution 21 raises a 1 to 3":            {"rogue", 1, con21, out{3, 2, 5}},
		"Constitution 21 raises a 2 to 3":            {"warrior", 2, con21, out{3, 6, 9}},
		"Constitution 21 keeps a 4":                  {"wizard", 4, con21, out{4, 2, 6}},
		"a warrior takes the bracketed value":        {"warrior", 7, con18, out{7, 4, 11}},
		"a priest takes the plain value":             {"priest", 7, con18, out{7, 2, 9}},
		"the gain is at least 1":                     {"wizard", 2, con3, out{2, -2, 1}},
		"a die above the penalty gains what is left": {"wizard", 4, con3, out{4, -2, 2}},
	} {
		counted, adj, gain := ruletablesvc.HitDieGain(c.group, c.rolled, c.con)
		assert.Equal(t, c.want, out{counted, adj, gain}, name)
	}
}
