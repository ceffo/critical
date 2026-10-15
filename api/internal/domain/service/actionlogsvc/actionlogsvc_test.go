package actionlogsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc/mocks"
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
	svc, dal, rounds, tx, _ := newSvcMaxHP(t)
	return svc, dal, rounds, tx
}

// newSvcMaxHP also returns the max HP port, which only damage (the HP floor)
// reads.
func newSvcMaxHP(t *testing.T) (actionlogsvc.SVC, *mocks.MockDAL, *mocks.MockRounds, *passTx, *mocks.MockMaxHP) {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	rounds := mocks.NewMockRounds(t)
	maxHP := mocks.NewMockMaxHP(t)
	tx := &passTx{}
	svc, err := actionlogsvc.NewWithConfig(actionlogsvc.DefaultConfig(), dal, tx, rounds, maxHP)
	require.NoError(t, err)
	return svc, dal, rounds, tx, maxHP
}

func intp(v int) *int { return &v }

func TestNewWithConfig_Validation(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	rounds := mocks.NewMockRounds(t)
	maxHP := mocks.NewMockMaxHP(t)
	cases := []struct {
		name   string
		cfg    *actionlogsvc.Config
		dal    actionlogsvc.DAL
		tx     domain.TxRunner
		rounds actionlogsvc.Rounds
		maxHP  actionlogsvc.MaxHP
	}{
		{"nil config", nil, dal, &passTx{}, rounds, maxHP},
		{"nil dal", actionlogsvc.DefaultConfig(), nil, &passTx{}, rounds, maxHP},
		{"nil tx", actionlogsvc.DefaultConfig(), dal, nil, rounds, maxHP},
		{"nil rounds", actionlogsvc.DefaultConfig(), dal, &passTx{}, nil, maxHP},
		{"nil max hp", actionlogsvc.DefaultConfig(), dal, &passTx{}, rounds, nil},
		{"default above max", &actionlogsvc.Config{DefaultLimit: 10, MaxLimit: 5}, dal, &passTx{}, rounds, maxHP},
		{"zero limit", &actionlogsvc.Config{DefaultLimit: 0, MaxLimit: 5}, dal, &passTx{}, rounds, maxHP},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := actionlogsvc.NewWithConfig(c.cfg, c.dal, c.tx, c.rounds, c.maxHP)
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
			name:    "summary or payload required",
			in:      actionlog.NewEntry{CharacterID: 1, Kind: actionlog.KindHPChange, Summary: "  "},
			wantErr: domain.ErrInvalidInput,
		},
		{
			name:      "a payload without a summary (the UI words it)",
			in:        actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: actionlog.KindBuffChange, Payload: json.RawMessage(`{"action":"dismiss"}`)},
			wantStore: &actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: actionlog.KindBuffChange, Payload: json.RawMessage(`{"action":"dismiss"}`)},
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
			svc, dal, rounds, tx, maxHP := newSvcMaxHP(t)
			if c.wantStore != nil {
				if c.in.HPDelta < 0 { // at full HP, far from the floor
					maxHP.EXPECT().GetMaxHP(mock.Anything, c.in.CharacterID).Return(40, nil).Once()
					dal.EXPECT().HPTotal(mock.Anything, c.in.CharacterID).Return(0, nil).Once()
				}
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

// Note-only amend: any kind, the correction carries only
// the trimmed note.
func TestAmend(t *testing.T) {
	voided := actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionVoid}
	note := func(s string) *string { return &s }
	cases := []struct {
		name    string
		entry   *actionlog.Entry
		getErr  error
		in      string
		wantErr error
		want    *actionlog.Correction
	}{
		{"not found", nil, domain.ErrNotFound, "x", domain.ErrNotFound, nil},
		{"voided", hpEntry(5, voided), nil, "x", actionlogsvc.ErrAlreadyVoided, nil},
		{
			"hp change note", hpEntry(5), nil, " axe, not club ", nil,
			&actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionAmend, Note: note("axe, not club")},
		},
		{
			"buff change note", &actionlog.Entry{ID: 5, CharacterID: 1, Kind: actionlog.KindBuffChange, Summary: "Activated Bless"}, nil, "DM ruling", nil,
			&actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionAmend, Note: note("DM ruling")},
		},
		{
			"attack note", &actionlog.Entry{ID: 5, CharacterID: 1, Kind: actionlog.KindAttack, Summary: "swing"}, nil, "flanking", nil,
			&actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionAmend, Note: note("flanking")},
		},
		{
			"empty clears", hpEntry(5), nil, "  ", nil,
			&actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionAmend, Note: note("")},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, _, tx := newSvc(t)
			dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(c.entry, c.getErr).Once()
			if c.want != nil {
				dal.EXPECT().InsertCorrection(mock.Anything, *c.want).Return(nil).Once()
				done := *c.entry
				done.Corrections = append(done.Corrections, *c.want)
				dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(&done, nil).Once()
			}
			got, err := svc.Amend(context.Background(), 1, 5, actionlogsvc.Amendment{Note: c.in})
			assert.Equal(t, 1, tx.calls)
			if c.wantErr != nil {
				require.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, *c.want.Note, got.Note())
			assert.Equal(t, c.entry.EffectiveHPDelta(), got.EffectiveHPDelta(), "values untouched")
			assert.Equal(t, c.entry.Status(), got.Status(), "a note is not a value amend")
		})
	}
	t.Run("too long", func(t *testing.T) {
		svc, _, _, tx := newSvc(t)
		_, err := svc.Amend(context.Background(), 1, 5, actionlogsvc.Amendment{Note: strings.Repeat("é", 501)})
		require.ErrorIs(t, err, domain.ErrInvalidInput)
		assert.Zero(t, tx.calls)
	})
}

// Pop-only void: only the newest non-voided entry, any
// kind; its snapshots are restored through their family's restorer.
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
		topID       int64 // newest non-voided entry; 0 = this one
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
			name: "an older entry is refused", entry: withSnaps("round"), topID: 9, restorers: []string{"round"},
			wantErr: actionlogsvc.ErrNotLatest, wantErrText: `only the latest entry can be undone, undo #9 (round_advance, round 3) first`,
		},
		{
			name: "a scoped key uses its family's restorer", entry: withSnaps("items:arrows_plus3"), restorers: []string{"items"},
			wantRestore: []string{`"before-items:arrows_plus3"`},
		},
		{name: "no restorer registered", entry: withSnaps("buffs"), wantErrText: `no restorer registered for state "buffs"`},
		{name: "no restorer for a scoped key's family", entry: withSnaps("items:x"), restorers: []string{"item"}, wantErrText: `no restorer registered for state "items:x"`},
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
			active := c.entry.Status() != actionlog.StatusVoided
			if active {
				if c.topID != 0 {
					dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(c.topID, true, nil).Once()
					dal.EXPECT().GetEntry(mock.Anything, int64(1), c.topID).
						Return(&actionlog.Entry{ID: c.topID, Round: 3, Kind: actionlog.KindRoundAdvance}, nil).Once()
				} else {
					dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(5, true, nil).Once()
					dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(c.entry, nil).Once()
				}
			}
			reachesInsert := active && c.topID == 0 && c.restoreErr == nil &&
				(len(c.entry.Snapshots) == 0 || len(c.restorers) > 0) && c.wantErrText == ""
			if reachesInsert {
				dal.EXPECT().InsertCorrection(mock.Anything, voidRow).Return(c.insertErr).Once()
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
				if errors.Is(c.wantErr, actionlogsvc.ErrNotLatest) {
					assert.Empty(t, restored, "nothing restored when refused")
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, actionlog.StatusVoided, got.Status())
			assert.Equal(t, c.wantRestore, restored)
		})
	}
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
		// Each value comes from the latest amend that set it (an empty summary
		// keeps the current one), so a later note-only amend hides nothing.
		{"latest amend wins", []actionlog.Correction{amend(intp(-12), "a"), amend(intp(-2), "")}, actionlog.StatusAmended, -2, "a"},
		{"note only stays active", []actionlog.Correction{{Kind: actionlog.CorrectionAmend, Note: new(string)}}, actionlog.StatusActive, -8, "orc"},
		{"note after a value amend", []actionlog.Correction{amend(intp(-12), "orc axe"), {Kind: actionlog.CorrectionAmend, Note: new(string)}}, actionlog.StatusAmended, -12, "orc axe"},
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

func TestFamily(t *testing.T) {
	assert.Equal(t, "items", actionlog.Family("items:arrows_plus3"))
	assert.Equal(t, "buffs", actionlog.Family("buffs"))
}

func saveEntry(id int64, corrections ...actionlog.Correction) *actionlog.Entry {
	return &actionlog.Entry{ID: id, CharacterID: 1, Kind: actionlog.KindSave, Summary: "save", Payload: json.RawMessage(`{"passed":false}`), Corrections: corrections}
}

func TestAmendLatest(t *testing.T) {
	voided := actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionVoid}
	good := actionlog.PayloadAmendment{Payload: json.RawMessage(`{"passed":true}`), Change: actionlog.Change{Field: "saveAdjustment", From: 0, To: 2}}
	step := int64(9)
	onlySave := map[actionlog.Kind]int64{actionlog.KindSave: 5}
	cases := []struct {
		name    string
		entry   *actionlog.Entry
		getErr  error
		kind    actionlog.Kind // what the caller amends; default save
		latest  map[actionlog.Kind]int64
		fnOut   actionlog.PayloadAmendment
		fnErr   error
		wantErr error
		want    *actionlog.Correction
	}{
		{name: "not found", getErr: domain.ErrNotFound, wantErr: domain.ErrNotFound},
		{name: "not a save", entry: hpEntry(5), wantErr: actionlogsvc.ErrNotAmendable},
		{name: "not a roll kind", entry: hpEntry(5), kind: actionlog.KindHPChange, wantErr: actionlogsvc.ErrNotAmendable},
		{name: "voided", entry: saveEntry(5, voided), wantErr: actionlogsvc.ErrAlreadyVoided},
		{name: "a later save exists", entry: saveEntry(5), latest: map[actionlog.Kind]int64{actionlog.KindSave: 8}, wantErr: actionlogsvc.ErrNotLatest},
		{name: "a later attack exists", entry: saveEntry(5), latest: map[actionlog.Kind]int64{actionlog.KindSave: 5, actionlog.KindAttack: 6}, wantErr: actionlogsvc.ErrNotLatest},
		{name: "a later cast exists", entry: saveEntry(5), latest: map[actionlog.Kind]int64{actionlog.KindSave: 5, actionlog.KindCast: 9}, wantErr: actionlogsvc.ErrNotLatest},
		{name: "a later item use exists", entry: saveEntry(5), latest: map[actionlog.Kind]int64{actionlog.KindSave: 5, actionlog.KindItemUse: 7}, wantErr: actionlogsvc.ErrNotLatest},
		{name: "a later skill check exists", entry: saveEntry(5), latest: map[actionlog.Kind]int64{actionlog.KindSave: 5, actionlog.KindSkillCheck: 7}, wantErr: actionlogsvc.ErrNotLatest},
		{name: "no roll at all", entry: saveEntry(5), latest: map[actionlog.Kind]int64{}, wantErr: actionlogsvc.ErrNotLatest},
		{name: "fn fails", entry: saveEntry(5), latest: onlySave, fnErr: errBoom, wantErr: errBoom},
		{name: "empty payload", entry: saveEntry(5), latest: onlySave, fnOut: actionlog.PayloadAmendment{Change: good.Change}, wantErr: domain.ErrInvalidInput},
		{name: "no changed field", entry: saveEntry(5), latest: onlySave, fnOut: actionlog.PayloadAmendment{Payload: good.Payload}, wantErr: domain.ErrInvalidInput},
		{
			name: "latest roll amended, older attack ignored", entry: saveEntry(5),
			latest: map[actionlog.Kind]int64{actionlog.KindSave: 5, actionlog.KindAttack: 3}, fnOut: good,
			want: &actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionAmend, Payload: good.Payload, StepID: &step},
		},
		{
			name: "amended again", entry: saveEntry(5, actionlog.Correction{Kind: actionlog.CorrectionAmend, Summary: "a", Payload: json.RawMessage(`{}`)}), latest: onlySave, fnOut: good,
			want: &actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionAmend, Payload: good.Payload, StepID: &step},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, rounds, tx := newSvc(t)
			kind := c.kind
			if kind == "" {
				kind = actionlog.KindSave
			}
			dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(c.entry, c.getErr).Once()
			if c.latest != nil {
				for _, k := range actionlog.RollKinds {
					id := c.latest[k]
					dal.EXPECT().LatestID(mock.Anything, int64(1), k).Return(id, id != 0, nil)
				}
			}
			if c.want != nil {
				rounds.EXPECT().GetRound(mock.Anything, int64(1)).Return(3, nil).Once()
				dal.EXPECT().InsertEntry(mock.Anything, mock.Anything).Return(step, nil).Once()
				dal.EXPECT().InsertCorrection(mock.Anything, *c.want).Return(nil)
				dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(saveEntry(5, *c.want), nil).Once()
			}
			called := false
			got, err := svc.AmendLatest(context.Background(), 1, 5, kind, func(e actionlog.Entry) (actionlog.PayloadAmendment, error) {
				called = true
				assert.Equal(t, int64(5), e.ID)
				return c.fnOut, c.fnErr
			})
			assert.Equal(t, 1, tx.calls)
			if c.wantErr != nil {
				require.ErrorIs(t, err, c.wantErr)
				if errors.Is(c.wantErr, actionlogsvc.ErrNotLatest) {
					assert.Contains(t, err.Error(), "only the latest roll can be adjusted")
				}
				return
			}
			require.NoError(t, err)
			assert.True(t, called)
			assert.JSONEq(t, `{"passed":true}`, string(got.EffectivePayload()))
			assert.Equal(t, 0, got.EffectiveHPDelta())
		})
	}

	t.Run("latest lookup fails", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(saveEntry(5), nil)
		dal.EXPECT().LatestID(mock.Anything, int64(1), actionlog.RollKinds[0]).Return(0, false, errBoom)
		_, err := svc.AmendLatest(context.Background(), 1, 5, actionlog.KindSave, nil)
		require.ErrorIs(t, err, errBoom)
	})
}

// ADR 0010 step 6: a correction is its own step. AmendLatest records a
// correction entry (stamped with the current round, no snapshot, its payload
// naming the corrected entry and the change) before the amend, which points
// at it; when either write fails nothing else is written.
func TestAmendLatest_RecordsTheCorrectionStep(t *testing.T) {
	change := actionlog.PayloadAmendment{Payload: json.RawMessage(`{"passed":true}`), Change: actionlog.Change{Field: "targetAc", From: nil, To: 4}}
	expect := func(dal *mocks.MockDAL) {
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(saveEntry(5), nil).Once()
		for _, k := range actionlog.RollKinds {
			dal.EXPECT().LatestID(mock.Anything, int64(1), k).Return(5, k == actionlog.KindSave, nil)
		}
	}
	amend := func(svc actionlogsvc.SVC) (*actionlog.Entry, error) {
		return svc.AmendLatest(context.Background(), 1, 5, actionlog.KindSave, func(actionlog.Entry) (actionlog.PayloadAmendment, error) {
			return change, nil
		})
	}

	t.Run("step then amend", func(t *testing.T) {
		svc, dal, rounds, tx := newSvc(t)
		expect(dal)
		rounds.EXPECT().GetRound(mock.Anything, int64(1)).Return(3, nil).Once()
		var calls []string
		dal.EXPECT().InsertEntry(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, e actionlog.NewEntry) (int64, error) {
			calls = append(calls, "step")
			assert.Equal(t, actionlog.NewEntry{CharacterID: 1, Round: 3, Kind: actionlog.KindCorrection, Payload: e.Payload}, e)
			assert.JSONEq(t, `{"entryId":5,"entryKind":"save","field":"targetAc","from":null,"to":4}`, string(e.Payload))
			return 12, nil
		}).Once()
		dal.EXPECT().InsertCorrection(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, c actionlog.Correction) error {
			calls = append(calls, "amend")
			require.NotNil(t, c.StepID)
			assert.Equal(t, int64(12), *c.StepID)
			return nil
		}).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(5)).Return(saveEntry(5), nil).Once()
		_, err := amend(svc)
		require.NoError(t, err)
		assert.Equal(t, []string{"step", "amend"}, calls)
		assert.Equal(t, 1, tx.calls, "the checks and both writes share one transaction")
	})
	t.Run("round fails: nothing written", func(t *testing.T) {
		svc, dal, rounds, _ := newSvc(t)
		expect(dal)
		rounds.EXPECT().GetRound(mock.Anything, int64(1)).Return(0, errBoom).Once()
		_, err := amend(svc)
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("step insert fails: no amend", func(t *testing.T) {
		svc, dal, rounds, _ := newSvc(t)
		expect(dal)
		rounds.EXPECT().GetRound(mock.Anything, int64(1)).Return(3, nil).Once()
		dal.EXPECT().InsertEntry(mock.Anything, mock.Anything).Return(0, errBoom).Once()
		_, err := amend(svc)
		require.ErrorIs(t, err, errBoom)
	})
}

// ADR 0010 step 6: an amend whose correction step was popped is out of
// effect: the payload, the status and the HP delta read as before it. An
// amend without a step (written before steps existed) stays in effect.
func TestEntry_PoppedStepAmendIsOutOfEffect(t *testing.T) {
	step := func(id int64, undone bool, p string) actionlog.Correction {
		return actionlog.Correction{Kind: actionlog.CorrectionAmend, Payload: json.RawMessage(p), StepID: &id, StepUndone: undone}
	}
	legacy := actionlog.Correction{Kind: actionlog.CorrectionAmend, Payload: json.RawMessage(`{"a":0}`)}
	cases := []struct {
		name        string
		corrections []actionlog.Correction
		payload     string
		status      actionlog.Status
	}{
		{"step in effect", []actionlog.Correction{step(6, false, `{"a":1}`)}, `{"a":1}`, actionlog.StatusAmended},
		{"newest step popped", []actionlog.Correction{step(6, false, `{"a":1}`), step(7, true, `{"a":2}`)}, `{"a":1}`, actionlog.StatusAmended},
		{"every step popped", []actionlog.Correction{step(6, true, `{"a":1}`), step(7, true, `{"a":2}`)}, `{"passed":false}`, actionlog.StatusActive},
		{"legacy amend under a popped step", []actionlog.Correction{legacy, step(7, true, `{"a":2}`)}, `{"a":0}`, actionlog.StatusAmended},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := saveEntry(5, c.corrections...)
			assert.JSONEq(t, c.payload, string(e.EffectivePayload()))
			assert.Equal(t, c.status, e.Status())
		})
	}
	popped := hpEntry(1, actionlog.Correction{Kind: actionlog.CorrectionAmend, HPDelta: intp(-3), StepID: new(int64(4)), StepUndone: true})
	assert.Equal(t, -8, popped.EffectiveHPDelta())
}

func TestEntryEffectivePayload(t *testing.T) {
	amend := func(p string) actionlog.Correction {
		c := actionlog.Correction{Kind: actionlog.CorrectionAmend}
		if p != "" {
			c.Payload = json.RawMessage(p)
		}
		return c
	}
	cases := []struct {
		name        string
		corrections []actionlog.Correction
		want        string
	}{
		{"original", nil, `{"passed":false}`},
		{"amended", []actionlog.Correction{amend(`{"passed":true}`)}, `{"passed":true}`},
		{"latest amend with a payload wins", []actionlog.Correction{amend(`{"a":1}`), amend(`{"a":2}`), amend("")}, `{"a":2}`},
		{"void keeps the effective payload", []actionlog.Correction{amend(`{"a":1}`), {Kind: actionlog.CorrectionVoid}}, `{"a":1}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.JSONEq(t, c.want, string(saveEntry(1, c.corrections...).EffectivePayload()))
		})
	}
}

// HP-1/HP-2: Record caps a heal at the gap to max HP, reading
// the total exactly once inside its transaction, and stores the breakdown.
// Damage never reads the total.
func TestRecord_CapsHeal(t *testing.T) {
	cases := []struct {
		name        string
		total       int // SUM before the entry; 0 = at max
		delta       int
		payload     string
		wantDelta   int
		wantPayload string
	}{
		{"heal at max-1", -1, 6, "", 1, `{"hpRequested":6,"hpApplied":1}`},
		{"heal exactly to max", -7, 7, "", 7, `{"hpRequested":7,"hpApplied":7}`},
		{"heal bigger than the gap", -5, 12, "", 5, `{"hpRequested":12,"hpApplied":5}`},
		{"heal at full HP logs 0", 0, 9, "", 0, `{"hpRequested":9,"hpApplied":0}`},
		{"payload kept", -3, 11, `{"spell":"cure"}`, 3, `{"spell":"cure","hpRequested":11,"hpApplied":3}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, _, tx := newSvc(t)
			dal.EXPECT().HPTotal(mock.Anything, int64(1)).Return(c.total, nil).Once()
			var stored actionlog.NewEntry
			dal.EXPECT().InsertEntry(mock.Anything, mock.Anything).
				Run(func(_ context.Context, e actionlog.NewEntry) { stored = e }).Return(42, nil).Once()
			_, err := svc.Record(context.Background(), actionlog.NewEntry{
				CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "potion", HPDelta: c.delta, Payload: json.RawMessage(c.payload),
			})
			require.NoError(t, err)
			assert.Equal(t, 1, tx.calls)
			assert.Equal(t, c.wantDelta, stored.HPDelta)
			assert.JSONEq(t, c.wantPayload, string(stored.Payload))
		})
	}
	t.Run("damage does not go through the cap", func(t *testing.T) {
		svc, dal, _, _, maxHP := newSvcMaxHP(t)
		maxHP.EXPECT().GetMaxHP(mock.Anything, int64(1)).Return(40, nil).Once()
		dal.EXPECT().HPTotal(mock.Anything, int64(1)).Return(0, nil).Once()
		dal.EXPECT().InsertEntry(mock.Anything, actionlog.NewEntry{
			CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "orc", HPDelta: -13, Payload: json.RawMessage(`{}`),
		}).Return(42, nil).Once()
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "orc", HPDelta: -13})
		require.NoError(t, err)
	})
	t.Run("total fails", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().HPTotal(mock.Anything, int64(1)).Return(0, errBoom).Once()
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "potion", HPDelta: 4})
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("payload not an object", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().HPTotal(mock.Anything, int64(1)).Return(-6, nil).Once()
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "potion", HPDelta: 4, Payload: json.RawMessage(`[1]`)})
		require.ErrorIs(t, err, domain.ErrInvalidInput)
	})
}

// ADR 0011 step 1: damage that would take HP below -10 is logged clamped, in
// the recording transaction, with the requested vs applied breakdown. Max HP
// is 40, so current HP is 40+total.
func TestRecord_FloorsDamage(t *testing.T) {
	cases := []struct {
		name        string
		total       int // SUM before the entry
		delta       int
		payload     string
		wantDelta   int
		wantPayload string
	}{
		{"damage above the floor is stored as is", 0, -13, "", -13, `{}`},
		{"damage below 0 but above the floor is stored as is", -35, -12, "", -12, `{}`},
		{"damage exactly to the floor is stored as is", -30, -20, "", -20, `{}`},
		{"damage past the floor is clamped", -30, -25, "", -20, `{"hpRequested":-25,"hpApplied":-20}`},
		{"damage at the floor logs 0", -50, -6, "", 0, `{"hpRequested":-6,"hpApplied":0}`},
		{"damage below the floor (legacy rows) logs 0", -55, -3, "", 0, `{"hpRequested":-3,"hpApplied":0}`},
		{"payload kept", -45, -9, `{"weapon":"club"}`, -5, `{"weapon":"club","hpRequested":-9,"hpApplied":-5}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, _, tx, maxHP := newSvcMaxHP(t)
			maxHP.EXPECT().GetMaxHP(mock.Anything, int64(1)).Return(40, nil).Once()
			dal.EXPECT().HPTotal(mock.Anything, int64(1)).Return(c.total, nil).Once()
			var stored actionlog.NewEntry
			dal.EXPECT().InsertEntry(mock.Anything, mock.Anything).
				Run(func(_ context.Context, e actionlog.NewEntry) { stored = e }).Return(42, nil).Once()
			_, err := svc.Record(context.Background(), actionlog.NewEntry{
				CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "ogre", HPDelta: c.delta, Payload: json.RawMessage(c.payload),
			})
			require.NoError(t, err)
			assert.Equal(t, 1, tx.calls)
			assert.Equal(t, c.wantDelta, stored.HPDelta)
			assert.JSONEq(t, c.wantPayload, string(stored.Payload))
		})
	}
	t.Run("max hp fails", func(t *testing.T) {
		svc, _, _, _, maxHP := newSvcMaxHP(t)
		maxHP.EXPECT().GetMaxHP(mock.Anything, int64(1)).Return(0, errBoom).Once()
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "ogre", HPDelta: -4})
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("total fails", func(t *testing.T) {
		svc, dal, _, _, maxHP := newSvcMaxHP(t)
		maxHP.EXPECT().GetMaxHP(mock.Anything, int64(1)).Return(40, nil).Once()
		dal.EXPECT().HPTotal(mock.Anything, int64(1)).Return(0, errBoom).Once()
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "ogre", HPDelta: -4})
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("payload not an object", func(t *testing.T) {
		svc, dal, _, _, maxHP := newSvcMaxHP(t)
		maxHP.EXPECT().GetMaxHP(mock.Anything, int64(1)).Return(40, nil).Once()
		dal.EXPECT().HPTotal(mock.Anything, int64(1)).Return(-48, nil).Once()
		_, err := svc.Record(context.Background(), actionlog.NewEntry{CharacterID: 1, Round: 2, Kind: actionlog.KindHPChange, Summary: "ogre", HPDelta: -4, Payload: json.RawMessage(`[1]`)})
		require.ErrorIs(t, err, domain.ErrInvalidInput)
	})
}
