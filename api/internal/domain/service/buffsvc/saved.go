package buffsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ListSaved returns the saved buff templates.
func (s *service) ListSaved(ctx context.Context, characterID int64) ([]SavedBuff, error) {
	return s.dal.ListSavedBuffs(ctx, characterID)
}

// SaveBuff stores saved as a template. A template with the same name
// (case-insensitive) is replaced, so saving again updates it.
func (s *service) SaveBuff(ctx context.Context, characterID int64, saved SavedBuff) ([]SavedBuff, error) {
	saved.ID = 0
	saved.Name = strings.TrimSpace(saved.Name)
	saved.Note = strings.TrimSpace(saved.Note)
	if err := validateBuff(BuffEffect{Name: saved.Name, Charges: saved.Charges, Duration: saved.Duration, Effects: saved.Effects}); err != nil {
		return nil, err
	}
	return s.changeSaved(ctx, characterID, func(before []SavedBuff) ([]SavedBuff, logPayload, error) {
		after := make([]SavedBuff, 0, len(before)+1)
		for _, b := range before {
			if !strings.EqualFold(b.Name, saved.Name) {
				after = append(after, b)
			}
		}
		after = append(after, saved)
		return after, logPayload{Action: "save_template", Buff: saved.Name, Rounds: saved.Duration, Charges: saved.Charges, Note: saved.Note}, nil
	})
}

// DeleteSaved removes the template with id.
func (s *service) DeleteSaved(ctx context.Context, characterID, id int64) ([]SavedBuff, error) {
	return s.changeSaved(ctx, characterID, func(before []SavedBuff) ([]SavedBuff, logPayload, error) {
		after := make([]SavedBuff, 0, len(before))
		var gone *SavedBuff
		for i, b := range before {
			if b.ID == id {
				gone = &before[i]
				continue
			}
			after = append(after, b)
		}
		if gone == nil {
			return nil, logPayload{}, fmt.Errorf("%w: saved buff %d", domain.ErrNotFound, id)
		}
		return after, logPayload{Action: "delete_template", Buff: gone.Name}, nil
	})
}

// changeSaved runs decide on the stored list, writes the result and records
// the entry with before/after snapshots in one transaction.
func (s *service) changeSaved(ctx context.Context, characterID int64,
	decide func(before []SavedBuff) ([]SavedBuff, logPayload, error),
) ([]SavedBuff, error) {
	var out []SavedBuff
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		before, err := s.dal.ListSavedBuffs(ctx, characterID)
		if err != nil {
			return fmt.Errorf("list saved buffs: %w", err)
		}
		next, payload, err := decide(append([]SavedBuff(nil), before...))
		if err != nil {
			return err
		}
		if err := s.dal.ReplaceSavedBuffs(ctx, characterID, next); err != nil {
			return fmt.Errorf("save saved buffs: %w", err)
		}
		// Read back: new templates now have their ids, and the snapshot a
		// void restores must carry them.
		out, err = s.dal.ListSavedBuffs(ctx, characterID)
		if err != nil {
			return fmt.Errorf("list saved buffs: %w", err)
		}
		b, err := encodeSaved(before)
		if err != nil {
			return err
		}
		a, err := encodeSaved(out)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode payload: %w", err)
		}
		if _, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindBuffChange,
			Payload:     raw,
			Snapshots:   []actionlog.Snapshot{{Key: StateKeySavedBuffs, Before: b, After: a}},
		}); err != nil {
			return fmt.Errorf("record %s: %w", actionlog.KindBuffChange, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// snapSaved is the JSON encoding of a template inside action-log snapshots
// (StateKeySavedBuffs): add fields, never rename them.
type snapSaved struct {
	ID       int64       `json:"id"`
	Name     string      `json:"name"`
	Duration int         `json:"duration"`
	Charges  int         `json:"charges"`
	Effects  snapEffects `json:"effects"`
	Note     string      `json:"note,omitempty"`
	// CreatedAt is absent in snapshots taken before it was recorded.
	CreatedAt time.Time `json:"createdAt,omitzero"`
}

func encodeSaved(list []SavedBuff) (json.RawMessage, error) {
	out := make([]snapSaved, len(list))
	for i, b := range list {
		out[i] = snapSaved{ID: b.ID, Name: b.Name, Duration: b.Duration, Charges: b.Charges, Effects: snapEffects(b.Effects), Note: b.Note, CreatedAt: b.CreatedAt}
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encode saved buffs: %w", err)
	}
	return raw, nil
}

func decodeSaved(raw json.RawMessage) ([]SavedBuff, error) {
	var in []snapSaved
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("decode saved buffs snapshot: %w", err)
	}
	out := make([]SavedBuff, len(in))
	for i, b := range in {
		out[i] = SavedBuff{ID: b.ID, Name: b.Name, Duration: b.Duration, Charges: b.Charges, Effects: BuffEffects(b.Effects), Note: b.Note, CreatedAt: b.CreatedAt}
	}
	return out, nil
}
