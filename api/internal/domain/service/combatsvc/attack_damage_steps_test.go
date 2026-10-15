package combatsvc_test

// Step tests of docs/adr/0002-attack-resolution.md and docs/adr/0003-damage.md
// that no other test already proves (the ADRs name the test of every step).
// Every die returns a distinct value and is expected an exact number of
// times, so a roll reused, skipped or taken out of order fails.

import (
	"context"
	"fmt"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
			f.weapons.EXPECT().Get(mock.Anything, "bastard_sword").Return(bastardSword, nil).Once()
			f.withChar(bard())
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(strMods(12), nil).Once()
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
	f.weapons.EXPECT().Get(mock.Anything, "longsword").Return(longsword, nil).Once()
	f.withChar(bard())
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(buffsvc.BuffModifiers{
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
	noLarge := combatsvc.Weapon{ID: "club", Name: "Club", BaseDamage: "1d6", Kind: combatsvc.WeaponMelee}
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
			f.weapons.EXPECT().Get(mock.Anything, c.weapon.ID).Return(c.weapon, nil).Once()
			f.withChar(bard())
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(strMods(12), nil).Once()
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
