package hpdal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/hpdal"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

func openDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open("file::memory:?cache=shared&mode=memory")
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

func TestGetMaxHP(t *testing.T) {
	db := openDB(t)
	charID := seedCharacter(t, db)
	dal := hpdal.New(db)

	maxHP, err := dal.GetMaxHP(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, 30, maxHP)
}

func TestGetMaxHP_NotFound(t *testing.T) {
	db := openDB(t)
	dal := hpdal.New(db)

	_, err := dal.GetMaxHP(context.Background(), 999)
	assert.ErrorIs(t, err, domain.ErrCharacterNotFound)
}
