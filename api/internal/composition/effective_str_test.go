package composition_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roll is one expected dice call: which die and the value it returns.
type roll struct {
	die   string // "d20", "d100" or "dN" for RollDie(N)
	value int
}

// scriptedDice returns the scripted values in order and fails the test on
// any call that is not the next one expected; at cleanup every scripted roll
// must have been used (exact call count).
type scriptedDice struct {
	t     *testing.T
	queue []roll
}

func newScriptedDice(t *testing.T) *scriptedDice {
	d := &scriptedDice{t: t}
	t.Cleanup(func() {
		assert.Empty(t, d.queue, "scripted rolls left unused")
	})
	return d
}

func (d *scriptedDice) expect(rs ...roll) { d.queue = append(d.queue, rs...) }

func (d *scriptedDice) next(die string) int {
	d.t.Helper()
	if len(d.queue) == 0 {
		d.t.Fatalf("unexpected %s roll: no roll scripted", die)
	}
	r := d.queue[0]
	d.queue = d.queue[1:]
	if r.die != die {
		d.t.Fatalf("rolled %s, script expected %s", die, r.die)
	}
	return r.value
}

func (d *scriptedDice) RollDie(sides int) int { return d.next(fmt.Sprintf("d%d", sides)) }
func (d *scriptedDice) RollD20() int          { return d.next("d20") }
func (d *scriptedDice) RollD100() int         { return d.next("d100") }

// Effective STR rules (docs/backend.md "Attack Modifiers", critical-qki.33)
// on the real stack, from received spells to the attack: two Strength casts
// add up to 18 and the Power Gem (min_str 18) gives +5 and x2; Improved
// Strength then replaces them (19, not 18 + 7) and the gem still applies
// (STR >= 18).
func TestEffectiveStr_StrengthStacksThenImprovedReplacesIt(t *testing.T) {
	dice := newScriptedDice(t)
	ctx := context.Background()
	bs, err := composition.New(ctx, composition.Config{
		DSN:      filepath.Join(t.TempDir(), "critical.db"),
		SeedsDir: filepath.Join("..", "..", "testdata", "seeds"),
		Dice:     dice,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bs.Close(ctx) })
	id := bs.CharacterID
	_, err = bs.DB.ExecContext(ctx, `UPDATE characters SET str = 12 WHERE id = ?`, id)
	require.NoError(t, err)

	_, err = bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{
		Name:    "Power Gem",
		Effects: buffsvc.BuffEffects{MinStr: intPtr(18), WeaponBuff: intPtr(5), Multiplier: floatPtr(2)},
	})
	require.NoError(t, err)
	receive := func(spell string) {
		t.Helper()
		_, err := bs.Cast.Receive(ctx, id, castsvc.ReceiveRequest{SpellID: spell, CasterLevel: 9})
		require.NoError(t, err)
	}
	effective := func() int {
		t.Helper()
		s, err := bs.Buffs.GetEffectiveStr(ctx, id)
		require.NoError(t, err)
		return s.Effective
	}
	attack := combatsvc.AttackRequest{WeaponID: "mace", TargetAC: 5}

	// Rule 1: Strength 1d6 = 2, then 1d6 = 4: 12 + 2 + 4 = 18.
	dice.expect(roll{"d6", 2})
	receive("strength")
	assert.Equal(t, 14, effective())
	dice.expect(roll{"d6", 4})
	receive("strength")
	assert.Equal(t, 18, effective())

	// Rule 3 at 18: the gem applies.
	dice.expect(roll{"d20", 11}, roll{"d100", 37}, roll{"d100", 58}, roll{"d6", 3})
	at18, err := bs.Combat.ResolveAttack(ctx, id, attack)
	require.NoError(t, err)
	assert.Equal(t, 2, at18.StrengthBonus, "PHB Table 1 damage at STR 18")
	assert.Equal(t, 5, at18.WeaponBuff, "Power Gem: the weapon becomes +5")
	assert.InDelta(t, 2.0, at18.Multipliers, 1e-9)
	assert.Equal(t, 3, at18.Rolled.Total)

	// Rule 2: Improved Strength 1d6+4 = 3+4 replaces the two Strength casts:
	// 12 + 7 = 19 (not 18 + anything).
	dice.expect(roll{"d6", 3})
	receive("improved_strength")
	assert.Equal(t, 19, effective())

	// Rule 3 at 19: the gem still applies (>= 18).
	dice.expect(roll{"d20", 13}, roll{"d100", 42}, roll{"d100", 71}, roll{"d6", 5})
	at19, err := bs.Combat.ResolveAttack(ctx, id, attack)
	require.NoError(t, err)
	assert.Equal(t, 7, at19.StrengthBonus, "PHB Table 1 damage at STR 19")
	assert.Equal(t, 5, at19.WeaponBuff, "Power Gem at STR 19")
	assert.InDelta(t, 2.0, at19.Multipliers, 1e-9)
	assert.InDelta(t, 2.0, at19.Rolled.BuffMultipliers, 1e-9)
	assert.Equal(t, 5, at19.Rolled.Total)
	assert.Equal(t, 13, at19.D20Roll)
}
