package castsvc_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// ADR 0004 step 19: a spell that adds a buff is added as an effect from the
// lookup at the Bard's level (15): the buff a cast would put on him, rolled
// with exactly the dice its entry rolls, no slot, no cast mechanics, on the
// Bard whatever the spell's target (self-only and hostile area spells
// included). One cast entry, action "effect", with only the buff list's
// snapshots.
func TestBuffs_Step19_AddAsEffect(t *testing.T) {
	tests := []struct {
		name      string
		spellID   string
		dice      []int
		wantSides []int
		want      buffsvc.BuffEffect
	}{
		{
			name: "strength: rogue 1d6, 60 rounds per Bard level", spellID: "strength",
			dice: []int{5}, wantSides: []int{6},
			want: buffsvc.BuffEffect{
				ID: buffsAdded, Name: "Strength", SpellID: "strength", Duration: 900,
				Effects: buffsvc.BuffEffects{StrBonus: intPtr(5), StrMax: intPtr(18)},
			},
		},
		{
			name: "mirror image (self only) is accepted: 1d4 + 15/3 images, 45 rounds", spellID: "mirror_image",
			dice: []int{2}, wantSides: []int{4},
			want: buffsvc.BuffEffect{ID: buffsAdded, Name: "Mirror Image", SpellID: "mirror_image", Duration: 45, Charges: 7},
		},
		{
			name: "stoneskin: 1d4 + 15/2 layers", spellID: "stoneskin",
			dice: []int{3}, wantSides: []int{4},
			want: buffsvc.BuffEffect{ID: buffsAdded, Name: "Stoneskin", SpellID: "stoneskin", Charges: 10},
		},
		{
			// ADR 0033 step 11: the seed's ac_set is the base_ac effect.
			name: "armor: AC 6 as if armoured, until discharged", spellID: "armor",
			want: buffsvc.BuffEffect{
				ID: buffsAdded, Name: "Armor", SpellID: "armor",
				Effects: buffsvc.BuffEffects{BaseAC: intPtr(6)},
			},
		},
		{
			name: "slow (hostile area) lands on the Bard: 3 + 15 rounds", spellID: "slow",
			want: buffsvc.BuffEffect{
				ID: buffsAdded, Name: "Slow", SpellID: "slow", Duration: 18,
				Effects: buffsvc.BuffEffects{ToHit: intPtr(-4), AC: intPtr(4)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.dice...)
			res, err := f.svc.AddEffect(context.Background(), charID, castsvc.AddEffectRequest{SpellID: tt.spellID})
			require.NoError(t, err)
			assert.Empty(t, f.dice.vals, "every queued die is rolled")
			assert.Equal(t, tt.wantSides, f.dice.sides, "exactly the buff's dice, no damage")
			assert.Equal(t, castsvc.SourceEffect, res.Source)
			assert.Equal(t, bardLevel, res.CasterLevel, "at the Bard's level")
			assert.Nil(t, res.SlotIndex)
			assert.Nil(t, res.Damage)
			assert.Nil(t, res.Attack)
			assert.Nil(t, res.Consumed)
			assert.Equal(t, castsvc.TextOnlyNone, res.TextOnly)
			require.NotNil(t, res.Buff)
			assert.True(t, res.Buff.Applied)
			require.Len(t, f.added, 1)
			assert.Equal(t, tt.want, f.added[0])
			assert.Equal(t, 1, f.tx.calls, "one transaction")
			require.Len(t, f.entries, 1, "one log entry")
			assert.Equal(t, []string{buffsvc.StateKeyBuffs}, keys(f.entries[0].Snapshots), "no slot used")
			assert.JSONEq(t, `"effect"`, string(payloadField(t, f.entries[0].Payload, "action")))
		})
	}
}

// ADR 0004 step 19: the log entry words itself from its payload: the action,
// the spell and the Bard's level, and the buff's rolls.
func TestBuffs_Step19_AddAsEffectLogPayload(t *testing.T) {
	f := newFixture(t, 4)
	_, err := f.svc.AddEffect(context.Background(), charID, castsvc.AddEffectRequest{SpellID: "strength"})
	require.NoError(t, err)
	e := f.entries[0]
	assert.Empty(t, e.Summary, "the UI words the entry from its payload")
	var p map[string]any
	require.NoError(t, json.Unmarshal(e.Payload, &p))
	assert.Equal(t, map[string]any{
		"action": "effect", "spellId": "strength", "spell": "Strength", "casterLevel": float64(bardLevel),
		"buff": map[string]any{
			"name": "Strength", "rounds": float64(900), "strGain": float64(4), "applied": true, "id": "added-buff",
			"durationRoll": map[string]any{"formula": "60*level", "dice": []any{}, "total": float64(900)},
			"strGainRoll":  map[string]any{"formula": "1d6", "dice": []any{map[string]any{"sides": float64(6), "value": float64(4)}}, "total": float64(4)},
			"effects":      map[string]any{"str_bonus": float64(4), "str_max": float64(18)},
		},
		"level": float64(2), "kind": "buff", "confidence": "high", "target": "self_or_other", "untracked": []any{},
	}, p)
}

// lowConfidenceInstant is a test-only buff entry the extraction does not
// trust and that does not last a number of rounds.
func lowConfidenceInstant(t *testing.T) spelleffectsvc.Catalog {
	t.Helper()
	c, err := spelleffectsadpt.Parse([]byte(`[{"id":"shaky_ward","name":"Shaky Ward","level":2,
	  "kind":"buff","confidence":"low","note":"Test only.","target":"self","duration":{"kind":"special"},
	  "modifiers":{"ac":-2}}]`), testGroups)
	require.NoError(t, err)
	return c
}

// ADR 0004 steps 19 and 20: a spell that gets no row is refused before
// anything is rolled, written or logged; an unknown one is not found.
func TestBuffs_Step19_AddAsEffectRefusals(t *testing.T) {
	tests := []struct {
		name    string
		effects spelleffectsvc.Catalog
		spellID string
		stats   spellstats.Duration
		wantErr error
	}{
		{name: "a damage spell with no duration rolls nothing", spellID: "fireball", wantErr: spelleffectsvc.ErrNotAnEffect},
		{name: "an instantaneous utility spell", spellID: "dispel_magic", wantErr: spelleffectsvc.ErrNotAnEffect},
		{name: "a utility spell with a special duration", spellID: "alarm", wantErr: spelleffectsvc.ErrNotAnEffect},
		{name: "a low-confidence buff with no rounds", effects: lowConfidenceInstant(t), spellID: "shaky_ward", wantErr: spelleffectsvc.ErrNotAnEffect},
		{name: "a permanent priest spell", spellID: "cure_light_wounds", stats: spellstats.ParseDuration("Permanent"), wantErr: spelleffectsvc.ErrNotAnEffect},
		{name: "a priest spell lasting the time of chanting", spellID: "chant", stats: spellstats.ParseDuration("Time of chanting"), wantErr: spelleffectsvc.ErrNotAnEffect},
		{name: "an unknown spell", spellID: "no_such_spell", wantErr: domain.ErrNotFound},
		{name: "no spell id", spellID: " ", wantErr: domain.ErrInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effects := tt.effects
			if effects == nil {
				effects = catalog(t)
			}
			f := newFixtureWith(t, effects)
			f.stats = map[string]spellstats.Stats{tt.spellID: {Duration: tt.stats}}
			_, err := f.svc.AddEffect(context.Background(), charID, castsvc.AddEffectRequest{SpellID: tt.spellID})
			require.ErrorIs(t, err, tt.wantErr)
			assert.Empty(t, f.dice.sides, "nothing rolled")
			assert.Empty(t, f.added, "no row")
			assert.Empty(t, f.entries, "nothing logged")
			assert.Zero(t, f.tx.calls, "refused before the transaction")
			assert.False(t, f.svc.AddsEffect(tt.spellID, tt.stats), "not offered")
		})
	}
}

// fixtureDurations are the catalogue durations of the test catalogue, by
// spell id, parsed as the importer parses them.
func fixtureDurations(t *testing.T) map[string]spellstats.Duration {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	var c struct {
		Spells []struct {
			ID       string `json:"id"`
			Duration string `json:"duration"`
		} `json:"spells"`
	}
	require.NoError(t, json.Unmarshal(raw, &c))
	out := map[string]spellstats.Duration{}
	for _, s := range c.Spells {
		out[s.ID] = spellstats.ParseDuration(s.Duration)
	}
	return out
}

// ADR 0004 step 20: the flag the lookup offers the action from and the
// action agree for every seed entry and every spell of the catalogue (the
// Priest spells, with no entry, by their catalogue duration): AddsEffect is
// true exactly when AddEffect puts a row on the Bard. Replaces step 19's
// check, whose rule was the buffs only.
func TestBuffs_Step20_OfferMatchesTheAction(t *testing.T) {
	durations := fixtureDurations(t)
	ids := map[string]bool{}
	for _, e := range catalog(t).All() {
		ids[e.ID] = true
	}
	for id := range durations {
		ids[id] = true
	}
	offered, priest := 0, 0
	for id := range ids {
		f := newFixture(t, ones(20)...)
		f.stats = map[string]spellstats.Stats{id: {Duration: durations[id]}}
		ok := f.svc.AddsEffect(id, durations[id])
		res, err := f.svc.AddEffect(context.Background(), charID, castsvc.AddEffectRequest{SpellID: id})
		if !ok {
			require.ErrorIs(t, err, spelleffectsvc.ErrNotAnEffect, id)
			assert.Empty(t, f.added, id)
			continue
		}
		offered++
		if _, has := catalog(t).Get(id); !has {
			priest++
		}
		require.NoError(t, err, id)
		require.NotNil(t, res.Buff, id)
		assert.True(t, res.Buff.Applied, id)
		require.Len(t, f.added, 1, id)
		assert.False(t, f.added[0].OnOther, "an added effect is on the Bard: "+id)
		assert.Nil(t, res.Damage, id)
	}
	// Every fixture entry that lasts rounds or adds a buff (all but the
	// instant, special and until-discharged non-buffs), plus the Priest
	// spells that last rounds (detect magic, bless, command, produce flame,
	// flame walk).
	assert.Equal(t, 26, offered)
	assert.Equal(t, 5, priest)
}
