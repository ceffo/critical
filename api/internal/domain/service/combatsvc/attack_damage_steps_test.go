package combatsvc_test

// Step tests of docs/adr/0002-attack-resolution.md and docs/adr/0003-damage.md
// that no other test already proves (the ADRs name the test of every step).
// Every die returns a distinct value and is expected an exact number of
// times, so a roll reused, skipped or taken out of order fails.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// seqDice is a Dice that hands out scripted values and records the order of
// the calls ("d20", "d100", "d8"...). It fails the test on a call it was not
// scripted for.
type seqDice struct {
	t     *testing.T
	want  []string
	vals  []int
	calls []string
}

func (d *seqDice) next(kind string) int {
	d.t.Helper()
	i := len(d.calls)
	d.calls = append(d.calls, kind)
	if i >= len(d.want) {
		d.t.Fatalf("unexpected roll %d (%s): only %d scripted", i+1, kind, len(d.want))
	}
	if d.want[i] != kind {
		d.t.Fatalf("roll %d is %s, want %s (calls so far %v)", i+1, kind, d.want[i], d.calls)
	}
	return d.vals[i]
}

func (d *seqDice) RollD20() int          { return d.next("d20") }
func (d *seqDice) RollD100() int         { return d.next("d100") }
func (d *seqDice) RollDie(sides int) int { return d.next(fmt.Sprintf("d%d", sides)) }

// ADR 0002 step 1: every attack rolls the d20, the crit-check d100, the
// multiplier d100 and then the damage dice for the target size, in that
// order, whether it hits or misses.
func TestAttack_Step1_DiceInOrderOnEveryAttack(t *testing.T) {
	cases := []struct {
		name    string
		ac      int
		size    combatsvc.TargetSize
		want    []string
		vals    []int
		wantHit bool
	}{
		// Bastard sword at STR 12: effective THAC0 11. AC 5 needs 6.
		{"hit, small/medium 2d4", 5, combatsvc.SizeSmallMedium, []string{"d20", "d100", "d100", "d4", "d4"}, []int{14, 61, 37, 3, 2}, true},
		// AC -10 needs 21: a miss still rolls its damage dice.
		{"miss, large 2d8", -10, combatsvc.SizeLarge, []string{"d20", "d100", "d100", "d8", "d8"}, []int{19, 64, 28, 7, 4}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			dice := &seqDice{t: t, want: c.want, vals: c.vals}
			svc, err := combatsvc.New(combatsvc.Deps{
				Chars: f.chars, Buffs: f.buffs, Dice: dice, Weapons: f.weapons, Crit: f.crit,
				Items: f.items, Forms: f.forms, Rules: phb(t), Log: f.log, Tx: noTx{},
			})
			require.NoError(t, err)
			f.weapons.EXPECT().Get(mock.Anything, charID, "bastard_sword").Return(bastardSword, nil).Once()
			f.withChar(bard())
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(strMods(12), nil).Once()
			f.records()

			res, err := svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{
				WeaponID: "bastard_sword", TargetAC: new(c.ac), TargetSize: c.size,
			})
			require.NoError(t, err)
			assert.Equal(t, c.want, dice.calls, "every die once, in order")
			assert.Equal(t, c.wantHit, res.Hit)
			assert.Equal(t, c.vals[0], res.D20Roll)
			assert.Equal(t, c.vals[1], res.D100CritRoll)
			assert.Equal(t, c.vals[2], res.D100MultiplierRoll)
			require.NotNil(t, res.Rolled)
			assert.Equal(t, c.vals[3]+c.vals[4], res.Rolled.Total)
		})
	}
}

// ADR 0002 step 8: each buff's to-hit effect is its own modifier, labelled
// with the buff, and each counts once in the effective THAC0.
func TestAttack_Step8_OneToHitModifierPerBuff(t *testing.T) {
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, charID, "longsword").Return(longsword, nil).Once()
	f.withChar(bard())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(buffsvc.BuffModifiers{
		SpellsBuff: 1, Multipliers: 1, EffectiveStr: 12, ToHit: -2,
		ToHitFrom: []buffsvc.Contribution{{Buff: "Bless", Value: 1}, {Buff: "Prayer", Value: 1}, {Buff: "Chill", Value: -4}},
	}, nil).Once()
	f.dice.EXPECT().RollD20().Return(13).Once()
	f.d100s(t, 71, 24)
	f.dice.EXPECT().RollDie(8).Return(6).Once()
	f.records()

	res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{WeaponID: "longsword", TargetAC: new(5)})
	require.NoError(t, err)
	assert.Equal(t, []combatsvc.Modifier{
		modStr(12, 0), modWeapon(longsword), modBuff("Bless", 1), modBuff("Prayer", 1), modBuff("Chill", -4),
	}, res.Modifiers)
	// 13 - (0 + 2 + 1 + 1 - 4) = 13: AC 5 needs 8, d20 13 hits by 5.
	assert.Equal(t, 13, res.EffectiveThac0)
	assert.True(t, res.Hit)
	assert.Equal(t, 5, res.HitMargin)
}

// ADR 0003 step 1: the damage dice follow the target size: the weapon's
// large dice against a large target, its base dice otherwise, and its base
// dice against a large target when it has no large dice.
func TestDamage_Step1_DiceByTargetSize(t *testing.T) {
	noLarge := combatsvc.Weapon{Wielded: true, ID: "club", Name: localized.Of("Club"), BaseDamage: "1d6", Kind: combatsvc.WeaponMelee}
	cases := []struct {
		name      string
		weapon    combatsvc.Weapon
		size      combatsvc.TargetSize
		sides     int
		dmg       []int
		wantDice  string
		wantTotal int
	}{
		// Bastard sword +2 at STR 12: ((dice × 1) + 2) × 1 + 0.
		{"small/medium: base dice", bastardSword, combatsvc.SizeSmallMedium, 4, []int{3, 1}, "2d4", 6},
		{"large: large dice", bastardSword, combatsvc.SizeLarge, 8, []int{7, 5}, "2d8", 14},
		// Club, no magic: ((4 × 1) + 0) × 1 + 0.
		{"large without large dice: base dice", noLarge, combatsvc.SizeLarge, 6, []int{4}, "1d6", 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.weapons.EXPECT().Get(mock.Anything, charID, c.weapon.ID).Return(c.weapon, nil).Once()
			f.withChar(bard())
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(strMods(12), nil).Once()
			f.dice.EXPECT().RollD20().Return(17).Once()
			f.d100s(t, 92, 38)
			for _, d := range c.dmg {
				f.dice.EXPECT().RollDie(c.sides).Return(d).Once()
			}
			f.records()

			res, err := f.svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{
				WeaponID: c.weapon.ID, TargetAC: new(5), TargetSize: c.size,
			})
			require.NoError(t, err)
			require.True(t, res.Hit)
			assert.Equal(t, c.wantDice, res.DamageDice)
			assert.Equal(t, c.wantTotal, res.TotalDamage)
		})
	}
}

// The log payload keeps the damage dice as thrown (value per die), so an older
// attack shows them after a reload; a miss deals none, and an AC correction
// keeps the same dice without rolling again.
func TestAttack_LogPayloadHoldsTheDamageDice(t *testing.T) {
	cases := []struct {
		name string
		ac   int
		want []any
	}{
		{"hit", 5, []any{map[string]any{"sides": float64(4), "value": float64(3)}, map[string]any{"sides": float64(4), "value": float64(2)}}},
		{"miss", -10, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			dice := &seqDice{t: t, want: []string{"d20", "d100", "d100", "d4", "d4"}, vals: []int{14, 61, 37, 3, 2}}
			svc, err := combatsvc.New(combatsvc.Deps{
				Chars: f.chars, Buffs: f.buffs, Dice: dice, Weapons: f.weapons, Crit: f.crit,
				Items: f.items, Forms: f.forms, Rules: phb(t), Log: f.log, Tx: noTx{},
			})
			require.NoError(t, err)
			f.weapons.EXPECT().Get(mock.Anything, charID, "bastard_sword").Return(bastardSword, nil).Once()
			f.withChar(bard())
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(strMods(12), nil).Once()
			f.records()

			res, err := svc.ResolveAttack(context.Background(), charID, combatsvc.AttackRequest{
				WeaponID: "bastard_sword", TargetAC: new(c.ac), TargetSize: combatsvc.SizeSmallMedium,
			})
			require.NoError(t, err)
			p := payloadOf(t, f.entries[0].Payload)
			if c.want == nil {
				assert.NotContains(t, p, "damageRolls")
				assert.Empty(t, res.DamageRolls)
			} else {
				assert.Equal(t, c.want, p["damageRolls"])
				assert.Equal(t, []combatsvc.DieRoll{{Sides: 4, Value: 3}, {Sides: 4, Value: 2}}, res.DamageRolls)
			}
			assert.Equal(t, []any{
				map[string]any{"sides": float64(4), "value": float64(3)}, map[string]any{"sides": float64(4), "value": float64(2)},
			}, p["rolled"].(map[string]any)["rolls"], "rolled damage keeps its dice even on a miss")
		})
	}
}

// A miss corrected to a hit (AC, then verdict) reads the damage dice rolled
// with the attack: same values, no die drawn (the mocks expect each once).
func TestAttack_CorrectionKeepsTheRolledDamageDice(t *testing.T) {
	dice := []combatsvc.DieRoll{{Sides: 8, Value: 5}}
	wantRolls := []any{map[string]any{"sides": float64(8), "value": float64(5)}}

	t.Run("AC corrected from a miss to a hit", func(t *testing.T) {
		f := newFixture(t)
		// Long sword at STR 12: effective THAC0 11; AC 5 needs 6, d20 3 misses; AC 9 needs 2 and hits.
		e := rollAttack(t, f, sword(5, 3, 50))
		assert.NotContains(t, payloadOf(t, e.Payload), "damageRolls", "a miss shows no damage dice")
		amends(f, e)

		res, err := f.svc.AdjustAttackAC(context.Background(), charID, 42, 9)
		require.NoError(t, err)
		require.True(t, res.Hit)
		assert.Equal(t, dice, res.DamageRolls)
		assert.Equal(t, wantRolls, payloadOf(t, e.EffectivePayload())["damageRolls"])
	})

	t.Run("verdict on an unknown AC", func(t *testing.T) {
		f := newFixture(t)
		e := rollAttack(t, f, unknownSword(3, 50, 64))
		amends(f, e)

		res, err := f.svc.SetAttackVerdict(context.Background(), charID, 42, true)
		require.NoError(t, err)
		require.True(t, res.Hit)
		assert.Equal(t, dice, res.DamageRolls)
		assert.Equal(t, wantRolls, payloadOf(t, e.EffectivePayload())["damageRolls"])
		res, err = f.svc.SetAttackVerdict(context.Background(), charID, 42, false)
		require.NoError(t, err)
		assert.Empty(t, res.DamageRolls, "a miss verdict hides the dice again")
	})

	t.Run("entry logged before the dice were recorded", func(t *testing.T) {
		f := newFixture(t)
		// Rolled damage without per-die values: a correction decodes, recomputes
		// from the total and records no dice.
		e := &actionlog.Entry{ID: 42, Kind: actionlog.KindAttack, Summary: "x", Payload: json.RawMessage(
			`{"d20Roll":3,"d100CritRoll":50,"d100MultiplierRoll":64,"hit":false,"effectiveThac0":11,"targetAc":5,` +
				`"weaponName":"Long Sword +2","modifiers":[],"damageTerms":[],` +
				`"rolled":{"dice":"1d8","total":5,"spellsBuff":1,"weaponBuff":2,"buffMultipliers":1,"strengthBonus":0,"bonusDamage":0,"terms":[]},` +
				`"critRow":{"from":1,"to":100,"multiplier":1.5,"placeholder":false}}`)}
		amends(f, e)
		res, err := f.svc.AdjustAttackAC(context.Background(), charID, 42, 9)
		require.NoError(t, err)
		require.True(t, res.Hit)
		assert.Equal(t, 5, res.BaseDamage)
		assert.Empty(t, res.DamageRolls)
		assert.NotContains(t, payloadOf(t, e.EffectivePayload()), "damageRolls")
	})
}
