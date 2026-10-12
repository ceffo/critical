package characterdal_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open("file::memory:?cache=shared&mode=memory")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background()))
	return db
}

func TestGetCharacter(t *testing.T) {
	db := openDB(t)
	res, err := db.ExecContext(context.Background(),
		`INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total)
		 VALUES ('Bard', 18, 14, 12, 10, 9, 16, 42, 16, 5, 1200)`)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)

	got, err := characterdal.New(db).GetCharacter(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "Bard", got.Name)
	assert.Equal(t, 18, got.Str)
	assert.Equal(t, 14, got.Dex)
	assert.Equal(t, 12, got.Con)
	assert.Equal(t, 10, got.Int)
	assert.Equal(t, 9, got.Wis)
	assert.Equal(t, 16, got.Cha)
	assert.Equal(t, 42, got.HPMax)
	assert.Equal(t, 16, got.Thac0Base)
	assert.Equal(t, 5, got.Level)
	assert.Equal(t, 1200, got.XPTotal)
}

func TestGetCharacter_Missing(t *testing.T) {
	db := openDB(t)

	got, err := characterdal.New(db).GetCharacter(context.Background(), 9999)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, domain.ErrCharacterNotFound)
}

func TestFirstID(t *testing.T) {
	db := openDB(t)
	dal := characterdal.New(db)

	_, err := dal.FirstID(context.Background())
	assert.ErrorIs(t, err, domain.ErrCharacterNotFound)

	for _, n := range []string{"A", "B"} {
		_, err := db.ExecContext(context.Background(),
			`INSERT INTO characters (name, str, dex, con, int, wis, cha, hp_max, thac0_base) VALUES (?, 10, 10, 10, 10, 10, 10, 30, 19)`, n)
		require.NoError(t, err)
	}
	id, err := dal.FirstID(context.Background())
	require.NoError(t, err)
	got, err := dal.GetCharacter(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "A", got.Name)
}

// The repo seed is the campaign sheet's Bard (critical-qki.2).
func TestGetCharacter_RepoSeedIsTheCampaignBard(t *testing.T) {
	db := openDB(t)
	require.NoError(t, db.Seed(context.Background(), filepath.Join("..", "..", "..", "..", "seeds")))
	dal := characterdal.New(db)
	id, err := dal.FirstID(context.Background())
	require.NoError(t, err)

	got, err := dal.GetCharacter(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "Bard", got.Name)
	assert.Equal(t, []int{12, 17, 16, 14, 8, 16}, []int{got.Str, got.Dex, got.Con, got.Int, got.Wis, got.Cha})
	assert.Equal(t, 15, got.Level)
	assert.Equal(t, 1293418, got.XPTotal)
	assert.Equal(t, 85, got.HPMax)
	assert.Equal(t, 13, got.Thac0Base)
	assert.Equal(t, []int{10, 8, 9, 13, 9},
		[]int{got.SaveParaPoison, got.SaveRodStaffWand, got.SavePetriPoly, got.SaveBreath, got.SaveSpell})
}
