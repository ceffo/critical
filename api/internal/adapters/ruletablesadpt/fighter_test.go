package ruletablesadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The fighter's rules (ADR 0030 steps 9, 11 and 12) over the fixture's class
// tables. Expected rates are read from PHB Tables 15 (p.25), 35 (p.53) and
// 45 (p.68), not from the artifact.

// ADR 0030 step 9: an exceptional strength is a class's that lists
// exceptional_strength (ADR 0040 step 3), at Strength 18, never a halfling's.
func TestExceptionalStrengthAllowed_Step9(t *testing.T) {
	tests := []struct {
		name, class, race string
		str               int
		want              bool
	}{
		{"human fighter at 18", "fighter", "human", 18, true},
		{"dwarf fighter at 18", "fighter", "dwarf", 18, true},
		{"fighter at 17", "fighter", "human", 17, false},
		{"fighter at 19", "fighter", "human", 19, false},
		{"halfling fighter", "fighter", "halfling", 18, false},
		{"bard at 18", "bard", "human", 18, false},
		{"mage at 18", "mage", "human", 18, false},
		{"cleric at 18", "cleric", "human", 18, false},
		{"thief at 18", "thief", "human", 18, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ruletablesvc.ExceptionalStrengthAllowed(fixtureClass(t, tt.class), tt.race, tt.str))
		})
	}
}

// ADR 0030 step 12: only fighters specialize (they list weapon_specialization).
func TestSpecializes_Step12_FightersOnly(t *testing.T) {
	assert.True(t, ruletablesvc.Specializes(fixtureClass(t, "fighter")))
	for _, class := range []string{"bard", "mage", "cleric", "thief"} {
		assert.False(t, ruletablesvc.Specializes(fixtureClass(t, class)), class)
	}
}

// ADR 0030 step 11: the rate by weapon kind, type, level and specialization.
func TestAttackRateFor_Step11(t *testing.T) {
	tbl := phb(t)
	rate := func(a, r int) *ruletablesvc.AttackRate { return &ruletablesvc.AttackRate{Attacks: a, Rounds: r} }
	tests := []struct {
		name             string
		class            string
		level            int
		kind, weaponType string
		specialized      bool
		want             *ruletablesvc.AttackRate // nil: no rate
	}{
		// 11.1 melee: Table 15, specialized Table 35's melee column.
		{"melee level 1", "fighter", 1, "melee", "long_sword", false, rate(1, 1)},
		{"melee level 6", "fighter", 6, "melee", "long_sword", false, rate(1, 1)},
		{"melee level 7", "fighter", 7, "melee", "short_sword", false, rate(3, 2)},
		{"melee level 12", "fighter", 12, "melee", "other_melee", false, rate(3, 2)},
		{"melee level 13", "fighter", 13, "melee", "long_sword", false, rate(2, 1)},
		{"specialized melee level 1", "fighter", 1, "melee", "long_sword", true, rate(3, 2)},
		{"specialized melee level 7", "fighter", 7, "melee", "long_sword", true, rate(2, 1)},
		{"specialized melee level 13", "fighter", 13, "melee", "long_sword", true, rate(5, 2)},
		{"melee dagger", "fighter", 13, "melee", "dagger", false, rate(2, 1)},
		// 11.2 missile: Table 45 whatever the level; specialized non-bow Table 35.
		{"heavy crossbow level 13", "fighter", 13, "missile", "heavy_crossbow", false, rate(1, 2)},
		{"specialized heavy crossbow level 13", "fighter", 13, "missile", "heavy_crossbow", true, rate(3, 2)},
		{"specialized heavy crossbow level 1", "fighter", 1, "missile", "heavy_crossbow", true, rate(1, 2)},
		{"light crossbow", "fighter", 1, "missile", "light_crossbow", false, rate(1, 1)},
		{"specialized light crossbow level 7", "fighter", 7, "missile", "light_crossbow", true, rate(3, 2)},
		{"long bow level 1", "fighter", 1, "missile", "long_bow", false, rate(2, 1)},
		{"long bow level 13", "fighter", 13, "missile", "long_bow", false, rate(2, 1)},
		{"specialized long bow keeps the rate of fire", "fighter", 13, "missile", "long_bow", true, rate(2, 1)},
		{"specialized short bow keeps the rate of fire", "fighter", 7, "missile", "short_bow", true, rate(2, 1)},
		{"thrown dagger", "fighter", 1, "missile", "dagger", false, rate(2, 1)},
		{"specialized thrown dagger level 7", "fighter", 7, "missile", "dagger", true, rate(4, 1)},
		{"dart", "fighter", 20, "missile", "dart", false, rate(3, 1)},
		{"specialized dart level 13", "fighter", 13, "missile", "dart", true, rate(6, 1)},
		{"sling", "fighter", 7, "missile", "sling", false, rate(1, 1)},
		{"specialized sling reads other missile", "fighter", 7, "missile", "sling", true, rate(2, 1)},
		{"specialized hand crossbow reads other missile", "fighter", 13, "missile", "hand_crossbow", true, rate(5, 2)},
		{"specialized other thrown reads other missile", "fighter", 1, "missile", "other_thrown", true, rate(3, 2)},
		// 11.3 no rate.
		{"no type", "fighter", 13, "melee", "", false, nil},
		{"specialized with no type", "fighter", 13, "melee", "", true, nil},
		{"missile type without a rate of fire", "fighter", 13, "missile", "other_missile", false, nil},
		{"other thrown without a rate of fire", "fighter", 13, "missile", "other_thrown", false, nil},
		{"not a warrior", "thief", 15, "missile", "long_bow", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := tbl.AttackRateFor(tt.class, tt.level, tt.kind, tt.weaponType, tt.specialized)
			require.NoError(t, err)
			if tt.want == nil {
				assert.False(t, ok)
				return
			}
			require.True(t, ok)
			assert.Equal(t, *tt.want, got)
		})
	}
}

// ADR 0030 step 11.4 (proposed): a half rate gives the lower count in odd
// rounds and the higher in even rounds.
func TestAttackRateInRound_Step11_4(t *testing.T) {
	tests := []struct {
		rate        ruletablesvc.AttackRate
		round, want int
	}{
		{ruletablesvc.AttackRate{Attacks: 3, Rounds: 2}, 1, 1},
		{ruletablesvc.AttackRate{Attacks: 3, Rounds: 2}, 2, 2},
		{ruletablesvc.AttackRate{Attacks: 3, Rounds: 2}, 3, 1},
		{ruletablesvc.AttackRate{Attacks: 5, Rounds: 2}, 1, 2},
		{ruletablesvc.AttackRate{Attacks: 5, Rounds: 2}, 4, 3},
		{ruletablesvc.AttackRate{Attacks: 1, Rounds: 2}, 1, 0},
		{ruletablesvc.AttackRate{Attacks: 1, Rounds: 2}, 2, 1},
		{ruletablesvc.AttackRate{Attacks: 3, Rounds: 2}, 0, 1},
		{ruletablesvc.AttackRate{Attacks: 2, Rounds: 1}, 1, 2},
		{ruletablesvc.AttackRate{Attacks: 2, Rounds: 1}, 2, 2},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.rate.InRound(tt.round), "%d/%d in round %d", tt.rate.Attacks, tt.rate.Rounds, tt.round)
	}
}
