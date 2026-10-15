package spelleffectsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
	"github.com/ceffo/critical/api/pkg/must"
)

func exposureEntry(x spelleffectsvc.Exposure) spelleffectsvc.Entry {
	e := damageEntry(spelleffectsvc.Damage{})
	e.Damage, e.Exposure = nil, &x
	return e
}

// ADR 0008 step 16: the exposure damage is served at the caster level, as
// one damage or every mode with its damage; none for a spell without one, a
// low-confidence one, or one that is not a damage spell. A cast of a spell
// whose only damage is its exposure rolls none.
func TestEntry_ExposureAt(t *testing.T) {
	p := func(src string) *formula.Formula { return must.Do(formula.Parse(src)) }
	ps := func(src string) *formula.Formula { return must.Do(formula.ParseStep(src)) }

	cloud := exposureEntry(spelleffectsvc.Exposure{Damage: &spelleffectsvc.Damage{Dice: p("1d10"), Type: "poison"}})
	got, err := cloud.ExposureAt(12)
	require.NoError(t, err)
	assert.Equal(t, &spelleffectsvc.ExposureAt{Damage: &spelleffectsvc.DamageAt{
		CasterLevel: 12, Dice: sum(0, dice(1, 10)), Instances: sum(1), Rounds: sum(1), Type: "poison",
	}}, got)
	cast, err := cloud.DamageAt(12)
	require.NoError(t, err)
	assert.Nil(t, cast, "a cast rolls no damage: each exposure does")

	wall := exposureEntry(spelleffectsvc.Exposure{Modes: []spelleffectsvc.Mode{
		{Key: "near", Damage: &spelleffectsvc.Damage{Dice: p("2d4"), Type: "fire"}},
		{Key: "through", Damage: &spelleffectsvc.Damage{Dice: p("2d6+level"), Type: "fire"}},
	}})
	got, err = wall.ExposureAt(9)
	require.NoError(t, err)
	assert.Equal(t, &spelleffectsvc.ExposureAt{Modes: []spelleffectsvc.ModeAt{
		{Key: "near", Damage: &spelleffectsvc.DamageAt{CasterLevel: 9, Dice: sum(0, dice(2, 4)), Instances: sum(1), Rounds: sum(1), Type: "fire"}},
		{Key: "through", Damage: &spelleffectsvc.DamageAt{CasterLevel: 9, Dice: sum(9, dice(2, 6)), Instances: sum(1), Rounds: sum(1), Type: "fire"}},
	}}, got)

	fog := exposureEntry(spelleffectsvc.Exposure{Damage: &spelleffectsvc.Damage{Type: "acid", Steps: &spelleffectsvc.DamageSteps{
		Per: spelleffectsvc.StepPerRound, List: []*formula.Formula{ps("1"), ps("2"), ps("4"), ps("8")},
	}}})
	got, err = fog.ExposureAt(15)
	require.NoError(t, err)
	assert.Equal(t, &spelleffectsvc.ExposureAt{Damage: &spelleffectsvc.DamageAt{
		CasterLevel: 15, Instances: sum(1), Rounds: sum(4), Type: "acid", StepsPer: spelleffectsvc.StepPerRound,
		Steps: []spelleffectsvc.DamageStepAt{{Step: 1, Dice: sum(1)}, {Step: 2, Dice: sum(2)}, {Step: 3, Dice: sum(4)}, {Step: 4, Dice: sum(8)}},
	}}, got)

	low := cloud
	low.Confidence = spelleffectsvc.ConfidenceLow
	plain := damageEntry(spelleffectsvc.Damage{Dice: p("1d6")})
	for name, e := range map[string]spelleffectsvc.Entry{"low confidence": low, "no exposure": plain} {
		got, err := e.ExposureAt(12)
		require.NoError(t, err)
		assert.Nil(t, got, name)
	}
}

// ADR 0008 step 16: an exposure names its mode by key (none: the first; an
// unlisted key, or any key without modes, is ErrUnknownMode); a spell without
// a usable exposure is ErrNoExposure. With steps, a round of exposure rolls
// its step, and a later round the last step; without, every round rolls the
// same dice.
func TestEntry_ExposureRound(t *testing.T) {
	p := func(src string) *formula.Formula { return must.Do(formula.Parse(src)) }
	ps := func(src string) *formula.Formula { return must.Do(formula.ParseStep(src)) }
	near := &spelleffectsvc.Damage{Dice: p("2d4")}
	through := &spelleffectsvc.Damage{Dice: p("2d6+level")}
	wall := exposureEntry(spelleffectsvc.Exposure{Modes: []spelleffectsvc.Mode{{Key: "near", Damage: near}, {Key: "through", Damage: through}}})

	d, key, err := wall.ExposureFor("")
	require.NoError(t, err)
	assert.Equal(t, "near", key, "the first is the default")
	assert.Same(t, near, d)
	d, key, err = wall.ExposureFor("through")
	require.NoError(t, err)
	assert.Equal(t, "through", key)
	assert.Same(t, through, d)
	_, _, err = wall.ExposureFor("far")
	require.ErrorIs(t, err, spelleffectsvc.ErrUnknownMode)

	cloud := exposureEntry(spelleffectsvc.Exposure{Damage: near})
	d, key, err = cloud.ExposureFor("")
	require.NoError(t, err)
	assert.Empty(t, key)
	assert.Same(t, near, d)
	_, _, err = cloud.ExposureFor("near")
	require.ErrorIs(t, err, spelleffectsvc.ErrUnknownMode, "an exposure without modes")

	low := cloud
	low.Confidence = spelleffectsvc.ConfidenceLow
	notDamage := cloud
	notDamage.Kind = spelleffectsvc.KindUtility
	for name, e := range map[string]spelleffectsvc.Entry{
		"no exposure": damageEntry(spelleffectsvc.Damage{Dice: p("1d6")}), "low confidence": low, "not a damage spell": notDamage,
	} {
		_, _, err := e.ExposureFor("")
		require.ErrorIs(t, err, spelleffectsvc.ErrNoExposure, name)
	}

	fog := &spelleffectsvc.Damage{Steps: &spelleffectsvc.DamageSteps{
		Per: spelleffectsvc.StepPerRound, List: []*formula.Formula{ps("1"), ps("2"), ps("4"), ps("8")},
	}}
	for round, want := range map[int]int{1: 1, 2: 2, 3: 3, 4: 4, 5: 4, 19: 4} {
		step, ok, err := fog.ExposureStep(15, round)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, want, step, "round %d", round)
	}
	_, ok, err := near.ExposureStep(15, 7)
	require.NoError(t, err)
	assert.False(t, ok, "without steps every round rolls the same dice")
	_, _, err = fog.ExposureStep(15, 0)
	require.Error(t, err, "rounds start at 1")
}
