package actionlogdal_test

// End-to-end checks of actionlogsvc on base SQLite: the entry/correction and
// the state write commit together or not at all.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/actionlogdal"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errBoom = errors.New("boom")

type fixedRound int

func (r fixedRound) GetRound(context.Context, int64) (int, error) { return int(r), nil }

// failingDAL performs the base InsertCorrection and then fails, like a
// storage error after a partial write.
type failingDAL struct {
	*actionlogdal.DAL
	failCorrection bool
}

func (f failingDAL) InsertCorrection(ctx context.Context, c actionlog.Correction) error {
	if err := f.DAL.InsertCorrection(ctx, c); err != nil {
		return err
	}
	if f.failCorrection {
		return errBoom
	}
	return nil
}

// roundState is a stand-in for a service that owns base state (round_state)
// and records its changes with snapshots.
type roundState struct {
	db   *sqlite.DB
	fail bool
}

func (r roundState) set(ctx context.Context, charID int64, round int) error {
	_, err := sqlite.Conn(ctx, r.db).ExecContext(ctx,
		`INSERT INTO round_state (character_id, round_number) VALUES (?, ?)
		 ON CONFLICT(character_id) DO UPDATE SET round_number = excluded.round_number`, charID, round)
	return err
}

func (r roundState) Restore(ctx context.Context, charID int64, state json.RawMessage) error {
	var round int
	if err := json.Unmarshal(state, &round); err != nil {
		return err
	}
	if err := r.set(ctx, charID, round); err != nil {
		return err
	}
	if r.fail {
		return errBoom
	}
	return nil
}

func round(t *testing.T, db *sqlite.DB, charID int64) int {
	t.Helper()
	var n int
	err := db.QueryRowContext(context.Background(), `SELECT COALESCE((SELECT round_number FROM round_state WHERE character_id = ?), 0)`, charID).Scan(&n)
	require.NoError(t, err)
	return n
}

func newLog(t *testing.T, db *sqlite.DB, dal actionlogsvc.DAL) actionlogsvc.SVC {
	t.Helper()
	svc, err := actionlogsvc.NewWithConfig(actionlogsvc.DefaultConfig(), dal, db, fixedRound(4))
	require.NoError(t, err)
	return svc
}

// advance sets round_state from..to and records it, in one transaction.
func advance(ctx context.Context, db *sqlite.DB, log actionlogsvc.SVC, rs roundState, charID int64, from, to int) (int64, error) {
	var id int64
	err := db.InTx(ctx, func(ctx context.Context) error {
		if err := rs.set(ctx, charID, to); err != nil {
			return err
		}
		var err error
		id, err = log.Record(ctx, actionlog.NewEntry{
			CharacterID: charID, Round: to, Kind: actionlog.KindRoundAdvance, Summary: "next round",
			Snapshots: []actionlog.Snapshot{{Key: "round", Before: json.RawMessage(itoa(from)), After: json.RawMessage(itoa(to))}},
		})
		return err
	})
	return id, err
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestRecord_RolledBackWithCallerStateWrite(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := actionlogdal.New(db)
	log := newLog(t, db, dal)
	ctx := context.Background()

	err := db.InTx(ctx, func(ctx context.Context) error {
		if _, err := log.Record(ctx, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindHPChange, Summary: "orc", HPDelta: -8}); err != nil {
			return err
		}
		return errBoom // the caller's state write fails after the entry was recorded
	})
	require.ErrorIs(t, err, errBoom)

	entries, err := log.List(ctx, charID, 0)
	require.NoError(t, err)
	assert.Empty(t, entries)
	total, err := log.HPTotal(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
}

func TestRecord_StampsRound(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	log := newLog(t, db, actionlogdal.New(db))
	ctx := context.Background()

	id, err := log.Record(ctx, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindHPChange, Summary: "orc", HPDelta: -8})
	require.NoError(t, err)
	entries, err := log.List(ctx, charID, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, id, entries[0].ID)
	assert.Equal(t, 4, entries[0].Round)
	assert.JSONEq(t, `{}`, string(entries[0].Payload))
}

func TestVoid_RestoresBeforeState(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	log := newLog(t, db, actionlogdal.New(db))
	rs := roundState{db: db}
	log.RegisterRestorer("round", rs)
	ctx := context.Background()

	id, err := advance(ctx, db, log, rs, charID, 0, 1)
	require.NoError(t, err)
	require.Equal(t, 1, round(t, db, charID))

	e, err := log.Void(ctx, charID, id)
	require.NoError(t, err)
	assert.Equal(t, actionlog.StatusVoided, e.Status())
	assert.Equal(t, 0, round(t, db, charID))
}

// Pop-only: any older entry is refused, even when the newer
// one touches other state; popping newest first unwinds the state in order.
func TestVoid_OnlyTheLatestEntry(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	log := newLog(t, db, actionlogdal.New(db))
	rs := roundState{db: db}
	log.RegisterRestorer("round", rs)
	ctx := context.Background()

	first, err := advance(ctx, db, log, rs, charID, 0, 1)
	require.NoError(t, err)
	second, err := advance(ctx, db, log, rs, charID, 1, 2)
	require.NoError(t, err)
	scratch, err := log.Record(ctx, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindHPChange, Summary: "scratch", HPDelta: -2})
	require.NoError(t, err)

	for _, older := range []int64{first, second} {
		_, err = log.Void(ctx, charID, older)
		require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)
		assert.Contains(t, err.Error(), fmt.Sprintf("void #%d (hp_change, round 4) first", scratch))
	}
	assert.Equal(t, 2, round(t, db, charID), "nothing restored")

	for i, id := range []int64{scratch, second, first} {
		_, err = log.Void(ctx, charID, id)
		require.NoError(t, err, "pop %d", i)
	}
	assert.Equal(t, 0, round(t, db, charID))
	_, err = log.Void(ctx, charID, first)
	require.ErrorIs(t, err, actionlogsvc.ErrAlreadyVoided)
}

func TestVoid_AtomicWithRestore(t *testing.T) {
	cases := []struct {
		name           string
		failRestore    bool
		failCorrection bool
	}{
		{"restorer fails after writing", true, false},
		{"correction insert fails after restore", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := openDB(t)
			charID := seedCharacter(t, db)
			base := actionlogdal.New(db)
			rs := roundState{db: db}
			setup := newLog(t, db, base)
			id, err := advance(context.Background(), db, setup, rs, charID, 0, 1)
			require.NoError(t, err)

			log := newLog(t, db, failingDAL{DAL: base, failCorrection: c.failCorrection})
			log.RegisterRestorer("round", roundState{db: db, fail: c.failRestore})

			_, err = log.Void(context.Background(), charID, id)
			require.ErrorIs(t, err, errBoom)

			assert.Equal(t, 1, round(t, db, charID), "restored state rolled back")
			e, err := base.GetEntry(context.Background(), charID, id)
			require.NoError(t, err)
			assert.Equal(t, actionlog.StatusActive, e.Status(), "void rolled back")
			assert.Empty(t, e.Corrections)
		})
	}
}

// Note-only amend and pop-only void on real SQLite.
func TestAmendNoteAndPop_HPTotal(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	log := newLog(t, db, actionlogdal.New(db))
	ctx := context.Background()
	rec := func(d int, s string) int64 {
		id, err := log.Record(ctx, actionlog.NewEntry{CharacterID: charID, Kind: actionlog.KindHPChange, Summary: s, HPDelta: d})
		require.NoError(t, err)
		return id
	}
	total := func() int {
		n, err := log.HPTotal(ctx, charID)
		require.NoError(t, err)
		return n
	}

	orc := rec(-8, "orc")
	rec(3, "potion")
	assert.Equal(t, -5, total())

	e, err := log.Amend(ctx, charID, orc, actionlogsvc.Amendment{Note: " axe, not club "})
	require.NoError(t, err)
	assert.Equal(t, "axe, not club", e.Note())
	assert.Equal(t, -8, e.EffectiveHPDelta())
	assert.Equal(t, actionlog.StatusActive, e.Status())
	assert.Equal(t, -5, total(), "a note changes no HP")

	ok, err := log.VoidLatest(ctx, charID, actionlog.KindHPChange)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, -8, total(), "potion popped")

	_, err = log.Void(ctx, charID, orc)
	require.NoError(t, err)
	assert.Equal(t, 0, total())

	_, err = log.Void(ctx, charID, orc)
	require.ErrorIs(t, err, actionlogsvc.ErrAlreadyVoided)
	_, err = log.Amend(ctx, charID, orc, actionlogsvc.Amendment{Note: "late"})
	require.ErrorIs(t, err, actionlogsvc.ErrAlreadyVoided)

	ok, err = log.VoidLatest(ctx, charID, actionlog.KindHPChange)
	require.NoError(t, err)
	assert.False(t, ok, "nothing left to undo")
}
