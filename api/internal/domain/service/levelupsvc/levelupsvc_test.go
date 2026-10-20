package levelupsvc_test

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
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The level-up roll of ADR 0030 step 7. Step 7.1: "Up to the name level
// (warriors and priests through level 9, wizards and rogues through 10):
// one hit die of the group (warrior d10, priest d8, rogue d6, wizard d4)
// rolled through the dice port, plus the Constitution hit point adjustment
// of PHB Table 3: warriors take the bracketed value, every other class at
// most +2. At Constitution 20 and more, Table 3's footnotes raise low dice
// [...]: the die shown is the one rolled, the gain uses the raised value. The
// gain is at least 1." Step 7.2: "Above the name level: a fixed gain, no die
// and no Constitution bonus." Step 7.3: "Recorded as one transaction: a
// level_up entry with the level reached, the die and its raised value (or
// none), the Constitution adjustment and the gain; the max HP rises by the
// gain and the realized level by 1, snapshotted for undo. [...] Several pending levels
// are rolled one by one, lowest first."

// recordingTx runs the unit of work directly and counts the transactions.
type recordingTx struct{ n int }

func (r *recordingTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	r.n++
	return fn(ctx)
}

type fixture struct {
	svc   levelupsvc.SVC
	dal   *mocks.MockDAL
	chars *mocks.MockCharacters
	rules *mocks.MockRules
	dice  *mocks.MockDice
	log   *mocks.MockLog
	tx    *recordingTx
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{
		dal: mocks.NewMockDAL(t), chars: mocks.NewMockCharacters(t), rules: mocks.NewMockRules(t),
		dice: mocks.NewMockDice(t), log: mocks.NewMockLog(t), tx: &recordingTx{},
	}
	svc, err := levelupsvc.New(levelupsvc.Deps{
		DAL: f.dal, Characters: f.chars, Rules: f.rules, Dice: f.dice, Log: f.log, Tx: f.tx,
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

const charID = int64(4)

// character is a class's character realized at realizedLevel with max HP
// hpMax, owing pending.
func character(class string, con, hpMax, realizedLevel int, pending ...charactersvc.PendingLevel) *charactersvc.Character {
	return &charactersvc.Character{
		ID: charID, Class: class, Con: con, HPMax: hpMax, RealizedLevel: realizedLevel,
		Derived: charactersvc.Derived{RealizedLevel: realizedLevel, LevelUp: charactersvc.LevelUp{Pending: pending}},
	}
}

func d6(level int) charactersvc.PendingLevel {
	return charactersvc.PendingLevel{Level: level, Die: 6}
}

func d10(level int) charactersvc.PendingLevel {
	return charactersvc.PendingLevel{Level: level, Die: 10}
}

// con is a PHB Table 3 row.
func con(hp, warrior, lowDieMin int) ruletablesvc.ConstitutionRow {
	return ruletablesvc.ConstitutionRow{HP: hp, HPWarrior: warrior, LowDieMin: lowDieMin}
}

// recorded captures the action-log entries the writes record.
func (f fixture) recorded() *[]actionlog.NewEntry {
	got := &[]actionlog.NewEntry{}
	f.log.EXPECT().Record(mock.Anything, mock.Anything).
		Run(func(_ context.Context, e actionlog.NewEntry) { *got = append(*got, e) }).Return(42, nil)
	return got
}

func intp(n int) *int { return &n }

func TestNew_RejectsNilDependencies(t *testing.T) {
	_, err := levelupsvc.New(levelupsvc.Deps{})
	require.Error(t, err)
}

// Step 7.1: a bard (rogue) of Constitution 16 reaching level 9 rolls one d6
// through the dice port (once) and adds Table 3's +2.
func TestRoll_Step7_1_OneHitDieOfTheGroupPlusTheConstitutionAdjustment(t *testing.T) {
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(character("bard", 16, 40, 8, d6(9)), nil).Once()
	f.dice.EXPECT().RollDie(6).Return(4).Once()
	f.rules.EXPECT().ConstitutionFor(16).Return(con(2, 2, 0), nil).Once()
	f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, 9, 46).Return(nil).Once()
	f.recorded()

	got, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: 9})
	require.NoError(t, err)
	assert.Equal(t, &levelupsvc.Result{
		Level: 9, HitDie: 6, Rolled: intp(4), Counted: intp(4), ConAdjustment: 2, Gain: 6,
		HPMaxBefore: 40, HPMaxAfter: 46, Pending: []charactersvc.PendingLevel{},
	}, got)
}

// Step 7.1: warriors take Table 3's bracketed value, every other class the
// plain one (at most +2): Constitution 18 is +4 for a fighter, +2 for a bard.
func TestRoll_Step7_1_WarriorsTakeTheBracketedAdjustment(t *testing.T) {
	for _, tc := range []struct {
		class   string
		pending charactersvc.PendingLevel
		die     int
		adj     int
	}{
		{"fighter", d10(3), 10, 4},
		{"bard", d6(3), 6, 2},
	} {
		t.Run(tc.class, func(t *testing.T) {
			f := newFixture(t)
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(character(tc.class, 18, 20, 2, tc.pending), nil).Once()
			f.dice.EXPECT().RollDie(tc.die).Return(5).Once()
			f.rules.EXPECT().ConstitutionFor(18).Return(con(2, 4, 0), nil).Once()
			f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, 3, 20+5+tc.adj).Return(nil).Once()
			f.recorded()

			got, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: 3})
			require.NoError(t, err)
			assert.Equal(t, tc.adj, got.ConAdjustment)
			assert.Equal(t, 5+tc.adj, got.Gain)
		})
	}
}

// Step 7.1, Table 3's footnotes: from Constitution 20 a low die counts as
// the row's floor (1 as 2 at 20; 1 and 2 as 3 at 21-22; 1 to 3 as 4 at 23 and
// up); the die shown is the one rolled, the gain uses the raised value. A die
// at or over the floor is kept.
func TestRoll_Step7_1_LowDiceRaisedFromConstitution20(t *testing.T) {
	for _, tc := range []struct {
		name               string
		con, floor, rolled int
		counted            int
	}{
		{"20: a 1 counts as 2", 20, 2, 1, 2},
		{"21: a 2 counts as 3", 21, 3, 2, 3},
		{"23: a 3 counts as 4", 23, 4, 3, 4},
		{"23: a 5 stays 5", 23, 4, 5, 5},
		{"19: no floor, a 1 stays 1", 19, 0, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(character("bard", tc.con, 30, 4, d6(5)), nil).Once()
			f.dice.EXPECT().RollDie(6).Return(tc.rolled).Once()
			f.rules.EXPECT().ConstitutionFor(tc.con).Return(con(2, 6, tc.floor), nil).Once()
			f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, 5, 30+tc.counted+2).Return(nil).Once()
			f.recorded()

			got, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: 5})
			require.NoError(t, err)
			assert.Equal(t, tc.rolled, *got.Rolled, "the die shown is the one rolled")
			assert.Equal(t, tc.counted, *got.Counted)
			assert.Equal(t, tc.counted+2, got.Gain)
		})
	}
}

// Step 7.1 (proposed, settled): the gain is at least 1: a 1 with
// Constitution 3's -2 still raises the max HP by 1.
func TestRoll_Step7_1_TheGainIsAtLeastOne(t *testing.T) {
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(character("bard", 3, 10, 2, d6(3)), nil).Once()
	f.dice.EXPECT().RollDie(6).Return(1).Once()
	f.rules.EXPECT().ConstitutionFor(3).Return(con(-2, -2, 0), nil).Once()
	f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, 3, 11).Return(nil).Once()
	f.recorded()

	got, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: 3})
	require.NoError(t, err)
	assert.Equal(t, -2, got.ConAdjustment)
	assert.Equal(t, 1, got.Gain)
}

// Step 7.2: above the name level the gain is fixed: no die is rolled and no
// Constitution is read (the mocks fail any such call).
func TestRoll_Step7_2_AFixedGainAboveTheNameLevel(t *testing.T) {
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).
		Return(character("bard", 18, 85, 15, charactersvc.PendingLevel{Level: 16, FixedGain: 2}), nil).Once()
	f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, 16, 87).Return(nil).Once()
	f.recorded()

	got, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: 16})
	require.NoError(t, err)
	assert.Equal(t, &levelupsvc.Result{
		Level: 16, Gain: 2, Fixed: true, HPMaxBefore: 85, HPMaxAfter: 87, Pending: []charactersvc.PendingLevel{},
	}, got)
}

// Step 7.3: one transaction, one level_up entry whose payload is the
// breakdown and whose "realized_level" snapshot holds the realized level and
// its max HP before and after; no HP delta (current HP is the max HP plus the ledger,
// so it rises with the max HP and the damage taken is kept).
func TestRoll_Step7_3_OneTransactionWithTheRealizedLevelSnapshot(t *testing.T) {
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(character("bard", 16, 40, 8, d6(9), d6(10)), nil).Once()
	f.dice.EXPECT().RollDie(6).Return(3).Once()
	f.rules.EXPECT().ConstitutionFor(16).Return(con(2, 2, 0), nil).Once()
	f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, 9, 45).Return(nil).Once()
	rec := f.recorded()

	got, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: 9})
	require.NoError(t, err)
	assert.Equal(t, 1, f.tx.n, "one transaction")
	assert.Equal(t, []charactersvc.PendingLevel{d6(10)}, got.Pending, "the level still owed")
	require.Len(t, *rec, 1)
	e := (*rec)[0]
	assert.Equal(t, actionlog.KindLevelUp, e.Kind)
	assert.Equal(t, charID, e.CharacterID)
	assert.Zero(t, e.HPDelta)
	assert.Empty(t, e.Summary, "no text from the backend")
	assert.JSONEq(t, `{"level":9,"hitDie":6,"rolled":3,"counted":3,"conAdjustment":2,"gain":5,
		"fixed":false,"manual":false,"hpMaxBefore":40,"hpMaxAfter":45}`, string(e.Payload))
	require.Len(t, e.Snapshots, 1)
	assert.Equal(t, levelupsvc.StateKey, e.Snapshots[0].Key)
	assert.JSONEq(t, `{"realizedLevel":8,"hpMax":40}`, snaptest.State(t, e.Snapshots[0].Before))
	assert.JSONEq(t, `{"realizedLevel":9,"hpMax":45}`, snaptest.State(t, e.Snapshots[0].After))
}

// Step 7.3: two levels gained at once are two rolls, one per request, lowest
// first (a request for the higher level first is refused: see
// TestRoll_RefusesALevelThatIsNotTheLowestPending): two dice (distinct
// values, exactly two calls), two entries, the max HP rising twice.
func TestRoll_Step7_3_TwoLevelsAtOnceAreTwoRollsLowestFirst(t *testing.T) {
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(character("bard", 16, 40, 8, d6(9), d6(10)), nil).Once()
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(character("bard", 16, 45, 9, d6(10)), nil).Once()
	f.dice.EXPECT().RollDie(6).Return(3).Once()
	f.dice.EXPECT().RollDie(6).Return(5).Once()
	f.rules.EXPECT().ConstitutionFor(16).Return(con(2, 2, 0), nil).Twice()
	f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, 9, 45).Return(nil).Once()
	f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, 10, 52).Return(nil).Once()
	rec := f.recorded()

	first, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: 9})
	require.NoError(t, err)
	second, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: 10})
	require.NoError(t, err)
	assert.Equal(t, 3, *first.Rolled)
	assert.Equal(t, 5, *second.Rolled)
	assert.Equal(t, 45, first.HPMaxAfter)
	assert.Equal(t, 52, second.HPMaxAfter)
	assert.Empty(t, second.Pending)
	assert.Len(t, *rec, 2)
	f.dice.AssertNumberOfCalls(t, "RollDie", 2)
}

// A roll for a level that is not the lowest pending one (none pending, a
// stale page, a level above it) is refused: nothing is rolled, written or
// recorded.
func TestRoll_RefusesALevelThatIsNotTheLowestPending(t *testing.T) {
	for _, tc := range []struct {
		name  string
		c     *charactersvc.Character
		level int
	}{
		{"none pending", character("bard", 16, 85, 15), 16},
		{"already rolled (a stale page)", character("bard", 16, 45, 9, d6(10)), 9},
		{"not the lowest", character("bard", 16, 40, 8, d6(9), d6(10)), 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t) // no dice, DAL or log call expected
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(tc.c, nil).Once()
			_, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: tc.level})
			require.ErrorIs(t, err, levelupsvc.ErrNotPending)
			require.ErrorIs(t, err, domain.ErrConflict)
		})
	}
}

// The manual entry: the die the player rolled at the table replaces the
// dice port's (never called); the rest of step 7.1 applies to it. A die out
// of 1 to the hit die's sides, or a die for a fixed gain, is a field error
// and nothing is written.
func TestRoll_TheManualEntryTakesTheDieRolledAtTheTable(t *testing.T) {
	t.Run("the entered die", func(t *testing.T) {
		f := newFixture(t)
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(character("bard", 16, 40, 8, d6(9)), nil).Once()
		f.rules.EXPECT().ConstitutionFor(16).Return(con(2, 2, 0), nil).Once()
		f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, 9, 48).Return(nil).Once()
		rec := f.recorded()

		got, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: 9, Die: intp(6)})
		require.NoError(t, err)
		assert.True(t, got.Manual)
		assert.Equal(t, 6, *got.Rolled)
		assert.Equal(t, 8, got.Gain)
		var p map[string]any
		require.NoError(t, json.Unmarshal((*rec)[0].Payload, &p))
		assert.Equal(t, true, p["manual"])
	})
	for _, tc := range []struct {
		name    string
		pending charactersvc.PendingLevel
		die     int
		code    string
	}{
		{"0", d6(9), 0, levelupsvc.CodeOutOfRange},
		{"over the hit die", d6(9), 7, levelupsvc.CodeOutOfRange},
		{"a fixed gain has no die", charactersvc.PendingLevel{Level: 9, FixedGain: 2}, 2, levelupsvc.CodeFixedGain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(character("bard", 16, 40, 8, tc.pending), nil).Once()
			_, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: 9, Die: intp(tc.die)})
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, "die", fe.Field)
			assert.Equal(t, tc.code, fe.Message)
		})
	}
}

// Undo (and redo) write a realized level snapshot back: the level and its max
// HP, so a popped roll's level is pending again. A broken snapshot is an error.
func TestRestorer_WritesTheRealizedLevelBack(t *testing.T) {
	f := newFixture(t)
	r := f.svc.Module().Restorer(levelupsvc.StateKey)
	require.NotNil(t, r)
	f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, 8, 40).Return(nil).Once()
	require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"realizedLevel":8,"hpMax":40}`)))
	require.Error(t, r.Restore(context.Background(), charID, snaptest.Env(`{"realizedLevel":8,"hpMax":0}`)))
	require.Error(t, r.Restore(context.Background(), charID, json.RawMessage(`nope`)))
}

// ADR 0037 step 15: "The pending level-up prompt and the level-up answer carry what the level
// grants beyond hit points." The answer gives the rolled level's grants as the character read
// gave them (charactersvc computes them, TestGrantsAt_Step15_GrantsSlotsAndPoints), not the next
// pending level's.
func TestLevelUp_Step15_GrantsSlotsAndPoints(t *testing.T) {
	f := newFixture(t)
	four := d6(4)
	four.Grants = charactersvc.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1, SkillPoints: 30}
	five := d6(5)
	five.Grants = charactersvc.LevelGrants{SkillPoints: 30}
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(character("thief", 12, 15, 3, four, five), nil).Once()
	f.dice.EXPECT().RollDie(6).Return(3).Once()
	f.rules.EXPECT().ConstitutionFor(12).Return(con(0, 0, 0), nil).Once()
	f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, 4, 18).Return(nil).Once()
	f.recorded()

	got, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Level: 4})
	require.NoError(t, err)
	assert.Equal(t, four.Grants, got.Grants)
	assert.Equal(t, []charactersvc.PendingLevel{five}, got.Pending)
}
