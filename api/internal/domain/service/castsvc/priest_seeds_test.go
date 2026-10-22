package castsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// The priest spells of the real seeds/spell_effects.json, cast by a level 15
// cleric (the caster's class group is the priest's, ADR 0030 step 2), one
// case per way an existing effect kind is used: a plain modifier, a formula of
// the caster level that sets the AC, a conditional modifier, a modifier shown
// and not applied, a hostile initiative buff, a damage spell with a save,
// damage with an exposure, an exposure with modes, cast-time modes, damage
// steps and a spell with an attack roll. The dice are distinct, so a reused
// or skipped die fails; a die left in the queue fails the case.
func TestCast_PriestSeedEntries(t *testing.T) {
	ac := 4
	tests := []struct {
		name     string
		slot     string
		level    int
		onSelf   *bool
		mode     string
		dice     []int
		sides    []int // the dice thrown, in order
		attack   bool
		wantBuff *buffsvc.BuffEffect
		check    func(t *testing.T, r castsvc.CastResult)
	}{
		{
			name: "bless: +1 to hit for 6 rounds", slot: "bless", level: 1,
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Bless", SpellID: "bless", Duration: 6, Effects: buffsvc.BuffEffects{ToHit: intPtr(1)}},
		},
		{
			name: "barkskin: AC 6 improved by 1 per 4 levels (3 at 15), the save bonus shown only vs non-magic attacks", slot: "barkskin", level: 2,
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Barkskin", SpellID: "barkskin", Duration: 19, Effects: buffsvc.BuffEffects{BaseAC: intPtr(3)}},
			check: func(t *testing.T, r castsvc.CastResult) {
				assert.Equal(t, []castsvc.Untracked{{Key: "save", Value: 1, Condition: "saves against attack forms other than magic"}}, r.Untracked)
			},
		},
		{
			name: "magical vestment: AC 5 improved by 1 per 3 levels beyond the 5th (2 at 15)", slot: "magical_vestment", level: 3,
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Magical Vestment", SpellID: "magical_vestment", Duration: 75, Effects: buffsvc.BuffEffects{BaseAC: intPtr(2)}},
		},
		{
			name: "protection from evil: every modifier is conditional, shown and not applied", slot: "protection_from_evil_priest", level: 1,
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Protection From Evil", SpellID: "protection_from_evil_priest", Duration: 45},
			check: func(t *testing.T, r castsvc.CastResult) {
				assert.Equal(t, []castsvc.Untracked{
					{Key: "attackers_to_hit", Value: -2, Condition: "attacks and saves against evil creatures"},
					{Key: "save", Value: 2, Condition: "attacks and saves against evil creatures"},
				}, r.Untracked)
			},
		},
		{
			name: "hesitation cast on oneself: initiative +4, a hostile area spell does not land on the cleric by default", slot: "hesitation", level: 2,
			onSelf:   boolPtr(true),
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Hesitation", SpellID: "hesitation", Duration: 15, Effects: buffsvc.BuffEffects{Initiative: intPtr(4)}},
		},
		{
			name: "flame strike: 6d8 fire, a save halves", slot: "flame_strike", level: 5,
			dice: []int{1, 2, 3, 4, 5, 6}, sides: []int{8, 8, 8, 8, 8, 8},
			check: func(t *testing.T, r castsvc.CastResult) {
				assert.Equal(t, &spelleffectsvc.Save{Type: "spell", Effect: spelleffectsvc.SaveHalf}, r.Save)
				require.NotNil(t, r.Damage)
				assert.Equal(t, "6d8", r.Damage.Dice)
				assert.Equal(t, "fire", r.Damage.Type)
				assert.Equal(t, 21, r.Damage.Total)
			},
		},
		{
			name: "fire storm: 2d8 + level now, and the same each round of exposure", slot: "fire_storm", level: 7,
			dice: []int{3, 5}, sides: []int{8, 8},
			check: func(t *testing.T, r castsvc.CastResult) {
				require.NotNil(t, r.Damage)
				assert.Equal(t, 3+5+15, r.Damage.Total)
				require.NotNil(t, r.Exposure)
				require.NotNil(t, r.Exposure.Damage)
				assert.Equal(t, 15, r.Exposure.Damage.Dice.Constant, "the caster's level")
			},
		},
		{
			name: "wall of fire (priest): casting rolls nothing, the exposure has three modes", slot: "wall_of_fire_priest", level: 5,
			check: func(t *testing.T, r castsvc.CastResult) {
				assert.Nil(t, r.Damage)
				require.NotNil(t, r.Exposure)
				require.Len(t, r.Exposure.Modes, 3)
				assert.Equal(t, "passing_through", r.Exposure.Modes[2].Key)
				assert.Equal(t, 15, r.Exposure.Modes[2].Damage.Dice.Constant, "4d4 + the caster's level")
			},
		},
		{
			name: "fire seeds, default mode: 2d8 per acorn", slot: "fire_seeds", level: 6,
			dice: []int{4, 7}, sides: []int{8, 8},
			check: func(t *testing.T, r castsvc.CastResult) {
				assert.Equal(t, "acorns", r.Mode)
				assert.Equal(t, 11, r.Damage.Total)
			},
		},
		{
			name: "fire seeds, berries: 1d8", slot: "fire_seeds", level: 6, mode: "berries",
			dice: []int{6}, sides: []int{8},
			check: func(t *testing.T, r castsvc.CastResult) {
				assert.Equal(t, "berries", r.Mode)
				assert.Equal(t, 6, r.Damage.Total)
			},
		},
		{
			name: "heat metal: no damage the first round, 1d4, three of 2d4, 1d4, then none", slot: "heat_metal", level: 2,
			dice: []int{3, 1, 2, 3, 4, 1, 4, 2}, sides: []int{4, 4, 4, 4, 4, 4, 4, 4},
			check: func(t *testing.T, r castsvc.CastResult) {
				require.NotNil(t, r.Damage)
				assert.Equal(t, spelleffectsvc.StepPerRound, r.Damage.StepsPer)
				require.Len(t, r.Damage.Rolls, 6, "the steps after the last with damage are dropped")
				var steps, totals []int
				for _, roll := range r.Damage.Rolls {
					steps = append(steps, roll.Step)
					totals = append(totals, roll.Total)
				}
				assert.Equal(t, []int{1, 2, 3, 4, 5, 6}, steps)
				assert.Equal(t, []int{0, 3, 3, 7, 5, 2}, totals)
			},
		},
		{
			name: "sol's searing orb: 6d6 after an attack roll against the target's AC", slot: "sols_searing_orb", level: 6,
			dice: []int{1, 2, 3, 4, 5, 6}, sides: []int{6, 6, 6, 6, 6, 6}, attack: true,
			check: func(t *testing.T, r castsvc.CastResult) {
				require.NotNil(t, r.Attack)
				assert.Equal(t, 21, r.Damage.Total)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixtureOf(t, repoCatalog(t), casterOf("cleric"), tt.dice...)
			if tt.attack {
				f.attacks.EXPECT().RollSpellToHit(mock.Anything, charID, ac).Return(&combatsvc.ToHitRoll{D20Roll: 12, TargetAC: ac, Hit: true}, nil)
			}
			f.prepared(slot(tt.slot, tt.slot, tt.level))
			req := castsvc.CastRequest{OnSelf: tt.onSelf, Mode: tt.mode}
			if tt.attack {
				req.TargetAC = &ac
			}
			r, err := f.svc.Cast(context.Background(), charID, 0, req)
			require.NoError(t, err)
			assert.Empty(t, f.dice.vals, "every queued die is rolled")
			assert.Equal(t, tt.sides, f.dice.sides, "the dice thrown")
			if tt.wantBuff != nil {
				require.Len(t, f.added, 1)
				assert.Equal(t, *tt.wantBuff, f.added[0])
			}
			if tt.check != nil {
				tt.check(t, r)
			}
		})
	}
}
