// Package itemsvc owns the counted items (ammo, charged items, consumables).
// Counters change at the table: Use spends one, ResetDaily refills the per-day
// items on a long rest.
package itemsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	List(ctx context.Context) ([]Item, error)
	// Use spends one count of the item atomically and returns the updated
	// item. ErrItemNotFound for an unknown id, ErrItemEmpty at 0.
	Use(ctx context.Context, itemID string) (Item, error)
	// ResetDaily refills every per-day item to its max (long rest) and
	// returns the full item list.
	ResetDaily(ctx context.Context) ([]Item, error)
}

// DAL is the driven port. Decrement must be a single atomic statement that
// never takes the count below 0.
type DAL interface {
	List(ctx context.Context) ([]Item, error)
	// Decrement returns the updated item, ErrItemNotFound or ErrItemEmpty.
	Decrement(ctx context.Context, itemID string) (Item, error)
	ResetPerDay(ctx context.Context) error
}

type service struct {
	dal DAL
}

// New constructs the service.
func New(dal DAL) (SVC, error) {
	if dal == nil {
		return nil, fmt.Errorf("itemsvc: nil dal")
	}
	return &service{dal: dal}, nil
}

// NewWithConfig is here for shape consistency.
func NewWithConfig(_ *Config, dal DAL) (SVC, error) { return New(dal) }

// Config is reserved for future options.
type Config struct{}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config { return &Config{} }

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

func (s *service) List(ctx context.Context) ([]Item, error) { return s.dal.List(ctx) }

func (s *service) Use(ctx context.Context, itemID string) (Item, error) {
	return s.dal.Decrement(ctx, itemID)
}

func (s *service) ResetDaily(ctx context.Context) ([]Item, error) {
	if err := s.dal.ResetPerDay(ctx); err != nil {
		return nil, fmt.Errorf("reset per-day items: %w", err)
	}
	return s.dal.List(ctx)
}
