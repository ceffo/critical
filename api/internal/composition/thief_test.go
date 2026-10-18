package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// attackDice serves each die from its own queue and counts the calls.
type attackDice struct {
	t                 *testing.T
	d20s, d100s, dies []int
	calls             map[string]int
}

func (d *attackDice) next(q *[]int, what string) int {
	d.t.Helper()
	require.NotEmpty(d.t, *q, "an unexpected %s", what)
	d.calls[what]++
	v := (*q)[0]
	*q = (*q)[1:]
	return v
}

func (d *attackDice) RollD20() int  { return d.next(&d.d20s, "d20") }
func (d *attackDice) RollD100() int { return d.next(&d.d100s, "d100") }
func (d *attackDice) RollDie(sides int) int {
	if sides == 100 {
		return d.next(&d.d100s, "d100")
	}
	return d.next(&d.dies, "die")
}

// ADR 0030 steps 1, 17 and 19 on the fixture's tables, through the real
// services: a 1st-level halfling thief of Dexterity 17 enters 30 points in
// Open Locks and Pick Pockets and wears no armour; Open Locks is 10 + 30 + 5
// (halfling) + 10 (Dexterity 17) + 0 (no armour) = 55, rolled as a skill
// check the DM's adjustment moves; the backstab is +4 to hit and the d4 x2
// (Table 30 at 1st): (3 x 2) + 1 (Strength 16) = 7.
func TestThief_Steps17_19_CreateRollASkillAndBackstab(t *testing.T) {
	ctx := context.Background()
	dice := &attackDice{t: t, d20s: []int{14}, d100s: []int{60, 95, 77}, dies: []int{3}, calls: map[string]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Name: "Vex", Class: "thief", Race: "halfling", Str: 16, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
		SkillArmor: "none", SkillPoints: map[string]int{"open_locks": 30, "pick_pockets": 30},
	})
	require.NoError(t, err)

	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 2, c.Derived.Backstab, "Table 30 at 1st")
	assert.Empty(t, c.Derived.SkillBudget, "60 points, 30 in each: PHB p.39's budget at 1st")
	skills, err := bs.Skills.List(ctx, id)
	require.NoError(t, err)
	require.Len(t, skills.ThiefSkills, 8)
	assert.Equal(t, "open_locks", skills.ThiefSkills[1].Skill)
	assert.Equal(t, 55, skills.ThiefSkills[1].Percent)

	check, err := bs.Skills.Roll(ctx, id, "open_locks", skillsvc.Adjustment{Value: 10, Reason: "old lock"})
	require.NoError(t, err)
	assert.Equal(t, skillsvc.KindThiefSkill, check.Kind)
	assert.Equal(t, 65, check.AdjustedTarget)
	assert.True(t, check.Success, "60 at or under 55 + 10")

	_, err = bs.Weapons.Create(ctx, id, weaponsvc.WeaponInput{
		Name: localized.Of("Dagger"), DamageSM: "1d4", DamageL: "1d3", Kind: "melee", Type: "dagger",
	})
	require.NoError(t, err)
	res, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{
		WeaponID: "dagger", TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium, Backstab: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 16, res.EffectiveThac0, "THAC0 20, +4 backstab, +0 Strength 16")
	assert.True(t, res.Hit)
	assert.Equal(t, 7, res.TotalDamage)
	assert.Equal(t, map[string]int{"d20": 1, "d100": 3, "die": 1}, dice.calls)
	entries, err := bs.Log.List(ctx, id, 3)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, actionlog.KindAttack, entries[0].Kind)
	assert.Equal(t, actionlog.KindSkillCheck, entries[2].Kind, "after it the dagger's weapon_add")
}
