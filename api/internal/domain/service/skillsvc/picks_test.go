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
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc/mocks"
)

type picksFixture struct {
	*fixture
	picks *mocks.MockPicks
}

// newPicksFixture is a bard of Intelligence 9 at realized level 1 (testBook's rogue row: 3 + 0 +
// 2 = 5 non-weapon slots) storing stored; every GetCharacter answers it (the read after a save
// included).
func newPicksFixture(t *testing.T, stored charactersvc.Picks) *picksFixture {
	t.Helper()
	f := &picksFixture{fixture: &fixture{
		chars: mocks.NewMockCharacters(t), abils: mocks.NewMockAbilities(t), dice: mocks.NewMockDice(t),
		log: mocks.NewMockLog(t), points: mocks.NewMockPoints(t), tx: &countingTx{},
	}, picks: mocks.NewMockPicks(t)}
	svc, err := skillsvc.New(skillsvc.Deps{
		Characters: f.chars, Abilities: f.abils, Dice: f.dice, Log: f.log, Points: f.points, Picks: f.picks, Book: testBook(t), Tx: f.tx,
	})
	require.NoError(t, err)
	f.svc = svc
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{
		ID: charID, Class: "bard", Int: 9, Dex: 15, RealizedLevel: 1, Proficiencies: stored,
		Derived: charactersvc.Derived{RealizedLevel: 1},
	}, nil).Maybe()
	return f
}

func nonweapon(ps ...charactersvc.Pick) charactersvc.Picks {
	return charactersvc.Picks{Nonweapon: append([]charactersvc.Pick{}, ps...)}
}

func pk(id string, slots int) charactersvc.Pick { return charactersvc.Pick{ID: id, Slots: slots} }

// ADR 0037 step 13: a save that changes the picks writes them and records one proficiencies entry
// (the picks before and after, snapshotted under "proficiencies") in one transaction.
func TestSetPicks_Step13_LogsOneMoveWithASnapshot(t *testing.T) {
	f := newPicksFixture(t, nonweapon(pk("tumbling", 1)))
	f.records()
	next := nonweapon(pk("tumbling", 2), pk("tracking", 2))
	f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, next).Return(nil).Once()

	_, err := f.svc.SetProficiencies(context.Background(), charID, next)
	require.NoError(t, err)
	assert.Equal(t, 1, f.tx.calls, "one transaction")
	require.Len(t, f.entries, 1, "one log entry")
	e := f.entries[0]
	assert.Equal(t, actionlog.KindProficiencies, e.Kind)
	assert.JSONEq(t, `{"before":{"nonweapon":[{"id":"tumbling","slots":1}]},
		"after":{"nonweapon":[{"id":"tumbling","slots":2},{"id":"tracking","slots":2}]}}`, string(e.Payload))
	require.Len(t, e.Snapshots, 1)
	assert.Equal(t, skillsvc.PicksStateKey, e.Snapshots[0].Key)
	assert.JSONEq(t, `{"nonweapon":[{"id":"tumbling","slots":1}]}`, snaptest.State(t, e.Snapshots[0].Before))
	assert.JSONEq(t, `{"nonweapon":[{"id":"tumbling","slots":2},{"id":"tracking","slots":2}]}`, snaptest.State(t, e.Snapshots[0].After))
}

// ADR 0037 step 13: the same picks as stored write and log nothing.
func TestSetPicks_Step13_NoChangeLogsNothing(t *testing.T) {
	f := newPicksFixture(t, nonweapon(pk("tumbling", 1))) // the picks and log mocks fail any write or record
	_, err := f.svc.SetProficiencies(context.Background(), charID, nonweapon(pk("tumbling", 1)))
	require.NoError(t, err)
}

// ADR 0037 step 13: a save that only reorders the stored picks writes and logs nothing (the
// journal would show an entry with no change).
func TestSetPicks_Step13_ReorderLogsNothing(t *testing.T) {
	f := newPicksFixture(t, nonweapon(pk("tumbling", 1), pk("tracking", 2))) // the picks and log mocks fail any write or record
	_, err := f.svc.SetProficiencies(context.Background(), charID, nonweapon(pk("tracking", 2), pk("tumbling", 1)))
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
		stored charactersvc.Picks
		save   charactersvc.Picks
		field  string
		budget *domain.BudgetError
	}{
		{"unknown id", nonweapon(), nonweapon(pk("juggling", 1)), "nonweapon.juggling", nil},
		{"slots below the cost", nonweapon(), nonweapon(pk("tracking", 1)), "nonweapon.tracking", nil},
		{"duplicate", nonweapon(), nonweapon(pk("tumbling", 1), pk("tumbling", 1)), "nonweapon.tumbling", nil},
		{"languages modern", nonweapon(), nonweapon(pk("languages_modern", 1)), "nonweapon.languages_modern", nil},
		{"a new overspend", nonweapon(pk("tumbling", 5)), nonweapon(pk("tumbling", 6)), "", &domain.BudgetError{Field: "nonweapon", Spent: 6, Limit: 5}},
		{"an existing overspend increased", nonweapon(pk("tumbling", 7)), nonweapon(pk("tumbling", 7), pk("tracking", 2)), "", &domain.BudgetError{Field: "nonweapon", Spent: 9, Limit: 5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPicksFixture(t, tc.stored) // the picks and log mocks fail any write or record
			_, err := f.svc.SetProficiencies(context.Background(), charID, tc.save)
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
		stored, save charactersvc.Picks
	}{
		{"an overspend reduced, still over", nonweapon(pk("tumbling", 8)), nonweapon(pk("tumbling", 6))},
		{"an overspend kept, moved between picks", nonweapon(pk("tumbling", 7)), nonweapon(pk("tumbling", 5), pk("tracking", 2))},
		{"leftovers", nonweapon(), nonweapon(pk("tumbling", 1))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPicksFixture(t, tc.stored)
			f.records()
			f.picks.EXPECT().SetProficiencyPicks(mock.Anything, charID, tc.save).Return(nil).Once()
			_, err := f.svc.SetProficiencies(context.Background(), charID, tc.save)
			require.NoError(t, err)
			assert.Len(t, f.entries, 1)
		})
	}
}

// The editor's preview is the character service's draft read: nothing written or logged.
func TestPreviewPicks_Step12_WritesNothing(t *testing.T) {
	f := newPicksFixture(t, nonweapon())
	draft := nonweapon(pk("tumbling", 2))
	f.chars.EXPECT().PreviewProficiencies(mock.Anything, charID, draft).Return(&charactersvc.Character{ID: charID}, nil).Once()
	got, err := f.svc.PreviewProficiencies(context.Background(), charID, draft)
	require.NoError(t, err)
	assert.Equal(t, charID, got.ID)
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
			c.Derived.Proficiencies = &charactersvc.ProficiencySlots{Picks: []charactersvc.PickFigures{
				{ID: "tumbling", Ability: "dex", Slots: tc.slots, Cost: 1, Extra: tc.extra, Known: true},
			}}
			c.Derived.Proficiencies.SetTargets(c.Score)
			f.chars.ExpectedCalls = nil
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(c, nil)
			f.records()
			f.dice.EXPECT().RollDie(20).Return(tc.d20).Once()
			r := f.roll(t, "tumbling")
			f.dice.AssertNumberOfCalls(t, "RollDie", 1)
			assert.Equal(t, tc.target, r.Target)
			assert.Equal(t, tc.extra, r.ExtraSlots)
			assert.Equal(t, tc.success, r.Success)
			assert.JSONEq(t, `{"id":"tumbling","kind":"proficiency","die":20,"roll":`+itoa(tc.d20)+`,"target":`+itoa(tc.target)+
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
