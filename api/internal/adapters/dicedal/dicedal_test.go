package dicedal_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/dicedal"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func open(t *testing.T) *dicedal.DAL {
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
	return dicedal.New(db)
}

func TestFavourites_RoundTripPerCharacter(t *testing.T) {
	dal := open(t)
	ctx := context.Background()

	id1, err := dal.Insert(ctx, 1, "2d6+1")
	require.NoError(t, err)
	id2, err := dal.Insert(ctx, 1, "d%")
	require.NoError(t, err)
	_, err = dal.Insert(ctx, 2, "d4")
	require.NoError(t, err)

	got, err := dal.List(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []dicesvc.Favourite{{ID: id1, Formula: "2d6+1"}, {ID: id2, Formula: "d%"}}, got)

	removed, err := dal.Delete(ctx, 2, id1)
	require.NoError(t, err)
	assert.False(t, removed, "another character's favourite is out of reach")
	removed, err = dal.Delete(ctx, 1, id1)
	require.NoError(t, err)
	assert.True(t, removed)
	removed, err = dal.Delete(ctx, 1, id1)
	require.NoError(t, err)
	assert.False(t, removed)

	got, err = dal.List(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []dicesvc.Favourite{{ID: id2, Formula: "d%"}}, got)
}

func TestInsert_RefusesTheSameFormulaTwice(t *testing.T) {
	dal := open(t)
	ctx := context.Background()
	_, err := dal.Insert(ctx, 1, "d20")
	require.NoError(t, err)
	_, err = dal.Insert(ctx, 1, "d20")
	require.Error(t, err)
	_, err = dal.Insert(ctx, 2, "d20")
	require.NoError(t, err, "another character may save it")
}
