package castsvc

// castRecord is castsvc's JSON encoding of a cast inside its action-log entry
// (payload), rendered by the UI in the player's language. Entries already in
// the log must stay decodable: add fields, never rename them.
type castRecord struct {
	// Action is "cast" or "received".
	Action      string `json:"action"`
	SpellID     string `json:"spellId"`
	Spell       string `json:"spell"`
	CasterLevel int    `json:"casterLevel"`
	// Mode is the key of the cast-time mode the spell was cast in; absent for
	// a spell without modes and on casts logged before modes existed.
	Mode   string        `json:"mode,omitempty"`
	Buff   *buffRecord   `json:"buff,omitempty"`
	Damage *damageRecord `json:"damage,omitempty"`
	// ExtraSlots are the levels of the extra slots the cast granted.
	ExtraSlots []int `json:"extraSlots,omitempty"`
	// Item is the inventory item the cast spent.
	Item *itemRecord `json:"item,omitempty"`
	// Attack is the spell's to-hit roll. Entries logged before the cast
	// mechanics were general carry it as "orb" (with the gem name), which the
	// UI no longer reads.
	Attack *attackRecord `json:"attack,omitempty"`
	// Save is the target's saving throw; absent on casts logged before it
	// was recorded.
	Save *saveRecord `json:"save,omitempty"`
}

// saveRecord is the target's saving throw (the DM's side).
type saveRecord struct {
	Type   string `json:"type"`
	Effect string `json:"effect"`
}

// dieRecord is one die thrown: its sides and the value it showed.
type dieRecord struct {
	Sides int `json:"sides"`
	Value int `json:"value"`
}

// rollRecord is one evaluated damage formula (one instance). Parts are the
// rolls of the instance's damage parts (absent: no parts, or logged before
// parts existed).
type rollRecord struct {
	Formula string       `json:"formula"`
	Dice    []dieRecord  `json:"dice"`
	Total   int          `json:"total"`
	Parts   []rollRecord `json:"parts,omitempty"`
	// Step is the dice step (round or instance number) of the roll; absent
	// without steps and on casts logged before steps existed.
	Step int `json:"step,omitempty"`
}

// itemRecord is the item a cast spent and how many are left.
type itemRecord struct {
	ItemID    string `json:"itemId"`
	Name      string `json:"name"`
	Remaining int    `json:"remaining"`
}

// attackRecord is a spell's to-hit roll.
type attackRecord struct {
	D20      int  `json:"d20"`
	TargetAC int  `json:"targetAc"`
	Hit      bool `json:"hit"`
}

type buffRecord struct {
	Name    string `json:"name"`
	Rounds  int    `json:"rounds,omitempty"`
	Charges int    `json:"charges,omitempty"`
	StrGain int    `json:"strGain,omitempty"`
	Applied bool   `json:"applied"`
}

// damageRecord: Dice, Instances, Rounds and Rolls (every instance roll, round
// by round, with its dice) are absent on casts logged before they were
// recorded. Parts and TotalOnSave are set only for a damage made of parts.
type damageRecord struct {
	Total       int          `json:"total"`
	Type        string       `json:"type"`
	Dice        string       `json:"dice,omitempty"`
	Instances   int          `json:"instances,omitempty"`
	Rounds      int          `json:"rounds,omitempty"`
	Rolls       []rollRecord `json:"rolls,omitempty"`
	Parts       []partRecord `json:"parts,omitempty"`
	TotalOnSave *int         `json:"totalOnSave,omitempty"`
	// StepsPer is "round" or "instance" for a damage made of dice steps (each
	// roll carries its step); absent otherwise.
	StepsPer string `json:"stepsPer,omitempty"`
}

// partRecord is one damage part: its formula and what a save does to it.
type partRecord struct {
	Formula string `json:"formula"`
	Save    string `json:"save"`
}

func toRecord(res CastResult) castRecord {
	r := castRecord{Action: string(res.Source), SpellID: res.SpellID, Spell: res.Name, CasterLevel: res.CasterLevel, Mode: res.Mode}
	if b := res.Buff; b != nil {
		r.Buff = &buffRecord{Name: b.Buff.Name, Rounds: b.Buff.Duration, Charges: b.Buff.Charges, Applied: b.Applied}
		if b.StrGain != nil {
			r.Buff.StrGain = b.StrGain.Total
		}
	}
	if d := res.Damage; d != nil {
		r.Damage = &damageRecord{
			Total: d.Total, Type: d.Type, Dice: d.Dice, Instances: d.Instances, Rounds: d.Rounds,
			TotalOnSave: d.TotalOnSave, StepsPer: string(d.StepsPer),
		}
		for _, roll := range d.Rolls {
			r.Damage.Rolls = append(r.Damage.Rolls, toRollRecord(roll))
		}
		for _, p := range d.Parts {
			r.Damage.Parts = append(r.Damage.Parts, partRecord{Formula: p.Formula, Save: string(p.Save)})
		}
	}
	if s := res.Save; s != nil {
		r.Save = &saveRecord{Type: string(s.Type), Effect: string(s.Effect)}
	}
	r.ExtraSlots = res.ExtraSlots
	if c := res.Consumed; c != nil {
		r.Item = &itemRecord{ItemID: c.ItemID, Name: c.Name, Remaining: c.Remaining}
	}
	if a := res.Attack; a != nil {
		r.Attack = &attackRecord{D20: a.D20Roll, TargetAC: a.TargetAC, Hit: a.Hit}
	}
	return r
}

func toRollRecord(roll Roll) rollRecord {
	dice := make([]dieRecord, len(roll.Dice))
	for i, die := range roll.Dice {
		dice[i] = dieRecord(die)
	}
	out := rollRecord{Formula: roll.Formula, Dice: dice, Total: roll.Total, Step: roll.Step}
	for _, p := range roll.Parts {
		out.Parts = append(out.Parts, toRollRecord(p))
	}
	return out
}
