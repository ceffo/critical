package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// conn records which connection ran each statement, as "<name> <operation>", in one shared log.
type conn struct {
	name string
	ran  *[]string
}

func (c conn) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	*c.ran = append(*c.ran, c.name+" exec")
	return nil, nil
}

func (c conn) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	*c.ran = append(*c.ran, c.name+" prepare")
	return nil, nil
}

func (c conn) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	*c.ran = append(*c.ran, c.name+" query")
	return nil, nil
}

func (c conn) QueryRowContext(context.Context, string, ...any) *sql.Row {
	*c.ran = append(*c.ran, c.name+" queryrow")
	return nil
}

// tx is a transaction that records its commit and rollback in the same log as its statements.
type tx struct {
	conn
	commitErr error
}

func (t tx) Commit() error {
	*t.ran = append(*t.ran, "commit")
	return t.commitErr
}

func (t tx) Rollback() error {
	*t.ran = append(*t.ran, "rollback")
	return nil
}

// recordingDB is a DB whose transactions are tx and whose writer lock is logged as "lock", so a
// test reads the order of everything a transaction did.
func recordingDB(ran *[]string, opts **sql.TxOptions) *DB {
	return &DB{
		begin: func(_ context.Context, o *sql.TxOptions) (Tx, error) {
			*ran = append(*ran, "begin")
			if opts != nil {
				*opts = o
			}
			return tx{conn: conn{name: "tx", ran: ran}}, nil
		},
		lock: func(ctx context.Context, on DBTX) error {
			_, ok := on.(tx)
			*ran = append(*ran, "lock")
			if !ok {
				return errors.New("lock taken outside the transaction")
			}
			return nil
		},
	}
}

// ADR 0025 step 4: a write transaction takes the writer lock as its first statement, before
// anything fn runs, on the transaction itself; the commit releases it, nothing else does (no
// statement runs after fn's but the commit).
func TestInTx_Step4_TakesTheWriterLockFirstAndReleasesItOnlyByCommit(t *testing.T) {
	var ran []string
	d := recordingDB(&ran, nil)

	require.NoError(t, d.InTx(context.Background(), func(ctx context.Context) error {
		_, err := Routed(conn{name: "pool", ran: &ran}).ExecContext(ctx, "")
		return err
	}))

	assert.Equal(t, []string{"begin", "lock", "tx exec", "commit"}, ran)
}

// ADR 0025 step 4.3: a failing fn rolls back, which releases the lock; nothing is committed and
// no statement releases the lock by hand.
func TestInTx_Step4_3_AFailureRollsBackAndReleasesNothingByHand(t *testing.T) {
	var ran []string
	d := recordingDB(&ran, nil)
	boom := errors.New("boom")

	err := d.InTx(context.Background(), func(ctx context.Context) error {
		_, _ = Routed(conn{name: "pool", ran: &ran}).ExecContext(ctx, "")
		return boom
	})

	require.ErrorIs(t, err, boom)
	assert.Equal(t, []string{"begin", "lock", "tx exec", "rollback"}, ran)
}

// A panic in fn rolls back and is re-raised.
func TestInTx_APanicRollsBack(t *testing.T) {
	var ran []string
	d := recordingDB(&ran, nil)

	assert.PanicsWithValue(t, "boom", func() {
		_ = d.InTx(context.Background(), func(context.Context) error { panic("boom") })
	})
	assert.Equal(t, []string{"begin", "lock", "rollback"}, ran)
}

// ADR 0025 step 4.2: write transactions run at READ COMMITTED, so the second writer, once the
// lock is granted, reads the first one's commit.
func TestInTx_Step4_2_ReadCommitted(t *testing.T) {
	var ran []string
	var opts *sql.TxOptions
	d := recordingDB(&ran, &opts)

	require.NoError(t, d.InTx(context.Background(), func(context.Context) error { return nil }))

	require.NotNil(t, opts)
	assert.Equal(t, sql.LevelReadCommitted, opts.Isolation)
	assert.False(t, opts.ReadOnly)
}

// A nested InTx joins the outer transaction: one begin, one lock, one commit.
func TestInTx_NestedJoinsTheOuterTransaction(t *testing.T) {
	var ran []string
	d := recordingDB(&ran, nil)

	require.NoError(t, d.InTx(context.Background(), func(ctx context.Context) error {
		return d.InTx(ctx, func(ctx context.Context) error {
			_, err := Routed(conn{name: "pool", ran: &ran}).ExecContext(ctx, "")
			return err
		})
	}))

	assert.Equal(t, []string{"begin", "lock", "tx exec", "commit"}, ran)
}

// A lock that cannot be taken fails the transaction before fn runs.
func TestInTx_NoLockNoWork(t *testing.T) {
	var ran []string
	d := recordingDB(&ran, nil)
	d.lock = func(context.Context, DBTX) error { return errors.New("connection lost") }
	called := false

	err := d.InTx(context.Background(), func(context.Context) error { called = true; return nil })

	require.ErrorContains(t, err, "take the writer lock")
	assert.False(t, called)
	assert.Equal(t, []string{"begin", "rollback"}, ran)
}

func runAll(ctx context.Context, d DBTX) {
	_, _ = d.ExecContext(ctx, "")
	_, _ = d.PrepareContext(ctx, "")
	_, _ = d.QueryContext(ctx, "")
	_ = d.QueryRowContext(ctx, "")
}

// Each statement runs on the transaction its context carries, else on the pool, so sqlc query
// code built once over Routed joins the caller's transaction (ADR 0025 step 5.2).
func TestRouted_RunsOnTheContextTransactionElseThePool(t *testing.T) {
	var ran []string
	d := Routed(conn{name: "pool", ran: &ran})

	runAll(context.Background(), d)
	runAll(WithTx(context.Background(), conn{name: "tx", ran: &ran}), d)

	assert.Equal(t, []string{
		"pool exec", "pool prepare", "pool query", "pool queryrow",
		"tx exec", "tx prepare", "tx query", "tx queryrow",
	}, ran)
}

// A context carrying a nil transaction carries none: statements go to the pool.
func TestRouted_NilTransactionMeansThePool(t *testing.T) {
	var ran []string
	d := Routed(conn{name: "pool", ran: &ran})

	runAll(WithTx(context.Background(), (*sql.Tx)(nil)), d)

	assert.Equal(t, []string{"pool exec", "pool prepare", "pool query", "pool queryrow"}, ran)
	_, ok := txFrom(WithTx(context.Background(), (*sql.Tx)(nil)))
	assert.False(t, ok)
}

// noConnector is a driver.Connector that never connects: sql.OpenDB over it builds a pool without
// touching a database, and nothing in the test asks it for a connection.
type noConnector struct{}

func (noConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("no database in tests")
}

func (noConnector) Driver() driver.Driver { return nil }

// Every waiting writer holds a connection: the pool is capped so a burst of writes cannot use up
// the cluster's connections.
func TestNewDB_CapsThePool(t *testing.T) {
	pool := sql.OpenDB(noConnector{})
	t.Cleanup(func() { _ = pool.Close() })
	newDB(pool)
	assert.Equal(t, maxOpenConns, pool.Stats().MaxOpenConnections)
}

// poolRecorder records what configurePool sets.
type poolRecorder struct {
	maxOpen int
	maxIdle time.Duration
}

func (p *poolRecorder) SetMaxOpenConns(n int)              { p.maxOpen = n }
func (p *poolRecorder) SetConnMaxIdleTime(d time.Duration) { p.maxIdle = d }

// Idle connections are closed well before the fly cluster's one-hour idle stop: held forever they
// would keep the shared cluster running, and one held across a stop would be dead when reused.
func TestConfigurePool_ClosesIdleConnectionsBeforeTheClusterStops(t *testing.T) {
	var p poolRecorder
	configurePool(&p)
	assert.Equal(t, maxOpenConns, p.maxOpen)
	assert.Positive(t, p.maxIdle)
	assert.Less(t, p.maxIdle, time.Hour)
}

// pings answers each call with the next error of errs (nil once they run out) and counts the calls.
type pings struct {
	errs  []error
	calls int
}

func (p *pings) ping(context.Context) error {
	p.calls++
	if len(p.errs) == 0 {
		return nil
	}
	err := p.errs[0]
	p.errs = p.errs[1:]
	return err
}

// A database that is still starting (connection refused, then "the database system is starting
// up") is waited for: the server does not exit on its first failed connection.
func TestWaitForDatabase_RetriesWhileTheDatabaseStarts(t *testing.T) {
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	starting := &pq.Error{Code: cannotConnectNow, Message: "the database system is starting up"}
	p := &pings{errs: []error{refused, starting}}
	require.NoError(t, waitForDatabase(context.Background(), p.ping, time.Millisecond))
	assert.Equal(t, 3, p.calls)
}

// An error waiting cannot fix (a wrong password) is returned at once.
func TestWaitForDatabase_FailsFastOnAnotherPostgresError(t *testing.T) {
	auth := &pq.Error{Code: "28P01", Message: "password authentication failed"}
	p := &pings{errs: []error{auth}}
	err := waitForDatabase(context.Background(), p.ping, time.Millisecond)
	require.ErrorIs(t, err, auth)
	assert.Equal(t, 1, p.calls)
}

// The wait is bounded: when the context ends, the last error is returned.
func TestWaitForDatabase_GivesUpWhenTheContextEnds(t *testing.T) {
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	ctx, cancel := context.WithCancel(context.Background())
	p := &pings{errs: []error{refused, refused, refused, refused}}
	ping := func(c context.Context) error {
		if p.calls == 2 {
			cancel()
		}
		return p.ping(c)
	}
	err := waitForDatabase(ctx, ping, time.Millisecond)
	require.ErrorIs(t, err, refused)
	assert.Equal(t, 3, p.calls)
}
