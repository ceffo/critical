package charactersvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
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
	for _, race := range charactersvc.Races {
		for _, l := range lists[race] {
			rows = append(rows, ruletablesvc.RaceLanguageRow{Race: race, Language: l, Native: l == native[race]})
		}
	}
	l, err := ruletablesvc.NewLanguages(list, rows)
	require.NoError(t, err)
	return l
}

// newEnvWithLanguages is newEnv with the PHB language catalogue and the proficiency book (the
// languages spend non-weapon slots, ADR 0037 step 6).
func newEnvWithLanguages(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	svc, err := charactersvc.New(charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: e.rules, Owners: e.owners, Tx: e.tx, Sessions: e.sessions, Armour: e.armour, HP: e.hp,
		Languages: phbLanguages(t), Proficiencies: profBook(t),
	})
	require.NoError(t, err)
	e.svc = svc
	return e
}

// createdDetails creates lyra, with the race and the languages given (nil sends none), and returns
// the details the owners port was asked to store.
func createdDetails(t *testing.T, race string, languages *[]string) (charactersvc.Details, error) {
	t.Helper()
	e := newEnvWithLanguages(t)
	n := lyra()
	n.Race = race
	n.Details.Languages = languages
	e.rules.EXPECT().RaceAllows(race, "bard").Return(true, nil).Maybe()
	e.rules.EXPECT().LevelFor("bard", 0).Return(1, nil).Maybe()
	e.rules.EXPECT().XPThreshold("bard", 2).Return(1_250, nil).Maybe()
	e.rules.EXPECT().THAC0For("rogue", 1).Return(20, nil).Maybe()
	e.rules.EXPECT().SavesFor("rogue", 1).Return(rogue1to4, nil).Maybe()
	e.rules.EXPECT().BardSpellsPerDay(1).Return(map[int]int{}, nil).Maybe()
	e.rules.EXPECT().ConSaveBonusFor(mock.Anything).Return(0, nil).Maybe()
	var stored charactersvc.Details
	e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.Anything).
		RunAndReturn(func(_ context.Context, _ uuid.UUID, c charactersvc.Character) (int64, error) {
			stored = c.Details
			return 1, nil
		}).Maybe()
	e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(1)).Return(nil).Maybe()
	e.sessions.EXPECT().ForgetUser(ownerID).Maybe()
	_, err := e.svc.Create(context.Background(), ownerID, n)
	return stored, err
}

// ADR 0032 step 4.1: a new character gets its race's native language without picking it, whether
// or not the form sent languages: each race its own (dwarf, elf, gnome, halfling), human and
// half-elf Common. It is not doubled when the form sends it, and it comes first.
func TestCreate_Step4_1_ANewCharacterGetsItsNativeLanguageWithoutPickingIt(t *testing.T) {
	for race, native := range map[string]string{
		"human": "common", "dwarf": "dwarvish", "elf": "elvish", "gnome": "gnomish", "half_elf": "common", "halfling": "halfling",
	} {
		t.Run(race, func(t *testing.T) {
			got, err := createdDetails(t, race, nil)
			require.NoError(t, err)
			assert.Equal(t, []string{native}, got.Languages, "none sent")

			got, err = createdDetails(t, race, &[]string{"goblin", native})
			require.NoError(t, err)
			assert.Equal(t, []string{"goblin", native}, got.Languages, "the native one sent is not doubled")

			got, err = createdDetails(t, race, &[]string{"goblin"})
			require.NoError(t, err)
			assert.Equal(t, []string{native, "goblin"}, got.Languages, "added in front of the picked ones")
		})
	}
}

// ADR 0032 step 4.3: the server accepts any catalogue language, the race's list marked but not
// enforced (a human picks from the whole catalogue; a dwarf may take a language its list lacks);
// a code outside the catalogue is `unknown`, a repeat `duplicate`, on details.languages at
// creation.
func TestCreate_Step4_3_AnyCatalogueLanguageIsAcceptedOthersRefused(t *testing.T) {
	got, err := createdDetails(t, "human", &[]string{"elvish", "gnoll", "burrowers"})
	require.NoError(t, err, "a human picks from the catalogue")
	assert.Equal(t, []string{"common", "elvish", "gnoll", "burrowers"}, got.Languages)
	got, err = createdDetails(t, "dwarf", &[]string{"gnoll"})
	require.NoError(t, err, "a dwarf may take a language outside the race's list")
	assert.Equal(t, []string{"dwarvish", "gnoll"}, got.Languages)

	for _, tt := range []struct {
		name  string
		sent  []string
		field string
		code  string
	}{
		{"free text", []string{"Elfique"}, "details.languages", "unknown"},
		{"a code of no catalogue language", []string{"draconic"}, "details.languages", "unknown"},
		{"a repeated language", []string{"goblin", "goblin"}, "details.languages", "duplicate"},
		{"a blank one", []string{""}, "details.languages", "empty"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := createdDetails(t, "dwarf", &tt.sent)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, tt.field, fe.Field)
			assert.Equal(t, tt.code, fe.Message)
		})
	}
}

// ADR 0037 step 6: every language other than the native one and Common spends one non-weapon slot:
// an elf's Elvish and Common are free, a half-elf's native language is Common, a dwarf with Dwarvish
// and Common spends nothing; without a catalogue (no native known) nothing is counted. Table 4's
// "# of Lang." is the pool's Intelligence term, counted once: the languages spend, they add no slot.
func TestLanguagePicks_Step6_EachExtraLanguageCostsOneSlot(t *testing.T) {
	assert.Equal(t, 2, charactersvc.LanguageSlots([]string{"elvish", "common", "gnomish", "orcish"}, "elvish"))
	assert.Equal(t, 3, charactersvc.LanguageSlots([]string{"common", "elvish", "orcish", "dwarvish"}, "common"), "a half-elf: Common is native")
	assert.Equal(t, 0, charactersvc.LanguageSlots([]string{"dwarvish", "common"}, "dwarvish"))
	assert.Equal(t, 0, charactersvc.LanguageSlots([]string{"common", "elvish"}, ""), "no catalogue, no verdict")

	// A level 1 bard (rogue: 3 + 0) with Intelligence 9 (Table 4: 2): 5 slots, the same with or
	// without languages; Tumbling and two languages spend 3.
	book := profBook(t)
	bare, ok := charactersvc.ProficiencyFigures(book, "bard", 1, 9, charactersvc.Spending{Picks: picks(p("tumbling", 1))})
	require.True(t, ok)
	f, _ := charactersvc.ProficiencyFigures(book, "bard", 1, 9, charactersvc.Spending{Picks: picks(p("tumbling", 1)), Languages: 2})
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
	spend := func(langs int, ps ...charactersvc.Pick) charactersvc.Spending {
		return charactersvc.Spending{Picks: picks(ps...), Languages: langs}
	}
	budget := func(before, after charactersvc.Spending) error {
		return charactersvc.PicksWithinBudget(book, "bard", 1, 9, before, after)
	}
	var be *domain.BudgetError
	require.ErrorAs(t, budget(spend(3, p("tumbling", 2)), spend(4, p("tumbling", 2))), &be, "5 to 6 of 5")
	assert.Equal(t, domain.BudgetError{Field: "languages", Spent: 6, Limit: 5}, *be)
	require.ErrorAs(t, budget(spend(4, p("tumbling", 3)), spend(5, p("tumbling", 3))), &be, "an overspend that grows")
	require.NoError(t, budget(spend(4, p("tumbling", 3)), spend(3, p("tumbling", 3))), "7 to 6 of 5: shrinking is stored")
	require.NoError(t, budget(spend(3, p("tumbling", 2)), spend(3, p("tumbling", 2))), "unchanged")
	require.ErrorAs(t, budget(spend(3, p("tumbling", 1)), spend(3, p("tumbling", 3))), &be)
	assert.Equal(t, "nonweapon", be.Field, "the picks grew")
	require.ErrorAs(t, budget(charactersvc.Spending{}, spend(6)), &be, "a creation with six languages")
	assert.Equal(t, domain.BudgetError{Field: "languages", Spent: 6, Limit: 5}, *be)
}

// ADR 0037 steps 6 and 10 at creation: Lyra (half-elf bard, level 1, Intelligence 14: 3 + 0 + 4 = 7
// slots) may learn seven languages beyond Common; an eighth is refused over budget, nothing stored.
func TestCreate_Step6_LanguagesBeyondTheSlotsAreRefused(t *testing.T) {
	seven := []string{"elvish", "gnomish", "halfling", "goblin", "hobgoblin", "orcish", "gnoll"}
	got, err := createdDetails(t, "half_elf", &seven)
	require.NoError(t, err)
	assert.Len(t, got.Languages, 8, "Common and seven")

	eight := append(append([]string{}, seven...), "dwarvish")
	_, err = createdDetails(t, "half_elf", &eight)
	var be *domain.BudgetError
	require.ErrorAs(t, err, &be)
	assert.Equal(t, domain.BudgetError{Field: "languages", Spent: 8, Limit: 7}, *be)
}

// ADR 0032 step 4.1 and ADR 0037 step 6 on a read: the character carries its native language (added
// when the stored languages lack it), and its other languages are spent from the non-weapon pool;
// languages_over_limit is no longer served.
func TestGetCharacter_Step6_TheLanguagesSpendTheNonweaponPool(t *testing.T) {
	c := kelsen() // a half-elf bard, level 15, Intelligence 14: 3 + 3 + 4 = 10 slots
	c.Details = charactersvc.Details{Alignment: "neutral_good", Sex: "male", Languages: []string{"elvish", "gnomish", "halfling", "goblin", "orcish"}}
	c.Proficiencies = picks(p("tumbling", 6))
	e := newEnvWithLanguages(t)
	e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(c, nil).Once()
	e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
	expectKelsenRules(e.rules, 1_293_418, 15)

	got, err := e.svc.GetCharacter(context.Background(), 3)
	require.NoError(t, err)
	assert.Equal(t, []string{"common", "elvish", "gnomish", "halfling", "goblin", "orcish"}, got.Details.Languages,
		"Common, a half-elf's native language, is added")
	pool := got.Derived.Proficiencies.Nonweapon
	assert.Equal(t, 5, pool.Languages)
	assert.Equal(t, 11, pool.Spent)
	assert.Equal(t, 10, pool.Available)
	require.NotEmpty(t, got.Warnings)
	assert.Equal(t, charactersvc.CodeSlotsOverBudget, got.Warnings[0].Code)
	assert.Equal(t, 11, *got.Warnings[0].Spent)
	for _, w := range got.Warnings {
		assert.NotEqual(t, "languages_over_limit", w.Code)
	}
}

// ADR 0037 steps 6 and 10 on a languages edit (detailssvc asks it): Kelsen over budget (11 of 10)
// may drop a language but not add one; under budget, a language that newly overspends is refused.
func TestLanguagesWithinBudget_Step6_AnEditIsCheckedByTheBudget(t *testing.T) {
	kelsenWith := func(slots int) *charactersvc.Character {
		c := kelsen()
		c.Details = charactersvc.Details{Languages: []string{"common", "elvish", "gnomish", "halfling", "goblin", "orcish"}}
		c.Proficiencies = picks(p("tumbling", slots))
		return c
	}
	check := func(c *charactersvc.Character, languages []string) error {
		e := newEnvWithLanguages(t)
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(c, nil).Once()
		e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
		expectKelsenRules(e.rules, 1_293_418, 15)
		return e.svc.LanguagesWithinBudget(context.Background(), 3, languages)
	}
	var be *domain.BudgetError
	err := check(kelsenWith(6), []string{"common", "elvish", "gnomish", "halfling", "goblin", "orcish", "gnoll"})
	require.ErrorAs(t, err, &be)
	assert.Equal(t, domain.BudgetError{Field: "languages", Spent: 12, Limit: 10}, *be)
	require.NoError(t, check(kelsenWith(6), []string{"common", "elvish", "gnomish", "halfling", "goblin"}), "11 to 10")
	require.NoError(t, check(kelsenWith(6), []string{"common", "elvish", "gnomish", "halfling", "orcish", "goblin"}), "the same, reordered")

	err = check(kelsenWith(5), []string{"common", "elvish", "gnomish", "halfling", "goblin", "orcish", "gnoll"})
	require.ErrorAs(t, err, &be, "10 to 11: a new overspend")
	require.NoError(t, check(kelsenWith(4), []string{"common", "elvish", "gnomish", "halfling", "goblin", "orcish", "gnoll"}), "9 to 10: at the budget")
}

// Before catalogue format 8 the native language is unknown: a migrated half-elf knowing common,
// elvish, orcish and dwarvish gets no warning for its languages.
func TestGetCharacter_Step6_NoLanguageVerdictOnAnEmptyCatalogue(t *testing.T) {
	c := kelsen()
	c.Int = 13
	c.Details = charactersvc.Details{Alignment: "neutral_good", Sex: "male", Languages: []string{"common", "elvish", "orcish", "dwarvish"}}
	e := newEnv(t) // no language book
	e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(c, nil).Once()
	e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
	expectKelsenRules(e.rules, 1_293_418, 15)

	got, err := e.svc.GetCharacter(context.Background(), 3)
	require.NoError(t, err)
	assert.Equal(t, []string{"common", "elvish", "orcish", "dwarvish"}, got.Details.Languages)
	assert.Empty(t, got.Warnings)
}

// ADR 0032 step 4.3: the creation options hold the whole catalogue with its names, each race's own
// language and PHB list (the form marks it), and Table 4 as bands of Intelligence scores.
func TestCreationOptions_Step4_OfferTheCatalogueTheRaceListsAndTheLimits(t *testing.T) {
	e := newEnvWithLanguages(t)
	e.rules.EXPECT().RaceAllows(mock.Anything, mock.Anything).Return(true, nil).Maybe()
	expectOptionTables(e)
	o, err := e.svc.CreationOptions(context.Background())
	require.NoError(t, err)
	assert.Len(t, o.Languages, 11)
	assert.Equal(t, "common", o.Languages[0].Code)
	require.Len(t, o.RaceLanguages, 6)
	byRace := map[string]charactersvc.RaceLanguageOption{}
	for _, r := range o.RaceLanguages {
		byRace[r.Race] = r
	}
	assert.Equal(t, "dwarvish", byRace["dwarf"].Native)
	assert.Equal(t, []string{"common", "dwarvish", "gnomish", "goblin", "kobold", "orcish"}, byRace["dwarf"].List)
	assert.Equal(t, "common", byRace["half_elf"].Native)

	// Before catalogue format 8 the form is offered no language.
	none := newEnv(t)
	none.rules.EXPECT().RaceAllows(mock.Anything, mock.Anything).Return(true, nil).Maybe()
	expectOptionTables(none)
	o, err = none.svc.CreationOptions(context.Background())
	require.NoError(t, err)
	assert.Empty(t, o.Languages)
	assert.Empty(t, o.RaceLanguages)
}

// ADR 0032 step 8.1 during a rolling deploy: a free-text word the release before wrote after the
// migration is not a code, so a read leaves it out; the codes stay, and without a catalogue
// (before format 8) nothing is left out.
func TestDetails_Step8_1_AWordThatIsNoCatalogueCodeIsNotRead(t *testing.T) {
	d := charactersvc.Details{Languages: []string{"elvish", "Elfique", "common", "Orque"}}
	assert.Equal(t, []string{"elvish", "common"}, d.Known(phbLanguages(t)).Languages)
	assert.Equal(t, d.Languages, d.Known((*ruletablesvc.Languages)(nil)).Languages)
	assert.Equal(t, []string{"elvish", "Elfique", "common", "Orque"}, d.Languages, "the details are not changed in place")
}

// ADR 0037 step 6 in the creation's checks: the draft's languages beyond the race's native one and
// Common spend the pool the proficiency step shows (a level 1 half-elf bard, Intelligence 9: 3 + 0 +
// 2 = 5 slots; Tumbling and two languages spend 3).
func TestCreationChecks_Step6_TheDraftLanguagesSpendThePool(t *testing.T) {
	e := newEnvWithLanguages(t)
	e.rules.EXPECT().LevelFor("bard", 0).Return(1, nil).Once()
	got, err := e.svc.CreationChecks(context.Background(), charactersvc.DraftInput{
		Class: "bard", Scores: map[string]int{"int": 9}, XP: ptr(0), Race: "half_elf",
		Languages: []string{"common", "elvish", "orcish"}, Proficiencies: picks(p("tumbling", 1)),
	})
	require.NoError(t, err)
	require.NotNil(t, got.ProficiencySlots)
	pool := got.ProficiencySlots.Nonweapon
	assert.Equal(t, 2, pool.Languages)
	assert.Equal(t, 3, pool.Spent)
	assert.Equal(t, 5, pool.Available)
}
