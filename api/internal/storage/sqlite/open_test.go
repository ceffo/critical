package sqlite

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestOpenPragmasOnEveryConnection(t *testing.T) {
	for name, dsn := range map[string]string{
		"bare": filepath.Join(t.TempDir(), "a.db"),
		"uri":  "file:" + filepath.Join(t.TempDir(), "b.db") + "?cache=shared",
		"mem":  "file::memory:?cache=shared&mode=memory",
		"mem2": ":memory:",
	} {
		t.Run(name, func(t *testing.T) {
			db, err := Open(dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()
			ctx := context.Background()
			var conns []interface{ Close() error }
			for i := 0; i < 3; i++ {
				c, err := db.Conn(ctx)
				if err != nil {
					t.Fatal(err)
				}
				conns = append(conns, c)
				var bt int
				if err := c.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&bt); err != nil || bt != 5000 {
					t.Fatalf("conn %d busy_timeout=%d err=%v", i, bt, err)
				}
				if name == "bare" || name == "uri" {
					var jm string
					if err := c.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&jm); err != nil || jm != "wal" {
						t.Fatalf("journal_mode=%q err=%v", jm, err)
					}
				}
			}
			for _, c := range conns {
				_ = c.Close()
			}
		})
	}
}

func TestOpenCallerPragmaWins(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "c.db") + "?_pragma=busy_timeout(1234)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var bt int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&bt); err != nil || bt != 1234 {
		t.Fatalf("busy_timeout=%d err=%v", bt, err)
	}
}

func TestConcurrentWriteTxsBothCommit(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE t (n INTEGER)"); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- db.WrapTx(ctx, func(q Querier) error {
			if _, err := q.ExecContext(ctx, "INSERT INTO t VALUES (1)"); err != nil {
				return err
			}
			close(started)
			time.Sleep(300 * time.Millisecond)
			return nil
		})
	}()
	go func() {
		defer wg.Done()
		<-started
		errs <- db.WrapTx(ctx, func(q Querier) error {
			_, err := q.ExecContext(ctx, "INSERT INTO t VALUES (2)")
			return err
		})
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("tx failed: %v", err)
		}
	}
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM t").Scan(&n); err != nil || n != 2 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
}
