package charactersvc

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The bounds of the details' fields. The one validation of the details: the
// card's edit (detailssvc) and the creation form (Create) both apply a
// DetailsPatch through Details.Apply.
const (
	MaxDetailShortLen   = 40 // hair, eyes
	MaxDetailMeasureLen = 20 // height, weight
	MaxAge              = 10_000
)

// Error codes of the details' FieldErrors besides CodeTooLong, CodeOutOfRange
// and CodeEmpty. The web owns the wording.
const (
	CodeDuplicate = "duplicate"
	// CodeAlignmentNotAllowed is an alignment the class does not admit (ADR
	// 0032 step 2), refused when it is set and read as a warning on a
	// character that already holds it (step 8.4).
	CodeAlignmentNotAllowed = "alignment_not_allowed"
)

// The sex codes (ADR 0032 step 3): the two rows of PHB Table 10.
const (
	SexMale   = "male"
	SexFemale = "female"
)

// Sexes are the sex codes.
var Sexes = []string{SexMale, SexFemale}

// The fields a character may lack (Details.Missing) or a warning is about.
const (
	FieldAlignment = "alignment"
	FieldSex       = "sex"
	FieldLanguages = "languages"
)

// CodeLanguagesOverLimit is the warning of a character that knows more
// languages than PHB Table 4 gives its Intelligence (ADR 0032 step 4.2): never
// a refusal, the DM decides.
const CodeLanguagesOverLimit = "languages_over_limit"

// LanguageBook is the language catalogue the details are checked against (ADR
// 0032 step 4); ruletablesvc.Languages satisfies it, the empty one included.
type LanguageBook interface {
	// Has reports whether code is a catalogue language.
	Has(code string) bool
	// Native is the race's own language, "" without a catalogue.
	Native(race string) string
	// All are the catalogue's languages and RaceList a race's PHB list (the
	// native language included), for the creation options.
	All() []ruletablesvc.Language
	RaceList(race string) []string
}

// Kind is the class and race code of the character whose details are checked:
// the alignment limit depends on the class and the native language on the race.
type Kind struct {
	Class, Race string
}

// DetailsField prefixes the field of a details refusal at creation, where
// "race" already names the race code: "details.age", "details.languages".
const DetailsField = "details."

// DetailsPatch holds the details to change, nil leaves a field as it is. An
// empty string clears a text field, an empty list the languages (but the
// native one, which the server keeps, ADR 0032 step 4.1) and ClearAge the age
// (Age itself must be 1 to MaxAge). Alignment and Sex are codes and cannot be
// cleared: an empty one is refused. Languages are catalogue codes.
type DetailsPatch struct {
	Alignment *string
	Age       *int
	ClearAge  bool
	Sex       *string
	Height    *string
	Weight    *string
	Hair      *string
	Eyes      *string
	Languages *[]string
}

// Coded is d with an alignment or sex that is not a code blanked: during a
// rolling deploy (ADR 0024) the old release keeps writing free text there
// after the identity migration ran, and such a value reads as missing, never
// as a code (ADR 0032 step 8.2).
func (d Details) Coded() Details {
	if !slices.Contains(ruletablesvc.Alignments, d.Alignment) {
		d.Alignment = ""
	}
	if !slices.Contains(Sexes, d.Sex) {
		d.Sex = ""
	}
	return d
}

// Missing lists the required fields the details leave empty or hold as a
// non-code (ADR 0032 step 8.2), in the card's order; a migrated character may
// lack them.
func (d Details) Missing() []string {
	d = d.Coded()
	out := []string{}
	if d.Alignment == "" {
		out = append(out, FieldAlignment)
	}
	if d.Sex == "" {
		out = append(out, FieldSex)
	}
	return out
}

// Warning is a rule the stored character breaks without being refused (ADR
// 0032 steps 4.2 and 8.4): a code and the field it is about, and for the
// languages over the limit the figures the web words (Count known, Limit
// allowed); nil for the others.
type Warning struct {
	Code         string
	Field        string
	Count, Limit *int
}

// Known is d with the languages the book does not hold left out: during a
// rolling deploy (ADR 0024) the release before keeps writing free-text words
// after the languages migration ran, and a word that is no catalogue code is
// not shown (ADR 0032 step 8.1). Without a catalogue (every book is empty
// before format 8) nothing is left out, so the codes stay.
func (d Details) Known(book LanguageBook) Details {
	d.Languages = append([]string{}, d.Languages...)
	if book == nil || len(book.All()) == 0 {
		return d
	}
	d.Languages = slices.DeleteFunc(d.Languages, func(l string) bool { return !book.Has(l) })
	return d
}

// WithNative is d with the race's native language among its languages, first
// when it was missing (ADR 0032 step 4.1); native "" (no catalogue) changes
// nothing.
func (d Details) WithNative(native string) Details {
	d.Languages = append([]string{}, d.Languages...)
	if native != "" && !slices.Contains(d.Languages, native) {
		d.Languages = append([]string{native}, d.Languages...)
	}
	return d
}

// LanguageWarning is ADR 0032 step 4.2: the additional languages (the native
// one not counted) beyond PHB Table 4's number for the Intelligence score. ok
// is false while within it. It is read again whenever the score or the
// languages change, so an Intelligence edited below the languages known warns.
func (d Details) LanguageWarning(intelligence int, native string) (w Warning, ok bool) {
	count := 0
	for _, l := range d.Languages {
		if l != native {
			count++
		}
	}
	limit := ruletablesvc.LanguageLimit(intelligence)
	if count <= limit {
		return Warning{}, false
	}
	return Warning{Code: CodeLanguagesOverLimit, Field: FieldLanguages, Count: &count, Limit: &limit}, true
}

// Warnings are the warnings the details of a character of class carry: the
// alignment its class no longer admits, kept because it is the player's own
// recorded choice.
func (d Details) Warnings(class string) []Warning {
	d = d.Coded()
	out := []Warning{}
	if d.Alignment != "" && !ruletablesvc.AlignmentAllowed(class, d.Alignment) {
		out = append(out, Warning{Code: CodeAlignmentNotAllowed, Field: FieldAlignment})
	}
	return out
}

// code checks v is one of the allowed codes of field: CodeEmpty, CodeUnknown.
func code(field, v string, allowed []string) error {
	switch {
	case v == "":
		return &domain.FieldError{Field: field, Message: CodeEmpty}
	case !slices.Contains(allowed, v):
		return &domain.FieldError{Field: field, Message: CodeUnknown}
	}
	return nil
}

// Apply is d with the patch applied for a character of kind, every field
// checked: the alignment one of the nine, admitted by the class (ADR 0032 step
// 2, on every set, at creation and on the card's edit alike), the sex male or
// female (step 3), text trimmed and bounded, the age 1 to MaxAge, the
// languages codes of the book without duplicates (step 4.3), the race's
// native language added to them (step 4.1); going over the Intelligence count
// is no refusal (step 4.2, LanguageWarning). A refusal is a *domain.FieldError
// naming the field (alignment, age, sex, height, weight, hair, eyes or
// languages).
func (d Details) Apply(k Kind, book LanguageBook, p DetailsPatch) (Details, error) {
	class := k.Class
	cur := d
	cur.Languages = append([]string{}, d.Languages...)
	text := func(field string, dst, v *string, limit int) error {
		if v == nil {
			return nil
		}
		t := strings.TrimSpace(*v)
		if utf8.RuneCountInString(t) > limit {
			return &domain.FieldError{Field: field, Message: CodeTooLong}
		}
		*dst = t
		return nil
	}
	for _, f := range []struct {
		name  string
		dst   *string
		v     *string
		limit int
	}{
		{"height", &cur.Height, p.Height, MaxDetailMeasureLen},
		{"weight", &cur.Weight, p.Weight, MaxDetailMeasureLen},
		{"hair", &cur.Hair, p.Hair, MaxDetailShortLen},
		{"eyes", &cur.Eyes, p.Eyes, MaxDetailShortLen},
	} {
		if err := text(f.name, f.dst, f.v, f.limit); err != nil {
			return d, err
		}
	}
	if p.Alignment != nil {
		a := strings.TrimSpace(*p.Alignment)
		if err := code(FieldAlignment, a, ruletablesvc.Alignments); err != nil {
			return d, err
		}
		// A kept forbidden alignment (step 8.4) stays: only a change to one is refused.
		if a != d.Alignment && !ruletablesvc.AlignmentAllowed(class, a) {
			return d, &domain.FieldError{Field: FieldAlignment, Message: CodeAlignmentNotAllowed}
		}
		cur.Alignment = a
	}
	if p.Sex != nil {
		sex := strings.TrimSpace(*p.Sex)
		if err := code(FieldSex, sex, Sexes); err != nil {
			return d, err
		}
		cur.Sex = sex
	}
	if p.ClearAge {
		cur.Age = 0
	}
	if p.Age != nil {
		if *p.Age < 1 || *p.Age > MaxAge {
			return d, &domain.FieldError{Field: "age", Message: CodeOutOfRange}
		}
		cur.Age = *p.Age
	}
	if p.Languages != nil {
		langs, err := cleanLanguages(book, *p.Languages)
		if err != nil {
			return d, err
		}
		cur.Languages = langs
		if book != nil {
			cur = cur.WithNative(book.Native(k.Race))
		}
	}
	return cur, nil
}

// cleanLanguages checks the codes against the book: non-empty (CodeEmpty), in
// the catalogue (CodeUnknown), each once (CodeDuplicate). A nil book is the
// empty catalogue, which knows no language.
func cleanLanguages(book LanguageBook, in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, l := range in {
		switch {
		case l == "":
			return nil, &domain.FieldError{Field: FieldLanguages, Message: CodeEmpty}
		case book == nil || !book.Has(l):
			return nil, &domain.FieldError{Field: FieldLanguages, Message: CodeUnknown}
		case seen[l]:
			return nil, &domain.FieldError{Field: FieldLanguages, Message: CodeDuplicate}
		}
		seen[l] = true
		out = append(out, l)
	}
	return out, nil
}

// newDetails are a new character's details of kind: none, with p applied,
// the alignment and the sex required (ADR 0032 steps 2 and 3). A refusal names
// its field under DetailsField.
func newDetails(k Kind, book LanguageBook, p DetailsPatch) (Details, error) {
	// A new character always gets its native language (ADR 0032 step 4.1),
	// whether or not the form sent languages.
	if p.Languages == nil {
		p.Languages = &[]string{}
	}
	d, err := Details{Languages: []string{}}.Apply(k, book, p)
	if err == nil && d.Alignment == "" {
		err = &domain.FieldError{Field: FieldAlignment, Message: CodeEmpty}
	}
	if err == nil && d.Sex == "" {
		err = &domain.FieldError{Field: FieldSex, Message: CodeEmpty}
	}
	if fe, ok := errors.AsType[*domain.FieldError](err); ok {
		return Details{}, &domain.FieldError{Field: DetailsField + fe.Field, Message: fe.Message}
	}
	return d, err
}

// languageLimitBands are PHB Table 4's language counts as runs of equal
// scores (ADR 0032 step 4.2), over the scores the table gives.
func languageLimitBands() []LanguageLimitBand {
	const lowest, highest = 1, 25
	var out []LanguageLimitBand
	for score := lowest; score <= highest; score++ {
		limit := ruletablesvc.LanguageLimit(score)
		if n := len(out); n > 0 && out[n-1].Limit == limit {
			out[n-1].IntMax = score
			continue
		}
		out = append(out, LanguageLimitBand{IntMin: score, IntMax: score, Limit: limit})
	}
	return out
}
