package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
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

// ADR 0030 step 17 after creation, through the real services: the thief of
// the test above reaches 2nd level (1,250 XP, Table 25) and its level-up is
// rolled, so PHB p.39's budget grows by 30 points, 15 at most in one skill.
// The player spends them, 15 in Open Locks and 15 in Pick Pockets: Open
// Locks rises from 55 to 70 with no budget warning, the spend is one
// skill_points entry, and undoing it gives the points back.
func TestThief_Step17_SpendPointsAfterALevelUpAndUndo(t *testing.T) {
	ctx := context.Background()
	cfg := fixtureConfig()
	cfg.Dice = &attackDice{t: t, dies: []int{4}, calls: map[string]int{}}
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
	openLocks := func() int {
		t.Helper()
		skills, err := bs.Skills.List(ctx, id)
		require.NoError(t, err)
		require.Equal(t, "open_locks", skills.ThiefSkills[1].Skill)
		return skills.ThiefSkills[1].Percent
	}
	assert.Equal(t, 55, openLocks())

	_, err = bs.XP.Add(ctx, id, xpsvc.EntryInput{Base: 1_250, Reason: "the vault"})
	require.NoError(t, err)
	_, err = bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Level: 2})
	require.NoError(t, err)
	spend := map[string]int{"open_locks": 45, "pick_pockets": 45}
	got, err := bs.Skills.SetPoints(ctx, id, spend)
	require.NoError(t, err)
	assert.Equal(t, spend, got)
	assert.Equal(t, 70, openLocks())
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 2, c.Derived.RealizedLevel)
	assert.Empty(t, c.Derived.SkillBudget, "90 points, 45 in each: the budget at 2nd")

	top, ok, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	e, err := bs.Log.Undo(ctx, id, top)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindSkillPoints, e.Kind)
	assert.Equal(t, 55, openLocks(), "the undo gives the points back")
	c, err = bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"open_locks": 30, "pick_pockets": 30}, c.SkillPoints)
	assert.Equal(t, 2, c.Derived.RealizedLevel, "the level-up stays")
}

// ADR 0030 step 17 on the PHB tables of the fixture, through the real
// services: a halfling thief of Dexterity 18 in no armour with 30 points in
// Open Locks and Pick Pockets has Pick Pockets 15 + 30 + 5 + 10 + 5 = 65 and
// Open Locks 10 + 30 + 5 + 15 = 60, within the 1st-level budget; a 1st-level
// thief with 40 and 30 is created with both warnings. A roll the DM adjusts
// after it changes target and verdict; Read Languages at 0% is refused
// before the die.
func TestThief_Step17_TermsBudgetAdjustmentAndZeroPercent(t *testing.T) {
	ctx := context.Background()
	dice := &attackDice{t: t, d100s: []int{58}, calls: map[string]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	thief := func(points map[string]int) int64 {
		t.Helper()
		id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
			Name: "Vex", Class: "thief", Race: "halfling", Str: 12, Dex: 18, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
			SkillArmor: "none", SkillPoints: points,
		})
		require.NoError(t, err)
		return id
	}

	id := thief(map[string]int{"open_locks": 30, "pick_pockets": 30})
	skills, err := bs.Skills.List(ctx, id)
	require.NoError(t, err)
	percents := map[string]int{}
	for _, sk := range skills.ThiefSkills {
		percents[sk.Skill] = sk.Percent
	}
	assert.Equal(t, map[string]int{
		"pick_pockets": 65, "open_locks": 60, "find_remove_traps": 15, "move_silently": 40,
		"hide_in_shadows": 35, "detect_noise": 20, "climb_walls": 55, "read_languages": 0,
	}, percents)
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, c.Derived.SkillBudget)

	check, err := bs.Skills.Roll(ctx, id, "open_locks", skillsvc.Adjustment{})
	require.NoError(t, err)
	assert.True(t, check.Success, "58 at or under 60")
	adjusted, err := bs.Skills.Adjust(ctx, id, check.LogEntryID, skillsvc.Adjustment{Value: -10, Reason: "rusty"})
	require.NoError(t, err)
	assert.Equal(t, 50, adjusted.AdjustedTarget)
	assert.False(t, adjusted.Success, "58 over 60 - 10")
	_, err = bs.Skills.Roll(ctx, id, "read_languages", skillsvc.Adjustment{Value: 50})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Equal(t, 1, dice.calls["d100"], "no die for a skill at 0%")

	over := thief(map[string]int{"open_locks": 40, "pick_pockets": 30})
	c, err = bs.Character.GetCharacter(ctx, over)
	require.NoError(t, err)
	assert.Equal(t, []ruletablesvc.BudgetWarning{
		{Code: ruletablesvc.BudgetTotal, Points: 70, Limit: 60},
		{Code: ruletablesvc.BudgetPerSkill, Skill: "open_locks", Points: 40, Limit: 30},
	}, c.Derived.SkillBudget)
}

// ADR 0030 step 19 with ADR 0012 step 5: a backstab rolled at an unknown AC
// keeps Table 30's multiplier when the DM gives the AC afterwards: the d4's
// 3 x 2 + 1 (Strength 16) = 7.
func TestThief_Step19_BackstabAtAnUnknownACKeepsItsMultiplier(t *testing.T) {
	ctx := context.Background()
	dice := &attackDice{t: t, d20s: []int{14}, d100s: []int{95, 77}, dies: []int{3}, calls: map[string]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Name: "Vex", Class: "thief", Race: "halfling", Str: 16, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
		SkillArmor: "none",
	})
	require.NoError(t, err)
	_, err = bs.Weapons.Create(ctx, id, weaponsvc.WeaponInput{
		Name: localized.Of("Dagger"), DamageSM: "1d4", DamageL: "1d3", Kind: "melee", Type: "dagger",
	})
	require.NoError(t, err)
	res, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{
		WeaponID: "dagger", TargetSize: combatsvc.SizeSmallMedium, Backstab: true,
	})
	require.NoError(t, err)
	got, err := bs.Combat.AdjustAttackAC(ctx, id, res.LogEntryID, 5)
	require.NoError(t, err)
	assert.True(t, got.Hit, "14 against THAC0 16 at AC 5")
	assert.Equal(t, 7, got.TotalDamage)
	assert.Equal(t, 1, dice.calls["die"], "no die rolled again")
}
