package castsvc

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// grantedLevels validates the split the player chose for a spell that grants
// extra slots and returns it sorted (ADR 0008 step 9): positive levels adding
// up to the spell's levels. Nothing chosen means all level 1 slots.
func grantedLevels(entry spelleffectsvc.Entry, levels []int) ([]int, error) {
	g := entry.GrantSlots
	if len(levels) == 0 {
		return g.SlotSplits()[0], nil
	}
	out := append([]int(nil), levels...)
	sort.Ints(out)
	if !slices.ContainsFunc(g.SlotSplits(), func(s []int) bool { return slices.Equal(s, out) }) {
		return nil, fmt.Errorf("%w: %s grants %d spell levels split into slots of level 1 or more, not %v",
			domain.ErrInvalidInput, entry.Name, g.Levels, levels)
	}
	return out, nil
}

// pickForExtra checks the spell the player casts from a blank extra slot of
// the given level and returns its id and name.
func (s *service) pickForExtra(ctx context.Context, level int, spellID string) (string, string, error) {
	id := spelleffectsvc.Slug(spellID)
	if id == "" {
		return "", "", fmt.Errorf("%w: choose the spell to cast from this extra slot", domain.ErrInvalidInput)
	}
	spell, found, err := s.Texts.Describe(ctx, id)
	if err != nil {
		return "", "", fmt.Errorf("describe %s: %w", id, err)
	}
	entry, hasEffect := s.Effects.Get(id)
	if !found && !hasEffect {
		return "", "", fmt.Errorf("%w: spell %q", domain.ErrNotFound, id)
	}
	if got := max(entry.Level, spell.Level); got != level {
		return "", "", fmt.Errorf("%w: %s is level %d, this slot is level %d", domain.ErrInvalidInput, firstNonEmpty(entry.Name, spell.Name, id), got, level)
	}
	return id, firstNonEmpty(entry.Name, spell.Name, id), nil
}
