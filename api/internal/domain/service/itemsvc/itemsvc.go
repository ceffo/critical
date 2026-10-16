// Package itemsvc owns the counted items (and adding,
// editing and removing them by hand). One model: an
// item's Behaviour says how its count moves. Use spends one of any item (a
// charges item is removed at 0), Restock adds to a stock item, ResetDaily
// refills the daily items on a long rest. Every change at the table is one
// action-log entry with the item's "items:<id>" snapshot, so popping it
// restores the count (and a removed item).
package itemsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	List(ctx context.Context) ([]Item, error)
	// Use spends one count of the item atomically and returns the updated
	// item; a charges item spent to 0 is removed (Item.Removed). It must run
	// in the caller's transaction (the attack that fires an arrow) and the
	// caller logs UseSnapshot of the result. ErrItemNotFound for an unknown
	// id, ErrItemEmpty at 0.
	Use(ctx context.Context, itemID string) (Item, error)
	// UseItem spends one count of any item (Use), rolls its effect dice (if
	// any) through the dice port and records the use in the action log with
	// the item's snapshot, all in one transaction.
	UseItem(ctx context.Context, characterID int64, itemID string) (*UseResult, error)
	// Restock adds n (1..MaxRestock) to a stock item and logs it with the
	// item's snapshot, in one transaction. ErrNotRestockable for a charges or
	// daily item, ErrInvalidInput for n out of range.
	Restock(ctx context.Context, characterID int64, itemID string, n int) (*RestockResult, error)
	// ResetDaily refills every daily item to its max (long rest) and
	// returns the full item list.
	ResetDaily(ctx context.Context) ([]Item, error)
	// Create adds an item (loot, a purchase) and logs it with the item's
	// snapshot, in one transaction. A *domain.FieldError for invalid input.
	Create(ctx context.Context, characterID int64, in ItemInput) (*ChangeResult, error)
	// Update edits any field of an item (only the set fields of the patch)
	// and logs the before/after fields. ErrItemNotFound, *domain.FieldError.
	Update(ctx context.Context, characterID int64, itemID string, p ItemPatch) (*ChangeResult, error)
	// Delete removes an item (sold, broken) and logs it; popping the entry
	// brings it back with its id, place and count. ErrItemNotFound.
	Delete(ctx context.Context, characterID int64, itemID string) (*ChangeResult, error)

	// ItemPowers is every carried item's powers, whatever its count (ADR
	// 0016 steps 9 and 11): buffsvc's Items port.
	ItemPowers(ctx context.Context) ([]buffsvc.ItemPower, error)

	// Restorers maps the items state key to the Restorer that writes the
	// counts of a "before" snapshot back when an action-log entry is voided.
	// The composition root registers it on the action log.
	Restorers() map[string]actionlog.Restorer
}

// DAL is the driven port. Decrement and Increment must be single atomic
// statements; Decrement never takes the count below 0. Every method joins
// the transaction carried by ctx.
type DAL interface {
	List(ctx context.Context) ([]Item, error)
	// Get returns one item, or ErrItemNotFound.
	Get(ctx context.Context, itemID string) (Item, error)
	// Update overwrites every field of an existing item (position
	// included); ErrItemNotFound for an unknown id.
	Update(ctx context.Context, item Item) error
	// Decrement returns the updated item, ErrItemNotFound or ErrItemEmpty.
	Decrement(ctx context.Context, itemID string) (Item, error)
	// Increment adds n and returns the updated item, or ErrItemNotFound.
	Increment(ctx context.Context, itemID string, n int) (Item, error)
	// ResetDaily refills every daily item to its max count.
	ResetDaily(ctx context.Context) error
	// SetCount writes a count back; ErrItemNotFound for an unknown id.
	SetCount(ctx context.Context, itemID string, count int) error
	// Delete removes an item; ErrItemNotFound for an unknown id.
	Delete(ctx context.Context, itemID string) error
	// Insert writes a whole item back (restoring a removed one).
	Insert(ctx context.Context, item Item) error
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
	item, err := s.dal.Decrement(ctx, itemID)
	if err != nil {
		return Item{}, err
	}
	if item.Removed() {
		if err := s.dal.Delete(ctx, item.ID); err != nil {
			return Item{}, fmt.Errorf("remove spent item %s: %w", item.ID, err)
		}
	}
	return item, nil
}

// ResetDaily refills the daily items only: stock and charges items are never
// refilled by a rest.
func (s *service) ResetDaily(ctx context.Context) ([]Item, error) {
	if err := s.dal.ResetDaily(ctx); err != nil {
		return nil, fmt.Errorf("reset daily items: %w", err)
	}
	return s.dal.List(ctx)
}
