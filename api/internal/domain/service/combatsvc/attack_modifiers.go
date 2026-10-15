package combatsvc

import (
	"fmt"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// attackMods are the to-hit modifiers of an attack (ADR 0002 steps 4 to 9)
// and the flat damage terms that come with them, computed without rolling.
// ResolveAttack and PreviewAttack share them, so the effective THAC0 shown
// before an attack is the one the attack uses.
type attackMods struct {
	modifiers   []Modifier
	strDmg      int // Strength damage (ADR 0003 step 7)
	weaponDmg   int // weapon magic or buff "+N" to damage (step 6)
	ammoDmg     int // magic ammunition to damage (step 7)
	weaponTerms []DamageTerm
	strTerms    []DamageTerm
}

// toHit is the sum of every to-hit modifier.
func (a attackMods) toHit() int {
	sum := 0
	for _, m := range a.modifiers {
		sum += m.Value
	}
	return sum
}

// attackModifiers lists the to-hit modifiers in the order of ADR 0002 steps
// 4 to 9 (Strength, Dexterity, weapon or buff "+N", ammunition, buffs,
// situational). A nil or zero situational adds nothing.
func (s *service) attackModifiers(char *charactersvc.Character, weapon Weapon, ammo *itemsvc.Item, mods buffsvc.BuffModifiers, sit *Situational) (attackMods, error) {
	out := attackMods{modifiers: []Modifier{}}
	add := func(m Modifier) { out.modifiers = append(out.modifiers, m) }

	// ADR 0002 step 4 and ADR 0003 step 7: Strength (PHB Table 1, from the
	// effective STR with buffs). PHB "Ability Modifiers in Missile Combat":
	// always for melee and hurled weapons; for bows only with a strength
	// bow, which the Bard's are not.
	if !weapon.FiresAmmo() {
		str := mods.EffectiveStr
		hit, dmg, err := s.rules.StrengthMods(str, 0)
		if err != nil {
			return attackMods{}, fmt.Errorf("strength table: %w", err)
		}
		detail := strconv.Itoa(str)
		add(Modifier{Kind: ModStrength, Detail: detail, Value: hit})
		out.strDmg = dmg
		out.strTerms = append(out.strTerms, DamageTerm{Term: TermStrength, Kind: ModStrength, Detail: detail, Value: float64(dmg)})
	}
	// ADR 0002 step 5: Dexterity missile attack adjustment (PHB Table 2) for
	// every missile attack with a hand-held weapon, bow or hurled.
	if weapon.Kind == WeaponMissile {
		missile, _, err := s.rules.DexMods(char.Dex)
		if err != nil {
			return attackMods{}, fmt.Errorf("dexterity table: %w", err)
		}
		add(Modifier{Kind: ModDexterity, Detail: strconv.Itoa(char.Dex), Value: missile})
	}

	// ADR 0002 step 6: weapon magic, the weapon's own +N, or a buff that
	// makes it +N (Power Gem: "the weapon becomes +5") when that is better.
	// They do not add up.
	if m, ok := weaponMagic(weapon.Name, weapon.MagicToHit, mods); ok {
		add(m)
	}
	if m, ok := weaponMagic(weapon.Name, weapon.MagicDamage, mods); ok {
		out.weaponDmg = m.Value
		out.weaponTerms = append(out.weaponTerms, DamageTerm{Term: TermWeapon, Kind: m.Kind, Detail: m.Detail, Value: float64(m.Value)})
	}
	// ADR 0002 step 7: magic ammunition adds to hit and damage, on top of
	// the weapon's (DMG 2e, "Bow +1").
	if ammo != nil && ammo.MagicBonus != 0 {
		add(Modifier{Kind: ModAmmoMagic, Detail: ammo.Name, Value: ammo.MagicBonus})
		out.ammoDmg = ammo.MagicBonus
		out.weaponTerms = append(out.weaponTerms, DamageTerm{Term: TermWeapon, Kind: ModAmmoMagic, Detail: ammo.Name, Value: float64(ammo.MagicBonus)})
	}

	// ADR 0002 step 8: one modifier per buff; step 9: the situational one.
	for _, c := range mods.ToHitFrom {
		add(Modifier{Kind: ModBuff, Detail: c.Buff, Value: c.Value})
	}
	if sit != nil && sit.Value != 0 {
		add(Modifier{Kind: ModSituational, Detail: sit.Label, Value: sit.Value})
	}
	return out, nil
}
