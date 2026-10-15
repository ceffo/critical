package actionlogsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

// Note-only amend (critical-qki.38): any kind, the correction carries only
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

// Pop-only void (critical-qki.38): only the newest non-voided entry, any
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
			wantErr: actionlogsvc.ErrNotLatest, wantErrText: `only the latest entry can be voided, void "Round 3 → 4" (round 3) first`,
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
						Return(&actionlog.Entry{ID: c.topID, Round: 3, Summary: "Round 3 → 4"}, nil).Once()
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

// VoidLatest pops the top of the stack only when it is of the asked kind.
func TestVoidLatest(t *testing.T) {
	t.Run("empty log", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(0, false, nil).Once()
		ok, err := svc.VoidLatest(context.Background(), 1, actionlog.KindHPChange)
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("pops the newest", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(7, true, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(7)).Return(hpEntry(7), nil).Once()
		dal.EXPECT().InsertCorrection(mock.Anything, actionlog.Correction{ActionID: 7, Kind: actionlog.CorrectionVoid}).Return(nil).Once()
		ok, err := svc.VoidLatest(context.Background(), 1, actionlog.KindHPChange)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("newest is another kind", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
		dal.EXPECT().LatestActiveID(mock.Anything, int64(1)).Return(8, true, nil).Once()
		dal.EXPECT().GetEntry(mock.Anything, int64(1), int64(8)).
			Return(&actionlog.Entry{ID: 8, Round: 2, Kind: actionlog.KindBuffChange, Summary: "Activated Haste"}, nil).Once()
		ok, err := svc.VoidLatest(context.Background(), 1, actionlog.KindHPChange)
		require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)
		assert.Contains(t, err.Error(), `the latest entry is not a hp_change, void "Activated Haste" (round 2) first`)
		assert.False(t, ok)
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
	good := actionlog.PayloadAmendment{Summary: " adjusted ", Payload: json.RawMessage(`{"passed":true}`)}
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
		{name: "no roll at all", entry: saveEntry(5), latest: map[actionlog.Kind]int64{}, wantErr: actionlogsvc.ErrNotLatest},
		{name: "fn fails", entry: saveEntry(5), latest: onlySave, fnErr: errBoom, wantErr: errBoom},
		{name: "empty summary", entry: saveEntry(5), latest: onlySave, fnOut: actionlog.PayloadAmendment{Summary: " ", Payload: good.Payload}, wantErr: domain.ErrInvalidInput},
		{name: "empty payload", entry: saveEntry(5), latest: onlySave, fnOut: actionlog.PayloadAmendment{Summary: "x"}, wantErr: domain.ErrInvalidInput},
		{
			name: "latest roll amended, older attack ignored", entry: saveEntry(5),
			latest: map[actionlog.Kind]int64{actionlog.KindSave: 5, actionlog.KindAttack: 3}, fnOut: good,
			want: &actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionAmend, Summary: "adjusted", Payload: good.Payload},
		},
		{
			name: "amended again", entry: saveEntry(5, actionlog.Correction{Kind: actionlog.CorrectionAmend, Summary: "a", Payload: json.RawMessage(`{}`)}), latest: onlySave, fnOut: good,
			want: &actionlog.Correction{ActionID: 5, Kind: actionlog.CorrectionAmend, Summary: "adjusted", Payload: good.Payload},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, dal, _, tx := newSvc(t)
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
			assert.Equal(t, "adjusted", got.EffectiveSummary())
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

// HP-1/HP-2 (critical-qki.38): Record caps a heal at the gap to max HP, reading
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
	t.Run("damage is stored as is, total not read", func(t *testing.T) {
		svc, dal, _, _ := newSvc(t)
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
