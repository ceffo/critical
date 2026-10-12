package actionlogsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// Amend records an amend correction on an hp_change entry. The original entry
// stays as it was; its effective delta and summary become the amendment's.
func (s *service) Amend(ctx context.Context, characterID, id int64, a Amendment) (*actionlog.Entry, error) {
	var out *actionlog.Entry
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		e, err := s.dal.GetEntry(ctx, characterID, id)
		if err != nil {
			return fmt.Errorf("get entry %d: %w", id, err)
		}
		if e.Status() == actionlog.StatusVoided {
			return fmt.Errorf("%w: #%d", ErrAlreadyVoided, id)
		}
		if e.Kind != actionlog.KindHPChange {
			return fmt.Errorf("%w: #%d is %s, only %s entries can be amended", ErrNotAmendable, id, e.Kind, actionlog.KindHPChange)
		}
		delta := a.HPDelta
		if err := s.dal.InsertCorrection(ctx, actionlog.Correction{
			ActionID: id,
			Kind:     actionlog.CorrectionAmend,
			HPDelta:  &delta,
			Summary:  strings.TrimSpace(a.Summary),
		}); err != nil {
			return fmt.Errorf("insert amend: %w", err)
		}
		out, err = s.dal.GetEntry(ctx, characterID, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
