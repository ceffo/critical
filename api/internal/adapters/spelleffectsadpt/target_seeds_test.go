package spelleffectsadpt_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// ADR 0027 step 10.1, on the campaign's real seeds/spell_effects.json: where
// a cast of every buff lands when the Bard names no target, and the target
// choice its cast offers (nil: a plain Cast button). Only a defensive buff
// that is not self-only offers one; a hostile buff (Blindness, Pain Touch,
// Slow, the Malisons) lands on somebody else and offers none. A buff added to
// the seeds fails the test until its row is written here.
func TestRepoSeeds_EveryBuffDefaultTargetAndChoice(t *testing.T) {
	c, err := spelleffectsadpt.New(filepath.Join("..", "..", "..", "..", "seeds"))
	require.NoError(t, err)

	onMe := &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnSelf, Alternate: spelleffectsvc.CastOnOther}
	onAlly := &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnOther, Alternate: spelleffectsvc.CastOnSelf}
	type want struct {
		onSelf bool
		choice *spelleffectsvc.TargetChoice
	}
	wants := map[string]want{
		// Self or other, defensive: on the Bard, the arrow offers an ally.
		"armor":                 {true, onMe},
		"enlarge":               {true, onMe},
		"protection_from_evil":  {true, onMe},
		"cats_grace":            {true, onMe},
		"strength":              {true, onMe},
		"improved_invisibility": {true, onMe},
		"improved_strength":     {true, onMe},
		"stoneskin":             {true, onMe},
		"dragon_scales":         {true, onMe},
		// Friendly areas: on the Bard, the arrow offers not on him.
		"haste":                          {true, onMe},
		"protection_from_evil_10_radius": {true, onMe},
		// Other, defensive: on an ally, the arrow offers the Bard.
		"enchanted_weapon": {false, onAlly},
		// Hostile: not on the Bard, no arrow.
		"blindness":       {false, nil},
		"pain_touch":      {false, nil},
		"slow":            {false, nil},
		"minor_malison":   {false, nil},
		"greater_malison": {false, nil},
		// Self-only: always on the Bard, no arrow.
		"expeditious_retreat":    {true, nil},
		"fist_of_stone":          {true, nil},
		"friends":                {true, nil},
		"shield":                 {true, nil},
		"blur":                   {true, nil},
		"mirror_image":           {true, nil},
		"spirit_armor":           {true, nil},
		"psychic_protection":     {true, nil},
		"tensers_transformation": {true, nil},
		"trollish_fortitude":     {true, nil},
	}

	const level = 15
	seen := map[string]bool{}
	for _, e := range c.All() {
		if e.Kind != spelleffectsvc.KindBuff {
			continue
		}
		seen[e.ID] = true
		w, ok := wants[e.ID]
		if !assert.True(t, ok, "buff %s has no expected row", e.ID) {
			continue
		}
		onSelf, err := e.DefaultOnSelf(level)
		require.NoError(t, err, e.ID)
		assert.Equal(t, w.onSelf, onSelf, "%s: default on the Bard", e.ID)
		choice, err := e.TargetChoiceAt(level)
		require.NoError(t, err, e.ID)
		assert.Equal(t, w.choice, choice, "%s: target choice", e.ID)
	}
	for id := range wants {
		assert.True(t, seen[id], "%s is no longer a buff in the seeds", id)
	}
}
