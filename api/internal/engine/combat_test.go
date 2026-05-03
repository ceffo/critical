package engine

import (
	"testing"

	"github.com/moncef-naji/critical/api/internal/domain"
	"github.com/stretchr/testify/assert"
)

type mockDiceRoller struct {
	nextRolls []int
	rollIdx   int
}

func (m *mockDiceRoller) RollDie(sides int) int {
	if m.rollIdx >= len(m.nextRolls) {
		return 1
	}
	res := m.nextRolls[m.rollIdx]
	m.rollIdx++
	return res
}

func (m *mockDiceRoller) RollD20() int {
	return m.RollDie(20)
}

func (m *mockDiceRoller) RollD100() int {
	return m.RollDie(100)
}

func intPtr(v int) *int { return &v }
func floatPtr(v float64) *float64 { return &v }

func TestResolveAttack(t *testing.T) {
	weapon := domain.Weapon{Name: "Longsword", BaseDamage: "1d8"}
	char := domain.Character{Str: 18, Thac0Base: 16}
	targetAC := 5
	// Effective THAC0 = 16 - 1 (STR) = 15
	// To hit 5: 15 - 5 = 10. Roll 10+ hits.

	t.Run("Miss", func(t *testing.T) {
		// D20=5, D100=50
		dr := &mockDiceRoller{nextRolls: []int{5, 50}}
		res := ResolveAttack(dr, weapon, targetAC, &domain.Actor{Stats: char})
		assert.False(t, res.Hit)
		assert.Equal(t, 5, res.D20Roll)
	})

	t.Run("Hit", func(t *testing.T) {
		// D20=12, D100=50, Damage=6
		dr := &mockDiceRoller{nextRolls: []int{12, 50, 6}}
		res := ResolveAttack(dr, weapon, targetAC, &domain.Actor{Stats: char})
		assert.True(t, res.Hit)
		assert.Equal(t, 12, res.D20Roll)
		assert.False(t, res.IsCrit)
		assert.Equal(t, 8, res.TotalDamage) // 6 + 2 (STR)
	})

	t.Run("Crit", func(t *testing.T) {
		// requiredToHit = 10
		// D20=15, D100=2 -> (15 - 2) = 13 >= 10. Crit!
		// Second D100 (crit lookup) = 50
		// Damage roll = 4
		dr := &mockDiceRoller{nextRolls: []int{15, 2, 50, 4}}
		res := ResolveAttack(dr, weapon, targetAC, &domain.Actor{Stats: char})
		assert.True(t, res.Hit)
		assert.True(t, res.IsCrit)
		assert.Equal(t, 2, res.CritMultiplier)
		// Damage = ((4 * 1.0) + 0) * 2 + 2 = 10
		assert.Equal(t, 10, res.TotalDamage)
	})

	t.Run("Damage Formula", func(t *testing.T) {
		// D20=10 (hit), D100=90, Damage=5
		dr := &mockDiceRoller{nextRolls: []int{10, 90, 5}}
		res := ResolveAttack(dr, weapon, targetAC, &domain.Actor{Stats: char})
		assert.Equal(t, 7, res.TotalDamage) // 5 + 2
		assert.Contains(t, res.DamageBreakdown, "((5 × 1.0) + 0) × 1.0 + 2 = 7")
	})

	t.Run("Multipliers Stack", func(t *testing.T) {
		buffs := []domain.BuffEffect{
			{
				Name: "Power Gem",
				Effects: domain.BuffEffects{
					Multiplier: floatPtr(2.0),
				},
			},
		}
		// D20=15, D100=2 -> Crit! (x2)
		// Second D100 = 50
		// Damage = 4
		dr := &mockDiceRoller{nextRolls: []int{15, 2, 50, 4}}
		res := ResolveAttack(dr, weapon, targetAC, &domain.Actor{Stats: char, ActiveBuffs: buffs})
		assert.True(t, res.IsCrit)
		assert.Equal(t, 4.0, res.Multipliers) // 2 (crit) * 2 (buff)
		// Damage = ((4 * 1.0) + 0) * 4 + 2 = 18
		assert.Equal(t, 18, res.TotalDamage)
	})

	t.Run("Weapon and Spells Buffs", func(t *testing.T) {
		buffs := []domain.BuffEffect{
			{
				Name: "Enlarge",
				Effects: domain.BuffEffects{
					SpellsBuff: floatPtr(1.5),
				},
			},
			{
				Name: "Power Gem (Weapon Buff)",
				Effects: domain.BuffEffects{
					WeaponBuff: intPtr(5),
				},
			},
		}
		// D20=12 (hit), D100=50, Damage=6
		dr := &mockDiceRoller{nextRolls: []int{12, 50, 6}}
		res := ResolveAttack(dr, weapon, targetAC, &domain.Actor{Stats: char, ActiveBuffs: buffs})
		// Damage = ((6 * 1.5) + 5) * 1 + 2 = (9 + 5) * 1 + 2 = 16
		assert.Equal(t, 16, res.TotalDamage)
		assert.Contains(t, res.DamageBreakdown, "((6 × 1.5) + 5) × 1.0 + 2 = 16")
	})

	t.Run("Buff Requirement (Min STR)", func(t *testing.T) {
		buffs := []domain.BuffEffect{
			{
				Name: "Heavy Buff",
				Effects: domain.BuffEffects{
					WeaponBuff: intPtr(10),
					MinStr:     intPtr(19),
				},
			},
		}
		// Char has STR 18, so buff should NOT apply
		dr := &mockDiceRoller{nextRolls: []int{12, 50, 6}}
		res := ResolveAttack(dr, weapon, targetAC, &domain.Actor{Stats: char, ActiveBuffs: buffs})
		// Damage = ((6 * 1.0) + 0) * 1 + 2 = 8
		assert.Equal(t, 8, res.TotalDamage)
	})
}
