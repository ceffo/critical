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

// newEnvWithLanguages is newEnv with the PHB language catalogue.
func newEnvWithLanguages(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	svc, err := charactersvc.New(charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: e.rules, Owners: e.owners, Tx: e.tx, Sessions: e.sessions, Log: e.log,
		Languages: phbLanguages(t),
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

// ADR 0032 step 4.2: PHB Table 4's "# of Lang." by Intelligence score: score 1: none; 2 to 8: 1; 9
// to 11: 2; 12 to 13: 3; 14 to 15: 4; 16: 5; 17: 6; 18: 7; 19 and up as the table. The native
// language is not counted; going over is a warning with count and limit, never a refusal, and the
// same details at another Intelligence warn or not (the warning is read again after the score is
// edited below the languages known).
func TestLanguageWarning_Step4_2_OverTheTable4CountIsAWarningAndTheNativeIsNotCounted(t *testing.T) {
	for score, limit := range map[int]int{
		1: 0, 2: 1, 5: 1, 8: 1, 9: 2, 11: 2, 12: 3, 13: 3, 14: 4, 15: 4, 16: 5, 17: 6, 18: 7, 19: 8, 20: 9, 21: 10, 22: 11, 23: 12, 24: 15, 25: 20,
	} {
		assert.Equal(t, limit, ruletablesvc.LanguageLimit(score), "Intelligence %d", score)
	}

	// An elf: elvish is native. Five additional languages.
	five := charactersvc.Details{Languages: []string{"elvish", "common", "gnomish", "halfling", "goblin", "orcish"}}
	_, over := five.LanguageWarning(16, "elvish")
	assert.False(t, over, "five additional languages at Intelligence 16 (limit 5): within the count, the native not counted")

	six := charactersvc.Details{Languages: append(five.Languages[:6:6], "gnoll")}
	w, over := six.LanguageWarning(16, "elvish")
	require.True(t, over, "six additional at Intelligence 16 warns")
	assert.Equal(t, charactersvc.CodeLanguagesOverLimit, w.Code)
	assert.Equal(t, "languages", w.Field)
	assert.Equal(t, 6, *w.Count)
	assert.Equal(t, 5, *w.Limit)

	// The score edited below the languages known: the same details read at Intelligence 8.
	w, over = five.LanguageWarning(8, "elvish")
	require.True(t, over)
	assert.Equal(t, 5, *w.Count)
	assert.Equal(t, 1, *w.Limit)
	_, over = five.LanguageWarning(18, "elvish")
	assert.False(t, over)

	// Intelligence 1: no additional language at all.
	w, over = charactersvc.Details{Languages: []string{"elvish", "common"}}.LanguageWarning(1, "elvish")
	require.True(t, over)
	assert.Equal(t, 0, *w.Limit, "a limit of 0 is still given")
}

// ADR 0032 step 4.2 at creation: going over the count is accepted, never refused.
func TestCreate_Step4_2_GoingOverTheCountIsNotRefused(t *testing.T) {
	// Lyra has Intelligence 14: four additional languages. Five are sent.
	got, err := createdDetails(t, "elf", &[]string{"common", "gnomish", "halfling", "goblin", "orcish"})
	require.NoError(t, err)
	assert.Len(t, got.Languages, 6)
}

// ADR 0032 steps 4.1 and 4.2 on a read: the character carries its native language (added when the
// stored languages lack it), and the warning with the count and limit when it knows more than its
// Intelligence allows.
func TestGetCharacter_Step4_TheReadAddsTheNativeLanguageAndWarnsOverTheCount(t *testing.T) {
	c := kelsen() // a half-elf bard, Intelligence 14: four additional languages
	c.Details = charactersvc.Details{Alignment: "neutral_good", Sex: "male", Languages: []string{"elvish", "gnomish", "halfling", "goblin", "orcish"}}
	e := newEnvWithLanguages(t)
	e.reader.EXPECT().GetCharacter(mock.Anything, int64(3)).Return(c, nil).Once()
	e.xp.EXPECT().EntriesTotal(mock.Anything, int64(3)).Return(0, nil).Once()
	expectKelsenRules(e.rules, 1_293_418, 15)

	got, err := e.svc.GetCharacter(context.Background(), 3)
	require.NoError(t, err)
	assert.Equal(t, []string{"common", "elvish", "gnomish", "halfling", "goblin", "orcish"}, got.Details.Languages,
		"Common, a half-elf's native language, is added")
	require.Len(t, got.Warnings, 1)
	assert.Equal(t, charactersvc.CodeLanguagesOverLimit, got.Warnings[0].Code)
	assert.Equal(t, 5, *got.Warnings[0].Count)
	assert.Equal(t, 4, *got.Warnings[0].Limit)
}

// ADR 0032 step 4.3: the creation options hold the whole catalogue with its names, each race's own
// language and PHB list (the form marks it), and Table 4 as bands of Intelligence scores.
func TestCreationOptions_Step4_OfferTheCatalogueTheRaceListsAndTheLimits(t *testing.T) {
	e := newEnvWithLanguages(t)
	e.rules.EXPECT().RaceAllows(mock.Anything, mock.Anything).Return(true, nil).Maybe()
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
