package spelleffectsadpt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// healSeed is one level-1 entry of the given kind with the rest of its fields.
func healSeed(kind, rest string) []byte {
	return []byte(`[{"id":"cure","name":"Cure","level":1,"kind":"` + kind + `","confidence":"high",
		"target":"self_or_other","duration":{"kind":"instant"}` + rest + `}]`)
}

// ADR 0008 step 17.1: a heal entry carries its dice, or full.
func TestParse_Heal(t *testing.T) {
	c, err := spelleffectsadpt.Parse(healSeed("heal", `,"heal":{"dice":"2d8+1"}`), testGroups)
	require.NoError(t, err)
	e, _ := c.Get("cure")
	require.NotNil(t, e.Heal)
	assert.Equal(t, "2d8+1", e.Heal.Dice.String())
	assert.False(t, e.Heal.Full)
	assert.True(t, e.Heals())

	c, err = spelleffectsadpt.Parse(healSeed("heal", `,"heal":{"full":true},"duration":{"kind":"permanent"}`), testGroups)
	require.NoError(t, err)
	e, _ = c.Get("cure")
	assert.Equal(t, &spelleffectsvc.Heal{Full: true}, e.Heal)
}

// ADR 0008 step 17.1: the loader refuses a heal block on another kind, a
// heal entry without one, both or neither of dice and full, dice that are
// not a sum or go below 0 at some level, a duration that puts a row and any
// other block.
func TestParse_HealRefused(t *testing.T) {
	tests := map[string]struct{ kind, rest, want string }{
		"heal on a buff":     {"buff", `,"heal":{"dice":"1d8"},"modifiers":{"ac":"-1"}`, "only a heal spell can have a heal block"},
		"heal on a utility":  {"utility", `,"heal":{"dice":"1d8"}`, "only a heal spell can have a heal block"},
		"no heal block":      {"heal", ``, "a heal spell needs a heal block"},
		"neither":            {"heal", `,"heal":{}`, "heal: set exactly one of dice and full"},
		"both":               {"heal", `,"heal":{"dice":"1d8","full":true}`, "heal: set exactly one of dice and full"},
		"bad formula":        {"heal", `,"heal":{"dice":"1d"}`, "heal.dice"},
		"dice not added":     {"heal", `,"heal":{"dice":"2*1d4"}`, "heal.dice: expand 2*1d4"},
		"below 0 at a level": {"heal", `,"heal":{"dice":"1d4-level"}`, "heal.dice 1d4-level: goes below 0 at level 2"},
		"lasts rounds":       {"heal", `,"heal":{"dice":"1d8"},"duration":{"kind":"rounds","rounds":"level"}`, "duration.kind"},
		"damage":             {"heal", `,"heal":{"dice":"1d8"},"damage":{"dice":"1d6"}`, "a heal spell cannot have buff, charges or damage fields"},
		"modifiers":          {"heal", `,"heal":{"dice":"1d8"},"modifiers":{"ac":"-1"}`, "a heal spell cannot have buff, charges or damage fields"},
		"modes":              {"heal", `,"heal":{"dice":"1d8"},"modes":[{"key":"a"},{"key":"b"}]`, "only a damage spell can have modes"},
		"exposure":           {"heal", `,"heal":{"dice":"1d8"},"exposure":{"damage":{"dice":"1d4"}}`, "only a damage spell can have an exposure"},
		"a cast mechanic":    {"heal", `,"heal":{"dice":"1d8"},"attack_roll":true`, "a heal spell cannot have cast mechanics"},
		"end effects":        {"heal", `,"heal":{"dice":"1d8"},"end_effects":[{"kind":"heal","dice":"1d4","on":["expired"]}]`, "a heal spell cannot have end effects"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := spelleffectsadpt.Parse(healSeed(tt.kind, tt.rest), testGroups)
			require.Error(t, err)
			assert.ErrorContains(t, err, `spell "cure"`)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}
