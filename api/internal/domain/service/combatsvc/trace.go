package combatsvc

// rollTrace collects the modifiers of a saving throw into its RollResult.
// Internal to the service.
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

func (t *rollTrace) RollD20() int { return t.dice.RollD20() }

func (t *rollTrace) AddModifier(kind ModifierKind, source string, value int) {
	t.result.Modifiers = append(t.result.Modifiers, Modifier{Kind: kind, Source: source, Value: value})
}

func (t *rollTrace) Result() *RollResult { return &t.result }
