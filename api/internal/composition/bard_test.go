package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// kelsenBardSkills are the fixture bard's skills (ADR 0030 step 18): his
// seeded points on Table 33's base, half-elf (Table 27), Dexterity 17 on Pick
// Pockets (Table 28) and leather (no armour stored, Table 29's baseline) give
// his sheet's 85 / 90 / 50 / 75.
var kelsenBardSkills = []ruletablesvc.SkillPercent{
	{Skill: "climb_walls", Base: 50, Points: 35, Percent: 85},
	{Skill: "detect_noise", Base: 20, Points: 70, Percent: 90},
	{Skill: "pick_pockets", Base: 10, Points: 25, Race: 10, Dexterity: 5, Percent: 50},
	{Skill: "read_languages", Base: 5, Points: 70, Percent: 75},
}

// ADR 0030 steps 1 and 18 on the fixture's tables, through the real
// services: a 1st-level human bard of Dexterity 16 in metal armour enters 15
// points in Climb Walls and 5 in Detect Noise (the whole 20 of the 1st
// level). Climb Walls is 50 + 15 - 20 (elven chain column) - 5 (metal) = 40,
// rolled as a bard skill check against it; Pick Pockets 10 - 25 is floored at
// 0. After creation the bard spends 10 more points in Read Languages (5 + 10
// - 5 = 10), over the budget (a warning), as one skill_points entry; undoing
// it gives the points back. A new armour changes the percentages. The armour
// is the one the bard wears (ADR 0033 step 14): a chain mail put on after
// creation, swapped for a leather armour at the end.
func TestBard_Step18_CreateRollSpendAndUndo(t *testing.T) {
	ctx := context.Background()
	dice := &attackDice{t: t, d100s: []int{40, 41}, calls: map[string]int{}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Details: identity(),
		Name:    "Lyra", Class: "bard", Race: "human", Str: 9, Dex: 16, Con: 10, Int: 13, Wis: 11, Cha: 17, HPMax: 4,
		SkillPoints: map[string]int{"climb_walls": 15, "detect_noise": 5},
	})
	require.NoError(t, err)
	wear := func(name, column, row string) {
		t.Helper()
		res, err := bs.Items.Create(ctx, id, itemsvc.ItemInput{
			Name: map[string]string{"en": name}, Category: itemsvc.CategoryArmour, Behaviour: itemsvc.BehaviourStock,
			Count: 1, BaseAC: 5, SkillColumn: column, ArmorRow: row,
		})
		require.NoError(t, err)
		_, err = bs.Items.SetState(ctx, id, res.Item.ID, itemsvc.StateWorn, true)
		require.NoError(t, err)
	}
	wear("Chain mail", "metal", "chain")
	percents := func() map[string]int {
		t.Helper()
		skills, err := bs.Skills.List(ctx, id)
		require.NoError(t, err)
		assert.Empty(t, skills.ThiefSkills)
		out := map[string]int{}
		for _, b := range skills.BardSkills {
			out[b.Skill] = b.Percent
		}
		return out
	}
	assert.Equal(t, map[string]int{"climb_walls": 40, "detect_noise": 15, "pick_pockets": 0, "read_languages": 0}, percents())
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, c.Derived.SkillBudget, "20 points: the bard's budget at 1st")

	check, err := bs.Skills.Roll(ctx, id, "climb_walls", skillsvc.Adjustment{})
	require.NoError(t, err)
	assert.Equal(t, skillsvc.KindBardSkill, check.Kind)
	assert.Equal(t, 40, check.Target, "the derived percentage")
	assert.True(t, check.Success, "40 at or under 40")
	check, err = bs.Skills.Roll(ctx, id, "climb_walls", skillsvc.Adjustment{})
	require.NoError(t, err)
	assert.False(t, check.Success, "41 over 40")
	assert.Equal(t, map[string]int{"d100": 2}, dice.calls)

	spend := map[string]int{"climb_walls": 15, "detect_noise": 5, "read_languages": 10}
	got, err := bs.Skills.SetPoints(ctx, id, spend)
	require.NoError(t, err)
	assert.Equal(t, spend, got)
	assert.Equal(t, 10, percents()["read_languages"])
	c, err = bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []ruletablesvc.BudgetWarning{{Code: ruletablesvc.BudgetTotal, Points: 30, Limit: 20}}, c.Derived.SkillBudget,
		"over the budget: a warning, the points stored")

	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, entries, 5, "the armour added and worn, two checks and one spend")
	assert.Equal(t, actionlog.KindSkillPoints, entries[0].Kind)
	top, ok, err := bs.Log.TopID(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	e, err := bs.Log.Undo(ctx, id, top)
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindSkillPoints, e.Kind)
	assert.Equal(t, 0, percents()["read_languages"], "the undo gives the points back")
	c, err = bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"climb_walls": 15, "detect_noise": 5}, c.SkillPoints)
	assert.Empty(t, c.Derived.SkillBudget)

	wear("Leather armor", "leather", "leather")
	assert.Equal(t, 65, percents()["climb_walls"], "50 + 15 in leather")
}

// ADR 0030 step 18 and the sheet (step 24) on the fixture's seed bard: his
// seeded points give exactly his sheet's percentages, and his skills roll
// against them.
func TestBard_Step18_TheSeedBardKeepsHisSheetsPercentages(t *testing.T) {
	ctx := context.Background()
	cfg := fixtureConfig()
	cfg.Dice = &attackDice{t: t, d100s: []int{50}, calls: map[string]int{}}
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id := rootCharacter(t, bs)
	skills, err := bs.Skills.List(ctx, id)
	require.NoError(t, err)
	got := make([]ruletablesvc.SkillPercent, len(skills.BardSkills))
	for i, b := range skills.BardSkills {
		got[i] = b.SkillPercent
	}
	assert.Equal(t, kelsenBardSkills, got)
	check, err := bs.Skills.Roll(ctx, id, "pick_pockets", skillsvc.Adjustment{})
	require.NoError(t, err)
	assert.Equal(t, 50, check.Target)
	assert.True(t, check.Success)
}
