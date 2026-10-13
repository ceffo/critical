// Package itemsvc owns the counted items (ammo, charged items, consumables).
// Counters change at the table: Use spends one, ResetDaily refills the per-day
// items on a long rest.
package itemsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	List(ctx context.Context) ([]Item, error)
	// Use spends one count of the item atomically and returns the updated
	// item. ErrItemNotFound for an unknown id, ErrItemEmpty at 0.
	Use(ctx context.Context, itemID string) (Item, error)
	// UseItem spends one count of a charged item or consumable, rolls its
	// effect dice (if any) through the dice port and records the use in the
	// action log with a before/after snapshot, all in one transaction.
	// Ammunition is spent by attacks, not here (ErrInvalidInput).
	UseItem(ctx context.Context, characterID int64, itemID string) (*UseResult, error)
	// ResetDaily refills every per-day item to its max (long rest) and
	// returns the full item list.
	ResetDaily(ctx context.Context) ([]Item, error)

	// Restorers maps the items state key to the Restorer that writes the
	// counts of a "before" snapshot back when an action-log entry is voided.
	// The composition root registers it on the action log.
	Restorers() map[string]actionlog.Restorer
}

// DAL is the driven port. Decrement must be a single atomic statement that
// never takes the count below 0.
type DAL interface {
	List(ctx context.Context) ([]Item, error)
	// Decrement returns the updated item, ErrItemNotFound or ErrItemEmpty.
	Decrement(ctx context.Context, itemID string) (Item, error)
	ResetPerDay(ctx context.Context) error
	// SetCount writes a count back; ErrItemNotFound for an unknown id.
	SetCount(ctx context.Context, itemID string, count int) error
}

// Dice is the dice adapter port.
type Dice interface {
	RollDie(sides int) int
}

// Log is the slice of the action log item uses are recorded into.
type Log interface {
	actionlog.Recorder
}

// Deps are the collaborators UseItem needs besides the DAL.
type Deps struct {
	Dice Dice
	Log  Log
	Tx   domain.TxRunner
}

type service struct {
	dal  DAL
	dice Dice
	log  Log
	tx   domain.TxRunner
}

// New constructs the service.
func New(dal DAL, d Deps) (SVC, error) {
	if dal == nil || d.Dice == nil || d.Log == nil || d.Tx == nil {
		return nil, fmt.Errorf("itemsvc: nil dependency")
	}
	return &service{dal: dal, dice: d.Dice, log: d.Log, tx: d.Tx}, nil
}

// NewWithConfig is here for shape consistency.
func NewWithConfig(_ *Config, dal DAL, d Deps) (SVC, error) { return New(dal, d) }

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
