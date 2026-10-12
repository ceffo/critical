package hpsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// AppendEvent records an hp_change entry in the action log and returns the
// new state. The business rule "source is required" lives here, not in the
// handler.
func (s *service) AppendEvent(ctx context.Context, characterID int64, delta int, source string) (*HPState, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("%w: source is required", domain.ErrInvalidInput)
	}
	if _, err := s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID,
		Kind:        actionlog.KindHPChange,
		Summary:     source,
		HPDelta:     delta,
	}); err != nil {
		return nil, fmt.Errorf("record hp change: %w", err)
	}
	return s.GetState(ctx, characterID)
}
