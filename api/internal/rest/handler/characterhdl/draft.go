package characterhdl

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

type drafter interface {
	Draft(ctx context.Context, n charactersvc.NewCharacter, blank []string) (*charactersvc.Draft, error)
}

// RefusalDTO is one refusal POST /api/characters would answer the draft with: its `field`, and
// its `code`, the creation's `error` for an invalid input (out_of_range, race_not_allowed...) or
// over_budget with the budget's `spent` and `limit`.
type RefusalDTO struct {
	Field string `json:"field"`
	Code  string `json:"code"`
	Spent *int   `json:"spent,omitempty"`
	Limit *int   `json:"limit,omitempty"`
}

// DraftDTO is POST /api/characters/draft (ADR 0034, amendment of 2026-10-20): the sheet as it
// would read once created, in GET /api/character's shapes (`derived`, `warnings`, `languages`
// with the native one added; `derived` null and the lists empty while the class, race, scores,
// skill points or XP are missing or refused), a thief's or a bard's skill point budget
// (`skillPoints`, the derived block's shape, as soon as the class and XP read: a score out of
// range does not hide it; null otherwise), the creation's `refusals`, the required fields still blank
// (`missing`: name, class, race, the six scores, hpMax, xp, alignment, sex) and, for every
// creatable class, the abilities whose entered score is under its minimum (`unmetMinimums`).
type DraftDTO struct {
	Derived       *DerivedDTO                `json:"derived"`
	SkillPoints   *wire.SkillPointsBudgetDTO `json:"skillPoints"`
	Warnings      []WarningDTO               `json:"warnings"`
	Languages     []string                   `json:"languages"`
	Refusals      []RefusalDTO               `json:"refusals"`
	Missing       []string                   `json:"missing"`
	UnmetMinimums []ClassUnmetDTO            `json:"unmetMinimums"`
}

// refusalDTO maps one of the draft's refusals.
func refusalDTO(err error) (RefusalDTO, error) {
	if be, ok := errors.AsType[*domain.BudgetError](err); ok {
		return RefusalDTO{Field: be.Field, Code: render.CodeOverBudget, Spent: &be.Spent, Limit: &be.Limit}, nil
	}
	if fe, ok := errors.AsType[*domain.FieldError](err); ok {
		return RefusalDTO{Field: fe.Field, Code: fe.Message}, nil
	}
	return RefusalDTO{}, fmt.Errorf("draft refusal: %w", err)
}

// draftDTO maps a draft's answer, its lists never null.
func draftDTO(d *charactersvc.Draft) (DraftDTO, error) {
	out := DraftDTO{
		Warnings: []WarningDTO{}, Languages: []string{}, Refusals: make([]RefusalDTO, 0, len(d.Refusals)),
		Missing: append([]string{}, d.Missing...), UnmetMinimums: make([]ClassUnmetDTO, 0, len(d.UnmetMinimums)),
		SkillPoints: wire.SkillPointsBudgetToDTO(d.SkillPoints),
	}
	if c := d.Character; c != nil {
		derived := derivedDTO(c.Derived)
		out.Derived, out.Warnings = &derived, warningsDTO(c.Warnings)
		if c.Details.Languages != nil {
			out.Languages = c.Details.Languages
		}
	}
	for _, err := range d.Refusals {
		r, err := refusalDTO(err)
		if err != nil {
			return DraftDTO{}, err
		}
		out.Refusals = append(out.Refusals, r)
	}
	for _, u := range d.UnmetMinimums {
		out.UnmetMinimums = append(out.UnmetMinimums, ClassUnmetDTO(u))
	}
	return out, nil
}

// Draft is POST /api/characters/draft: the creation's body as far as it is entered (every field
// optional), read as POST /api/characters would read it, storing nothing (ADR 0034, amendment of
// 2026-10-20: it replaces the checks and the preview). A draft's content is never a 400: a value
// the creation refuses is one of the answer's refusals. A body that is no JSON or names an
// unknown field is still a bad request.
func Draft(svc drafter) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if _, err := user(r); err != nil {
			return err
		}
		b, err := decodeBody(r)
		var fe *domain.FieldError
		if errors.As(err, &fe) {
			// A height or weight that is no number: nothing else can be read.
			out, err := draftDTO(&charactersvc.Draft{Refusals: []error{fe}})
			if err != nil {
				return err
			}
			render.OK(r.Context(), w, out)
			return nil
		}
		if err != nil {
			return err
		}
		n, blank, err := b.entered()
		if err != nil {
			return err
		}
		d, err := svc.Draft(r.Context(), n, blank)
		if err != nil {
			return err
		}
		out, err := draftDTO(d)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}
