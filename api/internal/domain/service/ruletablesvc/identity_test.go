package ruletablesvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// row is the catalogue's Tables 10 and 11 row of race.
func row(t *testing.T, race string) ruletablesvc.IdentityRow {
	t.Helper()
	r, ok := ruletablesvc.IdentityFor(cataloguetest.Definitions(t), race)
	require.True(t, ok, race)
	return r
}

// ADR 0032 step 5: PHB Tables 10 and 11, every figure written out again from
// the rulebook's tables (hand-checked), so a slip in the data fails here.
func TestIdentityRows_Step5_TheBooksTables(t *testing.T) {
	type d = ruletablesvc.Dice
	want := []ruletablesvc.IdentityRow{
		{"dwarf", 43, 41, d{Count: 1, Sides: 10}, 130, 105, d{Count: 4, Sides: 10}, 40, d{Count: 5, Sides: 6}, 250, d{Count: 2, Sides: 100}},
		{"elf", 55, 50, d{Count: 1, Sides: 10}, 90, 70, d{Count: 3, Sides: 10}, 100, d{Count: 5, Sides: 6}, 350, d{Count: 4, Sides: 100}},
		{"gnome", 38, 36, d{Count: 1, Sides: 6}, 72, 68, d{Count: 5, Sides: 4}, 60, d{Count: 3, Sides: 12}, 200, d{Count: 3, Sides: 100}},
		{"half_elf", 60, 58, d{Count: 2, Sides: 6}, 110, 85, d{Count: 3, Sides: 12}, 15, d{Count: 1, Sides: 6}, 125, d{Count: 3, Sides: 20}},
		{"halfling", 32, 30, d{Count: 2, Sides: 8}, 52, 48, d{Count: 5, Sides: 4}, 20, d{Count: 3, Sides: 4}, 100, d{Count: 1, Sides: 100}},
		{"human", 60, 59, d{Count: 2, Sides: 10}, 140, 100, d{Count: 6, Sides: 10}, 15, d{Count: 1, Sides: 4}, 90, d{Count: 2, Sides: 20}},
	}
	assert.Equal(t, want, ruletablesvc.IdentityRows(cataloguetest.Definitions(t)))
	for _, r := range want {
		got, ok := ruletablesvc.IdentityFor(cataloguetest.Definitions(t), r.Race)
		require.True(t, ok)
		assert.Equal(t, r, got)
	}
	_, ok := ruletablesvc.IdentityFor(cataloguetest.Definitions(t), "orc")
	assert.False(t, ok)
}

// ADR 0032 step 5.1: the range of a figure is the base plus the dice at 1 and
// at their maximum, converted (1 in = 2.54 cm, 1 lb = 0.45359237 kg) and
// rounded outward to the tenth; an empty sex is the union of the two.
func TestHeightWeightRange_Step5_1_RoundedOutward(t *testing.T) {
	type rg = ruletablesvc.Range
	cases := []struct {
		race, sex string
		h, w      rg
	}{
		{"dwarf", "male", rg{111.7, 134.7}, rg{60.7, 77.2}},
		{"dwarf", "female", rg{106.6, 129.6}, rg{49.4, 65.8}},
		{"elf", "male", rg{142.2, 165.1}, rg{42.1, 54.5}},
		{"elf", "female", rg{129.5, 152.4}, rg{33.1, 45.4}},
		{"gnome", "male", rg{99.0, 111.8}, rg{34.9, 41.8}},
		{"gnome", "female", rg{93.9, 106.7}, rg{33.1, 40.0}},
		{"half_elf", "male", rg{157.4, 182.9}, rg{51.2, 66.3}},
		{"half_elf", "female", rg{152.4, 177.8}, rg{39.9, 54.9}},
		{"halfling", "male", rg{86.3, 122.0}, rg{25.8, 32.7}},
		{"halfling", "female", rg{81.2, 116.9}, rg{24.0, 30.9}},
		{"human", "male", rg{157.4, 203.2}, rg{66.2, 90.8}},
		{"human", "female", rg{154.9, 200.7}, rg{48.0, 72.6}},
		// no sex: the union of the two rows
		{"human", "", rg{154.9, 203.2}, rg{48.0, 90.8}},
		{"halfling", "", rg{81.2, 122.0}, rg{24.0, 32.7}},
	}
	for _, c := range cases {
		h := row(t, c.race).HeightRange(c.sex)
		assert.InDeltaf(t, c.h.Min, h.Min, 1e-9, "%s %s height min", c.race, c.sex)
		assert.InDeltaf(t, c.h.Max, h.Max, 1e-9, "%s %s height max", c.race, c.sex)
		w := row(t, c.race).WeightRange(c.sex)
		assert.InDeltaf(t, c.w.Min, w.Min, 1e-9, "%s %s weight min", c.race, c.sex)
		assert.InDeltaf(t, c.w.Max, w.Max, 1e-9, "%s %s weight max", c.race, c.sex)
	}
}

// ADR 0032 step 5.1: the age runs from the lowest starting age to the highest
// maximum age.
func TestAgeRange_Step5_1_StartingToMaximum(t *testing.T) {
	want := map[string]ruletablesvc.Range{
		"dwarf": {Min: 45, Max: 450}, "elf": {Min: 105, Max: 750}, "gnome": {Min: 63, Max: 500},
		"half_elf": {Min: 16, Max: 185}, "halfling": {Min: 23, Max: 200}, "human": {Min: 16, Max: 130},
	}
	for race, w := range want {
		assert.Equal(t, w, row(t, race).AgeRange(), race)
	}
}

// Every value a roll can make is inside its range, so a rolled value never warns.
func TestRanges_Step5_1_ARolledValueNeverWarns(t *testing.T) {
	for _, row := range ruletablesvc.IdentityRows(cataloguetest.Definitions(t)) {
		for _, sex := range []string{"male", "female"} {
			h, w := row.HeightRange(sex), row.WeightRange(sex)
			bh, _ := row.BaseHeight(sex)
			bw, _ := row.BaseWeight(sex)
			for _, in := range []int{row.HeightDice.Min(), row.HeightDice.Max()} {
				v := ruletablesvc.InchesToCm(float64(bh + in))
				assert.GreaterOrEqual(t, v, h.Min, row.Race)
				assert.LessOrEqual(t, v, h.Max, row.Race)
			}
			for _, in := range []int{row.WeightDice.Min(), row.WeightDice.Max()} {
				v := ruletablesvc.PoundsToKg(float64(bw + in))
				assert.GreaterOrEqual(t, v, w.Min, row.Race)
				assert.LessOrEqual(t, v, w.Max, row.Race)
			}
		}
	}
}

// ADR 0040 step 2: the creation rolls and the ranges read the race definition's rows, so a race the
// code has never seen (a test giant) has its own Tables 10 and 11 row and range with no code change,
// and the book's rows are the catalogue's, not a literal.
func TestIdentity_Step2_TablesTenAndElevenFromTheCatalogue(t *testing.T) {
	book := cataloguetest.Definitions(t)
	giant := book.Races()[0]
	giant.ID = "test_giant"
	giant.Identity = classes.Identity{
		HeightMale: 100, HeightFemale: 90, HeightDice: classes.Dice{Count: 2, Sides: 10},
		WeightMale: 300, WeightFemale: 250, WeightDice: classes.Dice{Count: 3, Sides: 10},
		StartAge: 30, StartAgeDice: classes.Dice{Count: 2, Sides: 6}, MaxAge: 400, MaxAgeDice: classes.Dice{Count: 1, Sides: 100},
	}
	defs, err := classes.New(book.Classes(), book.GroupDefs(), append(book.Races(), giant))
	require.NoError(t, err)

	got, ok := ruletablesvc.IdentityFor(defs, "test_giant")
	require.True(t, ok)
	assert.Equal(t, giant.Identity.HeightDice, got.HeightDice)
	assert.Equal(t, 100, got.HeightMale)
	assert.InDelta(t, 2.54*102, got.HeightRange("male").Min, 0.2)
	assert.Equal(t, ruletablesvc.Range{Min: 32, Max: 500}, got.AgeRange(), "start age 30 + 2 to max age 400 + 100")
}
