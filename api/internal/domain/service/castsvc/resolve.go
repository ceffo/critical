package castsvc

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
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
// level's row, once per instance, every round it repeats. A damage made of
// parts rolls each part of each instance once, in the data's order (ADR 0008
// step 13). d is the entry's damage, or the damage of the cast-time mode
// picked (ADR 0008 step 14).
func (s *service) rollDamage(e spelleffectsvc.Entry, d *spelleffectsvc.Damage, level int) (*DamageResult, error) {
	if d.Steps != nil {
		return s.rollDamageSteps(d, level)
	}
	if len(d.Parts) > 0 {
		return s.rollDamageParts(e, d, level)
	}
	dice := d.DiceAt(level)
	if dice == nil {
		return nil, fmt.Errorf("no damage dice at caster level %d", level)
	}
	out := &DamageResult{Dice: dice.String(), Type: d.Type, Instances: 1, Rounds: 1}
	if err := s.rollRepeats(d, level, out); err != nil {
		return nil, err
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

// rollDamageSteps rolls a damage made of dice steps (ADR 0008 step 15): every
// step DamageSteps.At serves at the level, once, in order, each roll keeping
// its step number; a step without damage rolls nothing (total 0). The steps
// are the rounds or the instances.
func (s *service) rollDamageSteps(d *spelleffectsvc.Damage, level int) (*DamageResult, error) {
	steps, err := d.Steps.At(level)
	if err != nil {
		return nil, err
	}
	out := &DamageResult{Type: d.Type, Instances: 1, Rounds: 1, StepsPer: d.Steps.Per}
	if d.Steps.Per == spelleffectsvc.StepPerRound {
		out.Rounds = len(steps)
	} else {
		out.Instances = len(steps)
	}
	for _, st := range steps {
		f := d.Steps.FormulaOf(st.Step)
		r := Roll{Formula: f.String(), Step: st.Step}
		total, err := f.EvalStep(level, st.Step, func(sides int) int {
			v := s.Dice.RollDie(sides)
			r.Dice = append(r.Dice, DieRoll{Sides: sides, Value: v})
			return v
		})
		if err != nil {
			return nil, fmt.Errorf("roll %s at level %d step %d: %w", f, level, st.Step, err)
		}
		r.Total = total
		out.Rolls = append(out.Rolls, r)
		out.Total += total
	}
	return out, nil
}

// rollRepeats rolls the number of instances and of rounds into out.
func (s *service) rollRepeats(d *spelleffectsvc.Damage, level int, out *DamageResult) error {
	if d.Instances != nil {
		n, err := s.roll(d.Instances, level)
		if err != nil {
			return err
		}
		out.Instances = spelleffectsvc.InstanceCount(n.Total)
	}
	if d.Rounds != nil {
		n, err := s.roll(d.Rounds, level)
		if err != nil {
			return err
		}
		out.Rounds = spelleffectsvc.RoundCount(n.Total)
	}
	return nil
}

// rollDamageParts rolls a damage made of parts: every instance rolls each
// part once; the instance roll adds the parts up. TotalOnSave leaves out the
// parts a save negates.
func (s *service) rollDamageParts(e spelleffectsvc.Entry, d *spelleffectsvc.Damage, level int) (*DamageResult, error) {
	dice := make([]*formula.Formula, len(d.Parts))
	out := &DamageResult{Type: d.Type, Instances: 1, Rounds: 1, Parts: make([]DamagePart, len(d.Parts))}
	formulas := make([]string, len(d.Parts))
	for i, p := range d.Parts {
		if dice[i] = p.DiceAt(level); dice[i] == nil {
			return nil, fmt.Errorf("no damage dice for part %d at caster level %d", i, level)
		}
		formulas[i] = dice[i].String()
		out.Parts[i] = DamagePart{Formula: formulas[i], Save: e.PartSave(p)}
	}
	out.Dice = strings.Join(formulas, "+")
	if err := s.rollRepeats(d, level, out); err != nil {
		return nil, err
	}
	onSave := 0
	for range out.Rounds {
		for range out.Instances {
			inst := Roll{Formula: out.Dice}
			for i, f := range dice {
				r, err := s.roll(f, level)
				if err != nil {
					return nil, err
				}
				inst.Parts = append(inst.Parts, r)
				inst.Dice = append(inst.Dice, r.Dice...)
				inst.Total += r.Total
				if out.Parts[i].Save != spelleffectsvc.SaveNegates {
					onSave += r.Total
				}
			}
			out.Rolls = append(out.Rolls, inst)
			out.Total += inst.Total
		}
	}
	out.TotalOnSave = &onSave
	return out, nil
}

// trackedCharges are the charge units buffsvc counts down one hit at a time.
var trackedCharges = map[string]bool{"attacks": true, "images": true}

// rollBuff turns a buff entry into a buff on the character at the caster
// level, plus the effects the engine does not track. class is the
// character's: its group picks a by-group ability gain (ADR 0030 step 2).
func (s *service) rollBuff(e spelleffectsvc.Entry, level int, class string) (*AppliedBuff, []Untracked, error) {
	// ADR 0004 step 17: the buff carries the spell's catalogue id so the
	// client names it in the player's language.
	out := &AppliedBuff{Buff: buffsvc.BuffEffect{Name: e.Name, SpellID: e.ID}}
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
		// the engine has no notion of the condition. A condition limited to
		// some modifiers (condition_keys) leaves the others applied.
		if e.Buff.Conditional(key) {
			untracked = append(untracked, Untracked{Key: key, Value: v, Condition: e.Buff.Condition})
		} else if !applyModifier(fx, key, v) {
			untracked = append(untracked, Untracked{Key: key, Value: v})
		}
	}
	for _, ability := range sortedKeys(e.Buff.AbilityGain) {
		g := e.Buff.AbilityGain[ability]
		group, err := gainGroup(g, class)
		if err != nil {
			return nil, nil, err
		}
		f := g.Gain
		if gf, ok := g.ByGroup[group]; ok {
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
		if m, ok := g.MaxByGroup[group]; ok {
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

// gainGroup is the class group of class (ADR 0030 step 2) when g picks its
// die or its cap by group; empty when g is the same for everybody, so a gain
// that does not depend on the group never needs the class.
func gainGroup(g spelleffectsvc.AbilityGain, class string) (string, error) {
	if len(g.ByGroup) == 0 && len(g.MaxByGroup) == 0 {
		return "", nil
	}
	return ruletablesvc.ClassGroup(class)
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
	case "ac_set":
		// "AC becomes N as if armoured" (the Armor spell, Spirit Armor): the
		// base_ac effect of ADR 0033 step 11.
		fx.BaseAC = ptr(v)
	case "save":
		fx.SaveBonus = ptr(v)
	case "initiative":
		// Haste -2 (ADR 0033 step 11): added to the initiative (ADR 0035
		// item 6).
		fx.Initiative = ptr(v)
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

// appliesToSelf decides whether a buff the Bard casts lands on him: always
// for self-only spells; else the player's choice, defaulting to the spell's
// (spelleffectsvc.Entry.DefaultOnSelf: self or other and friendly areas on
// him, hostile areas and others' spells not).
func appliesToSelf(e spelleffectsvc.Entry, onSelf *bool, level int) (bool, error) {
	if e.Target == spelleffectsvc.TargetSelf {
		return true, nil
	}
	if onSelf != nil {
		return *onSelf, nil
	}
	return e.DefaultOnSelf(level)
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
