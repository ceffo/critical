package spelleffectsadpt_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
)

// ADR 0033 step 11 (the initiative effect: haste -2, slow +2), on the
// campaign's real seeds/spell_effects.json: every buff with an initiative
// effect, and its value (hesitation delays: +4 is later). A buff given one fails the test until its row is
// written here.
func TestRepoSeeds_ADR0033Step11_InitiativeEffects(t *testing.T) {
	c, err := spelleffectsadpt.New(filepath.Join("..", "..", "..", "..", "seeds"))
	require.NoError(t, err)

	got := map[string]int{}
	for _, e := range c.All() {
		if e.Buff == nil {
			continue
		}
		f, ok := e.Buff.Modifiers["initiative"]
		if !ok {
			continue
		}
		v, err := f.Eval(9, nil)
		require.NoError(t, err, e.ID)
		got[e.ID] = v
	}
	assert.Equal(t, map[string]int{"haste": -2, "hesitation": 4, "slow": 2}, got)
}
