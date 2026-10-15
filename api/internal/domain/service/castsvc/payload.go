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

type damageRecord struct {
	Total int    `json:"total"`
	Type  string `json:"type"`
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
		r.Damage = &damageRecord{Total: d.Total, Type: d.Type}
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
