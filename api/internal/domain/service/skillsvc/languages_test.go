package skillsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// phbLanguages is a small language catalogue with the PHB's elf, dwarf and human lists.
func phbLanguages(t *testing.T) *ruletablesvc.Languages {
	t.Helper()
	var list []ruletablesvc.Language
	for _, c := range []string{"common", "dwarvish", "elvish", "gnomish", "halfling", "goblin", "hobgoblin", "orcish", "gnoll", "burrowers"} {
		list = append(list, ruletablesvc.Language{Code: c, Name: localized.Text{"en": c}})
	}
	l, err := ruletablesvc.NewLanguages(list, []ruletablesvc.RaceLanguageRow{
		{Race: "human", Language: "common", Native: true},
		{Race: "dwarf", Language: "dwarvish", Native: true},
		{Race: "elf", Language: "elvish", Native: true},
		{Race: "elf", Language: "common"},
		{Race: "gnome", Language: "gnomish", Native: true},
		{Race: "half_elf", Language: "common", Native: true},
		{Race: "halfling", Language: "halfling", Native: true},
	})
	require.NoError(t, err)
	return l
}

// elf is an elf bard of Intelligence 9 at realized level 1 (5 non-weapon slots) with picks
// spending slots and the languages as a read shows them (the native one among them).
func elf(slots int, languages ...string) *charactersvc.Character {
	return &charactersvc.Character{
		ID: charID, Class: "bard", Race: "elf", Int: 9, Dex: 15, RealizedLevel: 1,
		Proficiencies: nonweapon(pk("tumbling", slots)),
		Details:       charactersvc.Details{Languages: languages},
		Derived:       charactersvc.Derived{RealizedLevel: 1},
	}
}

// ADR 0037 steps 6, 10 and 18 on the Skills tab's languages pool: the languages beyond the native
// one and Common spend the non-weapon pool under its budget rule. A save that newly overspends or
// grows an overspend is refused on field languages (409 over_budget), nothing written or logged;
// dropping one from an overspend is stored; the same languages reordered log nothing; the preview
// refuses as the save does.
func TestLanguages_Step6_EditCheckedByTheBudget(t *testing.T) {
	langs := func(ls ...string) skillsvc.Edit { return skillsvc.Edit{Languages: ls} }
	t.Run("a new overspend is refused", func(t *testing.T) {
		f := newSheetFixture(t, elf(3, "elvish", "common", "gnomish", "orcish"), phbLanguages(t)) // 3 + 2 = 5 of 5
		for _, save := range []func() error{
			func() error {
				_, err := f.svc.SetProficiencies(context.Background(), charID, langs("elvish", "common", "gnomish", "orcish", "goblin"))
				return err
			},
			func() error {
				_, err := f.svc.PreviewProficiencies(context.Background(), charID, langs("elvish", "common", "gnomish", "orcish", "goblin"))
				return err
			},
		} {
			err := save()
			require.ErrorIs(t, err, domain.ErrOverBudget)
			var be *domain.BudgetError
			require.ErrorAs(t, err, &be)
			assert.Equal(t, domain.BudgetError{Field: "languages", Spent: 6, Limit: 5}, *be)
		}
	})
	t.Run("an overspend that shrinks is stored", func(t *testing.T) {
		f := newSheetFixture(t, elf(4, "elvish", "common", "gnomish", "orcish"), phbLanguages(t)) // 4 + 2 = 6 of 5
		f.records()
		f.picks.EXPECT().SetLanguages(mock.Anything, charID, []string{"elvish", "common", "gnomish"}).Return(nil).Once()
		got, err := f.svc.SetProficiencies(context.Background(), charID, langs("elvish", "common", "gnomish"))
		require.NoError(t, err)
		assert.Equal(t, 1, got.Slots.Nonweapon.Languages)
		assert.Equal(t, 5, got.Slots.Nonweapon.Spent)
		assert.Len(t, f.entries, 1)
	})
	t.Run("the same languages reordered log nothing", func(t *testing.T) {
		f := newSheetFixture(t, elf(1, "elvish", "common", "gnomish"), phbLanguages(t)) // the picks and log mocks fail any write
		_, err := f.svc.SetProficiencies(context.Background(), charID, langs("gnomish", "common", "elvish"))
		require.NoError(t, err)
	})
	t.Run("a picks save that grows over the pool the languages share is refused on nonweapon", func(t *testing.T) {
		f := newSheetFixture(t, elf(3, "elvish", "common", "gnomish", "orcish"), phbLanguages(t))
		_, err := f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Picks: nonweapon(pk("tumbling", 4))})
		var be *domain.BudgetError
		require.ErrorAs(t, err, &be)
		assert.Equal(t, domain.BudgetError{Field: "nonweapon", Spent: 6, Limit: 5}, *be)
	})
}

// ADR 0032 steps 4.1 and 4.3 on the Skills tab's save (ADR 0037 step 18): the race's native
// language is added by the server and cannot be removed (in front when missing, never doubled);
// any catalogue language is accepted, outside the race's list too; a blank, an unknown code and a
// repeat are refused on field languages, nothing written.
func TestSetLanguages_Step4_TheNativeLanguageIsKeptAndCodesChecked(t *testing.T) {
	for _, tt := range []struct {
		name string
		sent []string
		want []string
	}{
		{"the native is added in front", []string{"gnomish"}, []string{"elvish", "gnomish"}},
		{"removing everything keeps the native", []string{}, []string{"elvish"}},
		{"the native sent is not doubled", []string{"gnomish", "elvish"}, []string{"gnomish", "elvish"}},
		{"a language outside the race's list", []string{"elvish", "burrowers"}, []string{"elvish", "burrowers"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newSheetFixture(t, elf(1, "elvish", "common"), phbLanguages(t))
			f.records()
			f.picks.EXPECT().SetLanguages(mock.Anything, charID, tt.want).Return(nil).Once()
			_, err := f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Languages: tt.sent})
			require.NoError(t, err)
		})
	}
	for _, tt := range []struct {
		name string
		sent []string
		code string
	}{
		{"blank", []string{"common", ""}, "empty"},
		{"free text, not a code", []string{"Elfique"}, "unknown"},
		{"a code of no catalogue language", []string{"draconic"}, "unknown"},
		{"a repeat", []string{"dwarvish", "dwarvish"}, "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newSheetFixture(t, elf(1, "elvish", "common"), phbLanguages(t)) // the picks and log mocks fail any write
			_, err := f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Languages: tt.sent})
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, domain.FieldError{Field: "languages", Message: tt.code}, *fe)
		})
	}
}

// ADR 0032 step 8.1: before catalogue format 8 there is no language book; the codes the character
// holds stay valid in a save (refusing them would lock it), any other is unknown.
func TestSetLanguages_Step8_1_OnAnEmptyCatalogueTheStoredCodesSurvive(t *testing.T) {
	for _, book := range []skills.LanguageBook{nil, (*ruletablesvc.Languages)(nil)} {
		f := newSheetFixture(t, elf(1, "elvish", "common"), book)
		_, err := f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Languages: []string{"common", "elvish"}})
		require.NoError(t, err, "the same codes, reordered: nothing written")
		_, err = f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Languages: []string{"elvish", "draconic"}})
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, domain.FieldError{Field: "languages", Message: "unknown"}, *fe)
	}
}

// ADR 0037 steps 13 and 18: a save of the picks and the languages is one proficiencies entry
// whose payload and snapshot carry both, before and after; the restorer writes both back (undo
// the before, redo the after), and a snapshot logged before the languages moved (no "languages"
// key) leaves the stored languages.
func TestSetPicks_Step13_LanguagesInTheOneMove(t *testing.T) {
	f := newSheetFixture(t, elf(1, "elvish", "common"), phbLanguages(t))
	f.records()
	next := nonweapon(pk("tumbling", 2))
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, next).Return(nil).Once()
	f.picks.EXPECT().SetLanguages(mock.Anything, charID, []string{"elvish", "common", "gnomish"}).Return(nil).Once()
	_, err := f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Picks: next, Languages: []string{"elvish", "common", "gnomish"}})
	require.NoError(t, err)
	assert.Equal(t, 1, f.tx.calls, "one transaction")
	require.Len(t, f.entries, 1, "one log entry")
	e := f.entries[0]
	assert.JSONEq(t, `{"before":{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[],"languages":["elvish","common"]},
		"after":{"nonweapon":[{"id":"tumbling","slots":2}],"weapon":[],"languages":["elvish","common","gnomish"]}}`, string(e.Payload))
	require.Len(t, e.Snapshots, 1)
	assert.JSONEq(t, `{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[],"languages":["elvish","common"]}`, snaptest.State(t, e.Snapshots[0].Before))

	r := f.svc.Module().Restorer(skillsvc.PicksStateKey)
	require.NotNil(t, r)
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, nonweapon(pk("tumbling", 1))).Return(nil).Once()
	f.picks.EXPECT().SetLanguages(mock.Anything, charID, []string{"elvish", "common", "gnomish"}).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[],"languages":["elvish","common","gnomish"]}`)))

	// A picks-only entry (the same languages as stored on both sides): the column is not
	// rewritten, so a hidden free-text word (ADR 0032 step 8.1) survives the undo (no SetLanguages).
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, nonweapon(pk("tumbling", 1))).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[],"languages":["common","elvish"]}`)))

	// An entry logged before: the picks only, the languages left as stored (no SetLanguages).
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, nonweapon(pk("tumbling", 1))).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[]}`)))
}
