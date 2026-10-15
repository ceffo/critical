package xpdal_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/xpdal"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func open(t *testing.T) (*xpdal.DAL, *sqlite.DB) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	require.NoError(t, db.Migrate(ctx))
	for _, name := range []string{"A", "B"} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (?, 10, 10, 10, 10, 10, 10, 10, 20)`,
			name)
		require.NoError(t, err)
	}
	return xpdal.New(db), db
}

func TestCRUD_RoundTripsNewestFirstPerCharacter(t *testing.T) {
	dal, _ := open(t)
	ctx := context.Background()

	first := xpsvc.Entry{Date: "2026-10-10", Session: "S41", Reason: "ogre", Base: 1200, Bonus: true}
	id1, err := dal.Insert(ctx, 1, first)
	require.NoError(t, err)
	id2, err := dal.Insert(ctx, 1, xpsvc.Entry{Date: "2026-10-17", Reason: "quest", Base: 500})
	require.NoError(t, err)
	_, err = dal.Insert(ctx, 2, xpsvc.Entry{Date: "2026-10-17", Base: 7})
	require.NoError(t, err)

	got, err := dal.List(ctx, 1)
	require.NoError(t, err)
	first.ID = id1
	assert.Equal(t, []xpsvc.Entry{
		{ID: id2, Date: "2026-10-17", Reason: "quest", Base: 500},
		first,
	}, got)

	edited := first
	edited.Base, edited.Bonus, edited.Reason, edited.Session = 1300, false, "ogre chief", "S42"
	require.NoError(t, dal.Update(ctx, 1, edited))
	one, err := dal.Get(ctx, 1, id1)
	require.NoError(t, err)
	assert.Equal(t, edited, one)

	require.NoError(t, dal.Delete(ctx, 1, id2))
	got, err = dal.List(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []xpsvc.Entry{edited}, got)
}

func TestNotFound_UnknownIDOrOtherCharacter(t *testing.T) {
	dal, _ := open(t)
	ctx := context.Background()
	id, err := dal.Insert(ctx, 1, xpsvc.Entry{Date: "2026-10-17", Base: 10})
	require.NoError(t, err)

	_, err = dal.Get(ctx, 2, id)
	require.ErrorIs(t, err, xpsvc.ErrEntryNotFound)
	require.ErrorIs(t, dal.Update(ctx, 2, xpsvc.Entry{ID: id, Base: 10}), xpsvc.ErrEntryNotFound)
	require.ErrorIs(t, dal.Delete(ctx, 1, id+100), xpsvc.ErrEntryNotFound)
	require.ErrorIs(t, dal.Delete(ctx, 2, id), xpsvc.ErrEntryNotFound)
}

func TestSchema_RejectsNonPositiveBase(t *testing.T) {
	dal, _ := open(t)
	_, err := dal.Insert(context.Background(), 1, xpsvc.Entry{Date: "2026-10-17", Base: 0})
	require.Error(t, err)
}
