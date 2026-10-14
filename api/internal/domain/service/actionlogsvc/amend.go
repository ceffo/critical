package actionlogsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
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

// AmendLatest records an amend carrying a new summary and payload on entry
// id. It is refused when the entry is another kind or kind is not a roll
// (ErrNotAmendable), voided (ErrAlreadyVoided) or not the latest roll: a newer
// entry of any of actionlog.RollKinds exists, voided ones included
// (ErrNotLatest). The check and the write share one transaction.
func (s *service) AmendLatest(ctx context.Context, characterID, id int64, kind actionlog.Kind, fn actionlog.AmendFunc) (*actionlog.Entry, error) {
	var out *actionlog.Entry
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		e, err := s.dal.GetEntry(ctx, characterID, id)
		if err != nil {
			return fmt.Errorf("get entry %d: %w", id, err)
		}
		if e.Kind != kind || !actionlog.IsRoll(kind) {
			return fmt.Errorf("%w: #%d is %s, not %s", ErrNotAmendable, id, e.Kind, kind)
		}
		if e.Status() == actionlog.StatusVoided {
			return fmt.Errorf("%w: #%d", ErrAlreadyVoided, id)
		}
		latest, err := s.latestRoll(ctx, characterID)
		if err != nil {
			return err
		}
		if latest != id {
			return fmt.Errorf("%w: only the latest roll can be adjusted, void this %s and roll again", ErrNotLatest, kind)
		}
		a, err := fn(*e)
		if err != nil {
			return err
		}
		a.Summary = strings.TrimSpace(a.Summary)
		if a.Summary == "" || len(a.Payload) == 0 {
			return fmt.Errorf("%w: amendment needs a summary and a payload", domain.ErrInvalidInput)
		}
		if err := s.dal.InsertCorrection(ctx, actionlog.Correction{
			ActionID: id,
			Kind:     actionlog.CorrectionAmend,
			Summary:  a.Summary,
			Payload:  a.Payload,
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

// latestRoll is the id of the newest entry of any roll kind (0: none).
func (s *service) latestRoll(ctx context.Context, characterID int64) (int64, error) {
	var latest int64
	for _, k := range actionlog.RollKinds {
		id, ok, err := s.dal.LatestID(ctx, characterID, k)
		if err != nil {
			return 0, fmt.Errorf("latest %s entry: %w", k, err)
		}
		if ok && id > latest {
			latest = id
		}
	}
	return latest, nil
}
