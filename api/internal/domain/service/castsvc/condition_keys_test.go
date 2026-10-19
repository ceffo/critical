package castsvc_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// repoCatalog is the campaign's real seeds/spell_effects.json.
func repoCatalog(t *testing.T) spelleffectsvc.Catalog {
	t.Helper()
	c, err := spelleffectsadpt.New(filepath.Join("..", "..", "..", "..", "..", "seeds"))
	require.NoError(t, err)
	return c
}

// A buff's condition (vs evil, vs magic) limits the modifiers condition_keys
// names, or all of them without it: those are shown with the condition, never
// applied; the others apply as always (a buff with none applied still goes
// on, for its duration). On the real seeds, cast by the Bard
// (level 15). Spirit Armor's ac_set is the base_ac effect of ADR 0033 step 11:
// its condition is about the save bonus only.
func TestCast_RepoSeedConditionLimitsItsModifiers(t *testing.T) {
	tests := []struct {
		name          string
		slot          string
		level         int
		wantEffects   buffsvc.BuffEffects
		wantDuration  int
		wantUntracked []castsvc.Untracked
	}{
		{
			name: "spirit armor: AC 4 as if armoured applied, the save bonus vs magic shown", slot: "spirit_armor", level: 3,
			wantEffects: buffsvc.BuffEffects{BaseAC: intPtr(4)}, wantDuration: 30,
			wantUntracked: []castsvc.Untracked{{Key: "save", Value: 3, Condition: "save bonus applies to magical attacks only"}},
		},
		{
			name: "blur: the attackers' penalties shown without the condition, the save bonus with it", slot: "blur", level: 2,
			wantDuration: 18,
			wantUntracked: []castsvc.Untracked{
				{Key: "attackers_to_hit", Value: -2}, {Key: "attackers_to_hit_first", Value: -4},
				{Key: "save", Value: 1, Condition: "save bonus applies to direct magical attacks only"},
			},
		},
		{
			name: "protection from evil: no condition_keys, the condition limits every modifier", slot: "protection_from_evil", level: 1,
			wantDuration: 30,
			wantUntracked: []castsvc.Untracked{
				{Key: "attackers_to_hit", Value: -2, Condition: "attacks and saves against evil creatures"},
				{Key: "save", Value: 2, Condition: "attacks and saves against evil creatures"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixtureWith(t, repoCatalog(t))
			f.prepared(slot(tt.slot, tt.slot, tt.level))
			r, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
			require.NoError(t, err)
			assert.Equal(t, tt.wantUntracked, r.Untracked)
			assert.Equal(t, castsvc.TextOnlyNone, r.TextOnly)
			require.Len(t, f.added, 1)
			assert.Equal(t, tt.wantEffects, f.added[0].Effects)
			assert.Equal(t, tt.wantDuration, f.added[0].Duration)
		})
	}
}

// Received from somebody else (added as an effect, ADR 0004 step 19), Spirit
// Armor is the same base_ac 4 buff.
func TestBuffs_Step19_AddAsEffectRepoSpiritArmor(t *testing.T) {
	f := newFixtureWith(t, repoCatalog(t))
	_, err := f.svc.AddEffect(context.Background(), charID, castsvc.AddEffectRequest{SpellID: "spirit_armor"})
	require.NoError(t, err)
	require.Len(t, f.added, 1)
	assert.Equal(t, buffsvc.BuffEffects{BaseAC: intPtr(4)}, f.added[0].Effects)
}
