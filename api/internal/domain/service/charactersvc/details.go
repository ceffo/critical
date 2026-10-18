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
	MaxLanguages        = 20
	MaxLanguageLen      = 30
)

// Error codes of the details' FieldErrors besides CodeTooLong, CodeOutOfRange
// and CodeEmpty. The web owns the wording.
const (
	CodeTooMany   = "too_many"
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

// The fields a character may lack (Details.Missing).
const (
	FieldAlignment = "alignment"
	FieldSex       = "sex"
)

// DetailsField prefixes the field of a details refusal at creation, where
// "race" already names the race code: "details.age", "details.languages".
const DetailsField = "details."

// DetailsPatch holds the details to change, nil leaves a field as it is. An
// empty string clears a text field, an empty list the languages and ClearAge
// the age (Age itself must be 1 to MaxAge). Alignment and Sex are codes and
// cannot be cleared: an empty one is refused.
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

// Missing lists the required fields the details leave empty (ADR 0032 step
// 8.2), in the card's order; a migrated character may lack them.
func (d Details) Missing() []string {
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
// 0032 step 8.4): a code and the field it is about.
type Warning struct {
	Code  string
	Field string
}

// Warnings are the warnings the details of a character of class carry: the
// alignment its class no longer admits, kept because it is the player's own
// recorded choice.
func (d Details) Warnings(class string) []Warning {
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

// Apply is d with the patch applied for a character of class, every field
// checked: the alignment one of the nine, admitted by the class (ADR 0032 step
// 2, on every set, at creation and on the card's edit alike), the sex male or
// female (step 3), text trimmed and bounded, the age 1 to MaxAge, the
// languages trimmed, non-empty, bounded and without duplicates
// (case-insensitive). A refusal is a *domain.FieldError naming the field
// (alignment, age, sex, height, weight, hair, eyes or languages).
func (d Details) Apply(class string, p DetailsPatch) (Details, error) {
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
		if !ruletablesvc.AlignmentAllowed(class, a) {
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
		langs, err := cleanLanguages(*p.Languages)
		if err != nil {
			return d, err
		}
		cur.Languages = langs
	}
	return cur, nil
}

func cleanLanguages(in []string) ([]string, error) {
	if len(in) > MaxLanguages {
		return nil, &domain.FieldError{Field: "languages", Message: CodeTooMany}
	}
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		l := strings.TrimSpace(raw)
		switch {
		case l == "":
			return nil, &domain.FieldError{Field: "languages", Message: CodeEmpty}
		case utf8.RuneCountInString(l) > MaxLanguageLen:
			return nil, &domain.FieldError{Field: "languages", Message: CodeTooLong}
		case seen[strings.ToLower(l)]:
			return nil, &domain.FieldError{Field: "languages", Message: CodeDuplicate}
		}
		seen[strings.ToLower(l)] = true
		out = append(out, l)
	}
	return out, nil
}

// newDetails are a new character's details of class: none, with p applied,
// the alignment and the sex required (ADR 0032 steps 2 and 3). A refusal names
// its field under DetailsField.
func newDetails(class string, p DetailsPatch) (Details, error) {
	d, err := Details{Languages: []string{}}.Apply(class, p)
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
