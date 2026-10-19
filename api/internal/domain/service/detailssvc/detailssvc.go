// Package detailssvc edits the character's free-text details (alignment,
// age, sex, height, weight, hair, eyes and languages). The server
// validates every field and names the rejected one in a *domain.FieldError
// whose message is a code the web maps to a localized text. The checks are
// charactersvc.Details.Apply, shared with the creation form.
package detailssvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// Input bounds: the details' own, kept by charactersvc so that the creation
// form checks the details the same way (one validation, Details.Apply).
const (
	MaxShortLen   = charactersvc.MaxDetailShortLen
	MaxMeasureLen = charactersvc.MaxDetailMeasureLen
	MaxAge        = charactersvc.MaxAge
)

// Error codes carried by the FieldError messages. The web owns the wording.
const (
	CodeTooLong    = charactersvc.CodeTooLong
	CodeOutOfRange = charactersvc.CodeOutOfRange
	CodeEmpty      = charactersvc.CodeEmpty
	CodeDuplicate  = charactersvc.CodeDuplicate
	CodeUnknown    = charactersvc.CodeUnknown
	// CodeAlignmentNotAllowed is an alignment the character's class refuses.
	CodeAlignmentNotAllowed = charactersvc.CodeAlignmentNotAllowed
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
}

// New constructs the service. languages is the language catalogue (ADR 0032
// step 4); nil is the empty one of a catalogue before format 8.
func New(dal DAL, tx domain.TxRunner, languages charactersvc.LanguageBook) (SVC, error) {
	if dal == nil || tx == nil {
		return nil, fmt.Errorf("detailssvc: nil dependency")
	}
	if languages == nil {
		languages = (*ruletablesvc.Languages)(nil)
	}
	return &service{dal: dal, tx: tx, languages: languages}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

func (s *service) Update(ctx context.Context, id int64, p Patch) (charactersvc.Details, error) {
	var out charactersvc.Details
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		cur, kind, err := s.dal.GetDetails(ctx, id)
		if err != nil {
			return err
		}
		next, err := cur.Apply(kind, s.languages, p)
		if err != nil {
			return err
		}
		if err := s.dal.UpdateDetails(ctx, id, next); err != nil {
			return err
		}
		// Read as the card reads it: the native language among them (ADR
		// 0032 step 4.1), a character stored before it was added included.
		out = next.Known(s.languages).WithNative(s.languages.Native(kind.Race))
		return nil
	})
	return out, err
}
