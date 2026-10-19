package creationrollsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/creationrollsvc"
	"github.com/ceffo/critical/api/internal/domain/service/creationrollsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The creation's rolls, ADR 0032 steps 6 and 7. Every die the mock gives is
// distinct (101, 102, ... in the order drawn) and the exact number drawn is
// asserted, so a reused, missing or misordered die fails.

type fixture struct {
	svc   creationrollsvc.SVC
	rules *mocks.MockRules
	dice  *mocks.MockDice
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{rules: mocks.NewMockRules(t), dice: mocks.NewMockDice(t)}
	svc, err := creationrollsvc.New(creationrollsvc.Deps{Rules: f.rules, Dice: f.dice})
	require.NoError(t, err)
	f.svc = svc
	return f
}

// distinctD6 expects exactly n d6 and gives them the values first, first+1,
// ... in the order drawn.
func (f fixture) distinctD6(n, first int) {
	next := first
	f.dice.EXPECT().RollDie(6).RunAndReturn(func(int) int {
		next++
		return next - 1
	}).Times(n)
}

// seq is the n values from first on.
func seq(first, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = first + i
	}
	return out
}

func (f fixture) scores(t *testing.T, method string) *creationrollsvc.ScoresResult {
	t.Helper()
	res, err := f.svc.Scores(context.Background(), creationrollsvc.ScoresInput{Method: method})
	require.NoError(t, err)
	return res
}

// totals are res's totals in order, abilities the ability of each.
func totals(res *creationrollsvc.ScoresResult) (totals []int, abilities []string) {
	for _, r := range res.Rolls {
		totals = append(totals, r.Total)
		abilities = append(abilities, r.Ability)
	}
	return totals, abilities
}

var abilities = []string{"str", "dex", "con", "int", "wis", "cha"}

// Step 6, method I: 3d6 six times (18 dice), for Strength to Charisma in
// that order; the totals are the scores.
func TestScores_Step6_MethodI_SixTotalsAreTheScoresInAbilityOrder(t *testing.T) {
	f := newFixture(t)
	f.distinctD6(18, 1)
	res := f.scores(t, "I")
	require.Len(t, res.Rolls, 6)
	for i, r := range res.Rolls {
		assert.Equal(t, seq(1+3*i, 3), r.Dice, "roll %d: its own three dice, in order", i)
		assert.Nil(t, r.Dropped)
	}
	got, ab := totals(res)
	assert.Equal(t, []int{6, 15, 24, 33, 42, 51}, got)
	assert.Equal(t, abilities, ab)
	assert.Equal(t, map[string]int{"str": 6, "dex": 15, "con": 24, "int": 33, "wis": 42, "cha": 51}, res.Scores)
	assert.Nil(t, res.Adjustments, "no race, no adjustments")
	assert.Equal(t, ruletablesvc.AssignInOrder, res.Method.Assign)
}

// Step 6, method II: 3d6 twice for each ability (36 dice), the pair of an
// ability together, in ability order; the player keeps one of each pair.
func TestScores_Step6_MethodII_TwoTotalsPerAbilityTogether(t *testing.T) {
	f := newFixture(t)
	f.distinctD6(36, 101)
	res := f.scores(t, "II")
	require.Len(t, res.Rolls, 12)
	for i, r := range res.Rolls {
		assert.Equal(t, seq(101+3*i, 3), r.Dice, "roll %d", i)
	}
	_, ab := totals(res)
	assert.Equal(t, []string{"str", "str", "dex", "dex", "con", "con", "int", "int", "wis", "wis", "cha", "cha"}, ab)
	assert.Nil(t, res.Scores, "the player keeps one of each pair")
	assert.Equal(t, ruletablesvc.AssignKeepPerAbility, res.Method.Assign)
}

// Step 6, method III: 3d6 six times (18 dice), the totals unassigned.
func TestScores_Step6_MethodIII_SixUnassignedTotals(t *testing.T) {
	f := newFixture(t)
	f.distinctD6(18, 1)
	res := f.scores(t, "III")
	got, ab := totals(res)
	assert.Equal(t, []int{6, 15, 24, 33, 42, 51}, got)
	assert.Equal(t, []string{"", "", "", "", "", ""}, ab)
	assert.Nil(t, res.Scores)
	assert.Equal(t, ruletablesvc.AssignArrange, res.Method.Assign)
	assert.Equal(t, 6, res.Method.Keep)
}

// Step 6, method IV: 3d6 twelve times (36 dice), unassigned; the player
// keeps six.
func TestScores_Step6_MethodIV_TwelveUnassignedTotalsSixKept(t *testing.T) {
	f := newFixture(t)
	f.distinctD6(36, 1)
	res := f.scores(t, "IV")
	require.Len(t, res.Rolls, 12)
	for i, r := range res.Rolls {
		assert.Equal(t, seq(1+3*i, 3), r.Dice, "roll %d", i)
		assert.Empty(t, r.Ability)
		assert.Equal(t, 3+9*i+3, r.Total, "roll %d", i)
	}
	assert.Nil(t, res.Scores)
	assert.Equal(t, 6, res.Method.Keep)
}

// Step 6, method V: 4d6 six times (24 dice), the lowest die of each
// dropped and the other three added.
func TestScores_Step6_MethodV_FourDiceTheLowestDropped(t *testing.T) {
	f := newFixture(t)
	// Each group of four draws its lowest at a different place.
	draws := []int{
		1, 5, 6, 4, // lowest first
		6, 2, 5, 4, // second
		6, 5, 3, 4, // third
		6, 5, 4, 2, // last
		3, 3, 6, 5, // a tie: one dropped
		6, 6, 6, 6, // all equal
	}
	i := 0
	f.dice.EXPECT().RollDie(6).RunAndReturn(func(int) int { i++; return draws[i-1] }).Times(24)
	res := f.scores(t, "V")
	require.Len(t, res.Rolls, 6)
	wantDropped := []int{0, 1, 2, 3, 0, 0}
	wantTotals := []int{15, 15, 15, 15, 14, 18}
	for k, r := range res.Rolls {
		assert.Equal(t, draws[4*k:4*k+4], r.Dice, "roll %d keeps every die drawn", k)
		require.NotNil(t, r.Dropped, "roll %d", k)
		assert.Equal(t, wantDropped[k], *r.Dropped, "roll %d drops its lowest", k)
		assert.Equal(t, wantTotals[k], r.Total, "roll %d adds the other three", k)
	}
	assert.True(t, res.Method.DropLowest)
	assert.Nil(t, res.Scores)
}

// Step 6, method VI: seven d6 (7 dice), each ability starting at 8, no
// ability above 18; the dice are the totals.
func TestScores_Step6_MethodVI_SevenDiceAndTheBaseOf8(t *testing.T) {
	f := newFixture(t)
	f.distinctD6(7, 1)
	res := f.scores(t, "VI")
	got, ab := totals(res)
	assert.Equal(t, seq(1, 7), got)
	assert.Equal(t, []string{"", "", "", "", "", "", ""}, ab)
	for k, r := range res.Rolls {
		assert.Equal(t, []int{k + 1}, r.Dice)
	}
	assert.Equal(t, 8, res.Method.Base)
	assert.Equal(t, 18, res.Method.Max)
	assert.Equal(t, ruletablesvc.AssignAddToBase, res.Method.Assign)
	assert.Nil(t, res.Scores)
}

// Step 6: the roll shows PHB Table 8 for the race chosen next to the
// totals, and method I's scores are the adjusted ones (the stored score,
// ADR 0030 step 1).
func TestScores_Step6_TheRacesAdjustmentsComeWithTheRoll(t *testing.T) {
	f := newFixture(t)
	dwarf := map[string]int{"str": 0, "dex": 0, "con": 1, "int": 0, "wis": 0, "cha": -1}
	f.rules.EXPECT().RaceAdjustmentsFor("dwarf").Return(dwarf, nil).Once()
	f.distinctD6(18, 1)
	res, err := f.svc.Scores(context.Background(), creationrollsvc.ScoresInput{Method: "I", Race: "dwarf"})
	require.NoError(t, err)
	assert.Equal(t, dwarf, res.Adjustments)
	assert.Equal(t, map[string]int{"str": 6, "dex": 15, "con": 25, "int": 33, "wis": 42, "cha": 50}, res.Scores)
	got, _ := totals(res)
	assert.Equal(t, []int{6, 15, 24, 33, 42, 51}, got, "the totals stay as rolled")
}

// Step 6: an unknown method is refused on "method", an unknown race on
// "race", and nothing is rolled.
func TestScores_Step6_RefusesAnUnknownMethodOrRace(t *testing.T) {
	f := newFixture(t)
	for _, in := range []struct {
		in    creationrollsvc.ScoresInput
		field string
	}{
		{creationrollsvc.ScoresInput{Method: ""}, "method"},
		{creationrollsvc.ScoresInput{Method: "VII"}, "method"},
		{creationrollsvc.ScoresInput{Method: "i"}, "method"},
		{creationrollsvc.ScoresInput{Method: "I", Race: "orc"}, "race"},
	} {
		_, err := f.svc.Scores(context.Background(), in.in)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe, "%+v", in.in)
		assert.Equal(t, in.field, fe.Field)
		assert.Equal(t, "unknown", fe.Message)
	}
}

// The PHB tables of the HP tests: Table 3 rows and the groups' hit dice.
var (
	hitDice = map[string]ruletablesvc.HitDiceRow{
		"warrior": {Group: "warrior", Die: 10, NameLevel: 9, GainAfter: 3},
		"priest":  {Group: "priest", Die: 8, NameLevel: 9, GainAfter: 2},
		"rogue":   {Group: "rogue", Die: 6, NameLevel: 10, GainAfter: 2},
		"wizard":  {Group: "wizard", Die: 4, NameLevel: 10, GainAfter: 1},
	}
	con = map[int]ruletablesvc.ConstitutionRow{
		3:  {ConMin: 3, ConMax: 3, HP: -2, HPWarrior: -2},
		12: {ConMin: 7, ConMax: 14},
		16: {ConMin: 16, ConMax: 16, HP: 2, HPWarrior: 2},
		18: {ConMin: 18, ConMax: 18, HP: 2, HPWarrior: 4},
	}
)

// hp expects the tables of a class at level and runs the roll.
func (f fixture) hp(t *testing.T, class, group string, score, xp, level int) *creationrollsvc.HPResult {
	t.Helper()
	f.rules.EXPECT().LevelFor(class, xp).Return(level, nil).Once()
	f.rules.EXPECT().HitDiceFor(group).Return(hitDice[group], nil).Once()
	f.rules.EXPECT().ConstitutionFor(score).Return(con[score], nil).Once()
	res, err := f.svc.HP(context.Background(), creationrollsvc.HPInput{Class: class, Con: score, XP: xp})
	require.NoError(t, err)
	return res
}

func ptr(v int) *int { return &v }

// Step 7: a level 1 mage draws exactly one d4, plus Constitution 16's +2.
func TestHP_Step7_ALevel1MageDrawsOneD4(t *testing.T) {
	f := newFixture(t)
	f.dice.EXPECT().RollDie(4).Return(3).Once()
	res := f.hp(t, "mage", "wizard", 16, 0, 1)
	assert.Equal(t, []creationrollsvc.HPLevel{
		{Level: 1, HitDie: 4, Rolled: ptr(3), Counted: ptr(3), ConAdjustment: 2, Gain: 5},
	}, res.Levels)
	assert.Equal(t, 5, res.Total)
	assert.Equal(t, 1, res.Level)
}

// Step 7: a level 3 fighter draws three d10 in level order, each with the
// warriors' bracketed Constitution bonus (18: +4, where a priest gets +2).
func TestHP_Step7_ALevel3FighterDrawsThreeD10WithTheBracketedBonus(t *testing.T) {
	f := newFixture(t)
	f.dice.EXPECT().RollDie(10).Return(7).Once()
	f.dice.EXPECT().RollDie(10).Return(2).Once()
	f.dice.EXPECT().RollDie(10).Return(9).Once()
	res := f.hp(t, "fighter", "warrior", 18, 4000, 3)
	assert.Equal(t, []creationrollsvc.HPLevel{
		{Level: 1, HitDie: 10, Rolled: ptr(7), Counted: ptr(7), ConAdjustment: 4, Gain: 11},
		{Level: 2, HitDie: 10, Rolled: ptr(2), Counted: ptr(2), ConAdjustment: 4, Gain: 6},
		{Level: 3, HitDie: 10, Rolled: ptr(9), Counted: ptr(9), ConAdjustment: 4, Gain: 13},
	}, res.Levels)
	assert.Equal(t, 30, res.Total)
}

// Step 7: a level 12 bard (a rogue, name level 10) draws ten d6, levels 1 to
// 10 in order, then takes two fixed gains of 2 with no die and no
// Constitution bonus.
func TestHP_Step7_ALevel12BardTenDiceThenTwoFixedGains(t *testing.T) {
	f := newFixture(t)
	f.distinctD6(10, 101)
	res := f.hp(t, "bard", "rogue", 16, 1_000_000, 12)
	require.Len(t, res.Levels, 12)
	total := 0
	for i, l := range res.Levels[:10] {
		assert.Equal(t, creationrollsvc.HPLevel{
			Level: i + 1, HitDie: 6, Rolled: ptr(101 + i), Counted: ptr(101 + i), ConAdjustment: 2, Gain: 103 + i,
		}, l, "level %d", i+1)
		total += l.Gain
	}
	for _, l := range res.Levels[10:] {
		assert.Equal(t, creationrollsvc.HPLevel{Level: l.Level, Gain: 2, Fixed: true}, l)
		total += l.Gain
	}
	assert.Equal(t, []int{11, 12}, []int{res.Levels[10].Level, res.Levels[11].Level})
	assert.Equal(t, total, res.Total)
	assert.Equal(t, 10*103+45+4, res.Total)
}

// Step 7: a gain is at least 1 (a 1 with Constitution 3's -2 still gains 1;
// a 4 gains 2).
func TestHP_Step7_TheGainIsAtLeast1(t *testing.T) {
	f := newFixture(t)
	f.dice.EXPECT().RollDie(8).Return(1).Once()
	f.dice.EXPECT().RollDie(8).Return(4).Once()
	res := f.hp(t, "cleric", "priest", 3, 1500, 2)
	assert.Equal(t, []int{1, 2}, []int{res.Levels[0].Gain, res.Levels[1].Gain})
	assert.Equal(t, -2, res.Levels[0].ConAdjustment)
	assert.Equal(t, 3, res.Total)
}

// Step 7: the counted die is the Constitution row's raised low die
// (ADR 0030 step 7.1's arithmetic, ruletablesvc.HitDieGain, tested there
// with Constitution 21); the row the roll reads is the character's.
func TestHP_Step7_TheCountedDieFollowsTheConstitutionRow(t *testing.T) {
	f := newFixture(t)
	f.rules.EXPECT().LevelFor("thief", 0).Return(1, nil).Once()
	f.rules.EXPECT().HitDiceFor("rogue").Return(hitDice["rogue"], nil).Once()
	f.rules.EXPECT().ConstitutionFor(19).Return(ruletablesvc.ConstitutionRow{ConMin: 19, ConMax: 19, HP: 2, HPWarrior: 5, LowDieMin: 3}, nil).Once()
	f.dice.EXPECT().RollDie(6).Return(1).Once()
	res, err := f.svc.HP(context.Background(), creationrollsvc.HPInput{Class: "thief", Con: 19, XP: 0})
	require.NoError(t, err)
	assert.Equal(t, creationrollsvc.HPLevel{Level: 1, HitDie: 6, Rolled: ptr(1), Counted: ptr(3), ConAdjustment: 2, Gain: 5}, res.Levels[0])
}

// Step 7's refusals: class unknown, Constitution and XP out of the ranges of
// ADR 0030 step 1 (3 to 19, 0 to 100,000,000). Nothing is read or rolled.
func TestHP_Step7_RefusesAnUnknownClassAndOutOfRangeInputs(t *testing.T) {
	f := newFixture(t)
	for _, c := range []struct {
		in          creationrollsvc.HPInput
		field, code string
	}{
		{creationrollsvc.HPInput{Class: "paladin", Con: 10}, "class", "unknown"},
		{creationrollsvc.HPInput{Class: "", Con: 10}, "class", "unknown"},
		{creationrollsvc.HPInput{Class: "bard", Con: 2}, "con", "out_of_range"},
		{creationrollsvc.HPInput{Class: "bard", Con: 20}, "con", "out_of_range"},
		{creationrollsvc.HPInput{Class: "bard", Con: 10, XP: -1}, "xp", "out_of_range"},
		{creationrollsvc.HPInput{Class: "bard", Con: 10, XP: 100_000_001}, "xp", "out_of_range"},
	} {
		_, err := f.svc.HP(context.Background(), c.in)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe, "%+v", c.in)
		assert.Equal(t, c.field, fe.Field, "%+v", c.in)
		assert.Equal(t, c.code, fe.Message, "%+v", c.in)
		assert.True(t, errors.Is(err, domain.ErrInvalidInput))
	}
}
