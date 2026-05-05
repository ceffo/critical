package buffsvc

// aggregate is the pure effect-stacking core: given a list of buffs and the
// character's STR, return the combined modifier set. Buffs whose requirements
// (currently MinStr) aren't met are skipped.
func aggregate(buffs []BuffEffect, str int) BuffModifiers {
	mods := BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0}
	for _, b := range buffs {
		if !meetsRequirements(b.Effects, str) {
			continue
		}
		applyEffects(&mods, b.Effects)
	}
	return mods
}

func meetsRequirements(e BuffEffects, str int) bool {
	if e.MinStr != nil && str < *e.MinStr {
		return false
	}
	return true
}

func applyEffects(m *BuffModifiers, e BuffEffects) {
	if e.SpellsBuff != nil {
		m.SpellsBuff *= *e.SpellsBuff
	}
	if e.WeaponBuff != nil {
		m.WeaponBuff += *e.WeaponBuff
	}
	if e.Multiplier != nil {
		m.Multipliers *= *e.Multiplier
	}
	if e.SaveBonus != nil {
		m.SaveBonus += *e.SaveBonus
	}
}

// advanceRoundBuffs decrements duration buffs by 1 and partitions into
// (still-active, just-expired). Charge buffs pass through.
func advanceRoundBuffs(buffs []BuffEffect) (active []BuffEffect, expired []BuffEffect) {
	for _, b := range buffs {
		if b.Type == "duration" {
			b.Duration--
			if b.Duration > 0 {
				active = append(active, b)
			} else {
				expired = append(expired, b)
			}
		} else {
			active = append(active, b)
		}
	}
	return
}

// triggerCharge decrements a charge buff by 1 and removes it when charges hit
// zero. Other buffs pass through unchanged.
func triggerCharge(buffs []BuffEffect, id string) []BuffEffect {
	out := make([]BuffEffect, 0, len(buffs))
	for _, b := range buffs {
		if b.ID == id && b.Type == "charge" {
			b.Charges--
			if b.Charges > 0 {
				out = append(out, b)
			}
			continue
		}
		out = append(out, b)
	}
	return out
}
