package ruletablesvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/slots"
)

func okCrossovers(t *testing.T) []ruletablesvc.Crossover {
	t.Helper()
	var out []ruletablesvc.Crossover
	for _, c := range cataloguetest.Definitions(t).ClassIDs() {
		out = append(out, ruletablesvc.Crossover{Class: c, Groups: []string{"general"}})
	}
	return out
}

func TestNewProficiencies_RefusesWhatTheSchemaCannotSay(t *testing.T) {
	row := func(edit func(*ruletablesvc.Proficiency)) []ruletablesvc.Proficiency {
		p := ruletablesvc.Proficiency{ID: "swimming", Ability: "str", Slots: 1, Groups: []string{"general"}}
		edit(&p)
		return []ruletablesvc.Proficiency{p}
	}
	cases := map[string]struct {
		list  []ruletablesvc.Proficiency
		cross []ruletablesvc.Crossover
		slots []slots.Row
		want  string
	}{
		"no id":             {row(func(p *ruletablesvc.Proficiency) { p.ID = "" }), nil, nil, "no id"},
		"unknown ability":   {row(func(p *ruletablesvc.Proficiency) { p.Ability = "luck" }), nil, nil, "unknown ability"},
		"modifier no check": {row(func(p *ruletablesvc.Proficiency) { p.Ability = ""; p.Modifier = 2 }), nil, nil, "modifier but no ability"},
		"no slots":          {row(func(p *ruletablesvc.Proficiency) { p.Slots = 0 }), nil, nil, "at least 1 slot"},
		"no group":          {row(func(p *ruletablesvc.Proficiency) { p.Groups = nil }), nil, nil, "no group"},
		"unknown group":     {row(func(p *ruletablesvc.Proficiency) { p.Groups = []string{"druid"} }), nil, nil, "unknown group"},
		"group twice":       {row(func(p *ruletablesvc.Proficiency) { p.Groups = []string{"general", "general"} }), nil, nil, "twice"},
		"unknown class":     {nil, []ruletablesvc.Crossover{{Class: "paladin", Groups: []string{"general"}}}, nil, "unknown class"},
		"class missing":     {nil, okCrossovers(t)[:4], nil, "missing"},
		"slot group":        {nil, nil, []slots.Row{{Group: "druid", WeaponLevels: 1, NonweaponLevels: 1}}, "unknown group"},
		"slot levels":       {nil, nil, []slots.Row{{Group: "rogue", WeaponLevels: 1}}, "#Levels"},
		"slot group gone":   {nil, nil, []slots.Row{{Group: "rogue", WeaponLevels: 4, NonweaponLevels: 4}}, "missing"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ruletablesvc.NewProficiencies(c.list, c.cross, c.slots, cataloguetest.Definitions(t))
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
	dup := append(row(func(*ruletablesvc.Proficiency) {}), row(func(*ruletablesvc.Proficiency) {})...)
	_, err := ruletablesvc.NewProficiencies(dup, okCrossovers(t), nil, cataloguetest.Definitions(t))
	require.ErrorContains(t, err, "twice")
}

func TestProficiencies_NoCheckRowAndEmptyCatalogue(t *testing.T) {
	p, err := ruletablesvc.NewProficiencies([]ruletablesvc.Proficiency{
		{ID: "blind_fighting", Slots: 2, Groups: []string{"rogue", "warrior"}},
	}, okCrossovers(t), nil, cataloguetest.Definitions(t))
	require.NoError(t, err)
	assert.Equal(t, "blind_fighting", p.All()[0].ID)
	assert.Len(t, p.Crossovers(), 6)

	var none *ruletablesvc.Proficiencies
	assert.Empty(t, none.All())
	assert.Empty(t, none.Crossovers())
}
