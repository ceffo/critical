package skills_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/skills"
)

// ADR 0037 step 23.1: the weapon pool names the entries whose Table 34 rate it is counted on (a
// fighter/cleric's weapon column is the fighter's, every 3 levels against the priest's 4), the
// non-weapon pool every active entry. The cleric is listed first: a pool tagged with the first
// class fails.
func TestPoolClasses_Step23_1_TheRateEntryAndEveryActiveEntry(t *testing.T) {
	book := profBook(t)
	got, ok := skills.FiguresOf(book, skills.Classes{{Class: "cleric", Level: 9}, {Class: "fighter", Level: 6}}, 0, skills.Spending{})
	require.True(t, ok)
	assert.Equal(t, []skills.ClassRef{{Class: "fighter", Level: 6}}, got.WeaponClasses)
	assert.Equal(t, []skills.ClassRef{{Class: "cleric", Level: 9}, {Class: "fighter", Level: 6}}, got.NonweaponClasses)

	// On a tie (both rows every 3 levels, non-weapon) the weapon column of two warriors' rate
	// names both entries.
	tie, ok := skills.FiguresOf(book, skills.Classes{{Class: "fighter", Level: 4}, {Class: "fighter", Level: 5}}, 0, skills.Spending{})
	require.True(t, ok)
	assert.Len(t, tie.WeaponClasses, 2)

	one, ok := skills.FiguresOf(book, skills.One("bard", 5), 0, skills.Spending{})
	require.True(t, ok)
	assert.Equal(t, []skills.ClassRef{{Class: "bard", Level: 5}}, one.WeaponClasses)
}

// ADR 0037 step 23.3: each held language is served with its slots: 0 for the native one and
// Common, 1 for the others, and the costs add up to the pool's languages term.
func TestLanguageCosts_Step23_3_PerLanguageAddsUpToThePool(t *testing.T) {
	langs := []string{"elvish", "common", "dwarvish", "gnomish"}
	costs := skills.LanguageCosts(langs, "elvish")
	assert.Equal(t, []skills.LanguageCost{{"elvish", 0}, {"common", 0}, {"dwarvish", 1}, {"gnomish", 1}}, costs)
	assert.Equal(t, 2, skills.LanguageSlots(langs, "elvish"))
	assert.Equal(t, []skills.LanguageCost{{"elvish", 0}, {"common", 0}}, skills.LanguageCosts(langs[:2], "elvish"))
	for _, c := range skills.LanguageCosts(langs, "") {
		assert.Zero(t, c.Slots, "no catalogue: no language is known to be free, none is counted")
	}

	sheet := skills.Sheet{Class: "bard", Level: 5, Intelligence: 12, Languages: langs, Native: "elvish"}
	f, _ := skills.Read(profBook(t), sheet)
	require.NotNil(t, f)
	assert.Equal(t, costs, f.LanguageCosts)
	assert.Equal(t, 2, f.Nonweapon.Languages)
}

// ADR 0037 step 23.4: each catalogue row carries its check and the target of a new pick at no
// extra slot (score + modifier), from the score in effect; none without a score.
func TestCatalogueTarget_Step23_4_ScorePlusModifierAtNoExtraSlot(t *testing.T) {
	book := profBook(t)
	f, ok := skills.ProficiencyFigures(book, "bard", 1, 9, skills.Spending{})
	require.True(t, ok)
	var before skills.CatalogueFigure
	for _, c := range f.Catalogue {
		if c.ID == "tumbling" {
			before = c
		}
	}
	assert.Equal(t, "dex", before.Ability)
	assert.False(t, before.HasCheck, "no score known yet")

	f.SetTargets(func(ability string) (int, bool) { return map[string]int{"dex": 17}[ability], ability == "dex" })
	for _, c := range f.Catalogue {
		if c.ID == "tumbling" {
			assert.True(t, c.HasCheck)
			assert.Equal(t, 17+c.Modifier, c.Target)
			assert.Equal(t, 17, c.Target, "tumbling's modifier is 0")
		} else if c.Ability != "dex" {
			assert.False(t, c.HasCheck, c.ID)
		}
	}
}
