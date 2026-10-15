package weapondal_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/weapondal"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

func seeded(t *testing.T) *weapondal.DAL {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	require.NoError(t, db.Migrate(ctx))
	require.NoError(t, db.Seed(ctx, filepath.Join("..", "..", "..", "testdata", "seeds")))
	return weapondal.New(db)
}

func TestGet_SheetWeapon(t *testing.T) {
	w, err := seeded(t).Get(context.Background(), "shortbow")
	require.NoError(t, err)
	assert.Equal(t, combatsvc.Weapon{
		ID: "shortbow", Name: "Short Bow +2", BaseDamage: "1d6", DamageLarge: "1d6",
		MagicToHit: 2, MagicDamage: 2, Kind: combatsvc.WeaponMissile, TwoHanded: true, AmmoKind: "arrow",
	}, w)
}

func TestGet_OriginalWeaponsKeepTheirDamageDice(t *testing.T) {
	dal := seeded(t)
	ls, err := dal.Get(context.Background(), "longsword")
	require.NoError(t, err)
	assert.Equal(t, "1d8", ls.BaseDamage)
	assert.Equal(t, "1d12", ls.DamageLarge)
}

func TestGet_Unknown(t *testing.T) {
	_, err := seeded(t).Get(context.Background(), "nope")
	require.ErrorIs(t, err, combatsvc.ErrWeaponNotFound)
}

func TestList_SheetOrder(t *testing.T) {
	got, err := seeded(t).List(context.Background())
	require.NoError(t, err)
	ids := make([]string, 0, len(got))
	for _, w := range got {
		ids = append(ids, w.ID)
	}
	assert.Equal(t, []string{"bastard_sword", "longsword", "shortbow", "dagger", "mace", "darts"}, ids)
}

func seededEditor(t *testing.T) *weapondal.Editor {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	require.NoError(t, db.Migrate(ctx))
	require.NoError(t, db.Seed(ctx, filepath.Join("..", "..", "..", "testdata", "seeds")))
	return weapondal.NewEditor(db)
}

func TestEditor_ListGetInsertUpdateDelete(t *testing.T) {
	ctx := context.Background()
	e := seededEditor(t)
	all, err := e.List(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, all)
	assert.Equal(t, 0, all[0].Position)

	w := weaponsvc.Weapon{
		ID: "long_bow", Position: 99, Name: "Long Bow", DamageSM: "1d8", DamageL: "1d8",
		MagicToHit: 1, MagicDamage: 2, Kind: "missile", TwoHanded: true, AmmoKind: "arrow",
	}
	require.NoError(t, e.Insert(ctx, w))
	got, err := e.Get(ctx, "long_bow")
	require.NoError(t, err)
	assert.Equal(t, w, got)

	w.MagicToHit, w.TwoHanded, w.AmmoKind = 3, false, "bolt"
	require.NoError(t, e.Update(ctx, w))
	got, err = e.Get(ctx, "long_bow")
	require.NoError(t, err)
	assert.Equal(t, w, got)

	require.NoError(t, e.Delete(ctx, "long_bow"))
	_, err = e.Get(ctx, "long_bow")
	require.ErrorIs(t, err, weaponsvc.ErrWeaponNotFound)
	require.ErrorIs(t, e.Delete(ctx, "long_bow"), weaponsvc.ErrWeaponNotFound)
	require.ErrorIs(t, e.Update(ctx, w), weaponsvc.ErrWeaponNotFound)
}
