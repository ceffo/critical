package critdal_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/critdal"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func open(t *testing.T) (*critdal.DAL, *sqlite.DB) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background()))
	return critdal.New(db), db
}

func TestList_EmptyUntilSaved(t *testing.T) {
	dal, _ := open(t)
	got, err := dal.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestReplace_SwapsTheWholeTable(t *testing.T) {
	dal, _ := open(t)
	ctx := context.Background()
	first := []combatsvc.CritRow{{From: 1, To: 50, Multiplier: 2}, {From: 51, To: 100, Multiplier: 3}}
	require.NoError(t, dal.Replace(ctx, first))
	got, err := dal.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, first, got)

	second := []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 5}}
	require.NoError(t, dal.Replace(ctx, second))
	got, err = dal.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, second, got)
}

// A failing write inside the transaction leaves the previous table.
func TestReplace_RolledBackWithItsTransaction(t *testing.T) {
	dal, db := open(t)
	ctx := context.Background()
	old := []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 2}}
	require.NoError(t, dal.Replace(ctx, old))

	err := db.InTx(ctx, func(ctx context.Context) error {
		// The second row breaks the table CHECK, after the old rows are gone.
		if err := dal.Replace(ctx, []combatsvc.CritRow{{From: 1, To: 50, Multiplier: 3}, {From: 51, To: 100, Multiplier: 0}}); err != nil {
			return err
		}
		return nil
	})
	require.Error(t, err)
	assert.False(t, errors.Is(err, nil))
	got, err := dal.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, old, got)
}
