// Package detailssvc edits the character's free-text details (race,
// alignment, age, sex, height, weight, hair, eyes and languages). The server
// validates every field and names the rejected one in a *domain.FieldError
// whose message is a code the web maps to a localized text.
package detailssvc

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// Input bounds.
const (
	MaxShortLen    = 40 // race, alignment, sex, hair, eyes
	MaxMeasureLen  = 20 // height, weight
	MaxAge         = 10_000
	MaxLanguages   = 20
	MaxLanguageLen = 30
)

// Error codes carried by the FieldError messages. The web owns the wording.
const (
	CodeTooLong    = "too_long"
	CodeOutOfRange = "out_of_range"
	CodeTooMany    = "too_many"
	CodeEmpty      = "empty"
	CodeDuplicate  = "duplicate"
)

// Patch holds the fields to change, nil leaves a field as it is. An empty
// string clears a text field, an empty list the languages and ClearAge the
// age (Age itself must be 1 to MaxAge).
type Patch struct {
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

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Update applies the patch and returns the details as stored. A
	// *domain.FieldError for invalid input, ErrCharacterNotFound.
	Update(ctx context.Context, characterID int64, p Patch) (charactersvc.Details, error)
}

// DAL is the driven port. Both methods join the transaction carried by ctx.
type DAL interface {
	GetDetails(ctx context.Context, id int64) (charactersvc.Details, error)
	UpdateDetails(ctx context.Context, id int64, v charactersvc.Details) error
}

type service struct {
	dal DAL
	tx  domain.TxRunner
}

// New constructs the service.
func New(dal DAL, tx domain.TxRunner) (SVC, error) {
	if dal == nil || tx == nil {
		return nil, fmt.Errorf("detailssvc: nil dependency")
	}
	return &service{dal: dal, tx: tx}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

func (s *service) Update(ctx context.Context, id int64, p Patch) (charactersvc.Details, error) {
	var out charactersvc.Details
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		cur, err := s.dal.GetDetails(ctx, id)
		if err != nil {
			return err
		}
		next, err := apply(cur, p)
		if err != nil {
			return err
		}
		if err := s.dal.UpdateDetails(ctx, id, next); err != nil {
			return err
		}
		out = next
		return nil
	})
	return out, err
}

func apply(cur charactersvc.Details, p Patch) (charactersvc.Details, error) {
	text := func(field string, dst *string, v *string, limit int) error {
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
		{"race", &cur.Race, p.Race, MaxShortLen},
		{"alignment", &cur.Alignment, p.Alignment, MaxShortLen},
		{"sex", &cur.Sex, p.Sex, MaxShortLen},
		{"height", &cur.Height, p.Height, MaxMeasureLen},
		{"weight", &cur.Weight, p.Weight, MaxMeasureLen},
		{"hair", &cur.Hair, p.Hair, MaxShortLen},
		{"eyes", &cur.Eyes, p.Eyes, MaxShortLen},
	} {
		if err := text(f.name, f.dst, f.v, f.limit); err != nil {
			return cur, err
		}
	}
	if p.ClearAge {
		cur.Age = 0
	}
	if p.Age != nil {
		if *p.Age < 1 || *p.Age > MaxAge {
			return cur, &domain.FieldError{Field: "age", Message: CodeOutOfRange}
		}
		cur.Age = *p.Age
	}
	if p.Languages != nil {
		langs, err := cleanLanguages(*p.Languages)
		if err != nil {
			return cur, err
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
