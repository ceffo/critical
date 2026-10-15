package castsvc

// castRecord is castsvc's JSON encoding of a cast inside its action-log entry
// (payload), rendered by the UI in the player's language. Entries already in
// the log must stay decodable: add fields, never rename them.
type castRecord struct {
	// Action is "cast" or "received".
	Action      string        `json:"action"`
	SpellID     string        `json:"spellId"`
	Spell       string        `json:"spell"`
	CasterLevel int           `json:"casterLevel"`
	Buff        *buffRecord   `json:"buff,omitempty"`
	Damage      *damageRecord `json:"damage,omitempty"`
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

// rollRecord is one evaluated damage formula (one instance).
type rollRecord struct {
	Formula string      `json:"formula"`
	Dice    []dieRecord `json:"dice"`
	Total   int         `json:"total"`
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
// recorded.
type damageRecord struct {
	Total     int          `json:"total"`
	Type      string       `json:"type"`
	Dice      string       `json:"dice,omitempty"`
	Instances int          `json:"instances,omitempty"`
	Rounds    int          `json:"rounds,omitempty"`
	Rolls     []rollRecord `json:"rolls,omitempty"`
}

func toRecord(res CastResult) castRecord {
	r := castRecord{Action: string(res.Source), SpellID: res.SpellID, Spell: res.Name, CasterLevel: res.CasterLevel}
	if b := res.Buff; b != nil {
		r.Buff = &buffRecord{Name: b.Buff.Name, Rounds: b.Buff.Duration, Charges: b.Buff.Charges, Applied: b.Applied}
		if b.StrGain != nil {
			r.Buff.StrGain = b.StrGain.Total
		}
	}
	if d := res.Damage; d != nil {
		r.Damage = &damageRecord{Total: d.Total, Type: d.Type, Dice: d.Dice, Instances: d.Instances, Rounds: d.Rounds}
		for _, roll := range d.Rolls {
			dice := make([]dieRecord, len(roll.Dice))
			for i, die := range roll.Dice {
				dice[i] = dieRecord(die)
			}
			r.Damage.Rolls = append(r.Damage.Rolls, rollRecord{Formula: roll.Formula, Dice: dice, Total: roll.Total})
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
