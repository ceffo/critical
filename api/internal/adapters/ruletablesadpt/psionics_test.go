package ruletablesadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The PSP maximum of ADR 0043 step 4 on the fixture's CPsiH Table 5 (rows 15
// to 18: base 20, 22, 24, 26; modifier 0, 1, 2, 3) through the psionicist's
// psionics params (step 2.2). Expected values are the book's (CPsiH ch. 1,
// Psionic Strength Points), worked by hand.

// pspRule is the fixture psionicist's PSP rule.
func pspRule(t *testing.T) classes.PSPRule {
	t.Helper()
	tbl := phb(t)
	defs, err := tbl.Definitions()
	require.NoError(t, err)
	c, err := defs.Get("psionicist")
	require.NoError(t, err)
	f, ok := c.Lists(classes.KindPsionics)
	require.True(t, ok)
	return f.Psionics.PSP
}

func scores(wis, intl, con int) func(string) int {
	return func(a string) int {
		return map[string]int{"wis": wis, "int": intl, "con": con}[a]
	}
}

// Step 4.1: the book's example, Wisdom 17, Constitution 16, Intelligence 15:
// Table 5's base at Wisdom (24) plus the modifiers at Intelligence (0) and
// Constitution (1) gives 25 at level 1.
func TestPSPMax_Step4_1_InherentPotential(t *testing.T) {
	got, err := phb(t).PSPMaxFor(pspRule(t), scores(17, 15, 16), 1)
	require.NoError(t, err)
	assert.Equal(t, ruletablesvc.PSPMax{InherentPotential: 25, PerLevel: 12, PerLevelModifier: 2, Max: 25}, got)

	got, err = phb(t).PSPMaxFor(pspRule(t), scores(15, 18, 18), 1)
	require.NoError(t, err)
	assert.Equal(t, 26, got.InherentPotential, "base 20 at Wis 15, +3 at Int 18 and +3 at Con 18")
}

// Step 4.2: each level from 2 adds Table 5's Wisdom modifier plus 10, no
// die (Wis 17: 12), and the gain never stops: a psionicist 15 with the
// example's scores has 25 + 14 x 12 = 193.
func TestPSPMax_Step4_2_EachLevelAddsWisdomModifierPlusTen(t *testing.T) {
	for _, tc := range []struct {
		level, want int
	}{{1, 25}, {2, 37}, {15, 193}, {20, 253}} {
		got, err := phb(t).PSPMaxFor(pspRule(t), scores(17, 15, 16), tc.level)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got.Max, "level %d", tc.level)
	}
	got, err := phb(t).PSPMaxFor(pspRule(t), scores(15, 15, 15), 3)
	require.NoError(t, err)
	assert.Equal(t, 20+2*10, got.Max, "Wis 15: modifier 0, so 10 a level")

	_, err = phb(t).PSPMaxFor(pspRule(t), scores(17, 15, 16), 0)
	require.Error(t, err, "no level 0")
}

// Step 4.3 (Q1): Table 5 has rows 15 to 18 only. A score under 15 gives a
// modifier of 0 (and a base ability under the table is refused: the class
// minimum keeps it in); over 18, each point adds 2 to the base and 1 to the
// modifier (19: 28 and +4).
func TestPSPMax_Step4_3_ScoresOutsideTable5(t *testing.T) {
	got, err := phb(t).PSPMaxFor(pspRule(t), scores(19, 9, 3), 2)
	require.NoError(t, err)
	assert.Equal(t, ruletablesvc.PSPMax{InherentPotential: 28, PerLevel: 14, PerLevelModifier: 4, Max: 42}, got,
		"Wis 19: base 28, modifier 4; Int 9 and Con 3 add 0")

	row, err := phb(t).InherentPotentialFor(20)
	require.NoError(t, err)
	assert.Equal(t, ruletablesvc.InherentPotentialRow{Score: 20, Base: 30, Modifier: 5}, row)

	_, err = phb(t).PSPMaxFor(pspRule(t), scores(14, 15, 16), 1)
	require.Error(t, err, "a base ability under Table 5 has no base")
}

// Step 6: Table 6's hourly rates read from the fixture as data (exertion 0,
// walking 3, resting 6, sleeping 12); an activity the table lacks is an error.
func TestPSPRecoveryRate_Step6_Table6AsData(t *testing.T) {
	for activity, want := range map[string]int{"exertion": 0, "walking": 3, "resting": 6, "sleeping": 12} {
		got, err := phb(t).PSPRecoveryRate(activity)
		require.NoError(t, err)
		assert.Equal(t, want, got, activity)
	}
	_, err := phb(t).PSPRecoveryRate("meditating")
	require.Error(t, err)
}

// Step 9.1: Table 4 as data, read at the entry's realized level. The book's
// level 1 psionicist has 1 discipline, 1 science, 3 devotions and 1 defence
// mode; level 3 has 2, 2, 7 and 2.
func TestPowerAllowance_Step9_1_Table4AsData(t *testing.T) {
	for _, tc := range []struct {
		level int
		want  ruletablesvc.PowerProgressionRow
	}{
		{1, ruletablesvc.PowerProgressionRow{Level: 1, Disciplines: 1, Sciences: 1, Devotions: 3, DefenceModes: 1}},
		{3, ruletablesvc.PowerProgressionRow{Level: 3, Disciplines: 2, Sciences: 2, Devotions: 7, DefenceModes: 2}},
	} {
		got, err := phb(t).PowerAllowanceFor(tc.level)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got, "level %d", tc.level)
	}
	last, err := phb(t).PowerAllowanceFor(20)
	require.NoError(t, err)
	past, err := phb(t).PowerAllowanceFor(25)
	require.NoError(t, err)
	assert.Equal(t, last, past, "past the table's last level the last row holds")
	_, err = phb(t).PowerAllowanceFor(0)
	assert.Error(t, err)
}
