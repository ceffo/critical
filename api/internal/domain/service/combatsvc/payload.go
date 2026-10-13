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

func toRecord(r *RollResult) attackRecord {
	mods := make([]modifierRecord, len(r.Modifiers))
	for i, m := range r.Modifiers {
		mods[i] = modifierRecord{Kind: string(m.Kind), Source: m.Source, Detail: m.Detail, Value: m.Value}
	}
	terms := make([]termRecord, len(r.DamageTerms))
	for i, t := range r.DamageTerms {
		terms[i] = termRecord{Term: string(t.Term), Kind: string(t.Kind), Source: t.Source, Detail: t.Detail, Value: t.Value}
	}
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
	return out
}
