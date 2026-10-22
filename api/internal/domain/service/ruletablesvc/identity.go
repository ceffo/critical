package ruletablesvc

import (
	"math"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// PHB Table 10 (average height and weight) and Table 11 (age), the data of
// ADR 0032 step 5: per race the base height and weight for each sex and the
// modifier dice, and the starting age's base and variable dice, read from
// the race definitions (ADR 0040 step 2). The book gives inches, pounds and
// years; the app stores centimetres and kilograms (ADR 0032 step 5),
// converted here once. identity_test.go holds every figure again, written
// out from the rulebook's tables, so a slip in the catalogue fails there.

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
type Dice = classes.Dice

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

// IdentityRows are the rows of Tables 10 and 11 of the race definitions (ADR
// 0040 step 2), in catalogue order.
func IdentityRows(defs *classes.Registry) []IdentityRow {
	var out []IdentityRow
	for _, r := range defs.Races() {
		out = append(out, identityRow(r))
	}
	return out
}

// IdentityFor is the race's row, false for a race the catalogue does not
// define.
func IdentityFor(defs *classes.Registry, race string) (IdentityRow, bool) {
	r, ok := defs.Race(race)
	if !ok {
		return IdentityRow{}, false
	}
	return identityRow(r), true
}

func identityRow(r classes.Race) IdentityRow {
	i := r.Identity
	return IdentityRow{
		Race:       r.ID,
		HeightMale: i.HeightMale, HeightFemale: i.HeightFemale, HeightDice: i.HeightDice,
		WeightMale: i.WeightMale, WeightFemale: i.WeightFemale, WeightDice: i.WeightDice,
		StartAge: i.StartAge, StartAgeDice: i.StartAgeDice, MaxAge: i.MaxAge, MaxAgeDice: i.MaxAgeDice,
	}
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
func (row IdentityRow) HeightRange(sex string) Range {
	out := Range{Min: math.Inf(1), Max: math.Inf(-1)}
	for _, s := range sexesOf(sex) {
		base, _ := row.BaseHeight(s)
		out.Min = math.Min(out.Min, floorStep(float64(base+row.HeightDice.Min())*CentimetresPerInch))
		out.Max = math.Max(out.Max, ceilStep(float64(base+row.HeightDice.Max())*CentimetresPerInch))
	}
	return out
}

// WeightRange is the stored range (kg) of a weight, as HeightRange.
func (row IdentityRow) WeightRange(sex string) Range {
	out := Range{Min: math.Inf(1), Max: math.Inf(-1)}
	for _, s := range sexesOf(sex) {
		base, _ := row.BaseWeight(s)
		out.Min = math.Min(out.Min, floorStep(float64(base+row.WeightDice.Min())*KilogramsPerPound))
		out.Max = math.Max(out.Max, ceilStep(float64(base+row.WeightDice.Max())*KilogramsPerPound))
	}
	return out
}

// AgeRange is the range of an age in years: from the lowest starting age
// (Table 11's base plus the variable dice at 1) to the highest maximum age
// (the maximum age range's base plus its dice at their maximum), so a young
// character is never warned for being young (ADR 0032 step 5.1).
func (row IdentityRow) AgeRange() Range {
	return Range{
		Min: float64(row.StartAge + row.StartAgeDice.Min()),
		Max: float64(row.MaxAge + row.MaxAgeDice.Max()),
	}
}

// StartAgeRange is the range a starting age roll gives (Table 11's base plus
// the variable dice at their lowest and highest), inside AgeRange.
func (row IdentityRow) StartAgeRange() Range {
	return Range{
		Min: float64(row.StartAge + row.StartAgeDice.Min()),
		Max: float64(row.StartAge + row.StartAgeDice.Max()),
	}
}
