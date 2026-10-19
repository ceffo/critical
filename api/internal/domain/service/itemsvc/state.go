package itemsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// StateResult is one state change (ADR 0033 step 7): the pieces as they are
// now (the one moved first, then the one a swap took off) and the entry
// written; LogEntryID is 0 when the piece already was in that state (nothing
// logged).
type StateResult struct {
	Items      []Item
	LogEntryID int64
}

// stateMove is one piece's move inside an equipment_state payload.
type stateMove struct {
	ItemID   string `json:"itemId"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Before   string `json:"before"`
	After    string `json:"after"`
}

// stateRecord is the equipment_state log payload (ADR 0033 step 7): the piece
// moved and, for a swap, the piece the move took off (Off). Add fields, never
// rename.
type stateRecord struct {
	stateMove
	Off []stateMove `json:"off,omitempty"`
}

func move(before, after Item) stateMove {
	return stateMove{
		ItemID: after.ID, Name: after.Name.Any(), Category: string(after.Category),
		Before: string(before.State), After: string(after.State),
	}
}

// SetState moves a piece to state (ADR 0033 steps 6 and 7): wearing,
// taking off, wielding and putting away are one equipment_state entry with
// each changed piece's snapshot, in one transaction. A state the category
// does not allow is refused (field "state", CodeNotAllowed). Wearing a body
// armour or a shield while another is worn is refused (CodeSlotTaken) unless
// swap: the worn one is then taken off in the same entry. The same state
// again logs nothing.
func (s *service) SetState(ctx context.Context, characterID int64, itemID string, state State, swap bool) (*StateResult, error) {
	var res *StateResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		res, err = s.setState(ctx, characterID, itemID, state, swap)
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// setState is SetState inside the caller's transaction.
func (s *service) setState(ctx context.Context, characterID int64, itemID string, state State, swap bool) (*StateResult, error) {
	old, err := s.dal.Get(ctx, characterID, itemID)
	if err != nil {
		return nil, err
	}
	if !state.Valid() || !Allows(old.Category, old.MagicKind, state) {
		return nil, fieldErr("state", CodeNotAllowed)
	}
	if old.State == state {
		return &StateResult{Items: []Item{old}}, nil
	}
	moved := old
	moved.State = state
	var off []Item
	if state == StateWorn && slotted(old.Category) {
		all, err := s.dal.List(ctx, characterID)
		if err != nil {
			return nil, err
		}
		for _, o := range all {
			if o.ID != old.ID && o.Category == old.Category && o.State == StateWorn {
				off = append(off, o)
			}
		}
		if len(off) > 0 && !swap {
			return nil, fieldErr("state", CodeSlotTaken)
		}
	}
	rec := stateRecord{stateMove: move(old, moved)}
	snaps := make([]actionlog.Snapshot, 0, 1+len(off))
	results := []Item{moved}
	for _, o := range off {
		taken := o
		taken.State = StateCarried
		if err := s.dal.Update(ctx, characterID, taken); err != nil {
			return nil, err
		}
		snap, err := itemSnapshot(o.ID, &o, &taken)
		if err != nil {
			return nil, err
		}
		snaps = append(snaps, snap)
		rec.Off = append(rec.Off, move(o, taken))
		results = append(results, taken)
	}
	if err := s.dal.Update(ctx, characterID, moved); err != nil {
		return nil, err
	}
	snap, err := itemSnapshot(old.ID, &old, &moved)
	if err != nil {
		return nil, err
	}
	snaps = append([]actionlog.Snapshot{snap}, snaps...)
	payload, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("encode %s payload: %w", actionlog.KindEquipmentState, err)
	}
	id, err := s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID, Kind: actionlog.KindEquipmentState, Payload: payload, Snapshots: snaps,
	})
	if err != nil {
		return nil, fmt.Errorf("record %s: %w", actionlog.KindEquipmentState, err)
	}
	return &StateResult{Items: results, LogEntryID: id}, nil
}

// slotFree reports whether piece may be worn as far as the other pieces go:
// no other body armour or shield (by its category) is worn.
func (s *service) slotFree(ctx context.Context, characterID int64, piece Item) (bool, error) {
	if piece.State != StateWorn || !slotted(piece.Category) {
		return true, nil
	}
	all, err := s.dal.List(ctx, characterID)
	if err != nil {
		return false, err
	}
	for _, o := range all {
		if o.ID != piece.ID && o.Category == piece.Category && o.State == StateWorn {
			return false, nil
		}
	}
	return true, nil
}

// Wield wields a carried weapon inside the caller's transaction (the attack
// form's pick, ADR 0033 step 8): one equipment_state entry, none when it is
// already wielded. ErrItemNotFound, or a field error for a piece that is not a
// weapon.
func (s *service) Wield(ctx context.Context, characterID int64, itemID string) (*StateResult, error) {
	res, err := s.setState(ctx, characterID, itemID, StateWielded, false)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Reviews is the character's review list (ADR 0033 step 27): the rows of
// the pieces it still holds, in list order.
func (s *service) Reviews(ctx context.Context, characterID int64) ([]Review, error) {
	rows, err := s.dal.Reviews(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	items, err := s.dal.List(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	byItem := map[string][]Review{}
	for _, r := range rows {
		byItem[r.ItemID] = append(byItem[r.ItemID], r)
	}
	out := []Review{}
	for _, it := range items {
		out = append(out, byItem[it.ID]...)
	}
	return out, nil
}

// DismissReview takes a piece off the review list (ADR 0033 step 27): not a
// game move, nothing is logged. ErrItemNotFound for a piece the character
// does not hold.
func (s *service) DismissReview(ctx context.Context, characterID int64, itemID string) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.dal.Get(ctx, characterID, itemID); err != nil {
			return err
		}
		if err := s.dal.DeleteReviews(ctx, characterID, itemID); err != nil {
			return fmt.Errorf("dismiss review: %w", err)
		}
		return nil
	})
}
