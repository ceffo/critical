package actionlogsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// GetHPState returns the current hit-point view: maxHP from the character row
// plus the action log's SUM of effective HP deltas, and the last N non-voided
// HP changes (amended values shown) for the UI.
func (s *service) GetHPState(ctx context.Context, characterID int64) (*actionlog.HPState, error) {
	maxHP, err := s.maxima.GetMaxHP(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get max hp: %w", err)
	}
	sum, err := s.HPTotal(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get hp sum: %w", err)
	}
	entries, err := s.ActiveEntries(ctx, characterID, actionlog.KindHPChange, s.cfg.HistoryLimit)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}
	events := make([]actionlog.HPEvent, len(entries))
	for i, e := range entries {
		events[i] = actionlog.HPEvent{
			ID:          e.ID,
			CharacterID: e.CharacterID,
			Delta:       e.EffectiveHPDelta(),
			Source:      e.EffectiveSummary(),
			CreatedAt:   e.CreatedAt,
		}
	}
	// ADR 0007 step 6: never shown above max. Writes keep the total <= 0
	// (actionlog.CapHeal); this guards rows logged before. ADR 0011 step 1
	// guards the other end the same way: writes stop at the floor
	// (actionlog.FloorDamage), rows logged before it never show below it.
	current := actionlog.CurrentHP(maxHP, sum)
	return &actionlog.HPState{
		CurrentHp: current,
		MaxHp:     maxHP,
		FloorHp:   actionlog.HPFloor,
		Condition: actionlog.ConditionOf(current, maxHP), // ADR 0011 step 2
		Events:    events,
	}, nil
}

// AppendHPEvent records an hp_change entry and returns the new state. The
// source is optional: without one the entry carries no player text (an empty
// payload keeps it a valid entry) and the web words the kind.
func (s *service) AppendHPEvent(ctx context.Context, characterID int64, delta int, source string) (*actionlog.HPState, error) {
	entry := actionlog.NewEntry{
		CharacterID: characterID,
		Kind:        actionlog.KindHPChange,
		Summary:     strings.TrimSpace(source),
		HPDelta:     delta,
	}
	if entry.Summary == "" {
		entry.Payload = json.RawMessage(`{}`)
	}
	if _, err := s.Record(ctx, entry); err != nil {
		return nil, fmt.Errorf("record hp change: %w", err)
	}
	return s.GetHPState(ctx, characterID)
}
