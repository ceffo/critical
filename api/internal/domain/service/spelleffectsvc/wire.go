package spelleffectsvc

import "github.com/ceffo/critical/api/pkg/formula"

// The wire shape of a damage at a caster level, defined once: the spell
// lookup (GET /api/spells), the spellbook and the cast and exposure entries
// of the action log (and their endpoints' answers) all serve it, so the UI
// reads one shape. Keys are snake_case, as the lookup has always served
// them. Entries already in the log must stay decodable: add fields, never
// rename them.

// DamageAtWire is the damage a cast of a spell rolls at CasterLevel: Dice
// for one instance, Instances separate instances every round, for Rounds
// rounds; Type is the damage type key ("fire"); the client words it.
type DamageAtWire struct {
	CasterLevel int         `json:"caster_level"`
	Dice        DiceSumWire `json:"dice"`
	Instances   DiceSumWire `json:"instances"`
	Rounds      DiceSumWire `json:"rounds"`
	Type        string      `json:"type,omitempty"`
	// Parts are the parts of one instance, rolled separately, each with its
	// save effect key (none, negates); Dice adds them up. Absent when the
	// damage has no parts.
	Parts []DamagePartAtWire `json:"parts,omitempty"`
	// StepsPer ("round", "instance") and Steps are the dice of every round or
	// instance of a damage whose dice change, in order; Dice is then empty
	// and Rounds or Instances counts the steps. Absent without steps.
	StepsPer string             `json:"steps_per,omitempty"`
	Steps    []DamageStepAtWire `json:"steps,omitempty"`
}

// DiceSumWire is a formula evaluated at a caster level down to what is left
// to roll: dice groups, in the order the spell's data writes them, plus a
// constant. No groups: a plain number.
type DiceSumWire struct {
	Groups   []DiceGroupWire `json:"groups"`
	Modifier int             `json:"modifier"`
}

// DiceGroupWire is Count dice of Sides sides.
type DiceGroupWire struct {
	Count int `json:"count"`
	Sides int `json:"sides"`
}

// DamagePartAtWire is one damage part at the caster level.
type DamagePartAtWire struct {
	Dice DiceSumWire `json:"dice"`
	Save string      `json:"save"`
}

// DamageStepAtWire is one dice step at the caster level: its 1-based round
// or instance number and its dice (no groups and 0: no damage that step).
type DamageStepAtWire struct {
	Step int         `json:"step"`
	Dice DiceSumWire `json:"dice"`
}

// ModeAtWire is one cast-time mode: its key (the client words it) and the
// damage a cast in it rolls (absent: none).
type ModeAtWire struct {
	Key    string        `json:"key"`
	Damage *DamageAtWire `json:"damage,omitempty"`
}

// ExposureAtWire is a spell's exposure damage at a caster level: one damage,
// or the modes picked at each exposure (the first is the default), each with
// its damage.
type ExposureAtWire struct {
	Damage *DamageAtWire `json:"damage,omitempty"`
	Modes  []ModeAtWire  `json:"modes,omitempty"`
}

// Wire is d in its wire shape (nil: none).
func (d *DamageAt) Wire() *DamageAtWire {
	if d == nil {
		return nil
	}
	out := &DamageAtWire{
		CasterLevel: d.CasterLevel, Dice: sumWire(d.Dice), Instances: sumWire(d.Instances),
		Rounds: sumWire(d.Rounds), Type: d.Type, StepsPer: string(d.StepsPer),
	}
	for _, p := range d.Parts {
		out.Parts = append(out.Parts, DamagePartAtWire{Dice: sumWire(p.Dice), Save: string(p.Save)})
	}
	for _, st := range d.Steps {
		out.Steps = append(out.Steps, DamageStepAtWire{Step: st.Step, Dice: sumWire(st.Dice)})
	}
	return out
}

// ModesWire is modes in their wire shape (nil: none).
func ModesWire(modes []ModeAt) []ModeAtWire {
	var out []ModeAtWire
	for _, m := range modes {
		out = append(out, ModeAtWire{Key: m.Key, Damage: m.Damage.Wire()})
	}
	return out
}

// Wire is x in its wire shape (nil: none).
func (x *ExposureAt) Wire() *ExposureAtWire {
	if x == nil {
		return nil
	}
	return &ExposureAtWire{Damage: x.Damage.Wire(), Modes: ModesWire(x.Modes)}
}

func sumWire(s formula.Sum) DiceSumWire {
	out := DiceSumWire{Groups: make([]DiceGroupWire, 0, len(s.Dice)), Modifier: s.Constant}
	for _, g := range s.Dice {
		out.Groups = append(out.Groups, DiceGroupWire{Count: g.Count, Sides: g.Sides})
	}
	return out
}
