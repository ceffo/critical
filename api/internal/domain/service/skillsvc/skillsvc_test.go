package skillsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

const charID int64 = 1

type countingTx struct{ calls int }

func (c *countingTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	c.calls++
	return fn(ctx)
}

type fixture struct {
	svc     skillsvc.SVC
	chars   *mocks.MockCharacters
	abils   *mocks.MockAbilities
	dice    *mocks.MockDice
	log     *mocks.MockLog
	points  *mocks.MockPoints
	tx      *countingTx
	entries []actionlog.NewEntry
}

// The sheet's scores (STR 12, DEX 17, CON 16, INT 14, WIS 8, CHA 16) and
// two of the bard skills its character service derives (ADR 0030 step 18).
// Its picks are derived by the character service (ADR 0037 steps 3 to 5):
// each with its catalogue ability and modifier and its target.
func bard() *charactersvc.Character {
	c := &charactersvc.Character{
		ID: charID, Class: "bard", Str: 12, Dex: 17, Con: 16, Int: 14, Wis: 8, Cha: 16,
		Derived: charactersvc.Derived{BardSkills: bardSkills},
	}
	slots := skills.ProficiencySlots{Picks: append([]skills.PickFigures{}, catalog...)}
	slots.SetTargets(func(a string) (int, bool) { return c.Score(charactersvc.Ability(a)) })
	c.Derived.Proficiencies = &slots
	return c
}

var catalog = []skills.PickFigures{
	{ID: "musical_instrument", Ability: string(skillsvc.AbilityDex), Modifier: -1, Slots: 1, Cost: 1, Known: true}, // 17 - 1 = 16
	{ID: "riding_land", Ability: string(skillsvc.AbilityWis), Modifier: 3, Slots: 1, Cost: 1, Known: true},         // 8 + 3 = 11
	{ID: "acrobatics", Ability: string(skillsvc.AbilityDex), Modifier: 5, Slots: 1, Cost: 1, Known: true},          // 17 + 5 = 22
	// A proficiency with no check, and an id the catalogue does not hold: never rolled.
	{ID: "blind_fighting", Slots: 2, Cost: 2, Known: true},
	{ID: "old_skill", Slots: 1, Cost: 1},
}

// testBook is a catalogue of the picks the save tests make: Tumbling (rogue, 1 slot), Tracking
// (warrior, 2 slots), Healing (priest, 2 slots), Languages, Modern, with Table 34's rogue row and
// the bard's Table 38 groups.
func testBook(t *testing.T) *ruletablesvc.Proficiencies {
	t.Helper()
	var cross []ruletablesvc.Crossover
	for _, c := range ruletablesvc.Classes {
		cross = append(cross, ruletablesvc.Crossover{Class: c, Groups: []string{"general"}})
	}
	cross[4] = ruletablesvc.Crossover{Class: "bard", Groups: []string{"rogue", "warrior", "wizard", "general"}}
	var slots []ruletablesvc.SlotRow
	for _, g := range ruletablesvc.Groups {
		slots = append(slots, ruletablesvc.SlotRow{Group: g, WeaponInitial: 2, WeaponLevels: 4, Penalty: -3, NonweaponInitial: 3, NonweaponLevels: 4})
	}
	b, err := ruletablesvc.NewProficiencies([]ruletablesvc.Proficiency{
		{ID: "tumbling", Ability: "dex", Slots: 1, Groups: []string{"rogue"}},
		{ID: "tracking", Ability: "wis", Slots: 2, Groups: []string{"warrior"}},
		{ID: "healing", Ability: "wis", Modifier: -2, Slots: 2, Groups: []string{"priest"}},
		{ID: "languages_modern", Ability: "int", Slots: 1, Groups: []string{"general"}},
	}, cross, slots)
	require.NoError(t, err)
	return b
}

var bardSkills = []ruletablesvc.SkillPercent{
	{Skill: "climb_walls", Base: 50, Points: 35, Percent: 85},
	{Skill: "pick_pockets", Base: 10, Points: 25, Race: 10, Dexterity: 5, Percent: 50},
}

// fakeTables are PHB Tables 26 to 29 and 33 as the points' preview asks them: each skill at its
// points, the terms it was asked for recorded.
type fakeTables struct {
	race, armor string
	dex         int
	points      map[string]int
}

func (f *fakeTables) skills(race string, dex int, armor string, points map[string]int, ids []string) []ruletablesvc.SkillPercent {
	f.race, f.dex, f.armor, f.points = race, dex, armor, points
	out := make([]ruletablesvc.SkillPercent, 0, len(ids))
	for _, id := range ids {
		out = append(out, ruletablesvc.SkillPercent{Skill: id, Points: points[id], Percent: points[id]})
	}
	return out
}

func (f *fakeTables) ThiefSkills(race string, dex int, armor string, points map[string]int) ([]ruletablesvc.SkillPercent, error) {
	return f.skills(race, dex, armor, points, ruletablesvc.SkillsOf("thief")), nil
}

func (f *fakeTables) BardSkills(race string, dex int, armor string, points map[string]int) ([]ruletablesvc.SkillPercent, error) {
	return f.skills(race, dex, armor, points, ruletablesvc.SkillsOf("bard")), nil
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		chars: mocks.NewMockCharacters(t), abils: mocks.NewMockAbilities(t), dice: mocks.NewMockDice(t),
		log: mocks.NewMockLog(t), points: mocks.NewMockPoints(t), tx: &countingTx{},
	}
	svc, err := skillsvc.New(skillsvc.Deps{Characters: f.chars, Abilities: f.abils, Dice: f.dice, Log: f.log, Points: f.points, Picks: mocks.NewMockPicks(t), Book: testBook(t), Tables: &fakeTables{}, Tx: f.tx})
	require.NoError(t, err)
	f.svc = svc
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(bard(), nil).Maybe()
	return f
}

// records makes the log accept entries (ids 100, 101, …) and keeps them.
func (f *fixture) records() {
	f.log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, e actionlog.NewEntry) (int64, error) {
			f.entries = append(f.entries, e)
			return int64(99 + len(f.entries)), nil
		})
}

func (f *fixture) roll(t *testing.T, id string) *skillsvc.CheckResult {
	t.Helper()
	r, err := f.svc.Roll(context.Background(), charID, id, skillsvc.Adjustment{})
	require.NoError(t, err)
	return r
}

// ADR 0014 step 1: a proficiency check is one d20, a success when the roll
// is at or under the ability score plus the modifier.
func TestSkillCheck_Step1_ProficiencyD20AtOrUnderTarget(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		d20     int
		score   int
		mod     int
		target  int
		success bool
	}{
		{"at the target succeeds", "musical_instrument", 16, 17, -1, 16, true},
		{"one over the target fails", "musical_instrument", 17, 17, -1, 16, false},
		{"natural 1 under the target", "musical_instrument", 1, 17, -1, 16, true},
		{"positive modifier added", "riding_land", 11, 8, 3, 11, true},
		{"positive modifier, one over", "riding_land", 12, 8, 3, 11, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.records()
			f.dice.EXPECT().RollDie(20).Return(c.d20).Once() // any d100 or second roll fails the mock
			r := f.roll(t, c.id)
			f.dice.AssertNumberOfCalls(t, "RollDie", 1)
			assert.Equal(t, &skillsvc.CheckResult{
				ID: c.id, Kind: skillsvc.KindProficiency, Die: 20, Roll: c.d20, Target: c.target, AdjustedTarget: c.target,
				Success: c.success, RolledSuccess: c.success, Ability: catalogAbility(c.id), AbilityScore: c.score, Modifier: c.mod, LogEntryID: 100,
			}, r)
		})
	}
}

func catalogAbility(id string) skillsvc.Ability {
	for _, p := range catalog {
		if p.ID == id {
			return skillsvc.Ability(p.Ability)
		}
	}
	return ""
}

// ADR 0014 step 2: a natural 20 fails a proficiency check even when the
// target is 20 or more; a 19 still succeeds against the same target.
func TestSkillCheck_Step2_NaturalTwentyAlwaysFails(t *testing.T) {
	for _, c := range []struct {
		d20     int
		success bool
	}{{20, false}, {19, true}} {
		f := newFixture(t)
		f.records()
		f.dice.EXPECT().RollDie(20).Return(c.d20).Once()
		r := f.roll(t, "acrobatics")
		f.dice.AssertNumberOfCalls(t, "RollDie", 1)
		assert.Equal(t, 22, r.Target)
		assert.Equal(t, c.success, r.Success, "d20 %d against 22", c.d20)
	}
	assert.False(t, skillsvc.ProficiencySuccess(20, 20))
	assert.True(t, skillsvc.BardSkillSuccess(20, 85), "the natural 20 rule is for the d20 only")
}

// ADR 0014 step 3: a bard skill check is one d100, a success when the roll is
// at or under the percentage.
func TestSkillCheck_Step3_BardSkillD100AtOrUnderPercent(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		d100    int
		percent int
		success bool
	}{
		{"at the percentage succeeds", "climb_walls", 85, 85, true},
		{"one over fails", "climb_walls", 86, 85, false},
		{"20 is plain on a d100", "climb_walls", 20, 85, true},
		{"100 fails", "climb_walls", 100, 85, false},
		{"other skill, its own percentage", "pick_pockets", 51, 50, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.records()
			f.dice.EXPECT().RollDie(100).Return(c.d100).Once() // any d20 or second roll fails the mock
			r := f.roll(t, c.id)
			f.dice.AssertNumberOfCalls(t, "RollDie", 1)
			assert.Equal(t, &skillsvc.CheckResult{
				ID: c.id, Kind: skillsvc.KindBardSkill, Die: 100, Roll: c.d100, Target: c.percent, AdjustedTarget: c.percent,
				Success: c.success, RolledSuccess: c.success, LogEntryID: 100,
			}, r)
		})
	}
}

// ADR 0014 step 4: each check is one skill_check entry, structured payload
// and no summary, no snapshots (it changes no state, so popping it restores
// nothing), recorded in the check's one transaction. Popping on the real log:
// composition TestSkillCheck_UndoPopsTheCheck.
func TestSkillCheck_Step4_OneLoggedEntryPerCheck(t *testing.T) {
	f := newFixture(t)
	f.records()
	f.dice.EXPECT().RollDie(20).Return(7).Once()
	f.dice.EXPECT().RollDie(100).Return(93).Once()

	prof := f.roll(t, "musical_instrument")
	skill := f.roll(t, "climb_walls")

	assert.Equal(t, 2, f.tx.calls, "one transaction per check")
	require.Len(t, f.entries, 2)
	assert.Equal(t, int64(100), prof.LogEntryID)
	assert.Equal(t, int64(101), skill.LogEntryID)
	for _, e := range f.entries {
		assert.Equal(t, actionlog.KindSkillCheck, e.Kind)
		assert.Empty(t, e.Summary, "the UI words the check")
		assert.Empty(t, e.Snapshots)
		assert.Zero(t, e.HPDelta)
		assert.Equal(t, charID, e.CharacterID)
	}
	assert.JSONEq(t, `{"id":"musical_instrument","kind":"proficiency","die":20,"roll":7,"target":16,
		"adjustment":0,"adjustedTarget":16,"success":true,"rolledSuccess":true,
		"ability":"dex","abilityScore":17,"modifier":-1}`, string(f.entries[0].Payload))
	assert.JSONEq(t, `{"id":"climb_walls","kind":"bardSkill","die":100,"roll":93,"target":85,
		"adjustment":0,"adjustedTarget":85,"success":false,"rolledSuccess":false,"modifier":0}`,
		string(f.entries[1].Payload))
	assert.True(t, actionlog.IsRoll(actionlog.KindSkillCheck), "a check replaces the result card")
}

func TestSkillCheck_UnknownIDRollsNothing(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Roll(context.Background(), charID, "juggling", skillsvc.Adjustment{})
	require.ErrorIs(t, err, domain.ErrNotFound)
	f.dice.AssertNotCalled(t, "RollDie", mock.Anything)
	f.log.AssertNotCalled(t, "Record", mock.Anything, mock.Anything)
}

func TestSkillCheck_RecordFailureFailsTheCheck(t *testing.T) {
	f := newFixture(t)
	f.dice.EXPECT().RollDie(20).Return(3).Once()
	boom := errors.New("boom")
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(0, boom)
	_, err := f.svc.Roll(context.Background(), charID, "musical_instrument", skillsvc.Adjustment{})
	require.ErrorIs(t, err, boom)
}

func checkEntry(t *testing.T, id int64, payload string) actionlog.Entry {
	t.Helper()
	require.True(t, json.Valid([]byte(payload)))
	return actionlog.Entry{ID: id, Kind: actionlog.KindSkillCheck, Payload: json.RawMessage(payload)}
}

// List: each proficiency's target is computed on the server; the last check
// of each skill is its newest non-voided entry. These entries were logged
// before adjustments: they read as unadjusted.
func TestList_TargetsAndLastCheckPerSkill(t *testing.T) {
	f := newFixture(t)
	f.log.EXPECT().ActiveEntries(mock.Anything, charID, actionlog.KindSkillCheck, skillsvc.LastLookup).Return([]actionlog.Entry{
		checkEntry(t, 30, `{"id":"climb_walls","kind":"bardSkill","die":100,"roll":12,"target":85,"success":true}`),
		checkEntry(t, 20, `{"id":"musical_instrument","kind":"proficiency","die":20,"roll":18,"target":16,"success":false}`),
		checkEntry(t, 10, `{"id":"climb_walls","kind":"bardSkill","die":100,"roll":99,"target":85,"success":false}`),
	}, nil)

	got, err := f.svc.List(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, &skillsvc.Skills{
		Proficiencies: []skillsvc.Proficiency{
			{
				ID: "musical_instrument", Ability: skillsvc.AbilityDex, AbilityScore: 17, Modifier: -1, Target: 16,
				Last: &skillsvc.LastCheck{EntryID: 20, Roll: 18, Target: 16, AdjustedTarget: 16, Success: false},
			},
			{ID: "riding_land", Ability: skillsvc.AbilityWis, AbilityScore: 8, Modifier: 3, Target: 11},
			{ID: "acrobatics", Ability: skillsvc.AbilityDex, AbilityScore: 17, Modifier: 5, Target: 22},
		},
		BardSkills: []skillsvc.BardSkill{
			{SkillPercent: bardSkills[0], Last: &skillsvc.LastCheck{EntryID: 30, Roll: 12, Target: 85, AdjustedTarget: 85, Success: true}},
			{SkillPercent: bardSkills[1]},
		},
		ThiefSkills: []skillsvc.ThiefSkill{},
	}, got)
}
