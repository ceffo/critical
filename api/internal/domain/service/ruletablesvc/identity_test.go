package ruletablesvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ADR 0032 step 5: PHB Tables 10 and 11, every figure written out again from
// the rulebook's tables (hand-checked), so a slip in the data fails here.
func TestIdentityRows_Step5_TheBooksTables(t *testing.T) {
	type d = ruletablesvc.Dice
	want := []ruletablesvc.IdentityRow{
		{"dwarf", 43, 41, d{1, 10}, 130, 105, d{4, 10}, 40, d{5, 6}, 250, d{2, 100}},
		{"elf", 55, 50, d{1, 10}, 90, 70, d{3, 10}, 100, d{5, 6}, 350, d{4, 100}},
		{"gnome", 38, 36, d{1, 6}, 72, 68, d{5, 4}, 60, d{3, 12}, 200, d{3, 100}},
		{"half_elf", 60, 58, d{2, 6}, 110, 85, d{3, 12}, 15, d{1, 6}, 125, d{3, 20}},
		{"halfling", 32, 30, d{2, 8}, 52, 48, d{5, 4}, 20, d{3, 4}, 100, d{1, 100}},
		{"human", 60, 59, d{2, 10}, 140, 100, d{6, 10}, 15, d{1, 4}, 90, d{2, 20}},
	}
	assert.Equal(t, want, ruletablesvc.IdentityRows())
	for _, r := range want {
		got, ok := ruletablesvc.IdentityFor(r.Race)
		require.True(t, ok)
		assert.Equal(t, r, got)
	}
	_, ok := ruletablesvc.IdentityFor("orc")
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
		h, ok := ruletablesvc.HeightRange(c.race, c.sex)
		require.True(t, ok)
		assert.InDeltaf(t, c.h.Min, h.Min, 1e-9, "%s %s height min", c.race, c.sex)
		assert.InDeltaf(t, c.h.Max, h.Max, 1e-9, "%s %s height max", c.race, c.sex)
		w, ok := ruletablesvc.WeightRange(c.race, c.sex)
		require.True(t, ok)
		assert.InDeltaf(t, c.w.Min, w.Min, 1e-9, "%s %s weight min", c.race, c.sex)
		assert.InDeltaf(t, c.w.Max, w.Max, 1e-9, "%s %s weight max", c.race, c.sex)
	}
	_, ok := ruletablesvc.HeightRange("orc", "male")
	assert.False(t, ok, "no race, no range")
}

// ADR 0032 step 5.1: the age runs from the lowest starting age to the highest
// maximum age.
func TestAgeRange_Step5_1_StartingToMaximum(t *testing.T) {
	want := map[string]ruletablesvc.Range{
		"dwarf": {Min: 45, Max: 450}, "elf": {Min: 105, Max: 750}, "gnome": {Min: 63, Max: 500},
		"half_elf": {Min: 16, Max: 185}, "halfling": {Min: 23, Max: 200}, "human": {Min: 16, Max: 130},
	}
	for race, w := range want {
		got, ok := ruletablesvc.AgeRange(race)
		require.True(t, ok)
		assert.Equal(t, w, got, race)
	}
}

// Every value a roll can make is inside its range, so a rolled value never warns.
func TestRanges_Step5_1_ARolledValueNeverWarns(t *testing.T) {
	for _, row := range ruletablesvc.IdentityRows() {
		for _, sex := range []string{"male", "female"} {
			h, _ := ruletablesvc.HeightRange(row.Race, sex)
			w, _ := ruletablesvc.WeightRange(row.Race, sex)
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
