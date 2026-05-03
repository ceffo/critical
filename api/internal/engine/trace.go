package engine

import (
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
)

// RollTrace tracks the modifiers and rolls applied during an action
// to produce a clean explanation.
type RollTrace struct {
	result domain.RollResult
	dr     DiceRoller
}

// NewRollTrace creates a new trace starting with the provided base THAC0.
func NewRollTrace(dr DiceRoller, baseThac0 int) *RollTrace {
	return &RollTrace{
		dr: dr,
		result: domain.RollResult{
			BaseThac0: baseThac0,
			Modifiers: []domain.Modifier{},
		},
	}
}

// Roll delegates to the underlying DiceRoller
func (t *RollTrace) Roll(sides int) int {
	return t.dr.RollDie(sides)
}

// RollD20 rolls a 20-sided die.
func (t *RollTrace) RollD20() int {
	return t.dr.RollD20()
}

// RollD100 rolls a 100-sided die.
func (t *RollTrace) RollD100() int {
	return t.dr.RollD100()
}

// AddModifier records a new modifier from a specific source.
func (t *RollTrace) AddModifier(source string, value int) {
	t.result.Modifiers = append(t.result.Modifiers, domain.Modifier{Source: source, Value: value})
}

// SumModifiers calculates the total value of all recorded modifiers.
func (t *RollTrace) SumModifiers() int {
	sum := 0
	for _, m := range t.result.Modifiers {
		sum += m.Value
	}
	return sum
}

// Result returns a pointer to the accumulated RollResult.
func (t *RollTrace) Result() *domain.RollResult {
	return &t.result
}

// ExplainDamage formats the damage breakdown string
func (t *RollTrace) ExplainDamage() string {
	return fmt.Sprintf("((%d × %.1f) + %d) × %.1f + %d = %d",
		t.result.BaseDamage, t.result.SpellsBuff, t.result.WeaponBuff,
		t.result.Multipliers, t.result.StrengthBonus, t.result.TotalDamage)
}
