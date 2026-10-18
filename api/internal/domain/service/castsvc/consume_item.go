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
		used, snap, err := s.consume(ctx, characterID, entry, req.ItemID)
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
func (s *service) consume(ctx context.Context, characterID int64, entry spelleffectsvc.Entry, itemID string) (*ConsumedItem, actionlog.Snapshot, error) {
	item, err := s.pickItem(ctx, characterID, entry, itemID)
	if err != nil {
		return nil, actionlog.Snapshot{}, err
	}
	used, err := s.Items.Use(ctx, characterID, item.ID)
	if err != nil {
		return nil, actionlog.Snapshot{}, fmt.Errorf("spend %s: %w", item.ID, err)
	}
	snap, err := itemsvc.UseSnapshot(used)
	if err != nil {
		return nil, actionlog.Snapshot{}, err
	}
	return &ConsumedItem{ItemID: item.ID, Name: item.Name.Any(), Remaining: used.Count}, snap, nil
}

// pickItem finds the item of the entry's type the cast spends, writing
// nothing: the chosen one, or the only one left. Refused when none is left,
// or when several are and none was chosen.
func (s *service) pickItem(ctx context.Context, characterID int64, entry spelleffectsvc.Entry, itemID string) (itemsvc.Item, error) {
	itemType := entry.ConsumeItem.Type
	items, err := s.Items.List(ctx, characterID)
	if err != nil {
		return itemsvc.Item{}, fmt.Errorf("list items: %w", err)
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
		return itemsvc.Item{}, fmt.Errorf("%w: %s needs a %s and none is left in the inventory", domain.ErrInvalidInput, entry.Name, itemType)
	case itemID != "":
		found := false
		for _, it := range left {
			if it.ID == itemID {
				item, found = it, true
			}
		}
		if !found {
			return itemsvc.Item{}, fmt.Errorf("%w: %q is not a %s left in the inventory", domain.ErrInvalidInput, itemID, itemType)
		}
	case len(left) == 1:
		item = left[0]
	default:
		return itemsvc.Item{}, fmt.Errorf("%w: choose the %s %s spends", domain.ErrInvalidInput, itemType, entry.Name)
	}
	return item, nil
}

// checkCast runs the refusals a cast's effect would raise, writing nothing
// and rolling no die (ADR 0030 step 14: they come before the spell failure
// d100, so an invalid request is a 400 whatever the die): a spell that rolls
// to hit needs the target's AC, a spell that spends an item needs one left
// (the chosen one, or the only one), a spell that grants slots a valid split,
// and a buff a valid on-self choice. resolve applies the same rules.
func (s *service) checkCast(ctx context.Context, characterID int64, id string, req CastRequest, casterLevel int) error {
	entry, ok := s.Effects.Get(id)
	if !ok {
		return nil
	}
	if entry.GrantSlots != nil {
		if _, err := grantedLevels(entry, req.ExtraSlots); err != nil {
			return err
		}
	}
	if entry.Confidence == spelleffectsvc.ConfidenceLow {
		return nil // resolve stops before the caster's mechanics
	}
	if entry.AttackRoll && req.TargetAC == nil {
		return fmt.Errorf("%w: %s needs the target's AC", domain.ErrInvalidInput, entry.Name)
	}
	if entry.ConsumeItem != nil {
		if _, err := s.pickItem(ctx, characterID, entry, req.ItemID); err != nil {
			return err
		}
	}
	if entry.Kind == spelleffectsvc.KindBuff && entry.AddsBuff() {
		if _, err := appliesToSelf(entry, req.OnSelf, casterLevel); err != nil {
			return err
		}
	}
	return nil
}
