// Package detailssvc edits the character's free-text details (alignment,
// age, sex, height, weight, hair and eyes). The languages are the skills
// module's (skillsvc, ADR 0037 step 18); this service keeps them. The server
// validates every field and names the rejected one in a *domain.FieldError
// whose message is a code the web maps to a localized text. The checks are
// charactersvc.Details.Apply, shared with the creation form.
package detailssvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// Patch holds the fields to change (charactersvc.DetailsPatch).
type Patch = charactersvc.DetailsPatch

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Update applies the patch and returns the details as stored. A
	// *domain.FieldError for invalid input, ErrCharacterNotFound.
	Update(ctx context.Context, characterID int64, p Patch) (charactersvc.Details, error)
}

// DAL is the driven port. Both methods join the transaction carried by ctx.
type DAL interface {
	// GetDetails are the stored details and the class and race codes of the
	// character (the alignment limit depends on the class, the native
	// language on the race).
	GetDetails(ctx context.Context, id int64) (charactersvc.Details, charactersvc.Kind, error)
	UpdateDetails(ctx context.Context, id int64, v charactersvc.Details) error
}

type service struct {
	dal       DAL
	tx        domain.TxRunner
	languages charactersvc.LanguageBook
	defs      *classes.Registry
}

// New constructs the service. languages is the language catalogue (ADR 0032
// step 4), for the languages the answer shows; nil is the empty one of a
// catalogue before format 8. defs are the class definitions, for the
// alignments a class admits (ADR 0040 step 1.5).
func New(dal DAL, tx domain.TxRunner, languages charactersvc.LanguageBook, defs *classes.Registry) (SVC, error) {
	if dal == nil || tx == nil || defs == nil {
		return nil, fmt.Errorf("detailssvc: nil dependency")
	}
	if languages == nil {
		languages = (*ruletablesvc.Languages)(nil)
	}
	return &service{dal: dal, tx: tx, languages: languages, defs: defs}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Module declares no codec: the details' edits are not on the action log.
func (s *service) Module() domain.Module { return domain.Module{Name: "detailssvc"} }

func (s *service) Update(ctx context.Context, id int64, p Patch) (charactersvc.Details, error) {
	var out charactersvc.Details
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		cur, kind, err := s.dal.GetDetails(ctx, id)
		if err != nil {
			return err
		}
		next, err := cur.Apply(charactersvc.AlignmentsOf(s.defs, kind.Class), p)
		if err != nil {
			return err
		}
		if err := s.dal.UpdateDetails(ctx, id, next); err != nil {
			return err
		}
		// Read as the card reads it: the native language among them (ADR
		// 0032 step 4.1), a character stored before it was added included.
		next.Languages = skills.KnownLanguages(s.languages, next.Languages)
		out = next.WithNative(s.languages.Native(kind.Race))
		return nil
	})
	return out, err
}
