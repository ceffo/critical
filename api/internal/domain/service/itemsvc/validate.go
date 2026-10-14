package itemsvc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
)

// Limits on what the item editor accepts.
const (
	MaxItemCount = 9999
	MaxBonus     = 10
	maxDiceCount = 100
	maxDiceSides = 1000
	maxDiceMod   = 1000
)

// SaveCategories are the save categories an item's effect may allow, as
// they appear on the wire (the same names as the character's saves).
var SaveCategories = []string{"para_poison", "rod_staff_wand", "petri_poly", "breath", "spell"}

var diceRE = regexp.MustCompile(`^(\d+)d(\d+)([+-]\d+)?$`)

// ParseDice parses "NdM" or "NdM+K" (case-insensitive, K may be negative).
func ParseDice(dice string) (n, sides, mod int, err error) {
	m := diceRE.FindStringSubmatch(strings.ToLower(strings.TrimSpace(dice)))
	if m == nil {
		return 0, 0, 0, fmt.Errorf("invalid effect dice %q", dice)
	}
	n, _ = strconv.Atoi(m[1])
	sides, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		mod, _ = strconv.Atoi(m[3])
	}
	if n < 1 || n > maxDiceCount || sides < 2 || sides > maxDiceSides || mod > maxDiceMod || mod < -maxDiceMod {
		return 0, 0, 0, fmt.Errorf("invalid effect dice %q", dice)
	}
	return n, sides, mod, nil
}

func fieldErr(field, msg string) error { return &domain.FieldError{Field: field, Message: msg} }

// Validate checks a whole item as it would be stored, returning a
// *domain.FieldError naming the first bad field (wire names).
func (i Item) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return fieldErr("name", "Name is required")
	}
	if !i.Behaviour.Valid() {
		return fieldErr("behaviour", "Behaviour must be stock, charges or daily")
	}
	if i.Count < 0 {
		return fieldErr("count", "Count must be 0 or more")
	}
	if i.Count > MaxItemCount {
		return fieldErr("count", fmt.Sprintf("Count must be at most %d", MaxItemCount))
	}
	// A charges item at 0 is gone (Removed, Use): removing it is what remove is for.
	if i.Behaviour.RemovedAtZero() && i.Count == 0 {
		return fieldErr("count", "A charges item needs at least 1 charge (remove it instead)")
	}
	if i.Behaviour == BehaviourDaily {
		if i.MaxCount < 1 {
			return fieldErr("maxCount", "Daily number must be at least 1")
		}
		if i.MaxCount > MaxItemCount {
			return fieldErr("maxCount", fmt.Sprintf("Daily number must be at most %d", MaxItemCount))
		}
		if i.Count > i.MaxCount {
			return fieldErr("count", "Count cannot exceed the daily number")
		}
	}
	if i.MagicBonus < -MaxBonus || i.MagicBonus > MaxBonus {
		return fieldErr("magicalBonus", fmt.Sprintf("Magical bonus must be between -%d and %d", MaxBonus, MaxBonus))
	}
	if i.EffectDice != "" {
		if _, _, _, err := ParseDice(i.EffectDice); err != nil {
			return fieldErr("effectDice", "Effect dice must look like 2d6 or 1d8+2")
		}
	}
	if i.EffectSave != "" {
		ok := false
		for _, c := range SaveCategories {
			ok = ok || c == i.EffectSave
		}
		if !ok {
			return fieldErr("effectSave", "Unknown save category")
		}
	}
	return nil
}
