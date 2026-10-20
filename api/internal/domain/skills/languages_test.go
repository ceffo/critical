package skills_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// phbLanguages is the language catalogue of the PHB racial lists (chapter 2) plus Common, with
// invented names: the six races' lists, each with its own language marked.
func phbLanguages(t *testing.T) *ruletablesvc.Languages {
	t.Helper()
	var list []ruletablesvc.Language
	for _, c := range []string{"common", "dwarvish", "elvish", "gnomish", "halfling", "goblin", "hobgoblin", "kobold", "orcish", "gnoll", "burrowers"} {
		list = append(list, ruletablesvc.Language{Code: c, Name: localized.Text{"en": c}})
	}
	lists := map[string][]string{
		"human":    {"common"},
		"dwarf":    {"common", "dwarvish", "gnomish", "goblin", "kobold", "orcish"},
		"elf":      {"common", "elvish", "gnomish", "halfling", "goblin", "hobgoblin", "orcish", "gnoll"},
		"gnome":    {"common", "dwarvish", "gnomish", "halfling", "goblin", "kobold", "burrowers"},
		"half_elf": {"common", "elvish", "gnomish", "halfling", "goblin", "hobgoblin", "orcish", "gnoll"},
		"halfling": {"common", "halfling", "dwarvish", "elvish", "gnomish", "goblin", "orcish"},
	}
	native := map[string]string{
		"human": "common", "dwarf": "dwarvish", "elf": "elvish", "gnome": "gnomish", "half_elf": "common", "halfling": "halfling",
	}
	var rows []ruletablesvc.RaceLanguageRow
	for _, race := range ruletablesvc.RaceCodes {
		for _, l := range lists[race] {
			rows = append(rows, ruletablesvc.RaceLanguageRow{Race: race, Language: l, Native: l == native[race]})
		}
	}
	l, err := ruletablesvc.NewLanguages(list, rows)
	require.NoError(t, err)
	return l
}

// ADR 0037 step 6: every language other than the native one and Common spends one non-weapon slot:
// an elf's Elvish and Common are free, a half-elf's native language is Common, a dwarf with Dwarvish
// and Common spends nothing; without a catalogue (no native known) nothing is counted. Table 4's
// "# of Lang." is the pool's Intelligence term, counted once: the languages spend, they add no slot.
func TestLanguagePicks_Step6_EachExtraLanguageCostsOneSlot(t *testing.T) {
	assert.Equal(t, 2, skills.LanguageSlots([]string{"elvish", "common", "gnomish", "orcish"}, "elvish"))
	assert.Equal(t, 3, skills.LanguageSlots([]string{"common", "elvish", "orcish", "dwarvish"}, "common"), "a half-elf: Common is native")
	assert.Equal(t, 0, skills.LanguageSlots([]string{"dwarvish", "common"}, "dwarvish"))
	assert.Equal(t, 0, skills.LanguageSlots([]string{"common", "elvish"}, ""), "no catalogue, no verdict")

	// A level 1 bard (rogue: 3 + 0) with Intelligence 9 (Table 4: 2): 5 slots, the same with or
	// without languages; Tumbling and two languages spend 3.
	book := profBook(t)
	bare, ok := skills.ProficiencyFigures(book, "bard", 1, 9, skills.Spending{Picks: picks(p("tumbling", 1))})
	require.True(t, ok)
	f, _ := skills.ProficiencyFigures(book, "bard", 1, 9, skills.Spending{Picks: picks(p("tumbling", 1)), Languages: 2})
	assert.Equal(t, 2, f.Nonweapon.Intelligence)
	assert.Equal(t, bare.Nonweapon.Available, f.Nonweapon.Available, "the Intelligence bonus is counted once")
	assert.Equal(t, 5, f.Nonweapon.Available)
	assert.Equal(t, 3, f.Nonweapon.Spent)
	assert.Equal(t, 2, f.Nonweapon.Languages)
	assert.Equal(t, 2, f.Nonweapon.Left)
	assert.Len(t, f.Picks, 1, "a language is no pick of its own")
}

// ADR 0037 steps 6 and 10: the languages spend the non-weapon pool under its budget rule. A languages
// change that newly overspends or grows an overspend is refused on field languages; dropping one
// from an overspend is stored; a picks change over budget is refused on field nonweapon; at
// creation (nothing spent before) languages alone over the pool are refused.
func TestBudget_Step6_LanguagesSpendTheNonweaponPool(t *testing.T) {
	book := profBook(t) // a level 1 bard, Intelligence 9: 5 slots
	spend := func(langs int, ps ...skills.Pick) skills.Spending {
		return skills.Spending{Picks: picks(ps...), Languages: langs}
	}
	budget := func(before, after skills.Spending) error {
		return skills.PicksWithinBudget(book, "bard", 1, 9, before, after)
	}
	var be *domain.BudgetError
	require.ErrorAs(t, budget(spend(3, p("tumbling", 2)), spend(4, p("tumbling", 2))), &be, "5 to 6 of 5")
	assert.Equal(t, domain.BudgetError{Field: "languages", Spent: 6, Limit: 5}, *be)
	require.ErrorAs(t, budget(spend(4, p("tumbling", 3)), spend(5, p("tumbling", 3))), &be, "an overspend that grows")
	require.NoError(t, budget(spend(4, p("tumbling", 3)), spend(3, p("tumbling", 3))), "7 to 6 of 5: shrinking is stored")
	require.NoError(t, budget(spend(3, p("tumbling", 2)), spend(3, p("tumbling", 2))), "unchanged")
	require.ErrorAs(t, budget(spend(3, p("tumbling", 1)), spend(3, p("tumbling", 3))), &be)
	assert.Equal(t, "nonweapon", be.Field, "the picks grew")
	require.ErrorAs(t, budget(skills.Spending{}, spend(6)), &be, "a creation with six languages")
	assert.Equal(t, domain.BudgetError{Field: "languages", Spent: 6, Limit: 5}, *be)
	require.ErrorAs(t, budget(skills.Spending{}, spend(4, p("tumbling", 2))), &be, "a creation whose picks fit, not with the languages")
	assert.Equal(t, "languages", be.Field)
	require.ErrorAs(t, budget(skills.Spending{}, spend(1, p("tumbling", 6))), &be, "a creation whose picks alone overspend")
	assert.Equal(t, "nonweapon", be.Field)
}

// ADR 0032 steps 4.1, 4.3 and 8.1 on a list of languages (at creation and on the Skills tab's
// save, ADR 0037 step 18): any catalogue code, each once, the race's native one added in front
// when missing and never doubled; a blank, an unknown code and a repeat refused on field
// languages; with no catalogue the codes the character holds stay valid, any other is unknown.
func TestValidLanguages_Step4_CodesCheckedAndTheNativeKept(t *testing.T) {
	book := phbLanguages(t)
	for _, tc := range []struct {
		name string
		race string
		in   []string
		want []string
	}{
		{"none sent: the native one", "dwarf", nil, []string{"dwarvish"}},
		{"added in front", "elf", []string{"gnomish"}, []string{"elvish", "gnomish"}},
		{"not doubled", "elf", []string{"gnomish", "elvish"}, []string{"gnomish", "elvish"}},
		{"outside the race's list", "human", []string{"burrowers"}, []string{"common", "burrowers"}},
	} {
		got, err := skills.ValidLanguages(book, tc.race, nil, tc.in)
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.want, got, tc.name)
	}
	for _, tc := range []struct {
		name string
		in   []string
		code string
	}{
		{"blank", []string{"common", ""}, "empty"},
		{"free text", []string{"Elfique"}, "unknown"},
		{"no catalogue language", []string{"draconic"}, "unknown"},
		{"a repeat", []string{"goblin", "goblin"}, "duplicate"},
	} {
		_, err := skills.ValidLanguages(book, "elf", nil, tc.in)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe, tc.name)
		assert.Equal(t, domain.FieldError{Field: "languages", Message: tc.code}, *fe, tc.name)
	}
	got, err := skills.ValidLanguages(nil, "elf", []string{"elvish", "common"}, []string{"common", "elvish"})
	require.NoError(t, err)
	assert.Equal(t, []string{"common", "elvish"}, got, "no catalogue: the held codes stay, no native added")
	_, err = skills.ValidLanguages((*ruletablesvc.Languages)(nil), "elf", []string{"elvish"}, []string{"draconic"})
	require.Error(t, err)
}
