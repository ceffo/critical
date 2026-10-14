package itemsvc

import "errors"

// ErrItemNotFound is returned when an item id is unknown.
var ErrItemNotFound = errors.New("item not found")

// ErrItemEmpty is returned by Use when the item has nothing left.
var ErrItemEmpty = errors.New("item is empty")

// ErrNotRestockable is returned by Restock for a charges or daily item: only
// stock items go up at the table.
var ErrNotRestockable = errors.New("item cannot be restocked")
