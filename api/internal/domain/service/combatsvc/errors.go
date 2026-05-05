package combatsvc

import "errors"

// ErrWeaponNotFound is returned when ResolveAttack cannot find the requested
// weapon in the catalog.
var ErrWeaponNotFound = errors.New("weapon not found")
