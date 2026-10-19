package acsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// StateKey is the action-log state key of the override.
const StateKey = "ac_override"

// jsonOverride is the one encoding of an override in the log: the
// ac_override entry's payload and its snapshots. Add fields, never rename.
type jsonOverride struct {
	Value int    `json:"value"`
	Label string `json:"label,omitempty"`
}

// overrideSnaps is the snapshotter of the AC override (ADR 0018 step 8).
var overrideSnaps = actionlog.NewSnapshotter[overrideState](StateKey)

// overrideState is the "ac_override" snapshot: the override, null for none.
type overrideState struct {
	Override *jsonOverride `json:"override"`
}

// overrideRecord is the payload of an ac_override entry, the override
// before and after (null: none). The UI words it.
type overrideRecord struct {
	Before *jsonOverride `json:"before"`
	After  *jsonOverride `json:"after"`
}

func toJSON(o *Override) *jsonOverride {
	if o == nil {
		return nil
	}
	return &jsonOverride{Value: o.Value, Label: o.Label}
}

func fromJSON(j *jsonOverride) *Override {
	if j == nil {
		return nil
	}
	return &Override{Value: j.Value, Label: j.Label}
}

// validOverride checks an override (ADR 0033 step 13): a number -10 to 10, a
// label (trimmed) of at most MaxLabelLen characters.
func validOverride(o Override) (Override, error) {
	if o.Value < buffsvc.MinAC || o.Value > buffsvc.MaxAC {
		return o, &domain.FieldError{Field: "value", Message: CodeOutOfRange}
	}
	o.Label = strings.TrimSpace(o.Label)
	if utf8.RuneCountInString(o.Label) > MaxLabelLen {
		return o, &domain.FieldError{Field: "label", Message: CodeTooLong}
	}
	return o, nil
}

// SetOverride is ADR 0033 step 13: the player sets an AC override, one
// logged move (ac_override, before and after) undone and redone by its
// snapshot. The same override as stored logs nothing.
func (s *service) SetOverride(ctx context.Context, characterID int64, o Override) (*AC, error) {
	o, err := validOverride(o)
	if err != nil {
		return nil, err
	}
	return s.change(ctx, characterID, &o)
}

// ClearOverride is step 13's clearing, logged the same way; none set logs
// nothing.
func (s *service) ClearOverride(ctx context.Context, characterID int64) (*AC, error) {
	return s.change(ctx, characterID, nil)
}

func (s *service) change(ctx context.Context, characterID int64, next *Override) (*AC, error) {
	var out *AC
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		before, err := s.overrides.GetACOverride(ctx, characterID)
		if err != nil {
			return err
		}
		if !sameOverride(before, next) {
			if err := s.record(ctx, characterID, before, next); err != nil {
				return err
			}
		}
		out, err = s.Get(ctx, characterID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func sameOverride(a, b *Override) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// record writes the override and its entry.
func (s *service) record(ctx context.Context, characterID int64, before, after *Override) error {
	if err := s.overrides.SetACOverride(ctx, characterID, after); err != nil {
		return fmt.Errorf("set ac override: %w", err)
	}
	payload, err := json.Marshal(overrideRecord{Before: toJSON(before), After: toJSON(after)})
	if err != nil {
		return fmt.Errorf("encode ac_override payload: %w", err)
	}
	b, a := overrideState{Override: toJSON(before)}, overrideState{Override: toJSON(after)}
	snap, err := overrideSnaps.Snapshot("", &b, &a)
	if err != nil {
		return err
	}
	if _, err := s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID, Kind: actionlog.KindACOverride, Payload: payload,
		Snapshots: []actionlog.Snapshot{snap},
	}); err != nil {
		return fmt.Errorf("record ac_override: %w", err)
	}
	return nil
}

// Restorers writes the override back: undo writes the entry's "before",
// redo its "after" (ADR 0019). It runs inside the undo's transaction.
func (s *service) Restorers() map[string]actionlog.Restorer {
	return map[string]actionlog.Restorer{
		StateKey: overrideSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, st overrideState) error {
			if err := s.overrides.SetACOverride(ctx, characterID, fromJSON(st.Override)); err != nil {
				return fmt.Errorf("restore ac override: %w", err)
			}
			return nil
		}, nil),
	}
}
