package creationrollsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
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
	// ADR 0040 step 5: the classes and races are the catalogue's.
	f.rules.EXPECT().Definitions().RunAndReturn(cataloguetest.Tables(t).Definitions).Maybe()
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

// xpAt is the XP at which the fixture catalogue's class reaches level.
func xpAt(t *testing.T, class string, level int) int {
	t.Helper()
	def, err := cataloguetest.Definitions(t).Get(class)
	require.NoError(t, err)
	xp, err := def.XPThreshold(level)
	require.NoError(t, err)
	return xp
}

// hp expects the tables of a class at xp and runs the roll; the class definition gives the level.
func (f fixture) hp(t *testing.T, class, group string, score, xp int) *creationrollsvc.HPResult {
	t.Helper()
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
	res := f.hp(t, "mage", "wizard", 16, 0)
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
	res := f.hp(t, "fighter", "warrior", 18, 4000)
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
	res := f.hp(t, "bard", "rogue", 16, xpAt(t, "bard", 12))
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
	res := f.hp(t, "cleric", "priest", 3, 1500)
	assert.Equal(t, []int{1, 2}, []int{res.Levels[0].Gain, res.Levels[1].Gain})
	assert.Equal(t, -2, res.Levels[0].ConAdjustment)
	assert.Equal(t, 3, res.Total)
}

// Step 7: the counted die is the Constitution row's raised low die
// (ADR 0030 step 7.1's arithmetic, ruletablesvc.HitDieGain, tested there
// with Constitution 21); the row the roll reads is the character's.
func TestHP_Step7_TheCountedDieFollowsTheConstitutionRow(t *testing.T) {
	f := newFixture(t)
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

// The PHB Table 8 rows of the arrangement tests.
var (
	dwarfAdj = map[string]int{"str": 0, "dex": 0, "con": 1, "int": 0, "wis": 0, "cha": -1}
	elfAdj   = map[string]int{"str": 0, "dex": 1, "con": -1, "int": 0, "wis": 0, "cha": 0}
)

func (f fixture) arrange(t *testing.T, in creationrollsvc.ArrangeInput) *creationrollsvc.ArrangeResult {
	t.Helper()
	res, err := f.svc.Arrange(context.Background(), in)
	require.NoError(t, err)
	return res
}

// Step 6, arrangement of an arranged method (III): an ability's score is the
// total placed plus the race's Table 8 adjustment; an ability with nothing
// placed has no score. No die is drawn (the dice mock expects none).
func TestArrange_Step6_ArrangedTotalPlusTheRacesAdjustment(t *testing.T) {
	f := newFixture(t)
	f.rules.EXPECT().RaceAdjustmentsFor("dwarf").Return(dwarfAdj, nil).Once()
	res := f.arrange(t, creationrollsvc.ArrangeInput{
		Method: "III", Race: "dwarf",
		Placed: map[string][]int{"str": {17}, "con": {12}, "cha": {9}},
	})
	assert.Equal(t, map[string]int{"str": 17, "con": 12, "cha": 9}, res.Rolled)
	assert.Equal(t, map[string]int{"str": 17, "con": 13, "cha": 8}, res.Scores)
	assert.Equal(t, dwarfAdj, res.Adjustments)
	assert.Equal(t, "III", res.Method.Code)
}

// Step 6, method VI: every ability starts at the base 8, the dice placed on
// it are added whole, then Table 8; with no race the score is the roll.
func TestArrange_Step6_MethodVIBase8PlusTheDicePlaced(t *testing.T) {
	f := newFixture(t)
	res := f.arrange(t, creationrollsvc.ArrangeInput{
		Method: "VI",
		Placed: map[string][]int{"str": {6, 4}, "dex": {1}, "int": {3, 2, 5}},
	})
	want := map[string]int{"str": 18, "dex": 9, "con": 8, "int": 18, "wis": 8, "cha": 8}
	assert.Equal(t, want, res.Rolled)
	assert.Equal(t, want, res.Scores)
	assert.Nil(t, res.Adjustments)
}

// Step 6, method VI's max: the dice may raise an ability to 18 and no
// further; the race's adjustment comes after the max (an elf's 18 Dexterity
// is 19), and a placement above 18 is refused on that ability.
func TestArrange_Step6_MethodVINoAbilityAbove18BeforeTheRace(t *testing.T) {
	f := newFixture(t)
	f.rules.EXPECT().RaceAdjustmentsFor("elf").Return(elfAdj, nil).Times(2)
	res := f.arrange(t, creationrollsvc.ArrangeInput{
		Method: "VI", Race: "elf",
		Placed: map[string][]int{"dex": {6, 4}},
	})
	assert.Equal(t, 18, res.Rolled["dex"])
	assert.Equal(t, 19, res.Scores["dex"])
	assert.Equal(t, 7, res.Scores["con"], "the base 8, less the elf's 1")

	_, err := f.svc.Arrange(context.Background(), creationrollsvc.ArrangeInput{
		Method: "VI", Race: "elf",
		Placed: map[string][]int{"dex": {6, 4, 1}},
	})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "placed.dex", fe.Field)
	assert.Equal(t, creationrollsvc.CodeAboveMax, fe.Message)
}

// Step 6 with ADR 0030 step 1: the same placement for another race gives
// that race's scores (the stored score is the adjusted one), so a race
// changed after the roll changes the scores and the adjustments shown.
func TestArrange_Step6_AnotherRaceAdjustsTheSamePlacement(t *testing.T) {
	f := newFixture(t)
	f.rules.EXPECT().RaceAdjustmentsFor("dwarf").Return(dwarfAdj, nil).Once()
	f.rules.EXPECT().RaceAdjustmentsFor("elf").Return(elfAdj, nil).Once()
	placed := map[string][]int{"str": {10}, "dex": {11}, "con": {12}, "int": {13}, "wis": {14}, "cha": {15}}
	dwarf := f.arrange(t, creationrollsvc.ArrangeInput{Method: "V", Race: "dwarf", Placed: placed})
	elf := f.arrange(t, creationrollsvc.ArrangeInput{Method: "V", Race: "elf", Placed: placed})
	assert.Equal(t, map[string]int{"str": 10, "dex": 11, "con": 13, "int": 13, "wis": 14, "cha": 14}, dwarf.Scores)
	assert.Equal(t, map[string]int{"str": 10, "dex": 12, "con": 11, "int": 13, "wis": 14, "cha": 15}, elf.Scores)
	assert.Equal(t, elfAdj, elf.Adjustments)
}

// Step 6: Table 8 may take a 3 to 2 (or an 18 to 19); the arrangement gives
// the adjusted score as it is, and the creation's range check refuses a 2
// (ADR 0030 step 1), not the arrangement.
func TestArrange_Step6_TheAdjustedScoreIsNotClamped(t *testing.T) {
	f := newFixture(t)
	f.rules.EXPECT().RaceAdjustmentsFor("dwarf").Return(dwarfAdj, nil).Once()
	res := f.arrange(t, creationrollsvc.ArrangeInput{
		Method: "IV", Race: "dwarf",
		Placed: map[string][]int{"con": {18}, "cha": {3}},
	})
	assert.Equal(t, map[string]int{"con": 19, "cha": 2}, res.Scores)
}

// Step 6: a placement the method cannot make is refused, on the ability or
// on "placed": two totals on one ability (II to V), a value one total cannot
// be (3d6: 3 to 18; method V keeps three dice; method VI's die: 1 to 6), more
// totals than the method keeps, an unknown ability; and an unknown method or
// race as for the roll.
func TestArrange_Step6_RefusesWhatTheMethodCannotMake(t *testing.T) {
	f := newFixture(t)
	vi := func(n int) []int {
		out := make([]int, n)
		for i := range out {
			out[i] = 1
		}
		return out
	}
	for _, c := range []struct {
		in          creationrollsvc.ArrangeInput
		field, code string
	}{
		{creationrollsvc.ArrangeInput{Method: "III", Placed: map[string][]int{"str": {10, 11}}}, "placed.str", "too_many"},
		{creationrollsvc.ArrangeInput{Method: "II", Placed: map[string][]int{"wis": {2}}}, "placed.wis", "out_of_range"},
		{creationrollsvc.ArrangeInput{Method: "IV", Placed: map[string][]int{"dex": {19}}}, "placed.dex", "out_of_range"},
		{creationrollsvc.ArrangeInput{Method: "V", Placed: map[string][]int{"int": {19}}}, "placed.int", "out_of_range"},
		{creationrollsvc.ArrangeInput{Method: "VI", Placed: map[string][]int{"cha": {7}}}, "placed.cha", "out_of_range"},
		{creationrollsvc.ArrangeInput{Method: "VI", Placed: map[string][]int{"cha": {0}}}, "placed.cha", "out_of_range"},
		{creationrollsvc.ArrangeInput{Method: "VI", Placed: map[string][]int{"str": vi(4), "dex": vi(4)}}, "placed", "too_many"},
		{creationrollsvc.ArrangeInput{Method: "III", Placed: map[string][]int{"luck": {10}}}, "placed", "unknown"},
		{creationrollsvc.ArrangeInput{Method: "VII"}, "method", "unknown"},
		{creationrollsvc.ArrangeInput{Method: "III", Race: "orc"}, "race", "unknown"},
	} {
		_, err := f.svc.Arrange(context.Background(), c.in)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe, "%+v", c.in)
		assert.Equal(t, c.field, fe.Field, "%+v", c.in)
		assert.Equal(t, c.code, fe.Message, "%+v", c.in)
	}
}

// Step 5.3, for each of the six races and both sexes: height, weight and age
// are rolled with that row's dice, in that order, every die given its own
// value and the exact number of dice asserted; each total is the row's base
// (for the sex) plus its dice, and the stored value the conversion of it.
func TestIdentity_Step5_3_EachRowRollsItsOwnBaseAndDice(t *testing.T) {
	for _, row := range ruletablesvc.IdentityRows(cataloguetest.Definitions(t)) {
		for _, sex := range []string{"male", "female"} {
			t.Run(row.Race+" "+sex, func(t *testing.T) {
				f := newFixture(t)
				next := 0
				expect := func(d ruletablesvc.Dice) []int {
					out := make([]int, d.Count)
					for i := range out {
						next++
						out[i] = next
						f.dice.EXPECT().RollDie(d.Sides).Return(next).Once()
					}
					return out
				}
				hDice, wDice, aDice := expect(row.HeightDice), expect(row.WeightDice), expect(row.StartAgeDice)

				// Asked in a different order: drawn height, weight, age all the same.
				res, err := f.svc.Identity(context.Background(), creationrollsvc.IdentityInput{
					Race: row.Race, Sex: sex, Figures: []string{"age", "weight", "height"},
				})
				require.NoError(t, err)
				require.Len(t, res.Rolls, 3)
				sum := func(v []int) (s int) {
					for _, x := range v {
						s += x
					}
					return s
				}
				baseH, _ := row.BaseHeight(sex)
				baseW, _ := row.BaseWeight(sex)
				assert.Equal(t, creationrollsvc.IdentityRoll{
					Figure: "height", Dice: hDice, Sides: row.HeightDice.Sides, Base: baseH, Total: baseH + sum(hDice),
					Unit: "in", Value: ruletablesvc.InchesToCm(float64(baseH + sum(hDice))),
				}, res.Rolls[0])
				assert.Equal(t, creationrollsvc.IdentityRoll{
					Figure: "weight", Dice: wDice, Sides: row.WeightDice.Sides, Base: baseW, Total: baseW + sum(wDice),
					Unit: "lb", Value: ruletablesvc.PoundsToKg(float64(baseW + sum(wDice))),
				}, res.Rolls[1])
				assert.Equal(t, creationrollsvc.IdentityRoll{
					Figure: "age", Dice: aDice, Sides: row.StartAgeDice.Sides, Base: row.StartAge,
					Total: row.StartAge + sum(aDice), Unit: "year", Value: float64(row.StartAge + sum(aDice)),
				}, res.Rolls[2])
			})
		}
	}
}

// Step 5.3, an exact conversion: a half-elf female rolling 2d6 = 3 and 4 is
// 58 + 7 = 65 in = 165.1 cm; one roll per figure asked, so asking the age
// alone draws only the age dice and needs no sex.
func TestIdentity_Step5_3_OneRollPerFigureAsked(t *testing.T) {
	f := newFixture(t)
	f.dice.EXPECT().RollDie(6).Return(3).Once()
	f.dice.EXPECT().RollDie(6).Return(4).Once()
	res, err := f.svc.Identity(context.Background(), creationrollsvc.IdentityInput{Race: "half_elf", Sex: "female", Figures: []string{"height"}})
	require.NoError(t, err)
	require.Len(t, res.Rolls, 1)
	assert.Equal(t, 65, res.Rolls[0].Total)
	assert.InDelta(t, 165.1, res.Rolls[0].Value, 1e-9)

	f.dice.EXPECT().RollDie(6).Return(5).Once() // half-elf's age: 1d6
	res, err = f.svc.Identity(context.Background(), creationrollsvc.IdentityInput{Race: "half_elf", Figures: []string{"age"}})
	require.NoError(t, err)
	assert.Equal(t, []int{5}, res.Rolls[0].Dice)
	assert.Equal(t, 20.0, res.Rolls[0].Value)
}

// Step 5.3, the refusals draw no die: the race and, for a height or a weight,
// the sex are needed, named on the figure rolled; an unknown value and a bad
// list of figures are refused.
func TestIdentity_Step5_3_RefusalsRollNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		in          creationrollsvc.IdentityInput
		field, code string
	}{
		"no race":           {creationrollsvc.IdentityInput{Sex: "male", Figures: []string{"weight", "age"}}, "weight", "race_required"},
		"no sex for height": {creationrollsvc.IdentityInput{Race: "elf", Figures: []string{"age", "height"}}, "height", "sex_required"},
		"no sex for weight": {creationrollsvc.IdentityInput{Race: "elf", Figures: []string{"weight"}}, "weight", "sex_required"},
		"unknown race":      {creationrollsvc.IdentityInput{Race: "orc", Sex: "male", Figures: []string{"age"}}, "race", "unknown"},
		"unknown sex":       {creationrollsvc.IdentityInput{Race: "elf", Sex: "other", Figures: []string{"age"}}, "sex", "unknown"},
		"no figure":         {creationrollsvc.IdentityInput{Race: "elf", Sex: "male"}, "figures", "empty"},
		"unknown figure":    {creationrollsvc.IdentityInput{Race: "elf", Sex: "male", Figures: []string{"hair"}}, "figures", "unknown"},
		"a figure twice":    {creationrollsvc.IdentityInput{Race: "elf", Sex: "male", Figures: []string{"age", "age"}}, "figures", "duplicate"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t) // a mock with no expectation fails on any die drawn
			_, err := f.svc.Identity(context.Background(), tc.in)
			fe, ok := errors.AsType[*domain.FieldError](err)
			require.True(t, ok, "%v", err)
			assert.Equal(t, tc.field, fe.Field)
			assert.Equal(t, tc.code, fe.Message)
		})
	}
}
