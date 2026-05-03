package domain

// Modifier represents a single bonus or penalty to a roll from a specific source.
type Modifier struct {
	Source string `json:"source"`
	Value  int    `json:"value"`
}

// RollResult encapsulates the full outcome of a combat roll, including hit/miss status and damage calculations.
type RollResult struct {
	D20Roll        int        `json:"d20Roll"`
	D100CritRoll   int        `json:"d100CritRoll"`
	IsCrit         bool       `json:"isCrit"`
	CritMultiplier int        `json:"critMultiplier"`
	BaseThac0      int        `json:"baseThac0"`
	Modifiers      []Modifier `json:"modifiers"`
	EffectiveThac0 int        `json:"effectiveThac0"`
	TargetACNeeded int        `json:"targetACNeeded"`
	Hit            bool       `json:"hit"`
	HitMargin      int        `json:"hitMargin"`
	// Damage breakdown (populated if hit)
	BaseDamage      int     `json:"baseDamage"`
	SpellsBuff      float64 `json:"spellsBuff"`
	WeaponBuff      int     `json:"weaponBuff"`
	Multipliers     float64 `json:"multipliers"`
	StrengthBonus   int     `json:"strengthBonus"`
	TotalDamage     int     `json:"totalDamage"`
	DamageBreakdown string  `json:"damageBreakdown"` // human-readable formula trace
}
