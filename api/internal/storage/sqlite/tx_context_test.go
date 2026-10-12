package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInTx(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name    string
		body    func(ctx context.Context, db *DB) error
		wantErr error
		rows    int
	}{
		{"commit", func(ctx context.Context, db *DB) error {
			_, err := Conn(ctx, db).ExecContext(ctx, `INSERT INTO t (v) VALUES (1)`)
			return err
		}, nil, 1},
		{"rollback on error", func(ctx context.Context, db *DB) error {
			if _, err := Conn(ctx, db).ExecContext(ctx, `INSERT INTO t (v) VALUES (1)`); err != nil {
				return err
			}
			return boom
		}, boom, 0},
		{"nested joins outer, outer error rolls back both", func(ctx context.Context, db *DB) error {
			if err := db.InTx(ctx, func(ctx context.Context) error {
				_, err := Conn(ctx, db).ExecContext(ctx, `INSERT INTO t (v) VALUES (1)`)
				return err
			}); err != nil {
				return err
			}
			if _, err := Conn(ctx, db).ExecContext(ctx, `INSERT INTO t (v) VALUES (2)`); err != nil {
				return err
			}
			return boom
		}, boom, 0},
		{"nested commit", func(ctx context.Context, db *DB) error {
			return db.InTx(ctx, func(ctx context.Context) error {
				_, err := Conn(ctx, db).ExecContext(ctx, `INSERT INTO t (v) VALUES (1)`)
				return err
			})
		}, nil, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			db, err := Open(filepath.Join(t.TempDir(), "t.db"))
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			_, err = db.ExecContext(ctx, `CREATE TABLE t (v INTEGER)`)
			require.NoError(t, err)

			err = db.InTx(ctx, func(ctx context.Context) error { return c.body(ctx, db) })
			if c.wantErr != nil {
				require.ErrorIs(t, err, c.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, c.rows, count(t, db, `SELECT COUNT(*) FROM t`))
		})
	}
}

func TestConn_FallbackOutsideTx(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	assert.Same(t, db, Conn(context.Background(), db))
}
