// Package buffsvc owns active buffs, the round counter, and effect aggregation.
// Round advancement is here because it expires duration-based buffs — the
// round counter and buff list are one bounded concern.
package buffsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	ListActive(ctx context.Context, characterID int64) ([]BuffEffect, error)
	Activate(ctx context.Context, characterID int64, buff BuffEffect) ([]BuffEffect, error)
	Dismiss(ctx context.Context, characterID int64, buffID string) ([]BuffEffect, error)
	Trigger(ctx context.Context, characterID int64, buffID string) ([]BuffEffect, error)
	GetRound(ctx context.Context, characterID int64) (int, error)
	AdvanceRound(ctx context.Context, characterID int64) (active []BuffEffect, expired []BuffEffect, round int, err error)

	// Aggregate is consumed by combatsvc (see clean_architecture.md §4.5).
	// Computed from the active buff set; pure given the actor's stats.
	Aggregate(ctx context.Context, characterID int64, str int) (BuffModifiers, error)
}

// DAL is the storage-driven port.
type DAL interface {
	GetActiveBuffs(ctx context.Context, characterID int64) ([]BuffEffect, error)
	SaveActiveBuffs(ctx context.Context, characterID int64, buffs []BuffEffect) error
	GetRoundNumber(ctx context.Context, characterID int64) (int, error)
	SaveRoundNumber(ctx context.Context, characterID int64, round int) error

	// WrapTx scopes a body to a transaction. The body receives a DAL bound to
	// the tx and uses it just like the pool-bound DAL.
	WrapTx(ctx context.Context, body func(DAL) error) error
}

// Compile-time guard: Querier returned by storage WrapTx is unused here, but
// keep the import live for documentation about where the tx Querier comes from.
var _ = sqlite.Querier(nil)

type service struct {
	dal DAL
}

// New constructs the service.
func New(dal DAL) (SVC, error) {
	if dal == nil {
		return nil, fmt.Errorf("buffsvc: nil dal")
	}
	return &service{dal: dal}, nil
}

// NewWithConfig is here for shape consistency.
func NewWithConfig(_ *Config, dal DAL) (SVC, error) { return New(dal) }

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
