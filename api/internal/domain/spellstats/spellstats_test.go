package spellstats_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// All inputs are literal PHB strings written here; nothing reads seeds/.

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
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := spellstats.ParseArea(tt.raw)
			assert.Equal(t, tt.raw, got.Raw)
			assert.Equal(t, tt.kind, got.Kind)
			assert.Equal(t, tt.shape, got.Shape)
			assert.Equal(t, tt.noun, got.Noun)
			at := got.At(tt.level)
			assert.Equal(t, tt.dims, at.Dims)
			assert.Equal(t, tt.targets, at.Targets)
		})
	}
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
