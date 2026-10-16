package buffsvc

import (
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
)

// maxAbility is the highest ability score a buff may name (AD&D 2.5 tops
// out at 25).
const maxAbility = 25

// strengthCap is the highest STR that stacked Strength bonuses reach
// (PHB Strength: it "cannot confer a Strength of 19 or more").
const strengthCap = 18

// isImproved reports whether e is an Improved Strength bonus: a StrBonus
// whose StrMax is above 18 (the seed caps it at 23 for a bard, 25 for a
// warrior). It is the only Strength magic that goes past 18.
func isImproved(e BuffEffects) bool {
	return e.StrBonus != nil && e.StrMax != nil && *e.StrMax > strengthCap
}

// isStrengthBonus reports whether e is a stacking Strength bonus: any
// StrBonus that is not Improved Strength (wizard L2 Strength, or a hand-made
// bonus with or without a StrMax).
func isStrengthBonus(e BuffEffects) bool {
	return e.StrBonus != nil && !isImproved(e)
}

// effectiveStr is the STR score with buffs (ADR 0004 steps 7 to 9). STR
// effects are not themselves gated by MinStr; no result is ever lower than
// base.
//
//  1. No Improved Strength active: base + the SUM of every Strength bonus,
//     total capped at 18 (each bonus's own StrMax is not used).
//  2. Improved Strength active: it REPLACES every other Strength bonus; the
//     best single Improved wins, base + bonus capped at its StrMax (23).
func effectiveStr(buffs []BuffEffect, base int) int {
	improved, hasImproved := base, false
	sum := 0
	for _, b := range buffs {
		e := b.Effects
		switch {
		case isImproved(e):
			hasImproved = true
			improved = max(improved, min(base+*e.StrBonus, *e.StrMax))
		case isStrengthBonus(e):
			sum += *e.StrBonus
		}
	}
	if hasImproved {
		return improved
	}
	return max(base, min(base+sum, strengthCap))
}

// improvedActive reports whether an Improved Strength is among buffs.
func improvedActive(buffs []BuffEffect) bool {
	for _, b := range buffs {
		if isImproved(b.Effects) {
			return true
		}
	}
	return false
}

// aggregate is the pure effect-stacking core: given a list of buffs, the
// carried items' powers and the character's base STR, return the combined
// modifier set for a roll made with a weapon of kind (WeaponNone: a save or
// a spell's to-hit). The effective STR comes from the buffs alone (a power
// has no Strength effect, ADR 0016 step 9); buffs whose requirements
// (MinStr) aren't met are skipped, so are powers whose trigger isn't (ADR
// 0016 step 10). Buffs fold first, in list order, then the powers in item
// and power order (step 11), so a weapon +N tie goes to the buff.
func aggregate(buffs []BuffEffect, powers []ItemPower, baseStr int, kind WeaponKind) BuffModifiers {
	str := effectiveStr(buffs, baseStr)
	mods := BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: str}
	for _, b := range buffs {
		if !meetsRequirements(b.Effects, str) {
			continue
		}
		applyEffects(&mods, source{name: b.Name}, b.Effects, kind)
	}
	for _, p := range powers {
		if !p.On(str) {
			continue
		}
		applyEffects(&mods, source{name: p.ItemName, itemID: p.ItemID}, p.Effects, kind)
	}
	return mods
}

// powersOn is the powers whose trigger the effective STR of buffs meets
// (ADR 0016 step 13: a power that is off is shown nowhere).
func powersOn(buffs []BuffEffect, powers []ItemPower, baseStr int) []ItemPower {
	str := effectiveStr(buffs, baseStr)
	out := []ItemPower{}
	for _, p := range powers {
		if p.On(str) {
			out = append(out, p)
		}
	}
	return out
}

// source names a contribution: a buff's name, or an item power's item (id
// and stored name, ADR 0016 step 14).
type source struct {
	name   string
	itemID string
}

// statuses marks each buff with whether its effects apply at baseStr. A
// Strength bonus does not apply while an Improved Strength replaces it
// (ADR 0004 step 8).
func statuses(buffs []BuffEffect, baseStr int) []BuffStatus {
	str := effectiveStr(buffs, baseStr)
	replaced := improvedActive(buffs)
	out := make([]BuffStatus, len(buffs))
	for i, b := range buffs {
		applies := meetsRequirements(b.Effects, str) && (!replaced || !isStrengthBonus(b.Effects))
		out[i] = BuffStatus{BuffEffect: b, Applies: applies}
	}
	return out
}

func meetsRequirements(e BuffEffects, str int) bool {
	if e.MinStr != nil && str < *e.MinStr {
		return false
	}
	return true
}

// applyEffects folds one applying buff or power into m. Multipliers
// multiply, flat bonuses add, and "weapon becomes +N" effects do not stack:
// the best wins (the first on a tie). Effects with a weapon-kind condition
// count only for a roll made with a weapon of that kind (ADR 0004 step 18).
func applyEffects(m *BuffModifiers, src source, e BuffEffects, kind WeaponKind) {
	if e.WeaponKind != WeaponNone && e.WeaponKind != kind {
		return
	}
	if e.SpellsBuff != nil {
		m.SpellsBuff *= *e.SpellsBuff
		m.SpellsBuffFrom = append(m.SpellsBuffFrom, Factor{Buff: src.name, ItemID: src.itemID, Value: *e.SpellsBuff})
	}
	if e.WeaponBuff != nil && *e.WeaponBuff > m.WeaponBuff {
		m.WeaponBuff = *e.WeaponBuff
		m.WeaponBuffFrom = src.name
		m.WeaponBuffItemID = src.itemID
	}
	if e.Multiplier != nil {
		m.Multipliers *= *e.Multiplier
		m.MultipliersFrom = append(m.MultipliersFrom, Factor{Buff: src.name, ItemID: src.itemID, Value: *e.Multiplier})
	}
	if e.ToHit != nil && *e.ToHit != 0 {
		m.ToHit += *e.ToHit
		m.ToHitFrom = append(m.ToHitFrom, Contribution{Buff: src.name, ItemID: src.itemID, Value: *e.ToHit})
	}
	if e.Damage != nil && *e.Damage != 0 {
		m.Damage += *e.Damage
		m.DamageFrom = append(m.DamageFrom, Contribution{Buff: src.name, ItemID: src.itemID, Value: *e.Damage})
	}
	if e.SaveBonus != nil && *e.SaveBonus != 0 {
		m.SaveBonus += *e.SaveBonus
		m.SaveBonusFrom = append(m.SaveBonusFrom, Contribution{Buff: src.name, ItemID: src.itemID, Value: *e.SaveBonus})
	}
	if e.AC != nil {
		m.AC += *e.AC
	}
}

// validateBuff checks a new buff. Counters are never negative; its effects
// pass ValidateEffects.
func validateBuff(b BuffEffect) error {
	if strings.TrimSpace(b.Name) == "" {
		return fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	if b.Duration < 0 || b.Charges < 0 {
		return fmt.Errorf("%w: rounds and charges cannot be negative", domain.ErrInvalidInput)
	}
	return ValidateEffects(b.Effects)
}

// ValidateEffects checks an effect set, a buff's or an item power's (ADR
// 0016 step 9): STR effects name a score in 1..25 and a bonus is positive, a
// weapon +N is positive, a weapon-kind condition is melee or missile (ADR
// 0004 step 18). ErrInvalidInput names the bad key.
func ValidateEffects(e BuffEffects) error {
	for _, s := range []struct {
		name string
		v    *int
	}{{"str_max", e.StrMax}, {"min_str", e.MinStr}} {
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
	if !e.WeaponKind.Valid() {
		return fmt.Errorf("%w: weapon_kind must be %q or %q", domain.ErrInvalidInput, WeaponMelee, WeaponMissile)
	}
	return nil
}

// advanceRoundBuffs decrements timed buffs by 1 and partitions into
// (still-active, just-expired). Untimed buffs pass through.
func advanceRoundBuffs(buffs []BuffEffect) (active, expired []BuffEffect) {
	return elapseBuffs(buffs, 1)
}

// elapseBuffs counts timed buffs down by rounds and partitions into
// (still-active, just-expired); an expired buff's Duration is 0. Untimed
// buffs (charge-only, until dismissed) pass through unchanged.
func elapseBuffs(buffs []BuffEffect, rounds int) (active, expired []BuffEffect) {
	active = []BuffEffect{}
	expired = []BuffEffect{}
	for _, b := range buffs {
		if !b.Timed() {
			active = append(active, b)
			continue
		}
		b.Duration -= rounds
		if b.Duration > 0 {
			active = append(active, b)
		} else {
			b.Duration = 0
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
		// ADR 0004 step 16: a known total is kept, but never falls below
		// the charges left.
		if b.ChargesTotal > 0 && b.Charges > b.ChargesTotal {
			b.ChargesTotal = b.Charges
		}
	}
	if e.Note != nil {
		b.Note = strings.TrimSpace(*e.Note)
	}
	if (before.Timed() && !b.Timed()) || (before.Charged() && !b.Charged()) {
		return without(buffs, i), b, true, nil
	}
	return replaced(buffs, i, b), b, false, nil
}
