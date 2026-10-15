package hpsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// AppendEvent records an hp_change entry in the action log and returns the
// new state. The source is optional: without one the entry carries no player
// text (an empty payload keeps it a valid entry) and the web words the kind.
func (s *service) AppendEvent(ctx context.Context, characterID int64, delta int, source string) (*HPState, error) {
	entry := actionlog.NewEntry{
		CharacterID: characterID,
		Kind:        actionlog.KindHPChange,
		Summary:     strings.TrimSpace(source),
		HPDelta:     delta,
	}
	if entry.Summary == "" {
		entry.Payload = json.RawMessage(`{}`)
	}
	if _, err := s.log.Record(ctx, entry); err != nil {
		return nil, fmt.Errorf("record hp change: %w", err)
	}
	return s.GetState(ctx, characterID)
}
