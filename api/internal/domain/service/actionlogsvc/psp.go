package actionlogsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// AppendPSPChange records a psp_change entry, the PSP pool's hand edit (ADR
// 0043 step 5), as AppendHPEvent does for HP: through Record, so a gain is
// capped at the PSPs spent and a loss never takes the pool under 0, and undo
// pops it. A delta of 0 changes nothing and a character with no PSP maximum
// has no pool: both are refused before anything is logged.
func (s *service) AppendPSPChange(ctx context.Context, characterID int64, delta int, source string) (actionlog.PSPState, error) {
	if delta == 0 {
		return actionlog.PSPState{}, domain.FieldErrorf("delta", "a PSP change needs a delta")
	}
	var state actionlog.PSPState
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		maxPSP, err := s.maxima.GetMaxPSP(ctx, characterID)
		if err != nil {
			return fmt.Errorf("max psp: %w", err)
		}
		if maxPSP <= 0 {
			return ErrNoPSPPool
		}
		entry := actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindPSPChange,
			Summary:     strings.TrimSpace(source),
			PSPDelta:    delta,
		}
		if entry.Summary == "" {
			entry.Payload = json.RawMessage(`{}`)
		}
		if _, err := s.Record(ctx, entry); err != nil {
			return fmt.Errorf("record psp change: %w", err)
		}
		total, err := s.dal.PSPTotal(ctx, characterID)
		if err != nil {
			return fmt.Errorf("psp total: %w", err)
		}
		state = actionlog.PSPState{Current: actionlog.CurrentPSP(maxPSP, total), Max: maxPSP}
		return nil
	})
	if err != nil {
		return actionlog.PSPState{}, err
	}
	return state, nil
}
