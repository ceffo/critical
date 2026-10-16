package composition_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

const gemID = "power_gem_dispel"

// powersOn is the ids of the item powers that are on, "item/power".
func (s *strStack) powersOn() []string {
	s.t.Helper()
	ps, err := s.bs.Buffs.ListPowers(s.ctx, s.id)
	require.NoError(s.t, err)
	out := []string{}
	for _, p := range ps {
		out = append(out, p.ItemID+"/"+p.PowerID)
	}
	return out
}

// attack rolls one attack with scripted dice: d20, the crit-check d100 (high:
// no crit), the multiplier d100, then one damage die of sides showing die.
func (s *strStack) attack(req combatsvc.AttackRequest, d20, sides, die int) *combatsvc.RollResult {
	s.t.Helper()
	s.dice.expect(roll{"d20", d20}, roll{"d100", 99}, roll{"d100", 41}, roll{fmtDie(sides), die})
	res, err := s.bs.Combat.ResolveAttack(s.ctx, s.id, req)
	require.NoError(s.t, err)
	return res
}

func fmtDie(sides int) string {
	return map[int]string{3: "d3", 6: "d6", 8: "d8"}[sides]
}

var swordAt5 = combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(5)}

// hammerMod is the hammer's +5 to hit, named by the gem.
var hammerMod = combatsvc.Modifier{Kind: combatsvc.ModWeaponBuff, Detail: "Power Gem", ItemID: gemID, Value: 5}

// ADR 0016 step 10 on the real stack: a received Strength that takes STR 12
// to 18 turns the hammer on by itself, popping it turns it off again; the
// switching is not logged (the receive is the only entry).
func TestItemPowers_Step10_StrengthBuffTurnsTheHammerOnAndVoidTurnsItOff(t *testing.T) {
	s := newStrStack(t)
	assert.Empty(t, s.powersOn())
	assert.False(t, s.gemOn())

	recv := s.receive("strength", 9, 6)
	require.Equal(t, 18, s.effective())
	assert.Equal(t, []string{gemID + "/thunder_hammer"}, s.powersOn())
	assert.True(t, s.gemOn())
	entries, err := s.bs.Log.List(s.ctx, s.id, 10)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "only the receive is logged, not the power switching on")

	_, err = s.bs.Log.Undo(s.ctx, s.id, recv.EntryID)
	require.NoError(t, err)
	assert.Equal(t, 12, s.effective())
	assert.Empty(t, s.powersOn())
	assert.False(t, s.gemOn())
}

// ADR 0016 step 15 on the real stack: removing the gem removes its powers
// (none on at STR 18); popping the remove brings the gem back with its
// hammer, which is on again.
func TestItemPowers_Step15_RemoveGemAndPopBringsThePowersBack(t *testing.T) {
	s := newStrStack(t)
	s.receive("strength", 9, 6)
	require.Equal(t, []string{gemID + "/thunder_hammer"}, s.powersOn())

	removed, err := s.bs.Items.Delete(s.ctx, s.id, gemID)
	require.NoError(t, err)
	assert.Empty(t, s.powersOn())
	assert.False(t, s.gemOn())

	_, err = s.bs.Log.Undo(s.ctx, s.id, removed.LogEntryID)
	require.NoError(t, err)
	items, err := s.bs.Items.List(s.ctx)
	require.NoError(t, err)
	var back bool
	for _, it := range items {
		if it.ID == gemID {
			back = true
			require.Len(t, it.Powers, 1)
			assert.Equal(t, "thunder_hammer", it.Powers[0].ID)
		}
	}
	assert.True(t, back, "the gem is back")
	assert.Equal(t, []string{gemID + "/thunder_hammer"}, s.powersOn())
	assert.True(t, s.gemOn())
}

// ADR 0016 step 16 on the real stack with the Long Sword +2 (ADR 0002 step
// 6, ADR 0003 steps 2 and 4): at STR 17 the sword's own +2 and x1, no gem
// anywhere; at 18 from two Strength buffs +5 to hit and to damage named by
// the gem and the raw dice doubled; the dispel spent (at 17: it needs no
// STR) leaves the hammer on; with Enlarge and Improved Strength to 23 the
// damage is floor(((dice x 2 x Enlarge) + 5) x crit + STR + buff damage);
// a Short Bow +2 at 18 gets neither the +5 nor the x2.
func TestItemPowers_Step16_HammerAt18WithTheLongSword(t *testing.T) {
	s := newStrStack(t)

	// STR 17: 12 + 5. PHB Table 1 at 17: +1 to hit, +1 damage.
	s.receive("strength", 9, 5)
	require.Equal(t, 17, s.effective())
	at17 := s.attack(swordAt5, 11, 8, 6)
	assert.Equal(t, []combatsvc.Modifier{
		{Kind: combatsvc.ModStrength, Detail: "17", Value: 1},
		{Kind: combatsvc.ModWeaponMagic, Detail: "Long Sword +2", Value: 2},
	}, at17.Modifiers)
	assert.Equal(t, 2, at17.WeaponBuff)
	assert.InDelta(t, 1.0, at17.DiceMultiplier, 1e-9)
	for _, term := range at17.DamageTerms {
		assert.Empty(t, term.ItemID, "no gem term at 17")
	}
	assert.Equal(t, 6+2+1, at17.TotalDamage)
	assert.Empty(t, s.powersOn())

	// The daily Dispel Magic is spent at 17: no STR needed.
	used, err := s.bs.Items.UseItem(s.ctx, s.id, gemID)
	require.NoError(t, err)
	assert.Zero(t, used.Remaining)

	// STR 18: 12 + 5 + 1. PHB at 18: +1 to hit, +2 damage. 1d8 = 6: 6 x 2 + 5 + 2.
	s.receive("strength", 9, 1)
	require.Equal(t, 18, s.effective())
	at18 := s.attack(swordAt5, 12, 8, 6)
	assert.Equal(t, []combatsvc.Modifier{{Kind: combatsvc.ModStrength, Detail: "18", Value: 1}, hammerMod}, at18.Modifiers)
	assert.Equal(t, 5, at18.WeaponBuff, "the higher of the sword's +2 and the hammer's +5, never both")
	assert.InDelta(t, 2.0, at18.DiceMultiplier, 1e-9)
	assert.Contains(t, at18.DamageTerms, combatsvc.DamageTerm{Term: combatsvc.TermDiceMult, Kind: combatsvc.ModBuff, Detail: "Power Gem", ItemID: gemID, Value: 2})
	assert.Equal(t, 6*2+5+2, at18.TotalDamage)
	assert.Equal(t, 13-1-5, at18.EffectiveThac0)

	// Enlarge x1.5 and Improved Strength to 23 (12 + 11, capped at 23).
	_, err = s.bs.Buffs.Activate(s.ctx, s.id, buffsvc.BuffEffect{Name: "Enlarge", Effects: buffsvc.BuffEffects{SpellsBuff: new(1.5)}})
	require.NoError(t, err)
	_, err = s.bs.Buffs.Activate(s.ctx, s.id, buffsvc.BuffEffect{Name: "Improved Strength", Effects: buffsvc.BuffEffects{StrBonus: new(11), StrMax: new(23)}})
	require.NoError(t, err)
	require.Equal(t, 23, s.effective())
	at23 := s.attack(swordAt5, 13, 8, 5)
	assert.Equal(t, hammerMod, at23.Modifiers[1])
	assert.InDelta(t, 2.0, at23.DiceMultiplier, 1e-9)
	assert.InDelta(t, 1.5, at23.SpellsBuff, 1e-9)
	want := int(math.Floor((5*2*1.5+5)*1 + float64(at23.StrengthBonus) + float64(at23.BonusDamage)))
	assert.Equal(t, want, at23.TotalDamage)
	assert.Equal(t, 11, at23.StrengthBonus, "PHB Table 1 at 23")

	// A Short Bow +2 at 23: the hammer is on but melee only.
	assert.Equal(t, []string{gemID + "/thunder_hammer"}, s.powersOn())
	bow := s.attack(combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus1", TargetAC: new(5)}, 14, 6, 4)
	assert.NotContains(t, bow.Modifiers, hammerMod)
	assert.Equal(t, combatsvc.Modifier{Kind: combatsvc.ModWeaponMagic, Detail: "Short Bow +2", Value: 2}, bow.Modifiers[1])
	assert.InDelta(t, 1.0, bow.DiceMultiplier, 1e-9)
	assert.Equal(t, 2+1, bow.WeaponBuff, "the bow's +2 and the arrow's +1")
}
