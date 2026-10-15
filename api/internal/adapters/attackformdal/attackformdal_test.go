package attackformdal_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/attackformdal"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func open(t *testing.T) *attackformdal.DAL {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background()))
	return attackformdal.New(db)
}

func TestGet_NoneUntilSaved(t *testing.T) {
	dal := open(t)
	_, ok, err := dal.Get(context.Background(), 1)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPut_RoundTripsAndOverwrites(t *testing.T) {
	dal := open(t)
	ctx := context.Background()
	first := combatsvc.AttackForm{
		WeaponID: "short_bow", PreviousWeaponID: "long_sword", AmmoItemID: "arrows",
		TargetAC: new(-3), TargetSize: combatsvc.SizeLarge,
		Situational: combatsvc.Situational{Value: 2, Label: "rear"},
	}
	require.NoError(t, dal.Put(ctx, 1, first))
	got, ok, err := dal.Get(ctx, 1)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, first, got)

	// An unknown target AC (nil) round-trips as unknown, not as AC 0.
	second := combatsvc.AttackForm{WeaponID: "long_sword", TargetSize: combatsvc.SizeSmallMedium}
	require.NoError(t, dal.Put(ctx, 1, second))
	got, _, err = dal.Get(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, second, got)

	_, ok, err = dal.Get(ctx, 2)
	require.NoError(t, err)
	assert.False(t, ok, "forms are per character")
}
