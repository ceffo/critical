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
	Orb         *orbRecord    `json:"orb,omitempty"`
}

// orbRecord is a Chromatic Orb's gem and attack.
type orbRecord struct {
	Gem      string `json:"gem"`
	D20      int    `json:"d20"`
	TargetAC int    `json:"targetAc"`
	Hit      bool   `json:"hit"`
	Effect   string `json:"effect,omitempty"`
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
	if o := res.Orb; o != nil {
		r.Orb = &orbRecord{Gem: o.Gem, D20: o.Attack.D20Roll, TargetAC: o.Attack.TargetAC, Hit: o.Attack.Hit, Effect: o.Effect}
	}
	return r
}
