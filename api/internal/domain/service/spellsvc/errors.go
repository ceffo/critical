package spellsvc

import "errors"

// Service-local sentinels.
var (
	ErrInvalidSlotIndex = errors.New("invalid slot index")
	ErrSlotAlreadyUsed  = errors.New("slot already used")
)
