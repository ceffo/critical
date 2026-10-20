package charactersvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

func f64(v float64) *float64 { return &v }

// ADR 0032 step 5.2: a height, weight or age outside the table range of the
// race (and, for the first two, the sex) is a warning with the value and the
// range, one inside has none; a number not set is not checked. A half-elf
// woman: height 152.4 to 177.8 cm, weight 39.9 to 54.9 kg, age 16 to 185.
func TestRangeWarnings_Step5_2_OutsideTheRangeWarnsInsideDoesNot(t *testing.T) {
	d := func(h, w float64, age int) charactersvc.Details {
		return charactersvc.Details{Sex: "female", Height: h, Weight: w, Age: age}
	}
	for name, tc := range map[string]struct {
		d    charactersvc.Details
		want []string
	}{
		"inside, the bounds included": {d(152.4, 54.9, 16), nil},
		"inside, the other bound":     {d(177.8, 39.9, 185), nil},
		"too tall":                    {d(177.9, 45, 30), []string{charactersvc.CodeHeightOutsideRange}},
		"too short":                   {d(152.3, 45, 30), []string{charactersvc.CodeHeightOutsideRange}},
		"too heavy":                   {d(165, 55, 30), []string{charactersvc.CodeWeightOutsideRange}},
		"too light":                   {d(165, 39.8, 30), []string{charactersvc.CodeWeightOutsideRange}},
		"too old":                     {d(165, 45, 186), []string{charactersvc.CodeAgeOutsideRange}},
		"too young":                   {d(165, 45, 15), []string{charactersvc.CodeAgeOutsideRange}},
		"all three, in the card's order": {d(200, 100, 500), []string{
			charactersvc.CodeHeightOutsideRange, charactersvc.CodeWeightOutsideRange, charactersvc.CodeAgeOutsideRange,
		}},
		"none set":                         {charactersvc.Details{Sex: "female"}, nil},
		"only the age set, outside":        {d(0, 0, 500), []string{charactersvc.CodeAgeOutsideRange}},
		"the male row is not the female's": {charactersvc.Details{Sex: "male", Height: 152.4, Weight: 55, Age: 30}, []string{charactersvc.CodeHeightOutsideRange}},
	} {
		t.Run(name, func(t *testing.T) {
			var got []string
			for _, w := range tc.d.RangeWarnings("half_elf") {
				got = append(got, w.Code)
			}
			assert.Equal(t, tc.want, got)
		})
	}

	ws := d(200, 45, 30).RangeWarnings("half_elf")
	require.Len(t, ws, 1)
	assert.Equal(t, charactersvc.FieldHeight, ws[0].Field)
	assert.Equal(t, 200.0, *ws[0].Value)
	assert.Equal(t, 152.4, *ws[0].Min)
	assert.Equal(t, 177.8, *ws[0].Max)
}

// ADR 0032 step 5.1: with no sex (a migrated character) the checks use the
// union of the two rows, with no race they check nothing.
func TestRangeWarnings_Step5_1_NoSexIsTheUnionNoRaceIsNothing(t *testing.T) {
	// 150 cm is a half-elf woman's too short but a halfling's fine; for a human with no sex the
	// union runs from the woman's 154.9 to the man's 203.2.
	assert.Empty(t, charactersvc.Details{Height: 203.2}.RangeWarnings("human"))
	assert.Empty(t, charactersvc.Details{Height: 154.9}.RangeWarnings("human"))
	assert.Len(t, charactersvc.Details{Height: 154.8}.RangeWarnings("human"), 1)
	assert.Len(t, charactersvc.Details{Height: 203.3}.RangeWarnings("human"), 1)
	assert.Empty(t, charactersvc.Details{Height: 5000, Weight: 1, Age: 9999}.RangeWarnings(""), "no race, no range")
	assert.Empty(t, charactersvc.Details{Sex: "garbage", Height: 160}.RangeWarnings("human"), "a non-code sex reads as missing")
}

// ADR 0032 step 5.2: the refusals are only about being a number: a height or
// weight above 0 and up to the sanity bound (kept to a tenth), an age 1 to
// 10,000; the range of the race refuses nothing. Null clears.
func TestApply_Step5_2_OnlyANonsenseNumberIsRefused(t *testing.T) {
	k := charactersvc.Kind{Class: "fighter", Race: "halfling"}
	_, err := charactersvc.Details{}.Apply(k, charactersvc.DetailsPatch{Height: f64(250.04), Weight: f64(400), Age: new(int)})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe, "age 0 is not a clear")
	assert.Equal(t, "age", fe.Field)

	age := 900
	got, err := charactersvc.Details{}.Apply(k, charactersvc.DetailsPatch{Height: f64(250.04), Weight: f64(400), Age: &age})
	require.NoError(t, err, "a 2.5 m, 400 kg, 900 years halfling is the DM's to allow")
	assert.Equal(t, 250.0, got.Height, "kept to a tenth")
	assert.Equal(t, 400.0, got.Weight)
	assert.Equal(t, 900, got.Age)

	for name, p := range map[string]charactersvc.DetailsPatch{
		"zero height": {Height: f64(0)},
		"tiny height": {Height: f64(0.04)},
		"huge height": {Height: f64(300.1)},
		"zero weight": {Weight: f64(0)},
		"huge weight": {Weight: f64(500.1)},
		"negative":    {Weight: f64(-3)},
	} {
		_, err := charactersvc.Details{}.Apply(k, p)
		require.ErrorAs(t, err, &fe, name)
		assert.Equal(t, charactersvc.CodeOutOfRange, fe.Message, name)
	}

	cleared, err := got.Apply(k, charactersvc.DetailsPatch{ClearHeight: true, ClearWeight: true})
	require.NoError(t, err)
	assert.Zero(t, cleared.Height)
	assert.Zero(t, cleared.Weight)
	assert.Equal(t, 900, cleared.Age, "a clear touches only its own field")
}

// ADR 0032 step 5.2 on a read: GET of a character carries the range warnings
// of its stored numbers, read again at every read.
func TestGetCharacter_Step5_2_TheReadWarnsOutsideTheRange(t *testing.T) {
	c := kelsen() // a half-elf bard
	c.Details = charactersvc.Details{Alignment: "neutral_good", Sex: "male", Height: 230, Weight: 60, Age: 40}
	e := newEnv(t)
	e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(c, nil).Once()
	e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
	expectKelsenRules(e.rules, 1_293_418, 15)

	got, err := e.svc.GetCharacter(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, got.Warnings, 1)
	assert.Equal(t, charactersvc.CodeHeightOutsideRange, got.Warnings[0].Code)
	assert.Equal(t, 230.0, *got.Warnings[0].Value)
	assert.Equal(t, 157.4, *got.Warnings[0].Min)
	assert.Equal(t, 182.9, *got.Warnings[0].Max)
}
