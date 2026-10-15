package buffsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// initiativePayload is the initiative entry's structured record, worded by
// the UI: the round it was rolled for and the d10.
type initiativePayload struct {
	Round int `json:"round"`
	D10   int `json:"d10"`
}

// snapInitiative is buffsvc's JSON encoding of the initiative in action-log
// snapshots (StateKeyInitiative); JSON null is "no initiative". EntryID is
// absent from the "after" of the entry that rolled it (its id is not known
// before it is recorded); restores only ever write a "before" back.
type snapInitiative struct {
	D10     int   `json:"d10"`
	EntryID int64 `json:"entryId,omitempty"`
}

func encodeInitiative(i *Initiative) (json.RawMessage, error) {
	if i == nil {
		return json.RawMessage(`null`), nil
	}
	return json.Marshal(snapInitiative{D10: i.D10, EntryID: i.LogEntryID})
}

func decodeInitiative(raw json.RawMessage) (*Initiative, error) {
	var in *snapInitiative
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("decode initiative snapshot: %w", err)
	}
	if in == nil {
		return nil, nil
	}
	if in.D10 < 1 || in.D10 > InitiativeDie {
		return nil, fmt.Errorf("decode initiative snapshot: d10 %d out of range", in.D10)
	}
	return &Initiative{D10: in.D10, LogEntryID: in.EntryID}, nil
}

func initiativeSnapshot(before, after *Initiative) (actionlog.Snapshot, error) {
	b, err := encodeInitiative(before)
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode initiative: %w", err)
	}
	a, err := encodeInitiative(after)
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode initiative: %w", err)
	}
	return actionlog.Snapshot{Key: StateKeyInitiative, Before: b, After: a}, nil
}

// rollInitiative rolls round's initiative and records it as its own entry,
// stamped with that round, then stores it with the entry's id. It must run
// inside a transaction.
//
// ADR 0013 step 1: one d10 through the dice port. Step 3: the value is the
// plain die, no modifier; the lowest acts first, which only the display says.
func (s *service) rollInitiative(ctx context.Context, characterID int64, round int) (Initiative, error) {
	before, err := s.dal.GetInitiative(ctx, characterID)
	if err != nil {
		return Initiative{}, fmt.Errorf("get initiative: %w", err)
	}
	d10 := s.dice.RollDie(InitiativeDie)
	out := Initiative{D10: d10}
	snap, err := initiativeSnapshot(before, &out)
	if err != nil {
		return Initiative{}, err
	}
	payload, err := json.Marshal(initiativePayload{Round: round, D10: d10})
	if err != nil {
		return Initiative{}, fmt.Errorf("encode payload: %w", err)
	}
	// ADR 0013 step 2: its own entry, stamped with the round it is for.
	out.LogEntryID, err = s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID,
		Round:       round,
		Kind:        actionlog.KindInitiative,
		Payload:     payload,
		Snapshots:   []actionlog.Snapshot{snap},
	})
	if err != nil {
		return Initiative{}, fmt.Errorf("record %s: %w", actionlog.KindInitiative, err)
	}
	if err := s.dal.SaveInitiative(ctx, characterID, &out); err != nil {
		return Initiative{}, fmt.Errorf("save initiative: %w", err)
	}
	return out, nil
}

// GetInitiative returns the round's initiative, nil when there is none.
func (s *service) GetInitiative(ctx context.Context, characterID int64) (*Initiative, error) {
	return s.dal.GetInitiative(ctx, characterID)
}

// RerollInitiative is ADR 0013 step 4: in one transaction, the round's
// initiative entry is popped (which must be the newest entry, else the log's
// not-latest conflict) and a new d10 is rolled and recorded.
func (s *service) RerollInitiative(ctx context.Context, characterID int64) (Initiative, error) {
	var out Initiative
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		round, err := s.dal.GetRoundNumber(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get round: %w", err)
		}
		current, err := s.dal.GetInitiative(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get initiative: %w", err)
		}
		// ADR 0013 step 5: out of combat there is nothing to reroll.
		if current == nil || round == 0 {
			return fmt.Errorf("%w: no initiative to reroll", domain.ErrConflict)
		}
		if _, err := s.log.Void(ctx, characterID, current.LogEntryID); err != nil {
			return fmt.Errorf("pop initiative: %w", err)
		}
		out, err = s.rollInitiative(ctx, characterID, round)
		return err
	})
	if err != nil {
		return Initiative{}, err
	}
	return out, nil
}

// restoreInitiative writes an initiative snapshot back (null clears it).
func (s *service) restoreInitiative(ctx context.Context, characterID int64, state json.RawMessage) error {
	initiative, err := decodeInitiative(state)
	if err != nil {
		return err
	}
	return s.dal.SaveInitiative(ctx, characterID, initiative)
}
