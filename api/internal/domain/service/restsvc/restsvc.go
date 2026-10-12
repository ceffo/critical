// Package restsvc owns the long rest: one operation that refills everything
// that resets overnight (spell slots and per-day items) in a single
// transaction, so a failure in one leaves the other untouched.
package restsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Result is the state after a long rest.
type Result struct {
	Slots []spellsvc.SpellSlot
	Items []itemsvc.Item
}

// SVC is the driver port.
type SVC interface {
	domain.Service
	LongRest(ctx context.Context, characterID int64) (Result, error)
}

// Spells is the narrow port to the spell-slot service.
type Spells interface {
	ResetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error)
}

// Items is the narrow port to the item service.
type Items interface {
	ResetDaily(ctx context.Context) ([]itemsvc.Item, error)
}

type service struct {
	spells Spells
	items  Items
	tx     domain.TxRunner
}

// New constructs the service.
func New(spells Spells, items Items, tx domain.TxRunner) (SVC, error) {
	if spells == nil || items == nil || tx == nil {
		return nil, fmt.Errorf("restsvc: nil dependency")
	}
	return &service{spells: spells, items: items, tx: tx}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// LongRest resets spell slots and per-day items atomically.
func (s *service) LongRest(ctx context.Context, characterID int64) (Result, error) {
	var res Result
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		slots, err := s.spells.ResetSlots(ctx, characterID)
		if err != nil {
			return fmt.Errorf("reset slots: %w", err)
		}
		items, err := s.items.ResetDaily(ctx)
		if err != nil {
			return fmt.Errorf("reset items: %w", err)
		}
		res = Result{Slots: slots, Items: items}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return res, nil
}
