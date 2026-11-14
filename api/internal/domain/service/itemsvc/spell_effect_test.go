package itemsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// ADR 0049 step 10: a spell effect is stored as {kind, spellId, level} and read back as written; an
// effect of an older kind reads as before, and the stored text is the wire-stable one a rollback past
// this slice keeps reading (unknown keys ignored).
func TestAbilities_ADR0049Step10_SpellEffectStorageRoundTrip(t *testing.T) {
	in := []itemsvc.Ability{{ID: "zap", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}, Effects: []itemsvc.Effect{
		{Kind: itemsvc.EffectSpell, SpellID: "magic_missile", Level: 5}, {Kind: itemsvc.EffectDice, Dice: "1d6"},
	}}}
	text, ok, err := itemsvc.EncodeAbilities(in)
	require.NoError(t, err)
	require.True(t, ok)
	assert.JSONEq(t, `[{"id":"zap","limit":{"kind":"unlimited"},"effects":[
		{"kind":"spell","spellId":"magic_missile","level":5},{"kind":"dice","dice":"1d6"}]}]`, text)
	out, err := itemsvc.DecodeAbilities(text, true)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

// ADR 0049 step 10: the pure rules of a spell effect, one case a row: a spell and a level 1 to 30 and
// nothing else.
func TestAbilities_ADR0049Step10_SpellEffectRules(t *testing.T) {
	wand := func(e itemsvc.Effect) itemsvc.Item {
		return itemsvc.Item{
			ID: "wand", Name: localized.Of("Wand"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicWand, State: itemsvc.StateCarried, Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1,
			Abilities: []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}, Effects: []itemsvc.Effect{e}}},
		}
	}
	for _, tt := range []struct {
		name string
		e    itemsvc.Effect
		ok   bool
	}{
		{"level 1", itemsvc.Effect{Kind: itemsvc.EffectSpell, SpellID: "magic_missile", Level: 1}, true},
		{"level 30", itemsvc.Effect{Kind: itemsvc.EffectSpell, SpellID: "magic_missile", Level: 30}, true},
		{"level 0", itemsvc.Effect{Kind: itemsvc.EffectSpell, SpellID: "magic_missile", Level: 0}, false},
		{"level 31", itemsvc.Effect{Kind: itemsvc.EffectSpell, SpellID: "magic_missile", Level: 31}, false},
		{"no spell", itemsvc.Effect{Kind: itemsvc.EffectSpell, Level: 5}, false},
		{"not a slug", itemsvc.Effect{Kind: itemsvc.EffectSpell, SpellID: "Magic Missile", Level: 5}, false},
		{"with rounds", itemsvc.Effect{Kind: itemsvc.EffectSpell, SpellID: "magic_missile", Level: 5, Rounds: 3}, false},
		{"with a label", itemsvc.Effect{Kind: itemsvc.EffectSpell, SpellID: "magic_missile", Level: 5, Label: "x"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := wand(tt.e).Validate()
			if tt.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
