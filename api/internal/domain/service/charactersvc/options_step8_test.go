package charactersvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// ADR 0040 step 8: every class the catalogue defines is creatable, with no Go
// list of the creatable classes. A catalogue with a sixth class, a rogue the
// code has never seen, offers it after the five, with its group's hit die,
// its minimums, its alignments and the races the level limits admit.
func TestOptions_Step8_EveryCatalogueClassIsCreatable(t *testing.T) {
	fixture := cataloguetest.Definitions(t)
	xp := make([]int, classes.MaxLevel)
	for i := range xp {
		xp[i] = i * 900
	}
	skald := classes.Class{
		ID: "test_skald", Name: localized.Text{"en": "Skald"}, Group: "rogue", PrimeRequisites: []string{"cha"},
		AbilityMinimums: []classes.AbilityMinimum{{Ability: "cha", Min: 14}},
		Alignments:      []string{"chaotic_neutral", "chaotic_good"}, Experience: xp,
	}
	defs, err := classes.New(append(fixture.Classes(), skald), fixture.Groups(), fixture.Races())
	require.NoError(t, err)

	e := newEnv(t)
	// The fixture's registry is newRules' default; this test's replaces it.
	for i, c := range e.rules.ExpectedCalls {
		if c.Method == "Definitions" {
			e.rules.ExpectedCalls = append(e.rules.ExpectedCalls[:i], e.rules.ExpectedCalls[i+1:]...)
			break
		}
	}
	e.rules.EXPECT().Definitions().Return(defs, nil)
	e.rules.EXPECT().RaceAllows(mock.Anything, mock.Anything).RunAndReturn(func(race, class string) (bool, error) {
		return class != "test_skald" || race == "human" || race == "elf", nil
	})
	expectOptionTables(e)

	got, err := e.svc.CreationOptions(context.Background())
	require.NoError(t, err)
	ids := make([]string, len(got.Classes))
	for i, c := range got.Classes {
		ids[i] = c.Class
	}
	assert.Equal(t, []string{"fighter", "mage", "cleric", "thief", "bard", "test_skald"}, ids, "the catalogue's classes, in its order")
	opt := got.Classes[5]
	assert.Equal(t, 6, opt.HitDie, "the rogue group's die")
	assert.Equal(t, []classes.AbilityMinimum{{Ability: "cha", Min: 14}}, opt.Minimums)
	assert.Equal(t, []string{"chaotic_good", "chaotic_neutral"}, opt.Alignments)
	assert.Equal(t, []string{"elf", "human"}, opt.Races, "the races the rules admit, in catalogue order")
	assert.Equal(t, 2*1000+len("test_skald"), opt.LevelXP[1].XP, "its level 2 read from the rules (expectOptionTables)")
}
