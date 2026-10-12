package itemdal_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/itemdal"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openDB(t *testing.T) *sqlite.DB {
	t.Helper()
	return openDSN(t, filepath.Join(t.TempDir(), "t.db"))
}

func openDSN(t *testing.T, dsn string) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background()))
	return db
}

func insert(t *testing.T, db *sqlite.DB, id string, pos, count, maxCount int, perDay bool) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO items (id, position, name, kind, count, max_count, per_day, effect_dice)
		VALUES (?, ?, ?, 'charged', ?, ?, ?, '6d6')`, id, pos, "Name "+id, count, maxCount, perDay)
	require.NoError(t, err)
}

func TestListAndMapping(t *testing.T) {
	db := openDB(t)
	insert(t, db, "b", 1, 4, 4, false)
	insert(t, db, "a", 0, 1, 1, true)

	got, err := itemdal.New(db).List(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].ID, "ordered by position, not id")
	assert.Equal(t, itemsvc.Item{
		ID: "a", Name: "Name a", Kind: itemsvc.KindCharged, Count: 1, MaxCount: 1, PerDay: true, EffectDice: "6d6",
	}, got[0])
}

func TestDecrement_StopsAtZero(t *testing.T) {
	db := openDB(t)
	insert(t, db, "wand", 0, 2, 4, false)
	dal := itemdal.New(db)
	ctx := context.Background()

	got, err := dal.Decrement(ctx, "wand")
	require.NoError(t, err)
	assert.Equal(t, 1, got.Count)
	got, err = dal.Decrement(ctx, "wand")
	require.NoError(t, err)
	assert.Equal(t, 0, got.Count)

	_, err = dal.Decrement(ctx, "wand")
	require.ErrorIs(t, err, itemsvc.ErrItemEmpty)
	var n int
	require.NoError(t, db.QueryRow(`SELECT count FROM items WHERE id = 'wand'`).Scan(&n))
	assert.Equal(t, 0, n, "never below 0")
}

func TestDecrement_UnknownItem(t *testing.T) {
	db := openDB(t)
	_, err := itemdal.New(db).Decrement(context.Background(), "nope")
	require.ErrorIs(t, err, itemsvc.ErrItemNotFound)
}

func TestDecrement_ConcurrentUsesNeverOverspend(t *testing.T) {
	// busy_timeout makes writers queue instead of failing with SQLITE_BUSY.
	db := openDSN(t, "file:"+filepath.Join(t.TempDir(), "t.db")+"?_pragma=busy_timeout(5000)")
	insert(t, db, "arrows", 0, 5, 5, false)
	dal := itemdal.New(db)

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, empty := 0, 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := dal.Decrement(context.Background(), "arrows")
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case assert.ErrorIs(t, err, itemsvc.ErrItemEmpty):
				empty++
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 5, ok)
	assert.Equal(t, 7, empty)
}

func TestDecrement_JoinsCtxTransaction(t *testing.T) {
	db := openDB(t)
	insert(t, db, "wand", 0, 2, 2, false)
	dal := itemdal.New(db)

	err := db.InTx(context.Background(), func(ctx context.Context) error {
		_, err := dal.Decrement(ctx, "wand")
		require.NoError(t, err)
		return assert.AnError
	})
	require.ErrorIs(t, err, assert.AnError)
	var n int
	require.NoError(t, db.QueryRow(`SELECT count FROM items WHERE id = 'wand'`).Scan(&n))
	assert.Equal(t, 2, n, "rolled back with the surrounding transaction")
}

func TestResetPerDay_OnlyRefillsPerDayItems(t *testing.T) {
	db := openDB(t)
	insert(t, db, "gem", 0, 0, 1, true)
	insert(t, db, "wand", 1, 1, 4, false)
	dal := itemdal.New(db)

	require.NoError(t, dal.ResetPerDay(context.Background()))
	got, err := dal.List(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, got[0].Count, "per-day gem refilled")
	assert.Equal(t, 1, got[1].Count, "wand untouched")
}
