package skillsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	rulesetmocks "github.com/ceffo/critical/api/internal/domain/ruleset/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc/viewtest"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

type picksFixture struct {
	*fixture
	picks *mocks.MockPicks
}

// newPicksFixture is a bard of Intelligence 9 and Dexterity 15 at realized level 1 (testBook's
// rogue row: 3 + 0 + 2 = 5 non-weapon slots) storing stored, with no language catalogue; every
// View answers it (the read after a save included).
func newPicksFixture(t *testing.T, stored skills.Picks) *picksFixture {
	t.Helper()
	c := viewtest.Single("bard", 1, viewtest.WithIdentity(charID, "Test", ""), viewtest.WithScores(0, 15, 0, 9, 0, 0))
	c.ProficiencyPicks = stored
	return newSheetFixture(t, c, nil)
}

// newSheetFixture is the service over c, read by every View, and the language catalogue
// languages (nil: none).
func newSheetFixture(t *testing.T, c charactersvc.View, languages skills.LanguageBook) *picksFixture {
	t.Helper()
	f := &picksFixture{fixture: &fixture{
		chars: mocks.NewMockCharacters(t), abils: mocks.NewMockAbilities(t), dice: mocks.NewMockDice(t),
		log: mocks.NewMockLog(t), points: mocks.NewMockPoints(t), tx: &countingTx{},
		rulesets: rulesetmocks.NewMockResolver(t), rs: ruleset.Default(),
	}, picks: mocks.NewMockPicks(t)}
	f.rulesets.EXPECT().ForCharacter(mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, int64) (ruleset.Ruleset, error) { return f.rs, nil }).Maybe()
	svc, err := skillsvc.New(skillsvc.Deps{
		Characters: f.chars, Abilities: f.abils, Dice: f.dice, Log: f.log, Points: f.points, Picks: f.picks, Buffs: mocks.NewMockBuffs(t), Rulesets: f.rulesets,
		Book: testBook(t), Languages: languages, Tables: &fakeTables{}, Tx: f.tx,
	})
	require.NoError(t, err)
	f.svc = svc
	f.chars.EXPECT().View(mock.Anything, charID).Return(c, nil).Maybe()
	return f
}

// nonweapon are non-weapon picks and no weapon pick.
func nonweapon(ps ...skills.Pick) skills.Picks {
	return skills.Picks{Nonweapon: append([]skills.Pick{}, ps...), Weapon: []skills.WeaponPick{}}
}

// withWeapons is ps with the weapon picks of types, 1 slot each.
func withWeapons(ps skills.Picks, types ...string) skills.Picks {
	ps.Weapon = []skills.WeaponPick{}
	for _, t := range types {
		ps.Weapon = append(ps.Weapon, skills.WeaponPick{Type: t, Slots: 1})
	}
	return ps
}

func pk(id string, slots int) skills.Pick { return skills.Pick{ID: id, Slots: slots} }

// ADR 0037 step 13: a save that changes the picks writes them and records one proficiencies entry
// (the picks before and after, snapshotted under "proficiencies") in one transaction.
func TestSetPicks_Step13_LogsOneMoveWithASnapshot(t *testing.T) {
	f := newPicksFixture(t, nonweapon(pk("tumbling", 1)))
	f.records()
	next := nonweapon(pk("tumbling", 2), pk("tracking", 2))
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, next).Return(nil).Once()

	_, err := f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Picks: next})
	require.NoError(t, err)
	assert.Equal(t, 1, f.tx.calls, "one transaction")
	require.Len(t, f.entries, 1, "one log entry")
	e := f.entries[0]
	assert.Equal(t, actionlog.KindProficiencies, e.Kind)
	assert.JSONEq(t, `{"before":{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[],"languages":[]},
		"after":{"nonweapon":[{"id":"tumbling","slots":2},{"id":"tracking","slots":2}],"weapon":[],"languages":[]}}`, string(e.Payload))
	require.Len(t, e.Snapshots, 1)
	assert.Equal(t, skillsvc.PicksStateKey, e.Snapshots[0].Key)
	assert.JSONEq(t, `{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[],"languages":[]}`, snaptest.State(t, e.Snapshots[0].Before))
	assert.JSONEq(t, `{"nonweapon":[{"id":"tumbling","slots":2},{"id":"tracking","slots":2}],"weapon":[],"languages":[]}`, snaptest.State(t, e.Snapshots[0].After))
}

// ADR 0037 steps 7 and 13: a save of the weapon picks alone (no non-weapon list) keeps the
// non-weapon picks, writes both pools and records one proficiencies entry whose before and after
// carry the weapon picks.
func TestSetPicks_Step13_WeaponSaveLogsOneMove(t *testing.T) {
	stored := withWeapons(nonweapon(pk("tumbling", 1)), "dagger")
	f := newPicksFixture(t, stored)
	f.records()
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, withWeapons(nonweapon(pk("tumbling", 1)), "dagger", "short_bow")).Return(nil).Once()

	_, err := f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Picks: skills.Picks{Weapon: withWeapons(skills.Picks{}, "dagger", "short_bow").Weapon}})
	require.NoError(t, err)
	assert.Equal(t, 1, f.tx.calls, "one transaction")
	require.Len(t, f.entries, 1, "one log entry")
	e := f.entries[0]
	assert.Equal(t, actionlog.KindProficiencies, e.Kind)
	assert.JSONEq(t, `{"before":{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[{"type":"dagger","slots":1}],"languages":[]},
		"after":{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[{"type":"dagger","slots":1},{"type":"short_bow","slots":1}],"languages":[]}}`, string(e.Payload))
	require.Len(t, e.Snapshots, 1)
	assert.JSONEq(t, `{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[{"type":"dagger","slots":1}],"languages":[]}`, snaptest.State(t, e.Snapshots[0].Before))
	assert.JSONEq(t, `{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[{"type":"dagger","slots":1},{"type":"short_bow","slots":1}],"languages":[]}`,
		snaptest.State(t, e.Snapshots[0].After))
}

// ADR 0019 on step 13 with the weapon pool: a snapshot with weapon picks writes them back; a
// snapshot logged before the weapon pool (no "weapon" key) restores the non-weapon picks and
// leaves the stored weapon picks, so undoing an old entry never drops the migration's grants.
func TestSetPicks_Step13_RestorerAndTheWeaponPool(t *testing.T) {
	f := newPicksFixture(t, withWeapons(nonweapon(pk("tumbling", 2)), "dagger", "dart"))
	r := f.svc.Module().Restorer(skillsvc.PicksStateKey)
	require.NotNil(t, r)
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, withWeapons(nonweapon(pk("tumbling", 1)), "dagger", "dart")).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"nonweapon":[{"id":"tumbling","slots":1}]}`)))
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, withWeapons(nonweapon(pk("tumbling", 1)), "sling")).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[{"type":"sling","slots":1}]}`)))
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, nonweapon(pk("tumbling", 1))).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"nonweapon":[{"id":"tumbling","slots":1}],"weapon":[]}`)))
}

// ADR 0037 step 13: the same picks as stored write and log nothing.
func TestSetPicks_Step13_NoChangeLogsNothing(t *testing.T) {
	f := newPicksFixture(t, nonweapon(pk("tumbling", 1))) // the picks and log mocks fail any write or record
	_, err := f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Picks: nonweapon(pk("tumbling", 1))})
	require.NoError(t, err)
}

// ADR 0037 step 13: a save that only reorders the stored picks writes and logs nothing (the
// journal would show an entry with no change).
func TestSetPicks_Step13_ReorderLogsNothing(t *testing.T) {
	f := newPicksFixture(t, nonweapon(pk("tumbling", 1), pk("tracking", 2))) // the picks and log mocks fail any write or record
	_, err := f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Picks: nonweapon(pk("tracking", 2), pk("tumbling", 1))})
	require.NoError(t, err)
}

// ADR 0019 on step 13: the restorer writes a snapshot's picks back (undo the before, redo the
// after), an empty list as none.
func TestSetPicks_Step13_RestorerWritesThePicksBack(t *testing.T) {
	f := newPicksFixture(t, nonweapon())
	r := f.svc.Module().Restorer(skillsvc.PicksStateKey)
	require.NotNil(t, r)
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, nonweapon(pk("tumbling", 1))).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"nonweapon":[{"id":"tumbling","slots":1}]}`)))
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, nonweapon()).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"nonweapon":[]}`)))
}

// ADR 0037 step 12: a refused save (unknown id, slots under the cost, a duplicate, Languages,
// Modern on its own) and step 10's budget refusal write and log nothing.
func TestSetPicks_Step12_RefusalsWriteNothing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stored skills.Picks
		save   skills.Picks
		field  string
		budget *domain.BudgetError
	}{
		{"unknown id", nonweapon(), nonweapon(pk("juggling", 1)), "nonweapon.juggling", nil},
		{"slots below the cost", nonweapon(), nonweapon(pk("tracking", 1)), "nonweapon.tracking", nil},
		{"duplicate", nonweapon(), nonweapon(pk("tumbling", 1), pk("tumbling", 1)), "nonweapon.tumbling", nil},
		{"languages modern", nonweapon(), nonweapon(pk("languages_modern", 1)), "nonweapon.languages_modern", nil},
		{"a new overspend", nonweapon(pk("tumbling", 5)), nonweapon(pk("tumbling", 6)), "", &domain.BudgetError{Field: "nonweapon", Spent: 6, Limit: 5}},
		{"an existing overspend increased", nonweapon(pk("tumbling", 7)), nonweapon(pk("tumbling", 7), pk("tracking", 2)), "", &domain.BudgetError{Field: "nonweapon", Spent: 9, Limit: 5}},
		{"an unknown weapon type", nonweapon(), withWeapons(nonweapon(), "katana"), "weapon.katana", nil},
		{"a duplicate weapon type", nonweapon(), withWeapons(nonweapon(), "dagger", "dagger"), "weapon.dagger", nil},
		{"a new weapon overspend (2 slots)", withWeapons(nonweapon(), "dagger", "dart"), withWeapons(nonweapon(), "dagger", "dart", "sling"), "", &domain.BudgetError{Field: "weapon", Spent: 3, Limit: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPicksFixture(t, tc.stored) // the picks and log mocks fail any write or record
			_, err := f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Picks: tc.save})
			if tc.budget != nil {
				require.ErrorIs(t, err, domain.ErrOverBudget)
				var be *domain.BudgetError
				require.ErrorAs(t, err, &be)
				assert.Equal(t, *tc.budget, *be)
				return
			}
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, tc.field, fe.Field)
		})
	}
}

// ADR 0037 step 10: an existing overspend reduced or kept is stored (the warning stays on the
// read); slots left to spend are never refused.
func TestBudget_Step10_OverAndShrinkingStored(t *testing.T) {
	for _, tc := range []struct {
		name         string
		stored, save skills.Picks
	}{
		{"an overspend reduced, still over", nonweapon(pk("tumbling", 8)), nonweapon(pk("tumbling", 6))},
		{"an overspend kept, moved between picks", nonweapon(pk("tumbling", 7)), nonweapon(pk("tumbling", 5), pk("tracking", 2))},
		{"leftovers", nonweapon(), nonweapon(pk("tumbling", 1))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPicksFixture(t, tc.stored)
			f.records()
			f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, tc.save).Return(nil).Once()
			_, err := f.svc.SetProficiencies(context.Background(), charID, skillsvc.Edit{Picks: tc.save})
			require.NoError(t, err)
			assert.Len(t, f.entries, 1)
		})
	}
}

// ADR 0037 step 12's preview: the pools read with the draft picks, refused as the save refuses
// (a field error, the budget), writing and logging nothing (the picks and log mocks fail any
// write or record). The bard's 5 non-weapon slots.
func TestPreviewPicks_Step12_SameRefusalsAsTheSave(t *testing.T) {
	t.Run("within budget", func(t *testing.T) {
		f := newPicksFixture(t, nonweapon())
		got, err := f.svc.PreviewProficiencies(context.Background(), charID, skillsvc.Edit{Picks: nonweapon(pk("tumbling", 3))})
		require.NoError(t, err)
		require.NotNil(t, got.Slots)
		assert.Equal(t, 2, got.Slots.Nonweapon.Left)
		assert.Equal(t, 2, got.Slots.Picks[0].Extra)
		assert.Equal(t, 17, got.Slots.Picks[0].Target, "Dex 15 + 2 extra slots")
		two := 2
		assert.Contains(t, got.Warnings, skills.Warning{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &two})
		assert.Equal(t, 1, f.tx.calls, "one transaction")
	})
	t.Run("over budget and growing", func(t *testing.T) {
		f := newPicksFixture(t, nonweapon(pk("tumbling", 5)))
		_, err := f.svc.PreviewProficiencies(context.Background(), charID, skillsvc.Edit{Picks: nonweapon(pk("tumbling", 6))})
		var be *domain.BudgetError
		require.ErrorAs(t, err, &be)
		assert.Equal(t, domain.BudgetError{Field: "nonweapon", Spent: 6, Limit: 5}, *be)
	})
	t.Run("a refused pick", func(t *testing.T) {
		f := newPicksFixture(t, nonweapon())
		_, err := f.svc.PreviewProficiencies(context.Background(), charID, skillsvc.Edit{Picks: nonweapon(pk("juggling", 1))})
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, domain.FieldError{Field: "nonweapon.juggling", Message: "unknown"}, *fe)
	})
}

// ADR 0037 step 5 (amending ADR 0014 step 1): a proficiency check is one d20 against the target
// the character derives, ability score + modifier + extra slots; the payload carries extraSlots
// (absent when 0, so an entry logged before reads 0).
func TestSkillCheck_Step5_TargetCountsExtraSlots(t *testing.T) {
	for _, tc := range []struct {
		name          string
		slots, extra  int
		target, d20   int
		success       bool
		payloadExtras string
	}{
		{"at the cost", 1, 0, 17, 17, true, ""},
		{"cost + 1", 2, 1, 18, 18, true, `"extraSlots":1,`},
		{"cost + 3, a roll the bare target fails", 4, 3, 20, 19, true, `"extraSlots":3,`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			c := bard() // Dex 17
			c.Proficiencies = &skills.ProficiencySlots{Picks: []skills.PickFigures{
				{ID: "jumping", Ability: "dex", Slots: tc.slots, Cost: 1, Extra: tc.extra, Known: true},
			}}
			c.Proficiencies.SetTargets(func(a string) (int, bool) { return c.Score(charactersvc.Ability(a)) })
			f.chars.ExpectedCalls = nil
			f.chars.EXPECT().View(mock.Anything, charID).Return(c, nil)
			f.records()
			f.dice.EXPECT().RollDie(20).Return(tc.d20).Once()
			r := f.roll(t, "jumping")
			f.dice.AssertNumberOfCalls(t, "RollDie", 1)
			assert.Equal(t, tc.target, r.Target)
			assert.Equal(t, tc.extra, r.ExtraSlots)
			assert.Equal(t, tc.success, r.Success)
			assert.JSONEq(t, `{"id":"jumping","kind":"proficiency","die":20,"roll":`+itoa(tc.d20)+`,"target":`+itoa(tc.target)+
				`,"adjustment":0,"adjustedTarget":`+itoa(tc.target)+`,"success":true,"rolledSuccess":true,"ability":"dex","abilityScore":17,`+
				tc.payloadExtras+`"modifier":0}`, string(f.entries[0].Payload))
		})
	}
	t.Run("a pick with no check is not rolled", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.svc.Roll(context.Background(), charID, "blind_fighting", skillsvc.Adjustment{})
		require.ErrorIs(t, err, domain.ErrNotFound)
		_, err = f.svc.Roll(context.Background(), charID, "old_skill", skillsvc.Adjustment{})
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
