package actionlogsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ADR 0018 step 1: everything one state-changing request writes to the log,
// in its one DB transaction, is one transaction, identified by the id of its
// first entry. The grouping is generic: Transactions opens a scope with the
// outermost InTx, and every entry recorded inside it joins that scope's
// transaction, whatever service or kind records it.

type scopeKey struct{}

// scope is the action-log transaction of one outermost InTx: 0 until its
// first entry is recorded, then that entry's id.
type scope struct{ transactionID int64 }

// Transactions wraps runner so that each outermost InTx is one action-log
// transaction: the entries recorded inside it (through Record, by any
// service) share one transaction id and are popped together by Undo. A nested
// InTx joins the outer scope, as it joins the outer DB transaction. The
// composition root hands the wrapped runner to every service that records.
func Transactions(runner domain.TxRunner) domain.TxRunner {
	if t, ok := runner.(transactions); ok {
		return t
	}
	return transactions{inner: runner}
}

type transactions struct{ inner domain.TxRunner }

func (t transactions) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(scopeKey{}).(*scope); ok {
		return t.inner.InTx(ctx, fn)
	}
	return t.inner.InTx(ctx, func(ctx context.Context) error {
		return fn(context.WithValue(ctx, scopeKey{}, &scope{}))
	})
}

// insert stores e in the transaction of the scope ctx carries, starting it
// when e is its first entry. It must run inside s.tx.InTx (which opens the
// scope).
func (s *service) insert(ctx context.Context, e actionlog.NewEntry) (int64, error) {
	sc, ok := ctx.Value(scopeKey{}).(*scope)
	if !ok {
		return 0, fmt.Errorf("record %s: no action-log transaction in scope", e.Kind)
	}
	id, err := s.dal.InsertEntry(ctx, e, sc.transactionID)
	if err != nil {
		return 0, err
	}
	if sc.transactionID == 0 {
		sc.transactionID = id
	}
	return id, nil
}
