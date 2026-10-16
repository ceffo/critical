package castsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
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
		"buff": map[string]any{"name": "Strength", "rounds": float64(900), "strGain": float64(4), "applied": true},
	}, p)
}

// lowConfidenceBuff is a test-only buff entry the extraction does not trust.
func lowConfidenceBuff(t *testing.T) spelleffectsvc.Catalog {
	t.Helper()
	c, err := spelleffectsadpt.Parse([]byte(`[{"id":"shaky_ward","name":"Shaky Ward","level":2,
	  "kind":"buff","confidence":"low","note":"Test only.","target":"self","duration":{"kind":"rounds","rounds":"level"},
	  "modifiers":{"ac":-2}}]`))
	require.NoError(t, err)
	return c
}

// ADR 0004 step 19: a spell that adds no buff is refused before anything is
// rolled, written or logged; an unknown one is not found.
func TestBuffs_Step19_AddAsEffectRefusals(t *testing.T) {
	tests := []struct {
		name    string
		effects spelleffectsvc.Catalog
		spellID string
		wantErr error
	}{
		{name: "a damage spell rolls nothing", spellID: "fireball", wantErr: spelleffectsvc.ErrNotAnEffect},
		{name: "a utility spell", spellID: "identify", wantErr: spelleffectsvc.ErrNotAnEffect},
		{name: "an ability set applied by hand", spellID: "fist_of_stone", wantErr: spelleffectsvc.ErrNotAnEffect},
		{name: "a low-confidence buff", effects: lowConfidenceBuff(t), spellID: "shaky_ward", wantErr: spelleffectsvc.ErrNotAnEffect},
		{name: "a spell outside the effects seed (priest)", spellID: "bless", wantErr: spelleffectsvc.ErrNotAnEffect},
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
			_, err := f.svc.AddEffect(context.Background(), charID, castsvc.AddEffectRequest{SpellID: tt.spellID})
			require.ErrorIs(t, err, tt.wantErr)
			assert.Empty(t, f.dice.sides, "nothing rolled")
			assert.Empty(t, f.added, "no buff")
			assert.Empty(t, f.entries, "nothing logged")
			assert.Zero(t, f.tx.calls, "refused before the transaction")
			ok, err := f.svc.AddsEffect(context.Background(), tt.spellID)
			require.NoError(t, err)
			assert.False(t, ok, "not offered")
		})
	}
}

// ADR 0004 step 19: the flag the lookup offers the action from and the
// action agree for every seed entry: AddsEffect is true exactly when
// AddEffect puts a buff on the Bard.
func TestBuffs_Step19_FlagMatchesTheAction(t *testing.T) {
	offered := 0
	for _, e := range catalog(t).All() {
		f := newFixture(t, ones(20)...)
		ok, err := f.svc.AddsEffect(context.Background(), e.ID)
		require.NoError(t, err)
		res, err := f.svc.AddEffect(context.Background(), charID, castsvc.AddEffectRequest{SpellID: e.ID})
		if !ok {
			require.ErrorIs(t, err, spelleffectsvc.ErrNotAnEffect, e.ID)
			assert.Empty(t, f.added, e.ID)
			continue
		}
		offered++
		require.NoError(t, err, e.ID)
		require.NotNil(t, res.Buff, e.ID)
		assert.True(t, res.Buff.Applied, e.ID)
		assert.Len(t, f.added, 1, e.ID)
		assert.Nil(t, res.Damage, e.ID)
	}
	assert.Equal(t, 8, offered, "the fixture's nine buff entries but Fist of Stone")
}
