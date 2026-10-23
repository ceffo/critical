package psionicsvc_test

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
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc/mocks"
)

func newSvc(t *testing.T) (psionicsvc.SVC, *mocks.MockTables, *mocks.MockLog) {
	t.Helper()
	return newSvcWithBuffs(t, nil)
}

// newSvcWithBuffs is newSvc with active on the buff list.
func newSvcWithBuffs(t *testing.T, active []buffsvc.BuffEffect) (psionicsvc.SVC, *mocks.MockTables, *mocks.MockLog) {
	t.Helper()
	rates, log := mocks.NewMockTables(t), mocks.NewMockLog(t)
	buffs := mocks.NewMockBuffs(t)
	buffs.EXPECT().ListActive(mock.Anything, int64(7)).Return(active, nil).Maybe()
	svc, err := psionicsvc.New(psionicsvc.Deps{
		Buffs:  buffs,
		Tables: rates, Log: log, Characters: mocks.NewMockCharacters(t), Picks: mocks.NewMockPicks(t), Dice: mocks.NewMockDice(t),
		Book: psionics.NewBook(nil, nil), Tx: passTx{},
	})
	require.NoError(t, err)
	return svc, rates, log
}

// ADR 0043 step 6: hours of an activity recover hours × Table 6's hourly
// rate, as one psp_recovery entry the log bounds (the cap is step 5's, in
// the recording path: TestRecover_Step6_CappedAndUndone in composition).
func TestRecover_Step6_HoursTimesTheActivityRate(t *testing.T) {
	for _, c := range []struct {
		name     string
		hours    float64
		activity string
		rate     int
		delta    int
	}{
		{"3 h resting gives 18", 3, "resting", 6, 18},
		{"8 h sleeping gives 96", 8, "sleeping", 12, 96},
		{"1.5 h walking gives 4 (rounded down)", 1.5, "walking", 3, 4},
		{"exertion logs the hours with no PSP", 2, "exertion", 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, rates, log := newSvc(t)
			rates.EXPECT().PSPRecoveryRate(c.activity).Return(c.rate, nil).Once()
			var got actionlog.NewEntry
			log.EXPECT().AppendPSP(mock.Anything, mock.Anything).
				Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).
				Return(actionlog.PSPState{Current: 20, Max: 25}, nil).Once()

			state, err := svc.Recover(context.Background(), 7, c.hours, c.activity)
			require.NoError(t, err)
			assert.Equal(t, actionlog.PSPState{Current: 20, Max: 25}, state)
			assert.Equal(t, int64(7), got.CharacterID)
			assert.Equal(t, actionlog.KindPSPRecovery, got.Kind)
			assert.Equal(t, c.delta, got.PSPDelta)
			assert.Empty(t, got.Summary)
			var rec map[string]any
			require.NoError(t, json.Unmarshal(got.Payload, &rec))
			assert.Equal(t, map[string]any{"hours": c.hours, "activity": c.activity, "perHour": float64(c.rate)}, rec)
		})
	}
}

// Invalid hours or an activity outside Table 6 are refused before Table 6 or
// the log is read; the log's own refusal (no pool) comes back as is.
func TestRecover_Step6_Refusals(t *testing.T) {
	for _, c := range []struct {
		name, activity, field string
		hours                 float64
	}{
		{"no hours", "resting", "hours", 0},
		{"negative hours", "resting", "hours", -1},
		{"over the cap", "resting", "hours", psionicsvc.MaxRecoveryHours + 1},
		{"unknown activity", "meditating", "activity", 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, _, _ := newSvc(t)
			_, err := svc.Recover(context.Background(), 7, c.hours, c.activity)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, c.field, fe.Field)
		})
	}
	t.Run("no pool", func(t *testing.T) {
		svc, rates, log := newSvc(t)
		rates.EXPECT().PSPRecoveryRate("resting").Return(6, nil).Once()
		log.EXPECT().AppendPSP(mock.Anything, mock.Anything).Return(actionlog.PSPState{}, actionlogsvc.ErrNoPSPPool).Once()
		_, err := svc.Recover(context.Background(), 7, 1, "resting")
		assert.True(t, errors.Is(err, actionlogsvc.ErrNoPSPPool))
	})
}

// ADR 0043 step 6: no recovery while a power is maintained (the book
// recovers only in hours with no PSP spent); a buff row that is not a
// maintained power does not refuse it.
func TestRecover_Step6_RefusedWhileMaintaining(t *testing.T) {
	svc, rates, _ := newSvcWithBuffs(t, []buffsvc.BuffEffect{
		{ID: "b", Name: "Bless"},
		{ID: "r", Name: "Levitation", PowerID: "levitation", MaintenanceCost: 2, MaintenancePeriod: 1},
	})
	rates.EXPECT().PSPRecoveryRate("resting").Return(6, nil).Maybe()
	_, err := svc.Recover(context.Background(), 7, 3, "resting")
	require.ErrorIs(t, err, psionicsvc.ErrRecoveryWhileMaintaining, "nothing logged: the log mock has no AppendPSP")

	svc, rates, log := newSvcWithBuffs(t, []buffsvc.BuffEffect{{ID: "b", Name: "Bless"}})
	rates.EXPECT().PSPRecoveryRate("resting").Return(6, nil).Once()
	log.EXPECT().AppendPSP(mock.Anything, mock.Anything).Return(actionlog.PSPState{Current: 18, Max: 25}, nil).Once()
	_, err = svc.Recover(context.Background(), 7, 3, "resting")
	require.NoError(t, err)
}

// passTx runs the unit of work as is: the mocks stand for the store.
type passTx struct{}

func (passTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func newKnownSvc(t *testing.T, c *charactersvc.Character) (psionicsvc.SVC, *mocks.MockPicks, *mocks.MockLog) {
	t.Helper()
	chars, picks, log := mocks.NewMockCharacters(t), mocks.NewMockPicks(t), mocks.NewMockLog(t)
	chars.EXPECT().GetCharacter(mock.Anything, int64(7)).Return(c, nil)
	book := psionics.NewBook(
		[]catalogue.Discipline{{ID: "telepathy"}},
		[]catalogue.Power{{ID: "contact", Discipline: "telepathy", Kind: catalogue.KindDevotion}},
	)
	svc, err := psionicsvc.New(psionicsvc.Deps{
		Buffs:  mocks.NewMockBuffs(t),
		Tables: mocks.NewMockTables(t), Log: log, Characters: chars, Picks: picks, Dice: mocks.NewMockDice(t), Book: book, Tx: passTx{},
	})
	require.NoError(t, err)
	return svc, picks, log
}

func psionicist(stored ...psionics.Pick) *charactersvc.Character {
	return &charactersvc.Character{ID: 7, PsionicPicks: stored, Derived: charactersvc.Derived{Powers: &charactersvc.PowerKnowledge{
		Allowance: psionics.Allowance{Disciplines: 3, Sciences: 1, Devotions: 4, DefenceModes: 1},
	}}}
}

func TestSetKnownPowers_StoresTheListAndLogsOneEntryWithItsSnapshot(t *testing.T) {
	svc, picks, log := newKnownSvc(t, psionicist())
	want := []psionics.Pick{{Kind: psionics.PickDiscipline, ID: "telepathy"}, {Kind: psionics.PickPower, ID: "contact"}}
	picks.EXPECT().SetPsionicPicks(mock.Anything, int64(7), want).Return(nil).Once()
	var got actionlog.NewEntry
	log.EXPECT().Record(mock.Anything, mock.Anything).
		Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(1), nil).Once()

	k, err := svc.SetKnownPowers(context.Background(), 7, want)
	require.NoError(t, err)
	assert.Equal(t, "telepathy", k.Primary)
	assert.Equal(t, psionics.Used{Disciplines: 1, Devotions: 1}, k.Used)
	assert.Equal(t, actionlog.KindPsionicPick, got.Kind)
	require.Len(t, got.Snapshots, 1)
	assert.Equal(t, psionicsvc.KnownPowersStateKey, got.Snapshots[0].Key)
}

func TestSetKnownPowers_TheSameListRecordsNothing(t *testing.T) {
	same := []psionics.Pick{{Kind: psionics.PickDiscipline, ID: "telepathy"}}
	svc, _, _ := newKnownSvc(t, psionicist(same...))
	_, err := svc.SetKnownPowers(context.Background(), 7, same)
	require.NoError(t, err)
}

func TestSetKnownPowers_RefusalsWriteNothing(t *testing.T) {
	svc, _, _ := newKnownSvc(t, psionicist())
	_, err := svc.SetKnownPowers(context.Background(), 7, []psionics.Pick{{Kind: psionics.PickPower, ID: "contact"}})
	var refusal *psionics.Refusal
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, psionics.CodeDisciplineNotKnown, refusal.Code)

	_, err = svc.SetKnownPowers(context.Background(), 7, []psionics.Pick{{Kind: psionics.PickPower, ID: "nope"}})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "picks", fe.Field)

	other, _, _ := newKnownSvc(t, &charactersvc.Character{ID: 7})
	_, err = other.SetKnownPowers(context.Background(), 7, nil)
	require.ErrorIs(t, err, psionicsvc.ErrNotPsionicist)
}
