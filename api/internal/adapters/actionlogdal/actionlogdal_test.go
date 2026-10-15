package actionlogdal_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/actionlogdal"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background()))
	return db
}

func seedCharacter(t *testing.T, db *sqlite.DB) int64 {
	t.Helper()
	res, err := db.ExecContext(context.Background(),
		`INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES ('Test', 10, 10, 10, 10, 10, 10, 30, 19)`)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	return id
}

func intp(v int) *int { return &v }

func hp(charID int64, delta int, source string) actionlog.NewEntry {
	return actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindHPChange, Summary: source, HPDelta: delta}
}

func insert(t *testing.T, dal *actionlogdal.DAL, e actionlog.NewEntry) int64 {
	t.Helper()
	id, err := dal.InsertEntry(context.Background(), e)
	require.NoError(t, err)
	return id
}

func correct(t *testing.T, dal *actionlogdal.DAL, c actionlog.Correction) {
	t.Helper()
	require.NoError(t, dal.InsertCorrection(context.Background(), c))
}

func TestHPTotal(t *testing.T) {
	amend := func(id int64, d int) actionlog.Correction {
		return actionlog.Correction{ActionID: id, Kind: actionlog.CorrectionAmend, HPDelta: intp(d)}
	}
	void := func(id int64) actionlog.Correction {
		return actionlog.Correction{ActionID: id, Kind: actionlog.CorrectionVoid}
	}
	cases := []struct {
		name        string
		deltas      []int
		corrections func(ids []int64) []actionlog.Correction
		want        int
	}{
		{"empty", nil, nil, 0},
		{"plain sum", []int{-8, 3, -2}, nil, -7},
		{"amend replaces delta", []int{-8, 3}, func(ids []int64) []actionlog.Correction {
			return []actionlog.Correction{amend(ids[0], -12)}
		}, -9},
		{"latest amend wins", []int{-8}, func(ids []int64) []actionlog.Correction {
			return []actionlog.Correction{amend(ids[0], -12), amend(ids[0], -1)}
		}, -1},
		{"amend to zero", []int{-8, -2}, func(ids []int64) []actionlog.Correction {
			return []actionlog.Correction{amend(ids[0], 0)}
		}, -2},
		{"void counts zero", []int{-8, 3}, func(ids []int64) []actionlog.Correction {
			return []actionlog.Correction{void(ids[0])}
		}, 3},
		{"void after amend counts zero", []int{-8, 3}, func(ids []int64) []actionlog.Correction {
			return []actionlog.Correction{amend(ids[0], -20), void(ids[0])}
		}, 3},
		{"summary-only amend keeps delta", []int{-8}, func(ids []int64) []actionlog.Correction {
			return []actionlog.Correction{{ActionID: ids[0], Kind: actionlog.CorrectionAmend, Summary: "orc"}}
		}, -8},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := openDB(t)
			charID := seedCharacter(t, db)
			other := seedCharacter(t, db)
			dal := actionlogdal.New(db)
			insert(t, dal, hp(other, -100, "other character"))
			var ids []int64
			for _, d := range c.deltas {
				ids = append(ids, insert(t, dal, hp(charID, d, "x")))
			}
			insert(t, dal, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindAttack, Summary: "swing"})
			if c.corrections != nil {
				for _, corr := range c.corrections(ids) {
					correct(t, dal, corr)
				}
			}
			total, err := dal.HPTotal(context.Background(), charID)
			require.NoError(t, err)
			assert.Equal(t, c.want, total)
		})
	}
}

func TestGetEntry_WithSnapshotsAndCorrections(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := actionlogdal.New(db)
	ctx := context.Background()

	id := insert(t, dal, actionlog.NewEntry{
		CharacterID: charID, Round: 3, Kind: actionlog.KindBuffChange, Summary: "bless",
		Payload: json.RawMessage(`{"buff":"bless"}`),
		Snapshots: []actionlog.Snapshot{
			{Key: "buffs", Before: json.RawMessage(`[]`), After: json.RawMessage(`[{"id":"b"}]`)},
			{Key: "round", Before: json.RawMessage(`2`), After: json.RawMessage(`3`)},
		},
	})
	correct(t, dal, actionlog.Correction{ActionID: id, Kind: actionlog.CorrectionVoid})

	e, err := dal.GetEntry(ctx, charID, id)
	require.NoError(t, err)
	assert.Equal(t, 3, e.Round)
	assert.Equal(t, actionlog.KindBuffChange, e.Kind)
	assert.JSONEq(t, `{"buff":"bless"}`, string(e.Payload))
	require.Len(t, e.Snapshots, 2)
	assert.Equal(t, "buffs", e.Snapshots[0].Key)
	assert.JSONEq(t, `[{"id":"b"}]`, string(e.Snapshots[0].After))
	require.Len(t, e.Corrections, 1)
	assert.Equal(t, actionlog.StatusVoided, e.Status())

	_, err = dal.GetEntry(ctx, charID+1, id)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = dal.GetEntry(ctx, charID, 999)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListEntries(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := actionlogdal.New(db)
	ctx := context.Background()

	a := insert(t, dal, hp(charID, -8, "orc"))
	b := insert(t, dal, hp(charID, 3, "potion"))
	c := insert(t, dal, hp(charID, -5, "goblin"))
	correct(t, dal, actionlog.Correction{ActionID: a, Kind: actionlog.CorrectionAmend, HPDelta: intp(-6), Summary: "orc (fixed)"})
	correct(t, dal, actionlog.Correction{ActionID: c, Kind: actionlog.CorrectionVoid})

	all, err := dal.ListEntries(ctx, charID, 10)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, []int64{c, b, a}, []int64{all[0].ID, all[1].ID, all[2].ID})
	assert.Equal(t, actionlog.StatusVoided, all[0].Status())
	assert.Equal(t, actionlog.StatusActive, all[1].Status())
	assert.Equal(t, actionlog.StatusAmended, all[2].Status())
	assert.Equal(t, -8, all[2].HPDelta, "original stays visible")
	assert.Equal(t, -6, all[2].EffectiveHPDelta())
	assert.Equal(t, "orc (fixed)", all[2].EffectiveSummary())

	limited, err := dal.ListEntries(ctx, charID, 2)
	require.NoError(t, err)
	assert.Len(t, limited, 2)

	active, err := dal.ListActiveEntries(ctx, charID, actionlog.KindHPChange, 10)
	require.NoError(t, err)
	require.Len(t, active, 2)
	assert.Equal(t, []int64{b, a}, []int64{active[0].ID, active[1].ID})
	assert.Equal(t, -6, active[1].EffectiveHPDelta())

	id, ok, err := dal.LatestActiveID(ctx, charID)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, b, id, "c is voided: b is the top of the stack")
	_, ok, err = dal.LatestActiveID(ctx, charID+1)
	require.NoError(t, err)
	assert.False(t, ok, "another character's log is empty")
}

// A note amend round-trips its note, leaves HP, summary,
// payload and status alone, and does not hide an earlier value amend (an
// hp_change amended before amends became note-only).
func TestNoteAmend_RoundTrip(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := actionlogdal.New(db)
	ctx := context.Background()
	note := func(s string) *string { return &s }

	legacy := insert(t, dal, hp(charID, -8, "orc"))
	correct(t, dal, actionlog.Correction{ActionID: legacy, Kind: actionlog.CorrectionAmend, HPDelta: intp(-11), Summary: "orc axe"})
	correct(t, dal, actionlog.Correction{ActionID: legacy, Kind: actionlog.CorrectionAmend, Note: note("DM said 11")})
	heal := insert(t, dal, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindHPChange, Summary: "potion", HPDelta: 4, Payload: json.RawMessage(`{"hpRequested":4,"hpApplied":4}`)})
	correct(t, dal, actionlog.Correction{ActionID: heal, Kind: actionlog.CorrectionAmend, Note: note("first")})
	correct(t, dal, actionlog.Correction{ActionID: heal, Kind: actionlog.CorrectionAmend, Note: note("second")})

	l, err := dal.GetEntry(ctx, charID, legacy)
	require.NoError(t, err)
	assert.Equal(t, -11, l.EffectiveHPDelta(), "the note does not hide the value amend")
	assert.Equal(t, "orc axe", l.EffectiveSummary())
	assert.Equal(t, "DM said 11", l.Note())
	assert.Equal(t, actionlog.StatusAmended, l.Status())

	h, err := dal.GetEntry(ctx, charID, heal)
	require.NoError(t, err)
	assert.Equal(t, "second", h.Note(), "latest note wins")
	assert.Equal(t, 4, h.EffectiveHPDelta())
	assert.Equal(t, "potion", h.EffectiveSummary())
	assert.JSONEq(t, `{"hpRequested":4,"hpApplied":4}`, string(h.EffectivePayload()))
	assert.Equal(t, actionlog.StatusActive, h.Status(), "a note is not a value amend")
	require.Len(t, h.Corrections, 2)
	assert.Nil(t, h.Corrections[0].HPDelta)

	total, err := dal.HPTotal(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, -7, total)
}

// A save adjustment is an amend with a payload and no HP delta: it round-trips
// as the effective payload and leaves current HP alone.
func TestPayloadAmend_RoundTripAndHPUnchanged(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := actionlogdal.New(db)
	ctx := context.Background()

	insert(t, dal, hp(charID, -8, "orc"))
	id := insert(t, dal, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindSave, Summary: "save, FAILED", Payload: json.RawMessage(`{"passed":false}`)})
	correct(t, dal, actionlog.Correction{ActionID: id, Kind: actionlog.CorrectionAmend, Summary: "save, adjusted +4: FAILED → PASSED", Payload: json.RawMessage(`{"passed":true}`)})

	e, err := dal.GetEntry(ctx, charID, id)
	require.NoError(t, err)
	require.Len(t, e.Corrections, 1)
	assert.Nil(t, e.Corrections[0].HPDelta)
	assert.JSONEq(t, `{"passed":true}`, string(e.Corrections[0].Payload))
	assert.JSONEq(t, `{"passed":true}`, string(e.EffectivePayload()))
	assert.JSONEq(t, `{"passed":false}`, string(e.Payload), "the entry itself is never edited")
	assert.Equal(t, "save, adjusted +4: FAILED → PASSED", e.EffectiveSummary())

	total, err := dal.HPTotal(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, -8, total)

	// An amend without a payload (HP) stores NULL and reads back empty.
	hpID := insert(t, dal, hp(charID, -2, "rat"))
	correct(t, dal, actionlog.Correction{ActionID: hpID, Kind: actionlog.CorrectionAmend, HPDelta: intp(-3)})
	h, err := dal.GetEntry(ctx, charID, hpID)
	require.NoError(t, err)
	assert.Empty(t, h.Corrections[0].Payload)
}

// LatestID counts voided entries too, so an older save never becomes the
// latest by voiding a newer one.
func TestLatestID(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := actionlogdal.New(db)
	ctx := context.Background()
	save := actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindSave, Summary: "save"}

	_, ok, err := dal.LatestID(ctx, charID, actionlog.KindSave)
	require.NoError(t, err)
	assert.False(t, ok)

	first := insert(t, dal, save)
	second := insert(t, dal, save)
	insert(t, dal, hp(charID, -1, "later hp"))
	correct(t, dal, actionlog.Correction{ActionID: second, Kind: actionlog.CorrectionVoid})

	got, ok, err := dal.LatestID(ctx, charID, actionlog.KindSave)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, second, got)
	assert.NotEqual(t, first, got)

	_, ok, err = dal.LatestID(ctx, charID+1, actionlog.KindSave)
	require.NoError(t, err)
	assert.False(t, ok)
}

// Checklist 3: a value amend stored before migration 014
// (raw row, no note column written) still gives the amended delta and HP
// total through the DAL, and a note added afterwards changes neither.
func TestLegacyValueAmend_ReadsAfterMigration014(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := actionlogdal.New(db)
	ctx := context.Background()

	orc := insert(t, dal, hp(charID, -8, "orc"))
	insert(t, dal, hp(charID, 3, "potion"))
	trap := insert(t, dal, hp(charID, -6, "trap"))
	_, err := db.ExecContext(ctx, `INSERT INTO action_corrections (action_id, kind, hp_delta, summary) VALUES (?, 'amend', -12, 'orc axe')`, orc)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO action_corrections (action_id, kind) VALUES (?, 'void')`, trap)
	require.NoError(t, err)

	check := func(wantNote string) {
		t.Helper()
		e, err := dal.GetEntry(ctx, charID, orc)
		require.NoError(t, err)
		assert.Equal(t, actionlog.StatusAmended, e.Status())
		assert.Equal(t, -8, e.HPDelta)
		assert.Equal(t, -12, e.EffectiveHPDelta())
		assert.Equal(t, "orc axe", e.EffectiveSummary())
		assert.Equal(t, wantNote, e.Note())
		total, err := dal.HPTotal(ctx, charID)
		require.NoError(t, err)
		assert.Equal(t, -9, total, "-12 + 3, trap voided")
	}
	check("")
	note := "DM: the axe crit"
	correct(t, dal, actionlog.Correction{ActionID: orc, Kind: actionlog.CorrectionAmend, Note: &note})
	check(note)
}

// ADR 0010 step 6: an amend linked to a correction step reads StepID, and
// StepUndone once the step is voided (popped); the entry then reads the
// payload before it, while an amend without a step (written before steps
// existed) stays in effect. The HP total leaves out a popped step's delta.
func TestCorrectionStep_PoppedStepPutsItsAmendOutOfEffect(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := actionlogdal.New(db)
	ctx := context.Background()

	save := insert(t, dal, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindSave, Payload: json.RawMessage(`{"a":0}`)})
	correct(t, dal, actionlog.Correction{ActionID: save, Kind: actionlog.CorrectionAmend, Payload: json.RawMessage(`{"a":1}`)})
	step := insert(t, dal, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindCorrection, Payload: json.RawMessage(`{"entryId":1}`)})
	correct(t, dal, actionlog.Correction{ActionID: save, Kind: actionlog.CorrectionAmend, Payload: json.RawMessage(`{"a":2}`), StepID: &step})

	e, err := dal.GetEntry(ctx, charID, save)
	require.NoError(t, err)
	require.Len(t, e.Corrections, 2)
	assert.Nil(t, e.Corrections[0].StepID, "legacy amend")
	require.NotNil(t, e.Corrections[1].StepID)
	assert.Equal(t, step, *e.Corrections[1].StepID)
	assert.False(t, e.Corrections[1].StepUndone)
	assert.JSONEq(t, `{"a":2}`, string(e.EffectivePayload()))

	correct(t, dal, actionlog.Correction{ActionID: step, Kind: actionlog.CorrectionVoid})
	e, err = dal.GetEntry(ctx, charID, save)
	require.NoError(t, err)
	assert.True(t, e.Corrections[1].StepUndone)
	assert.JSONEq(t, `{"a":1}`, string(e.EffectivePayload()), "the legacy amend is in effect again")
	listed, err := dal.ListEntries(ctx, charID, 10)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	assert.True(t, listed[1].Corrections[1].StepUndone)

	// HP: a popped step's delta no longer counts (corrections carry none
	// today; the SQL mirrors EffectiveHPDelta).
	hit := insert(t, dal, hp(charID, -5, "orc"))
	hpStep := insert(t, dal, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindCorrection, Payload: json.RawMessage(`{}`)})
	correct(t, dal, actionlog.Correction{ActionID: hit, Kind: actionlog.CorrectionAmend, HPDelta: intp(-9), StepID: &hpStep})
	total, err := dal.HPTotal(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, -9, total)
	correct(t, dal, actionlog.Correction{ActionID: hpStep, Kind: actionlog.CorrectionVoid})
	total, err = dal.HPTotal(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, -5, total)
}
