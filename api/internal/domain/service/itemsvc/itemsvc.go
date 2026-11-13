// Package itemsvc owns a character's equipment, the one possessions list of
// ADR 0033: weapons, ammunition, armour, shields, magic items and gear are
// one model (Item), each piece carried, worn or wielded (SetState). An
// item has a quantity and abilities (ADR 0049). UseAbility pays an ability's
// limit and runs its effects, Use and UseItem spend one of a stock (a potion
// or a scroll spent to 0 is removed), Restock adds to a stock, ResetDaily
// refills the per_day abilities on a long rest. Every change at the table is
// one action-log entry with the item's "items:<id>" snapshot, so popping it
// restores the counters (and a removed item).
package itemsvc

import (
	"context"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	// List is the character's items in sheet order.
	List(ctx context.Context, characterID int64) ([]Item, error)
	// Use spends one count of the item atomically and returns the updated
	// item; a potion or a scroll spent to 0 is removed (Item.Removed). It must run
	// in the caller's transaction (the attack that fires an arrow) and the
	// caller logs UseSnapshot of the result. ErrItemNotFound for an unknown
	// id, ErrItemEmpty at 0.
	Use(ctx context.Context, characterID int64, itemID string) (Item, error)
	// UseItem spends one of a stock item's quantity (the inventory's minus, ADR
	// 0049 step 2): no ability, no effect, no initiative; logged as an item_use
	// entry with the item's snapshot, in one transaction. ErrNotUsable for an
	// item with no stock or with a per_day or charges ability.
	UseItem(ctx context.Context, characterID int64, itemID string) (*UseResult, error)
	// UseAbility uses one ability of an item (ADR 0049 step 7): pays its limit,
	// runs its effects, logs the use and corrects the round's initiative, in
	// one transaction. ErrItemNotFound, ErrAbilityNotFound, ErrAbilityExhausted.
	UseAbility(ctx context.Context, characterID int64, itemID, abilityID string) (*AbilityUseResult, error)
	// Restock adds n (1..MaxRestock) to a stock item and logs it with the
	// item's snapshot, in one transaction. ErrNotRestockable for an item with
	// no stock or with a per_day or charges ability, ErrInvalidInput for n out of range.
	Restock(ctx context.Context, characterID int64, itemID string, n int) (*RestockResult, error)
	// ResetDaily refills every per_day ability of the character's items to its
	// max (long rest, ADR 0049 step 6) and returns the character's full item
	// list. It must run in the caller's transaction.
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
	// Warnings is the character's class warnings (ADR 0033 step 20) for the
	// pieces worn or wielded, computed on every read, from the full list of
	// pieces the caller holds, or (nil) the pieces as stored: every answer
	// that serves pieces maps them with it.
	Warnings(ctx context.Context, characterID int64, items []Item) ([]Warning, error)
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
// takes the count below 0 and Increment never above MaxQuantity (ErrQuantityBound). Every method joins the transaction carried by ctx.
type DAL interface {
	List(ctx context.Context, characterID int64) ([]Item, error)
	// Get returns one item, or ErrItemNotFound.
	Get(ctx context.Context, characterID int64, itemID string) (Item, error)
	// Update overwrites every field of an existing item (position
	// included); ErrItemNotFound for an unknown id.
	Update(ctx context.Context, characterID int64, item Item) error
	// Decrement returns the updated item, ErrItemNotFound or ErrItemEmpty.
	Decrement(ctx context.Context, characterID int64, itemID string) (Item, error)
	// Increment adds n and returns the updated item, ErrItemNotFound or
	// ErrQuantityBound (count + n above MaxQuantity, nothing written).
	Increment(ctx context.Context, characterID int64, itemID string, n int) (Item, error)
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

// Characters reads the character whose pieces are read or change: its class
// warnings from its class (ADR 0033 step 20), and a weapon's Specialized from
// the weapon type it is specialized in (ADR 0037 step 9), a read of the
// character row alone.
type Characters interface {
	View(ctx context.Context, id int64) (charactersvc.View, error)
	WeaponProficiency(ctx context.Context, id int64) (skills.WeaponProficiency, error)
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
	// Classes reads a class's definition for its equipment limits (ADR 0033
	// step 20, ADR 0040 step 1.6). Required: a nil one would drop every class
	// warning silently.
	Classes ClassLookup
	// Initiative corrects the round's initiative when a magic item is used
	// (ADR 0035 item 6.2); InitiativeItems is PHB Table 56, the modifier of
	// each magic kind (nil before catalogue format 7: a use corrects
	// nothing).
	Initiative      Initiative
	InitiativeItems *ruletablesvc.InitiativeItems
	// Spells is the spell catalogue: a scroll names one of its spells (ADR
	// 0033 step 2) and its use reads that spell's casting time (ADR 0035
	// item 6.2). Nil: no spell is known (naming one is refused, a named
	// scroll's casting time reads as special).
	Spells Spells
	// Rulesets gives the character its ruleset, whose weapon lift the class
	// warnings read (ADR 0041 steps 2.5 and 3). Required.
	Rulesets ruleset.Resolver
	// Strength is the character's Strength in effect (buffsvc), the one the
	// attack uses: the bow_too_strong warning reads it (ADR 0002 step 4.1).
	// Nil reads the sheet's Strength instead.
	Strength EffectiveStrength
}

// EffectiveStrength is the character's Strength with its Strength buffs
// (buffsvc.SVC.GetEffectiveStr).
type EffectiveStrength interface {
	GetEffectiveStr(ctx context.Context, characterID int64) (buffsvc.Strength, error)
}

// ClassLookup is the class definitions (ruletablesvc.Tables.Class).
type ClassLookup interface {
	Class(class string) (classes.Class, error)
}

// Spells looks a spell up in the catalogue (spelldal's catalogue, as
// castsvc reads it).
type Spells interface {
	Describe(ctx context.Context, spellID string) (spellsvc.Spell, bool, error)
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
	classes   ClassLookup
	// initiative and initiativeItems: ADR 0035 item 6.2 (Deps).
	initiative      Initiative
	initiativeItems *ruletablesvc.InitiativeItems
	spells          Spells
	rulesets        ruleset.Resolver
	strength        EffectiveStrength
}

// New constructs the service.
func New(dal DAL, d Deps) (SVC, error) {
	if dal == nil || d.Dice == nil || d.Log == nil || d.Tx == nil || d.Chars == nil || d.Classes == nil || d.Initiative == nil || d.Rulesets == nil {
		return nil, fmt.Errorf("itemsvc: nil dependency")
	}
	s := &service{
		dal: specializedDAL{DAL: dal, chars: d.Chars}, dice: d.Dice, log: d.Log, tx: d.Tx, chars: d.Chars, equipment: d.Equipment, classes: d.Classes,
		initiative: d.Initiative, initiativeItems: d.InitiativeItems, spells: d.Spells, rulesets: d.Rulesets,
		strength: d.Strength,
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

// ResetDaily is the item half of the long rest (ADR 0049 step 6): every
// per_day ability of every item goes back to its max. charges and with_item
// abilities are untouched (nothing recharges them), and only an item whose
// counter changes is written. It returns the character's full list.
func (s *service) ResetDaily(ctx context.Context, characterID int64) ([]Item, error) {
	items, err := s.dal.List(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	for n, it := range items {
		refilled, ok := it.refilled()
		if !ok {
			continue
		}
		if err := s.dal.Update(ctx, characterID, refilled); err != nil {
			return nil, fmt.Errorf("reset daily item %s: %w", it.ID, err)
		}
		items[n] = refilled
	}
	return items, nil
}

// refilled is the item with its per_day abilities back to their max, ok false
// when none was short of it.
func (i Item) refilled() (Item, bool) {
	out, changed := i, false
	for n, a := range i.Abilities {
		if a.Limit.Kind != LimitPerDay || a.Limit.Left >= a.Limit.Max {
			continue
		}
		if !changed {
			out.Abilities, changed = slices.Clone(i.Abilities), true
		}
		out.Abilities[n].Limit.Left = a.Limit.Max
	}
	return out, changed
}

// SameCounters reports whether o has every ability counter i has: what a long
// rest compares to know which items it refilled.
func (i Item) SameCounters(o Item) bool {
	return i.Count == o.Count && slices.EqualFunc(i.Abilities, o.Abilities, func(a, b Ability) bool {
		return a.ID == b.ID && a.Limit == b.Limit
	})
}
