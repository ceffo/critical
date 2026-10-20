package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
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
		Details: identity(),
		Name:    "Vex", Class: "thief", Race: "halfling", Str: 16, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
		SkillPoints: map[string]int{"open_locks": 30, "pick_pockets": 30},
		// Proficient with the dagger, so no non-proficiency penalty (ADR 0037 step 8).
		Proficiencies: charactersvc.Picks{Weapon: []charactersvc.WeaponPick{{Type: "dagger", Slots: 1}}},
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

	dagger := newWeapon("Dagger", "1d4", "1d3")
	dagger.WeaponType = "dagger"
	_, err = bs.Items.Create(ctx, id, dagger)
	require.NoError(t, err)
	// A new piece is carried (ADR 0033 step 6): the attack needs it wielded.
	_, err = bs.Items.SetState(ctx, id, "dagger", itemsvc.StateWielded, false)
	require.NoError(t, err)
	res, err := bs.Combat.ResolveAttack(ctx, id, combatsvc.AttackRequest{
		WeaponID: "dagger", TargetAC: new(5), TargetSize: combatsvc.SizeSmallMedium, Backstab: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 16, res.EffectiveThac0, "THAC0 20, +4 backstab, +0 Strength 16")
	assert.True(t, res.Hit)
	assert.Equal(t, 7, res.TotalDamage)
	assert.Equal(t, map[string]int{"d20": 1, "d100": 3, "die": 1}, dice.calls)
	entries, err := bs.Log.List(ctx, id, 4)
	require.NoError(t, err)
	require.Len(t, entries, 4)
	assert.Equal(t, actionlog.KindAttack, entries[0].Kind)
	assert.Equal(t, actionlog.KindEquipmentState, entries[1].Kind, "the dagger wielded")
	assert.Equal(t, actionlog.KindSkillCheck, entries[3].Kind, "after it the dagger's item_add")
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
		Details: identity(),
		Name:    "Vex", Class: "thief", Race: "halfling", Str: 16, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
		SkillPoints: map[string]int{"open_locks": 30, "pick_pockets": 30},
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
// thief with 40 and 30 is refused at creation (ADR 0037 step 10), and one
// whose stored points are 40 and 30 (imported data) reads both warnings and
// may reduce them, not grow them. A roll the DM adjusts after it changes
// target and verdict; Read Languages at 0% is refused before the die.
func TestThief_Step17_TermsBudgetAdjustmentAndZeroPercent(t *testing.T) {
	ctx := context.Background()
	dice := &attackDice{t: t, d100s: []int{58}, calls: map[string]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	store := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(store), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	thief := func(points map[string]int) int64 {
		t.Helper()
		id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
			Details: identity(),
			Name:    "Vex", Class: "thief", Race: "halfling", Str: 12, Dex: 18, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
			SkillPoints: points,
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

	overspent := map[string]int{"open_locks": 40, "pick_pockets": 30}
	_, err = bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Vex", Class: "thief", Race: "halfling", Str: 12, Dex: 18, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
		SkillPoints: overspent,
	})
	require.ErrorIs(t, err, domain.ErrOverBudget, "never created over the budget")

	over := thief(nil)
	require.NoError(t, store.Characters().SetSkillPoints(ctx, over, overspent), "imported over the budget")
	c, err = bs.Character.GetCharacter(ctx, over)
	require.NoError(t, err)
	assert.Equal(t, []ruletablesvc.BudgetWarning{
		{Code: ruletablesvc.BudgetTotal, Points: 70, Limit: 60},
		{Code: ruletablesvc.BudgetPerSkill, Skill: "open_locks", Points: 40, Limit: 30},
	}, c.Derived.SkillBudget)
	_, err = bs.Skills.SetPoints(ctx, over, map[string]int{"open_locks": 40, "pick_pockets": 31})
	require.ErrorIs(t, err, domain.ErrOverBudget, "the overspend grown")
	entries, err := bs.Log.List(ctx, over, 10)
	require.NoError(t, err)
	assert.Empty(t, entries, "a refused spend logs nothing")
	_, err = bs.Skills.SetPoints(ctx, over, map[string]int{"open_locks": 35, "pick_pockets": 30})
	require.NoError(t, err, "the overspend reduced")
	c, err = bs.Character.GetCharacter(ctx, over)
	require.NoError(t, err)
	assert.Equal(t, []ruletablesvc.BudgetWarning{
		{Code: ruletablesvc.BudgetTotal, Points: 65, Limit: 60},
		{Code: ruletablesvc.BudgetPerSkill, Skill: "open_locks", Points: 35, Limit: 30},
	}, c.Derived.SkillBudget, "still over: the warnings stay")

	// The undo of the reduction writes the imported overspend back: a restore, no refusal.
	top, ok, err := bs.Log.TopID(ctx, over)
	require.NoError(t, err)
	require.True(t, ok)
	e, err := bs.Log.Undo(ctx, over, top)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindSkillPoints, e.Kind)
	c, err = bs.Character.GetCharacter(ctx, over)
	require.NoError(t, err)
	assert.Equal(t, overspent, c.SkillPoints)
	assert.Len(t, c.Derived.SkillBudget, 2, "the overspend's warnings are back")
}

// ADR 0037 step 10, each pool on its own (a thief's total and each skill's cap, a bard's total):
// an imported thief with 40 in Open Locks (cap 30 at 1st) may not move the overspend to another
// skill, nor add points while over; a 1st-level bard may put its 20 points in one skill (no cap),
// and 21 in all is refused by the preview and the save alike, nothing logged.
func TestSkillPoints_Step10_EachPoolOnItsOwn(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(store), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)

	thief, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Vex", Class: "thief", Race: "halfling", Str: 12, Dex: 18, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
	})
	require.NoError(t, err)
	require.NoError(t, store.Characters().SetSkillPoints(ctx, thief, map[string]int{"open_locks": 40}), "imported over the cap")
	for _, tt := range []struct {
		name   string
		points map[string]int
		want   domain.BudgetError
	}{
		{"the overspend moved to another skill", map[string]int{"pick_pockets": 40}, domain.BudgetError{Field: "skillPoints.pick_pockets", Spent: 40, Limit: 30}},
		{"moved and kept partly: the total grows first", map[string]int{"open_locks": 30, "pick_pockets": 40}, domain.BudgetError{Field: "skillPoints", Spent: 70, Limit: 60}},
	} {
		_, err := bs.Skills.SetPoints(ctx, thief, tt.points)
		var be *domain.BudgetError
		require.ErrorAs(t, err, &be, tt.name)
		assert.Equal(t, tt.want, *be, tt.name)
	}

	bard, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Lyra", Class: "bard", Race: "human", Str: 9, Dex: 16, Con: 10, Int: 13, Wis: 11, Cha: 17, HPMax: 4,
	})
	require.NoError(t, err)
	_, err = bs.Skills.SetPoints(ctx, bard, map[string]int{"climb_walls": 20})
	require.NoError(t, err, "20 points in one skill: no cap for a bard")
	over := map[string]int{"climb_walls": 20, "detect_noise": 1}
	_, err = bs.Skills.PreviewPoints(ctx, bard, over)
	var previewed *domain.BudgetError
	require.ErrorAs(t, err, &previewed)
	assert.Equal(t, domain.BudgetError{Field: "skillPoints", Spent: 21, Limit: 20}, *previewed)
	_, err = bs.Skills.SetPoints(ctx, bard, over)
	var saved *domain.BudgetError
	require.ErrorAs(t, err, &saved)
	assert.Equal(t, *previewed, *saved)
	entries, err := bs.Log.List(ctx, bard, 10)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "only the 20-point spend is logged")
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
		Details: identity(),
		Name:    "Vex", Class: "thief", Race: "halfling", Str: 16, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
	})
	require.NoError(t, err)
	dagger := newWeapon("Dagger", "1d4", "1d3")
	dagger.WeaponType = "dagger"
	_, err = bs.Items.Create(ctx, id, dagger)
	require.NoError(t, err)
	// A new piece is carried (ADR 0033 step 6): the attack needs it wielded.
	_, err = bs.Items.SetState(ctx, id, "dagger", itemsvc.StateWielded, false)
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

// ADR 0033 step 14 (amending ADR 0030 steps 1 and 17), through the real
// services: the skills read the body armour the thief wears. The halfling
// thief of Dexterity 17 with 30 points in Open Locks and Pick Pockets wears
// nothing at creation (the none column: Open Locks 55, Pick Pockets' armour
// term +5). An elven chain mail carried changes nothing; wearing it (one
// equipment_state entry) takes PHB Table 29's 5 off Open Locks (50) and puts
// Pick Pockets' term at -20; Undo gives the skills back and Redo applies them
// again. Swapping it for a studded leather reads the padded or studded column
// (45, -30), wearing an armour a thief may not wear (plate) the same worst
// column, and taking the armour off the none column again. A shield changes
// no skill.
func TestThief_Step14_TheSkillsFollowTheWornArmour(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Vex", Class: "thief", Race: "halfling", Str: 16, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
		SkillPoints: map[string]int{"open_locks": 30, "pick_pockets": 30},
	})
	require.NoError(t, err)
	skill := func(skill string) (percent, term int) {
		t.Helper()
		skills, err := bs.Skills.List(ctx, id)
		require.NoError(t, err)
		for _, k := range skills.ThiefSkills {
			if k.Skill == skill {
				return k.Percent, k.Armor
			}
		}
		require.Failf(t, "no such skill", skill)
		return 0, 0
	}
	expect := func(openLocks, pickPocketsTerm int, armour, msg string) {
		t.Helper()
		percent, _ := skill("open_locks")
		assert.Equal(t, openLocks, percent, msg)
		_, term := skill("pick_pockets")
		assert.Equal(t, pickPocketsTerm, term, msg)
		c, err := bs.Character.GetCharacter(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, armour, c.Derived.SkillArmor, msg)
		skills, err := bs.Skills.List(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, armour, skills.SkillArmor, "the Skills tab names it: "+msg)
	}
	add := func(name, column, row string) string {
		t.Helper()
		res, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{
			Name: map[string]string{"en": name}, Category: itemsvc.CategoryArmour, Behaviour: itemsvc.BehaviourStock,
			Count: 1, BaseAC: 5, SkillColumn: column, ArmorRow: row,
		})
		require.NoError(t, err)
		return res.Item.ID
	}
	wear := func(itemID string, state itemsvc.State, swap bool) {
		t.Helper()
		_, err := bs.Items.SetState(ctx, id, itemID, state, swap)
		require.NoError(t, err)
	}
	expect(55, 5, "none", "nothing worn at creation: the none column")

	chain := add("Elven chain mail", "elven_chain", "chain")
	expect(55, 5, "none", "a carried armour changes no skill")
	wear(chain, itemsvc.StateWorn, false)
	expect(50, -20, "elven_chain", "Table 29: elven chain")

	top, ok, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	e, err := bs.Log.Undo(ctx, id, top)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindEquipmentState, e.Kind, "the wear change is the logged move")
	expect(55, 5, "none", "the undo gives the skills back")
	_, err = bs.Log.Redo(ctx, id, top)
	require.NoError(t, err)
	expect(50, -20, "elven_chain", "the redo puts the armour on again")

	studded := add("Studded leather", "padded_studded", "studded_leather")
	wear(studded, itemsvc.StateWorn, true)
	expect(45, -30, "padded_studded_leather", "the swap: Table 29's padded or studded column")
	plate := add("Plate mail", "none_allowed", "plate")
	wear(plate, itemsvc.StateWorn, true)
	expect(45, -30, "padded_studded_leather", "an armour a thief may not wear reads the worst column")
	wear(plate, itemsvc.StateCarried, false)
	expect(55, 5, "none", "taken off: the none column")

	res, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{
		Name: map[string]string{"en": "Buckler"}, Category: itemsvc.CategoryShield, Behaviour: itemsvc.BehaviourStock,
		Count: 1, ShieldKind: "buckler", ACMelee: 1, ACMissile: 1,
	})
	require.NoError(t, err)
	wear(res.Item.ID, itemsvc.StateWorn, false)
	expect(55, 5, "none", "a shield changes no skill")
}

// ADR 0033 step 14 for the bard (amending ADR 0030 step 18): a bard in a
// metal armour up to chain mail reads its own metal column, Table 29's elven
// chain column with the bard's extra -5%.
func TestBard_Step14_MetalArmourReadsTheBardsColumn(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Lyra", Class: "bard", Race: "human", Str: 10, Dex: 15, Con: 12, Int: 14, Wis: 9, Cha: 16, HPMax: 6,
		SkillPoints: map[string]int{"climb_walls": 15},
	})
	require.NoError(t, err)
	res, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{
		Name: map[string]string{"en": "Chain mail"}, Category: itemsvc.CategoryArmour, Behaviour: itemsvc.BehaviourStock,
		Count: 1, BaseAC: 5, SkillColumn: "metal", ArmorRow: "chain",
	})
	require.NoError(t, err)
	_, err = bs.Items.SetState(ctx, id, res.Item.ID, itemsvc.StateWorn, false)
	require.NoError(t, err)
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, ruletablesvc.ArmorMetal, c.Derived.SkillArmor)
	for _, b := range c.Derived.BardSkills {
		if b.Skill == "climb_walls" {
			assert.Equal(t, -25, b.Armor, "elven chain -20 and the bard's -5")
		}
	}
}

// ADR 0030 step 17: the editor's preview, through the real services, gives the skills and budget
// the same points give once stored (percentages, terms, 1st-level budget 60 with 30 in one skill),
// and writes nothing: the stored points and the action log are as they were. It refuses what the
// save refuses: a range, and a new overspend (ADR 0037 step 10).
func TestThief_Step17_PreviewEqualsTheStoredPointsAndWritesNothing(t *testing.T) {
	ctx := context.Background()
	cfg := fixtureConfig()
	cfg.Dice = &attackDice{t: t, calls: map[string]int{}}
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Vex", Class: "thief", Race: "halfling", Str: 16, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
		SkillPoints: map[string]int{"open_locks": 30},
	})
	require.NoError(t, err)
	before, err := bs.Skills.List(ctx, id)
	require.NoError(t, err)
	logBefore, err := bs.Log.List(ctx, id, 50)
	require.NoError(t, err)

	draft := map[string]int{"open_locks": 30, "pick_pockets": 25}
	preview, err := bs.Skills.PreviewPoints(ctx, id, draft)
	require.NoError(t, err)

	after, err := bs.Skills.List(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the preview stores nothing")
	logAfter, err := bs.Log.List(ctx, id, 50)
	require.NoError(t, err)
	assert.Equal(t, logBefore, logAfter, "the preview logs nothing")

	assert.Equal(t, &ruletablesvc.SkillPointsBudget{
		Level: 1, First: 60, PerLevel: 30, Total: 60, Spent: 55, Left: 5, PerSkillFirst: 30, PerSkillPerLevel: 15, PerSkill: 30,
	}, preview.Budget, "the draft's budget: 55 spent of 60")
	assert.Equal(t, 55, preview.ThiefSkills[1].Percent, "open locks: 10 + 30 + 5 + 10")
	assert.Equal(t, 55, preview.ThiefSkills[1].Raw)

	_, err = bs.Skills.SetPoints(ctx, id, draft)
	require.NoError(t, err)
	stored, err := bs.Skills.List(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, stored, preview, "the preview equals the derived values of the same points")

	_, err = bs.Skills.PreviewPoints(ctx, id, map[string]int{"open_locks": 96})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "skillPoints.open_locks", fe.Field)

	over := map[string]int{"open_locks": 20, "pick_pockets": 31}
	_, err = bs.Skills.PreviewPoints(ctx, id, over)
	var previewed *domain.BudgetError
	require.ErrorAs(t, err, &previewed)
	assert.Equal(t, domain.BudgetError{Field: "skillPoints.pick_pockets", Spent: 31, Limit: 30}, *previewed)
	_, err = bs.Skills.SetPoints(ctx, id, over)
	var saved *domain.BudgetError
	require.ErrorAs(t, err, &saved)
	assert.Equal(t, *previewed, *saved, "the preview refuses as the save")
}

// A class without skill points has no budget; its preview of no points answers and of any point
// is refused with the field the PUT names.
func TestSkillPoints_AClassWithoutThemHasNoBudgetAndRefusesAPreviewedPoint(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Bram", Class: "fighter", Race: "human", Str: 16, Dex: 16, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 10,
	})
	require.NoError(t, err)
	got, err := bs.Skills.List(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, got.Budget)
	empty, err := bs.Skills.PreviewPoints(ctx, id, map[string]int{})
	require.NoError(t, err)
	assert.Nil(t, empty.Budget)
	_, err = bs.Skills.PreviewPoints(ctx, id, map[string]int{"open_locks": 5})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "skillPoints", fe.Field)
}
