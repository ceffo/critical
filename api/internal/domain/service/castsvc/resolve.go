package castsvc

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
)

// roll evaluates f at the caster level, recording every die.
func (s *service) roll(f *formula.Formula, level int) (Roll, error) {
	r := Roll{Formula: f.String()}
	total, err := f.Eval(level, func(sides int) int {
		v := s.Dice.RollDie(sides)
		r.Dice = append(r.Dice, DieRoll{Sides: sides, Value: v})
		return v
	})
	if err != nil {
		return Roll{}, fmt.Errorf("roll %s at level %d: %w", f, level, err)
	}
	r.Total = total
	return r, nil
}

// rollDamage rolls a damage entry at the caster level: the dice of the
// level's row, once per instance, every round it repeats.
func (s *service) rollDamage(d *spelleffectsvc.Damage, level int) (*DamageResult, error) {
	dice := d.DiceAt(level)
	if dice == nil {
		return nil, fmt.Errorf("no damage dice at caster level %d", level)
	}
	out := &DamageResult{Dice: dice.String(), Type: d.Type, Instances: 1, Rounds: 1}
	if d.Instances != nil {
		n, err := s.roll(d.Instances, level)
		if err != nil {
			return nil, err
		}
		out.Instances = spelleffectsvc.InstanceCount(n.Total)
	}
	if d.Rounds != nil {
		n, err := s.roll(d.Rounds, level)
		if err != nil {
			return nil, err
		}
		out.Rounds = spelleffectsvc.RoundCount(n.Total)
	}
	for range out.Rounds {
		for range out.Instances {
			r, err := s.roll(dice, level)
			if err != nil {
				return nil, err
			}
			out.Rolls = append(out.Rolls, r)
			out.Total += r.Total
		}
	}
	return out, nil
}

// trackedCharges are the charge units buffsvc counts down one hit at a time.
var trackedCharges = map[string]bool{"attacks": true, "images": true}

// rollBuff turns a buff entry into a buff on the Bard at the caster level,
// plus the effects the engine does not track.
func (s *service) rollBuff(e spelleffectsvc.Entry, level int) (*AppliedBuff, []Untracked, error) {
	out := &AppliedBuff{Buff: buffsvc.BuffEffect{Name: e.Name}}
	var untracked []Untracked

	if e.Duration.Kind == spelleffectsvc.DurationRounds && e.Duration.Rounds != nil {
		r, err := s.roll(e.Duration.Rounds, level)
		if err != nil {
			return nil, nil, err
		}
		out.Duration = &r
		out.Buff.Duration = max(r.Total, 0)
	}
	if e.Charges != nil {
		r, err := s.roll(e.Charges.Count, level)
		if err != nil {
			return nil, nil, err
		}
		if trackedCharges[e.Charges.Unit] {
			out.Charges = &r
			out.ChargeUnit = e.Charges.Unit
			out.Buff.Charges = max(r.Total, 0)
		} else {
			untracked = append(untracked, Untracked{Key: "charges_" + e.Charges.Unit, Value: r.Total})
		}
	}
	if e.Buff == nil {
		return out, untracked, nil
	}

	fx := &out.Buff.Effects
	for _, key := range sortedKeys(e.Buff.Modifiers) {
		r, err := s.roll(e.Buff.Modifiers[key], level)
		if err != nil {
			return nil, nil, err
		}
		v := r.Total
		// Conditional bonuses (vs evil, from the front) are not tracked:
		// the engine has no notion of the condition.
		if e.Buff.Condition != "" || !applyModifier(fx, key, v) {
			untracked = append(untracked, Untracked{Key: key, Value: v, Condition: e.Buff.Condition})
		}
	}
	for _, ability := range sortedKeys(e.Buff.AbilityGain) {
		g := e.Buff.AbilityGain[ability]
		f := g.Gain
		if gf, ok := g.ByGroup[ClassGroup]; ok {
			f = gf
		}
		if f == nil {
			continue
		}
		r, err := s.roll(f, level)
		if err != nil {
			return nil, nil, err
		}
		limit := g.Max
		if m, ok := g.MaxByGroup[ClassGroup]; ok {
			limit = m
		}
		if ability == "str" && r.Total > 0 {
			out.StrGain = &r
			fx.StrBonus = ptr(r.Total)
			if limit > 0 {
				fx.StrMax = ptr(limit)
			}
			continue
		}
		untracked = append(untracked, Untracked{Key: ability + "_gain", Value: r.Total})
	}
	// Ability sets are shown, never applied: the only one (Fist of Stone,
	// STR 18/00) is for the fist alone and must not raise STR for every
	// attack (it would turn the Power Gem on).
	for _, ability := range sortedKeys(e.Buff.AbilitySet) {
		score, err := strconv.Atoi(strings.SplitN(e.Buff.AbilitySet[ability], "/", 2)[0])
		if err != nil {
			return nil, nil, fmt.Errorf("ability_set %s %q: %w", ability, e.Buff.AbilitySet[ability], err)
		}
		untracked = append(untracked, Untracked{Key: ability + "_set", Value: score})
	}
	return out, untracked, nil
}

// applyModifier maps a seed modifier onto the buff effects the engine
// applies; false when it has no counterpart (or is 0).
func applyModifier(fx *buffsvc.BuffEffects, key string, v int) bool {
	if v == 0 {
		return false
	}
	switch key {
	case "to_hit":
		fx.ToHit = ptr(v)
	case "damage":
		fx.Damage = ptr(v)
	case "ac":
		fx.AC = ptr(v)
	case "save":
		fx.SaveBonus = ptr(v)
	case "weapon_buff":
		if v < 1 {
			return false
		}
		fx.WeaponBuff = ptr(v)
	case "damage_multiplier_pct":
		// Enlarge: 100% + 10% per level multiplies the weapon damage roll.
		f := float64(v) / 100
		fx.SpellsBuff = &f
	default:
		return false
	}
	return true
}

// hostile reports a buff whose effects hurt the bearer (Slow, Malison): an
// area spell the Bard casts does not land on him by default.
func hostile(b *AppliedBuff, untracked []Untracked) bool {
	fx := b.Buff.Effects
	neg := func(p *int) bool { return p != nil && *p < 0 }
	if neg(fx.ToHit) || neg(fx.Damage) || neg(fx.SaveBonus) || (fx.AC != nil && *fx.AC > 0) {
		return true
	}
	for _, u := range untracked {
		if (u.Key == "move_pct" || u.Key == "attacks_pct") && u.Value < 100 {
			return true
		}
	}
	return false
}

// appliesToSelf decides whether a buff the Bard casts lands on him: always
// for self-only spells; else the player's choice, defaulting to yes for
// "self or other", yes for an area unless it is hostile, no for "other".
func appliesToSelf(target spelleffectsvc.Target, onSelf *bool, isHostile bool) bool {
	if target == spelleffectsvc.TargetSelf {
		return true
	}
	if onSelf != nil {
		return *onSelf
	}
	switch target {
	case spelleffectsvc.TargetSelfOrOther:
		return true
	case spelleffectsvc.TargetArea:
		return !isHostile
	default:
		return false
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func ptr[T any](v T) *T { return &v }
