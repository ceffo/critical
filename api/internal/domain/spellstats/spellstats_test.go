package spellstats_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// All inputs are literal PHB strings written here; the seed-wide check is in seedcheck.

func TestCastingTime(t *testing.T) {
	tests := []struct {
		raw   string
		level int
		kind  string
		want  spellstats.Value
	}{
		{"3", 1, "timed", spellstats.Value{Amount: 3, Unit: "segment"}},
		{"1 rd.", 1, "timed", spellstats.Value{Amount: 1, Unit: "round"}},
		{"1rd.", 1, "timed", spellstats.Value{Amount: 1, Unit: "round"}},
		{"1 rd", 1, "timed", spellstats.Value{Amount: 1, Unit: "round"}},
		{"3 rounds", 1, "timed", spellstats.Value{Amount: 3, Unit: "round"}},
		{"1 turn", 1, "timed", spellstats.Value{Amount: 1, Unit: "turn"}},
		{"4 turns", 1, "timed", spellstats.Value{Amount: 4, Unit: "turn"}},
		{"1 hr.", 1, "timed", spellstats.Value{Amount: 1, Unit: "hour"}},
		{"1 hour", 1, "timed", spellstats.Value{Amount: 1, Unit: "hour"}},
		{"2 hrs.", 1, "timed", spellstats.Value{Amount: 2, Unit: "hour"}},
		{"1/2 hour", 1, "timed", spellstats.Value{Amount: 0.5, Unit: "hour"}},
		{"1 day", 1, "timed", spellstats.Value{Amount: 1, Unit: "day"}},
		{"2d12 hours", 1, "timed", spellstats.Value{Dice: "2d12", Unit: "hour"}},
		{"3-4", 1, "timed", spellstats.Value{Amount: 3, Upper: 4, Unit: "segment"}},
		{"1 to 4 days", 1, "timed", spellstats.Value{Amount: 1, Upper: 4, Unit: "day"}},
		{"1 round/recipient", 1, "timed", spellstats.Value{Amount: 1, Unit: "round"}},
		{"1 rd./seed", 1, "timed", spellstats.Value{Amount: 1, Unit: "round"}},
		{"Special", 1, "special", spellstats.Value{}},
		{"Special + 1", 1, "special", spellstats.Value{}},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := spellstats.ParseCastingTime(tt.raw)
			assert.Equal(t, tt.raw, got.Raw)
			assert.Equal(t, tt.kind, got.Kind)
			assert.Equal(t, tt.want, got.At(tt.level))
		})
	}
}

func TestRange(t *testing.T) {
	tests := []struct {
		raw    string
		level  int
		kind   string
		radius bool
		want   spellstats.Value
	}{
		{"60 yds. + 10 yds./level", 15, "distance", false, spellstats.Value{Amount: 210, Unit: "yard"}},
		{"60 yds. + 10 yds./level", 1, "distance", false, spellstats.Value{Amount: 70, Unit: "yard"}},
		{"10 yds. + 10 yds./level", 5, "distance", false, spellstats.Value{Amount: 60, Unit: "yard"}},
		{"90 yds. + 10 yd./level", 5, "distance", false, spellstats.Value{Amount: 140, Unit: "yard"}},
		{"30 yds.", 15, "distance", false, spellstats.Value{Amount: 30, Unit: "yard"}},
		{"5 yds./level", 15, "distance", false, spellstats.Value{Amount: 75, Unit: "yard"}},
		{"10 yds./level .", 7, "distance", false, spellstats.Value{Amount: 70, Unit: "yard"}},
		{"10 yds./2 levels", 15, "distance", false, spellstats.Value{Amount: 70, Unit: "yard"}},
		{"10 yds./level, max. 100 yds.", 15, "distance", false, spellstats.Value{Amount: 100, Unit: "yard"}},
		{"10 yds./level, max. 100 yds.", 4, "distance", false, spellstats.Value{Amount: 40, Unit: "yard"}},
		{"10 ft.", 15, "distance", false, spellstats.Value{Amount: 10, Unit: "foot"}},
		{"1 mile/level", 15, "distance", false, spellstats.Value{Amount: 15, Unit: "mile"}},
		{"1 mi./level", 3, "distance", false, spellstats.Value{Amount: 3, Unit: "mile"}},
		{"1 mi. radius", 3, "distance", true, spellstats.Value{Amount: 1, Unit: "mile"}},
		{"10-ft. radius", 3, "distance", true, spellstats.Value{Amount: 10, Unit: "foot"}},
		{"Touch", 3, "touch", false, spellstats.Value{}},
		{"Touch.", 3, "touch", false, spellstats.Value{}},
		{"0", 3, "self", false, spellstats.Value{}},
		{"Unlimited", 3, "unlimited", false, spellstats.Value{}},
		{"Special", 3, "special", false, spellstats.Value{}},
		{"Infinite + special", 3, "special", false, spellstats.Value{}},
		{"1", 3, "special", false, spellstats.Value{}},
		{"1\"", 3, "distance", false, spellstats.Value{Amount: 10, Unit: "yard"}},
		{"3\"/level", 2, "distance", false, spellstats.Value{Amount: 60, Unit: "yard"}},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := spellstats.ParseRange(tt.raw)
			assert.Equal(t, tt.raw, got.Raw)
			assert.Equal(t, tt.kind, got.Kind)
			assert.Equal(t, tt.radius, got.Radius)
			assert.Equal(t, tt.want, got.At(tt.level))
		})
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		raw   string
		level int
		kind  string
		cond  string
		want  spellstats.Value
	}{
		{"3 rds./level", 15, "timed", "", spellstats.Value{Amount: 45, Unit: "round"}},
		{"1 rd./level", 15, "timed", "", spellstats.Value{Amount: 15, Unit: "round"}},
		{"1 round/level", 15, "timed", "", spellstats.Value{Amount: 15, Unit: "round"}},
		{"3 rds. + 1 rd./level", 15, "timed", "", spellstats.Value{Amount: 18, Unit: "round"}},
		{"4 rds. + 1 rd./2 levels", 15, "timed", "", spellstats.Value{Amount: 11, Unit: "round"}},
		{"1 round/3 levels", 15, "timed", "", spellstats.Value{Amount: 5, Unit: "round"}},
		{"1 usage/2 levels", 15, "timed", "", spellstats.Value{Amount: 7, Unit: "usage"}},
		{"1 turn/level", 4, "timed", "", spellstats.Value{Amount: 4, Unit: "turn"}},
		{"2 turns + 1 turn/level", 4, "timed", "", spellstats.Value{Amount: 6, Unit: "turn"}},
		{"1 hr./level", 15, "timed", "", spellstats.Value{Amount: 15, Unit: "hour"}},
		{"4 hrs. + 1/2 hr./level", 15, "timed", "", spellstats.Value{Amount: 11.5, Unit: "hour"}},
		{"1 1/2 hrs./level", 4, "timed", "", spellstats.Value{Amount: 6, Unit: "hour"}},
		{"1 day/level", 3, "timed", "", spellstats.Value{Amount: 3, Unit: "day"}},
		{"2 hrs.", 15, "timed", "", spellstats.Value{Amount: 2, Unit: "hour"}},
		{"1 rd", 15, "timed", "", spellstats.Value{Amount: 1, Unit: "round"}},
		{"1d4 rds. + 1 rd./level", 15, "timed", "", spellstats.Value{Amount: 15, Dice: "1d4", Unit: "round"}},
		{"3d4 turns + 1/level", 15, "timed", "", spellstats.Value{Amount: 15, Dice: "3d4", Unit: "turn"}},
		{"3d4 turns +1/level", 15, "timed", "", spellstats.Value{Amount: 15, Dice: "3d4", Unit: "turn"}},
		{"1 rd. + 1/level", 15, "timed", "", spellstats.Value{Amount: 16, Unit: "round"}},
		{"1d4+1 hrs. + 1 hr./level", 15, "timed", "", spellstats.Value{Amount: 16, Dice: "1d4", Unit: "hour"}},
		{"1+1d4 rds.", 15, "timed", "", spellstats.Value{Amount: 1, Dice: "1d4", Unit: "round"}},
		{"2 rds. + 1d4 rds.", 15, "timed", "", spellstats.Value{Amount: 2, Dice: "1d4", Unit: "round"}},
		{"1 hr./level + 1d4 hrs.", 15, "timed", "", spellstats.Value{Amount: 15, Dice: "1d4", Unit: "hour"}},
		{"1 turn/level + 1d6 turns", 3, "timed", "", spellstats.Value{Amount: 3, Dice: "1d6", Unit: "turn"}},
		{"1 turn + 1 rd./level", 15, "timed", "", spellstats.Value{Amount: 25, Unit: "round"}},
		{"1 hour + 1 turn/level", 6, "timed", "", spellstats.Value{Amount: 12, Unit: "turn"}},
		{"1 week + 1 day/level", 3, "timed", "", spellstats.Value{Amount: 10, Unit: "day"}},
		{"up to 4 hrs.", 15, "timed", "", spellstats.Value{Amount: 4, Unit: "hour"}},
		{"Up to 2 rds./level", 5, "timed", "", spellstats.Value{Amount: 10, Unit: "round"}},
		{"1 hour/lvl", 6, "timed", "", spellstats.Value{Amount: 6, Unit: "hour"}},
		{"1 turn/lvl", 6, "timed", "", spellstats.Value{Amount: 6, Unit: "turn"}},
		{"1rd/lvl", 6, "timed", "", spellstats.Value{Amount: 6, Unit: "round"}},
		{"1 turn + 2 rds/lvl", 6, "timed", "", spellstats.Value{Amount: 22, Unit: "round"}},
		{"Instantaneous", 15, "instantaneous", "", spellstats.Value{}},
		{"Permanent", 15, "permanent", "", spellstats.Value{}},
		{"Permanent until dispelled", 15, "permanent", "dispelled", spellstats.Value{}},
		{"Permanent until discharged", 15, "permanent", "discharged", spellstats.Value{}},
		{"Until dispelled", 15, "until", "dispelled", spellstats.Value{}},
		{"Until triggered", 15, "until", "triggered", spellstats.Value{}},
		{"One touch", 15, "until", "one touch", spellstats.Value{}},
		{"Concentration", 15, "concentration", "", spellstats.Value{}},
		{"Special", 15, "special", "", spellstats.Value{}},
		{"Variable", 15, "special", "", spellstats.Value{}},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := spellstats.ParseDuration(tt.raw)
			assert.Equal(t, tt.raw, got.Raw)
			assert.Equal(t, tt.kind, got.Kind)
			assert.Equal(t, tt.cond, got.Condition)
			assert.Equal(t, tt.want, got.At(tt.level))
		})
	}
}

func TestSave(t *testing.T) {
	tests := []struct {
		raw  string
		want []string
	}{
		{"None", []string{"none"}},
		{"Neg.", []string{"negates"}},
		{"1/2", []string{"half"}},
		{"Special", []string{"special"}},
		{"Neg. or 1/2", []string{"negates", "half"}},
		{"None or 1/2", []string{"none", "half"}},
		{"None (special)", []string{"none", "special"}},
		{"_", []string{"special"}},
		{"", []string{"special"}},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := spellstats.ParseSave(tt.raw)
			assert.Equal(t, tt.raw, got.Raw)
			assert.Equal(t, tt.want, got.Kinds)
		})
	}
}

func TestComponents(t *testing.T) {
	tests := []struct {
		raw  string
		want spellstats.Components
	}{
		{"V, S, M", spellstats.Components{V: true, S: true, M: true}},
		{"V, S", spellstats.Components{V: true, S: true}},
		{"V", spellstats.Components{V: true}},
		{"S", spellstats.Components{S: true}},
		{"V, M", spellstats.Components{V: true, M: true}},
		{"S, M", spellstats.Components{S: true, M: true}},
		{"V, S (M optional)", spellstats.Components{V: true, S: true, M: true, MOptional: true}},
		{"V, S, special", spellstats.Components{V: true, S: true, Special: true}},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			tt.want.Raw = tt.raw
			assert.Equal(t, tt.want, spellstats.ParseComponents(tt.raw))
		})
	}
}

func TestAreaOfEffect(t *testing.T) {
	dim := func(unit string, amount float64) spellstats.Value {
		return spellstats.Value{Amount: amount, Unit: unit}
	}
	tests := []struct {
		raw     string
		level   int
		kind    string
		shape   string
		dims    []spellstats.Value
		targets *spellstats.Value
		noun    string
	}{
		{"20-ft. radius", 15, "area", "radius", []spellstats.Value{dim("foot", 20)}, nil, ""},
		{"20-ft.-radius", 15, "area", "radius", []spellstats.Value{dim("foot", 20)}, nil, ""},
		{"20-ft.-radius globe", 15, "area", "radius", []spellstats.Value{dim("foot", 20)}, nil, ""},
		{"10 yd. radius", 15, "area", "radius", []spellstats.Value{dim("yard", 10)}, nil, ""},
		{"Up to 20-ft. cube", 15, "area", "cube", []spellstats.Value{dim("foot", 20)}, nil, ""},
		{"10-ft. cube/level", 5, "area", "cube", []spellstats.Value{dim("foot", 50)}, nil, ""},
		{"10-ft. cube + 10-ft. cube/level", 5, "area", "cube", []spellstats.Value{dim("foot", 60)}, nil, ""},
		{"10 ft. + 5 ft./level", 4, "area", "", []spellstats.Value{dim("foot", 30)}, nil, ""},
		{"10 x 60 ft.", 15, "area", "", []spellstats.Value{dim("foot", 10), dim("foot", 60)}, nil, ""},
		{"10 x 20 x 30 ft.", 15, "area", "", []spellstats.Value{dim("foot", 10), dim("foot", 20), dim("foot", 30)}, nil, ""},
		{"10 ft./level x 10 ft.", 3, "area", "", []spellstats.Value{dim("foot", 30), dim("foot", 10)}, nil, ""},
		{"10 ft. x 5 yds.", 3, "area", "", []spellstats.Value{dim("foot", 10), dim("yard", 5)}, nil, ""},
		{"10 sq. ft./level", 3, "area", "square", []spellstats.Value{dim("sq foot", 30)}, nil, ""},
		{"10 cu. ft./level", 3, "area", "", []spellstats.Value{dim("cu foot", 30)}, nil, ""},
		{"1 cu. yd.", 3, "area", "", []spellstats.Value{dim("cu yard", 1)}, nil, ""},
		{"40-ft. cube, 1 creature/level", 15, "area", "cube", []spellstats.Value{dim("foot", 40)}, &spellstats.Value{Amount: 15}, "creature"},
		{"2-8 creatures in a 10-ft. cube", 15, "area", "cube", []spellstats.Value{dim("foot", 10)}, &spellstats.Value{Amount: 2, Upper: 8}, "creatures"},
		{"2d4 creatures in 10-ft. radius", 15, "area", "radius", []spellstats.Value{dim("foot", 10)}, &spellstats.Value{Dice: "2d4"}, "creatures"},
		{"(level x 10)-ft.-sq.", 5, "area", "square", []spellstats.Value{dim("foot", 50)}, nil, ""},
		{"1 creature", 15, "targets", "", nil, &spellstats.Value{Amount: 1}, "creature"},
		{"One creature", 15, "targets", "", nil, &spellstats.Value{Amount: 1}, "creature"},
		{"1-5 targets", 15, "targets", "", nil, &spellstats.Value{Amount: 1, Upper: 5}, "targets"},
		{"1 item/level", 15, "targets", "", nil, &spellstats.Value{Amount: 15}, "item"},
		{"1 creature/3 levels", 15, "targets", "", nil, &spellstats.Value{Amount: 5}, "creature"},
		{"1 oak club", 15, "targets", "", nil, &spellstats.Value{Amount: 1}, "oak club"},
		{"The caster", 15, "self", "", nil, nil, ""},
		{"The caster's hand", 15, "self", "", nil, nil, ""},
		{"Creature touched", 15, "touch", "", nil, nil, ""},
		{"Person touched", 15, "touch", "", nil, nil, ""},
		{"Special", 15, "special", "", nil, nil, ""},
		{"Line of sight", 15, "special", "", nil, nil, ""},
		{"1–7 targets", 15, "targets", "", nil, &spellstats.Value{Amount: 1, Upper: 7}, "targets"},
		{"1d6 creatures/level", 3, "targets", "", nil, &spellstats.Value{Dice: "3d6"}, "creatures"},
		{"1d6 creatures/2 levels", 5, "targets", "", nil, &spellstats.Value{Dice: "2d6"}, "creatures"},
		{"30' long + 5'/level", 4, "area", "", []spellstats.Value{dim("foot", 50)}, nil, ""},
		{"3\" diameter circle", 15, "area", "diameter", []spellstats.Value{dim("yard", 30)}, nil, ""},
		{"One 10'-wide path, 60 feet long", 15, "area", "", []spellstats.Value{dim("foot", 10), dim("foot", 60)}, nil, ""},
		{"Creates 1d4+1", 15, "targets", "", nil, &spellstats.Value{Amount: 1, Dice: "1d4"}, ""},
		{"Create 7 eyes", 15, "targets", "", nil, &spellstats.Value{Amount: 7}, "eyes"},
		{"Creates a sphere", 15, "special", "", nil, nil, ""},
		{"Two or more creatures", 15, "special", "", nil, nil, ""},
		{"1d4 sticks + 1 stick/level in a 10-ft. cube", 5, "area", "cube", []spellstats.Value{dim("foot", 10)}, nil, ""},
		{"10-foot circle", 15, "area", "circle", []spellstats.Value{dim("foot", 10)}, nil, ""},
		{"20-foot-radius circle", 15, "area", "radius", []spellstats.Value{dim("foot", 20)}, nil, ""},
		{"40-foot-diameter circle", 15, "area", "diameter", []spellstats.Value{dim("foot", 40)}, nil, ""},
		{"One 10'-radius circle", 15, "area", "radius", []spellstats.Value{dim("foot", 10)}, nil, ""},
		{"10-ft. cube/2 levels", 5, "area", "cube", []spellstats.Value{dim("foot", 20)}, nil, ""},
		{"One 10-ft. cube/lvl.", 5, "area", "cube", []spellstats.Value{dim("foot", 50)}, nil, ""},
		{"1,000 square feet/level", 3, "area", "square", []spellstats.Value{dim("sq foot", 3000)}, nil, ""},
		{"One creature/two levels", 10, "targets", "", nil, &spellstats.Value{Amount: 5}, "creature"},
		{"One unit/five levels", 10, "targets", "", nil, &spellstats.Value{Amount: 2}, "unit"},
		{"Up to 6 creatures", 15, "targets", "", nil, &spellstats.Value{Amount: 6}, "creatures"},
		{"Up to 8 animals in a 20-ft. sq.", 15, "area", "square", []spellstats.Value{dim("foot", 20)}, &spellstats.Value{Amount: 8}, "animals"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := spellstats.ParseArea(tt.raw)
			assert.Equal(t, tt.raw, got.Raw)
			assert.Equal(t, tt.kind, got.Kind)
			assert.Equal(t, tt.shape, got.Shape)
			assert.Equal(t, tt.noun, got.Noun)
			// What the parser read, evaluated; a partial reading serves no value
			// (TestAreaOfEffect_PartialReadingHasNoValueAtACasterLevel).
			read := got
			read.Partial = false
			at := read.At(tt.level)
			assert.Equal(t, tt.dims, at.Dims)
			assert.Equal(t, tt.targets, at.Targets)
		})
	}
}

// A "Special (...)" text stays special but keeps the number the book gives.
func TestSpecialKeepsItsNumber(t *testing.T) {
	ct := spellstats.ParseCastingTime("Special (8 turns)")
	assert.Equal(t, "special", ct.Kind)
	assert.Equal(t, spellstats.Value{}, ct.At(10))
	assert.Equal(t, []spellstats.Term{{Count: 8, Unit: "turn"}}, ct.Time.Terms)

	tests := []struct {
		raw   string
		terms []spellstats.Term
		upTo  bool
	}{
		{"Special (up to three rounds)", []spellstats.Term{{Count: 3, Unit: "round"}}, true},
		{"Special (2 rounds/level maximum)", []spellstats.Term{{Count: 2, Unit: "round", PerLevel: true, Step: 1}}, true},
		{"Special; max. 10 rounds", []spellstats.Term{{Count: 10, Unit: "round"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			d := spellstats.ParseDuration(tt.raw)
			assert.Equal(t, "special", d.Kind)
			assert.Equal(t, tt.terms, d.Time.Terms)
			assert.Equal(t, tt.upTo, d.Time.UpTo)
		})
	}
	assert.Empty(t, spellstats.ParseDuration("Variable").Time.Terms)
	assert.Empty(t, spellstats.ParseCastingTime("Special + 1").Time.Terms)
}

func TestParse_PicksTheAcceptanceExamples(t *testing.T) {
	mm := spellstats.Parse(spellstats.Raw{
		CastingTime: "1", Range: "60 yds. + 10 yds./level", Duration: "Instantaneous",
		AreaOfEffect: "1-5 targets", Components: "V, S", SavingThrow: "None",
	})
	assert.Equal(t, 210.0, mm.At(15).Range.Amount)
	assert.Equal(t, "210 yard", mm.At(15).Range.String())

	mi := spellstats.Parse(spellstats.Raw{Duration: "3 rds./level", SavingThrow: "None"})
	assert.Equal(t, "45 round", mi.At(15).Duration.String())

	fb := spellstats.Parse(spellstats.Raw{SavingThrow: "1/2"})
	assert.Equal(t, []string{"half"}, fb.Save.Kinds)
}

func TestSpecial_ListsFallbackFields(t *testing.T) {
	got := spellstats.Parse(spellstats.Raw{
		CastingTime: "Special", Range: "Touch", Duration: "Variable",
		AreaOfEffect: "Line of sight", Components: "V, S, special", SavingThrow: "Special",
	}).Special()
	assert.Equal(t, []string{"casting_time", "duration", "area_of_effect", "components", "saving_throw"}, got)
}

// ADR 0021 step 8: an area or targets reading is partial when the text says
// more than the typed form holds, so a reader shows the raw text; a text the
// form covers whole (numbers, dice, units, shape, noun, "x", "in a",
// "level") is not. Other kinds are never partial.
func TestAreaOfEffect_PartialWhenTheTextSaysMore(t *testing.T) {
	tests := []struct {
		raw     string
		partial bool
	}{
		{"20-ft. radius", false},
		{"20-ft. radius sphere", false}, // the figure is read
		{"20-ft. cube sphere", true},    // a second shape it does not carry
		{"10 x 60 ft.", false},
		{"40-ft. cube, 1 creature/level", false},
		{"2-8 creatures in a 10-ft. cube", false},
		{"2d4 creatures in 10-ft. radius", false},
		{"1 creature/3 levels", false},
		{"One creature", false},
		{"Two creatures", false},
		{"1 to 6 insects", false},
		{"(level x 10)-ft.-sq.", false},
		{"10 sq. ft./level", false},
		{"One creature/two levels", false},
		{"10 ft. cube/2 levels", false},
		{"1,000 square feet/level", false},
		{"Up to 6 creatures", false},
		{"20-foot-radius circle", false},
		{"One 10-ft. cube/level", false},                        // "One" is the article of an extent
		{"Two 10-ft. cubes/level", true},                        // "Two" counts cubes
		{"1 creature or 1 cu. ft. of substance/2 levels", true}, // an alternative it does not read whole
		{"4d4 sq. mi.", true},                                   // dice it dropped
		{"5 yds./level (90 yds. maximum)", true},                // a cap it dropped
		{"Line of sight, max. 60 ft.", true},                    // words it does not read
		{"1 object up to 10 ft. long", true},                    // "long"
		{"1 or more creatures in a 10-ft. radius", true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			assert.Equal(t, tt.partial, spellstats.ParseArea(tt.raw).Partial)
		})
	}
	for _, raw := range []string{"The caster's hand", "Creature touched", "Special", "Variable"} {
		assert.False(t, spellstats.ParseArea(raw).Partial, raw)
	}
}

// ADR 0021 step 8: a partial area reading has no value at a caster level (the
// lossy reading would give "10 creatures" for "One creature/two levels" at
// level 10, and 0 for "1,000 square feet/level"); a whole reading has one.
func TestAreaOfEffect_PartialReadingHasNoValueAtACasterLevel(t *testing.T) {
	for _, raw := range []string{"4d4 sq. mi.", "5 yds./level (90 yds. maximum)", "1 creature or 1 cu. ft. of substance/2 levels"} {
		t.Run(raw, func(t *testing.T) {
			a := spellstats.ParseArea(raw)
			require.True(t, a.Partial)
			assert.Equal(t, spellstats.AreaValue{}, a.At(10))
		})
	}
	whole := spellstats.ParseArea("1 creature/3 levels")
	require.False(t, whole.Partial)
	require.NotNil(t, whole.At(9).Targets)
	assert.Equal(t, 3.0, whole.At(9).Targets.Amount)
}

// "Up to" is kept on the quantity it bounds (the extent or the count), so
// the reading is whole and a reader words it.
func TestAreaOfEffect_UpToBoundsItsQuantity(t *testing.T) {
	cube := spellstats.ParseArea("Up to 20-ft. cube")
	require.Len(t, cube.Dims, 1)
	assert.True(t, cube.Dims[0].UpTo)
	assert.False(t, cube.Partial)

	animals := spellstats.ParseArea("Up to 8 animals in a 20-ft cube")
	require.NotNil(t, animals.Targets)
	assert.True(t, animals.Targets.UpTo)
	assert.False(t, animals.Dims[0].UpTo, "the cube is not bounded")
	assert.False(t, animals.Partial)

	assert.False(t, spellstats.ParseArea("20-ft. cube").Dims[0].UpTo)
}

// ADR 0021 step 8: a choice ("1 creature or object", "A, B, or C") is read
// as one whole reading per alternative; a bare noun counts as many as the
// alternative before it and a bare number takes the unit and shape of the
// one after it. A choice with an alternative the parser does not read whole
// stays a partial single reading. Alternatives have no value at a caster
// level: the book does not say which one the caster picks.
func TestAreaOfEffect_Alternatives(t *testing.T) {
	one := &spellstats.Quantity{Terms: []spellstats.Term{{Count: 1}}}
	foot := func(n float64) spellstats.Quantity {
		return spellstats.Quantity{Terms: []spellstats.Term{{Count: n, Unit: "foot"}}}
	}
	tests := []struct {
		raw  string
		want []spellstats.AreaOfEffect
	}{
		{"1 creature or object", []spellstats.AreaOfEffect{
			{Raw: "1 creature", Kind: "targets", Targets: one, Noun: "creature"},
			{Raw: "object", Kind: "targets", Targets: one, Noun: "object"},
		}},
		{"One doorway, portal, or small object", []spellstats.AreaOfEffect{
			{Raw: "One doorway", Kind: "targets", Targets: one, Noun: "doorway"},
			{Raw: "portal", Kind: "targets", Targets: one, Noun: "portal"},
			{Raw: "small object", Kind: "targets", Targets: one, Noun: "small object"},
		}},
		{"1 scroll or 2 pages", []spellstats.AreaOfEffect{
			{Raw: "1 scroll", Kind: "targets", Targets: one, Noun: "scroll"},
			{Raw: "2 pages", Kind: "targets", Targets: &spellstats.Quantity{Terms: []spellstats.Term{{Count: 2}}}, Noun: "pages"},
		}},
		{"10-ft. cube or one item", []spellstats.AreaOfEffect{
			{Raw: "10-ft. cube", Kind: "area", Shape: "cube", Dims: []spellstats.Quantity{foot(10)}},
			{Raw: "one item", Kind: "targets", Targets: one, Noun: "item"},
		}},
		{"10-ft. radius sphere or 15-ft. radius hemisphere", []spellstats.AreaOfEffect{
			{Raw: "10-ft. radius sphere", Kind: "area", Shape: "radius", Figure: "sphere", Dims: []spellstats.Quantity{foot(10)}},
			{Raw: "15-ft. radius hemisphere", Kind: "area", Shape: "radius", Figure: "hemisphere", Dims: []spellstats.Quantity{foot(15)}},
		}},
		{"20 or 40 ft. radius", []spellstats.AreaOfEffect{
			{Raw: "20 ft. radius", Kind: "area", Shape: "radius", Dims: []spellstats.Quantity{foot(20)}},
			{Raw: "40 ft. radius", Kind: "area", Shape: "radius", Dims: []spellstats.Quantity{foot(40)}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := spellstats.ParseArea(tt.raw)
			assert.Equal(t, spellstats.AreaOfEffect{Raw: tt.raw, Kind: "alternatives", Alternatives: tt.want}, got)
			assert.Equal(t, spellstats.AreaValue{}, got.At(10))
		})
	}
	for _, raw := range []string{
		"1 creature or 1 cu. ft. of substance/2 levels", "1 or more creatures in a 10-ft. radius",
		"Two or more creatures", "1 dead human or demihuman",
	} {
		assert.NotEqual(t, "alternatives", spellstats.ParseArea(raw).Kind, raw)
	}
}

// A radius or diameter carries the figure it measures, so a reading never
// drops it ("10-ft. radius sphere" is not "10-ft. radius hemisphere").
func TestAreaOfEffect_RadiusCarriesItsFigure(t *testing.T) {
	tests := []struct{ raw, shape, figure string }{
		{"20-foot-radius circle", "radius", "circle"},
		{"15-ft.-diameter sphere", "diameter", "sphere"},
		{"20-ft.-radius globe", "radius", "globe"},
		{"20-ft. radius", "radius", ""},
		{"10-ft. cube", "cube", ""},
		{"10-foot circle", "circle", ""},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			a := spellstats.ParseArea(tt.raw)
			assert.Equal(t, tt.shape, a.Shape)
			assert.Equal(t, tt.figure, a.Figure)
			assert.False(t, a.Partial)
		})
	}
}
