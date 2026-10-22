package combatsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// ResolveAttack resolves one attack (AD&D 2.5):
//
//   - to hit: docs/adr/0002-attack-resolution.md;
//   - house crit: docs/adr/0001-critical-hits.md;
//   - damage: docs/adr/0003-damage.md.
//
// A weapon that fires ammunition spends one through itemsvc (ADR 0002 step
// 10). The ammo use and the action-log entry (full RollResult + items
// snapshot) share one transaction, so a failure leaves the count untouched
// and voiding the entry gives the arrow back.
func (s *service) ResolveAttack(ctx context.Context, characterID int64, req AttackRequest) (*RollResult, error) {
	if req.TargetSize == "" {
		req.TargetSize = SizeSmallMedium
	}
	if !req.TargetSize.Valid() {
		return nil, fmt.Errorf("%w: target size must be %q or %q", domain.ErrInvalidInput, SizeSmallMedium, SizeLarge)
	}
	if sit := req.Situational; sit != nil {
		// An empty label stays empty: the UI names it.
		req.Situational = &Situational{Value: sit.Value, Label: strings.TrimSpace(sit.Label)}
		if sit.Value == 0 {
			req.Situational = nil
		}
	}

	var res *RollResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		weapon, err := s.weapons.Get(ctx, characterID, req.WeaponID)
		if err != nil {
			return fmt.Errorf("get weapon %s: %w", req.WeaponID, err)
		}
		// The weapon in hand is a wielded one (ADR 0033 step 8).
		if !weapon.Wielded {
			return domain.FieldErrorf("weaponId", "%s is not wielded", weapon.Name.Any())
		}
		char, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get character: %w", err)
		}
		mods, err := s.buffs.Aggregate(ctx, characterID, buffsvc.BaseStrengthOf(char), buffsvc.WeaponKind(weapon.Kind))
		if err != nil {
			return fmt.Errorf("aggregate buffs: %w", err)
		}
		crit, err := s.loadCritTable(ctx, characterID)
		if err != nil {
			return err
		}
		ammo, err := s.spendAmmo(ctx, characterID, weapon, req.AmmoItemID)
		if err != nil {
			return err
		}

		r, err := s.resolve(char, weapon, ammo, mods, crit, req)
		if err != nil {
			return err
		}
		if r.Warnings, err = s.attackWarnings(ctx, char, weapon, req.Backstab); err != nil {
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
		payload, err := json.Marshal(toBreakdown(r))
		if err != nil {
			return fmt.Errorf("encode attack payload: %w", err)
		}
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindAttack,
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
func (s *service) spendAmmo(ctx context.Context, characterID int64, weapon Weapon, ammoID string) (*itemsvc.Item, error) {
	if !weapon.FiresAmmo() {
		if ammoID != "" {
			return nil, fmt.Errorf("%w: %s does not fire ammunition", domain.ErrInvalidInput, weapon.Name.Any())
		}
		return nil, nil
	}
	if ammoID == "" {
		return nil, domain.FieldErrorf("ammoItemId", "%s needs ammunition (%s)", weapon.Name.Any(), weapon.AmmoKind)
	}
	item, err := s.items.Use(ctx, characterID, ammoID)
	if err != nil {
		return nil, fmt.Errorf("use ammunition %s: %w", ammoID, err)
	}
	// Ammo is any item of the weapon's ammo kind, whatever its behaviour
	// (ADR 0002 step 10). The weapon fires ammo, so its kind is never empty.
	if !isAmmo(item, weapon) {
		return nil, fmt.Errorf("%w: %s cannot fire %s", domain.ErrInvalidInput, weapon.Name.Any(), item.Name.Any())
	}
	return &item, nil
}

// resolve rolls and computes the attack. Dice are rolled in this order:
// d20, crit-check d100, multiplier d100, then the damage dice, all on every
// attack (ADR 0002 step 1). The crit multiplier is the one of the crit-table
// range holding the multiplier d100.
func (s *service) resolve(char *charactersvc.Character, weapon Weapon, ammo *itemsvc.Item, mods buffsvc.BuffModifiers, crit CritTable, req AttackRequest) (*RollResult, error) {
	res := &RollResult{
		BaseThac0:      char.Derived.THAC0,
		Modifiers:      []Modifier{},
		DamageTerms:    []DamageTerm{},
		CritMultiplier: 1,
		WeaponID:       weapon.ID,
		WeaponName:     weapon.Name.Any(),
		TargetSize:     req.TargetSize,
	}
	// ADR 0012 step 1: an attack may omit the target AC. The dice are the
	// same either way (step 6).
	if req.TargetAC != nil {
		res.TargetAC, res.ACKnown = *req.TargetAC, true
	} else {
		res.RolledACUnknown = true
	}
	if ammo != nil {
		res.Ammo = &AmmoUse{ItemID: ammo.ID, Name: ammo.Name.Any(), Magic: ammo.MagicBonus, Remaining: ammo.Count}
	}
	am, err := s.attackModifiers(char, weapon, ammo, mods, req.Situational, attackOptions{pointBlank: req.PointBlank, backstab: req.Backstab})
	if err != nil {
		return nil, err
	}
	res.Modifiers = am.modifiers
	strDmg, weaponDmg, ammoDmg := am.strDmg, am.weaponDmg, am.ammoDmg
	weaponTerms, strTerms := am.weaponTerms, am.strTerms

	res.EffectiveThac0 = res.BaseThac0 - am.toHit()
	// Checked before any die is rolled: an attack with no damage dice is refused.
	dice := damageDice(weapon, ammo, req.TargetSize)
	if dice == "" {
		if weapon.FiresAmmo() {
			return nil, domain.FieldErrorf("ammoItemId", "neither %s nor its ammunition has damage dice", weapon.Name.Any())
		}
		return nil, domain.FieldErrorf("weaponId", "%s has no damage dice", weapon.Name.Any())
	}

	// ADR 0001 steps 1 and 3: the d20 and the crit-check d100 together,
	// then a second, independent d100 for the multiplier, on every attack.
	res.D20Roll = s.dice.RollD20()
	res.D100CritRoll = s.dice.RollD100()
	res.D100MultiplierRoll = s.dice.RollD100()
	row, err := crit.Lookup(res.D100MultiplierRoll)
	if err != nil {
		return nil, err
	}
	res.RollCritRow = &row
	res.RollCritPlaceholder = crit.Placeholder

	// ADR 0003 step 1: the dice for the target size, rolled on every attack,
	// even a miss, so a target AC corrected after the roll (AdjustAttackAC)
	// never rolls again.
	base, dieRolls, flat, err := rollDice(s.dice, dice)
	if err != nil {
		return nil, fmt.Errorf("weapon %s damage: %w", weapon.ID, err)
	}
	rolled := &RolledDamage{
		Dice: dice, Total: base, Rolls: dieRolls, DiceModifier: flat, SpellsBuff: mods.SpellsBuff, WeaponBuff: weaponDmg + ammoDmg,
		BuffMultipliers: mods.Multipliers, StrengthBonus: strDmg, BonusDamage: mods.Damage,
		Terms: []DamageTerm{},
	}
	for _, f := range mods.MultipliersFrom {
		rolled.Terms = append(rolled.Terms, DamageTerm{Term: TermDiceMult, Kind: ModBuff, Detail: f.Buff, ItemID: f.ItemID, Value: f.Value})
	}
	// ADR 0003 step 2: the flat part of the damage as printed (1d4+1) is
	// part of the weapon's standard damage: added to the dice after the
	// Power Gem's raw-dice factor, before Enlarge, the backstab and the crit,
	// which multiply it.
	if flat != 0 {
		rolled.Terms = append(rolled.Terms, DamageTerm{Term: TermWeapon, Kind: ModDiceModifier, Detail: dice, Value: float64(flat)})
	}
	for _, f := range mods.SpellsBuffFrom {
		rolled.Terms = append(rolled.Terms, DamageTerm{Term: TermSpells, Kind: ModBuff, Detail: f.Buff, ItemID: f.ItemID, Value: f.Value})
	}
	// ADR 0030 step 19: the backstab multiplies the weapon's standard damage
	// (PHB p.40, Table 30 at the realized level), its dice and K, before
	// Strength and magic.
	if req.Backstab {
		rolled.BackstabMultiplier = float64(char.Derived.Backstab)
		rolled.Terms = append(rolled.Terms, DamageTerm{Term: TermStandardMult, Kind: ModBackstab, Value: rolled.BackstabMultiplier})
	}
	rolled.Terms = append(rolled.Terms, weaponTerms...)
	rolled.Terms = append(rolled.Terms, strTerms...)
	for _, c := range mods.DamageFrom {
		rolled.Terms = append(rolled.Terms, DamageTerm{Term: TermBonus, Kind: ModBuff, Detail: c.Buff, ItemID: c.ItemID, Value: float64(c.Value)})
	}
	res.Rolled = rolled

	settle(res)
	return res, nil
}

// settle computes the outcome of a rolled attack from its d20, crit-check
// and multiplier d100s, effective THAC0, target AC (or the GM's verdict),
// crit row and rolled damage, all already set:
//
//   - HitsAC = effective THAC0 - d20, the lowest AC the d20 hits, and
//     CritsFromAC = HitsAC + crit-check d100, the lowest AC it crits, on
//     every attack (ADR 0012 steps 2 and 3);
//   - with a known AC: hit when d20 >= effective THAC0 - target AC, that is
//     target AC >= HitsAC; a natural 1 or 20 is plain arithmetic (ADR 0002
//     steps 2 and 3); crit when d20 - crit-check d100 >= that same number,
//     that is target AC >= CritsFromAC; the multiplier is the one of the
//     crit row of the multiplier d100 (ADR 0001 steps 2 and 4);
//   - with an unknown AC: the GM's verdict decides hit or miss, never a crit
//     (ADR 0012 step 4); without a verdict the attack is not settled: no
//     hit, no crit, and the damage is the one a hit would do at x1;
//   - on a hit (or unsettled), damage = ((dice × DiceMultiplier +
//     DiceModifier) × SpellsBuff × backstab + WeaponBuff) × crit +
//     StrengthBonus + BonusDamage,
//     full precision, floored once at the end, never below 0 (ADR 0003
//     steps 2 to 10). A miss has no damage.
//
// ResolveAttack, AdjustAttackAC and SetAttackVerdict share it so a corrected
// attack is exactly the attack rolled against the new AC.
func settle(res *RollResult) {
	res.HitsAC = res.EffectiveThac0 - res.D20Roll
	res.CritsFromAC = res.HitsAC + res.D100CritRoll
	res.TargetACNeeded = res.HitsAC

	res.IsCrit = false
	res.CritMultiplier = 1
	res.CritFrom, res.CritTo = 0, 0
	res.CritPlaceholder = false
	withDamage := false
	switch {
	case res.ACKnown:
		res.Verdict = VerdictNone
		needed := res.EffectiveThac0 - res.TargetAC
		res.Hit = res.D20Roll >= needed
		res.HitMargin = res.D20Roll - needed
		// ADR 0001 step 2: crit when d20 - crit-check d100 >= the roll needed;
		// step 4: the multiplier is the row of the multiplier d100.
		if row := res.RollCritRow; row != nil && res.D20Roll-res.D100CritRoll >= needed {
			res.IsCrit = true
			res.CritMultiplier = row.Multiplier
			res.CritFrom, res.CritTo = row.From, row.To
			res.CritPlaceholder = res.RollCritPlaceholder
		}
		withDamage = res.Hit
	case res.Verdict != VerdictNone:
		// ADR 0012 step 4: the GM's word, no margin, no crit (x1).
		res.Hit = res.Verdict == VerdictHit
		res.HitMargin = 0
		withDamage = res.Hit
	default:
		// Not settled: the damage shown is the one of a hit at x1.
		res.Hit = false
		res.HitMargin = 0
		withDamage = true
	}

	res.BaseDamage, res.DamageDice, res.DiceModifier = 0, "", 0
	res.SpellsBuff, res.WeaponBuff, res.Multipliers, res.DiceMultiplier, res.BackstabMultiplier = 0, 0, 0, 0, 0
	res.StrengthBonus, res.BonusDamage = 0, 0
	res.DamageTerms = []DamageTerm{}
	res.DamageRolls = nil
	res.UnroundedDamage, res.TotalDamage = 0, 0
	d := res.Rolled
	if !withDamage || d == nil {
		return
	}

	res.BaseDamage = d.Total
	res.DamageDice = d.Dice
	res.DamageRolls = d.Rolls
	// ADR 0003 step 11: terms in formula order (dice multipliers, the flat
	// part, spells, backstab, weapon magic, crit, strength, bonus).
	critAt := len(d.Terms)
	for i, t := range d.Terms {
		if t.Term != TermSpells && t.Term != TermDiceMult && t.Term != TermStandardMult && t.Term != TermWeapon {
			critAt = i
			break
		}
	}
	res.DamageTerms = append(res.DamageTerms, d.Terms[:critAt]...)
	if res.IsCrit {
		res.DamageTerms = append(res.DamageTerms, DamageTerm{Term: TermMultiplier, Kind: ModCrit, Value: res.CritMultiplier})
	}
	res.DamageTerms = append(res.DamageTerms, d.Terms[critAt:]...)

	res.DiceModifier = d.DiceModifier
	res.SpellsBuff = d.SpellsBuff
	res.WeaponBuff = d.WeaponBuff
	res.Multipliers = res.CritMultiplier
	res.DiceMultiplier = d.BuffMultipliers
	res.BackstabMultiplier = d.BackstabMultiplier
	res.StrengthBonus = d.StrengthBonus
	res.BonusDamage = d.BonusDamage

	// ADR 0003 steps 2, 9 and 10: full precision through the whole formula,
	// floored once at the end, never below 0.
	// The backstab is absent (0) without one, and on attacks logged before
	// it had its own factor (it was then inside DiceMultiplier).
	backstab := 1.0
	if res.BackstabMultiplier > 0 {
		backstab = res.BackstabMultiplier
	}
	standard := float64(res.BaseDamage)*res.DiceMultiplier + float64(res.DiceModifier)
	res.UnroundedDamage = (standard*res.SpellsBuff*backstab+float64(res.WeaponBuff))*res.Multipliers +
		float64(res.StrengthBonus) + float64(res.BonusDamage)
	res.TotalDamage = max(int(math.Floor(res.UnroundedDamage+floorEpsilon)), 0)
}

// weaponMagic is the better of the weapon's own bonus and the best "weapon
// becomes +N" of the buffs and item powers (ADR 0002 step 6), named by its
// buff or, for a power, by its item (ADR 0016 step 14); ok is false when
// neither gives anything.
func weaponMagic(weaponName string, own int, mods buffsvc.BuffModifiers) (Modifier, bool) {
	if mods.WeaponBuff > 0 && mods.WeaponBuff >= own {
		return Modifier{
			Kind:   ModWeaponBuff,
			Detail: mods.WeaponBuffFrom,
			ItemID: mods.WeaponBuffItemID,
			Value:  mods.WeaponBuff,
		}, true
	}
	if own == 0 {
		return Modifier{}, false
	}
	return Modifier{Kind: ModWeaponMagic, Detail: weaponName, Value: own}, true
}

// floorEpsilon absorbs float noise (3 × 1.1 = 3.3000000000000003, 1.1 × 10 =
// 11.000000000000002 but 0.1 sums can land just under) before the final floor.
const floorEpsilon = 1e-9

// damageDice is ADR 0003 step 1 as amended by ADR 0033 step 4: a launcher
// that fires ammunition with dice rolls the ammunition's dice; any other
// weapon (and a launcher whose ammunition has none) its own. Of the pair
// chosen, the large-target dice against a Large target when it has them,
// else the small-medium dice. Empty when the pair chosen has none (a
// catalogue launcher firing dice-less ammunition).
func damageDice(weapon Weapon, ammo *itemsvc.Item, size TargetSize) string {
	sm, l := damagePair(weapon, ammo)
	if size == SizeLarge && l != "" {
		return l
	}
	return sm
}

// damagePair is the pair of dice (small-medium, large) an attack with weapon
// and ammo rolls from (ADR 0033 step 4): the ammunition's when the weapon
// fires it and it has dice, else the weapon's own.
func damagePair(weapon Weapon, ammo *itemsvc.Item) (sm, l string) {
	if weapon.FiresAmmo() && ammo != nil && ammo.DamageSM != "" {
		return ammo.DamageSM, ammo.DamageL
	}
	return weapon.BaseDamage, weapon.DamageLarge
}

// rollDice rolls a damage as printed ("NdM", "NdM+K", "NdM-K" or a flat
// "K", itemsvc.ParseDamage) through the dice port and returns the dice
// total, every die thrown in order, and the flat part K (ADR 0003 step 1).
func rollDice(d Dice, dice string) (int, []DieRoll, int, error) {
	num, sides, k, ok := itemsvc.ParseDamage(dice)
	if !ok {
		return 0, nil, 0, fmt.Errorf("invalid damage dice %q", dice)
	}
	total := 0
	rolls := make([]DieRoll, 0, num)
	for i := 0; i < num; i++ {
		v := d.RollDie(sides)
		total += v
		rolls = append(rolls, DieRoll{Sides: sides, Value: v})
	}
	return total, rolls, k, nil
}

// ListWeapons returns the character's weapon pieces in list order, wielded
// or not (ADR 0033 step 8: the form offers the wielded ones first).
func (s *service) ListWeapons(ctx context.Context, characterID int64) ([]Weapon, error) {
	return s.weapons.List(ctx, characterID)
}

// attackWarnings are an attack's class warnings, never a refusal: ADR 0042
// step 13, dual_class_old_ability when a dual-class uses its old class
// before the crossover: a backstab only a dormant entry grants, or a weapon
// only an old class allows (itemsvc's warning on the piece). A character
// with no dormant entry reads nothing.
func (s *service) attackWarnings(ctx context.Context, char *charactersvc.Character, weapon Weapon, backstab bool) ([]string, error) {
	if !char.Derived.HasDormant() {
		return nil, nil
	}
	if g, ok := char.Derived.Grant(classes.KindBackstab); backstab && ok && g.Dormant {
		return []string{classes.WarnOldAbility}, nil
	}
	items, err := s.items.List(ctx, char.ID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	ws, err := s.items.Warnings(ctx, char.ID, items)
	if err != nil {
		return nil, fmt.Errorf("class warnings: %w", err)
	}
	if slices.Contains(ws, itemsvc.Warning{ItemID: weapon.ID, Code: classes.WarnOldAbility}) {
		return []string{classes.WarnOldAbility}, nil
	}
	return nil, nil
}
