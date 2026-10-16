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
		weapon, err := s.weapons.Get(ctx, req.WeaponID)
		if err != nil {
			return fmt.Errorf("get weapon %s: %w", req.WeaponID, err)
		}
		char, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get character: %w", err)
		}
		mods, err := s.buffs.Aggregate(ctx, characterID, char.Str, buffsvc.WeaponKind(weapon.Kind))
		if err != nil {
			return fmt.Errorf("aggregate buffs: %w", err)
		}
		crit, err := s.loadCritTable(ctx)
		if err != nil {
			return err
		}
		ammo, err := s.spendAmmo(ctx, weapon, req.AmmoItemID)
		if err != nil {
			return err
		}

		r, err := s.resolve(char, weapon, ammo, mods, crit, req)
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
	// Ammo is any item of the weapon's ammo kind, whatever its behaviour
	// (ADR 0002 step 10). The weapon fires ammo, so its kind is never empty.
	if item.AmmoKind != weapon.AmmoKind {
		return nil, fmt.Errorf("%w: %s cannot fire %s", domain.ErrInvalidInput, weapon.Name, item.Name)
	}
	return &item, nil
}

// resolve rolls and computes the attack. Dice are rolled in this order:
// d20, crit-check d100, multiplier d100, then the damage dice, all on every
// attack (ADR 0002 step 1). The crit multiplier is the one of the crit-table
// range holding the multiplier d100.
func (s *service) resolve(char *charactersvc.Character, weapon Weapon, ammo *itemsvc.Item, mods buffsvc.BuffModifiers, crit CritTable, req AttackRequest) (*RollResult, error) {
	res := &RollResult{
		BaseThac0:      char.Thac0Base,
		Modifiers:      []Modifier{},
		DamageTerms:    []DamageTerm{},
		CritMultiplier: 1,
		WeaponID:       weapon.ID,
		WeaponName:     weapon.Name,
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
		res.Ammo = &AmmoUse{ItemID: ammo.ID, Name: ammo.Name, Magic: ammo.MagicBonus, Remaining: ammo.Count}
	}
	am, err := s.attackModifiers(char, weapon, ammo, mods, req.Situational)
	if err != nil {
		return nil, err
	}
	res.Modifiers = am.modifiers
	strDmg, weaponDmg, ammoDmg := am.strDmg, am.weaponDmg, am.ammoDmg
	weaponTerms, strTerms := am.weaponTerms, am.strTerms

	res.EffectiveThac0 = res.BaseThac0 - am.toHit()

	// ADR 0001 steps 1 and 3: the d20 and the crit-check d100 together,
	// then a second, independent d100 for the multiplier, on every attack.
	res.D20Roll = s.dice.RollD20()
	res.D100CritRoll = s.dice.RollD100()
	res.D100MultiplierRoll = s.dice.RollD100()
	row, err := lookupCrit(crit.Rows, res.D100MultiplierRoll)
	if err != nil {
		return nil, err
	}
	res.RollCritRow = &row
	res.RollCritPlaceholder = crit.Placeholder

	// ADR 0003 step 1: the dice for the target size, rolled on every attack,
	// even a miss, so a target AC corrected after the roll (AdjustAttackAC)
	// never rolls again.
	dice := weapon.BaseDamage
	if req.TargetSize == SizeLarge && weapon.DamageLarge != "" {
		dice = weapon.DamageLarge
	}
	base, dieRolls, err := rollDice(s.dice, dice)
	if err != nil {
		return nil, fmt.Errorf("weapon %s damage: %w", weapon.ID, err)
	}
	rolled := &RolledDamage{
		Dice: dice, Total: base, Rolls: dieRolls, SpellsBuff: mods.SpellsBuff, WeaponBuff: weaponDmg + ammoDmg,
		BuffMultipliers: mods.Multipliers, StrengthBonus: strDmg, BonusDamage: mods.Damage,
		Terms: []DamageTerm{},
	}
	for _, f := range mods.SpellsBuffFrom {
		rolled.Terms = append(rolled.Terms, DamageTerm{Term: TermSpells, Kind: ModBuff, Detail: f.Buff, ItemID: f.ItemID, Value: f.Value})
	}
	for _, f := range mods.MultipliersFrom {
		rolled.Terms = append(rolled.Terms, DamageTerm{Term: TermDiceMult, Kind: ModBuff, Detail: f.Buff, ItemID: f.ItemID, Value: f.Value})
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
//   - on a hit (or unsettled), damage = ((dice × SpellsBuff ×
//     DiceMultiplier) + WeaponBuff) × crit + StrengthBonus + BonusDamage,
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

	res.BaseDamage, res.DamageDice = 0, ""
	res.SpellsBuff, res.WeaponBuff, res.Multipliers, res.DiceMultiplier = 0, 0, 0, 0
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
	// ADR 0003 step 11: terms in formula order (spells, dice multipliers,
	// weapon, crit, strength, bonus).
	critAt := len(d.Terms)
	for i, t := range d.Terms {
		if t.Term != TermSpells && t.Term != TermDiceMult && t.Term != TermWeapon {
			critAt = i
			break
		}
	}
	res.DamageTerms = append(res.DamageTerms, d.Terms[:critAt]...)
	if res.IsCrit {
		res.DamageTerms = append(res.DamageTerms, DamageTerm{Term: TermMultiplier, Kind: ModCrit, Value: res.CritMultiplier})
	}
	res.DamageTerms = append(res.DamageTerms, d.Terms[critAt:]...)

	res.SpellsBuff = d.SpellsBuff
	res.WeaponBuff = d.WeaponBuff
	res.Multipliers = res.CritMultiplier
	res.DiceMultiplier = d.BuffMultipliers
	res.StrengthBonus = d.StrengthBonus
	res.BonusDamage = d.BonusDamage

	// ADR 0003 steps 2, 9 and 10: full precision through the whole formula,
	// floored once at the end, never below 0.
	res.UnroundedDamage = (float64(res.BaseDamage)*res.SpellsBuff*res.DiceMultiplier+float64(res.WeaponBuff))*res.Multipliers +
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

// rollDice rolls "NdM" (case-insensitive) through the dice port and returns
// the total and every die thrown, in order.
func rollDice(d Dice, dice string) (int, []DieRoll, error) {
	n, m, ok := strings.Cut(strings.ToLower(strings.TrimSpace(dice)), "d")
	num, err1 := strconv.Atoi(n)
	sides, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || num < 1 || sides < 1 {
		return 0, nil, fmt.Errorf("invalid damage dice %q", dice)
	}
	total := 0
	rolls := make([]DieRoll, 0, num)
	for i := 0; i < num; i++ {
		v := d.RollDie(sides)
		total += v
		rolls = append(rolls, DieRoll{Sides: sides, Value: v})
	}
	return total, rolls, nil
}

// ListWeapons returns the weapons catalog in sheet order.
func (s *service) ListWeapons(ctx context.Context) ([]Weapon, error) {
	return s.weapons.List(ctx)
}
