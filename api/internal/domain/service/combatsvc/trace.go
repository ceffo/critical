package combatsvc

import "fmt"

// rollTrace tracks modifiers and rolls applied during one combat resolution
// so we can produce a human-readable explanation. Internal to the service.
type rollTrace struct {
	result RollResult
	dice   Dice
}

func newRollTrace(d Dice, baseThac0 int) *rollTrace {
	return &rollTrace{
		dice:   d,
		result: RollResult{BaseThac0: baseThac0, Modifiers: []Modifier{}},
	}
}

func (t *rollTrace) RollDie(sides int) int { return t.dice.RollDie(sides) }
func (t *rollTrace) RollD20() int          { return t.dice.RollD20() }
func (t *rollTrace) RollD100() int         { return t.dice.RollD100() }

func (t *rollTrace) AddModifier(source string, value int) {
	t.result.Modifiers = append(t.result.Modifiers, Modifier{Source: source, Value: value})
}

func (t *rollTrace) SumModifiers() int {
	sum := 0
	for _, m := range t.result.Modifiers {
		sum += m.Value
	}
	return sum
}

func (t *rollTrace) Result() *RollResult { return &t.result }

func (t *rollTrace) ExplainDamage() string {
	return fmt.Sprintf("((%d × %.1f) + %d) × %.1f + %d = %d",
		t.result.BaseDamage, t.result.SpellsBuff, t.result.WeaponBuff,
		t.result.Multipliers, t.result.StrengthBonus, t.result.TotalDamage)
}
