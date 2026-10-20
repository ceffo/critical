package skills

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The languages: a pool of picks of the non-weapon slots (ADR 0037 steps 6 and 18), stored as ADR
// 0032 step 4's list of catalogue codes.

// LanguageSlots is ADR 0037 step 6: the non-weapon slots a list of languages spends, one for
// each language other than the race's native one and Common. Without a catalogue (native "")
// no language is known to be free, so none is counted.
func LanguageSlots(languages []string, native string) int {
	if native == "" {
		return 0
	}
	n := 0
	for _, l := range languages {
		if l != native && l != ruletablesvc.CommonLanguage {
			n++
		}
	}
	return n
}

// KnownLanguages is languages with the codes the book does not hold left out, in a new slice:
// during a rolling deploy (ADR 0024) the release before keeps writing free-text words after the
// languages migration ran, and a word that is no catalogue code is not shown (ADR 0032 step 8.1).
// Without a catalogue (every book is empty before format 8) nothing is left out, so the codes stay.
func KnownLanguages(book LanguageBook, languages []string) []string {
	out := append([]string{}, languages...)
	if book == nil || len(book.All()) == 0 {
		return out
	}
	return slices.DeleteFunc(out, func(l string) bool { return !book.Has(l) })
}

// ValidLanguages is ADR 0032 step 4's check on a list of languages, at creation and on every
// save: each code non-empty (CodeEmpty), in the catalogue (CodeUnknown), once (CodeDuplicate),
// a refusal being a *domain.FieldError on field languages; then the native language of the
// character's race is added, first, when it is missing (step 4.1: the server keeps it). A nil
// or empty book (every catalogue before format 8) knows no language, but the codes the
// character already holds (stored) stay valid there: refusing them would lock the edit (step
// 8.1). Going over the slots is the budget's (PicksWithinBudget), not this check's. Never nil.
func ValidLanguages(book LanguageBook, race string, stored, in []string) ([]string, error) {
	empty := book == nil || len(book.All()) == 0
	out := make([]string, 0, len(in)+1)
	seen := map[string]bool{}
	for _, l := range in {
		switch {
		case l == "":
			return nil, &domain.FieldError{Field: FieldLanguages, Message: CodeEmpty}
		case (book == nil || !book.Has(l)) && (!empty || !slices.Contains(stored, l)):
			return nil, &domain.FieldError{Field: FieldLanguages, Message: CodeUnknown}
		case seen[l]:
			return nil, &domain.FieldError{Field: FieldLanguages, Message: CodeDuplicate}
		}
		seen[l] = true
		out = append(out, l)
	}
	if book != nil {
		out = WithNative(out, book.Native(race))
	}
	return out, nil
}

// WithNative is languages with the race's native language among them, first when it was
// missing (ADR 0032 step 4.1); native "" (no catalogue) changes nothing. Never nil.
func WithNative(languages []string, native string) []string {
	out := append([]string{}, languages...)
	if native != "" && !slices.Contains(out, native) {
		out = append([]string{native}, out...)
	}
	return out
}

// LanguagesEqual reports the same languages, in any order (ADR 0037 step 13: a save that only
// reorders them changes nothing, as for the picks).
func LanguagesEqual(a, b []string) bool { return sameItems(a, b) }
