package combatsvc

// attackRecord is combatsvc's JSON encoding of an attack inside its
// action-log entry (payload). It uses the same camelCase names as the
// /api/roll/attack response, so the UI reads both with one type. Entries
// already in the log must stay decodable: add fields, never rename them.
type attackRecord struct {
	D20Roll         int              `json:"d20Roll"`
	D100CritRoll    int              `json:"d100CritRoll"`
	IsCrit          bool             `json:"isCrit"`
	CritMultiplier  float64          `json:"critMultiplier"`
	CritPlaceholder bool             `json:"critPlaceholder"`
	CritFrom        int              `json:"critFrom"`
	CritTo          int              `json:"critTo"`
	BaseThac0       int              `json:"baseThac0"`
	Modifiers       []modifierRecord `json:"modifiers"`
	EffectiveThac0  int              `json:"effectiveThac0"`
	TargetACNeeded  int              `json:"targetACNeeded"`
	Hit             bool             `json:"hit"`
	HitMargin       int              `json:"hitMargin"`
	BaseDamage      int              `json:"baseDamage"`
	DamageDice      string           `json:"damageDice"`
	SpellsBuff      float64          `json:"spellsBuff"`
	WeaponBuff      int              `json:"weaponBuff"`
	Multipliers     float64          `json:"multipliers"`
	StrengthBonus   int              `json:"strengthBonus"`
	BonusDamage     int              `json:"bonusDamage"`
	DamageTerms     []termRecord     `json:"damageTerms"`
	TotalDamage     int              `json:"totalDamage"`
	DamageBreakdown string           `json:"damageBreakdown"`
	WeaponID        string           `json:"weaponId"`
	WeaponName      string           `json:"weaponName"`
	TargetAC        int              `json:"targetAc"`
	TargetSize      string           `json:"targetSize"`
	Ammo            *ammoRecord      `json:"ammo,omitempty"`
	// Rolled and CritRow are what a target AC correction recomputes the
	// outcome from (absent on attacks logged before corrections existed).
	Rolled  *rolledRecord  `json:"rolled,omitempty"`
	CritRow *critRowRecord `json:"critRow,omitempty"`
	// RolledTargetAC, RolledHit and RolledCrit are the target AC and
	// outcome as rolled, set while a correction differs from them.
	RolledTargetAC *int  `json:"rolledTargetAc,omitempty"`
	RolledHit      *bool `json:"rolledHit,omitempty"`
	RolledCrit     *bool `json:"rolledCrit,omitempty"`
}

// rolledRecord is RolledDamage.
type rolledRecord struct {
	Dice            string       `json:"dice"`
	Total           int          `json:"total"`
	SpellsBuff      float64      `json:"spellsBuff"`
	WeaponBuff      int          `json:"weaponBuff"`
	BuffMultipliers float64      `json:"buffMultipliers"`
	StrengthBonus   int          `json:"strengthBonus"`
	BonusDamage     int          `json:"bonusDamage"`
	Terms           []termRecord `json:"terms"`
}

// critRowRecord is the crit-table row of the crit d100 when rolling.
type critRowRecord struct {
	From        int     `json:"from"`
	To          int     `json:"to"`
	Multiplier  float64 `json:"multiplier"`
	Placeholder bool    `json:"placeholder"`
}

type modifierRecord struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Detail string `json:"detail,omitempty"`
	Value  int    `json:"value"`
}

type termRecord struct {
	Term   string  `json:"term"`
	Kind   string  `json:"kind"`
	Source string  `json:"source"`
	Detail string  `json:"detail,omitempty"`
	Value  float64 `json:"value"`
}

type ammoRecord struct {
	ItemID    string `json:"itemId"`
	Name      string `json:"name"`
	Magic     int    `json:"magic"`
	Remaining int    `json:"remaining"`
}

func toTermRecords(in []DamageTerm) []termRecord {
	out := make([]termRecord, len(in))
	for i, t := range in {
		out[i] = termRecord{Term: string(t.Term), Kind: string(t.Kind), Source: t.Source, Detail: t.Detail, Value: t.Value}
	}
	return out
}

func fromTermRecords(in []termRecord) []DamageTerm {
	out := make([]DamageTerm, len(in))
	for i, t := range in {
		out[i] = DamageTerm{Term: DamageTermKind(t.Term), Kind: ModifierKind(t.Kind), Source: t.Source, Detail: t.Detail, Value: t.Value}
	}
	return out
}

func toRecord(r *RollResult) attackRecord {
	mods := make([]modifierRecord, len(r.Modifiers))
	for i, m := range r.Modifiers {
		mods[i] = modifierRecord{Kind: string(m.Kind), Source: m.Source, Detail: m.Detail, Value: m.Value}
	}
	terms := toTermRecords(r.DamageTerms)
	out := attackRecord{
		D20Roll: r.D20Roll, D100CritRoll: r.D100CritRoll, IsCrit: r.IsCrit, CritMultiplier: r.CritMultiplier,
		CritPlaceholder: r.CritPlaceholder, CritFrom: r.CritFrom, CritTo: r.CritTo, BaseThac0: r.BaseThac0, Modifiers: mods,
		EffectiveThac0: r.EffectiveThac0, TargetACNeeded: r.TargetACNeeded, Hit: r.Hit, HitMargin: r.HitMargin,
		BaseDamage: r.BaseDamage, DamageDice: r.DamageDice, SpellsBuff: r.SpellsBuff, WeaponBuff: r.WeaponBuff,
		Multipliers: r.Multipliers, StrengthBonus: r.StrengthBonus, BonusDamage: r.BonusDamage,
		DamageTerms: terms, TotalDamage: r.TotalDamage, DamageBreakdown: r.DamageBreakdown,
		WeaponID: r.WeaponID, WeaponName: r.WeaponName, TargetAC: r.TargetAC, TargetSize: string(r.TargetSize),
	}
	if a := r.Ammo; a != nil {
		out.Ammo = &ammoRecord{ItemID: a.ItemID, Name: a.Name, Magic: a.Magic, Remaining: a.Remaining}
	}
	if d := r.Rolled; d != nil {
		out.Rolled = &rolledRecord{
			Dice: d.Dice, Total: d.Total, SpellsBuff: d.SpellsBuff, WeaponBuff: d.WeaponBuff,
			BuffMultipliers: d.BuffMultipliers, StrengthBonus: d.StrengthBonus, BonusDamage: d.BonusDamage,
			Terms: toTermRecords(d.Terms),
		}
	}
	if c := r.RollCritRow; c != nil {
		out.CritRow = &critRowRecord{From: c.From, To: c.To, Multiplier: c.Multiplier, Placeholder: r.RollCritPlaceholder}
	}
	out.RolledTargetAC, out.RolledHit, out.RolledCrit = r.RolledTargetAC, r.RolledHit, r.RolledCrit
	return out
}

// result decodes the attack as rolled: the inputs settle needs (dice,
// modifiers, target, rolled damage, crit row) and the context. The outcome
// fields are copied as stored; settle recomputes them.
func (a attackRecord) result(entryID int64) *RollResult {
	mods := make([]Modifier, len(a.Modifiers))
	for i, m := range a.Modifiers {
		mods[i] = Modifier{Kind: ModifierKind(m.Kind), Source: m.Source, Detail: m.Detail, Value: m.Value}
	}
	r := &RollResult{
		D20Roll: a.D20Roll, D100CritRoll: a.D100CritRoll, IsCrit: a.IsCrit, CritMultiplier: a.CritMultiplier,
		CritPlaceholder: a.CritPlaceholder, CritFrom: a.CritFrom, CritTo: a.CritTo, BaseThac0: a.BaseThac0, Modifiers: mods,
		EffectiveThac0: a.EffectiveThac0, TargetACNeeded: a.TargetACNeeded, Hit: a.Hit, HitMargin: a.HitMargin,
		BaseDamage: a.BaseDamage, DamageDice: a.DamageDice, SpellsBuff: a.SpellsBuff, WeaponBuff: a.WeaponBuff,
		Multipliers: a.Multipliers, StrengthBonus: a.StrengthBonus, BonusDamage: a.BonusDamage,
		DamageTerms: fromTermRecords(a.DamageTerms), TotalDamage: a.TotalDamage, DamageBreakdown: a.DamageBreakdown,
		WeaponID: a.WeaponID, WeaponName: a.WeaponName, TargetAC: a.TargetAC, TargetSize: TargetSize(a.TargetSize),
		RolledTargetAC: a.RolledTargetAC, RolledHit: a.RolledHit, RolledCrit: a.RolledCrit,
		LogEntryID: entryID,
	}
	if m := a.Ammo; m != nil {
		r.Ammo = &AmmoUse{ItemID: m.ItemID, Name: m.Name, Magic: m.Magic, Remaining: m.Remaining}
	}
	if d := a.Rolled; d != nil {
		r.Rolled = &RolledDamage{
			Dice: d.Dice, Total: d.Total, SpellsBuff: d.SpellsBuff, WeaponBuff: d.WeaponBuff,
			BuffMultipliers: d.BuffMultipliers, StrengthBonus: d.StrengthBonus, BonusDamage: d.BonusDamage,
			Terms: fromTermRecords(d.Terms),
		}
	}
	if c := a.CritRow; c != nil {
		r.RollCritRow = &CritRow{From: c.From, To: c.To, Multiplier: c.Multiplier}
		r.RollCritPlaceholder = c.Placeholder
	}
	return r
}
