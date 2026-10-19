package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// derivedOf is what the character's readers see: its derived block, the XP
// ledger's level and the preparation limits.
func derivedOf(t *testing.T, bs *composition.BackingServices, id int64) (charactersvc.Derived, int, []prepsvc.LevelLimit) {
	t.Helper()
	ctx := context.Background()
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	l, err := bs.XP.Get(ctx, id)
	require.NoError(t, err)
	info, err := bs.Prep.Info(ctx, id)
	require.NoError(t, err)
	return c.Derived, l.Level, info.Limits
}

// ADR 0030 steps 3, 5, 6, 7 and 8 over the fixture's tables (the books'
// numbers): the fixture bard (the seed's sheet, 1,293,418 XP) is level 15,
// THAC0 13, saves 10/8/9/13/9, slots 3/3/3/3/2, as his sheet says. An XP award
// that reaches 1,540,000 (Table 25's level 17) raises the target level only
// (the XP ledger's level, the pending level-ups): the THAC0, saves and slots
// stay level 15's until the level-up step takes 16 and 17, then they are
// level 17's (Table 53: 12; Table 60, rogues 17-20; Table 32) for every
// reader. Undoing the two rolls and the award brings them all back.
func TestDerived_BenefitsFollowTheRealizedLevelNotTheXP(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id := rootCharacter(t, bs)

	next := 1_320_000
	at15 := charactersvc.Derived{
		XPTotal: 1_293_418, TargetLevel: 15, RealizedLevel: 15, NextLevelXP: &next, THAC0: 13,
		Saves:      charactersvc.Saves{ParaPoison: 10, RodStaffWand: 8, PetriPoly: 9, Breath: 13, Spell: 9},
		SpellSlots: map[int]int{1: 3, 2: 3, 3: 3, 4: 3, 5: 2},
		LevelUp:    charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{}},
		// Kelsen's Intelligence 14 (PHB Table 4, ADR 0030 step 20.1).
		Spellbook: &charactersvc.SpellbookRules{ClassMaxSpellLevel: 6, IntMaxSpellLevel: 7, LearnChance: 60, MaxSpellsPerLevel: 9},
		// His seeded points give his sheet's 85 / 90 / 50 / 75 (ADR 0030
		// step 18), within the budget at any of these levels.
		BardSkills:  kelsenBardSkills,
		SkillBudget: []ruletablesvc.BudgetWarning{},
		// The seed bard wears the Leather armour none entered gave (ADR 0033 step 14).
		SkillArmor: ruletablesvc.ArmorLeather,
	}
	limits15 := []prepsvc.LevelLimit{{Level: 1, Max: 3}, {Level: 2, Max: 3}, {Level: 3, Max: 3}, {Level: 4, Max: 3}, {Level: 5, Max: 2}}
	d, ledgerLevel, limits := derivedOf(t, bs, id)
	assert.Equal(t, at15, d)
	assert.Equal(t, 15, ledgerLevel)
	assert.Equal(t, limits15, limits)

	_, err = bs.XP.Add(ctx, id, xpsvc.EntryInput{Base: 1_540_000 - 1_293_418, Reason: "the dragon"})
	require.NoError(t, err)
	next17 := 1_760_000
	d, ledgerLevel, limits = derivedOf(t, bs, id)
	pending := at15
	pending.XPTotal, pending.TargetLevel, pending.NextLevelXP = 1_540_000, 17, &next17
	// Above the rogues' name level, +2 each (step 7.2).
	pending.LevelUp = charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{{Level: 16, FixedGain: 2}, {Level: 17, FixedGain: 2}}}
	assert.Equal(t, pending, d, "the benefits wait for the level-up step")
	assert.Equal(t, 17, ledgerLevel, "the XP ledger shows the target level")
	assert.Equal(t, limits15, limits)

	for _, level := range []int{16, 17} {
		_, err = bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Level: level})
		require.NoError(t, err)
	}
	d, ledgerLevel, limits = derivedOf(t, bs, id)
	assert.Equal(t, charactersvc.Derived{
		XPTotal: 1_540_000, TargetLevel: 17, RealizedLevel: 17, NextLevelXP: &next17, THAC0: 12,
		Saves:      charactersvc.Saves{ParaPoison: 9, RodStaffWand: 6, PetriPoly: 8, Breath: 12, Spell: 7},
		SpellSlots: map[int]int{1: 4, 2: 4, 3: 3, 4: 3, 5: 3, 6: 1},
		LevelUp:    charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{}},
		Spellbook:  at15.Spellbook, BardSkills: kelsenBardSkills, SkillBudget: []ruletablesvc.BudgetWarning{},
		SkillArmor: ruletablesvc.ArmorLeather,
	}, d)
	assert.Equal(t, 17, ledgerLevel)
	assert.Equal(t, []prepsvc.LevelLimit{
		{Level: 1, Max: 4}, {Level: 2, Max: 4}, {Level: 3, Max: 3}, {Level: 4, Max: 3}, {Level: 5, Max: 3}, {Level: 6, Max: 1},
	}, limits)

	// Undo the second roll: level 16's benefits; then the first roll and the
	// award: everything is level 15's again.
	undoTop(t, bs, id)
	d, _, _ = derivedOf(t, bs, id)
	assert.Equal(t, 16, d.RealizedLevel)
	assert.Equal(t, []charactersvc.PendingLevel{{Level: 17, FixedGain: 2}}, d.LevelUp.Pending)
	undoTop(t, bs, id)
	undoTop(t, bs, id)
	d, ledgerLevel, limits = derivedOf(t, bs, id)
	assert.Equal(t, at15, d)
	assert.Equal(t, 15, ledgerLevel)
	assert.Equal(t, limits15, limits)
}
