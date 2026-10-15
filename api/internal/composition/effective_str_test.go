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

// strStack is the real stack on STR 12 with scripted dice and a Power Gem
// (min_str 18) active.
type strStack struct {
	t    *testing.T
	ctx  context.Context
	bs   *composition.BackingServices
	id   int64
	dice *scriptedDice
}

func newStrStack(t *testing.T) *strStack {
	t.Helper()
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
	return &strStack{t: t, ctx: ctx, bs: bs, id: id, dice: dice}
}

// receive applies spell from a party caster of level, rolling the given STR
// gain die (exactly one d6).
func (s *strStack) receive(spell string, level, d6 int) castsvc.CastResult {
	s.t.Helper()
	s.dice.expect(roll{"d6", d6})
	res, err := s.bs.Cast.Receive(s.ctx, s.id, castsvc.ReceiveRequest{SpellID: spell, CasterLevel: level})
	require.NoError(s.t, err)
	require.NotNil(s.t, res.Buff)
	return res
}

func (s *strStack) effective() int {
	s.t.Helper()
	str, err := s.bs.Buffs.GetEffectiveStr(s.ctx, s.id)
	require.NoError(s.t, err)
	return str.Effective
}

// applies maps each active buff's id to its applies flag, and lists the names.
func (s *strStack) status() (names []string, applies map[string]bool) {
	s.t.Helper()
	list, err := s.bs.Buffs.ListStatus(s.ctx, s.id)
	require.NoError(s.t, err)
	applies = map[string]bool{}
	for _, b := range list {
		names = append(names, b.Name)
		applies[b.ID] = b.Applies
	}
	return names, applies
}

func (s *strStack) gemOn() bool {
	s.t.Helper()
	mods, err := s.bs.Buffs.Aggregate(s.ctx, s.id, 12)
	require.NoError(s.t, err)
	return mods.WeaponBuff == 5 && mods.Multipliers == 2
}

// ADR 0004 steps 7, 8 and 10 on the real stack, from received spells to the attack: two Strength casts
// add up to 18 and the Power Gem (min_str 18) gives +5 and x2; Improved
// Strength then replaces them (19, not 18 + 7) and the gem still applies
// (STR >= 18).
func TestEffectiveStr_StrengthStacksThenImprovedReplacesIt(t *testing.T) {
	s := newStrStack(t)
	attack := combatsvc.AttackRequest{WeaponID: "mace", TargetAC: 5}

	// Step 7: Strength 1d6 = 2, then 1d6 = 4: 12 + 2 + 4 = 18.
	s.receive("strength", 9, 2)
	assert.Equal(t, 14, s.effective())
	s.receive("strength", 9, 4)
	assert.Equal(t, 18, s.effective())

	// Step 10 at 18: the gem applies.
	s.dice.expect(roll{"d20", 11}, roll{"d100", 37}, roll{"d100", 58}, roll{"d6", 3})
	at18, err := s.bs.Combat.ResolveAttack(s.ctx, s.id, attack)
	require.NoError(t, err)
	assert.Equal(t, 2, at18.StrengthBonus, "PHB Table 1 damage at STR 18")
	assert.Equal(t, 5, at18.WeaponBuff, "Power Gem: the weapon becomes +5")
	assert.InDelta(t, 2.0, at18.DiceMultiplier, 1e-9)
	assert.Equal(t, 3, at18.Rolled.Total)

	// Step 8: Improved Strength 1d6+4 = 3+4 replaces the two Strength casts:
	// 12 + 7 = 19 (not 18 + anything).
	s.receive("improved_strength", 9, 3)
	assert.Equal(t, 19, s.effective())

	// Step 10 at 19: the gem still applies (>= 18).
	s.dice.expect(roll{"d20", 13}, roll{"d100", 42}, roll{"d100", 71}, roll{"d6", 5})
	at19, err := s.bs.Combat.ResolveAttack(s.ctx, s.id, attack)
	require.NoError(t, err)
	assert.Equal(t, 7, at19.StrengthBonus, "PHB Table 1 damage at STR 19")
	assert.Equal(t, 5, at19.WeaponBuff, "Power Gem at STR 19")
	assert.InDelta(t, 2.0, at19.DiceMultiplier, 1e-9)
	assert.InDelta(t, 2.0, at19.Rolled.BuffMultipliers, 1e-9)
	assert.Equal(t, 5, at19.Rolled.Total)
	assert.Equal(t, 13, at19.D20Roll)
}

// Checklist 2: two Strengths reach 18 and the gem is on; voiding the second
// receive leaves one Strength, STR drops, the gem goes off.
func TestEffectiveStr_VoidSecondStrength(t *testing.T) {
	s := newStrStack(t)
	s.receive("strength", 9, 1)
	second := s.receive("strength", 9, 5)
	names, _ := s.status()
	assert.Equal(t, []string{"Power Gem", "Strength", "Strength"}, names)
	assert.Equal(t, 18, s.effective())
	assert.True(t, s.gemOn())

	_, err := s.bs.Log.Void(s.ctx, s.id, second.EntryID)
	require.NoError(t, err)
	names, _ = s.status()
	assert.Equal(t, []string{"Power Gem", "Strength"}, names)
	assert.Equal(t, 13, s.effective(), "12 + the first cast's 1")
	assert.False(t, s.gemOn())
}

// Checklist 3 and nit: two Strengths (18), then Improved rolls low (1+4 =
// 17): Improved replaces them, the gem goes OFF, and the Strength badges
// show as not applying.
func TestEffectiveStr_LowImprovedTurnsTheGemOff(t *testing.T) {
	s := newStrStack(t)
	a := s.receive("strength", 9, 3)
	b := s.receive("strength", 9, 6)
	require.Equal(t, 18, s.effective())
	require.True(t, s.gemOn())

	imp := s.receive("improved_strength", 9, 1)
	assert.Equal(t, 17, s.effective(), "12 + 1 + 4, the Strength casts no longer count")
	assert.False(t, s.gemOn())
	_, applies := s.status()
	assert.False(t, applies[a.Buff.Buff.ID])
	assert.False(t, applies[b.Buff.Buff.ID])
	assert.True(t, applies[imp.Buff.Buff.ID])
}

// Checklist 4: Improved expires while two Strengths remain: STR goes back
// to their sum and the gem comes back on.
func TestEffectiveStr_ImprovedExpiresStrengthsCountAgain(t *testing.T) {
	s := newStrStack(t)
	s.receive("strength", 9, 2)
	s.receive("strength", 9, 4)
	imp := s.receive("improved_strength", 1, 1) // 1 round at caster level 1
	require.Equal(t, 1, imp.Buff.Buff.Duration)
	require.Equal(t, 17, s.effective())
	require.False(t, s.gemOn())

	s.dice.expect(roll{"d10", 4}) // Next Round's initiative
	_, expired, _, _, err := s.bs.Buffs.AdvanceRound(s.ctx, s.id)
	require.NoError(t, err)
	assert.Equal(t, []string{"Improved Strength"}, names(expired))
	assert.Equal(t, 18, s.effective())
	assert.True(t, s.gemOn())
}

// Checklist 5: dismissing one of two Strengths removes only that one, and
// the dismiss is logged.
func TestEffectiveStr_DismissOneStrength(t *testing.T) {
	s := newStrStack(t)
	a := s.receive("strength", 9, 2)
	b := s.receive("strength", 9, 4)

	left, err := s.bs.Buffs.Dismiss(s.ctx, s.id, a.Buff.Buff.ID)
	require.NoError(t, err)
	require.Len(t, left, 2)
	assert.Equal(t, b.Buff.Buff.ID, left[1].ID)
	assert.Equal(t, 16, s.effective())

	entries, err := s.bs.Log.List(s.ctx, s.id, 1)
	require.NoError(t, err)
	assert.JSONEq(t, `{"action":"dismiss","buff":"Strength"}`, string(entries[0].Payload))
}

// Checklist 6: two Strengths with different rounds left expire independently
// on Next Round.
func TestEffectiveStr_StrengthsExpireIndependently(t *testing.T) {
	s := newStrStack(t)
	a := s.receive("strength", 9, 2)
	b := s.receive("strength", 9, 4)
	_, err := s.bs.Buffs.Edit(s.ctx, s.id, a.Buff.Buff.ID, buffsvc.BuffEdit{Duration: intPtr(1)})
	require.NoError(t, err)
	_, err = s.bs.Buffs.Edit(s.ctx, s.id, b.Buff.Buff.ID, buffsvc.BuffEdit{Duration: intPtr(2)})
	require.NoError(t, err)

	s.dice.expect(roll{"d10", 4}, roll{"d10", 7}) // each Next Round's initiative
	active, expired, _, _, err := s.bs.Buffs.AdvanceRound(s.ctx, s.id)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, a.Buff.Buff.ID, expired[0].ID)
	assert.Equal(t, []string{"Power Gem", "Strength"}, names(active))
	assert.Equal(t, 16, s.effective())

	_, expired, _, _, err = s.bs.Buffs.AdvanceRound(s.ctx, s.id)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, b.Buff.Buff.ID, expired[0].ID)
	assert.Equal(t, 12, s.effective())
}

// Checklist 7: a hand-made Strength +4 with no max plus a received Strength
// +3 on STR 12 reach 18 (capped) and the gem is on.
func TestEffectiveStr_HandMadeUncappedStacksWithReceived(t *testing.T) {
	s := newStrStack(t)
	_, err := s.bs.Buffs.Activate(s.ctx, s.id, buffsvc.BuffEffect{
		Name: "Strength", Duration: 10, Effects: buffsvc.BuffEffects{StrBonus: intPtr(4)},
	})
	require.NoError(t, err)
	s.receive("strength", 9, 3)
	assert.Equal(t, 18, s.effective(), "12 + 4 + 3 = 19, capped at 18")
	assert.True(t, s.gemOn())
}
