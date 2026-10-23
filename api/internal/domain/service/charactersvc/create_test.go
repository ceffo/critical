package charactersvc_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

var ownerID = uuid.MustParse("0b6a8c7e-5f1d-4a59-9a3e-3c2d1e0f4a11")

// lyra is a valid new character: a level 1 half-elf bard.
func lyra() charactersvc.NewCharacter {
	return charactersvc.NewCharacter{
		Details: identity(),
		Name:    "  Lyra ", Class: "bard", Race: "half_elf",
		Str: 10, Dex: 15, Con: 12, Int: 14, Wis: 9, Cha: 16, HPMax: 6, XP: 0,
	}
}

// rogue1to4 is PHB Table 60's rogue row for levels 1 to 4.
var rogue1to4 = ruletablesvc.SavingThrowRow{
	Group: "rogue", LevelMin: 1, LevelMax: 4, ParaPoison: 13, RodStaffWand: 14, PetriPoly: 12, Breath: 16, Spell: 15,
}

var rogue1to4Saves = charactersvc.Saves{ParaPoison: 13, RodStaffWand: 14, PetriPoly: 12, Breath: 16, Spell: 15}

// expectLevel1Bard answers the derivation of a level 1 bard at 0 XP (PHB
// Tables 25, 53, 60 and 32).
func expectLevel1Bard(e *env) {
	e.rules.EXPECT().SavesFor("rogue", 1).Return(rogue1to4, nil).Once()
	e.rules.EXPECT().SpellsPerDay(classDef("bard"), 1, mock.Anything).Return(map[int]int{}, nil).Once()
}

// A new character is stored with its entered values, the name trimmed, and
// as the sheet's figures the values the rules derive at its starting XP (ADR
// 0030 step 24: it states no sheet, so it shows no difference); it becomes
// the owner's current character in the same transaction, and the owner's
// remembered sessions are forgotten (ADR 0029 step 4.7). Its max HP is set
// for that level (ADR 0030 step 7.5: no level-up pending).
func TestCreate_StoresTheEnteredValuesAndTheDerivedSheetAndMakesItCurrent(t *testing.T) {
	e := newEnv(t)
	e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
	expectLevel1Bard(e)
	want := charactersvc.Character{
		Name: "Lyra", Class: "bard", Race: "half_elf",
		Str: 10, Dex: 15, Con: 12, Int: 14, Wis: 9, Cha: 16, HPMax: 6, RealizedLevel: 1, XPTotal: 0,
		Sheet:         charactersvc.Sheet{Level: 1, THAC0: 20, Saves: rogue1to4Saves},
		Details:       charactersvc.Details{Alignment: "neutral_good", Sex: "male", Languages: []string{}},
		Proficiencies: skills.Picks{Nonweapon: []skills.Pick{}, Weapon: []skills.WeaponPick{}},
	}
	created := e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, want).Return(12, nil).Once()
	e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(12)).Return(nil).Once().NotBefore(created)
	e.sessions.EXPECT().ForgetUser(ownerID).Once()

	id, err := e.svc.Create(context.Background(), ownerID, lyra())
	require.NoError(t, err)
	assert.Equal(t, int64(12), id)
	assert.Equal(t, 1, e.tx.n, "one transaction")
}

// A failed write forgets no session and returns the error.
func TestCreate_AFailedWriteChangesNothingElse(t *testing.T) {
	e := newEnv(t)
	e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
	expectLevel1Bard(e)
	boom := errors.New("boom")
	e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.Anything).Return(0, boom).Once()

	_, err := e.svc.Create(context.Background(), ownerID, lyra())
	require.ErrorIs(t, err, boom)
}

// Invalid input is refused with the field and a code the web words, before
// anything is written: a missing or long name, a class no character may be
// created with (a paladin: no such class), a race that
// is none of the six, one the class does not admit (DMG Table 7), an ability
// score outside 3 to 19, a max HP outside 1 to 999, a starting XP below 0.
func TestCreate_RefusesInvalidInputWithFieldErrors(t *testing.T) {
	for _, tt := range []struct {
		name        string
		edit        func(n *charactersvc.NewCharacter)
		raceAllowed *bool
		field, code string
	}{
		{"empty name", func(n *charactersvc.NewCharacter) { n.Name = "   " }, nil, "name", "empty"},
		{"long name", func(n *charactersvc.NewCharacter) { n.Name = strings.Repeat("é", 41) }, nil, "name", "too_long"},
		{"unknown class", func(n *charactersvc.NewCharacter) { n.Class = "paladin" }, nil, "class", "not_creatable"},
		{"no race", func(n *charactersvc.NewCharacter) { n.Race = "" }, nil, "race", "empty"},
		{"unknown race", func(n *charactersvc.NewCharacter) { n.Race = "orc" }, nil, "race", "unknown"},
		{"race the class does not admit", func(n *charactersvc.NewCharacter) { n.Race = "dwarf" }, ptr(false), "race", "race_not_allowed"},
		// ADR 0032 steps 2 and 3 at creation: required codes, and the class limit refuses.
		{"no alignment", func(n *charactersvc.NewCharacter) { n.Details.Alignment = nil }, ptr(true), "details.alignment", "empty"},
		{"unknown alignment", func(n *charactersvc.NewCharacter) { n.Details.Alignment = ptr("good") }, ptr(true), "details.alignment", "unknown"},
		{"bard lawful good", func(n *charactersvc.NewCharacter) { n.Details.Alignment = ptr("lawful_good") }, ptr(true), "details.alignment", "alignment_not_allowed"},
		{"no sex", func(n *charactersvc.NewCharacter) { n.Details.Sex = nil }, ptr(true), "details.sex", "empty"},
		{"unknown sex", func(n *charactersvc.NewCharacter) { n.Details.Sex = ptr("x") }, ptr(true), "details.sex", "unknown"},
		{"str 2", func(n *charactersvc.NewCharacter) { n.Str = 2 }, ptr(true), "str", "out_of_range"},
		{"dex 20", func(n *charactersvc.NewCharacter) { n.Dex = 20 }, ptr(true), "dex", "out_of_range"},
		{"con 0", func(n *charactersvc.NewCharacter) { n.Con = 0 }, ptr(true), "con", "out_of_range"},
		{"int 25", func(n *charactersvc.NewCharacter) { n.Int = 25 }, ptr(true), "int", "out_of_range"},
		{"wis -1", func(n *charactersvc.NewCharacter) { n.Wis = -1 }, ptr(true), "wis", "out_of_range"},
		{"cha 20", func(n *charactersvc.NewCharacter) { n.Cha = 20 }, ptr(true), "cha", "out_of_range"},
		{"no max HP", func(n *charactersvc.NewCharacter) { n.HPMax = 0 }, ptr(true), "hpMax", "out_of_range"},
		{"max HP 1000", func(n *charactersvc.NewCharacter) { n.HPMax = 1000 }, ptr(true), "hpMax", "out_of_range"},
		{"negative XP", func(n *charactersvc.NewCharacter) { n.XP = -1 }, ptr(true), "xp", "out_of_range"},
		{"XP past the cap", func(n *charactersvc.NewCharacter) { n.XP = charactersvc.MaxStartingXP + 1 }, ptr(true), "xp", "out_of_range"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			n := lyra()
			tt.edit(&n)
			if tt.raceAllowed != nil {
				e.rules.EXPECT().RaceAllows(n.Race, n.Class).Return(*tt.raceAllowed, nil).Once()
			}
			_, err := e.svc.Create(context.Background(), ownerID, n)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, tt.field, fe.Field)
			assert.Equal(t, tt.code, fe.Message)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.Zero(t, e.tx.n, "nothing written")
		})
	}
}

// The bounds are accepted: 3 and 19, 1 HP and 999, 0 XP.
func TestCreate_AcceptsTheBounds(t *testing.T) {
	e := newEnv(t)
	n := lyra()
	n.Str, n.Dex, n.Con, n.Int, n.Wis, n.Cha, n.HPMax = 3, 19, 3, 19, 3, 19, 999
	e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
	expectLevel1Bard(e)
	e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.Anything).Return(4, nil).Once()
	e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(4)).Return(nil).Once()
	e.sessions.EXPECT().ForgetUser(ownerID).Once()
	_, err := e.svc.Create(context.Background(), ownerID, n)
	require.NoError(t, err)
}

func ptr[T any](v T) *T { return &v }

// ADR 0030 step 9: a fighter with Strength 18 enters an exceptional strength
// of 1 to 100 or none; it is refused for any other class, any other
// Strength and a halfling, and stored as entered.
func TestCreate_Step9_ExceptionalStrength(t *testing.T) {
	fighter := func(race string, str, exc int) charactersvc.NewCharacter {
		return charactersvc.NewCharacter{
			Details: identity(),
			Name:    "Brom", Class: "fighter", Race: race,
			Str: str, StrExceptional: exc, Dex: 12, Con: 15, Int: 9, Wis: 10, Cha: 8, HPMax: 10,
		}
	}
	bard := lyra()
	bard.Str, bard.StrExceptional = 18, 50
	for _, tt := range []struct {
		name string
		n    charactersvc.NewCharacter
		code string
	}{
		{"above 18/00", fighter("human", 18, 101), "out_of_range"},
		{"negative", fighter("human", 18, -1), "out_of_range"},
		{"strength 17", fighter("human", 17, 50), "not_allowed"},
		{"strength 19", fighter("human", 19, 50), "not_allowed"},
		{"a halfling fighter", fighter("halfling", 18, 50), "not_allowed"},
		{"a bard", bard, "not_allowed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.rules.EXPECT().RaceAllows(tt.n.Race, tt.n.Class).Return(true, nil).Once()
			_, err := e.svc.Create(context.Background(), ownerID, tt.n)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, "strExceptional", fe.Field)
			assert.Equal(t, tt.code, fe.Message)
			assert.Zero(t, e.tx.n, "nothing written")
		})
	}
	for _, exc := range []int{0, 1, 100} {
		t.Run(fmt.Sprintf("stored %d", exc), func(t *testing.T) {
			e := newEnv(t)
			e.rules.EXPECT().RaceAllows("human", "fighter").Return(true, nil).Once()
			e.rules.EXPECT().SavesFor("warrior", 1).Return(ruletablesvc.SavingThrowRow{Group: "warrior"}, nil).Once()
			e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.MatchedBy(func(c charactersvc.Character) bool {
				return c.Class == "fighter" && c.Str == 18 && c.StrExceptional == exc
			})).Return(5, nil).Once()
			e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(5)).Return(nil).Once()
			e.sessions.EXPECT().ForgetUser(ownerID).Once()
			_, err := e.svc.Create(context.Background(), ownerID, fighter("human", 18, exc))
			require.NoError(t, err)
		})
	}
}

// ADR 0030 steps 1 and 17: a thief enters its points (each a thieving
// skill, 0 to 95); any other class enters none. Refused before anything is
// written, the field named; the points are stored without the zeros. No
// armour is entered (ADR 0033 step 14): nothing is worn yet, so the skills
// read the none column.
func TestCreate_Step17_ThiefSkillPointsAndArmour(t *testing.T) {
	thief := func(points map[string]int) charactersvc.NewCharacter {
		return charactersvc.NewCharacter{
			Details: identity(),
			Name:    "Vex", Class: "thief", Race: "halfling", Str: 10, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
			SkillPoints: points,
		}
	}
	fighter := func(points map[string]int) charactersvc.NewCharacter {
		return charactersvc.NewCharacter{
			Details: identity(),
			Name:    "Bram", Class: "fighter", Race: "human", Str: 16, Dex: 12, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 10,
			SkillPoints: points,
		}
	}
	for _, tt := range []struct {
		name        string
		n           charactersvc.NewCharacter
		field, code string
	}{
		{"an unknown skill", thief(map[string]int{"backstab": 10}), "skillPoints.backstab", "unknown"},
		{"negative points", thief(map[string]int{"open_locks": -1}), "skillPoints.open_locks", "out_of_range"},
		{"more than 95 points", thief(map[string]int{"open_locks": 96}), "skillPoints.open_locks", "out_of_range"},
		{"a fighter's points", fighter(map[string]int{"climb_walls": 10}), "skillPoints", "not_allowed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.rules.EXPECT().RaceAllows(tt.n.Race, tt.n.Class).Return(true, nil).Once()
			_, err := e.svc.Create(context.Background(), ownerID, tt.n)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, tt.field, fe.Field)
			assert.Equal(t, tt.code, fe.Message)
			assert.Zero(t, e.tx.n, "nothing written")
		})
	}
	t.Run("stored", func(t *testing.T) {
		e := newEnv(t)
		e.rules.EXPECT().RaceAllows("halfling", "thief").Return(true, nil).Once()
		e.rules.EXPECT().SavesFor("rogue", 1).Return(rogue1to4, nil).Once()
		e.rules.EXPECT().ConSaveBonusFor(12).Return(0, nil).Once()
		want := map[string]int{"open_locks": 30, "pick_pockets": 25}
		e.rules.EXPECT().RogueSkills(skillSet("thief"), "halfling", 17, "none", want).Return(nil, nil).Once()
		e.rules.EXPECT().BackstabFor(1).Return(2, nil).Once()
		e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.MatchedBy(func(c charactersvc.Character) bool {
			return c.Class == "thief" && assert.ObjectsAreEqual(skills.SetPoints{"thief": want}, c.SkillPoints)
		})).Return(6, nil).Once()
		e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(6)).Return(nil).Once()
		e.sessions.EXPECT().ForgetUser(ownerID).Once()
		_, err := e.svc.Create(context.Background(), ownerID, thief(map[string]int{
			"open_locks": 30, "pick_pockets": 25, "climb_walls": 0,
		}))
		require.NoError(t, err)
		assert.Zero(t, e.armour.reads, "a character being created wears nothing")
	})
}

// ADR 0030 steps 1 and 18: a bard enters its points (each one of its four
// skills, 0 to 95), refused as the thief's are; the budget is ADR 0037 step
// 10's (Create_Step10_OverBudgetRefused).
// The points are stored without the zeros; no armour is entered (ADR 0033
// step 14).
func TestCreate_Step18_BardSkillPointsAndArmour(t *testing.T) {
	bard := func(points map[string]int) charactersvc.NewCharacter {
		n := lyra()
		n.SkillPoints = points
		return n
	}
	for _, tt := range []struct {
		name        string
		n           charactersvc.NewCharacter
		field, code string
	}{
		{"a thieving skill the bard lacks", bard(map[string]int{"open_locks": 10}), "skillPoints.open_locks", "unknown"},
		{"negative points", bard(map[string]int{"climb_walls": -1}), "skillPoints.climb_walls", "out_of_range"},
		{"more than 95 points", bard(map[string]int{"read_languages": 96}), "skillPoints.read_languages", "out_of_range"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.rules.EXPECT().RaceAllows(tt.n.Race, tt.n.Class).Return(true, nil).Once()
			_, err := e.svc.Create(context.Background(), ownerID, tt.n)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, tt.field, fe.Field)
			assert.Equal(t, tt.code, fe.Message)
			assert.Zero(t, e.tx.n, "nothing written")
		})
	}
	t.Run("stored", func(t *testing.T) {
		e := newEnv(t)
		e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
		expectLevel1Bard(e)
		// The bard's 20 points at level 1, all spent (ADR 0037 step 10
		// refuses more: Create_Step10_OverBudgetRefused).
		want := map[string]int{"climb_walls": 10, "pick_pockets": 10}
		e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.MatchedBy(func(c charactersvc.Character) bool {
			return c.Class == "bard" && assert.ObjectsAreEqual(skills.SetPoints{"bard": want}, c.SkillPoints)
		})).Return(7, nil).Once()
		e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(7)).Return(nil).Once()
		e.sessions.EXPECT().ForgetUser(ownerID).Once()
		_, err := e.svc.Create(context.Background(), ownerID, bard(map[string]int{
			"climb_walls": 10, "pick_pockets": 10, "detect_noise": 0,
		}))
		require.NoError(t, err)
	})
}

// ADR 0037 step 10 at creation: nothing was spent before, so points over the
// budget of the starting level are refused (a 409 over_budget naming the
// field, the points and the limit) before anything is written; the preview
// refuses them as the creation does. At level 1 a thief has 60 points, at
// most 30 in one skill; a bard 20, with no limit in one skill. Leftover
// points are no refusal (the thief and bard "stored" cases of steps 17 and
// 18).
func TestCreate_Step10_OverBudgetRefused(t *testing.T) {
	thief := func(points map[string]int) charactersvc.NewCharacter {
		return charactersvc.NewCharacter{
			Details: identity(),
			Name:    "Vex", Class: "thief", Race: "halfling", Str: 10, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
			SkillPoints: points,
		}
	}
	bard := func(points map[string]int) charactersvc.NewCharacter {
		n := lyra()
		n.SkillPoints = points
		return n
	}
	expectLevel1Thief := func(e *env, points map[string]int, times int) {
		e.rules.EXPECT().SavesFor("rogue", 1).Return(rogue1to4, nil).Times(times)
		e.rules.EXPECT().ConSaveBonusFor(12).Return(0, nil).Times(times)
		e.rules.EXPECT().RogueSkills(skillSet("thief"), "halfling", 17, "none", points).Return(nil, nil).Times(times)
		e.rules.EXPECT().BackstabFor(1).Return(2, nil).Times(times)
	}
	for _, tt := range []struct {
		name         string
		n            charactersvc.NewCharacter
		expect       func(e *env)
		field        string
		spent, limit int
	}{
		{
			"a thief's total", thief(map[string]int{"open_locks": 30, "pick_pockets": 30, "climb_walls": 1}),
			func(e *env) {
				expectLevel1Thief(e, map[string]int{"open_locks": 30, "pick_pockets": 30, "climb_walls": 1}, 1)
			},
			"skillPoints", 61, 60,
		},
		{
			"a thief's skill over its cap", thief(map[string]int{"open_locks": 31}),
			func(e *env) { expectLevel1Thief(e, map[string]int{"open_locks": 31}, 1) },
			"skillPoints.open_locks", 31, 30,
		},
		{
			"a bard's total", bard(map[string]int{"climb_walls": 15, "read_languages": 6}),
			func(e *env) { expectLevel1Bard(e) },
			"skillPoints", 21, 20,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.rules.EXPECT().RaceAllows(tt.n.Race, tt.n.Class).Return(true, nil).Once()
			tt.expect(e) // the owners and sessions mocks fail any write
			_, err := e.svc.Create(context.Background(), ownerID, tt.n)
			require.ErrorIs(t, err, domain.ErrOverBudget)
			var be *domain.BudgetError
			require.ErrorAs(t, err, &be)
			assert.Equal(t, domain.BudgetError{Field: tt.field, Spent: tt.spent, Limit: tt.limit}, *be)
			assert.Zero(t, e.tx.n, "nothing written")
		})
	}
	t.Run("the draft lists the refusal and still reads the sheet", func(t *testing.T) {
		e := newEnv(t)
		points := map[string]int{"open_locks": 31}
		e.rules.EXPECT().RaceAllows("halfling", "thief").Return(true, nil).Once()
		expectLevel1Thief(e, points, 2) // the starting level, then the read
		got, err := e.svc.Draft(context.Background(), thief(points), nil)
		require.NoError(t, err)
		assert.Equal(t, []error{&domain.BudgetError{Field: "skillPoints.open_locks", Spent: 31, Limit: 30}}, got.Refusals)
		require.NotNil(t, got.Character)
		assert.Equal(t, 1, got.Character.Derived.TargetLevel)
	})
}

// Retiring a character retires it, then repairs the owner's current
// character (the first one left when it was the current one), in one
// transaction, and answers the current character after it.
func TestChange_RetireRepairsTheCurrentCharacter(t *testing.T) {
	e := newEnv(t)
	retired := e.owners.EXPECT().RetireCharacter(mock.Anything, ownerID, int64(5)).Return(nil).Once()
	repaired := e.owners.EXPECT().RepairCurrentCharacter(mock.Anything, ownerID).Return(nil).Once().NotBefore(retired)
	e.owners.EXPECT().CurrentCharacter(mock.Anything, ownerID).Return(3, nil).Once().NotBefore(repaired)
	e.sessions.EXPECT().ForgetUser(ownerID).Once()

	current, err := e.svc.Change(context.Background(), ownerID, 5, charactersvc.Change{Retired: ptr(true)})
	require.NoError(t, err)
	assert.Equal(t, int64(3), current)
	assert.Equal(t, 1, e.tx.n)
}

// A rename trims the name; it changes no current character.
func TestChange_Rename(t *testing.T) {
	e := newEnv(t)
	e.owners.EXPECT().RenameCharacter(mock.Anything, ownerID, int64(5), "Bram").Return(nil).Once()
	e.owners.EXPECT().CurrentCharacter(mock.Anything, ownerID).Return(3, nil).Once()

	current, err := e.svc.Change(context.Background(), ownerID, 5, charactersvc.Change{Name: ptr(" Bram ")})
	require.NoError(t, err)
	assert.Equal(t, int64(3), current)

	_, err = e.svc.Change(context.Background(), ownerID, 5, charactersvc.Change{Name: ptr(" ")})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "name", fe.Field)
}

// ADR 0033 step 14: a thief's or a bard's skills read the body armour it
// wears, through the Armour port, on every read: wearing, changing or taking
// off a piece (a logged equipment move) moves the percentages, and so does
// its undo. Another class's read never asks.
func TestGetCharacter_Step14_TheSkillsReadTheWornArmour(t *testing.T) {
	for _, tc := range []struct {
		name, class, worn, asked string
	}{
		{"a thief in elven chain", "thief", "elven_chain", "elven_chain"},
		{"a thief wearing none", "thief", "", "none"},
		{"a thief in metal armour reads the worst column", "thief", "metal", "padded_studded_leather"},
		{"a thief in an armour none may wear reads the worst column", "thief", "none_allowed", "padded_studded_leather"},
		{"a bard in metal armour", "bard", "metal", "metal"},
		{"a bard in leather", "bard", "leather", "leather"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			// The skills are what this test asks: none of newRules' defaults.
			e.rules.ExpectedCalls = nil
			withDefinitions(t, e.rules)
			e.rules.EXPECT().IntelligenceFor(mock.Anything).Return(int14, nil).Maybe()
			e.rules.EXPECT().MagicDefenceFor(mock.Anything).Return(0, nil).Maybe()
			e.rules.EXPECT().SpellImmunitiesFor(mock.Anything).Return(nil, nil).Maybe()
			e.rules.EXPECT().DexMods(mock.Anything).Return(0, 0, nil).Maybe()
			e.armour.columns[5] = tc.worn
			e.reader.EXPECT().GetCharacter(mock.Anything, int64(5)).Return(&charactersvc.Character{
				ID: 5, Class: tc.class, Race: "human", Dex: 12, RealizedLevel: 1,
			}, nil).Once()
			e.xp.EXPECT().Awards(mock.Anything, int64(5)).Return(nil, nil).Once()
			e.rules.EXPECT().SavesFor("rogue", 1).Return(rogue1to4, nil).Once()
			skills := []ruletablesvc.SkillPercent{{Skill: "climb_walls", Percent: 60}}
			if tc.class == "thief" {
				e.rules.EXPECT().RogueSkills(skillSet("thief"), "human", 12, tc.asked, map[string]int(nil)).Return(skills, nil).Once()
				e.rules.EXPECT().BackstabFor(1).Return(2, nil).Once()
			} else {
				e.rules.EXPECT().RogueSkills(skillSet("bard"), "human", 12, tc.asked, map[string]int(nil)).Return(skills, nil).Once()
				e.rules.EXPECT().SpellsPerDay(classDef("bard"), 1, mock.Anything).Return(map[int]int{}, nil).Once()
			}
			got, err := e.svc.GetCharacter(context.Background(), 5)
			require.NoError(t, err)
			require.Len(t, got.Derived.SkillSets, 1)
			assert.Equal(t, tc.asked, got.Derived.SkillSets[0].Armor)
			assert.Equal(t, 1, e.armour.reads)
		})
	}
	t.Run("a fighter's read asks for no armour", func(t *testing.T) {
		e := newEnv(t)
		e.reader.EXPECT().GetCharacter(mock.Anything, int64(5)).Return(&charactersvc.Character{
			ID: 5, Class: "fighter", Race: "human", Str: 12, RealizedLevel: 1,
		}, nil).Once()
		e.xp.EXPECT().Awards(mock.Anything, int64(5)).Return(nil, nil).Once()
		e.rules.EXPECT().SavesFor("warrior", 1).Return(ruletablesvc.SavingThrowRow{}, nil).Once()
		got, err := e.svc.GetCharacter(context.Background(), 5)
		require.NoError(t, err)
		assert.Empty(t, got.Derived.SkillSets)
		assert.Zero(t, e.armour.reads)
	})
}

// ADR 0033 step 14: the skill_armor entries logged before the entered armour
// was retired still pop and redo: the restorer writes the snapshot's armour
// back through the owner-less writer (into a column nothing reads).
func TestRestorers_Step14_AnOldSkillArmourEntryStillPops(t *testing.T) {
	e := newEnv(t)
	e.owners.EXPECT().RestoreSkillArmor(mock.Anything, int64(5), "none").Return(nil).Once()
	r := e.svc.Module().Restorer(charactersvc.ArmorStateKey)
	require.NotNil(t, r)
	require.NoError(t, r.Restore(context.Background(), 5, snaptest.Env(`{"armor":"none"}`)))
}

// Another user's character, a retired one and an unknown id are not found
// (ADR 0029 step 7.1): nothing else runs.
func TestChange_AnotherUsersCharacterIsNotFound(t *testing.T) {
	e := newEnv(t)
	e.owners.EXPECT().RetireCharacter(mock.Anything, ownerID, int64(9)).Return(domain.ErrCharacterNotFound).Once()
	_, err := e.svc.Change(context.Background(), ownerID, 9, charactersvc.Change{Retired: ptr(true)})
	require.ErrorIs(t, err, domain.ErrCharacterNotFound)
}

// The form offers the creatable classes, each with the races DMG Table 7
// admits, in the six races' order, and the ranges Create checks.
func TestCreationOptions_TheCreatableClassesAndTheirRaces(t *testing.T) {
	e := newEnv(t)
	for _, race := range cataloguetest.Definitions(t).RaceIDs() {
		e.rules.EXPECT().RaceAllows(race, "bard").Return(race == "human" || race == "half_elf", nil).Once()
		e.rules.EXPECT().RaceAllows(race, "fighter").Return(race != "elf", nil).Once()
		e.rules.EXPECT().RaceAllows(race, "mage").Return(race == "human" || race == "elf" || race == "half_elf", nil).Once()
		e.rules.EXPECT().RaceAllows(race, "cleric").Return(race != "elf", nil).Once()
		e.rules.EXPECT().RaceAllows(race, "thief").Return(true, nil).Once()
		e.rules.EXPECT().RaceAllows(race, "psionicist").Return(true, nil).Once()
	}
	expectOptionTables(e)
	got, err := e.svc.CreationOptions(context.Background())
	require.NoError(t, err)
	// The tables' figures are checked by the tests below; this one checks the classes and ranges.
	for i := range got.Classes {
		got.Classes[i].HitDie, got.Classes[i].LevelXP, got.Classes[i].Minimums, got.Classes[i].WeaponTypes = 0, nil, nil, nil
		got.Classes[i].Spellbook = nil // TestOptions_Step20_7_ReadMagicAndDetectMagicPreTickedForAMage
	}
	got.ScoreMethods, got.RaceAdjustments, got.RaceIdentity, got.WizardSpells = nil, nil, nil, nil
	assert.Equal(t, charactersvc.CreationOptions{
		// ADR 0040 step 8: every class of the catalogue, in its order, with its races in the
		// catalogue's race order.
		Classes: []charactersvc.ClassOption{
			{Class: "fighter", Races: []string{"dwarf", "gnome", "half_elf", "halfling", "human"}, Skills: []string{}, Alignments: ruletablesvc.Alignments},
			{Class: "mage", Races: []string{"elf", "half_elf", "human"}, Skills: []string{}, Alignments: ruletablesvc.Alignments},
			{Class: "cleric", Races: []string{"dwarf", "gnome", "half_elf", "halfling", "human"}, Skills: []string{}, Alignments: ruletablesvc.Alignments},
			// ADR 0030 step 17: the thief enters a point count per skill and its armour.
			{
				Class: "thief", Races: cataloguetest.Definitions(t).RaceIDs(), Skills: []string{"pick_pockets", "open_locks", "find_remove_traps", "move_silently", "hide_in_shadows", "detect_noise", "climb_walls", "read_languages"},
				Alignments: []string{"neutral_good", "chaotic_good", "lawful_neutral", "true_neutral", "chaotic_neutral", "lawful_evil", "neutral_evil", "chaotic_evil"},
			},
			{
				Class: "bard", Races: []string{"half_elf", "human"},
				Skills: []string{"climb_walls", "detect_noise", "pick_pockets", "read_languages"},
				// ADR 0032 step 2: the bard admits the five alignments with a neutral part.
				Alignments: []string{"neutral_good", "lawful_neutral", "true_neutral", "chaotic_neutral", "neutral_evil"},
			},
			// ADR 0043 step 1: creatable once its kinds are computed (step 3).
			{
				Class: "psionicist", Races: cataloguetest.Definitions(t).RaceIDs(), Skills: []string{},
				Alignments: []string{"lawful_good", "neutral_good", "lawful_neutral", "true_neutral", "lawful_evil", "neutral_evil"},
			},
		},
		MinAbility: 3, MaxAbility: 19, MinHPMax: 1, MaxHPMax: 999, MinXP: 0, MaxXP: charactersvc.MaxStartingXP,
		MinStrExceptional: 1, MaxStrExceptional: 100, MaxSkillPoints: 95, MaxNameLen: 40, MaxClasses: 3, MinLeftAtLevel: 2, MulticlassAnyRace: true, DualclassAnyRace: true, RestrictedRaces: []string{"dwarf", "elf", "gnome", "half_elf", "halfling"},
		// ADR 0042 step 2.1: the classes the catalogue marks `multiclass: false`.
		NoMultiClass: []string{"bard"}, DualClassOldPrime: 15, DualClassNewPrime: 17, Sexes: []string{"male", "female"},
	}, got)
}

// expectOptionTables answers the tables the creation options read beyond DMG Table 7: each race's
// Table 8 adjustments, each group's hit die and each class's XP table, every value distinct.
func expectOptionTables(e *env) {
	e.rules.EXPECT().RaceAdjustmentsFor(mock.Anything).RunAndReturn(func(race string) (map[string]int, error) {
		return map[string]int{"str": len(race)}, nil
	}).Maybe()
	dice := map[string]int{"rogue": 6, "warrior": 10, "wizard": 4, "priest": 8, "psionicist": 6}
	e.rules.EXPECT().HitDiceFor(mock.Anything).RunAndReturn(func(group string) (ruletablesvc.HitDiceRow, error) {
		return ruletablesvc.HitDiceRow{Group: group, Die: dice[group]}, nil
	}).Maybe()
}

// ADR 0034 step 4.1: each class carries its XP table read at levels 1 to 20, so a picked level
// is sent as the lowest XP of that level; and its hit die (step 3.1's class card).
func TestCreationOptions_Step4_1_EachClassCarriesItsLevelToXPTableAndHitDie(t *testing.T) {
	e := newEnv(t)
	e.rules.EXPECT().RaceAllows(mock.Anything, mock.Anything).Return(true, nil).Maybe()
	expectOptionTables(e)
	got, err := e.svc.CreationOptions(context.Background())
	require.NoError(t, err)
	byClass := map[string]charactersvc.ClassOption{}
	for _, c := range got.Classes {
		byClass[c.Class] = c
	}
	bard := byClass["bard"]
	require.Len(t, bard.LevelXP, 20)
	// The thresholds are the class definitions' (PHB Table 25 for the bard, Table 14 for the fighter).
	assert.Equal(t, charactersvc.LevelXP{Level: 1, XP: 0}, bard.LevelXP[0])
	assert.Equal(t, charactersvc.LevelXP{Level: 5, XP: 10_000}, bard.LevelXP[4])
	assert.Equal(t, charactersvc.LevelXP{Level: 20, XP: 2_200_000}, bard.LevelXP[19])
	assert.Equal(t, charactersvc.LevelXP{Level: 5, XP: 16_000}, byClass["fighter"].LevelXP[4], "each class its own table")
	assert.Equal(t, 6, bard.HitDie)
	assert.Equal(t, 10, byClass["fighter"].HitDie)
	assert.Equal(t, 4, byClass["mage"].HitDie)
	assert.Equal(t, 8, byClass["cleric"].HitDie)
}

// ADR 0034 steps 3.2 and 3.3: the options carry the six score methods in the book's order and
// PHB Table 8 for every race, in the races' order.
func TestCreationOptions_CarryTheScoreMethodsAndTheRaceAdjustments(t *testing.T) {
	e := newEnv(t)
	e.rules.EXPECT().RaceAllows(mock.Anything, mock.Anything).Return(true, nil).Maybe()
	expectOptionTables(e)
	got, err := e.svc.CreationOptions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, ruletablesvc.ScoreMethods, got.ScoreMethods)
	races := cataloguetest.Definitions(t).RaceIDs()
	require.Len(t, got.RaceAdjustments, len(races))
	for i, r := range got.RaceAdjustments {
		assert.Equal(t, races[i], r.Race)
		assert.Equal(t, map[string]int{"str": len(r.Race)}, r.Adjustments)
	}
}

// identity is the alignment and sex every test character enters (both required, ADR 0032 steps 2
// and 3), valid for each creatable class.
func identity() charactersvc.DetailsPatch {
	alignment, sex := "neutral_good", "male"
	return charactersvc.DetailsPatch{Alignment: &alignment, Sex: &sex}
}

func minimum(ability string, min int) ruletablesvc.AbilityMinimum {
	return ruletablesvc.AbilityMinimum{Ability: ability, Min: min}
}

// PHB Table 13: each creatable class's card carries its ability minimums, in ability order.
func TestCreationOptions_Table13_EachClassCarriesItsAbilityMinimums(t *testing.T) {
	e := newEnv(t)
	e.rules.EXPECT().RaceAllows(mock.Anything, mock.Anything).Return(true, nil).Maybe()
	expectOptionTables(e)
	got, err := e.svc.CreationOptions(context.Background())
	require.NoError(t, err)
	byClass := map[string][]ruletablesvc.AbilityMinimum{}
	for _, c := range got.Classes {
		byClass[c.Class] = c.Minimums
	}
	assert.Equal(t, map[string][]ruletablesvc.AbilityMinimum{
		"fighter": {minimum("str", 9)},
		"mage":    {minimum("int", 9)},
		"cleric":  {minimum("wis", 9)},
		"thief":   {minimum("dex", 9)},
		"bard":    {minimum("dex", 12), minimum("int", 13), minimum("cha", 15)},
		// ADR 0043 step 1.2: creatable once every kind it lists is computed (step 3).
		"psionicist": {minimum("con", 11), minimum("int", 12), minimum("wis", 15)},
	}, byClass)
}

func unmetOf(d *charactersvc.Draft) map[string][]string {
	out := map[string][]string{}
	for _, u := range d.UnmetMinimums {
		out[u.Class] = u.Abilities
	}
	return out
}

// allNumbers are the numbers of a creation body, every one blank in an empty draft.
var allNumbers = []string{"str", "dex", "con", "int", "wis", "cha", "hpMax", "xp"}

// PHB Table 13: a score under a class's minimum is unmet; at the minimum it is met; an ability
// with no score yet is not unmet. A draft with no race and no XP reads no sheet and asks no table.
func TestDraft_Table13_ScoresUnderAClassMinimumAreUnmet(t *testing.T) {
	e := newEnv(t)
	got, err := e.svc.Draft(context.Background(), charactersvc.NewCharacter{
		Class: "fighter", Str: 8, Dex: 11, Int: 13, Cha: 14, Wis: 9,
	}, []string{"con", "hpMax", "xp"})
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"fighter":    {"str"},        // 8 < 9
		"mage":       {},             // int 13 >= 9
		"cleric":     {},             // wis 9 is the minimum itself
		"thief":      {},             // dex 11 >= 9
		"bard":       {"dex", "cha"}, // dex 11 < 12, int 13 ok, cha 14 < 15
		"psionicist": {"wis"},        // con blank, int 13 >= 12, wis 9 < 15
	}, unmetOf(got))
	assert.Nil(t, got.Character, "no race and no XP: nothing to derive")

	none, err := e.svc.Draft(context.Background(), charactersvc.NewCharacter{Class: "bard"}, allNumbers)
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{"fighter": {}, "mage": {}, "cleric": {}, "thief": {}, "bard": {}, "psionicist": {}}, unmetOf(none))
}

// ADR 0034 amendment of 2026-10-20: an empty draft is no refusal; it lists every field the
// creation requires as missing, in the form's order, and reads no sheet.
func TestDraft_AnEmptyDraftIsAllMissingAndNoRefusal(t *testing.T) {
	e := newEnv(t)
	got, err := e.svc.Draft(context.Background(), charactersvc.NewCharacter{}, allNumbers)
	require.NoError(t, err)
	assert.Empty(t, got.Refusals)
	assert.Equal(t, []string{"name", "class", "race", "str", "dex", "con", "int", "wis", "cha", "hpMax", "xp", "alignment", "sex"}, got.Missing)
	assert.Nil(t, got.Character)
}

// A value the creation refuses is one of the draft's refusals, with the creation's field and
// code, and the draft goes on reading the other fields: an XP outside the range and a Strength of
// 25 are both listed, and with the XP refused no sheet is read (no table is asked).
func TestDraft_EveryRefusalIsListedAndARefusedXPReadsNoSheet(t *testing.T) {
	e := newEnv(t)
	n := lyra()
	n.Str, n.XP = 25, -5
	e.rules.EXPECT().RaceAllows("half_elf", "bard").Return(true, nil).Once()
	got, err := e.svc.Draft(context.Background(), n, nil)
	require.NoError(t, err)
	assert.Equal(t, []error{
		&domain.FieldError{Field: "str", Message: charactersvc.CodeOutOfRange},
		&domain.FieldError{Field: "xp", Message: charactersvc.CodeOutOfRange},
	}, got.Refusals)
	assert.Nil(t, got.Character)
}
