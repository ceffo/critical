package spelleffectsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
)

// ADR 0008 step 17.3: a heal lands as a friendly buff does: the cast box
// offers the choice when its target allows both, defaulting as a buff does;
// a self-only, untargeted or low-confidence heal offers none.
func TestEntry_TargetChoiceAt_Heal(t *testing.T) {
	dice, err := formula.Parse("1d8")
	require.NoError(t, err)
	heal := func(target spelleffectsvc.Target, conf spelleffectsvc.Confidence) spelleffectsvc.Entry {
		return spelleffectsvc.Entry{
			ID: "cure", Kind: spelleffectsvc.KindHeal, Confidence: conf, Target: target,
			Duration: spelleffectsvc.Duration{Kind: spelleffectsvc.DurationInstant}, Heal: &spelleffectsvc.Heal{Dice: dice},
		}
	}
	onSelf := &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnSelf, Alternate: spelleffectsvc.CastOnOther}
	onOther := &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnOther, Alternate: spelleffectsvc.CastOnSelf}
	tests := map[string]struct {
		e    spelleffectsvc.Entry
		want *spelleffectsvc.TargetChoice
	}{
		"self or other":  {heal(spelleffectsvc.TargetSelfOrOther, spelleffectsvc.ConfidenceHigh), onSelf},
		"other":          {heal(spelleffectsvc.TargetOther, spelleffectsvc.ConfidenceHigh), onOther},
		"area":           {heal(spelleffectsvc.TargetArea, spelleffectsvc.ConfidenceHigh), onSelf},
		"self":           {heal(spelleffectsvc.TargetSelf, spelleffectsvc.ConfidenceHigh), nil},
		"none":           {heal(spelleffectsvc.TargetNone, spelleffectsvc.ConfidenceHigh), nil},
		"low confidence": {heal(spelleffectsvc.TargetSelfOrOther, spelleffectsvc.ConfidenceLow), nil},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.e.TargetChoiceAt(9)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
