package castsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// ADR 0035 item 6.2: casting a spell corrects the round's initiative by its
// casting time at the caster level (buffsvc.SpellInitiative), as the cast
// transaction's last entry. A failed cast took its casting time too. A spell
// the catalogue gives no casting time for reads as special. A spell received
// or added as an effect is not cast by the character: no correction.
func TestCast_ADR0035Item6_2_CastingCorrectsTheInitiative(t *testing.T) {
	ctx := context.Background()
	fireball := spellstats.Parse(spellstats.Raw{CastingTime: "3"})

	t.Run("a cast: its casting time, after its entry", func(t *testing.T) {
		f := newFixture(t, 1, 2, 3, 4, 5, 6, 1, 2, 3, 4, 5, 6, 1, 2, 3)
		f.stats = map[string]spellstats.Stats{"fireball": fireball}
		f.prepared(slot("fireball", "Fireball", 3))
		_, err := f.svc.Cast(ctx, charID, 0, castsvc.CastRequest{})
		require.NoError(t, err)
		assert.Equal(t, []buffsvc.InitiativeModifier{
			{Source: buffsvc.InitiativeSpell, Value: 3, SpellID: "fireball", Name: "Fireball"},
		}, f.corrections)
		assert.Equal(t, []int{len(f.entries)}, f.correctedAfter, "after the cast entry, the last one")
	})

	t.Run("a failed cast corrects too", func(t *testing.T) {
		f := newFixtureOf(t, catalog(t), clericCaster(), 20)
		f.stats = map[string]spellstats.Stats{"fireball": fireball}
		f.prepared(slot("fireball", "Fireball", 3))
		res, err := f.svc.Cast(ctx, charID, 0, castsvc.CastRequest{})
		require.NoError(t, err)
		require.True(t, res.Failure.Failed)
		assert.Equal(t, []buffsvc.InitiativeModifier{
			{Source: buffsvc.InitiativeSpell, Value: 3, SpellID: "fireball", Name: "Fireball"},
		}, f.corrections)
	})

	t.Run("no casting time known: special", func(t *testing.T) {
		f := newFixture(t, 1)
		f.prepared(slot("alarm", "Alarm", 1))
		_, err := f.svc.Cast(ctx, charID, 0, castsvc.CastRequest{})
		require.NoError(t, err)
		assert.Equal(t, []buffsvc.InitiativeModifier{
			{Source: buffsvc.InitiativeSpell, SpellID: "alarm", Name: "Alarm", Timing: buffsvc.TimingSpecial},
		}, f.corrections)
	})

	t.Run("received or added as an effect: no correction", func(t *testing.T) {
		f := newFixture(t, 2, 3)
		_, err := f.svc.Receive(ctx, charID, castsvc.ReceiveRequest{SpellID: "haste", CasterLevel: 7})
		require.NoError(t, err)
		_, err = f.svc.AddEffect(ctx, charID, castsvc.AddEffectRequest{SpellID: "haste"})
		require.NoError(t, err)
		assert.Empty(t, f.corrections)
	})
}
