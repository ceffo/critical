// Package itemsvc owns a character's equipment, the one possessions list of
// ADR 0033: weapons, ammunition, armour, shields, magic items and gear are
// one model (Item), each piece carried, worn or wielded (SetState). An
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
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	// List is the character's items in sheet order.
	List(ctx context.Context, characterID int64) ([]Item, error)
	// Use spends one count of the item atomically and returns the updated
	// item; a charges item spent to 0 is removed (Item.Removed). It must run
	// in the caller's transaction (the attack that fires an arrow) and the
	// caller logs UseSnapshot of the result. ErrItemNotFound for an unknown
	// id, ErrItemEmpty at 0.
	Use(ctx context.Context, characterID int64, itemID string) (Item, error)
	// UseItem spends one count of any item (Use), rolls its effect dice (if
	// any) through the dice port and records the use in the action log with
	// the item's snapshot, all in one transaction.
	UseItem(ctx context.Context, characterID int64, itemID string) (*UseResult, error)
	// Restock adds n (1..MaxRestock) to a stock item and logs it with the
	// item's snapshot, in one transaction. ErrNotRestockable for a charges or
	// daily item, ErrInvalidInput for n out of range.
	Restock(ctx context.Context, characterID int64, itemID string, n int) (*RestockResult, error)
	// ResetDaily refills every daily item of the character to its max (long
	// rest) and returns the character's full item list.
	ResetDaily(ctx context.Context, characterID int64) ([]Item, error)
	// Create adds an item (loot, a purchase) and logs it with the item's
	// snapshot, in one transaction: typed, or picked from the catalogue
	// (ItemInput.CatalogueCode, ADR 0033 step 24); Equip equips it in the
	// same transaction (one undo). A *domain.FieldError for invalid input.
	Create(ctx context.Context, characterID int64, in ItemInput) (*ChangeResult, error)
	// Catalogue is the equipment reference data pieces are picked from
	// (ADR 0033 step 21), empty before catalogue format 7.
	Catalogue() ruletablesvc.Equipment
	// Update edits any field of an item (only the set fields of the patch)
	// and logs the before/after fields. ErrItemNotFound, *domain.FieldError.
	Update(ctx context.Context, characterID int64, itemID string, p ItemPatch) (*ChangeResult, error)
	// Delete removes an item (sold, broken) and logs it; popping the entry
	// brings it back with its id, place and count. ErrItemNotFound.
	Delete(ctx context.Context, characterID int64, itemID string) (*ChangeResult, error)
	// SetState wears, takes off, wields or puts away a piece (ADR 0033 steps
	// 6 and 7): one equipment_state entry, nothing for the same state again.
	// swap takes off the body armour or shield worn in the way. A
	// *domain.FieldError on "state" (CodeNotAllowed, CodeSlotTaken).
	SetState(ctx context.Context, characterID int64, itemID string, state State, swap bool) (*StateResult, error)
	// Wield is SetState(wielded) inside the caller's transaction: the attack
	// form wields the weapon picked (ADR 0033 step 8).
	Wield(ctx context.Context, characterID int64, itemID string) (*StateResult, error)
	// Reviews is the review list (ADR 0033 step 27), in list order.
	Reviews(ctx context.Context, characterID int64) ([]Review, error)
	// DismissReview takes a piece off the review list, not logged.
	// ErrItemNotFound.
	DismissReview(ctx context.Context, characterID int64, itemID string) error

	// ItemPowers is every worn or wielded piece's powers, whatever its count
	// (ADR 0016 steps 9 and 11, ADR 0033 step 9): buffsvc's Items port.
	ItemPowers(ctx context.Context, characterID int64) ([]buffsvc.ItemPower, error)
}

// DAL is the driven port. Items belong to a character: every method reads
// or writes the items of characterID only, and an item id is looked up among
// that character's (another character's item of the same id is unknown).
// Decrement and Increment must be single atomic statements; Decrement never
// takes the count below 0. Every method joins the transaction carried by ctx.
type DAL interface {
	List(ctx context.Context, characterID int64) ([]Item, error)
	// Get returns one item, or ErrItemNotFound.
	Get(ctx context.Context, characterID int64, itemID string) (Item, error)
	// Update overwrites every field of an existing item (position
	// included); ErrItemNotFound for an unknown id.
	Update(ctx context.Context, characterID int64, item Item) error
	// Decrement returns the updated item, ErrItemNotFound or ErrItemEmpty.
	Decrement(ctx context.Context, characterID int64, itemID string) (Item, error)
	// Increment adds n and returns the updated item, or ErrItemNotFound.
	Increment(ctx context.Context, characterID int64, itemID string, n int) (Item, error)
	// ResetDaily refills every daily item of the character to its max count.
	ResetDaily(ctx context.Context, characterID int64) error
	// SetCount writes a count back; ErrItemNotFound for an unknown id.
	SetCount(ctx context.Context, characterID int64, itemID string, count int) error
	// Delete removes an item; ErrItemNotFound for an unknown id.
	Delete(ctx context.Context, characterID int64, itemID string) error
	// Insert writes a whole item for the character (a new one, or a
	// removed one restored).
	Insert(ctx context.Context, characterID int64, item Item) error
	// Reviews is the character's review rows (ADR 0033 step 27), any order.
	Reviews(ctx context.Context, characterID int64) ([]Review, error)
	// DeleteReviews removes every review row of a piece (none: no error).
	DeleteReviews(ctx context.Context, characterID int64, itemID string) error
}

// Characters reads the character whose pieces change: a specialized weapon
// needs a class that specializes (ADR 0030 step 12, Derived.Specializes).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Dice is the dice adapter port.
type Dice interface {
	RollDie(sides int) int
}

// Log is the slice of the action log item uses are recorded into.
type Log interface {
	actionlog.Recorder
}

// Deps are the service's collaborators besides the DAL. Equipment is the
// catalogue's equipment reference data (ADR 0033 step 21): a piece is picked
// from it (step 24) and an old weapon entry popped back is matched to its
// weapons (step 26.4); nil before catalogue format 7.
type Deps struct {
	Dice      Dice
	Log       Log
	Tx        domain.TxRunner
	Chars     Characters
	Equipment *ruletablesvc.Equipment
	// Initiative corrects the round's initiative when a magic item is used
	// (ADR 0035 item 6.2); InitiativeItems is PHB Table 56, the modifier of
	// each magic kind (nil before catalogue format 7: a use corrects
	// nothing).
	Initiative      Initiative
	InitiativeItems *ruletablesvc.InitiativeItems
}

// Initiative corrects the round's initiative by an item used (buffsvc),
// inside the use's transaction, as its last entry; it does nothing out of
// combat.
type Initiative interface {
	CorrectInitiative(ctx context.Context, characterID int64, cause buffsvc.InitiativeModifier) (*buffsvc.Initiative, error)
}

type service struct {
	dal       DAL
	dice      Dice
	log       Log
	tx        domain.TxRunner
	chars     Characters
	catalogue []ruletablesvc.Weapon
	equipment *ruletablesvc.Equipment
	// initiative and initiativeItems: ADR 0035 item 6.2 (Deps).
	initiative      Initiative
	initiativeItems *ruletablesvc.InitiativeItems
}

// New constructs the service.
func New(dal DAL, d Deps) (SVC, error) {
	if dal == nil || d.Dice == nil || d.Log == nil || d.Tx == nil || d.Chars == nil || d.Initiative == nil {
		return nil, fmt.Errorf("itemsvc: nil dependency")
	}
	s := &service{
		dal: dal, dice: d.Dice, log: d.Log, tx: d.Tx, chars: d.Chars, equipment: d.Equipment,
		initiative: d.Initiative, initiativeItems: d.InitiativeItems,
	}
	if d.Equipment != nil {
		s.catalogue = d.Equipment.Weapons
	}
	return s, nil
}

// NewWithConfig is here for shape consistency.
func NewWithConfig(_ *Config, dal DAL, d Deps) (SVC, error) { return New(dal, d) }

// Config is reserved for future options.
type Config struct{}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config { return &Config{} }

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

func (s *service) List(ctx context.Context, characterID int64) ([]Item, error) {
	return s.dal.List(ctx, characterID)
}

func (s *service) Use(ctx context.Context, characterID int64, itemID string) (Item, error) {
	item, err := s.dal.Decrement(ctx, characterID, itemID)
	if err != nil {
		return Item{}, err
	}
	if item.Removed() {
		if err := s.dal.Delete(ctx, characterID, item.ID); err != nil {
			return Item{}, fmt.Errorf("remove spent item %s: %w", item.ID, err)
		}
	}
	return item, nil
}

// ResetDaily refills the daily items only: stock and charges items are never
// refilled by a rest.
func (s *service) ResetDaily(ctx context.Context, characterID int64) ([]Item, error) {
	if err := s.dal.ResetDaily(ctx, characterID); err != nil {
		return nil, fmt.Errorf("reset daily items: %w", err)
	}
	return s.dal.List(ctx, characterID)
}
