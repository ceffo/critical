package combatsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// critPlaceholderMultiplier is the house crit multiplier until the crit
// table lands (critical-qki.12).
const critPlaceholderMultiplier = 2

// ResolveAttack resolves one attack (AD&D 2.5, docs/backend.md):
//
//   - hit when d20 + modifiers >= THAC0 - target AC; a natural 1 or 20 is
//     plain arithmetic, nothing special;
//   - house crit: (d20 - d100) >= the roll needed -> crit, x2 placeholder;
//   - damage = ((dice × SpellsBuff) + WeaponBuff) × Multipliers +
//     StrengthBonus + buff damage, floored, never below 0.
//
// A weapon that fires ammunition spends one through itemsvc. The ammo use
// and the action-log entry (full RollResult + items snapshot) share one
// transaction, so a failure leaves the count untouched and voiding the entry
// gives the arrow back.
func (s *service) ResolveAttack(ctx context.Context, characterID int64, req AttackRequest) (*RollResult, error) {
	if req.TargetSize == "" {
		req.TargetSize = SizeSmallMedium
	}
	if !req.TargetSize.Valid() {
		return nil, fmt.Errorf("%w: target size must be %q or %q", domain.ErrInvalidInput, SizeSmallMedium, SizeLarge)
	}
	if sit := req.Situational; sit != nil {
		label := strings.TrimSpace(sit.Label)
		switch {
		case sit.Value == 0:
			req.Situational = nil
		case label == "":
			req.Situational = &Situational{Value: sit.Value, Label: "Situational"}
		default:
			req.Situational = &Situational{Value: sit.Value, Label: label}
		}
	}

	var res *RollResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		weapon, err := s.weapons.Get(ctx, req.WeaponID)
		if err != nil {
			return fmt.Errorf("get weapon %s: %w", req.WeaponID, err)
		}
		char, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get character: %w", err)
		}
		mods, err := s.buffs.Aggregate(ctx, characterID, char.Str)
		if err != nil {
			return fmt.Errorf("aggregate buffs: %w", err)
		}
		ammo, err := s.spendAmmo(ctx, weapon, req.AmmoItemID)
		if err != nil {
			return err
		}

		r, err := s.resolve(char, weapon, ammo, mods, req)
		if err != nil {
			return err
		}

		var snaps []actionlog.Snapshot
		if ammo != nil {
			snap, err := itemsvc.UseSnapshot(*ammo)
			if err != nil {
				return err
			}
			snaps = append(snaps, snap)
		}
		payload, err := json.Marshal(toRecord(r))
		if err != nil {
			return fmt.Errorf("encode attack payload: %w", err)
		}
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindAttack,
			Summary:     summary(r),
			Payload:     payload,
			Snapshots:   snaps,
		})
		if err != nil {
			return fmt.Errorf("record attack: %w", err)
		}
		r.LogEntryID = id
		res = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// spendAmmo uses one piece of the chosen ammunition when the weapon fires
// it. The decrement is undone with the transaction if the ammo turns out not
// to fit the weapon.
func (s *service) spendAmmo(ctx context.Context, weapon Weapon, ammoID string) (*itemsvc.Item, error) {
	if !weapon.FiresAmmo() {
		if ammoID != "" {
			return nil, fmt.Errorf("%w: %s does not fire ammunition", domain.ErrInvalidInput, weapon.Name)
		}
		return nil, nil
	}
	if ammoID == "" {
		return nil, fmt.Errorf("%w: %s needs ammunition (%s)", domain.ErrInvalidInput, weapon.Name, weapon.AmmoKind)
	}
	item, err := s.items.Use(ctx, ammoID)
	if err != nil {
		return nil, fmt.Errorf("use ammunition %s: %w", ammoID, err)
	}
	if item.Kind != itemsvc.KindAmmo || item.AmmoKind != weapon.AmmoKind {
		return nil, fmt.Errorf("%w: %s cannot fire %s", domain.ErrInvalidInput, weapon.Name, item.Name)
	}
	return &item, nil
}

// resolve rolls and computes the attack. Dice are rolled in this order:
// d20, crit d100, then the damage dice on a hit.
func (s *service) resolve(char *charactersvc.Character, weapon Weapon, ammo *itemsvc.Item, mods buffsvc.BuffModifiers, req AttackRequest) (*RollResult, error) {
	res := &RollResult{
		BaseThac0:      char.Thac0Base,
		Modifiers:      []Modifier{},
		DamageTerms:    []DamageTerm{},
		CritMultiplier: 1,
		WeaponID:       weapon.ID,
		WeaponName:     weapon.Name,
		TargetAC:       req.TargetAC,
		TargetSize:     req.TargetSize,
	}
	if ammo != nil {
		res.Ammo = &AmmoUse{ItemID: ammo.ID, Name: ammo.Name, Magic: ammo.MagicBonus, Remaining: ammo.Count}
	}
	add := func(m Modifier) { res.Modifiers = append(res.Modifiers, m) }
	// Damage terms are collected per formula term and listed in formula
	// order on a hit.
	var weaponTerms, strTerms []DamageTerm

	// Strength (PHB Table 1, from the effective STR with buffs). PHB
	// "Ability Modifiers in Missile Combat": always for melee and hurled
	// weapons; for bows only with a strength bow, which the Bard's are not.
	strDmg := 0
	if !weapon.FiresAmmo() {
		str := mods.EffectiveStr
		hit, dmg, err := s.rules.StrengthMods(str, 0)
		if err != nil {
			return nil, fmt.Errorf("strength table: %w", err)
		}
		detail := strconv.Itoa(str)
		add(Modifier{Kind: ModStrength, Source: "Strength " + detail, Detail: detail, Value: hit})
		strDmg = dmg
		strTerms = append(strTerms, DamageTerm{Term: TermStrength, Kind: ModStrength, Source: "Strength " + detail, Detail: detail, Value: float64(dmg)})
	}
	// Dexterity missile attack adjustment (PHB Table 2) for every missile
	// attack with a hand-held weapon, bow or hurled.
	if weapon.Kind == WeaponMissile {
		missile, _, err := s.rules.DexMods(char.Dex)
		if err != nil {
			return nil, fmt.Errorf("dexterity table: %w", err)
		}
		detail := strconv.Itoa(char.Dex)
		add(Modifier{Kind: ModDexterity, Source: "Dexterity " + detail, Detail: detail, Value: missile})
	}

	// Weapon magic: the weapon's own +N, or a buff that makes it +N (Power
	// Gem: "the weapon becomes +5") when that is better. They do not add up.
	weaponDmg := 0
	if m, ok := weaponMagic(weapon.Name, weapon.MagicToHit, mods); ok {
		add(m)
	}
	if m, ok := weaponMagic(weapon.Name, weapon.MagicDamage, mods); ok {
		weaponDmg = m.Value
		weaponTerms = append(weaponTerms, DamageTerm{Term: TermWeapon, Kind: m.Kind, Source: m.Source, Detail: m.Detail, Value: float64(m.Value)})
	}
	// Magic ammunition: DMG 2e, "Bow +1": "If magical arrows are used, total
	// the bonuses of both the bow and the arrows used" (to hit and damage).
	ammoDmg := 0
	if ammo != nil && ammo.MagicBonus != 0 {
		add(Modifier{Kind: ModAmmoMagic, Source: ammo.Name, Detail: ammo.Name, Value: ammo.MagicBonus})
		ammoDmg = ammo.MagicBonus
		weaponTerms = append(weaponTerms, DamageTerm{Term: TermWeapon, Kind: ModAmmoMagic, Source: ammo.Name, Detail: ammo.Name, Value: float64(ammo.MagicBonus)})
	}

	for _, c := range mods.ToHitFrom {
		add(Modifier{Kind: ModBuff, Source: c.Buff, Detail: c.Buff, Value: c.Value})
	}
	if sit := req.Situational; sit != nil {
		add(Modifier{Kind: ModSituational, Source: sit.Label, Detail: sit.Label, Value: sit.Value})
	}

	sum := 0
	for _, m := range res.Modifiers {
		sum += m.Value
	}
	res.EffectiveThac0 = res.BaseThac0 - sum
	needed := res.EffectiveThac0 - req.TargetAC

	res.D20Roll = s.dice.RollD20()
	res.D100CritRoll = s.dice.RollD100()
	res.TargetACNeeded = res.EffectiveThac0 - res.D20Roll
	res.Hit = res.D20Roll >= needed
	res.HitMargin = res.D20Roll - needed
	if res.D20Roll-res.D100CritRoll >= needed {
		res.IsCrit = true
		res.CritMultiplier = critPlaceholderMultiplier
		res.CritPlaceholder = true
	}
	if !res.Hit {
		return res, nil
	}

	dice := weapon.BaseDamage
	if req.TargetSize == SizeLarge && weapon.DamageLarge != "" {
		dice = weapon.DamageLarge
	}
	base, err := rollDice(s.dice, dice)
	if err != nil {
		return nil, fmt.Errorf("weapon %s damage: %w", weapon.ID, err)
	}
	res.BaseDamage = base
	res.DamageDice = dice

	for _, f := range mods.SpellsBuffFrom {
		res.DamageTerms = append(res.DamageTerms, DamageTerm{Term: TermSpells, Kind: ModBuff, Source: f.Buff, Detail: f.Buff, Value: f.Value})
	}
	res.DamageTerms = append(res.DamageTerms, weaponTerms...)
	if res.IsCrit {
		res.DamageTerms = append(res.DamageTerms, DamageTerm{Term: TermMultiplier, Kind: ModCrit, Source: "Critical hit (placeholder)", Value: critPlaceholderMultiplier})
	}
	for _, f := range mods.MultipliersFrom {
		res.DamageTerms = append(res.DamageTerms, DamageTerm{Term: TermMultiplier, Kind: ModBuff, Source: f.Buff, Detail: f.Buff, Value: f.Value})
	}
	res.DamageTerms = append(res.DamageTerms, strTerms...)
	for _, c := range mods.DamageFrom {
		res.DamageTerms = append(res.DamageTerms, DamageTerm{Term: TermBonus, Kind: ModBuff, Source: c.Buff, Detail: c.Buff, Value: float64(c.Value)})
	}

	res.SpellsBuff = mods.SpellsBuff
	res.WeaponBuff = weaponDmg + ammoDmg
	res.Multipliers = float64(res.CritMultiplier) * mods.Multipliers
	res.StrengthBonus = strDmg
	res.BonusDamage = mods.Damage

	total := math.Floor((float64(res.BaseDamage)*res.SpellsBuff+float64(res.WeaponBuff))*res.Multipliers) +
		float64(res.StrengthBonus) + float64(res.BonusDamage)
	res.TotalDamage = max(int(total), 0)
	res.DamageBreakdown = explainDamage(res)
	return res, nil
}

// weaponMagic is the better of the weapon's own bonus and a buff's "weapon
// becomes +N"; ok is false when neither gives anything.
func weaponMagic(weaponName string, own int, mods buffsvc.BuffModifiers) (Modifier, bool) {
	if mods.WeaponBuff > own {
		return Modifier{
			Kind:   ModWeaponBuff,
			Source: fmt.Sprintf("%s (weapon +%d)", mods.WeaponBuffFrom, mods.WeaponBuff),
			Detail: mods.WeaponBuffFrom,
			Value:  mods.WeaponBuff,
		}, true
	}
	if own == 0 {
		return Modifier{}, false
	}
	return Modifier{Kind: ModWeaponMagic, Source: weaponName, Detail: weaponName, Value: own}, true
}

// explainDamage is the formula with the numbers filled in.
func explainDamage(r *RollResult) string {
	s := fmt.Sprintf("((%d × %s) + %d) × %s + %d",
		r.BaseDamage, factor(r.SpellsBuff), r.WeaponBuff, factor(r.Multipliers), r.StrengthBonus)
	switch {
	case r.BonusDamage > 0:
		s += fmt.Sprintf(" + %d", r.BonusDamage)
	case r.BonusDamage < 0:
		s += fmt.Sprintf(" - %d", -r.BonusDamage)
	}
	return fmt.Sprintf("%s = %d", s, r.TotalDamage)
}

func factor(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) }

// rollDice rolls "NdM" (case-insensitive) through the dice port.
func rollDice(d Dice, dice string) (int, error) {
	n, m, ok := strings.Cut(strings.ToLower(strings.TrimSpace(dice)), "d")
	num, err1 := strconv.Atoi(n)
	sides, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || num < 1 || sides < 1 {
		return 0, fmt.Errorf("invalid damage dice %q", dice)
	}
	total := 0
	for i := 0; i < num; i++ {
		total += d.RollDie(sides)
	}
	return total, nil
}

// summary is the English log line (the UI renders the payload instead).
func summary(r *RollResult) string {
	weapon := r.WeaponName
	if r.Ammo != nil {
		weapon = fmt.Sprintf("%s (%s)", weapon, r.Ammo.Name)
	}
	switch {
	case !r.Hit:
		return fmt.Sprintf("%s vs AC %d: miss", weapon, r.TargetAC)
	case r.IsCrit:
		return fmt.Sprintf("%s vs AC %d: crit ×%d, %d damage", weapon, r.TargetAC, r.CritMultiplier, r.TotalDamage)
	default:
		return fmt.Sprintf("%s vs AC %d: hit, %d damage", weapon, r.TargetAC, r.TotalDamage)
	}
}

// ListWeapons returns the weapons catalog in sheet order.
func (s *service) ListWeapons(ctx context.Context) ([]Weapon, error) {
	return s.weapons.List(ctx)
}
