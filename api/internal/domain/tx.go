package domain

import "context"

// TxRunner scopes a unit of work to one storage transaction. fn receives a
// context that carries the transaction; every adapter call made with that
// context joins it. Services use it when a state write and its action-log
// entry must commit together. A nested call joins the outer transaction.
type TxRunner interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}
