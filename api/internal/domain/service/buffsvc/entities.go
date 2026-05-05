package buffsvc

// BuffEffects is the modifier payload of one buff. Pointers are used so a
// missing field means "no contribution" rather than zero.
type BuffEffects struct {
	MinStr     *int
	SpellsBuff *float64
	WeaponBuff *int
	Multiplier *float64
	SaveBonus  *int
}

// BuffEffect is a single active buff or debuff on a character.
type BuffEffect struct {
	ID       string
	Name     string
	Type     string // "charge" or "duration"
	Charges  int
	Duration int
	Effects  BuffEffects
}

// BuffModifiers is the aggregated effect set across all currently active buffs
// (after requirement gating like MinStr).
type BuffModifiers struct {
	SpellsBuff  float64
	WeaponBuff  int
	Multipliers float64
	SaveBonus   int
}
