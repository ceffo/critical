package charactersvc

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
)

// The bounds of the details' fields. The one validation of the details: the
// card's edit (detailssvc) and the creation form (Create) both apply a
// DetailsPatch through Details.Apply.
const (
	MaxDetailShortLen   = 40 // race, alignment, sex, hair, eyes
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
)

// DetailsField prefixes the field of a details refusal at creation, where
// "race" already names the race code: "details.age", "details.languages".
const DetailsField = "details."

// DetailsPatch holds the details to change, nil leaves a field as it is. An
// empty string clears a text field, an empty list the languages and ClearAge
// the age (Age itself must be 1 to MaxAge).
type DetailsPatch struct {
	Race      *string
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

// Apply is d with the patch applied, every field checked: text trimmed and
// bounded, the age 1 to MaxAge, the languages trimmed, non-empty, bounded and
// without duplicates (case-insensitive). A refusal is a *domain.FieldError
// naming the field (race, alignment, age, sex, height, weight, hair, eyes or
// languages).
func (d Details) Apply(p DetailsPatch) (Details, error) {
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
		{"race", &cur.Race, p.Race, MaxDetailShortLen},
		{"alignment", &cur.Alignment, p.Alignment, MaxDetailShortLen},
		{"sex", &cur.Sex, p.Sex, MaxDetailShortLen},
		{"height", &cur.Height, p.Height, MaxDetailMeasureLen},
		{"weight", &cur.Weight, p.Weight, MaxDetailMeasureLen},
		{"hair", &cur.Hair, p.Hair, MaxDetailShortLen},
		{"eyes", &cur.Eyes, p.Eyes, MaxDetailShortLen},
	} {
		if err := text(f.name, f.dst, f.v, f.limit); err != nil {
			return d, err
		}
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

// newDetails are a new character's details: none, with p applied. A refusal
// names its field under DetailsField.
func newDetails(p DetailsPatch) (Details, error) {
	d, err := Details{Languages: []string{}}.Apply(p)
	if fe, ok := errors.AsType[*domain.FieldError](err); ok {
		return Details{}, &domain.FieldError{Field: DetailsField + fe.Field, Message: fe.Message}
	}
	return d, err
}
