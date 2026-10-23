package castsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// ADR 0006 step 10.3: a spell cast on a character immune to it (the derived
// Table 5 immunities) applies nothing and rolls nothing; the one entry names
// the immunity. A spell outside the list still lands as usual.
func TestReceive_Step10_ImmuneSpellAppliesNothing(t *testing.T) {
	immuneTo := func(t *testing.T, dice ...int) *fixture {
		v := viewtest.Single("bard", bardLevel, viewtest.WithIdentity(charID, "Test", ""), viewtest.WithScores(12, 0, 0, 0, 19, 0))
		v.SpellImmunities = []string{"strength"}
		return newFixtureOf(t, catalog(t), v, dice...)
	}

	t.Run("immune: no die, no buff, textOnly immune", func(t *testing.T) {
		f := immuneTo(t) // no die queued: any roll fails the test
		r, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "strength", CasterLevel: 9})
		require.NoError(t, err)
		assert.Equal(t, castsvc.TextOnlyImmune, r.TextOnly)
		assert.Equal(t, &combatsvc.Immunity{Source: combatsvc.ImmunitySourceWisdom, Score: 19}, r.Immunity)
		assert.Equal(t, castsvc.SourceReceived, r.Source)
		assert.Nil(t, r.Buff)
		assert.Empty(t, f.added, "no buff put on the character")
		require.Len(t, f.entries, 1, "one log entry")
		assert.Empty(t, f.entries[0].Snapshots, "nothing changed")
		var p map[string]any
		require.NoError(t, json.Unmarshal(f.entries[0].Payload, &p))
		assert.Equal(t, "immune", p["textOnly"])
		assert.Equal(t, map[string]any{"source": "wisdom", "score": float64(19)}, p["immunity"])
		assert.NotContains(t, p, "buff")
	})

	t.Run("not immune to another spell: it lands", func(t *testing.T) {
		f := immuneTo(t)
		r, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "haste", CasterLevel: 7})
		require.NoError(t, err)
		assert.Nil(t, r.Immunity)
		assert.Empty(t, string(r.TextOnly))
		require.Len(t, f.added, 1)
		assert.Equal(t, []string{buffsvc.StateKeyBuffs}, keys(f.entries[0].Snapshots))
	})
}
