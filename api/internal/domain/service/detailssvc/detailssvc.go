// Package detailssvc edits the character's free-text details (race,
// alignment, age, sex, height, weight, hair, eyes and languages). The server
// validates every field and names the rejected one in a *domain.FieldError
// whose message is a code the web maps to a localized text. The checks are
// charactersvc.Details.Apply, shared with the creation form.
package detailssvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// Input bounds: the details' own, kept by charactersvc so that the creation
// form checks the details the same way (one validation, Details.Apply).
const (
	MaxShortLen    = charactersvc.MaxDetailShortLen
	MaxMeasureLen  = charactersvc.MaxDetailMeasureLen
	MaxAge         = charactersvc.MaxAge
	MaxLanguages   = charactersvc.MaxLanguages
	MaxLanguageLen = charactersvc.MaxLanguageLen
)

// Error codes carried by the FieldError messages. The web owns the wording.
const (
	CodeTooLong    = charactersvc.CodeTooLong
	CodeOutOfRange = charactersvc.CodeOutOfRange
	CodeTooMany    = charactersvc.CodeTooMany
	CodeEmpty      = charactersvc.CodeEmpty
	CodeDuplicate  = charactersvc.CodeDuplicate
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
		next, err := cur.Apply(p)
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
