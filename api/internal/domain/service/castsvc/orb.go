package castsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// OrbID is Chromatic Orb.
const OrbID = "chromatic_orb"

// isOrbGem tells the gems the orb can spend: type "gem", not a daily item
// (the Power Gem is a daily item that is never spent).
func isOrbGem(it itemsvc.Item) bool {
	return strings.EqualFold(it.Type, "gem") && it.Behaviour != itemsvc.BehaviourDaily
}

// throwOrb spends the chosen gem and rolls the orb's attack (no crit; the gem has no effect of its own). It
// returns the item snapshot to log. Refuses with a clear message when no
// gem is left, or when several are and none was chosen.
func (s *service) throwOrb(ctx context.Context, characterID int64, req CastRequest) (*OrbResult, actionlog.Snapshot, error) {
	if req.TargetAC == nil {
		return nil, actionlog.Snapshot{}, fmt.Errorf("%w: Chromatic Orb needs the target's AC", domain.ErrInvalidInput)
	}
	items, err := s.Items.List(ctx)
	if err != nil {
		return nil, actionlog.Snapshot{}, fmt.Errorf("list items: %w", err)
	}
	var gems []itemsvc.Item
	for _, it := range items {
		if isOrbGem(it) && it.Count > 0 {
			gems = append(gems, it)
		}
	}
	var gem itemsvc.Item
	switch {
	case len(gems) == 0:
		return nil, actionlog.Snapshot{}, fmt.Errorf("%w: Chromatic Orb needs a gem and none is left in the inventory", domain.ErrInvalidInput)
	case req.GemItemID != "":
		found := false
		for _, g := range gems {
			if g.ID == req.GemItemID {
				gem, found = g, true
			}
		}
		if !found {
			return nil, actionlog.Snapshot{}, fmt.Errorf("%w: %q is not a gem left in the inventory", domain.ErrInvalidInput, req.GemItemID)
		}
	case len(gems) == 1:
		gem = gems[0]
	default:
		return nil, actionlog.Snapshot{}, fmt.Errorf("%w: choose the gem Chromatic Orb spends", domain.ErrInvalidInput)
	}

	used, err := s.Items.Use(ctx, gem.ID)
	if err != nil {
		return nil, actionlog.Snapshot{}, fmt.Errorf("spend gem %s: %w", gem.ID, err)
	}
	snap, err := itemsvc.UseSnapshot(used)
	if err != nil {
		return nil, actionlog.Snapshot{}, err
	}
	roll, err := s.Attacks.RollSpellToHit(ctx, characterID, *req.TargetAC)
	if err != nil {
		return nil, actionlog.Snapshot{}, err
	}
	out := &OrbResult{GemItemID: gem.ID, Gem: gem.Name, Remaining: used.Count, Attack: *roll}
	return out, snap, nil
}
