package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// warriorStr is a warrior's sheet Strength (ADR 0030 step 10's scale).
func warriorStr(score, exceptional int) buffsvc.BaseStrength {
	return buffsvc.BaseStrength{StrScore: buffsvc.StrScore{Score: score, Exceptional: exceptional}, Warrior: true}
}

// ADR 0030 step 10: Strength spells and 18/xx. The score in effect is ADR
// 0004's for every class; a warrior's Strength bonuses past 18 add 10
// percentile points a point (10.1), Improved Strength counts an 18/xx as 18
// (10.2), nothing lowers the sheet's percentile (10.3), and every other
// class stops at a plain 18.
func TestAggregate_Step10_StrengthSpellsAnd18xx(t *testing.T) {
	tests := []struct {
		name  string
		base  buffsvc.BaseStrength
		buffs []buffsvc.BuffEffect
		want  buffsvc.StrScore
	}{
		// 10.1
		{"17 with +3: 18/20", warriorStr(17, 0), []buffsvc.BuffEffect{strength(3)}, buffsvc.StrScore{Score: 18, Exceptional: 20}},
		{"18/50 with +4: 18/90", warriorStr(18, 50), []buffsvc.BuffEffect{strength(4)}, buffsvc.StrScore{Score: 18, Exceptional: 90}},
		{"two bonuses add up: 16 with +2 and +3: 18/30", warriorStr(16, 0), []buffsvc.BuffEffect{strength(2), strength(3)}, buffsvc.StrScore{Score: 18, Exceptional: 30}},
		{"plain 18 with +1: 18/10", warriorStr(18, 0), []buffsvc.BuffEffect{strength(1)}, buffsvc.StrScore{Score: 18, Exceptional: 10}},
		{"capped at 18/00", warriorStr(18, 75), []buffsvc.BuffEffect{strength(6)}, buffsvc.StrScore{Score: 18, Exceptional: 100}},
		{"short of 18: 14 with +3: 17", warriorStr(14, 0), []buffsvc.BuffEffect{strength(3)}, buffsvc.StrScore{Score: 17}},
		{"just 18: 15 with +3: 18", warriorStr(15, 0), []buffsvc.BuffEffect{strength(3)}, buffsvc.StrScore{Score: 18}},
		{"no buff keeps the sheet's 18/50", warriorStr(18, 50), nil, buffsvc.StrScore{Score: 18, Exceptional: 50}},
		{"a sheet 19 stays 19", warriorStr(19, 0), []buffsvc.BuffEffect{strength(3)}, buffsvc.StrScore{Score: 19}},
		// 10.2
		{"18/50 with Improved +4: 22", warriorStr(18, 50), []buffsvc.BuffEffect{improvedStrength(4)}, buffsvc.StrScore{Score: 22}},
		{
			"Improved replaces the bonuses: 18/50 with +4 and Improved +2: 20", warriorStr(18, 50),
			[]buffsvc.BuffEffect{strength(4), improvedStrength(2)},
			buffsvc.StrScore{Score: 20},
		},
		// 10.3
		{
			"Improved to a plain 18 keeps 18/50", warriorStr(18, 50),
			[]buffsvc.BuffEffect{improvedStrength(0)},
			buffsvc.StrScore{Score: 18, Exceptional: 50},
		},
		{"Improved to 18 from 16: plain 18", warriorStr(16, 0), []buffsvc.BuffEffect{improvedStrength(2)}, buffsvc.StrScore{Score: 18}},
		// Other classes stop at 18 as today.
		{"a bard: 17 with +3: 18", buffsvc.BaseStrength{StrScore: buffsvc.StrScore{Score: 17}}, []buffsvc.BuffEffect{strength(3)}, buffsvc.StrScore{Score: 18}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(tt.buffs, nil).Once()
			f.powers()
			got, err := f.svc.Aggregate(context.Background(), charID, tt.base, buffsvc.WeaponNone)
			require.NoError(t, err)
			assert.Equal(t, tt.want, buffsvc.StrScore{Score: got.EffectiveStr, Exceptional: got.EffectiveStrExceptional})
		})
	}
}

// ADR 0030 step 10.4: gates read an 18/xx as 18: the Power Gem (STR 18 or
// more) is on at 18/50, a STR 19 requirement is not met at 18/00.
func TestAggregate_Step10_4_GatesRead18xxAs18(t *testing.T) {
	needs19 := buffsvc.BuffEffect{ID: "n", Name: "Needs 19", Effects: buffsvc.BuffEffects{MinStr: intPtr(19), ToHit: intPtr(3)}}
	f := newFixture(t)
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{powerGem, needs19, strength(5)}, nil).Once()
	f.powers()
	got, err := f.svc.Aggregate(context.Background(), charID, warriorStr(18, 50), buffsvc.WeaponNone)
	require.NoError(t, err)
	assert.Equal(t, 100, got.EffectiveStrExceptional)
	assert.Equal(t, 5, got.WeaponBuff, "the Power Gem's gate (18) is met")
	assert.Zero(t, got.ToHit, "a STR 19 gate is not met at 18/00")
}

// ADR 0030 steps 9 and 10: the Strength the sheet shows is a fighter's
// 18/xx and the 18/xx in effect; BaseStrengthOf gives the warrior's scale to
// a character whose derived block has exceptional_strength (a fighter) only.
func TestGetEffectiveStr_Step10_TheFightersPercentile(t *testing.T) {
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).
		Return(&charactersvc.Character{ID: charID, Class: "fighter", Str: 18, StrExceptional: 50, Derived: charactersvc.Derived{Group: "warrior", ExceptionalStrength: true}}, nil).Once()
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{strength(2)}, nil).Once()
	got, err := f.svc.GetEffectiveStr(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, buffsvc.Strength{Base: 18, Effective: 18, BaseExceptional: 50, EffectiveExceptional: 70}, got)

	assert.False(t, buffsvc.BaseStrengthOf(&charactersvc.Character{Class: "bard", Str: 18, Derived: charactersvc.Derived{Group: "rogue"}}).Warrior)
	assert.True(t, buffsvc.BaseStrengthOf(&charactersvc.Character{Class: "fighter", Str: 18, Derived: charactersvc.Derived{Group: "warrior", ExceptionalStrength: true}}).Warrior)
}
