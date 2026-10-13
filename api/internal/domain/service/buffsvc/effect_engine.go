package buffsvc

import (
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
)

// maxAbility is the highest ability score a buff may name (AD&D 2.5 tops
// out at 25).
const maxAbility = 25

// effectiveStr is the STR score with buffs. STR magic is not cumulative
// (PHB Strength: "not cumulative with other Strength magic"), so the best
// single effect wins: a StrSet raises the score to its value, a StrBonus
// gives base+bonus capped at its StrMax. Neither ever lowers the score.
// STR effects are not themselves gated by MinStr.
func effectiveStr(buffs []BuffEffect, base int) int {
	best := base
	for _, b := range buffs {
		e := b.Effects
		if e.StrSet != nil && *e.StrSet > best {
			best = *e.StrSet
		}
		if e.StrBonus != nil {
			v := base + *e.StrBonus
			if e.StrMax != nil && v > *e.StrMax {
				v = *e.StrMax
			}
			if v > best {
				best = v
			}
		}
	}
	return best
}

// aggregate is the pure effect-stacking core: given a list of buffs and the
// character's base STR, return the combined modifier set. Buffs whose
// requirements (MinStr, checked against the effective STR) aren't met are
// skipped.
func aggregate(buffs []BuffEffect, baseStr int) BuffModifiers {
	str := effectiveStr(buffs, baseStr)
	mods := BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: str}
	for _, b := range buffs {
		if !meetsRequirements(b.Effects, str) {
			continue
		}
		applyEffects(&mods, b.Name, b.Effects)
	}
	return mods
}

// statuses marks each buff with whether its effects apply at baseStr.
func statuses(buffs []BuffEffect, baseStr int) []BuffStatus {
	str := effectiveStr(buffs, baseStr)
	out := make([]BuffStatus, len(buffs))
	for i, b := range buffs {
		out[i] = BuffStatus{BuffEffect: b, Applies: meetsRequirements(b.Effects, str)}
	}
	return out
}

func meetsRequirements(e BuffEffects, str int) bool {
	if e.MinStr != nil && str < *e.MinStr {
		return false
	}
	return true
}

// applyEffects folds one applying buff into m. Multipliers multiply, flat
// bonuses add, and "weapon becomes +N" effects do not stack: the best wins.
func applyEffects(m *BuffModifiers, name string, e BuffEffects) {
	if e.SpellsBuff != nil {
		m.SpellsBuff *= *e.SpellsBuff
		m.SpellsBuffFrom = append(m.SpellsBuffFrom, Factor{Buff: name, Value: *e.SpellsBuff})
	}
	if e.WeaponBuff != nil && *e.WeaponBuff > m.WeaponBuff {
		m.WeaponBuff = *e.WeaponBuff
		m.WeaponBuffFrom = name
	}
	if e.Multiplier != nil {
		m.Multipliers *= *e.Multiplier
		m.MultipliersFrom = append(m.MultipliersFrom, Factor{Buff: name, Value: *e.Multiplier})
	}
	if e.ToHit != nil && *e.ToHit != 0 {
		m.ToHit += *e.ToHit
		m.ToHitFrom = append(m.ToHitFrom, Contribution{Buff: name, Value: *e.ToHit})
	}
	if e.Damage != nil && *e.Damage != 0 {
		m.Damage += *e.Damage
		m.DamageFrom = append(m.DamageFrom, Contribution{Buff: name, Value: *e.Damage})
	}
	if e.SaveBonus != nil {
		m.SaveBonus += *e.SaveBonus
	}
	if e.AC != nil {
		m.AC += *e.AC
	}
}

// validateBuff checks a new buff. Counters are never negative; STR effects
// name a score in 1..25 and a bonus is positive.
func validateBuff(b BuffEffect) error {
	if strings.TrimSpace(b.Name) == "" {
		return fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	if b.Duration < 0 || b.Charges < 0 {
		return fmt.Errorf("%w: rounds and charges cannot be negative", domain.ErrInvalidInput)
	}
	e := b.Effects
	for _, s := range []struct {
		name string
		v    *int
	}{{"str_set", e.StrSet}, {"str_max", e.StrMax}, {"min_str", e.MinStr}} {
		if s.v != nil && (*s.v < 1 || *s.v > maxAbility) {
			return fmt.Errorf("%w: %s must be between 1 and %d", domain.ErrInvalidInput, s.name, maxAbility)
		}
	}
	if e.StrBonus != nil && *e.StrBonus < 1 {
		return fmt.Errorf("%w: str_bonus must be positive", domain.ErrInvalidInput)
	}
	if e.WeaponBuff != nil && *e.WeaponBuff < 1 {
		return fmt.Errorf("%w: weapon_buff must be positive (the weapon becomes +N)", domain.ErrInvalidInput)
	}
	return nil
}

// advanceRoundBuffs decrements timed buffs by 1 and partitions into
// (still-active, just-expired). Untimed buffs pass through.
func advanceRoundBuffs(buffs []BuffEffect) (active []BuffEffect, expired []BuffEffect) {
	active = []BuffEffect{}
	expired = []BuffEffect{}
	for _, b := range buffs {
		if !b.Timed() {
			active = append(active, b)
			continue
		}
		b.Duration--
		if b.Duration > 0 {
			active = append(active, b)
		} else {
			expired = append(expired, b)
		}
	}
	return active, expired
}

// indexOf returns the position of the buff with id, or -1.
func indexOf(buffs []BuffEffect, id string) int {
	for i, b := range buffs {
		if b.ID == id {
			return i
		}
	}
	return -1
}

// without returns a copy of buffs without the element at i.
func without(buffs []BuffEffect, i int) []BuffEffect {
	out := make([]BuffEffect, 0, len(buffs))
	out = append(out, buffs[:i]...)
	return append(out, buffs[i+1:]...)
}

// replaced returns a copy of buffs with the element at i set to b.
func replaced(buffs []BuffEffect, i int, b BuffEffect) []BuffEffect {
	out := append([]BuffEffect(nil), buffs...)
	out[i] = b
	return out
}

// absorb uses one charge of the buff at i: the buff ends when its charges
// reach 0. ended reports that.
func absorb(buffs []BuffEffect, i int) (next []BuffEffect, after BuffEffect, ended bool, err error) {
	b := buffs[i]
	if !b.Charged() {
		return nil, b, false, fmt.Errorf("%w: %s has no charges to absorb a hit", domain.ErrInvalidInput, b.Name)
	}
	b.Charges--
	if b.Charges == 0 {
		return without(buffs, i), b, true, nil
	}
	return replaced(buffs, i, b), b, false, nil
}

// edit applies e to the buff at i. A counter the buff used (rounds or
// charges) set to 0 ends the buff; ended reports that.
func edit(buffs []BuffEffect, i int, e BuffEdit) (next []BuffEffect, after BuffEffect, ended bool, err error) {
	if e.Duration == nil && e.Charges == nil && e.Note == nil {
		return nil, BuffEffect{}, false, fmt.Errorf("%w: nothing to change", domain.ErrInvalidInput)
	}
	before := buffs[i]
	b := before
	if e.Duration != nil {
		if *e.Duration < 0 {
			return nil, b, false, fmt.Errorf("%w: rounds cannot be negative", domain.ErrInvalidInput)
		}
		b.Duration = *e.Duration
	}
	if e.Charges != nil {
		if *e.Charges < 0 {
			return nil, b, false, fmt.Errorf("%w: charges cannot be negative", domain.ErrInvalidInput)
		}
		b.Charges = *e.Charges
	}
	if e.Note != nil {
		b.Note = strings.TrimSpace(*e.Note)
	}
	if (before.Timed() && !b.Timed()) || (before.Charged() && !b.Charged()) {
		return without(buffs, i), b, true, nil
	}
	return replaced(buffs, i, b), b, false, nil
}
