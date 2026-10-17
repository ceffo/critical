package spellsvc

import "errors"

// Service-local sentinels.
var (
	ErrInvalidSlotIndex = errors.New("invalid slot index")
	ErrSlotAlreadyUsed  = errors.New("slot already used")
	// ErrSpellNotFound: the catalogue holds no spell with that id.
	ErrSpellNotFound = errors.New("spell not found")
)
