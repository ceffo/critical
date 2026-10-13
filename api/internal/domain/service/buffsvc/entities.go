package buffsvc

// State keys under which buffsvc snapshots its state in the action log. Each
// has a Restorer (see Restorers) that the composition root registers.
const (
	StateKeyBuffs = "buffs"
	StateKeyRound = "round"
)

// BuffEffects is the modifier payload of one buff. Pointers are used so a
// missing field means "no contribution" rather than zero.
//
// STR effects (StrSet, StrBonus, StrMax) feed the effective STR score; the
// other effects are gated by MinStr against that effective score.
type BuffEffects struct {
	// MinStr gates the buff: it contributes only while effective STR >= MinStr
	// (the Power Gem needs STR 18).
	MinStr     *int
	SpellsBuff *float64
	// WeaponBuff makes the wielded weapon count as +N (Power Gem: "the
	// weapon becomes +5"), to hit and damage. It replaces the weapon's own
	// magic when higher; it does not add to it.
	WeaponBuff *int
	Multiplier *float64
	SaveBonus  *int

	// StrSet raises STR to this score (never lowers it).
	StrSet *int
	// StrBonus adds to the base STR score, capped at StrMax when set
	// (Strength: +1d6 for a bard, max 18; Improved Strength: 1d6+4, max 23).
	StrBonus *int
	StrMax   *int
	// AC is added to armour class (descending AC: negative is better).
	AC *int
	// ToHit is added to attack rolls (Bless +1, Prayer +1, Chant +1).
	ToHit *int
	// Damage is added to damage rolls after the multipliers, like the
	// Strength bonus.
	Damage *int
}

// BuffEffect is a single active buff or debuff on a character.
//
// Duration is the number of rounds left (0: not timed); Next Round
// decrements it and the buff expires when it reaches 0. Charges is what is
// left to absorb (Stoneskin layers, Mirror Image images; 0: no charges); each
// absorbed hit decrements it and the buff ends at 0. A buff can have both
// (Mirror Image) or neither (lasts until dismissed).
type BuffEffect struct {
	ID       string
	Name     string
	Charges  int
	Duration int
	Effects  BuffEffects
}

// Timed reports whether Next Round counts this buff down.
func (b BuffEffect) Timed() bool { return b.Duration > 0 }

// Charged reports whether the buff absorbs hits.
func (b BuffEffect) Charged() bool { return b.Charges > 0 }

// BuffStatus is an active buff plus whether its effects currently apply
// (its MinStr requirement is met by the effective STR).
type BuffStatus struct {
	BuffEffect
	Applies bool
}

// BuffEdit sets a buff's remaining rounds and/or charges. Nil leaves the
// value unchanged. Setting a counter the buff uses to 0 ends the buff.
type BuffEdit struct {
	Duration *int
	Charges  *int
}

// Strength is the base STR score from the character sheet and the effective
// score with active buffs.
type Strength struct {
	Base      int
	Effective int
}

// Contribution is one applying buff's share of an additive effect, so a
// roll breakdown can name every buff.
type Contribution struct {
	Buff  string
	Value int
}

// Factor is one applying buff's share of a multiplicative effect.
type Factor struct {
	Buff  string
	Value float64
}

// BuffModifiers is the aggregated effect set across all currently active buffs
// (after requirement gating like MinStr). The *From fields name the buffs
// behind each total, in buff order.
type BuffModifiers struct {
	SpellsBuff  float64
	Multipliers float64
	SaveBonus   int
	AC          int
	ToHit       int
	Damage      int
	// WeaponBuff is the best "weapon becomes +N" effect (0: none) and
	// WeaponBuffFrom the buff that gives it.
	WeaponBuff     int
	WeaponBuffFrom string
	// EffectiveStr is the STR score the gating used (base plus STR buffs).
	EffectiveStr int

	SpellsBuffFrom  []Factor
	MultipliersFrom []Factor
	ToHitFrom       []Contribution
	DamageFrom      []Contribution
}
