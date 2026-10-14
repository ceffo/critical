package castsvc

import (
	"context"
	"fmt"
	"sort"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// RaryID is Rary's Mnemonic Enhancer, the one spell that adds spell slots.
const RaryID = "rarys_mnemonic_enhancer"

// raryLevelsTotal is the spell levels Rary's grants (PHB: "three levels of
// spells"; critical-qki.41): 3 x L1, or L1 + L2, or one L3.
const raryLevelsTotal = 3

// raryLevels validates the split the player chose and returns it sorted.
// Nothing chosen means three level 1 slots.
func raryLevels(levels []int) ([]int, error) {
	if len(levels) == 0 {
		return []int{1, 1, 1}, nil
	}
	sum := 0
	for _, l := range levels {
		if l < 1 {
			return nil, fmt.Errorf("%w: a Rary's slot must be level 1 or more", domain.ErrInvalidInput)
		}
		sum += l
	}
	if sum != raryLevelsTotal {
		return nil, fmt.Errorf("%w: Rary's Mnemonic Enhancer grants %d spell levels, not %d", domain.ErrInvalidInput, raryLevelsTotal, sum)
	}
	out := append([]int(nil), levels...)
	sort.Ints(out)
	return out, nil
}

// pickForExtra checks the spell the player casts from a blank extra slot of
// the given level and returns its id and name.
func (s *service) pickForExtra(ctx context.Context, level int, spellID string) (string, string, error) {
	id := spelleffectsvc.Slug(spellID)
	if id == "" {
		return "", "", fmt.Errorf("%w: choose the spell to cast from this Rary's slot", domain.ErrInvalidInput)
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
