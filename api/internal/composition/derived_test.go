package composition_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
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
	// The class entries' blocks and grants (ADR 0042 steps 7, 12 and 14) are
	// the multi-class tests'; these compare the single-class figures.
	d := c.Derived
	d.Classes, d.Features = nil, nil
	return d, l.Level, info.Limits
}

// kelsenPicks are the seed bard's eight picks (ADR 0037 step 16) with their figures at the realized
// level: each costs 1 (all in his groups), no extra slot, and each check target is the one the
// seed's ability and modifier gave before the picks (ADR 0014 step 1): Musical Instrument DEX 17
// -1, Tumbling DEX 17, Ventriloquism INT 14 -2, Riding WIS 8 +3, Appraising INT 14, Singing CHA 16,
// Reading/Writing INT 14 +1, Local History CHA 16. The non-weapon pool (step 3) is a rogue's: 3 +
// one slot every 4 realized levels + 4 for Intelligence 14; 8 spent.
func kelsenPicks(level int) *skills.ProficiencySlots {
	pick := func(id, ability string, modifier, target int) skills.PickFigures {
		return skills.PickFigures{ID: id, Slots: 1, Cost: 1, Ability: ability, Modifier: modifier, Known: true, Target: target, HasCheck: true}
	}
	pool := ruletablesvc.NonweaponPool{Initial: 3, PerLevels: 4, LevelGained: level / 4, Intelligence: 4, Spent: 8}
	pool.Available = 3 + level/4 + 4
	pool.Left = pool.Available - 8
	// ADR 0037 step 3, weapon pool: 2 + one slot every 4 realized levels; the seed's five weapon
	// types spend 5.
	weapon := ruletablesvc.WeaponPool{Initial: 2, PerLevels: 4, LevelGained: level / 4, Available: 2 + level/4, Spent: 5}
	weapon.Left = weapon.Available - 5
	wpick := func(t string) skills.WeaponPickFigures {
		return skills.WeaponPickFigures{Type: t, Slots: 1, Allowed: true}
	}
	// ADR 0037 step 8: Table 34's penalty for the rogue group.
	return &skills.ProficiencySlots{Nonweapon: pool, Weapon: weapon, Penalty: -3, Picks: []skills.PickFigures{
		pick("musical_instrument", "dex", -1, 16), pick("tumbling", "dex", 0, 17), pick("ventriloquism", "int", -2, 12),
		pick("riding_land", "wis", 3, 11), pick("appraising", "int", 0, 14), pick("singing", "cha", 0, 16),
		pick("reading_writing", "int", 1, 15), pick("local_history", "cha", 0, 16),
	}, WeaponPicks: []skills.WeaponPickFigures{
		wpick("short_bow"), wpick("dagger"), wpick("dart"), wpick("long_sword"), wpick("other_melee"),
	}, WeaponTypes: func() (out []skills.WeaponTypeFigure) {
		// The bard may pick every type (ADR 0042 step 16.1), none specialized.
		for _, t := range ruletablesvc.WeaponTypes {
			out = append(out, skills.WeaponTypeFigure{Type: t})
		}
		return out
	}()}
}

// ADR 0030 steps 3, 5, 6, 7 and 8 over the fixture's tables (the books'
// numbers): the fixture bard (the seed's sheet, 1,293,418 XP) is level 15,
// THAC0 13, saves 10/8/9/13/9, slots 3/3/3/3/2, as his sheet says. An XP award
// that reaches 1,540,000 (Table 25's level 17) raises the target level only
// (the XP ledger's level, the pending level-ups): the THAC0, saves and slots
// stay level 15's until the level-up step takes 16 and 17, then they are
// level 17's (Table 53: 12; Table 60, rogues 17-20; Table 32) for every
// reader. Undoing the two rolls and the award brings them all back. The proficiency slots follow
// the realized level too (ADR 0037 step 3): 10 non-weapon and 5 weapon slots at 15, the same with
// 17 pending, 11 and 6 once 17 is taken (a rogue's slots at 16). Step 15: each pending level says
// what it grants (16: a weapon slot, a non-weapon slot and a bard's 15 points; 17: 15 points), and
// what is left to spend (2 non-weapon slots and 30 points at 15) grows with the levels taken (1
// weapon slot, 3 non-weapon, 60 points at 17); the undo takes it back, with the level.
func TestDerived_BenefitsFollowTheRealizedLevelNotTheXP(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id := rootCharacter(t, bs)

	next := 1_320_000
	at15 := charactersvc.Derived{
		XPTotal: 1_293_418, Group: "rogue", TargetLevel: 15, RealizedLevel: 15, NextLevelXP: &next, THAC0: 13,
		Saves: charactersvc.Saves{ParaPoison: 10, RodStaffWand: 8, PetriPoly: 9, Breath: 13, Spell: 9},
		// Dexterity 17: PHB Table 2's missile +2 and defensive -3 (ADR 0046 step 2.1).
		DexMissile: 2, DexDefence: -3,
		SpellSlots: map[int]int{1: 3, 2: 3, 3: 3, 4: 3, 5: 2},
		LevelUp: charactersvc.LevelUp{
			Pending: []charactersvc.PendingLevel{},
			ToSpend: skills.LevelGrants{NonweaponSlots: 2, SkillPoints: 30},
		},
		// Kelsen's Intelligence 14 (PHB Table 4, ADR 0030 step 20.1).
		Spellbook: &charactersvc.SpellbookRules{ClassMaxSpellLevel: 6, IntMaxSpellLevel: 7, LearnChance: 60, MaxSpellsPerLevel: 9},
		// His seeded points give his sheet's 85 / 90 / 50 / 75 (ADR 0030
		// step 18), within the budget at any of these levels.
		// The seed bard wears the Leather armour none entered gave (ADR 0033 step 14).
		SkillColumn: ruletablesvc.ColumnLeather,
		SkillSets: []skills.SkillSet{{
			Set: "bard", Name: localized.Text{"en": "Bard skills", "fr": "Talents de barde"}, Skills: kelsenBardSkills, Armor: ruletablesvc.ArmorLeather, Budget: []ruletablesvc.BudgetWarning{},
			Points: &ruletablesvc.SkillPointsBudget{Level: 15, First: 20, PerLevel: 15, Total: 230, Spent: 200, Left: 30},
		}},
		Proficiencies: kelsenPicks(15),
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
	pending.LevelUp = charactersvc.LevelUp{Pending: []charactersvc.PendingLevel{
		{Class: "bard", Classes: 1, Level: 16, FixedGain: 2, Grants: skills.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1, SkillPoints: 15}},
		{Class: "bard", Classes: 1, Level: 17, FixedGain: 2, Grants: skills.LevelGrants{SkillPoints: 15}},
	}, ToSpend: at15.LevelUp.ToSpend}
	assert.Equal(t, pending, d, "the benefits wait for the level-up step")
	assert.Equal(t, 17, ledgerLevel, "the XP ledger shows the target level")
	assert.Equal(t, limits15, limits)

	for _, level := range []int{16, 17} {
		_, err = bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Level: level})
		require.NoError(t, err)
	}
	d, ledgerLevel, limits = derivedOf(t, bs, id)
	assert.Equal(t, charactersvc.Derived{
		XPTotal: 1_540_000, Group: "rogue", TargetLevel: 17, RealizedLevel: 17, NextLevelXP: &next17, THAC0: 12,
		DexMissile: 2, DexDefence: -3,
		Saves:      charactersvc.Saves{ParaPoison: 9, RodStaffWand: 6, PetriPoly: 8, Breath: 12, Spell: 7},
		SpellSlots: map[int]int{1: 4, 2: 4, 3: 3, 4: 3, 5: 3, 6: 1},
		LevelUp: charactersvc.LevelUp{
			Pending: []charactersvc.PendingLevel{},
			ToSpend: skills.LevelGrants{WeaponSlots: 1, NonweaponSlots: 3, SkillPoints: 60},
		},
		Spellbook:   at15.Spellbook,
		SkillColumn: ruletablesvc.ColumnLeather,
		SkillSets: []skills.SkillSet{{
			Set: "bard", Name: localized.Text{"en": "Bard skills", "fr": "Talents de barde"}, Skills: kelsenBardSkills, Armor: ruletablesvc.ArmorLeather, Budget: []ruletablesvc.BudgetWarning{},
			Points: &ruletablesvc.SkillPointsBudget{Level: 17, First: 20, PerLevel: 15, Total: 260, Spent: 200, Left: 60},
		}},
		Proficiencies: kelsenPicks(17),
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
	assert.Equal(t, []charactersvc.PendingLevel{{Class: "bard", Classes: 1, Level: 17, FixedGain: 2, Grants: skills.LevelGrants{SkillPoints: 15}}}, d.LevelUp.Pending)
	assert.Equal(t, skills.LevelGrants{WeaponSlots: 1, NonweaponSlots: 3, SkillPoints: 45}, d.LevelUp.ToSpend,
		"level 17's points go with its roll")
	undoTop(t, bs, id)
	undoTop(t, bs, id)
	d, ledgerLevel, limits = derivedOf(t, bs, id)
	assert.Equal(t, at15, d)
	assert.Equal(t, 15, ledgerLevel)
	assert.Equal(t, limits15, limits)
}
