package combatsvc

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/strength"
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
	itemsvc.TypeLongBow, itemsvc.TypeShortBow,
	itemsvc.TypeLightCrossbow, itemsvc.TypeHeavyCrossbow, itemsvc.TypeHandCrossbow,
}

// thrownTypes are the weapon types (ADR 0030 step 23) of the weapons made to
// be hurled: a race's to-hit bonus listing one counts it only on a hurled
// attack (raceToHit).
var thrownTypes = []string{itemsvc.TypeDagger, itemsvc.TypeDart, itemsvc.TypeOtherThrown}

// raceToHit is ADR 0030 step 22.3: the race's weapon_to_hit bonus
// (View.RaceToHitBonus, ADR 0040 step 4) when the weapon's type is one the
// trait lists, never read from the weapon's name; 0 otherwise or for a weapon
// without a type. A thrown type counts only hurled, as a missile piece (ADR
// 0016 step 8): PHB chapter 2 gives the bonus "when using thrown weapons", and
// a dagger in melee is not thrown.
func raceToHit(char charactersvc.View, weapon Weapon) int {
	if weapon.Type == "" || !slices.Contains(char.RaceToHitTypes, weapon.Type) {
		return 0
	}
	if slices.Contains(thrownTypes, weapon.Type) && weapon.Kind != WeaponMissile {
		return 0
	}
	return char.RaceToHitBonus
}

// strengthMods are the Strength adjustments an attack takes and the score
// whose Table 1 row gave them (the modifier's detail).
type strengthMods struct {
	strength.Mods
	score, exceptional int
}

// strengthOf is ADR 0002 step 4 and ADR 0003 step 7 (PHB p.90-91 "Strength
// Modifiers", p.99 "Ability Modifiers in Missile Combat"): the Table 1
// adjustments of the effective Strength score/exceptional, and whether the
// weapon takes Strength at all.
//   - A melee or hurled weapon: always, bonus or penalty.
//   - A bow (itemsvc.IsBowType): each adjustment that is a penalty for the
//     effective Strength, always; a bonus only with a strength bow, the
//     adjustment of the lower of the effective Strength and the bow's rating
//     (PHB p.73 "Bows"). An ordinary bow with no penalty takes none.
//   - Any other launcher (a crossbow, a sling, one with no type): none.
func (s *service) strengthOf(score, exceptional int, weapon Weapon) (strengthMods, bool, error) {
	bow := itemsvc.IsBowType(weapon.Type)
	if weapon.FiresAmmo() && !bow {
		return strengthMods{}, false, nil
	}
	own, err := s.strength.Of(score, exceptional)
	if err != nil {
		return strengthMods{}, false, fmt.Errorf("strength table: %w", err)
	}
	if !bow {
		return strengthMods{Mods: own, score: score, exceptional: exceptional}, true, nil
	}
	out := strengthMods{score: score, exceptional: exceptional}
	if r := weapon.StrengthRating; r.Rated() && own.Hit >= 0 && own.Dmg >= 0 && !r.Exceeds(score, exceptional) {
		out.score, out.exceptional = r.Score, r.Exceptional
	}
	used, err := s.strength.Of(out.score, out.exceptional)
	if err != nil {
		return strengthMods{}, false, fmt.Errorf("strength table: %w", err)
	}
	out.Hit = bowAdjustment(own.Hit, used.Hit, weapon.StrengthRating.Rated())
	out.Dmg = bowAdjustment(own.Dmg, used.Dmg, weapon.StrengthRating.Rated())
	return out, out.Hit != 0 || out.Dmg != 0 || weapon.StrengthRating.Rated(), nil
}

// bowAdjustment is one Strength adjustment with a bow: own, the effective
// Strength's, when it is a penalty; else rated's positive part (the lower of
// the Strength and the bow's rating) for a strength bow; else 0.
func bowAdjustment(own, rated int, isRated bool) int {
	switch {
	case own < 0:
		return own
	case isRated:
		return max(rated, 0)
	}
	return 0
}

// PointBlankAllowed reports whether an attack with weapon may take the
// point-blank option (ADR 0030 step 12): a specialized bow or crossbow, by
// its weapon type.
func (w Weapon) PointBlankAllowed() bool {
	return w.Specialized && slices.Contains(bowAndCrossbowTypes, w.Type)
}

// BackstabAllowed reports whether char may backstab with weapon (ADR 0030
// step 19): a thief's class has the backstab (View.Backstab above 0),
// and the blow is struck with a melee weapon (PHB p.40).
func BackstabAllowed(char charactersvc.View, weapon Weapon) bool {
	return char.Backstab > 0 && weapon.Kind == WeaponMelee
}

// attackModifiers lists the to-hit modifiers in the order of ADR 0002 steps
// 4 to 9 (Strength, Dexterity, weapon or buff "+N", ammunition, buffs,
// situational), the race's bonus after Dexterity (ADR 0030 step 22.3), weapon specialization after the weapon's magic (ADR 0030
// step 12), the backstab after it (step 19) and the non-proficiency penalty
// between the buffs and the situational modifier (ADR 0037 step 8). A nil or zero situational
// adds nothing. opt.pointBlank is the point-blank option, ErrInvalidInput for
// a weapon that does not allow it; opt.backstab the backstab, ErrInvalidInput
// for a character whose class has none (View.Backstab 0) or a weapon
// other than a melee one (BackstabAllowed).
func (s *service) attackModifiers(char charactersvc.View, weapon Weapon, ammo *itemsvc.Item, mods buffsvc.BuffModifiers, sit *Situational, opt attackOptions) (attackMods, error) {
	if opt.pointBlank && !weapon.PointBlankAllowed() {
		return attackMods{}, fmt.Errorf("%w: point blank needs a specialized bow or crossbow", domain.ErrInvalidInput)
	}
	switch {
	case opt.backstab && char.Backstab == 0:
		return attackMods{}, fmt.Errorf("%w: backstab is a thief's", domain.ErrInvalidInput)
	case opt.backstab && !BackstabAllowed(char, weapon):
		return attackMods{}, fmt.Errorf("%w: backstab needs a melee weapon", domain.ErrInvalidInput)
	}
	out := attackMods{modifiers: []Modifier{}}
	add := func(m Modifier) { out.modifiers = append(out.modifiers, m) }

	// ADR 0002 step 4 and ADR 0003 step 7: Strength (PHB Table 1, from the
	// effective STR with buffs), to hit and to damage (strengthOf).
	str, apply, err := s.strengthOf(mods.EffectiveStr, mods.EffectiveStrExceptional, weapon)
	if err != nil {
		return attackMods{}, err
	}
	if apply {
		detail := strconv.Itoa(str.score)
		add(Modifier{Kind: ModStrength, Detail: detail, StrExceptional: str.exceptional, Value: str.Hit})
		out.strDmg = str.Dmg
		out.strTerms = append(out.strTerms, DamageTerm{Term: TermStrength, Kind: ModStrength, Detail: detail, StrExceptional: str.exceptional, Value: float64(str.Dmg)})
	}
	// ADR 0002 step 5: Dexterity missile adjustment (Table 2, from the view) for every
	// missile attack, bow or hurled.
	if weapon.Kind == WeaponMissile {
		add(Modifier{Kind: ModDexterity, Detail: strconv.Itoa(char.Dex), Value: char.DexMissile})
	}
	// ADR 0030 step 22.3: the race's bonus with the weapon's type, a named
	// modifier (Detail the race code), not magical, to hit only.
	if b := raceToHit(char, weapon); b != 0 {
		add(Modifier{Kind: ModRace, Detail: char.Race, Value: b})
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
	// ADR 0037 step 8 (amends ADR 0002 steps 2 and 12): a weapon type
	// without a pick takes Table 34's penalty for the class group, after
	// the buffs and before the situational modifier. A launcher is judged by
	// its own type; a weapon without a type takes none.
	if p := char.Proficiencies.NonproficientPenalty(weapon.Type); p != 0 {
		add(Modifier{Kind: ModNonproficient, Detail: weapon.Type, Value: p})
	}
	if sit != nil && sit.Value != 0 {
		add(Modifier{Kind: ModSituational, Detail: sit.Label, Value: sit.Value})
	}
	return out, nil
}
