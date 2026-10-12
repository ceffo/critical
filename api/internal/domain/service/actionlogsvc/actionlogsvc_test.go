package actionlogsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// passTx runs the body directly and counts calls; transactional behaviour is
// covered against real SQLite in actionlogdal's atomicity tests.
type passTx struct{ calls int }

func (p *passTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	p.calls++
	return fn(ctx)
}

type restorerFunc func(ctx context.Context, characterID int64, state json.RawMessage) error

func (f restorerFunc) Restore(ctx context.Context, characterID int64, state json.RawMessage) error {
	return f(ctx, characterID, state)
}

var errBoom = errors.New("boom")

func newSvc(t *testing.T) (actionlogsvc.SVC, *mocks.MockDAL, *mocks.MockRounds, *passTx) {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	rounds := mocks.NewMockRounds(t)
	tx := &passTx{}
	svc, err := actionlogsvc.NewWithConfig(actionlogsvc.DefaultConfig(), dal, tx, rounds)
	require.NoError(t, err)
	return svc, dal, rounds, tx
}

func intp(v int) *int { return &v }

func TestNewWithConfig_Validation(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	rounds := mocks.NewMockRounds(t)
	cases := []struct {
		name   string
		cfg    *actionlogsvc.Config
		dal    actionlogsvc.DAL
		tx     domain.TxRunner
		rounds actionlogsvc.Rounds
	}{
		{"nil config", nil, dal, &passTx{}, rounds},
		{"nil dal", actionlogsvc.DefaultConfig(), nil, &passTx{}, rounds},
		{"nil tx", actionlogsvc.DefaultConfig(), dal, nil, rounds},
		{"nil rounds", actionlogsvc.DefaultConfig(), dal, &passTx{}, nil},
		{"default above max", &actionlogsvc.Config{DefaultLimit: 10, MaxLimit: 5}, dal, &passTx{}, rounds},
		{"zero limit", &actionlogsvc.Config{DefaultLimit: 0, MaxLimit: 5}, dal, &passTx{}, rounds},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := actionlogsvc.NewWithConfig(c.cfg, c.dal, c.tx, c.rounds)
			require.Error(t, err)
		})
	}
}

func TestRecord(t *testing.T) {
	snap := func(k string) actionlog.Snapshot {
		return actionlog.Snapshot{Key: k, Before: json.RawMessage(`0`), After: json.RawMessage(`1`)}
	}
	cases := []struct {
		name      string
		in        actionlog.NewEntry
		round     int
		wantErr   error
		wantStore *actionlog.NewEntry
	}{
		{
			name:    "kind required",
			in:      actionlog.NewEntry{CharacterID: 1, Summary: "x"},
			wantErr: domain.ErrInvalidInput,
		},
		{
			name:    "summary required",
			in:      actionlog.NewEntry{CharacterID: 1, Kind: actionlog.KindHPChange, Summary: "  "},
			wantErr: domain.ErrInvalidInput,
		},
		{
			name:    "duplicate snapshot keys",
			in:      actionlog.NewEntry{CharacterID: 1, Kind: actionlog.KindBuffChange, Summary: "x", Snapshots: []actionlog.Snapshot{snap("buffs"), snap("buffs")}},
			wantErr: domain.ErrInvalidInput,
		},
		{
			name:    "empty snapshot key",
			in:      actionlog.NewEntry{CharacterID: 1, Kind: actionlog.KindBuffChange, Summary: "x", Snapshots: []actionlog.Snapshot{snap("")}},
			wantErr: domain.ErrInvalidInput,
		},
		{
			name:      "round 0 is stamped, summary trimmed, payload defaulted",
			in:        actionlog.NewEntry{CharacterID: 1, Kind: actionlog.KindHPChange, Summary: " orc ", HPDelta: -8},
			round:     3,
			wantStore: &actionlog.NewEntry{CharacterID: 1, Round: 3, Kind: actionlog.KindHPChange, Summary: "orc", HPDelta: -8, Payload: json.RawMessage(`{}`)},
		},
		{
			name:      "explicit round kept",
			in:        actionlog.NewEntry{CharacterID: 1, Round: 7, Kind: actionlog.KindRoundAdvance, Summary: "next", Payload: json.RawMessage(`{"a":1}`), Snapshots: []actionlog.Snapshot{snap("round")}},
			wantStore: &actionlog.NewEntry{CharacterID: 1, Round: 7, Kind: actionlog.KindRoundAdvance, Summary: "next", Payload: json.RawMessage(`{"a":1}`), Snapshots: []actionlog.Snapshot{snap("round")}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, rounds, tx := newSvc(t)
			if c.wantStore != nil {
				if c.in.Round == 0 {
					rounds.EXPECT().GetRound(mock.Anything, c.in.CharacterID).Return(c.round, nil)
				}
				dal.EXPECT().InsertEntry(mock.Anything, *c.wantStore).Return(42, nil)
			}
			id, err := svc.Record(context.Background(), c.in)
			if c.wantErr != nil {
				require.ErrorIs(t, err, c.wantErr)
				assert.Equal(t, 0, tx.calls)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, int64(42), id)
			assert.Equal(t, 1, tx.calls)
		})
	}
}

func TestRecord_PropagatesErrors(t *testing.T) {
	t.Run("round", func(t *testing.T) {
		svc, _, rounds, _ := newSvc(t)
		rounds.EXPECT().GetRound(mock.Anything, int64(1)).Return(0, errBoom)
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Kind: actionlog.KindHPChange, Summary: "x"})
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("insert", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().InsertEntry(mock.Anything, mock.Anything).Return(0, errBoom)
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "x"})
		require.ErrorIs(t, err, errBoom)
	})
}

func hpEntry(id int64, corrections ...actionlog.Correction) *actionlog.Entry {
	return &actionlog.Entry{ID: id, CharacterID: 1, Kind: actionlog.KindHPChange, Summary: "orc", HPDelta: -8, Corrections: corrections}
}

func TestAmend(t *testing.T) {
	voided := actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionVoid}
	cases := []struct {
		name    string
		entry   *actionlog.Entry
		getErr  error
		in      actionlogsvc.Amendment
		wantErr error
		want    *actionlog.Correction
	}{
		{"not found", nil, domain.ErrNotFound, actionlogsvc.Amendment{HPDelta: -1}, domain.ErrNotFound, nil},
		{"voided", hpEntry(5, voided), nil, actionlogsvc.Amendment{HPDelta: -1}, actionlogsvc.ErrAlreadyVoided, nil},
		{"not an hp change", &actionlog.Entry{ID: 5, Kind: actionlog.KindAttack, Summary: "swing"}, nil, actionlogsvc.Amendment{HPDelta: -1}, actionlogsvc.ErrNotAmendable, nil},
		{
			"delta and source", hpEntry(5), nil,
			actionlogsvc.Amendment{HPDelta: -12, Summary: " orc axe "},
			nil,
			&actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionAmend, HPDelta: intp(-12), Summary: "orc axe"},
		},
		{
			"delta to zero keeps source", hpEntry(5), nil,
			actionlogsvc.Amendment{HPDelta: 0},
			nil,
			&actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionAmend, HPDelta: intp(0)},
		},
		{
			"amend twice", hpEntry(5, actionlog.Correction{Kind: actionlog.CorrectionAmend, HPDelta: intp(-3)}), nil,
			actionlogsvc.Amendment{HPDelta: -4},
			nil,
			&actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionAmend, HPDelta: intp(-4)},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, _, tx := newSvc(t)
			dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(c.entry, c.getErr).Once()
			if c.want != nil {
				dal.EXPECT().InsertCorrection(mock.Anything, *c.want).Return(nil)
				dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(hpEntry(5, *c.want), nil).Once()
			}
			got, err := svc.Amend(context.Background(), 1, 5, c.in)
			assert.Equal(t, 1, tx.calls)
			if c.wantErr != nil {
				require.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, actionlog.StatusAmended, got.Status())
		})
	}
}

func TestVoid(t *testing.T) {
	withSnaps := func(keys ...string) *actionlog.Entry {
		e := &actionlog.Entry{ID: 5, CharacterID: 1, Kind: actionlog.KindRoundAdvance, Summary: "next"}
		for _, k := range keys {
			e.Snapshots = append(e.Snapshots, actionlog.Snapshot{Key: k, Before: json.RawMessage(`"before-` + k + `"`), After: json.RawMessage(`"after"`)})
		}
		return e
	}
	voidRow := actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionVoid}

	cases := []struct {
		name        string
		entry       *actionlog.Entry
		laterID     int64 // 0: no later entry touches the state
		restorers   []string
		restoreErr  error
		insertErr   error
		wantErr     error
		wantErrText string
		wantRestore []string
	}{
		{name: "hp change, no snapshots", entry: hpEntry(5)},
		{name: "already voided", entry: hpEntry(5, voidRow), wantErr: actionlogsvc.ErrAlreadyVoided},
		{
			name: "restores every snapshot", entry: withSnaps("round", "buffs"), restorers: []string{"round", "buffs"},
			wantRestore: []string{`"before-round"`, `"before-buffs"`},
		},
		{
			name: "blocked by later entry", entry: withSnaps("round"), laterID: 9, restorers: []string{"round"},
			wantErr: actionlogsvc.ErrVoidBlocked, wantErrText: "entry #9",
		},
		{name: "no restorer registered", entry: withSnaps("buffs"), wantErrText: `no restorer registered for state "buffs"`},
		{name: "restorer fails", entry: withSnaps("round"), restorers: []string{"round"}, restoreErr: errBoom, wantErr: errBoom},
		{name: "insert fails", entry: hpEntry(5), insertErr: errBoom, wantErr: errBoom},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, _, tx := newSvc(t)
			var restored []string
			for _, k := range c.restorers {
				svc.RegisterRestorer(k, restorerFunc(func(_ context.Context, charID int64, state json.RawMessage) error {
					assert.Equal(t, int64(1), charID)
					restored = append(restored, string(state))
					return c.restoreErr
				}))
			}
			dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(c.entry, nil).Once()
			if len(c.entry.Snapshots) > 0 && c.entry.Status() != actionlog.StatusVoided {
				dal.EXPECT().LaterEntryTouchingState(mock.Anything, int64(1), int64(5)).Return(c.laterID, c.laterID != 0, nil)
			}
			reachesInsert := c.laterID == 0 && c.restoreErr == nil && c.entry.Status() != actionlog.StatusVoided &&
				(len(c.entry.Snapshots) == 0 || len(c.restorers) > 0)
			if reachesInsert {
				dal.EXPECT().InsertCorrection(mock.Anything, voidRow).Return(c.insertErr)
			}
			if reachesInsert && c.insertErr == nil {
				done := *c.entry
				done.Corrections = append(done.Corrections, voidRow)
				dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(&done, nil).Once()
			}

			got, err := svc.Void(context.Background(), 1, 5)
			assert.Equal(t, 1, tx.calls)
			if c.wantErr != nil || c.wantErrText != "" {
				require.Error(t, err)
				if c.wantErr != nil {
					assert.ErrorIs(t, err, c.wantErr)
				}
				assert.Contains(t, err.Error(), c.wantErrText)
				if errors.Is(c.wantErr, actionlogsvc.ErrVoidBlocked) {
					assert.Empty(t, restored, "nothing restored when blocked")
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, actionlog.StatusVoided, got.Status())
			assert.Equal(t, c.wantRestore, restored)
		})
	}
}

func TestVoidLatest(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1), actionlog.KindHPChange).Return(0, false, nil)
		ok, err := svc.VoidLatest(context.Background(), 1, actionlog.KindHPChange)
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("voids newest", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1), actionlog.KindHPChange).Return(7, true, nil)
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(7)).Return(hpEntry(7), nil)
		dal.EXPECT().InsertCorrection(mock.Anything, actionlog.Correction{ActionID: 7, Kind: actionlog.CorrectionVoid}).Return(nil)
		ok, err := svc.VoidLatest(context.Background(), 1, actionlog.KindHPChange)
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestList_ClampsLimit(t *testing.T) {
	cases := []struct{ in, want int }{{0, 50}, {-3, 50}, {10, 10}, {9999, 500}}
	for _, c := range cases {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().ListEntries(mock.Anything, int64(1), c.want).Return(nil, nil)
		_, err := svc.List(context.Background(), 1, c.in)
		require.NoError(t, err)
	}
}

func TestEntryStatusAndEffectiveValues(t *testing.T) {
	amend := func(d *int, s string) actionlog.Correction {
		return actionlog.Correction{Kind: actionlog.CorrectionAmend, HPDelta: d, Summary: s}
	}
	void := actionlog.Correction{Kind: actionlog.CorrectionVoid}
	cases := []struct {
		name        string
		corrections []actionlog.Correction
		status      actionlog.Status
		delta       int
		summary     string
	}{
		{"active", nil, actionlog.StatusActive, -8, "orc"},
		{"amended", []actionlog.Correction{amend(intp(-12), "orc axe")}, actionlog.StatusAmended, -12, "orc axe"},
		{"latest amend wins", []actionlog.Correction{amend(intp(-12), "a"), amend(intp(-2), "")}, actionlog.StatusAmended, -2, "orc"},
		{"voided", []actionlog.Correction{void}, actionlog.StatusVoided, 0, "orc"},
		{"amended then voided", []actionlog.Correction{amend(intp(-12), "x"), void}, actionlog.StatusVoided, 0, "x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := hpEntry(1, c.corrections...)
			assert.Equal(t, c.status, e.Status())
			assert.Equal(t, c.delta, e.EffectiveHPDelta())
			assert.Equal(t, c.summary, e.EffectiveSummary())
		})
	}
}
