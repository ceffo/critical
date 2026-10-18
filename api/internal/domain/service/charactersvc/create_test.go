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

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

var ownerID = uuid.MustParse("0b6a8c7e-5f1d-4a59-9a3e-3c2d1e0f4a11")

// lyra is a valid new character: a level 1 half-elf bard.
func lyra() charactersvc.NewCharacter {
	return charactersvc.NewCharacter{
		Name: "  Lyra ", Class: "bard", Race: "half_elf",
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
	e.rules.EXPECT().LevelFor("bard", 0).Return(1, nil).Once()
	e.rules.EXPECT().XPThreshold("bard", 2).Return(1_250, nil).Once()
	e.rules.EXPECT().THAC0For("rogue", 1).Return(20, nil).Once()
	e.rules.EXPECT().SavesFor("rogue", 1).Return(rogue1to4, nil).Once()
	e.rules.EXPECT().BardSpellsPerDay(1).Return(map[int]int{}, nil).Once()
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
		Sheet:   charactersvc.Sheet{Level: 1, THAC0: 20, Saves: rogue1to4Saves},
		Details: charactersvc.Details{Languages: []string{}},
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
			Name: "Brom", Class: "fighter", Race: race,
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
			e.rules.EXPECT().LevelFor("fighter", 0).Return(1, nil).Once()
			e.rules.EXPECT().XPThreshold("fighter", 2).Return(2_000, nil).Once()
			e.rules.EXPECT().THAC0For("warrior", 1).Return(20, nil).Once()
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

// ADR 0030 steps 1 and 17: a thief enters its armour (one of the four) and
// its points (each a thieving skill, 0 to 95); any other class enters
// neither. Refused before anything is written, the field named; the points
// are stored without the zeros, with the armour.
func TestCreate_Step17_ThiefSkillPointsAndArmour(t *testing.T) {
	thief := func(armor string, points map[string]int) charactersvc.NewCharacter {
		return charactersvc.NewCharacter{
			Name: "Vex", Class: "thief", Race: "halfling", Str: 10, Dex: 17, Con: 12, Int: 11, Wis: 9, Cha: 10, HPMax: 5,
			SkillArmor: armor, SkillPoints: points,
		}
	}
	bardArmor, bardPoints := lyra(), lyra()
	bardArmor.SkillArmor = "none"
	bardPoints.SkillPoints = map[string]int{"climb_walls": 10}
	for _, tt := range []struct {
		name        string
		n           charactersvc.NewCharacter
		field, code string
	}{
		{"no armour", thief("", nil), "skillArmor", "empty"},
		{"an armour Table 29 lacks", thief("plate", nil), "skillArmor", "unknown"},
		{"an unknown skill", thief("none", map[string]int{"backstab": 10}), "skillPoints.backstab", "unknown"},
		{"negative points", thief("none", map[string]int{"open_locks": -1}), "skillPoints.open_locks", "out_of_range"},
		{"more than 95 points", thief("none", map[string]int{"open_locks": 96}), "skillPoints.open_locks", "out_of_range"},
		{"a bard's armour", bardArmor, "skillArmor", "not_allowed"},
		{"a bard's points", bardPoints, "skillPoints", "not_allowed"},
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
		e.rules.EXPECT().LevelFor("thief", 0).Return(1, nil).Once()
		e.rules.EXPECT().XPThreshold("thief", 2).Return(1_250, nil).Once()
		e.rules.EXPECT().THAC0For("rogue", 1).Return(20, nil).Once()
		e.rules.EXPECT().SavesFor("rogue", 1).Return(rogue1to4, nil).Once()
		e.rules.EXPECT().ConSaveBonusFor(12).Return(0, nil).Once()
		want := map[string]int{"open_locks": 30, "pick_pockets": 95}
		e.rules.EXPECT().ThiefSkills("halfling", 17, "padded_studded_leather", want).Return(nil, nil).Once()
		e.rules.EXPECT().BackstabFor(1).Return(2, nil).Once()
		e.owners.EXPECT().CreateCharacter(mock.Anything, ownerID, mock.MatchedBy(func(c charactersvc.Character) bool {
			return c.Class == "thief" && c.SkillArmor == "padded_studded_leather" && assert.ObjectsAreEqual(want, c.SkillPoints)
		})).Return(6, nil).Once()
		e.owners.EXPECT().SetCurrentCharacter(mock.Anything, ownerID, int64(6)).Return(nil).Once()
		e.sessions.EXPECT().ForgetUser(ownerID).Once()
		_, err := e.svc.Create(context.Background(), ownerID, thief("padded_studded_leather", map[string]int{
			"open_locks": 30, "pick_pockets": 95, "climb_walls": 0,
		}))
		require.NoError(t, err)
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

// ADR 0030 step 1 after creation: a thief's skill armour is set to one of
// Table 29's armours; the thief skills are derived on read, so they follow.
func TestChange_ASkillArmourIsSetOnAThief(t *testing.T) {
	e := newEnv(t)
	set := e.owners.EXPECT().SetSkillArmor(mock.Anything, ownerID, int64(5), "elven_chain").Return(nil).Once()
	e.reader.EXPECT().GetCharacter(mock.Anything, int64(5)).Return(&charactersvc.Character{Class: "thief"}, nil).Once().NotBefore(set)
	e.owners.EXPECT().CurrentCharacter(mock.Anything, ownerID).Return(3, nil).Once()

	current, err := e.svc.Change(context.Background(), ownerID, 5, charactersvc.Change{SkillArmor: ptr("elven_chain")})
	require.NoError(t, err)
	assert.Equal(t, int64(3), current)
}

// An armour that is empty or none of Table 29's is refused before anything
// runs; a class without thieving skills has no armour to change (the write
// is rolled back with the refusal).
func TestChange_ASkillArmourIsRefusedWhenInvalid(t *testing.T) {
	e := newEnv(t)
	for armor, code := range map[string]string{"": "empty", "plate": "unknown"} {
		_, err := e.svc.Change(context.Background(), ownerID, 5, charactersvc.Change{SkillArmor: ptr(armor)})
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "skillArmor", fe.Field)
		assert.Equal(t, code, fe.Message)
	}

	e.owners.EXPECT().SetSkillArmor(mock.Anything, ownerID, int64(5), "leather").Return(nil).Once()
	e.reader.EXPECT().GetCharacter(mock.Anything, int64(5)).Return(&charactersvc.Character{Class: "bard"}, nil).Once()
	_, err := e.svc.Change(context.Background(), ownerID, 5, charactersvc.Change{SkillArmor: ptr("leather")})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "not_allowed", fe.Message)
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
	for _, race := range charactersvc.Races {
		e.rules.EXPECT().RaceAllows(race, "bard").Return(race == "human" || race == "half_elf", nil).Once()
		e.rules.EXPECT().RaceAllows(race, "fighter").Return(race != "elf", nil).Once()
		e.rules.EXPECT().RaceAllows(race, "mage").Return(race == "human" || race == "elf" || race == "half_elf", nil).Once()
		e.rules.EXPECT().RaceAllows(race, "cleric").Return(race != "elf", nil).Once()
		e.rules.EXPECT().RaceAllows(race, "thief").Return(true, nil).Once()
	}
	got, err := e.svc.CreationOptions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, charactersvc.CreationOptions{
		Classes: []charactersvc.ClassOption{
			{Class: "bard", Races: []string{"human", "half_elf"}, ExceptionalStrengthRaces: []string{}, Skills: []string{}, SkillArmors: []string{}},
			{Class: "fighter", Races: []string{"human", "dwarf", "gnome", "half_elf", "halfling"}, ExceptionalStrengthRaces: []string{"human", "dwarf", "gnome", "half_elf"}, Skills: []string{}, SkillArmors: []string{}},
			{Class: "mage", Races: []string{"human", "elf", "half_elf"}, ExceptionalStrengthRaces: []string{}, Skills: []string{}, SkillArmors: []string{}},
			{Class: "cleric", Races: []string{"human", "dwarf", "gnome", "half_elf", "halfling"}, ExceptionalStrengthRaces: []string{}, Skills: []string{}, SkillArmors: []string{}},
			// ADR 0030 step 17: the thief enters a point count per skill and its armour.
			{Class: "thief", Races: charactersvc.Races, ExceptionalStrengthRaces: []string{}, Skills: ruletablesvc.ThiefSkills, SkillArmors: ruletablesvc.ThiefArmors},
		},
		MinAbility: 3, MaxAbility: 19, MinHPMax: 1, MaxHPMax: 999, MinXP: 0, MaxXP: charactersvc.MaxStartingXP,
		MinStrExceptional: 1, MaxStrExceptional: 100, MaxSkillPoints: 95, MaxNameLen: 40,
	}, got)
}
