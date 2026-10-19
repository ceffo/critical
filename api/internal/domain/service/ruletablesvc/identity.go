package ruletablesvc

import (
	"math"
	"slices"
)

// PHB Table 10 (average height and weight) and Table 11 (age), the data of
// ADR 0032 step 5: per race the base height and weight for each sex and the
// modifier dice, and the starting age's base and variable dice. The book
// gives inches, pounds and years; the app stores centimetres and kilograms
// (ADR 0032 step 5), converted here once. The rows are hand-checked against
// the rulebook's tables; identity_test.go holds every figure again, written
// out, so a slip in this file fails there.

// The sex codes of a Table 10 row (the same as charactersvc.Sexes).
const (
	sexMale   = "male"
	sexFemale = "female"
)

// The conversions of ADR 0032 step 5, exact by definition.
const (
	CentimetresPerInch = 2.54
	KilogramsPerPound  = 0.45359237
)

// StoredStep is the precision of a stored height (cm) or weight (kg): a
// tenth. A range is rounded outward to it, so a rolled value never warns.
const StoredStep = 0.1

// Dice is count dice of sides sides ("2d6").
type Dice struct{ Count, Sides int }

// Min is the lowest total of the dice (every die 1), Max the highest.
func (d Dice) Min() int { return d.Count }

// Max is the highest total of the dice (every die at its maximum).
func (d Dice) Max() int { return d.Count * d.Sides }

// IdentityRow is one race's row of Tables 10 and 11. Base heights are in
// inches and base weights in pounds, male then female; ages in years.
type IdentityRow struct {
	Race         string
	HeightMale   int
	HeightFemale int
	HeightDice   Dice
	WeightMale   int
	WeightFemale int
	WeightDice   Dice
	StartAge     int
	StartAgeDice Dice
	MaxAge       int
	MaxAgeDice   Dice
}

// identityRows is Tables 10 and 11, in the book's race order.
var identityRows = []IdentityRow{
	{"dwarf", 43, 41, Dice{1, 10}, 130, 105, Dice{4, 10}, 40, Dice{5, 6}, 250, Dice{2, 100}},
	{"elf", 55, 50, Dice{1, 10}, 90, 70, Dice{3, 10}, 100, Dice{5, 6}, 350, Dice{4, 100}},
	{"gnome", 38, 36, Dice{1, 6}, 72, 68, Dice{5, 4}, 60, Dice{3, 12}, 200, Dice{3, 100}},
	{"half_elf", 60, 58, Dice{2, 6}, 110, 85, Dice{3, 12}, 15, Dice{1, 6}, 125, Dice{3, 20}},
	{"halfling", 32, 30, Dice{2, 8}, 52, 48, Dice{5, 4}, 20, Dice{3, 4}, 100, Dice{1, 100}},
	{"human", 60, 59, Dice{2, 10}, 140, 100, Dice{6, 10}, 15, Dice{1, 4}, 90, Dice{2, 20}},
}

// IdentityRows are the rows of Tables 10 and 11.
func IdentityRows() []IdentityRow { return slices.Clone(identityRows) }

// IdentityFor is the race's row, false for a race that is none of the six.
func IdentityFor(race string) (IdentityRow, bool) {
	for _, r := range identityRows {
		if r.Race == race {
			return r, true
		}
	}
	return IdentityRow{}, false
}

// BaseHeight is the Table 10 base height in inches for the sex, false for a
// sex that is none of the two.
func (r IdentityRow) BaseHeight(sex string) (int, bool) {
	switch sex {
	case sexMale:
		return r.HeightMale, true
	case sexFemale:
		return r.HeightFemale, true
	}
	return 0, false
}

// BaseWeight is the Table 10 base weight in pounds for the sex.
func (r IdentityRow) BaseWeight(sex string) (int, bool) {
	switch sex {
	case sexMale:
		return r.WeightMale, true
	case sexFemale:
		return r.WeightFemale, true
	}
	return 0, false
}

// Range is the lowest and highest value of a figure, in its stored unit.
type Range struct{ Min, Max float64 }

// InchesToCm is inches in centimetres, rounded to the stored step.
func InchesToCm(in float64) float64 { return roundStep(in * CentimetresPerInch) }

// PoundsToKg is pounds in kilograms, rounded to the stored step.
func PoundsToKg(lb float64) float64 { return roundStep(lb * KilogramsPerPound) }

// roundStep rounds to the nearest StoredStep.
func roundStep(v float64) float64 { return math.Round(v*10) / 10 }

// floorStep and ceilStep round outward to the stored step, ignoring float
// noise below a thousandth.
func floorStep(v float64) float64 { return math.Floor(clean(v)*10) / 10 }
func ceilStep(v float64) float64  { return math.Ceil(clean(v)*10) / 10 }
func clean(v float64) float64     { return math.Round(v*1000) / 1000 }

// sexesOf are the sexes a range covers: the one given, or both when it is
// empty (a migrated character without a sex, ADR 0032 step 5.1).
func sexesOf(sex string) []string {
	if sex == sexMale || sex == sexFemale {
		return []string{sex}
	}
	return []string{sexMale, sexFemale}
}

// HeightRange is the stored range (cm) of a height: the Table 10 base for the
// sex plus the modifier dice at their lowest and highest, converted and
// rounded outward (ADR 0032 step 5.1). An empty sex is the union of the two;
// ok is false for an unknown race.
func HeightRange(race, sex string) (Range, bool) {
	row, ok := IdentityFor(race)
	if !ok {
		return Range{}, false
	}
	out := Range{Min: math.Inf(1), Max: math.Inf(-1)}
	for _, s := range sexesOf(sex) {
		base, _ := row.BaseHeight(s)
		out.Min = math.Min(out.Min, floorStep(float64(base+row.HeightDice.Min())*CentimetresPerInch))
		out.Max = math.Max(out.Max, ceilStep(float64(base+row.HeightDice.Max())*CentimetresPerInch))
	}
	return out, true
}

// WeightRange is the stored range (kg) of a weight, as HeightRange.
func WeightRange(race, sex string) (Range, bool) {
	row, ok := IdentityFor(race)
	if !ok {
		return Range{}, false
	}
	out := Range{Min: math.Inf(1), Max: math.Inf(-1)}
	for _, s := range sexesOf(sex) {
		base, _ := row.BaseWeight(s)
		out.Min = math.Min(out.Min, floorStep(float64(base+row.WeightDice.Min())*KilogramsPerPound))
		out.Max = math.Max(out.Max, ceilStep(float64(base+row.WeightDice.Max())*KilogramsPerPound))
	}
	return out, true
}

// AgeRange is the range of an age in years: from the lowest starting age
// (Table 11's base plus the variable dice at 1) to the highest maximum age
// (the maximum age range's base plus its dice at their maximum), so a young
// character is never warned for being young (ADR 0032 step 5.1).
func AgeRange(race string) (Range, bool) {
	row, ok := IdentityFor(race)
	if !ok {
		return Range{}, false
	}
	return Range{
		Min: float64(row.StartAge + row.StartAgeDice.Min()),
		Max: float64(row.MaxAge + row.MaxAgeDice.Max()),
	}, true
}
