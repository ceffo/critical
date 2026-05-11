package sqlite

import (
	"context"
	"database/sql"

	"github.com/stephenafamo/scan"
)

// BobExec wraps a Querier to satisfy bob.Executor.
// bob.Executor requires QueryContext to return scan.Rows rather than *sql.Rows.
// Both *sql.Rows and scan.Rows expose the same method set; the adapter bridges
// the return type so our existing Querier works with bob's generated code.
type BobExec struct{ Querier }

// QueryContext delegates to the wrapped Querier and returns *sql.Rows as scan.Rows.
func (e BobExec) QueryContext(ctx context.Context, query string, args ...any) (scan.Rows, error) {
	return e.Querier.QueryContext(ctx, query, args...)
}

// ExecContext delegates to the wrapped Querier (promoted from Querier, redeclared
// here so the concrete BobExec struct has an unambiguous ExecContext signature).
func (e BobExec) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return e.Querier.ExecContext(ctx, query, args...)
}
