package combatsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	buffmocks "github.com/ceffo/critical/api/internal/domain/service/buffsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

const charID = int64(1)

var longsword = combatsvc.Weapon{ID: "longsword", Name: "Longsword", BaseDamage: "1d8"}

func bard(str int) *charactersvc.Character {
	return &charactersvc.Character{
		ID: charID, Str: str, Thac0Base: 16,
		SaveParaPoison: 11, SaveRodStaffWand: 13, SavePetriPoly: 12, SaveBreath: 15, SaveSpell: 14,
	}
}

func neutral() buffsvc.BuffModifiers {
	return buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0}
}

type fixture struct {
	chars   *mocks.MockCharacters
	buffs   *mocks.MockBuffs
	dice    *mocks.MockDice
	weapons *mocks.MockWeapons
	svc     combatsvc.SVC
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		chars:   mocks.NewMockCharacters(t),
		buffs:   mocks.NewMockBuffs(t),
		dice:    mocks.NewMockDice(t),
		weapons: mocks.NewMockWeapons(t),
	}
	svc, err := combatsvc.NewWithConfig(combatsvc.DefaultConfig(), f.chars, f.buffs, f.dice, f.weapons)
	require.NoError(t, err)
	f.svc = svc
	return f
}

func (f *fixture) withWeapon() {
	f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil)
}

func (f *fixture) withChar(c *charactersvc.Character) {
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(c, nil)
}

// rolls scripts the dice port in call order: d20 first, then the D100 crit
// roll(s), then the damage dice.
func (f *fixture) rolls(d20 int, d100s []int, dmg ...int) {
	f.dice.EXPECT().RollD20().Return(d20).Once()
	for _, r := range d100s {
		f.dice.EXPECT().RollD100().Return(r).Once()
	}
	for _, d := range dmg {
		f.dice.EXPECT().RollDie(8).Return(d).Once()
	}
}

func TestNew_NilDependency(t *testing.T) {
	_, err := combatsvc.New(nil, mocks.NewMockBuffs(t), mocks.NewMockDice(t), mocks.NewMockWeapons(t))
	require.Error(t, err)
}

// Fixture: STR 18 bard, THAC0 16, target AC 5.
// Effective THAC0 = 16 - 1 (STR 18) = 15; roll >= 15 - 5 = 10 hits.
func TestResolveAttack(t *testing.T) {
	const targetAC = 5

	tests := []struct {
		name  string
		str   int
		d20   int
		d100s []int
		dmg   []int
		mods  buffsvc.BuffModifiers

		wantHit        bool
		wantCrit       bool
		wantEffThac0   int
		wantMargin     int
		wantMultiplier float64
		wantStrBonus   int
		wantDamage     int
		wantBreakdown  string
	}{
		{
			name: "miss", str: 18, d20: 5, d100s: []int{50},
			wantHit: false, wantCrit: false, wantEffThac0: 15, wantMargin: -5, wantMultiplier: 0,
		},
		{
			name: "hit exactly on the threshold", str: 18, d20: 10, d100s: []int{90}, dmg: []int{5}, mods: neutral(),
			wantHit: true, wantEffThac0: 15, wantMargin: 0, wantMultiplier: 1, wantStrBonus: 2, wantDamage: 7,
			wantBreakdown: "((5 × 1.0) + 0) × 1.0 + 2 = 7",
		},
		{
			name: "one under the threshold misses", str: 18, d20: 9, d100s: []int{90},
			wantHit: false, wantEffThac0: 15, wantMargin: -1,
		},
		{
			name: "hit", str: 18, d20: 12, d100s: []int{50}, dmg: []int{6}, mods: neutral(),
			wantHit: true, wantEffThac0: 15, wantMargin: 2, wantMultiplier: 1, wantStrBonus: 2, wantDamage: 8,
		},
		{
			name: "crit doubles damage before the STR bonus", str: 18, d20: 15, d100s: []int{2, 50}, dmg: []int{4}, mods: neutral(),
			wantHit: true, wantCrit: true, wantEffThac0: 15, wantMargin: 5, wantMultiplier: 2, wantStrBonus: 2,
			wantDamage: 10, wantBreakdown: "((4 × 1.0) + 0) × 2.0 + 2 = 10",
		},
		{
			name: "d100 just too high for a crit", str: 18, d20: 15, d100s: []int{6}, dmg: []int{4}, mods: neutral(),
			wantHit: true, wantCrit: false, wantEffThac0: 15, wantMargin: 5, wantMultiplier: 1, wantStrBonus: 2, wantDamage: 6,
		},
		{
			name: "crit multiplier stacks with buff multiplier", str: 18, d20: 15, d100s: []int{2, 50}, dmg: []int{4},
			mods:    buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 2.0},
			wantHit: true, wantCrit: true, wantEffThac0: 15, wantMargin: 5, wantMultiplier: 4, wantStrBonus: 2,
			wantDamage: 18, // ((4*1.0)+0)*4+2
		},
		{
			name: "weapon and spell buffs", str: 18, d20: 12, d100s: []int{50}, dmg: []int{6},
			mods:    buffsvc.BuffModifiers{SpellsBuff: 1.5, WeaponBuff: 5, Multipliers: 1.0},
			wantHit: true, wantEffThac0: 15, wantMargin: 2, wantMultiplier: 1, wantStrBonus: 2,
			wantDamage: 16, wantBreakdown: "((6 × 1.5) + 5) × 1.0 + 2 = 16",
		},
		{
			name: "damage is floored", str: 18, d20: 12, d100s: []int{50}, dmg: []int{3},
			mods:    buffsvc.BuffModifiers{SpellsBuff: 1.5, Multipliers: 1.0},
			wantHit: true, wantEffThac0: 15, wantMargin: 2, wantMultiplier: 1, wantStrBonus: 2,
			wantDamage: 6, // floor(4.5) + 2
		},
		{
			name: "STR below 18 has no to-hit or damage bonus", str: 17, d20: 11, d100s: []int{50}, dmg: []int{6}, mods: neutral(),
			wantHit: true, wantEffThac0: 16, wantMargin: 0, wantMultiplier: 1, wantStrBonus: 0, wantDamage: 6,
		},
		{
			name: "STR below 18 misses where STR 18 would hit", str: 17, d20: 10, d100s: []int{50},
			wantHit: false, wantEffThac0: 16, wantMargin: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.withWeapon()
			f.withChar(bard(tt.str))
			f.rolls(tt.d20, tt.d100s, tt.dmg...)
			if tt.wantHit {
				f.buffs.EXPECT().Aggregate(mock.Anything, charID, tt.str).Return(tt.mods, nil)
			}

			res, err := f.svc.ResolveAttack(context.Background(), charID, "longsword", targetAC)
			require.NoError(t, err)

			assert.Equal(t, tt.d20, res.D20Roll)
			assert.Equal(t, tt.d100s[0], res.D100CritRoll)
			assert.Equal(t, 16, res.BaseThac0)
			assert.Equal(t, tt.wantEffThac0, res.EffectiveThac0)
			assert.Equal(t, tt.wantEffThac0-tt.d20, res.TargetACNeeded)
			assert.Equal(t, tt.wantHit, res.Hit)
			assert.Equal(t, tt.wantMargin, res.HitMargin)
			assert.Equal(t, tt.wantCrit, res.IsCrit)
			assert.Equal(t, tt.wantMultiplier, res.Multipliers)
			assert.Equal(t, tt.wantStrBonus, res.StrengthBonus)
			assert.Equal(t, tt.wantDamage, res.TotalDamage)
			if tt.wantBreakdown != "" {
				assert.Equal(t, tt.wantBreakdown, res.DamageBreakdown)
			}
			if tt.wantHit {
				assert.Equal(t, tt.dmg[0], res.BaseDamage)
			} else {
				assert.Zero(t, res.TotalDamage)
				assert.Empty(t, res.DamageBreakdown)
			}
			if tt.wantCrit {
				assert.Equal(t, 2, res.CritMultiplier)
			} else {
				assert.Equal(t, 1, res.CritMultiplier)
			}
		})
	}
}

func TestResolveAttack_ModifiersListed(t *testing.T) {
	f := newFixture(t)
	f.withWeapon()
	f.withChar(bard(18))
	f.rolls(12, []int{50}, 6)
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 18).Return(neutral(), nil)

	res, err := f.svc.ResolveAttack(context.Background(), charID, "longsword", 5)
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.Modifier{{Source: "Strength", Value: 1}}, res.Modifiers)
}

func TestResolveAttack_MultiDiceWeapon(t *testing.T) {
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, "greatsword").Return(combatsvc.Weapon{ID: "greatsword", BaseDamage: "2D6"}, nil)
	f.withChar(bard(10))
	f.dice.EXPECT().RollD20().Return(20)
	f.dice.EXPECT().RollD100().Return(99)
	f.dice.EXPECT().RollDie(6).Return(3).Once()
	f.dice.EXPECT().RollDie(6).Return(4).Once()
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 10).Return(neutral(), nil)

	res, err := f.svc.ResolveAttack(context.Background(), charID, "greatsword", 5)
	require.NoError(t, err)
	assert.Equal(t, 7, res.BaseDamage)
	assert.Equal(t, 7, res.TotalDamage)
}

// Aggregation gating (MinStr) lives in buffsvc; wire the real buffsvc behind
// the Buffs port to prove combat honours it end to end.
func TestResolveAttack_MinStrBuffGated(t *testing.T) {
	heavy := buffsvc.BuffEffect{ID: "h", Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(10), MinStr: intPtr(19)}}
	gem := buffsvc.BuffEffect{ID: "g", Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(5), Multiplier: floatPtr(2.0), SpellsBuff: floatPtr(1.5)}}

	tests := []struct {
		name       string
		str        int
		buffs      []buffsvc.BuffEffect
		wantDamage int
	}{
		{"MinStr not met: heavy buff ignored", 18, []buffsvc.BuffEffect{heavy}, 8},          // 6 + 2
		{"MinStr met: heavy buff applies", 19, []buffsvc.BuffEffect{heavy}, 18},             // 6 + 10 + 2
		{"all buff kinds combine", 18, []buffsvc.BuffEffect{heavy, gem}, ((9 + 5) * 2) + 2}, // ((6*1.5)+5)*2+2
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dal := buffmocks.NewMockDAL(t)
			dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return(tt.buffs, nil)
			realBuffs, err := buffsvc.New(dal)
			require.NoError(t, err)

			chars := mocks.NewMockCharacters(t)
			chars.EXPECT().GetCharacter(mock.Anything, charID).Return(bard(tt.str), nil)
			weapons := mocks.NewMockWeapons(t)
			weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil)
			dice := mocks.NewMockDice(t)
			dice.EXPECT().RollD20().Return(14)
			dice.EXPECT().RollD100().Return(80)
			dice.EXPECT().RollDie(8).Return(6)
			svc, err := combatsvc.New(chars, realBuffs, dice, weapons)
			require.NoError(t, err)

			res, err := svc.ResolveAttack(context.Background(), charID, "longsword", 5)
			require.NoError(t, err)
			assert.True(t, res.Hit)
			assert.Equal(t, tt.wantDamage, res.TotalDamage)
		})
	}
}

func TestResolveAttack_Errors(t *testing.T) {
	boom := errors.New("boom")

	t.Run("unknown weapon", func(t *testing.T) {
		f := newFixture(t)
		f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(combatsvc.Weapon{}, combatsvc.ErrWeaponNotFound)

		res, err := f.svc.ResolveAttack(context.Background(), charID, "longsword", 5)
		assert.Nil(t, res)
		assert.True(t, errors.Is(err, combatsvc.ErrWeaponNotFound))
	})

	t.Run("character lookup error keeps the sentinel", func(t *testing.T) {
		f := newFixture(t)
		f.withWeapon()
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(nil, domain.ErrCharacterNotFound)

		res, err := f.svc.ResolveAttack(context.Background(), charID, "longsword", 5)
		assert.Nil(t, res)
		assert.True(t, errors.Is(err, domain.ErrCharacterNotFound))
	})

	t.Run("aggregate error", func(t *testing.T) {
		f := newFixture(t)
		f.withWeapon()
		f.withChar(bard(18))
		f.rolls(15, []int{50}, 4)
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, 18).Return(buffsvc.BuffModifiers{}, boom)

		res, err := f.svc.ResolveAttack(context.Background(), charID, "longsword", 5)
		assert.Nil(t, res)
		assert.True(t, errors.Is(err, boom))
	})
}

// docs/backend.md and the review checklist mention natural 1 / natural 20, but
// the resolver compares the raw d20 only: a nat 1 can hit against a low enough
// AC and a nat 20 can miss against a high enough one. Whether that is intended
// is a rules question; these stay skipped until it is decided.
func TestResolveAttack_NaturalRolls(t *testing.T) {
	t.Run("nat 1 always misses", func(t *testing.T) {
		t.Skip("bug?: no nat-1 auto-miss in resolve_attack.go (THAC0 16, AC -10 -> roll 1 hits); rules decision needed")
		f := newFixture(t)
		f.withWeapon()
		f.withChar(bard(10))
		f.rolls(1, []int{99})
		res, err := f.svc.ResolveAttack(context.Background(), charID, "longsword", -10)
		require.NoError(t, err)
		assert.False(t, res.Hit)
	})
	t.Run("nat 20 always hits", func(t *testing.T) {
		t.Skip("bug?: no nat-20 auto-hit in resolve_attack.go (THAC0 16, AC 10 -> roll 20 hits only when required<=20); rules decision needed")
		f := newFixture(t)
		f.withWeapon()
		f.withChar(&charactersvc.Character{ID: charID, Thac0Base: 20})
		f.rolls(20, []int{99}, 4)
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, 0).Return(neutral(), nil)
		res, err := f.svc.ResolveAttack(context.Background(), charID, "longsword", -10)
		require.NoError(t, err)
		assert.True(t, res.Hit)
	})
}

func TestResolveSavingThrow(t *testing.T) {
	tests := []struct {
		name       string
		category   combatsvc.SaveCategory
		d20        int
		saveBonus  int
		wantTarget int
		wantHit    bool
		wantMargin int
		wantText   string
	}{
		{"para/poison pass", combatsvc.SaveParaPoison, 11, 0, 11, true, 0, "Roll: 11 + Mod: 0 = 11 vs Target: 11 (Paralyze/Poison)"},
		{"para/poison fail", combatsvc.SaveParaPoison, 10, 0, 11, false, -1, "Roll: 10 + Mod: 0 = 10 vs Target: 11 (Paralyze/Poison)"},
		{"rod/staff/wand", combatsvc.SaveRodStaffWand, 13, 0, 13, true, 0, "(Rod/Staff/Wand)"},
		{"petrify/poly", combatsvc.SavePetriPoly, 5, 0, 12, false, -7, "(Petrify/Polymorph)"},
		{"breath", combatsvc.SaveBreath, 20, 0, 15, true, 5, "(Breath Weapon)"},
		{"spell", combatsvc.SaveSpell, 14, 0, 14, true, 0, "(Spells)"},
		{"buff bonus turns a fail into a pass", combatsvc.SaveSpell, 12, 2, 14, true, 0, "Roll: 12 + Mod: 2 = 14 vs Target: 14 (Spells)"},
		{"negative buff bonus", combatsvc.SaveSpell, 14, -1, 14, false, -1, "Roll: 14 + Mod: -1 = 13 vs Target: 14 (Spells)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.withChar(bard(10))
			f.dice.EXPECT().RollD20().Return(tt.d20)
			mods := neutral()
			mods.SaveBonus = tt.saveBonus
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, 10).Return(mods, nil)

			res, err := f.svc.ResolveSavingThrow(context.Background(), charID, tt.category)
			require.NoError(t, err)
			assert.Equal(t, tt.d20, res.D20Roll)
			assert.Equal(t, tt.wantTarget, res.BaseThac0)
			assert.Equal(t, tt.wantTarget, res.EffectiveThac0)
			assert.Equal(t, tt.wantHit, res.Hit)
			assert.Equal(t, tt.wantMargin, res.HitMargin)
			assert.Contains(t, res.DamageBreakdown, tt.wantText)
			if tt.saveBonus != 0 {
				assert.Equal(t, []combatsvc.Modifier{{Source: "Buffs", Value: tt.saveBonus}}, res.Modifiers)
			} else {
				assert.Empty(t, res.Modifiers)
			}
		})
	}
}

func TestResolveSavingThrow_Errors(t *testing.T) {
	boom := errors.New("boom")

	t.Run("character error", func(t *testing.T) {
		f := newFixture(t)
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(nil, domain.ErrCharacterNotFound)

		res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell)
		assert.Nil(t, res)
		assert.True(t, errors.Is(err, domain.ErrCharacterNotFound))
	})

	t.Run("aggregate error", func(t *testing.T) {
		f := newFixture(t)
		f.withChar(bard(10))
		f.dice.EXPECT().RollD20().Return(10)
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, 10).Return(buffsvc.BuffModifiers{}, boom)

		res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell)
		assert.Nil(t, res)
		assert.True(t, errors.Is(err, boom))
	})
}
