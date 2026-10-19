package itemsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// Power is one of an item's own powers (ADR 0016 steps 9 to 16): effects in
// the buffs' vocabulary that apply while its trigger is met, for as long as
// the item is carried. It has an id (a slug, unique within its item), an
// optional trigger, MinStr (on while the effective STR is at least that;
// nil: always on), and no name, rounds or charges: the item names it. Powers
// come from the seed, data migrations and the equipment editor (ADR 0033
// step 17, amending step 15), validated alike.
type Power struct {
	ID      string
	MinStr  *int
	Effects buffsvc.BuffEffects
}

// maxAbility is the highest score a trigger may name (AD&D 2.5 tops out at
// 25), as for a buff's min_str.
const maxAbility = 25

var powerIDRE = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)

// ValidatePowers checks an item's power list (ADR 0016 steps 9 and 10): ids
// are slugs, unique within the item; a trigger names a score in 1..25; the
// effects pass the buffs' checks (buffsvc.ValidateEffects) and hold no
// Strength effect, so a power never changes the effective STR its trigger
// reads. ErrInvalidInput names the power and the key.
func ValidatePowers(powers []Power) error {
	seen := make(map[string]bool, len(powers))
	for i, p := range powers {
		if !powerIDRE.MatchString(p.ID) {
			return fmt.Errorf("%w: power %d: id %q must be a slug (a-z, 0-9, _)", domain.ErrInvalidInput, i, p.ID)
		}
		if seen[p.ID] {
			return fmt.Errorf("%w: power %q: duplicate id", domain.ErrInvalidInput, p.ID)
		}
		seen[p.ID] = true
		if p.MinStr != nil && (*p.MinStr < 1 || *p.MinStr > maxAbility) {
			return fmt.Errorf("%w: power %q: trigger min_str must be between 1 and %d", domain.ErrInvalidInput, p.ID, maxAbility)
		}
		e := p.Effects
		switch {
		case e.StrBonus != nil:
			return fmt.Errorf("%w: power %q: str_bonus is a Strength effect, a power has none", domain.ErrInvalidInput, p.ID)
		case e.StrMax != nil:
			return fmt.Errorf("%w: power %q: str_max is a Strength effect, a power has none", domain.ErrInvalidInput, p.ID)
		case e.MinStr != nil:
			return fmt.Errorf("%w: power %q: min_str is the trigger, not an effect", domain.ErrInvalidInput, p.ID)
		}
		if err := buffsvc.ValidateEffects(e); err != nil {
			return fmt.Errorf("power %q: %w", p.ID, err)
		}
	}
	return nil
}

// MagicPowerID is the reserved id of an armour's or a shield's "+N" (ADR 0033
// step 17): the one power of that id is one effect ac -N, no trigger and no
// condition. The AC breakdown names it armour magic or shield magic.
const MagicPowerID = "magic"

// MagicPower is the power an armour's or a shield's "+N" stands for (ADR
// 0033 step 17): ac -n.
func MagicPower(n int) Power {
	ac := -n
	return Power{ID: MagicPowerID, Effects: buffsvc.BuffEffects{AC: &ac}}
}

// validPiecePowers is the editor's check of a piece's powers (ADR 0033 step
// 17): the seed's (ValidatePowers), and the reserved magic power only on an
// armour or a shield, as one ac effect of -1 or better (the "+N", N at least
// 1, at most MaxBonus) with no trigger and no condition. A refusal names the
// field powers.
func (i Item) validPiecePowers() error {
	if err := ValidatePowers(i.Powers); err != nil {
		return &domain.FieldError{Field: "powers", Message: strings.TrimPrefix(err.Error(), domain.ErrInvalidInput.Error()+": ")}
	}
	for _, p := range i.Powers {
		if p.ID != MagicPowerID {
			continue
		}
		if i.Category != CategoryArmour && i.Category != CategoryShield {
			return fieldErr("powers", `Only an armour or a shield has the "magic" power (its +N)`)
		}
		rest := p.Effects
		rest.AC = nil
		if p.MinStr != nil || rest != (buffsvc.BuffEffects{}) || magicN(p) < 1 || magicN(p) > MaxBonus {
			return domain.FieldErrorf("powers", `The "magic" power is one ac effect of -1 to -%d, without trigger or condition`, MaxBonus)
		}
	}
	return nil
}

// magicN is the N of a magic power's ac -N (0 without an ac effect).
func magicN(p Power) int {
	if p.Effects.AC == nil {
		return 0
	}
	return -*p.Effects.AC
}

// withPowerIDs gives every power sent without an id the first free
// power_<n> (n from 1), so the editor need not invent slugs; an id sent is
// kept as is (ValidatePowers checks it).
func withPowerIDs(powers []Power) []Power {
	if len(powers) == 0 {
		return nil
	}
	out := slices.Clone(powers)
	taken := map[string]bool{}
	for _, p := range out {
		taken[p.ID] = true
	}
	n := 1
	for i := range out {
		if out[i].ID != "" {
			continue
		}
		for taken["power_"+strconv.Itoa(n)] {
			n++
		}
		out[i].ID = "power_" + strconv.Itoa(n)
		taken[out[i].ID] = true
	}
	return out
}

// jsonPower is the one encoding of a power: the items.powers column, the
// seed's "powers" and the items:<id> snapshots. Add fields, never rename.
type jsonPower struct {
	ID      string       `json:"id"`
	Trigger *jsonTrigger `json:"trigger,omitempty"`
	Effects jsonEffects  `json:"effects"`
}

type jsonTrigger struct {
	MinStr *int `json:"min_str,omitempty"`
}

// jsonEffects mirrors buffsvc.BuffEffects with the buffs' wire keys.
type jsonEffects struct {
	MinStr     *int               `json:"min_str,omitempty"`
	SpellsBuff *float64           `json:"spells_buff,omitempty"`
	WeaponBuff *int               `json:"weapon_buff,omitempty"`
	Multiplier *float64           `json:"multiplier,omitempty"`
	SaveBonus  *int               `json:"save_bonus,omitempty"`
	StrBonus   *int               `json:"str_bonus,omitempty"`
	StrMax     *int               `json:"str_max,omitempty"`
	AC         *int               `json:"ac,omitempty"`
	ToHit      *int               `json:"to_hit,omitempty"`
	Damage     *int               `json:"damage,omitempty"`
	BaseAC     *int               `json:"base_ac,omitempty"`
	WeaponKind buffsvc.WeaponKind `json:"weapon_kind,omitempty"`
	// WithoutMagicArmour is the AC effects' condition (ADR 0033 step 12).
	WithoutMagicArmour bool `json:"without_magic_armour,omitempty"`
}

func toJSONPowers(powers []Power) []jsonPower {
	out := make([]jsonPower, 0, len(powers))
	for _, p := range powers {
		j := jsonPower{ID: p.ID, Effects: jsonEffects(p.Effects)}
		if p.MinStr != nil {
			j.Trigger = &jsonTrigger{MinStr: p.MinStr}
		}
		out = append(out, j)
	}
	return out
}

// fromJSONPowers decodes a list; none is nil, like an item without powers.
func fromJSONPowers(js []jsonPower) []Power {
	if len(js) == 0 {
		return nil
	}
	out := make([]Power, 0, len(js))
	for _, j := range js {
		p := Power{ID: j.ID, Effects: buffsvc.BuffEffects(j.Effects)}
		if j.Trigger != nil {
			p.MinStr = j.Trigger.MinStr
		}
		out = append(out, p)
	}
	return out
}

// EncodePowers is the stored JSON of a power list ("[]" for none).
func EncodePowers(powers []Power) (string, error) {
	b, err := json.Marshal(toJSONPowers(powers))
	if err != nil {
		return "", fmt.Errorf("encode powers: %w", err)
	}
	return string(b), nil
}

// DecodePowers reads a stored power list; "" and "[]" read as none (nil). Unknown keys are
// refused, so a typo in the seed or a migration cannot drop an effect.
func DecodePowers(raw string) ([]Power, error) {
	if raw == "" {
		return nil, nil
	}
	var js []jsonPower
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&js); err != nil {
		return nil, fmt.Errorf("decode powers: %w", err)
	}
	return fromJSONPowers(js), nil
}

// ItemPowers is every power of every worn or wielded piece (ADR 0033 step 9),
// in item list order then
// power order, whatever the item's count (ADR 0016 step 9: the Power Gem's
// powers work at 0 dispels left). It is buffsvc's Items port; whether a
// power is on is buffsvc's to decide (step 10).
func (s *service) ItemPowers(ctx context.Context, characterID int64) ([]buffsvc.ItemPower, error) {
	items, err := s.dal.List(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	out := []buffsvc.ItemPower{}
	for _, it := range items {
		// A carried ring protects nothing (ADR 0033 step 9).
		if !it.Equipped() {
			continue
		}
		for _, p := range it.Powers {
			out = append(out, buffsvc.ItemPower{
				ItemID: it.ID, ItemName: it.Name.Any(), PowerID: p.ID, MinStr: p.MinStr, Effects: p.Effects,
			})
		}
	}
	return out, nil
}
