package spelleffectsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
	"github.com/ceffo/critical/api/pkg/must"
)

func heal(dice string, on ...spelleffectsvc.EndCause) spelleffectsvc.EndEffect {
	return spelleffectsvc.EndEffect{Kind: spelleffectsvc.EndHeal, Dice: must.Do(formula.Parse(dice)), On: on}
}

// ADR 0004 step 21: an entry's end effects name a known kind, at least one
// known cause each once, and dice that are a sum without the level.
func TestBuffs_Step21_ValidateEndEffects(t *testing.T) {
	v, x, d := spelleffectsvc.EndVoluntary, spelleffectsvc.EndExpired, spelleffectsvc.EndDispelled
	tests := map[string]struct {
		effects []spelleffectsvc.EndEffect
		want    string // empty = valid
	}{
		"heal on voluntary and expired": {[]spelleffectsvc.EndEffect{heal("1d12", v, x)}, ""},
		"every cause, constant added":   {[]spelleffectsvc.EndEffect{heal("2d4+1", v, x, d)}, ""},
		"empty list":                    {[]spelleffectsvc.EndEffect{}, "empty list"},
		"unknown kind": {[]spelleffectsvc.EndEffect{{
			Kind: "damage", Dice: must.Do(formula.Parse("1d6")), On: []spelleffectsvc.EndCause{v},
		}}, "end_effects[0].kind"},
		"unknown cause":   {[]spelleffectsvc.EndEffect{heal("1d12", v, "slain")}, "end_effects[0].on"},
		"no cause":        {[]spelleffectsvc.EndEffect{heal("1d12")}, "at least one cause"},
		"duplicate cause": {[]spelleffectsvc.EndEffect{heal("1d12", v, v)}, "twice"},
		"no dice": {[]spelleffectsvc.EndEffect{{
			Kind: spelleffectsvc.EndHeal, On: []spelleffectsvc.EndCause{v},
		}}, "dice is required"},
		"dice use the level":   {[]spelleffectsvc.EndEffect{heal("leveld4", v)}, "may not use level"},
		"dice not a sum":       {[]spelleffectsvc.EndEffect{heal("2*1d6", v)}, "not a sum of dice"},
		"dice below 0":         {[]spelleffectsvc.EndEffect{heal("1d4-2", v)}, "may go below 0"},
		"second effect is bad": {[]spelleffectsvc.EndEffect{heal("1d12", v), heal("1d4", "nope")}, "end_effects[1].on"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := utility()
			e.EndEffects = tt.effects
			err := e.Validate()
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.want)
		})
	}
}

// ADR 0004 step 21: the effects a cause triggers, and whether the stop asks.
func TestBuffs_Step21_EndEffectsOn(t *testing.T) {
	e := utility()
	assert.False(t, e.AsksEndCause())
	assert.Empty(t, e.EndEffectsOn(spelleffectsvc.EndVoluntary))
	e.EndEffects = []spelleffectsvc.EndEffect{heal("1d12", spelleffectsvc.EndVoluntary, spelleffectsvc.EndExpired)}
	assert.True(t, e.AsksEndCause())
	assert.Len(t, e.EndEffectsOn(spelleffectsvc.EndVoluntary), 1)
	assert.Len(t, e.EndEffectsOn(spelleffectsvc.EndExpired), 1)
	assert.Empty(t, e.EndEffectsOn(spelleffectsvc.EndDispelled))
}

// ADR 0004 step 21: the player names voluntary or dispelled, or nothing;
// running out is never his to send.
func TestBuffs_Step21_ParseEndCause(t *testing.T) {
	for _, ok := range []string{"", "voluntary", "dispelled"} {
		c, err := spelleffectsvc.ParseEndCause(ok)
		require.NoError(t, err, ok)
		assert.Equal(t, spelleffectsvc.EndCause(ok), c)
	}
	for _, bad := range []string{"expired", "slain", "VOLUNTARY"} {
		_, err := spelleffectsvc.ParseEndCause(bad)
		require.Error(t, err, bad)
	}
}
