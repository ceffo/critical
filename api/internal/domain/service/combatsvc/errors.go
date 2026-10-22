package combatsvc

import "errors"

// ErrWeaponNotFound is returned when ResolveAttack cannot find the requested
// weapon in the catalog.
var ErrWeaponNotFound = errors.New("weapon not found")

// ErrSaveImmune refuses the adjustment of a save the character was immune
// to (ADR 0006 step 10.2): no roll, so nothing for the target to move.
var ErrSaveImmune = errors.New("the character is immune to that spell: there is no roll to adjust")
