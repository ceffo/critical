package combatsvc

// AttackBreakdown is the one wire shape of an attack: the payload of its
// action-log entry and the answer of the attack endpoints (/api/roll/attack
// and its correction, verdict and unknown-AC routes) are this struct, so the
// UI reads both with one type. Entries already in the log must stay
// decodable: add fields, never rename them (ADR 0010, ADR 0018).
type AttackBreakdown struct {
	D20Roll      int `json:"d20Roll"`
	D100CritRoll int `json:"d100CritRoll"`
	// D100MultiplierRoll is absent on attacks logged before it existed.
	D100MultiplierRoll int                 `json:"d100MultiplierRoll,omitempty"`
	IsCrit             bool                `json:"isCrit"`
	CritMultiplier     float64             `json:"critMultiplier"`
	CritPlaceholder    bool                `json:"critPlaceholder"`
	CritFrom           int                 `json:"critFrom"`
	CritTo             int                 `json:"critTo"`
	BaseThac0          int                 `json:"baseThac0"`
	Modifiers          []BreakdownModifier `json:"modifiers"`
	EffectiveThac0     int                 `json:"effectiveThac0"`
	TargetACNeeded     int                 `json:"targetACNeeded"`
	// HitsAC and CritsFromAC (ADR 0012 steps 2 and 3); 0 on attacks logged
	// before them (targetACNeeded is HitsAC there).
	HitsAC      int `json:"hitsAc"`
	CritsFromAC int `json:"critsFromAc"`
	// CritPossible: some real AC (at most 10) is crit, i.e. critsFromAc is
	// not beyond every AC (ADR 0012 step 7). Absent on attacks logged before
	// it was recorded; the UI reads absent as possible.
	CritPossible *bool  `json:"critPossible,omitempty"`
	Hit          bool   `json:"hit"`
	HitMargin    int    `json:"hitMargin"`
	BaseDamage   int    `json:"baseDamage"`
	DamageDice   string `json:"damageDice"`
	// DiceModifier is the flat part of damageDice (ADR 0003 step 2),
	// absent when none.
	DiceModifier   int     `json:"diceModifier,omitempty"`
	SpellsBuff     float64 `json:"spellsBuff"`
	WeaponBuff     int     `json:"weaponBuff"`
	Multipliers    float64 `json:"multipliers"`
	DiceMultiplier float64 `json:"diceMultiplier"`
	// BackstabMultiplier multiplies the standard damage (ADR 0030 step 19),
	// absent without a backstab and on attacks logged before it was apart
	// (their backstab is inside diceMultiplier).
	BackstabMultiplier float64         `json:"backstabMultiplier,omitempty"`
	StrengthBonus      int             `json:"strengthBonus"`
	BonusDamage        int             `json:"bonusDamage"`
	DamageTerms        []BreakdownTerm `json:"damageTerms"`
	// DamageRolls are the damage dice as thrown (absent on attacks logged
	// before they were recorded, and when the attack deals no damage).
	DamageRolls []BreakdownDie `json:"damageRolls,omitempty"`
	TotalDamage int            `json:"totalDamage"`
	// UnroundedDamage is the formula result before the final floor (absent
	// on attacks logged before it was recorded).
	UnroundedDamage float64 `json:"unroundedDamage,omitempty"`
	WeaponID        string  `json:"weaponId"`
	WeaponName      string  `json:"weaponName"`
	// TargetAC is null while the AC is unknown (ADR 0012).
	TargetAC   *int           `json:"targetAc"`
	TargetSize string         `json:"targetSize"`
	Ammo       *BreakdownAmmo `json:"ammo,omitempty"`
	// ACKnown is absent on attacks logged before unknown ACs existed, which
	// all had one. RolledACUnknown: rolled without the AC. Verdict: the GM's
	// hit or miss while the AC is unknown.
	ACKnown         *bool  `json:"acKnown,omitempty"`
	RolledACUnknown bool   `json:"rolledAcUnknown,omitempty"`
	Verdict         string `json:"verdict,omitempty"`
	// Rolled and CritRow are what a target AC correction recomputes the
	// outcome from (absent on attacks logged before corrections existed).
	Rolled  *BreakdownRolled  `json:"rolled,omitempty"`
	CritRow *BreakdownCritRow `json:"critRow,omitempty"`
	// RolledTargetAC, RolledHit and RolledCrit are the target AC and
	// outcome as rolled, set while a correction differs from them.
	RolledTargetAC *int  `json:"rolledTargetAc,omitempty"`
	RolledHit      *bool `json:"rolledHit,omitempty"`
	RolledCrit     *bool `json:"rolledCrit,omitempty"`
	// Warnings are the attack's class warnings, served and logged, never a
	// refusal (ADR 0042 step 13: dual_class_old_ability, a dual-class's old
	// class's backstab or weapon before the crossover). Absent when none.
	Warnings []string `json:"warnings,omitempty"`
	// LogEntryID is the entry recording the attack: set on the endpoints'
	// answer, never in the payload (the entry carries its own id). Last, so
	// an answer is its entry's payload plus this one key.
	LogEntryID int64 `json:"logEntryId,omitempty"`
}

// BreakdownRolled is RolledDamage: the damage as rolled, which a target AC
// correction recomputes the outcome from.
type BreakdownRolled struct {
	Dice            string  `json:"dice"`
	Total           int     `json:"total"`
	DiceModifier    int     `json:"diceModifier,omitempty"`
	SpellsBuff      float64 `json:"spellsBuff"`
	WeaponBuff      int     `json:"weaponBuff"`
	BuffMultipliers float64 `json:"buffMultipliers"`
	// BackstabMultiplier as on AttackBreakdown.
	BackstabMultiplier float64         `json:"backstabMultiplier,omitempty"`
	StrengthBonus      int             `json:"strengthBonus"`
	BonusDamage        int             `json:"bonusDamage"`
	Terms              []BreakdownTerm `json:"terms"`
	// Rolls are the dice thrown for Total, in order.
	Rolls []BreakdownDie `json:"rolls,omitempty"`
}

// BreakdownDie is one die thrown: its sides and the value it showed. It is
// the one wire shape of a die: the attack's and the cast's payloads (castsvc)
// and their answers all use it.
type BreakdownDie struct {
	Sides int `json:"sides"`
	Value int `json:"value"`
}

func toBreakdownDice(in []DieRoll) []BreakdownDie {
	var out []BreakdownDie
	for _, d := range in {
		out = append(out, BreakdownDie(d))
	}
	return out
}

func fromBreakdownDice(in []BreakdownDie) []DieRoll {
	var out []DieRoll
	for _, d := range in {
		out = append(out, DieRoll(d))
	}
	return out
}

// BreakdownCritRow is the crit-table row of the multiplier d100 when rolling.
type BreakdownCritRow struct {
	From        int     `json:"from"`
	To          int     `json:"to"`
	Multiplier  float64 `json:"multiplier"`
	Placeholder bool    `json:"placeholder"`
}

// BreakdownModifier is one to-hit (or save) modifier, the one wire shape of a
// modifier: the attack, the save, the attack form's preview and a spell's
// to-hit roll (castsvc) all use it. The UI labels it from
// Kind and Detail. Entries logged before the UI labelled them also carry an
// English "source", no longer read. ItemID is the item of an item power's
// contribution (ADR 0016 step 14), absent otherwise and from entries logged
// before powers.
type BreakdownModifier struct {
	Kind           string `json:"kind"`
	Detail         string `json:"detail,omitempty"`
	ItemID         string `json:"itemId,omitempty"`
	Value          int    `json:"value"`
	StrExceptional int    `json:"strExceptional,omitempty"` // a Strength modifier's 18/xx percentile (ADR 0030 step 9)
}

// BreakdownTerm is one contribution to the damage formula (Detail, ItemID
// and the English "source" of old entries as on BreakdownModifier).
type BreakdownTerm struct {
	Term           string  `json:"term"`
	Kind           string  `json:"kind"`
	Detail         string  `json:"detail,omitempty"`
	ItemID         string  `json:"itemId,omitempty"`
	Value          float64 `json:"value"`
	StrExceptional int     `json:"strExceptional,omitempty"` // a Strength modifier's 18/xx percentile (ADR 0030 step 9)
}

// BreakdownAmmo is the ammunition an attack spent and what is left.
type BreakdownAmmo struct {
	ItemID    string `json:"itemId"`
	Name      string `json:"name"`
	Magic     int    `json:"magic"`
	Remaining int    `json:"remaining"`
}

func toBreakdownTerms(in []DamageTerm) []BreakdownTerm {
	out := make([]BreakdownTerm, len(in))
	for i, t := range in {
		out[i] = BreakdownTerm{Term: string(t.Term), Kind: string(t.Kind), Detail: t.Detail, ItemID: t.ItemID, Value: t.Value, StrExceptional: t.StrExceptional}
	}
	return out
}

func fromBreakdownTerms(in []BreakdownTerm) []DamageTerm {
	out := make([]DamageTerm, len(in))
	for i, t := range in {
		out[i] = DamageTerm{Term: DamageTermKind(t.Term), Kind: ModifierKind(t.Kind), Detail: t.Detail, ItemID: t.ItemID, Value: t.Value, StrExceptional: t.StrExceptional}
	}
	return out
}

// BreakdownModifiers is mods in their wire shape, never nil (an empty list
// is "[]" on the wire).
func BreakdownModifiers(mods []Modifier) []BreakdownModifier {
	out := make([]BreakdownModifier, len(mods))
	for i, m := range mods {
		out[i] = BreakdownModifier{Kind: string(m.Kind), Detail: m.Detail, ItemID: m.ItemID, Value: m.Value, StrExceptional: m.StrExceptional}
	}
	return out
}

// NewAttackBreakdown is the breakdown of r as the attack endpoints answer it:
// the payload logged for r plus the id of its entry.
func NewAttackBreakdown(r *RollResult) AttackBreakdown {
	out := toBreakdown(r)
	out.LogEntryID = r.LogEntryID
	return out
}

// toBreakdown is the payload logged for r: its breakdown without the entry id.
func toBreakdown(r *RollResult) AttackBreakdown {
	mods := BreakdownModifiers(r.Modifiers)
	terms := toBreakdownTerms(r.DamageTerms)
	out := AttackBreakdown{
		D20Roll: r.D20Roll, D100CritRoll: r.D100CritRoll, D100MultiplierRoll: r.D100MultiplierRoll, IsCrit: r.IsCrit, CritMultiplier: r.CritMultiplier,
		CritPlaceholder: r.CritPlaceholder, CritFrom: r.CritFrom, CritTo: r.CritTo, BaseThac0: r.BaseThac0, Modifiers: mods,
		EffectiveThac0: r.EffectiveThac0, TargetACNeeded: r.TargetACNeeded, HitsAC: r.HitsAC, CritsFromAC: r.CritsFromAC,
		Hit: r.Hit, HitMargin: r.HitMargin,
		BaseDamage: r.BaseDamage, DamageDice: r.DamageDice, DiceModifier: r.DiceModifier, SpellsBuff: r.SpellsBuff, WeaponBuff: r.WeaponBuff,
		Multipliers: r.Multipliers, DiceMultiplier: r.DiceMultiplier, BackstabMultiplier: r.BackstabMultiplier, StrengthBonus: r.StrengthBonus, BonusDamage: r.BonusDamage,
		DamageTerms: terms, DamageRolls: toBreakdownDice(r.DamageRolls), TotalDamage: r.TotalDamage, UnroundedDamage: r.UnroundedDamage,
		WeaponID: r.WeaponID, WeaponName: r.WeaponName, TargetSize: string(r.TargetSize), Warnings: r.Warnings,
		ACKnown: &r.ACKnown, RolledACUnknown: r.RolledACUnknown, Verdict: string(r.Verdict),
	}
	possible := r.CritPossible()
	out.CritPossible = &possible
	if r.ACKnown {
		ac := r.TargetAC
		out.TargetAC = &ac
	}
	if a := r.Ammo; a != nil {
		out.Ammo = &BreakdownAmmo{ItemID: a.ItemID, Name: a.Name, Magic: a.Magic, Remaining: a.Remaining}
	}
	if d := r.Rolled; d != nil {
		out.Rolled = &BreakdownRolled{
			Dice: d.Dice, Total: d.Total, DiceModifier: d.DiceModifier, SpellsBuff: d.SpellsBuff, WeaponBuff: d.WeaponBuff,
			BuffMultipliers: d.BuffMultipliers, BackstabMultiplier: d.BackstabMultiplier, StrengthBonus: d.StrengthBonus, BonusDamage: d.BonusDamage,
			Terms: toBreakdownTerms(d.Terms), Rolls: toBreakdownDice(d.Rolls),
		}
	}
	if c := r.RollCritRow; c != nil {
		out.CritRow = &BreakdownCritRow{From: c.From, To: c.To, Multiplier: c.Multiplier, Placeholder: r.RollCritPlaceholder}
	}
	out.RolledTargetAC, out.RolledHit, out.RolledCrit = r.RolledTargetAC, r.RolledHit, r.RolledCrit
	return out
}

// result decodes the attack as rolled: the inputs settle needs (dice,
// modifiers, target, rolled damage, crit row) and the context. The outcome
// fields are copied as stored; settle recomputes them.
func (a AttackBreakdown) result(entryID int64) *RollResult {
	mods := make([]Modifier, len(a.Modifiers))
	for i, m := range a.Modifiers {
		mods[i] = Modifier{Kind: ModifierKind(m.Kind), Detail: m.Detail, ItemID: m.ItemID, Value: m.Value, StrExceptional: m.StrExceptional}
	}
	r := &RollResult{
		D20Roll: a.D20Roll, D100CritRoll: a.D100CritRoll, D100MultiplierRoll: a.D100MultiplierRoll, IsCrit: a.IsCrit, CritMultiplier: a.CritMultiplier,
		CritPlaceholder: a.CritPlaceholder, CritFrom: a.CritFrom, CritTo: a.CritTo, BaseThac0: a.BaseThac0, Modifiers: mods,
		EffectiveThac0: a.EffectiveThac0, TargetACNeeded: a.TargetACNeeded, HitsAC: a.HitsAC, CritsFromAC: a.CritsFromAC,
		Hit: a.Hit, HitMargin: a.HitMargin,
		BaseDamage: a.BaseDamage, DamageDice: a.DamageDice, DiceModifier: a.DiceModifier, SpellsBuff: a.SpellsBuff, WeaponBuff: a.WeaponBuff,
		Multipliers: a.Multipliers, DiceMultiplier: a.DiceMultiplier, BackstabMultiplier: a.BackstabMultiplier, StrengthBonus: a.StrengthBonus, BonusDamage: a.BonusDamage,
		DamageTerms: fromBreakdownTerms(a.DamageTerms), DamageRolls: fromBreakdownDice(a.DamageRolls), TotalDamage: a.TotalDamage, UnroundedDamage: a.UnroundedDamage,
		WeaponID: a.WeaponID, WeaponName: a.WeaponName, TargetSize: TargetSize(a.TargetSize), Warnings: a.Warnings,
		ACKnown: a.ACKnown == nil || *a.ACKnown, RolledACUnknown: a.RolledACUnknown, Verdict: Verdict(a.Verdict),
		RolledTargetAC: a.RolledTargetAC, RolledHit: a.RolledHit, RolledCrit: a.RolledCrit,
		LogEntryID: entryID,
	}
	if a.TargetAC != nil {
		r.TargetAC = *a.TargetAC
	}
	if m := a.Ammo; m != nil {
		r.Ammo = &AmmoUse{ItemID: m.ItemID, Name: m.Name, Magic: m.Magic, Remaining: m.Remaining}
	}
	if d := a.Rolled; d != nil {
		r.Rolled = &RolledDamage{
			Dice: d.Dice, Total: d.Total, DiceModifier: d.DiceModifier, SpellsBuff: d.SpellsBuff, WeaponBuff: d.WeaponBuff,
			BuffMultipliers: d.BuffMultipliers, BackstabMultiplier: d.BackstabMultiplier, StrengthBonus: d.StrengthBonus, BonusDamage: d.BonusDamage,
			Terms: fromBreakdownTerms(d.Terms), Rolls: fromBreakdownDice(d.Rolls),
		}
	}
	if c := a.CritRow; c != nil {
		r.RollCritRow = &CritRow{From: c.From, To: c.To, Multiplier: c.Multiplier}
		r.RollCritPlaceholder = c.Placeholder
	}
	return r
}
