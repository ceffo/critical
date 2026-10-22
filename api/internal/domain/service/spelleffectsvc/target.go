package spelleffectsvc

import "fmt"

// CastOn is who a buff the caster casts lands on: the caster or somebody else.
type CastOn string

// Who a cast lands on.
const (
	CastOnSelf  CastOn = "self"
	CastOnOther CastOn = "other"
)

// TargetChoice is the choice of target a cast offers the caster: where it
// lands when he names none (Default) and the one other choice (Alternate).
type TargetChoice struct {
	Default   CastOn
	Alternate CastOn
}

// TargetChoiceAt returns the target choice a cast of the spell offers at the
// caster level (ADR 0027 step 10.1), nil when there is none to make: a spell
// that neither adds a buff nor lasts a number of rounds (where it lands
// changes nothing the engine tracks), a self-only spell, a spell with no
// target, and a hostile buff (Blindness, Slow, the Malisons: only a defensive
// spell offers the other target; the cast still takes an explicit onSelf). A
// spell that only lasts (it gets a row with no effects, ADR 0004 step 20)
// offers the choice too, since its row reads "not on you" when cast on
// someone else (step 20.5), except a damage spell, which hurts its target.
func (e Entry) TargetChoiceAt(level int) (*TargetChoice, error) {
	if !e.AddsBuff() {
		return e.rowTargetChoice(), nil
	}
	switch e.Target {
	case TargetOther, TargetSelfOrOther, TargetArea:
	default:
		return nil, nil
	}
	if hostile, err := e.Hostile(level); err != nil || hostile {
		return nil, err
	}
	onSelf, err := e.DefaultOnSelf(level)
	if err != nil {
		return nil, err
	}
	if onSelf {
		return &TargetChoice{Default: CastOnSelf, Alternate: CastOnOther}, nil
	}
	return &TargetChoice{Default: CastOnOther, Alternate: CastOnSelf}, nil
}

// rowTargetChoice is the target choice of a spell that adds no buff but lasts
// a number of rounds: its row lands where RowOnSelf says when the caster names
// no target, and the alternate is the other one.
func (e Entry) rowTargetChoice() *TargetChoice {
	if e.Kind == KindDamage || e.Duration.Kind != DurationRounds || e.Duration.Rounds == nil {
		return nil
	}
	switch e.Target {
	case TargetOther, TargetSelfOrOther, TargetArea:
	default:
		return nil
	}
	if RowOnSelf(&e, nil) {
		return &TargetChoice{Default: CastOnSelf, Alternate: CastOnOther}
	}
	return &TargetChoice{Default: CastOnOther, Alternate: CastOnSelf}
}

// DefaultOnSelf says whether a buff the caster casts lands on him when he
// names no target: always for a self-only spell and for "self or other", for
// an area unless the buff is hostile, never for "other" or no target.
func (e Entry) DefaultOnSelf(level int) (bool, error) {
	switch e.Target {
	case TargetSelf, TargetSelfOrOther:
		return true, nil
	case TargetArea:
		hostile, err := e.Hostile(level)
		return !hostile, err
	default:
		return false, nil
	}
}

// hostileBelow are the modifiers that hurt the bearer when below a threshold
// (a penalty to hit, to damage or to saves; less movement or fewer attacks),
// hostileAbove those that hurt him above one (a worse armour class, attackers
// who hit him more easily, a later initiative).
var (
	hostileBelow = map[string]int{"to_hit": 0, "damage": 0, "save": 0, "move_pct": 100, "attacks_pct": 100}
	hostileAbove = map[string]int{"ac": 0, "attackers_to_hit": 0, "initiative": 0}
)

// Hostile reports a buff whose effects hurt the bearer (Slow, Malison) at the
// caster level: an area spell the caster casts does not land on him by
// default. A modifier's dice are taken at their largest.
func (e Entry) Hostile(level int) (bool, error) {
	if e.Buff == nil {
		return false, nil
	}
	for key, f := range e.Buff.Modifiers {
		below, isBelow := hostileBelow[key]
		above, isAbove := hostileAbove[key]
		if !isBelow && !isAbove {
			continue
		}
		v, err := f.Max(level)
		if err != nil {
			return false, fmt.Errorf("%s modifier %s: %w", e.ID, key, err)
		}
		if (isBelow && v < below) || (isAbove && v > above) {
			return true, nil
		}
	}
	return false, nil
}
