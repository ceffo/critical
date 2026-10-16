package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
)

// conn records which connection ran each statement.
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

func runAll(ctx context.Context, d DBTX) {
	_, _ = d.ExecContext(ctx, "")
	_, _ = d.PrepareContext(ctx, "")
	_, _ = d.QueryContext(ctx, "")
	_ = d.QueryRowContext(ctx, "")
}

// Each statement runs on the transaction its context carries, else on the pool,
// so sqlc query code built once over Routed joins the caller's transaction.
// (ADR 0025 step 5.2).
func TestRouted_RunsOnTheContextTransactionElseThePool(t *testing.T) {
	var ran []string
	d := Routed(conn{name: "pool", ran: &ran})

	runAll(context.Background(), d)
	runAll(withTx(context.Background(), conn{name: "tx", ran: &ran}), d)

	assert.Equal(t, []string{
		"pool exec", "pool prepare", "pool query", "pool queryrow",
		"tx exec", "tx prepare", "tx query", "tx queryrow",
	}, ran)
}
