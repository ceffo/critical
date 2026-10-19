package buffsvc

import (
	"context"
	"encoding/json"
	"errors"
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
// snapshots (StateKeyInitiative); no initiative is absent. EntryID is
// absent from the "after" of the entry that rolled it (its id is not known
// before it is recorded): a redo writes that "after" back through
// initiativeSnaps' replay, which knows the entry (ADR 0019 step 1).
type snapInitiative struct {
	D10     int   `json:"d10"`
	EntryID int64 `json:"entryId,omitempty"`
}

func toSnapInitiative(i *Initiative) *snapInitiative {
	if i == nil {
		return nil
	}
	return &snapInitiative{D10: i.D10, EntryID: i.LogEntryID}
}

// initiativeSnapshot is the snapshot of the initiative going from before to
// after (nil: none).
func initiativeSnapshot(before, after *Initiative) (actionlog.Snapshot, error) {
	return initiativeSnaps.Snapshot("", toSnapInitiative(before), toSnapInitiative(after))
}

// rollInitiative rolls round's initiative and records it as its own entry,
// stamped with that round, whose snapshot has the stored initiative as
// "before", then stores it with the entry's id. It must run inside a
// transaction, as the last entry that transaction records.
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

// ErrNoInitiative is the conflict for rerolling out of combat; it also wraps
// domain.ErrConflict.
var ErrNoInitiative = errors.New("no initiative to reroll")

// ErrInitiativeNotOnTop is the conflict for rerolling once another
// transaction is newer than the one that recorded the round's initiative; it
// also wraps domain.ErrConflict.
var ErrInitiativeNotOnTop = errors.New("the initiative is no longer the newest action")

// RerollInitiative is ADR 0013 step 4 as amended by ADR 0018 step 5: a new
// d10 recorded as its own transaction, whose initiative snapshot keeps the
// roll it replaces as "before", so undo gives that roll back. Nothing is
// voided. It is allowed only when the newest transaction is the one that
// recorded the round's current initiative (the Next Round or a previous
// reroll): that transaction ends with the initiative entry (rollInitiative is
// the last thing both record), so it is the newest one exactly when the
// initiative's entry is the newest non-voided entry. Refused, with nothing
// rolled, otherwise or when there is no initiative.
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
			return fmt.Errorf("%w: %w", domain.ErrConflict, ErrNoInitiative)
		}
		top, ok, err := s.log.TopID(ctx, characterID)
		if err != nil {
			return fmt.Errorf("top of the log: %w", err)
		}
		if !ok || top != current.LogEntryID {
			return fmt.Errorf("%w: %w: the initiative is #%d, the newest entry #%d",
				domain.ErrConflict, ErrInitiativeNotOnTop, current.LogEntryID, top)
		}
		out, err = s.rollInitiative(ctx, characterID, round)
		return err
	})
	if err != nil {
		return Initiative{}, err
	}
	return out, nil
}
