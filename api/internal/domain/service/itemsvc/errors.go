package itemsvc

import (
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
)

// ErrItemNotFound is returned when an item id is unknown.
var ErrItemNotFound = errors.New("item not found")

// ErrItemEmpty is returned by Use when the item has nothing left.
var ErrItemEmpty = errors.New("item is empty")

// ErrNotRestockable is returned by Restock for a charges or daily item: only
// stock items go up at the table.
var ErrNotRestockable = errors.New("item cannot be restocked")

// ErrNotUsable is returned by UseItem for a weapon, an armour or a shield:
// one object, never spent (ADR 0049 step 2).
var ErrNotUsable = errors.New("item cannot be used")

// ErrAbilityNotFound is returned by UseAbility for an ability id the item does
// not have (ADR 0049 step 7).
var ErrAbilityNotFound = errors.New("ability not found")

// ErrAbilityExhausted is returned by UseAbility when the ability's limit
// cannot pay a use: nothing left (ADR 0049 step 5, code `exhausted`).
var ErrAbilityExhausted = errors.New("ability exhausted")

// ErrQuantityBound is returned by DAL.Increment when the add would take the
// quantity above MaxQuantity (ADR 0049 step 2); it is an invalid input (400).
var ErrQuantityBound = fmt.Errorf("%w: a quantity is at most %d", domain.ErrInvalidInput, MaxQuantity)
