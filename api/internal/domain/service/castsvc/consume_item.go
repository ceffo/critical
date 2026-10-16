package castsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// castMechanics applies what casting the spell does besides its effect, from
// the catalog entry (ADR 0008 step 10): check the target AC of a spell with an
// attack roll, spend the item it consumes, then roll the to-hit (one d20,
// never a crit: ADR 0002 Consequences). The target AC is checked first so a
// refused cast spends nothing. It returns the item snapshot to log (none when
// no item was spent).
func (s *service) castMechanics(ctx context.Context, characterID int64, entry spelleffectsvc.Entry, req CastRequest, res *CastResult) ([]actionlog.Snapshot, error) {
	if entry.AttackRoll && req.TargetAC == nil {
		return nil, fmt.Errorf("%w: %s needs the target's AC", domain.ErrInvalidInput, entry.Name)
	}
	var snaps []actionlog.Snapshot
	if entry.ConsumeItem != nil {
		used, snap, err := s.consume(ctx, entry, req.ItemID)
		if err != nil {
			return nil, err
		}
		res.Consumed, snaps = used, append(snaps, snap)
	}
	if entry.AttackRoll {
		roll, err := s.Attacks.RollSpellToHit(ctx, characterID, *req.TargetAC)
		if err != nil {
			return nil, err
		}
		res.Attack = roll
	}
	return snaps, nil
}

// consume spends one unit of the chosen item of the entry's type. Refuses with
// a clear message when none is left, or when several are and none was chosen.
func (s *service) consume(ctx context.Context, entry spelleffectsvc.Entry, itemID string) (*ConsumedItem, actionlog.Snapshot, error) {
	itemType := entry.ConsumeItem.Type
	items, err := s.Items.List(ctx)
	if err != nil {
		return nil, actionlog.Snapshot{}, fmt.Errorf("list items: %w", err)
	}
	var left []itemsvc.Item
	for _, it := range items {
		if it.Spendable(itemType) {
			left = append(left, it)
		}
	}
	var item itemsvc.Item
	switch {
	case len(left) == 0:
		return nil, actionlog.Snapshot{}, fmt.Errorf("%w: %s needs a %s and none is left in the inventory", domain.ErrInvalidInput, entry.Name, itemType)
	case itemID != "":
		found := false
		for _, it := range left {
			if it.ID == itemID {
				item, found = it, true
			}
		}
		if !found {
			return nil, actionlog.Snapshot{}, fmt.Errorf("%w: %q is not a %s left in the inventory", domain.ErrInvalidInput, itemID, itemType)
		}
	case len(left) == 1:
		item = left[0]
	default:
		return nil, actionlog.Snapshot{}, fmt.Errorf("%w: choose the %s %s spends", domain.ErrInvalidInput, itemType, entry.Name)
	}

	used, err := s.Items.Use(ctx, item.ID)
	if err != nil {
		return nil, actionlog.Snapshot{}, fmt.Errorf("spend %s: %w", item.ID, err)
	}
	snap, err := itemsvc.UseSnapshot(used)
	if err != nil {
		return nil, actionlog.Snapshot{}, err
	}
	return &ConsumedItem{ItemID: item.ID, Name: item.Name.Any(), Remaining: used.Count}, snap, nil
}
