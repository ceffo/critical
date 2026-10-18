package combatsvc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// ToHitRoll is a to-hit roll with no weapon and no damage: a spell attack
// (Chromatic Orb; ADR 0002, Consequences). There is no crit check, no crit table
// and no natural 1 or 20 rule: a 20 is just a hit if the sum reaches the
// number needed, like any other roll (ADR 0001 does not apply).
type ToHitRoll struct {
	D20Roll        int
	BaseThac0      int
	Modifiers      []Modifier
	EffectiveThac0 int
	TargetAC       int
	TargetACNeeded int
	Hit            bool
	HitMargin      int
}

// RollSpellToHit rolls one d20 against targetAC for a thrown spell: Strength
// hit adjustment (a hurled object), Dexterity missile adjustment and the
// active buffs' to-hit. It reads the sheet and the buffs and writes nothing:
// the caller (castsvc) logs the cast it belongs to.
func (s *service) RollSpellToHit(ctx context.Context, characterID int64, targetAC int) (*ToHitRoll, error) {
	char, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get character: %w", err)
	}
	mods, err := s.buffs.Aggregate(ctx, characterID, buffsvc.BaseStrengthOf(char), buffsvc.WeaponNone)
	if err != nil {
		return nil, fmt.Errorf("aggregate buffs: %w", err)
	}
	strHit, _, err := s.rules.StrengthMods(mods.EffectiveStr, mods.EffectiveStrExceptional)
	if err != nil {
		return nil, fmt.Errorf("strength table: %w", err)
	}
	dexMissile, _, err := s.rules.DexMods(char.Dex)
	if err != nil {
		return nil, fmt.Errorf("dexterity table: %w", err)
	}
	res := &ToHitRoll{BaseThac0: char.Derived.THAC0, TargetAC: targetAC, Modifiers: []Modifier{}}
	sum := 0
	add := func(m Modifier) {
		res.Modifiers = append(res.Modifiers, m)
		sum += m.Value
	}
	add(Modifier{Kind: ModStrength, Detail: strDetail(mods), Value: strHit})
	add(Modifier{Kind: ModDexterity, Detail: strconv.Itoa(char.Dex), Value: dexMissile})
	for _, c := range mods.ToHitFrom {
		add(Modifier{Kind: ModBuff, Detail: c.Buff, ItemID: c.ItemID, Value: c.Value})
	}
	res.EffectiveThac0 = res.BaseThac0 - sum
	res.D20Roll = s.dice.RollD20()
	res.TargetACNeeded = res.EffectiveThac0 - res.TargetAC
	res.Hit = res.D20Roll >= res.TargetACNeeded
	res.HitMargin = res.D20Roll - res.TargetACNeeded
	return res, nil
}
