package combatsvc

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
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

// Weapon specialization (ADR 0030 step 12, PHB "Effects of Specialization").
const (
	specializationToHit  = 1 // melee weapon
	specializationDamage = 2 // melee weapon, after the multipliers like Strength
	pointBlankToHit      = 2 // bow or crossbow, at point-blank range
)

// backstabToHit is the backstab's +4 to hit (ADR 0030 step 19, PHB p.40).
const backstabToHit = 4

// attackOptions are the per-attack options of the request: point-blank
// range (ADR 0030 step 12) and backstab (step 19).
type attackOptions struct {
	pointBlank, backstab bool
}

// bowAndCrossbowTypes are the weapon types (ADR 0030 step 23) whose
// specialist gains the point-blank range category (PHB p.53).
var bowAndCrossbowTypes = []string{
	weaponsvc.TypeLongBow, weaponsvc.TypeShortBow,
	weaponsvc.TypeLightCrossbow, weaponsvc.TypeHeavyCrossbow, weaponsvc.TypeHandCrossbow,
}

// PointBlankAllowed reports whether an attack with weapon may take the
// point-blank option (ADR 0030 step 12): a specialized bow or crossbow, by
// its weapon type.
func (w Weapon) PointBlankAllowed() bool {
	return w.Specialized && slices.Contains(bowAndCrossbowTypes, w.Type)
}

// BackstabAllowed reports whether char may backstab with weapon (ADR 0030
// step 19): a thief's class has the backstab (Derived.Backstab above 0),
// and the blow is struck with a melee weapon (PHB p.40).
func BackstabAllowed(char *charactersvc.Character, weapon Weapon) bool {
	return char.Derived.Backstab > 0 && weapon.Kind == WeaponMelee
}

// attackModifiers lists the to-hit modifiers in the order of ADR 0002 steps
// 4 to 9 (Strength, Dexterity, weapon or buff "+N", ammunition, buffs,
// situational), weapon specialization after the weapon's magic (ADR 0030
// step 12) and the backstab after it (step 19). A nil or zero situational
// adds nothing. opt.pointBlank is the point-blank option, ErrInvalidInput for
// a weapon that does not allow it; opt.backstab the backstab, ErrInvalidInput
// for a character whose class has none (Derived.Backstab 0) or a weapon
// other than a melee one (BackstabAllowed).
func (s *service) attackModifiers(char *charactersvc.Character, weapon Weapon, ammo *itemsvc.Item, mods buffsvc.BuffModifiers, sit *Situational, opt attackOptions) (attackMods, error) {
	if opt.pointBlank && !weapon.PointBlankAllowed() {
		return attackMods{}, fmt.Errorf("%w: point blank needs a specialized bow or crossbow", domain.ErrInvalidInput)
	}
	switch {
	case opt.backstab && char.Derived.Backstab == 0:
		return attackMods{}, fmt.Errorf("%w: backstab is a thief's", domain.ErrInvalidInput)
	case opt.backstab && !BackstabAllowed(char, weapon):
		return attackMods{}, fmt.Errorf("%w: backstab needs a melee weapon", domain.ErrInvalidInput)
	}
	out := attackMods{modifiers: []Modifier{}}
	add := func(m Modifier) { out.modifiers = append(out.modifiers, m) }

	// ADR 0002 step 4 and ADR 0003 step 7: Strength (PHB Table 1, from the
	// effective STR with buffs). PHB "Ability Modifiers in Missile Combat":
	// always for melee and hurled weapons; for bows only with a strength
	// bow, which the Bard's are not.
	if !weapon.FiresAmmo() {
		hit, dmg, err := s.rules.StrengthMods(mods.EffectiveStr, mods.EffectiveStrExceptional)
		if err != nil {
			return attackMods{}, fmt.Errorf("strength table: %w", err)
		}
		detail := strconv.Itoa(mods.EffectiveStr)
		exc := mods.EffectiveStrExceptional
		add(Modifier{Kind: ModStrength, Detail: detail, StrExceptional: exc, Value: hit})
		out.strDmg = dmg
		out.strTerms = append(out.strTerms, DamageTerm{Term: TermStrength, Kind: ModStrength, Detail: detail, StrExceptional: exc, Value: float64(dmg)})
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

	// ADR 0002 step 6: weapon magic, the weapon's own +N, or a buff or an
	// item power that makes it +N (the Power Gem's thunder hammer: "the
	// weapon becomes +5") when that is better. They do not add up. A +N with
	// a weapon-kind condition is already left out of mods for another kind
	// (ADR 0004 step 18, ADR 0016 step 12).
	if m, ok := weaponMagic(weapon.Name.Any(), weapon.MagicToHit, mods); ok {
		add(m)
	}
	if m, ok := weaponMagic(weapon.Name.Any(), weapon.MagicDamage, mods); ok {
		out.weaponDmg = m.Value
		out.weaponTerms = append(out.weaponTerms, DamageTerm{Term: TermWeapon, Kind: m.Kind, Detail: m.Detail, ItemID: m.ItemID, Value: float64(m.Value)})
	}
	// ADR 0002 step 7: magic ammunition adds to hit and damage, on top of
	// the weapon's (DMG 2e, "Bow +1").
	if ammo != nil && ammo.MagicBonus != 0 {
		add(Modifier{Kind: ModAmmoMagic, Detail: ammo.Name.Any(), ItemID: ammo.ID, Value: ammo.MagicBonus})
		out.ammoDmg = ammo.MagicBonus
		out.weaponTerms = append(out.weaponTerms, DamageTerm{Term: TermWeapon, Kind: ModAmmoMagic, Detail: ammo.Name.Any(), ItemID: ammo.ID, Value: float64(ammo.MagicBonus)})
	}

	// ADR 0030 step 12 (PHB p.53): specialization, not magical (never a "+N"
	// of step 6). A melee weapon gets +1 to hit and +2 damage (added after
	// the multipliers, with Strength); a bow or crossbow gets no damage, and
	// +2 to hit only at point-blank range; a sling or a thrown weapon gets
	// neither (only its attacks, step 11).
	switch {
	case weapon.Specialized && weapon.Kind == WeaponMelee:
		add(Modifier{Kind: ModSpecialization, Value: specializationToHit})
		out.strDmg += specializationDamage
		out.strTerms = append(out.strTerms, DamageTerm{Term: TermStrength, Kind: ModSpecialization, Value: specializationDamage})
	case opt.pointBlank:
		add(Modifier{Kind: ModPointBlank, Value: pointBlankToHit})
	}
	// ADR 0030 step 19: the backstab's +4 to hit, a named modifier; its
	// multiplier is on the damage dice (resolve).
	if opt.backstab {
		add(Modifier{Kind: ModBackstab, Value: backstabToHit})
	}

	// ADR 0002 step 8: one modifier per buff; step 9: the situational one.
	for _, c := range mods.ToHitFrom {
		add(Modifier{Kind: ModBuff, Detail: c.Buff, ItemID: c.ItemID, Value: c.Value})
	}
	if sit != nil && sit.Value != 0 {
		add(Modifier{Kind: ModSituational, Detail: sit.Label, Value: sit.Value})
	}
	return out, nil
}
